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
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/util/topology"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const kubeadmConfigTemplateImmutableMsg = "KubeadmConfigTemplate spec.template.spec field is immutable. Please create a new resource instead."

func (r *KubeadmConfigTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(&KubeadmConfigTemplateWebhook{}).
		WithValidator(&KubeadmConfigTemplateWebhook{}).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/mutate-bootstrap-cluster-x-k8s-io-v1beta1-kubeadmconfigtemplate,mutating=true,failurePolicy=fail,groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigtemplates,versions=v1beta1,name=default.kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// KubeadmConfigTemplateWebhook implements a custom validation webhook for KubeadmConfigTemplate.
// +kubebuilder:object:generate=false
type KubeadmConfigTemplateWebhook struct{}

var _ webhook.CustomDefaulter = &KubeadmConfigTemplateWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (KubeadmConfigTemplateWebhook) Default(_ context.Context, obj runtime.Object) error {
	in, ok := obj.(*KubeadmConfigTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a KubeadmConfigTemplate but got a %T", obj))
	}
	DefaultKubeadmConfigSpec(&in.Spec.Template.Spec)
	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-bootstrap-cluster-x-k8s-io-v1beta1-kubeadmconfigtemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigtemplates,versions=v1beta1,name=validation.kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.CustomValidator = &KubeadmConfigTemplateWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*KubeadmConfigTemplateWebhook) ValidateCreate(_ context.Context, newRaw runtime.Object) error {
	newObj, ok := newRaw.(*KubeadmConfigTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a KubeadmConfigTemplate but got a %T", newRaw))
	}

	var allErrs field.ErrorList
	allErrs = append(allErrs, newObj.Spec.validate(newObj.Name)...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("KubeadmConfigTemplate").GroupKind(), newObj.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*KubeadmConfigTemplateWebhook) ValidateUpdate(ctx context.Context, oldRaw runtime.Object, newRaw runtime.Object) error {
	newObj, ok := newRaw.(*KubeadmConfigTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a KubeadmConfigTemplate but got a %T", newRaw))
	}
	oldObj, ok := oldRaw.(*KubeadmConfigTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a KubeadmConfigTemplate but got a %T", oldRaw))
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a admission.Request inside context: %v", err))
	}

	var allErrs field.ErrorList
	allErrs = append(allErrs, newObj.Spec.validate(newObj.Name)...)

	if !topology.ShouldSkipImmutabilityChecks(req, newObj) &&
		!reflect.DeepEqual(newObj.Spec.Template.Spec, oldObj.Spec.Template.Spec) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "template", "spec"), newObj, kubeadmConfigTemplateImmutableMsg))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("KubeadmConfigTemplate").GroupKind(), newObj.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*KubeadmConfigTemplateWebhook) ValidateDelete(_ context.Context, _ runtime.Object) error {
	return nil
}

func (r *KubeadmConfigTemplateSpec) validate(name string) field.ErrorList {
	var allErrs field.ErrorList

	return append(allErrs, r.Template.Spec.Validate(field.NewPath("spec", "template", "spec"))...)
}
