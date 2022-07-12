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

package v1beta1

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/internal/webhooks/util"
)

func TestKubeadmConfigDefault(t *testing.T) {
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.ClusterTopology, true)()

	g := NewWithT(t)
	ctx := admission.NewContextWithRequest(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: pointer.Bool(true)}})

	kubeadmConfig := &KubeadmConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: KubeadmConfigSpec{},
	}
	updateDefaultingKubeadmConfig := kubeadmConfig.DeepCopy()
	updateDefaultingKubeadmConfig.Spec.Verbosity = pointer.Int32Ptr(4)
	webhook := &KubeadmConfigWebhook{}
	t.Run("for KubeadmConfig", util.CustomDefaultValidateTest(ctx, updateDefaultingKubeadmConfig, webhook))

	g.Expect(webhook.Default(ctx, kubeadmConfig)).To(Succeed())

	g.Expect(kubeadmConfig.Spec.Format).To(Equal(CloudConfig))

	ignitionKubeadmConfig := &KubeadmConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: KubeadmConfigSpec{
			Format: Ignition,
		},
	}
	g.Expect(webhook.Default(ctx, ignitionKubeadmConfig)).To(Succeed())
	g.Expect(ignitionKubeadmConfig.Spec.Format).To(Equal(Ignition))
}

func TestKubeadmConfigValidate(t *testing.T) {
	cases := map[string]struct {
		new                   *KubeadmConfig
		old                   *KubeadmConfig
		enableIgnitionFeature bool
		expectCreateErr       bool
		expectUpdateErr       bool
	}{
		"valid content": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Files: []File{
						{
							Content: "foo",
						},
					},
				},
			},
		},
		"valid contentFrom": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Files: []File{
						{
							ContentFrom: &FileSource{
								Secret: SecretFileSource{
									Name: "foo",
									Key:  "bar",
								},
							},
						},
					},
				},
			},
		},
		"invalid content and contentFrom": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Files: []File{
						{
							ContentFrom: &FileSource{},
							Content:     "foo",
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"invalid contentFrom without name": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Files: []File{
						{
							ContentFrom: &FileSource{
								Secret: SecretFileSource{
									Key: "bar",
								},
							},
							Content: "foo",
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"invalid contentFrom without key": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Files: []File{
						{
							ContentFrom: &FileSource{
								Secret: SecretFileSource{
									Name: "foo",
								},
							},
							Content: "foo",
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"invalid with duplicate file path": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Files: []File{
						{
							Content: "foo",
						},
						{
							Content: "bar",
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"valid passwd": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Users: []User{
						{
							Passwd: pointer.StringPtr("foo"),
						},
					},
				},
			},
		},
		"valid passwdFrom": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Users: []User{
						{
							PasswdFrom: &PasswdSource{
								Secret: SecretPasswdSource{
									Name: "foo",
									Key:  "bar",
								},
							},
						},
					},
				},
			},
		},
		"invalid passwd and passwdFrom": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Users: []User{
						{
							PasswdFrom: &PasswdSource{},
							Passwd:     pointer.StringPtr("foo"),
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"invalid passwdFrom without name": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Users: []User{
						{
							PasswdFrom: &PasswdSource{
								Secret: SecretPasswdSource{
									Key: "bar",
								},
							},
							Passwd: pointer.StringPtr("foo"),
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"invalid passwdFrom without key": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Users: []User{
						{
							PasswdFrom: &PasswdSource{
								Secret: SecretPasswdSource{
									Name: "foo",
								},
							},
							Passwd: pointer.StringPtr("foo"),
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"Ignition field is set, format is not Ignition": {
			enableIgnitionFeature: true,
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Ignition: &IgnitionSpec{},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"Ignition field is not set, format is Ignition": {
			enableIgnitionFeature: true,
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Format: Ignition,
				},
			},
		},
		"format is Ignition, user is inactive": {
			enableIgnitionFeature: true,
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Format: Ignition,
					Users: []User{
						{
							Inactive: pointer.BoolPtr(true),
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"format is Ignition, non-GPT partition configured": {
			enableIgnitionFeature: true,
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Format: Ignition,
					DiskSetup: &DiskSetup{
						Partitions: []Partition{
							{
								TableType: pointer.StringPtr("MS-DOS"),
							},
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"format is Ignition, experimental retry join is set": {
			enableIgnitionFeature: true,
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Format:                   Ignition,
					UseExperimentalRetryJoin: true,
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"feature gate disabled, format is Ignition": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Format: Ignition,
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"feature gate disabled, Ignition field is set": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Format: Ignition,
					Ignition: &IgnitionSpec{
						ContainerLinuxConfig: &ContainerLinuxConfig{},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"replaceFS specified with Ignition": {
			enableIgnitionFeature: true,
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Format: Ignition,
					DiskSetup: &DiskSetup{
						Filesystems: []Filesystem{
							{
								ReplaceFS: pointer.StringPtr("ntfs"),
							},
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"filesystem partition specified with Ignition": {
			enableIgnitionFeature: true,
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Format: Ignition,
					DiskSetup: &DiskSetup{
						Filesystems: []Filesystem{
							{
								Partition: pointer.StringPtr("1"),
							},
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"file encoding gzip specified with Ignition": {
			enableIgnitionFeature: true,
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Format: Ignition,
					Files: []File{
						{
							Encoding: Gzip,
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"file encoding gzip+base64 specified with Ignition": {
			enableIgnitionFeature: true,
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: "default",
				},
				Spec: KubeadmConfigSpec{
					Format: Ignition,
					Files: []File{
						{
							Encoding: GzipBase64,
						},
					},
				},
			},
			expectCreateErr: true,
			expectUpdateErr: true,
		},
		"immutable spec": {
			new: &KubeadmConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baz",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: KubeadmConfigSpec{
					Files: []File{
						{
							Content: "foo",
						},
					},
				},
			},
			old:             &KubeadmConfig{},
			expectUpdateErr: true,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			if tt.enableIgnitionFeature {
				// NOTE: KubeadmBootstrapFormatIgnition feature flag is disabled by default.
				// Enabling the feature flag temporarily for this test.
				defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.KubeadmBootstrapFormatIgnition, true)()
			}
			g := NewWithT(t)
			ctx := admission.NewContextWithRequest(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: pointer.Bool(true)}})
			webhook := &KubeadmConfigWebhook{}
			oldRaw := tt.old
			if oldRaw == nil {
				oldRaw = tt.new.DeepCopy()
			}
			if tt.expectCreateErr {
				g.Expect(webhook.ValidateCreate(ctx, tt.new)).NotTo(Succeed())
			} else {
				g.Expect(webhook.ValidateCreate(ctx, tt.new)).To(Succeed())
			}
			if tt.expectUpdateErr {
				g.Expect(webhook.ValidateUpdate(ctx, oldRaw, tt.new)).NotTo(Succeed())
			} else {
				g.Expect(webhook.ValidateUpdate(ctx, oldRaw, tt.new)).To(Succeed())
			}
		})
	}
}
