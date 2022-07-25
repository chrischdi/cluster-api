/*
Copyright 2022 The Kubernetes Authors.

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

// Package container provides an interface for interacting with containerd
package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/cmd/ctr/commands/tasks"
	"github.com/containerd/containerd/images/archive"
	clabels "github.com/containerd/containerd/labels"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/pkg/cap"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/containerd/snapshots"
	gocni "github.com/containerd/go-cni"
	"github.com/containerd/nerdctl/pkg/containerinspector"
	"github.com/containerd/nerdctl/pkg/formatter"
	"github.com/containerd/nerdctl/pkg/idgen"
	"github.com/containerd/nerdctl/pkg/idutil/containerwalker"
	"github.com/containerd/nerdctl/pkg/inspecttypes/dockercompat"
	"github.com/containerd/nerdctl/pkg/labels"
	"github.com/containerd/nerdctl/pkg/labels/k8slabels"
	"github.com/containerd/nerdctl/pkg/logging/jsonfile"
	"github.com/containerd/nerdctl/pkg/platformutil"
	"github.com/containerd/nerdctl/pkg/referenceutil"
	"github.com/containerd/nerdctl/pkg/strutil"
	"github.com/containerd/nerdctl/pkg/taskutil"
	"github.com/docker/cli/templates"
	"github.com/docker/docker/errdefs"
	sysignal "github.com/moby/sys/signal"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

type containerdRuntime struct {
	client    *containerd.Client
	namespace string
}

const defaultSignal = "SIGTERM"

func NewContainerdClient(socketPath string, namespace string) (Runtime, error) {
	client, err := containerd.New(socketPath)
	if err != nil {
		return &containerdRuntime{}, fmt.Errorf("failed to create containerd client.")
	}

	return &containerdRuntime{client: client, namespace: namespace}, nil
}

func (c *containerdRuntime) SaveContainerImage(ctx context.Context, image, dest string) error {
	var saveOpts = []archive.ExportOpt{}

	tar, err := os.Create(dest) //nolint:gosec // No security issue: dest is safe.
	if err != nil {
		return fmt.Errorf("failed to create destination file %q: %v", dest, err)
	}
	defer tar.Close()

	platform := []string{"amd64"}
	platMC, err := platformutil.NewMatchComparer(false, platform)
	if err != nil {
		return err
	}

	saveOpts = append(saveOpts, archive.WithPlatform(platMC))

	imageStore := c.client.ImageService()
	named, err := referenceutil.ParseAny(image)
	if err != nil {
		return err
	}

	saveOpts = append(saveOpts, archive.WithImage(imageStore, named.String()))
	return c.client.Export(ctx, tar, saveOpts...)
}

func (c *containerdRuntime) PullContainerImageIfNotExists(ctx context.Context, image string) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)

	ref, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return fmt.Errorf("failed to parse image reference: %v", err)
	}

	images, err := c.client.ListImages(ctx, fmt.Sprintf("name==%s", ref.String()))
	if err != nil {
		return fmt.Errorf("error listing images: %v", err)
	}

	// image already exists
	if len(images) > 0 {
		return nil
	}

	if _, err := c.client.Pull(ctx, image); err != nil {
		return fmt.Errorf("error pulling image: %v", err)
	}

	return nil
}

func (c *containerdRuntime) GetHostPort(ctx context.Context, containerName, portAndProtocol string) (string, error) {
	argPort := -1
	argProto := ""
	portProto := portAndProtocol
	var err error

	if portProto != "" {
		splitBySlash := strings.Split(portProto, "/")
		argPort, err = strconv.Atoi(splitBySlash[0])
		if err != nil {
			return "", err
		}
		if argPort <= 0 {
			return "", fmt.Errorf("unexpected port %d", argPort)
		}
		switch len(splitBySlash) {
		case 1:
			argProto = "tcp"
		case 2:
			argProto = strings.ToLower(splitBySlash[1])
		default:
			return "", fmt.Errorf("failed to parse %q", portProto)
		}
	}

	var port string
	walker := &containerwalker.ContainerWalker{
		Client: c.client,
		OnFound: func(ctx context.Context, found containerwalker.Found) error {
			if found.MatchCount > 1 {
				return fmt.Errorf("ambiguous ID %q", found.Req)
			}
			port, err = printPort(ctx, found.Req, found.Container, argPort, argProto)
			if err != nil {
				return err
			}
			return nil
		},
	}

	n, err := walker.Walk(ctx, containerName)
	if err != nil {
		return "", err
	} else if n == 0 {
		return "", fmt.Errorf("no such container %s", containerName)
	}
	return port, nil
}

type containerInspector struct {
	entries interface{}
}

func (c *containerdRuntime) GetContainerIPs(ctx context.Context, containerName string) (string, string, error) {
	f := &containerInspector{}
	walker := containerwalker.ContainerWalker{
		Client:  c.client,
		OnFound: f.Handler,
	}
	n, err := walker.Walk(ctx, containerName)
	if err != nil {
		return "", "", err
	} else if n == 0 {
		return "", "", fmt.Errorf("no such object %s", containerName)
	}

	format := "{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}"
	ip, err := formatSlice(f.entries, format)
	if err != nil {
		return "", "", err
	}

	format = "{{range.NetworkSettings.Networks}}{{.GlobalIPv6Address}}{{end}}"
	ipv6, err := formatSlice(f.entries, format)
	if err != nil {
		return "", "", err
	}

	return ip, ipv6, nil
}

func (c *containerdRuntime) ExecContainer(ctx context.Context, containerName string, config *ExecContainerInput, command string, args ...string) error {
	arg := append([]string{command}, args...)
	walker := &containerwalker.ContainerWalker{
		Client: c.client,
		OnFound: func(ctx context.Context, found containerwalker.Found) error {
			if found.MatchCount > 1 {
				return fmt.Errorf("ambiguous ID %q", found.Req)
			}
			return execActionWithContainer(ctx, config, arg, found.Container, c.client)
		},
	}
	n, err := walker.Walk(ctx, containerName)
	if err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("no such container %s", containerName)
	}
	return nil
}

func (c *containerdRuntime) RunContainer(ctx context.Context, runConfig *RunContainerInput, output io.Writer) error {
	var (
		err error
		id  = runConfig.Name
		ref = runConfig.Image

		tty = true
		// detach    = context.Bool("detach")
		// config    = context.IsSet("config")
		// enableCNI = context.Bool("cni")

		opts  []oci.SpecOpts
		cOpts []containerd.NewContainerOpts
		spec  containerd.NewContainerOpts
	)

	opts = append(opts, oci.WithDefaultSpec(), oci.WithDefaultUnixDevices)
	opts = append(opts, oci.WithTTY)
	opts = append(opts, oci.WithPrivileged, oci.WithAllDevicesAllowed, oci.WithHostDevices)

	snapshotter := "" // snapshotter name. Empty value stands for the default value. [$CONTAINERD_SNAPSHOTTER]
	i, err := c.client.ImageService().Get(ctx, ref)
	if err != nil {
		return err
	}
	image := containerd.NewImage(c.client, i)

	unpacked, err := image.IsUnpacked(ctx, snapshotter)
	if err != nil {
		return err
	}
	if !unpacked {
		if err := image.Unpack(ctx, snapshotter); err != nil {
			return err
		}
	}

	labels := buildLabels(runConfig.Labels, image.Labels())
	opts = append(opts, oci.WithImageConfig(image))
	cOpts = append(cOpts,
		containerd.WithImage(image),
		containerd.WithImageConfigLabels(image),
		containerd.WithAdditionalContainerLabels(labels),
		containerd.WithSnapshotter(snapshotter))

	cOpts = append(cOpts, containerd.WithNewSnapshot(id, image,
		snapshots.WithLabels(nil)))
	cOpts = append(cOpts, containerd.WithImageStopSignal(image, "SIGTERM"))

	opts = append(opts, oci.WithAnnotations(runConfig.Labels))
	var s specs.Spec
	spec = containerd.WithSpec(&s, opts...)

	cOpts = append(cOpts, spec)

	// oci.WithImageConfig (WithUsername, WithUserID) depends on access to rootfs for resolving via
	// the /etc/{passwd,group} files. So cOpts needs to have precedence over opts.
	container, err := c.client.NewContainer(ctx, id, cOpts...)
	if err != nil {
		return err
	}

	var con console.Console
	if tty {
		con = console.Current()
		defer con.Reset()
		if err := con.SetRaw(); err != nil {
			return err
		}
	}

	task, err := tasks.NewTask(ctx, c.client, container, "", con, false, "", nil)
	if err != nil {
		return err
	}

	var statusC <-chan containerd.ExitStatus
	if statusC, err = task.Wait(ctx); err != nil {
		return err
	}

	if err := task.Start(ctx); err != nil {
		return err
	}

	if tty {
		if err := tasks.HandleConsoleResize(ctx, task, con); err != nil {
			logrus.WithError(err).Error("console resize")
		}
	}

	status := <-statusC
	code, _, err := status.Result()
	if err != nil {
		return err
	}
	if _, err := task.Delete(ctx); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("", int(code))
	}
	return nil
}

func (c *containerdRuntime) ListContainers(ctx context.Context, filters FilterBuilder) ([]Container, error) {
	containers, err := c.client.Containers(ctx)
	if err != nil {
		return nil, err
	}
	var result []Container
	for _, c := range containers {
		info, err := c.Info(ctx, containerd.WithoutRefreshedMetadata)
		if err != nil {
			if errdefs.IsNotFound(err) {
				logrus.Warn(err)
				continue
			}
			return nil, err
		}
		imageName := info.Image
		cStatus := formatter.ContainerStatus(ctx, c)

		p := Container{
			Name:   getPrintableContainerName(info.Labels),
			Image:  imageName,
			Status: cStatus,
		}
		result = append(result, p)
	}
	return result, nil
}

// ContainerDebugInfo gets the container metadata and logs.
// Currently, only containers created with `nerdctl run -d` are supported for log collection.
func (c *containerdRuntime) ContainerDebugInfo(ctx context.Context, containerName string, w io.Writer) error {
	f := &containerInspector{}
	walker := containerwalker.ContainerWalker{
		Client:  c.client,
		OnFound: f.Handler,
	}
	n, err := walker.Walk(ctx, containerName)
	if err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("no such object %s", containerName)
	}

	containerInfo, err := json.MarshalIndent(f.entries, "", "    ")
	if err != nil {
		return err
	}
	fmt.Fprintln(w, "Inspected the container:")
	fmt.Fprintf(w, "%+v\n", string(containerInfo))

	// "1935db9" is from `$(echo -n "/run/containerd/containerd.sock" | sha256sum | cut -c1-8)``
	// on Windows it will return "%PROGRAMFILES%/nerdctl/1935db59"
	dataStore := "/var/lib/nerdctl/1935db59"
	ns := "default"

	walker = containerwalker.ContainerWalker{
		Client: c.client,
		OnFound: func(ctx context.Context, found containerwalker.Found) error {
			logJSONFilePath := jsonfile.Path(dataStore, ns, found.Container.ID())
			if _, err := os.Stat(logJSONFilePath); err != nil {
				return fmt.Errorf("failed to open %q, container is not created with `nerdctl run -d`?: %w", logJSONFilePath, err)
			}
			var reader io.Reader
			//chan for non-follow tail to check the logsEOF
			logsEOFChan := make(chan struct{})
			f, err := os.Open(logJSONFilePath)
			if err != nil {
				return err
			}
			defer f.Close()
			reader = f
			go func() {
				<-logsEOFChan
			}()

			fmt.Fprintln(w, "Got logs from the container:")
			return jsonfile.Decode(w, w, reader, false, "", "", logsEOFChan)
		},
	}
	n, err = walker.Walk(ctx, containerName)
	if err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("no such container %s", containerName)
	}
	return nil
}

func (c *containerdRuntime) DeleteContainer(ctx context.Context, containerName string) error {
	deleteOpts := []containerd.DeleteOpts{}
	deleteOpts = append(deleteOpts, containerd.WithSnapshotCleanup) // delete volumes
	container, err := c.client.LoadContainer(ctx, containerName)
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, cio.Load)
	if err != nil {
		return container.Delete(ctx, deleteOpts...)
	}
	status, err := task.Status(ctx)
	if err != nil {
		return err
	}
	if status.Status == containerd.Stopped || status.Status == containerd.Created {
		if _, err := task.Delete(ctx); err != nil {
			return err
		}
		return container.Delete(ctx, deleteOpts...)
	}
	return fmt.Errorf("cannot delete a non stopped container: %v", status)
}

func (c *containerdRuntime) KillContainer(ctx context.Context, containerName, signal string) error {
	sig, err := sysignal.ParseSignal(defaultSignal)
	if err != nil {
		return err
	}
	opts := []containerd.KillOpts{}
	opts = append(opts, containerd.WithKillAll) // send signal to all processes inside the container
	container, err := c.client.LoadContainer(ctx, containerName)
	if err != nil {
		return err
	}
	if signal != "" {
		sig, err = sysignal.ParseSignal(signal)
		if err != nil {
			return err
		}
	} else {
		sig, err = containerd.GetStopSignal(ctx, container, sig)
		if err != nil {
			return err
		}
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}
	// err = tasks.RemoveCniNetworkIfExist(ctx, container)
	// if err != nil {
	// 	return err
	// }
	return task.Kill(ctx, sig, opts...)
}

func getPrintableContainerName(containerLabels map[string]string) string {
	if name, ok := containerLabels[labels.Name]; ok {
		return name
	}

	if ns, ok := containerLabels[k8slabels.PodNamespace]; ok {
		if podName, ok := containerLabels[k8slabels.PodName]; ok {
			if containerName, ok := containerLabels[k8slabels.ContainerName]; ok {
				// Container
				return fmt.Sprintf("k8s://%s/%s/%s", ns, podName, containerName)
			} else {
				// Pod sandbox
				return fmt.Sprintf("k8s://%s/%s", ns, podName)
			}
		}
	}
	return ""
}

func (x *containerInspector) Handler(ctx context.Context, found containerwalker.Found) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	n, err := containerinspector.Inspect(ctx, found.Container)
	if err != nil {
		return err
	}

	d, err := dockercompat.ContainerFromNative(n)
	if err != nil {
		return err
	}
	x.entries = d
	return nil
}

func formatSlice(x interface{}, format string) (string, error) {
	var tmpl *template.Template
	var err error
	tmpl, err = parseTemplate(format)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, x); err != nil {
		if _, ok := err.(template.ExecError); ok {
			// FallBack to Raw Format
			if err = tryRawFormat(&b, x, tmpl); err != nil {
				return "", err
			}
		}
	}
	return b.String(), nil
}

func tryRawFormat(b *bytes.Buffer, f interface{}, tmpl *template.Template) error {
	m, err := json.MarshalIndent(f, "", "    ")
	if err != nil {
		return err
	}

	var raw interface{}
	rdr := bytes.NewReader(m)
	dec := json.NewDecoder(rdr)
	dec.UseNumber()

	if rawErr := dec.Decode(&raw); rawErr != nil {
		return fmt.Errorf("unable to read inspect data: %v", rawErr)
	}

	tmplMissingKey := tmpl.Option("missingkey=error")
	if rawErr := tmplMissingKey.Execute(b, raw); rawErr != nil {
		return fmt.Errorf("Template parsing error: %v", rawErr)
	}

	return nil
}

// parseTemplate wraps github.com/docker/cli/templates.Parse() to allow `json` as an alias of `{{json .}}`.
// parseTemplate can be removed when https://github.com/docker/cli/pull/3355 gets merged and tagged (Docker 22.XX).
func parseTemplate(format string) (*template.Template, error) {
	aliases := map[string]string{
		"json": "{{json .}}",
	}
	if alias, ok := aliases[format]; ok {
		format = alias
	}
	return templates.Parse(format)
}

// buildLabel builds the labels from command line labels and the image labels
func buildLabels(cmdLabels, imageLabels map[string]string) map[string]string {
	labels := make(map[string]string)
	for k, v := range imageLabels {
		if err := clabels.Validate(k, v); err == nil {
			labels[k] = v
		} else {
			// In case the image label is invalid, we output a warning and skip adding it to the
			// container.
			logrus.WithError(err).Warnf("unable to add image label with key %s to the container", k)
		}
	}
	// labels from the command line will override image and the initial image config labels
	for k, v := range cmdLabels {
		labels[k] = v
	}
	return labels
}

func printPort(ctx context.Context, containerName string, container containerd.Container, argPort int, argProto string) (string, error) {
	l, err := container.Labels(ctx)
	if err != nil {
		return "", err
	}
	portsJSON := l[labels.Ports]
	if portsJSON == "" {
		return "", nil
	}
	var ports []gocni.PortMapping
	if err := json.Unmarshal([]byte(portsJSON), &ports); err != nil {
		return "", err
	}
	// Loop through the ports and return the first HostPort.
	for _, p := range ports {
		if p.ContainerPort == int32(argPort) && strings.ToLower(p.Protocol) == argProto {
			return strconv.Itoa(int(p.HostPort)), nil
		}
	}
	return "", fmt.Errorf("no host port found for load balancer %q", containerName)
}

func execActionWithContainer(ctx context.Context, config *ExecContainerInput, args []string, container containerd.Container, client *containerd.Client) error {
	flagI := config.InputBuffer != nil

	pspec, err := generateExecProcessSpec(ctx, config, args, container, client)
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}
	var (
		ioCreator cio.Creator
		in        io.Reader
		stdinC    = &taskutil.StdinCloser{
			Stdin: os.Stdin,
		}
	)

	if flagI {
		in = stdinC
	}
	cioOpts := []cio.Opt{cio.WithStreams(in, os.Stdout, os.Stderr)}
	ioCreator = cio.NewCreator(cioOpts...)

	execID := "exec-" + idgen.GenerateID()
	process, err := task.Exec(ctx, execID, pspec, ioCreator)
	if err != nil {
		return err
	}
	stdinC.Closer = func() {
		process.CloseIO(ctx, containerd.WithStdinCloser)
	}
	defer process.Delete(ctx)

	statusC, err := process.Wait(ctx)
	if err != nil {
		return err
	}

	sigc := commands.ForwardAllSignals(ctx, process)
	defer commands.StopCatch(sigc)

	if err := process.Start(ctx); err != nil {
		return err
	}

	status := <-statusC
	code, _, err := status.Result()
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("exec failed with exit code %d", code)
	}
	return nil
}

func generateExecProcessSpec(ctx context.Context, config *ExecContainerInput, args []string, container containerd.Container, client *containerd.Client) (*specs.Process, error) {
	spec, err := container.Spec(ctx)
	if err != nil {
		return nil, err
	}

	pspec := spec.Process
	pspec.Args = args

	env := config.EnvironmentVars
	if err != nil {
		return nil, err
	}
	for _, e := range strutil.DedupeStrSlice(env) {
		pspec.Env = append(pspec.Env, e)
	}

	privileged := true
	if privileged {
		err = setExecCapabilities(pspec)
		if err != nil {
			return nil, err
		}
	}

	return pspec, nil
}

func setExecCapabilities(pspec *specs.Process) error {
	if pspec.Capabilities == nil {
		pspec.Capabilities = &specs.LinuxCapabilities{}
	}
	allCaps, err := cap.Current()
	if err != nil {
		return err
	}
	pspec.Capabilities.Bounding = allCaps
	pspec.Capabilities.Permitted = pspec.Capabilities.Bounding
	pspec.Capabilities.Inheritable = pspec.Capabilities.Bounding
	pspec.Capabilities.Effective = pspec.Capabilities.Bounding

	// https://github.com/moby/moby/pull/36466/files
	// > `docker exec --privileged` does not currently disable AppArmor
	// > profiles. Privileged configuration of the container is inherited
	return nil
}
