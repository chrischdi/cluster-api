//go:build e2e
// +build e2e

/*
Copyright 2020 The Kubernetes Authors.

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

package e2e

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	"k8s.io/utils/ptr"
)

var _ = Describe("When testing the machinery for scale testing using in-memory provider", func() {
	// Note: This test does not support MachinePools.
	ScaleSpec(ctx, func() ScaleSpecInput {
		return ScaleSpecInput{
			E2EConfig:                       e2eConfig,
			ClusterctlConfigPath:            clusterctlConfigPath,
			InfrastructureProvider:          ptr.To("in-memory"),
			BootstrapClusterProxy:           bootstrapClusterProxy,
			ArtifactFolder:                  artifactFolder,
			ClusterCount:                    ptr.To[int64](mustParseInt64(e2eConfig.GetVariable("E2E_SCALE_CLUSTER_COUNT"))),
			Concurrency:                     ptr.To[int64](mustParseInt64(e2eConfig.GetVariable("E2E_SCALE_CONCURRENCY"))),
			Flavor:                          ptr.To(""),
			ControlPlaneMachineCount:        ptr.To[int64](mustParseInt64(e2eConfig.GetVariable("E2E_SCALE_CONTROL_PLANE_MACHINE_COUNT"))),
			MachineDeploymentCount:          ptr.To[int64](mustParseInt64(e2eConfig.GetVariable("E2E_SCALE_MACHINE_DEPLOYMENT_COUNT"))),
			WorkerPerMachineDeploymentCount: ptr.To[int64](mustParseInt64(e2eConfig.GetVariable("E2E_SCALE_WORKER_PER_MACHINE_DEPLOYMENT_COUNT"))),
			// FIXME: cleanup below
			// CAPI tests with v1.5:
			// 2000 Clusters with 1CP,   0MD,  0 workers each, concurrency 50
			//  200 Clusters with 3CP,   1MD, 10 workers each, concurrency 20
			//   40 Clusters with 3CP,   1MD, 50 workers each, concurrency 5
			//    1 Clusters with 3CP, 500MD,  2 workers each, concurrency 1
			// Now:
			//  500 Clusters with 3CP,   3MD,  1 workers each, concurrency 50
			//SkipWaitForCreation: true,
			SkipUpgrade:                       true,
			SkipCleanup:                       true,
			DeployClusterInSeparateNamespaces: mustParseBool(e2eConfig.GetVariable("E2E_SCALE_DEPLOY_CLUSTER_IN_SEPARATE_NAMESPACES")),
			UseCrossNamespaceClusterClass:     mustParseBool(e2eConfig.GetVariable("E2E_SCALE_USE_CROSS_NAMESPACE_CLUSTER_CLASS")),
			AdditionalClusterClasses:          mustParseInt(e2eConfig.GetVariable("E2E_SCALE_ADDITIONAL_CLUSTER_CLASS_COUNT")),
			// The runtime extension gets deployed to the test-extension-system namespace and is exposed
			// by the test-extension-webhook-service.
			// The below values are used when creating the cluster-wide ExtensionConfig to refer
			// the actual service.
			ExtensionServiceNamespace: "test-extension-system",
			ExtensionServiceName:      "test-extension-webhook-service",
		}
	})
})

func mustParseInt64(variable string) int64 {
	i, err := strconv.ParseInt(variable, 10, 64)
	if err != nil {
		panic(err)
	}
	return i
}

func mustParseInt(variable string) int {
	i, err := strconv.Atoi(variable)
	if err != nil {
		panic(err)
	}
	return i
}

func mustParseBool(variable string) bool {
	i, err := strconv.ParseBool(variable)
	if err != nil {
		panic(err)
	}
	return i
}
