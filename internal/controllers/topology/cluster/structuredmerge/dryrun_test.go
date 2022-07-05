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

package structuredmerge

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Test_cleanupTopologyDryRunAnnotation(t *testing.T) {
	rawManagedFieldWithAnnotation := `{"f:metadata":{"f:annotations":{"f:topology.cluster.x-k8s.io/dry-run":{}}}}`
	rawManagedFieldWithAnnotationSpecLabels := `{"f:metadata":{"f:annotations":{"f:topology.cluster.x-k8s.io/dry-run":{}},"f:labels":{}},"f:spec":{"f:foo":{}}}`
	rawManagedFieldWithSpecLabels := `{"f:metadata":{"f:labels":{}},"f:spec":{"f:foo":{}}}`

	tests := []struct {
		name    string
		obj     *unstructured.Unstructured
		wantErr bool
		want    *unstructured.Unstructured
	}{
		{
			name:    "no-op",
			obj:     newCleanupTopologyDryRunBuilder().Build(),
			wantErr: false,
		},
		{
			name: "filter out annotation",
			obj: newCleanupTopologyDryRunBuilder().
				WithAnnotation(clusterv1.TopologyDryRunAnnotation, "").
				Build(),
			wantErr: false,
			want: newCleanupTopologyDryRunBuilder().
				Build(),
		},
		{
			name: "managedFields: manager does not match",
			obj: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry("other", "", metav1.ManagedFieldsOperationApply, []byte(`{}`), nil).
				Build(),
			wantErr: false,
			want: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry("other", "", metav1.ManagedFieldsOperationApply, []byte(`{}`), nil).
				Build(),
		},
		{
			name: "managedFields: subresource does not match",
			obj: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry(TopologyManagerName, "status", metav1.ManagedFieldsOperationApply, []byte(`{}`), nil).
				Build(),
			wantErr: false,
			want: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry(TopologyManagerName, "status", metav1.ManagedFieldsOperationApply, []byte(`{}`), nil).
				Build(),
		},
		{
			name: "managedFields: operation does not match",
			obj: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry(TopologyManagerName, "", metav1.ManagedFieldsOperationUpdate, []byte(`{}`), nil).
				Build(),
			wantErr: false,
			want: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry(TopologyManagerName, "", metav1.ManagedFieldsOperationUpdate, []byte(`{}`), nil).
				Build(),
		},
		{
			name: "managedFields: cleanup up the managed field entry",
			obj: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry(TopologyManagerName, "", metav1.ManagedFieldsOperationApply, []byte(rawManagedFieldWithAnnotation), nil).
				Build(),
			wantErr: false,
			want: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry(TopologyManagerName, "", metav1.ManagedFieldsOperationApply, []byte(`{}`), nil).
				Build(),
		},
		{
			name: "managedFields: cleanup the managed field entry but preserve other ownership",
			obj: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry(TopologyManagerName, "", metav1.ManagedFieldsOperationApply, []byte(rawManagedFieldWithAnnotationSpecLabels), nil).
				Build(),
			wantErr: false,
			want: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry(TopologyManagerName, "", metav1.ManagedFieldsOperationApply, []byte(rawManagedFieldWithSpecLabels), nil).
				Build(),
		},
		{
			name: "managedFields: remove time",
			obj: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry(TopologyManagerName, "", metav1.ManagedFieldsOperationApply, []byte(`{}`), &metav1.Time{Time: time.Time{}}).
				Build(),
			wantErr: false,
			want: newCleanupTopologyDryRunBuilder().
				WithManagedFieldsEntry(TopologyManagerName, "", metav1.ManagedFieldsOperationApply, []byte(`{}`), nil).
				Build(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			if err := cleanupTopologyDryRunAnnotation(tt.obj); (err != nil) != tt.wantErr {
				t.Errorf("cleanupTopologyDryRunAnnotation() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.want != nil {
				g.Expect(tt.obj).To(BeEquivalentTo(tt.want))
			}
		})
	}
}

type cleanupTopologyDryRunBuilder struct {
	u *unstructured.Unstructured
}

func newCleanupTopologyDryRunBuilder() cleanupTopologyDryRunBuilder {
	return cleanupTopologyDryRunBuilder{&unstructured.Unstructured{Object: map[string]interface{}{}}}
}

func (b cleanupTopologyDryRunBuilder) DeepCopy() cleanupTopologyDryRunBuilder {
	return cleanupTopologyDryRunBuilder{b.u.DeepCopy()}
}

func (b cleanupTopologyDryRunBuilder) Build() *unstructured.Unstructured {
	return b.u.DeepCopy()
}

func (b cleanupTopologyDryRunBuilder) WithAnnotation(k, v string) cleanupTopologyDryRunBuilder {
	annotations := b.u.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[k] = v
	b.u.SetAnnotations(annotations)
	return b
}

func (b cleanupTopologyDryRunBuilder) WithManagedFieldsEntry(manager, subresource string, operation metav1.ManagedFieldsOperationType, fieldsV1 []byte, time *metav1.Time) cleanupTopologyDryRunBuilder {
	managedFields := append(b.u.GetManagedFields(), metav1.ManagedFieldsEntry{
		Manager:     manager,
		Operation:   operation,
		Subresource: subresource,
		FieldsV1:    &metav1.FieldsV1{Raw: fieldsV1},
		Time:        time,
	})
	b.u.SetManagedFields(managedFields)
	return b
}
