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

package store

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/kube-state-metrics/v2/pkg/customresource"
	"k8s.io/kube-state-metrics/v2/pkg/options"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var scheme = runtime.NewScheme()

func init() {
	_ = clusterv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

type ControllerRuntimeClientFactory struct{}

func Factories() []customresource.RegistryFactory {
	return []customresource.RegistryFactory{
		&ClusterFactory{},
		&KubeadmControlPlaneFactory{},
		&MachineDeploymentFactory{},
		&MachineSetFactory{},
		&MachineFactory{},
	}
}

func (f *ControllerRuntimeClientFactory) CreateClient(cfg *rest.Config) (interface{}, error) {
	return client.NewWithWatch(cfg, client.Options{
		Scheme: scheme,
	})
}

var DefaultResources = options.ResourceSet{}

func init() {
	if len(DefaultResources) == 0 {
		for _, factory := range Factories() {
			DefaultResources[factory.Name()] = struct{}{}
		}
	}
}
