#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

POD_YAML_PATH="/home/vmware/go/src/sigs.k8s.io/cluster-api/scripts/pod.yaml"
POD_NAME="$(yq '.metadata.name' < "${POD_YAML_PATH}")"

function cleanup() {
  kubectl delete pod ${POD_NAME}
  # kind delete clusters $(kind get clusters | grep -v -e mkpod| tr "\n" " ") || true
}

function prepareOnce() {
#   temp_config=/tmp/kind.yaml
#   out_dir="${OUT_DIR:-/mnt/disks/prowjob-out}"
#   node_dir="${NODE_DIR:-/mnt/disks/kind-node}"  # Any pod hostPath mounts should be under this dir to reach the true host via the kind node.
# cat <<EOF > "${temp_config}"
# kind: Cluster
# apiVersion: kind.x-k8s.io/v1alpha4
# nodes:
#   - extraMounts:
#       - containerPath: /home-go
#         hostPath: /home/vmware/go
#       - containerPath: ${out_dir}
#         hostPath: ${out_dir}
#       # host <-> node mount for hostPath volumes in Pods. (All hostPaths should be under ${node_dir} to reach the host.)
#       - containerPath: ${node_dir}
#         hostPath: ${node_dir}
# EOF
#   kind create cluster --name=mkpod "--config=${temp_config}" --wait=5m

  # make docker-build-e2e generate-e2e-templates
  # docker pull quay.io/jetstack/cert-manager-cainjector:v1.14.4
  # docker pull quay.io/jetstack/cert-manager-webhook:v1.14.4
  # docker pull registry.k8s.io/conformance:v1.29.2
  # docker pull quay.io/jetstack/cert-manager-controller:v1.14.4
  # docker pull kindest/node:v1.29.2
  # docker pull kindest/haproxy:v20230510-486859a6

  # docker save -o scripts/images.tar \
  #   quay.io/jetstack/cert-manager-cainjector:v1.14.4 \
  #   quay.io/jetstack/cert-manager-webhook:v1.14.4 \
  #   registry.k8s.io/conformance:v1.29.2 \
  #   quay.io/jetstack/cert-manager-controller:v1.14.4 \
  #   kindest/node:v1.29.2 \
  #   kindest/haproxy:v20230510-486859a6 \
  #   gcr.io/k8s-staging-cluster-api/cluster-api-controller-amd64:dev \
  #   gcr.io/k8s-staging-cluster-api/capd-manager-amd64:dev \
  #   gcr.io/k8s-staging-cluster-api/test-extension-amd64:dev \
  #   gcr.io/k8s-staging-cluster-api/capim-manager-amd64:dev \
  #   gcr.io/k8s-staging-cluster-api/kubeadm-control-plane-controller-amd64:dev \
  #   gcr.io/k8s-staging-cluster-api/kubeadm-bootstrap-controller-amd64:dev
  # mkdir -p _artifacts
  mkdir -p out
}

function run() {
  kubectl apply -f ${POD_YAML_PATH}
  sleep 10

  # wait for pod started
  printf "$(date '+%Y-%m-%d %H:%M:%S') Waiting for pod to start\n"
  kubectl wait --for=condition=ready pod ${POD_NAME} --timeout=-1s
  # wait for pod completed
  printf "$(date '+%Y-%m-%d %H:%M:%S') Waiting for pod to finish\n"
  kubectl wait --for=condition=ready=False pod ${POD_NAME} --timeout=40m

  cleanup
}

INC=0

prepareOnce
while [ true ]; do
  INC=$((INC+1))

  printf "$(date '+%Y-%m-%d %H:%M:%S') ## Starting iteration number %3d\n" "${INC}" | tee -a out/pj-status.txt

  run
done | tee out/pj-test.log

