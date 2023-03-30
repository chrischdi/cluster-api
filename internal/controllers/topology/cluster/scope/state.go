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

package scope

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/labels/format"
)

// ClusterState holds all the objects representing the state of a managed Cluster topology.
// NOTE: please note that we are going to deal with two different type state, the current state as read from the API server,
// and the desired state resulting from processing the ClusterBlueprint.
type ClusterState struct {
	// Cluster holds the Cluster object.
	Cluster *clusterv1.Cluster

	// InfrastructureCluster holds the infrastructure cluster object referenced by the Cluster.
	InfrastructureCluster *unstructured.Unstructured

	// ControlPlane holds the controlplane object referenced by the Cluster.
	ControlPlane *ControlPlaneState

	// MachineDeployments holds the machine deployments in the Cluster.
	MachineDeployments MachineDeploymentsStateMap

	// MachinePools holds the MachinePools in the Cluster.
	MachinePools MachinePoolsStateMap
}

// ControlPlaneState holds all the objects representing the state of a managed control plane.
type ControlPlaneState struct {
	// Object holds the ControlPlane object.
	Object *unstructured.Unstructured

	// InfrastructureMachineTemplate holds the infrastructure template referenced by the ControlPlane object.
	InfrastructureMachineTemplate *unstructured.Unstructured

	// MachineHealthCheckClass holds the MachineHealthCheck for this ControlPlane.
	// +optional
	MachineHealthCheck *clusterv1.MachineHealthCheck
}

// MachineDeploymentsStateMap holds a collection of MachineDeployment states.
type MachineDeploymentsStateMap map[string]*MachineDeploymentState

// Upgrading returns the list of the machine deployments
// that are upgrading.
func (mds MachineDeploymentsStateMap) Upgrading(ctx context.Context, c client.Client) ([]string, error) {
	names := []string{}
	for _, md := range mds {
		upgrading, err := md.IsUpgrading(ctx, c)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list upgrading MachineDeployments")
		}
		if upgrading {
			names = append(names, md.Object.Name)
		}
	}
	return names, nil
}

// MachineDeploymentState holds all the objects representing the state of a managed deployment.
type MachineDeploymentState struct {
	// Object holds the MachineDeployment object.
	Object *clusterv1.MachineDeployment

	// BootstrapTemplate holds the bootstrap template referenced by the MachineDeployment object.
	BootstrapTemplate *unstructured.Unstructured

	// InfrastructureMachineTemplate holds the infrastructure machine template referenced by the MachineDeployment object.
	InfrastructureMachineTemplate *unstructured.Unstructured

	// MachineHealthCheck holds a MachineHealthCheck linked to the MachineDeployment object.
	// +optional
	MachineHealthCheck *clusterv1.MachineHealthCheck
}

// IsUpgrading determines if the MachineDeployment is upgrading.
// A machine deployment is considered upgrading if at least one of the Machines of this
// MachineDeployment has a different version.
func (md *MachineDeploymentState) IsUpgrading(ctx context.Context, c client.Client) (bool, error) {
	// If the MachineDeployment has no version there is no definitive way to check if it is upgrading. Therefore, return false.
	// Note: This case should not happen.
	if md.Object.Spec.Template.Spec.Version == nil {
		return false, nil
	}
	selectorMap, err := metav1.LabelSelectorAsMap(&md.Object.Spec.Selector)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check if MachineDeployment %s is upgrading: failed to convert label selector to map", md.Object.Name)
	}
	machines := &clusterv1.MachineList{}
	if err := c.List(ctx, machines, client.InNamespace(md.Object.Namespace), client.MatchingLabels(selectorMap)); err != nil {
		return false, errors.Wrapf(err, "failed to check if MachineDeployment %s is upgrading: failed to list Machines", md.Object.Name)
	}
	mdVersion := *md.Object.Spec.Template.Spec.Version
	// Check if the versions of the all the Machines match the MachineDeployment version.
	for i := range machines.Items {
		machine := machines.Items[i]
		if machine.Spec.Version == nil {
			return false, fmt.Errorf("failed to check if MachineDeployment %s is upgrading: Machine %s has no version", md.Object.Name, machine.Name)
		}
		if *machine.Spec.Version != mdVersion {
			return true, nil
		}
	}
	return false, nil
}

// MachinePoolsStateMap holds a collection of MachinePool states.
type MachinePoolsStateMap map[string]*MachinePoolState

// Upgrading returns the list of the machine pools
// that are upgrading.
func (mps MachinePoolsStateMap) Upgrading(ctx context.Context, c client.Client) ([]string, error) {
	names := []string{}
	for _, mp := range mps {
		upgrading, err := mp.IsUpgrading(ctx, c)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list upgrading MachinePools")
		}
		if upgrading {
			names = append(names, mp.Object.Name)
		}
	}
	return names, nil
}

// MachinePoolState holds all the objects representing the state of a managed pool.
type MachinePoolState struct {
	// Object holds the MachinePool object.
	Object *expv1.MachinePool

	// BootstrapObject holds the MachinePool bootstrap object.
	BootstrapObject *unstructured.Unstructured

	// InfrastructureMachinePoolObject holds the infrastructure machine template referenced by the MachinePool object.
	InfrastructureMachinePoolObject *unstructured.Unstructured
}

// IsUpgrading determines if the MachinePool is upgrading.
// A machine deployment is considered upgrading if at least one of the Machines of this
// MachinePool has a different version.
func (mp *MachinePoolState) IsUpgrading(ctx context.Context, c client.Client) (bool, error) {
	// If the MachinePool has no version there is no definitive way to check if it is upgrading. Therefore, return false.
	// Note: This case should not happen.
	if mp.Object.Spec.Template.Spec.Version == nil {
		return false, nil
	}
	machineSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			clusterv1.MachinePoolNameLabel: format.MustFormatValue(mp.Object.Name),
			clusterv1.ClusterNameLabel:     mp.Object.Spec.ClusterName,
		},
	}
	selectorMap, err := metav1.LabelSelectorAsMap(&machineSelector)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check if MachinePool %s is upgrading: failed to convert label selector to map", mp.Object.Name)
	}
	machines := &clusterv1.MachineList{}
	if err := c.List(ctx, machines, client.InNamespace(mp.Object.Namespace), client.MatchingLabels(selectorMap)); err != nil {
		return false, errors.Wrapf(err, "failed to check if MachinePool %s is upgrading: failed to list MachinePools", mp.Object.Name)
	}
	mpVersion := *mp.Object.Spec.Template.Spec.Version
	// Check if the versions of the all the MachinePoolMachines match the MachinePool version.
	for i := range machines.Items {
		machine := machines.Items[i]
		if machine.Spec.Version == nil {
			return false, fmt.Errorf("failed to check if MachinePool %s is upgrading: Machine %s has no version", mp.Object.Name, machine.Name)
		}
		if *machine.Spec.Version != mpVersion {
			return true, nil
		}
	}
	return false, nil
}
