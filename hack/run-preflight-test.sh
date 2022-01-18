#!/usr/bin/env bash
set -o errexit
set -o pipefail
set -x

COMPONENTS=("$@")

export STORAGE_CLASS="csi-gce-pd"
export APP_SCOPE="Namespaced"
export JOB_TYPE="github-actions"
export UPDATE_INGRESS="true"

cleanup() {
  local rc=$?

  # cleanup namespaces and helm release
  #shellcheck disable=SC2143
  if [[ $(helm list -n "${INSTALL_NAMESPACE}" | grep "${INSTALL_NAMESPACE}") ]]; then
    helm delete "${HELM_RELEASE_NAME}" --namespace "${INSTALL_NAMESPACE}"
  fi

  kubectl get validatingwebhookconfigurations -A | grep "${INSTALL_NAMESPACE}" | awk '{print $1}' | xargs -r kubectl delete validatingwebhookconfigurations || true
  kubectl get mutatingwebhookconfigurations -A | grep "${INSTALL_NAMESPACE}" | awk '{print $1}' | xargs -r kubectl delete mutatingwebhookconfigurations || true

  # NOTE: need sleep for resources to be garbage collected by api-controller
  sleep 20

  kubectl delete ns "${INSTALL_NAMESPACE}" --request-timeout 2m || true

  kubectl get po,rs,deployment,pvc,svc,sts,cm,secret,sa,role,rolebinding,job,target,backup,backupplan,policy,restore,cronjob -n "${INSTALL_NAMESPACE}" || true

  kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations -A | grep -E "${INSTALL_NAMESPACE}" || true

  exit ${rc}
}

run_tests() {
  components=("$@")

  GO111MODULE=off go get -u github.com/onsi/ginkgo/ginkgo
  ginkgo -r -keepGoing "${components[@]}"
}

trap "cleanup" EXIT

run_tests "${COMPONENTS[@]}"
