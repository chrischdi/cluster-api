/*
Copyright 2024 The Kubernetes Authors.

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

// Package paused implements paused helper functions.
package paused

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"
)

type pausedConditionSetter interface {
	v1beta2conditions.Setter
	client.Object
}

// EnsurePausedCondition sets the paused condition on the object and returns if it should be considered as paused.
func EnsurePausedCondition[T pausedConditionSetter](ctx context.Context, c client.Client, cluster *clusterv1.Cluster, obj T) (isPaused bool, conditionChanged bool, err error) {
	oldCondition := v1beta2conditions.Get(obj, clusterv1.PausedV1Beta2Condition)
	newCondition := pausedCondition(cluster, obj, clusterv1.PausedV1Beta2Condition)

	isPaused = newCondition.Status == metav1.ConditionTrue

	// Return early if the paused condition did not change.
	if oldCondition != nil &&
		oldCondition.Type == newCondition.Type &&
		oldCondition.Status == newCondition.Status &&
		oldCondition.Reason == newCondition.Reason &&
		oldCondition.Message == newCondition.Message &&
		oldCondition.ObservedGeneration == obj.GetGeneration() {
		return isPaused, false, nil
	}

	patchHelper, err := patch.NewHelper(obj, c)
	if err != nil {
		return isPaused, false, err
	}

	log := ctrl.LoggerFrom(ctx)
	if isPaused {
		log.V(4).Info("Pausing reconciliation for this object")
	} else {
		log.V(4).Info("Unpausing reconciliation for this object")
	}

	v1beta2conditions.Set(obj, newCondition)

	if err := patchHelper.Patch(ctx, obj, patch.WithOwnedV1Beta2Conditions{Conditions: []string{
		clusterv1.PausedV1Beta2Condition,
	}}); err != nil {
		return isPaused, false, err
	}

	return isPaused, true, nil
}

// pausedCondition sets the paused condition on the object and returns if it should be considered as paused.
func pausedCondition[T pausedConditionSetter](cluster *clusterv1.Cluster, obj T, targetConditionType string) metav1.Condition {
	if (cluster != nil && cluster.Spec.Paused) || annotations.HasPaused(obj) {
		var messages []string
		if cluster != nil && cluster.Spec.Paused {
			messages = append(messages, "Cluster spec.paused is set to true")
		}
		if annotations.HasPaused(obj) {
			messages = append(messages, fmt.Sprintf("%s has the cluster.x-k8s.io/paused annotation", obj.GetObjectKind().GroupVersionKind().Kind))
		}

		return metav1.Condition{
			Type:    targetConditionType,
			Status:  metav1.ConditionTrue,
			Reason:  clusterv1.PausedV1Beta2Reason,
			Message: strings.Join(messages, ", "),
		}
	}

	return metav1.Condition{
		Type:   targetConditionType,
		Status: metav1.ConditionFalse,
		Reason: clusterv1.NotPausedV1Beta2Reason,
	}
}
