/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/internal/webhooks/util"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestKubeadmConfigTemplateDefault(t *testing.T) {
	g := NewWithT(t)

	ctx := admission.NewContextWithRequest(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: pointer.Bool(true)}})
	kubeadmConfigTemplate := &bootstrapv1.KubeadmConfigTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
	}
	updateDefaultingKubeadmConfigTemplate := kubeadmConfigTemplate.DeepCopy()
	updateDefaultingKubeadmConfigTemplate.Spec.Template.Spec.Verbosity = pointer.Int32Ptr(4)
	webhook := &bootstrapv1.KubeadmConfigTemplateWebhook{}
	t.Run("for KubeadmConfigTemplate", util.CustomDefaultValidateTest(ctx, updateDefaultingKubeadmConfigTemplate, webhook))

	g.Expect(webhook.Default(ctx, kubeadmConfigTemplate)).To(Succeed())

	g.Expect(kubeadmConfigTemplate.Spec.Template.Spec.Format).To(Equal(bootstrapv1.CloudConfig))
}

func TestKubeadmConfigTemplateValidation(t *testing.T) {
	cases := map[string]struct {
		new             *bootstrapv1.KubeadmConfigTemplate
		old             *bootstrapv1.KubeadmConfigTemplate
		expectUpdateErr bool
	}{
		"valid configuration": {
			new: &bootstrapv1.KubeadmConfigTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: bootstrapv1.KubeadmConfigTemplateSpec{
					Template: bootstrapv1.KubeadmConfigTemplateResource{
						Spec: bootstrapv1.KubeadmConfigSpec{
							PreKubeadmCommands: []string{"foo"},
						},
					},
				},
			},
			old: &bootstrapv1.KubeadmConfigTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: bootstrapv1.KubeadmConfigTemplateSpec{
					Template: bootstrapv1.KubeadmConfigTemplateResource{
						Spec: bootstrapv1.KubeadmConfigSpec{
							PreKubeadmCommands: []string{"foo"},
						},
					},
				},
			},
			expectUpdateErr: false,
		},
		"immutable spec.template.spec": {
			new: &bootstrapv1.KubeadmConfigTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: bootstrapv1.KubeadmConfigTemplateSpec{
					Template: bootstrapv1.KubeadmConfigTemplateResource{
						Spec: bootstrapv1.KubeadmConfigSpec{
							PreKubeadmCommands: []string{"foo"},
						},
					},
				},
			},
			old:             &bootstrapv1.KubeadmConfigTemplate{},
			expectUpdateErr: true,
		},
	}

	for name, tt := range cases {
		tt := tt

		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := admission.NewContextWithRequest(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: pointer.Bool(true)}})
			webhook := &bootstrapv1.KubeadmConfigTemplateWebhook{}
			g.Expect(webhook.ValidateCreate(ctx, tt.new)).To(Succeed())
			if tt.expectUpdateErr {
				g.Expect(webhook.ValidateUpdate(ctx, tt.old, tt.new)).NotTo(Succeed())
			} else {
				g.Expect(webhook.ValidateUpdate(ctx, tt.old, tt.new)).To(Succeed())
			}
		})
	}
}
