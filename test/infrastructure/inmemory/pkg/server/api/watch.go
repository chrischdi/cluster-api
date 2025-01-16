/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Event records a lifecycle event for a Kubernetes object.
type Event struct {
	Type   watch.EventType `json:"type,omitempty"`
	Object runtime.Object  `json:"object,omitempty"`
}

// WatchEventDispatcher dispatches events for a single resourceGroup.
type WatchEventDispatcher struct {
	resourceGroup string
	events        chan *Event
}

// OnCreate dispatches Create events.
func (m *WatchEventDispatcher) OnCreate(resourceGroup string, o client.Object) {
	if resourceGroup != m.resourceGroup {
		return
	}
	m.events <- &Event{
		Type:   watch.Added,
		Object: o,
	}
}

// OnUpdate dispatches Update events.
func (m *WatchEventDispatcher) OnUpdate(resourceGroup string, _, o client.Object) {
	if resourceGroup != m.resourceGroup {
		return
	}
	m.events <- &Event{
		Type:   watch.Modified,
		Object: o,
	}
}

// OnDelete dispatches Delete events.
func (m *WatchEventDispatcher) OnDelete(resourceGroup string, o client.Object) {
	if resourceGroup != m.resourceGroup {
		return
	}
	m.events <- &Event{
		Type:   watch.Deleted,
		Object: o,
	}
}

// OnGeneric dispatches Generic events.
func (m *WatchEventDispatcher) OnGeneric(resourceGroup string, o client.Object) {
	if resourceGroup != m.resourceGroup {
		return
	}
	m.events <- &Event{
		Type:   "GENERIC",
		Object: o,
	}
}

func (h *apiServerHandler) watchForResource(req *restful.Request, resp *restful.Response, resourceGroup string, gvk schema.GroupVersionKind) (reterr error) {
	ctx := req.Request.Context()
	queryTimeout := req.QueryParameter("timeoutSeconds")
	c := h.manager.GetCache()
	i, err := c.GetInformerForKind(ctx, gvk)
	if err != nil {
		return err
	}
	h.log.Info(fmt.Sprintf("Serving Watch for %v", req.Request.URL), "resourceGroup", resourceGroup)
	// With an unbuffered event channel RemoveEventHandler could be blocked because it requires a lock on the informer.
	// When Run stops reading from the channel the informer could be blocked with an unbuffered chanel and then RemoveEventHandler never goes through.
	// 1000 is used to avoid deadlocks in clusters with a higher number of Machines/Nodes.
	events := make(chan *Event, 1000)
	watcher := &WatchEventDispatcher{
		resourceGroup: resourceGroup,
		events:        events,
	}

	if err := i.AddEventHandler(watcher); err != nil {
		return err
	}

	// Defer cleanup which removes the event handler and ensures the channel is empty of events.
	defer func() {
		// Doing this to ensure the channel is empty.
		// This reduces the probability of a deadlock when removing the event handler.
	L:
		for {
			select {
			case event, ok := <-events:
				o, _ := event.Object.(client.Object)
				fmt.Printf("watchForResource[%s]: dropped event while shutting down watch for gvk %s ok=%t name=%s\n", resourceGroup, gvk.String(), ok, o.GetName())
			default:
				break L
			}
		}
		reterr = i.RemoveEventHandler(watcher)
		// Note: After we removed the handler, no new events will be written to the events channel.
	}()

	return watcher.Run(ctx, queryTimeout, resp)
}

// Run serves a series of encoded events via HTTP with Transfer-Encoding: chunked.
func (m *WatchEventDispatcher) Run(ctx context.Context, timeout string, w http.ResponseWriter) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("can't start Watch: can't get http.Flusher")
	}
	resp, ok := w.(*restful.Response)
	if !ok {
		return errors.New("can't start Watch: can't get restful.Response")
	}
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	seconds, err := time.ParseDuration(fmt.Sprintf("%ss", timeout))
	if err != nil {
		return errors.Wrapf(err, "can't start Watch: could parse timeout %s", timeout)
	}

	ctx, cancel := context.WithTimeoutCause(ctx, seconds, errors.New("Run(...) timed out"))
	defer cancel()
	timeoutTimer, cleanup := setTimer(seconds)
	defer cleanup()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("WatchEventDispatcher.Run[%s] ctx.Done: returning nil with %d leftover events: %v\n", m.resourceGroup, len(m.events), ctx.Err())
			return nil
		case <-timeoutTimer:
			fmt.Printf("WatchEventDispatcher.Run[%s] timeoutTimer: returning nil with %d leftover events\n", m.resourceGroup, len(m.events))
			return nil
		case event, ok := <-m.events:
			if !ok {
				// End of results.
				return nil
			}
			o, _ := event.Object.(client.Object)
			fmt.Printf("WatchEventDispatcher.Run[%s] event: Writing event (type=%s) for %s %s\n", m.resourceGroup, event.Type, o.GetObjectKind().GroupVersionKind().Kind, o.GetName())
			if err := resp.WriteEntity(event); err != nil {
				fmt.Printf("WatchEventDispatcher.Run[%s] event: Error response %s %s: %v\n", m.resourceGroup, o.GetObjectKind().GroupVersionKind().Kind, o.GetName(), err)
				if err := resp.WriteErrorString(http.StatusInternalServerError, err.Error()); err != nil {
					fmt.Printf("WatchEventDispatcher.Run[%s] event: Error writing error response %s %s: %v\n", m.resourceGroup, o.GetObjectKind().GroupVersionKind().Kind, o.GetName(), err)
				}
			} else {
				fmt.Printf("WatchEventDispatcher.Run[%s] event: Writing event for %s %s succeeded\n", m.resourceGroup, o.GetObjectKind().GroupVersionKind().Kind, o.GetName())
			}
			if len(m.events) == 0 {
				flusher.Flush()
			}
		}
	}
}

var neverExitWatch <-chan time.Time = make(chan time.Time)
var neverExitWatchStop = func() bool { return false }

// setTimer creates a time.Timer with the passed `timeout` or a default timeout of 120 seconds if `timeout` is empty.
func setTimer(timeout time.Duration) (<-chan time.Time, func() bool) {
	if timeout == 0 {
		return neverExitWatch, func() bool { return false }
	}

	t := time.NewTimer(timeout)
	return t.C, t.Stop
}
