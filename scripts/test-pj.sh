#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

POD_YAML_PATH="/home/vmware/go/src/k8s.io/test-infra/config/pod.yaml"
POD_NAME="$(yq '.metadata.name' < "${POD_YAML_PATH}")"

function cleanup() {
  kubectl delete pod ${POD_NAME}
  # kind delete clusters $(kind get clusters | grep -v -e mkpod| tr "\n" " ") || true
}

function prepareOnce() {
  # make docker-build-e2e generate-e2e-templates
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

