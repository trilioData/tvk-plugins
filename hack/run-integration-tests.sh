#!/usr/bin/env bash
set -o errexit
set -o pipefail
set -x

export INGRESS_HOST="k8s-tvk.com"

COMPONENTS=("$@")

cleanup() {
  local rc=$?
  # save_junit_artifacts

  # cleanup namespaces and helm release
  #shellcheck disable=SC2143
  if [[ $(helm list -n "${INSTALL_NAMESPACE}" | grep "${INSTALL_NAMESPACE}") ]]; then
    helm delete "${HELM_RELEASE_NAME}" --namespace "${INSTALL_NAMESPACE}"
  fi
  kubectl get validatingwebhookconfigurations -A | grep "${INSTALL_NAMESPACE}" | awk '{print $1}' | xargs -r kubectl delete validatingwebhookconfigurations || true
  kubectl get mutatingwebhookconfigurations -A | grep "${INSTALL_NAMESPACE}" | awk '{print $1}' | xargs -r kubectl delete mutatingwebhookconfigurations || true

  # NOTE: need sleep for resources to be garbage collected by api-controller
  sleep 20

  kubectl delete ns "${INSTALL_NAMESPACE}" "${RESTORE_NAMESPACE}" --request-timeout 2m || true

  kubectl get po,rs,deployment,pvc,svc,sts,cm,secret,sa,role,rolebinding,job,target,backup,backupplan,policy,restore,cronjob -n "${INSTALL_NAMESPACE}" || true

  kubectl get po,rs,deployment,pvc,svc,sts,cm,secret,sa,role,rolebinding,job,target,backup,backupplan,policy,restore,cronjob -n "${RESTORE_NAMESPACE}" || true

  kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations -A | grep -E "${INSTALL_NAMESPACE}|${RESTORE_NAMESPACE}" || true

  exit ${rc}
}

prepare_namespaces() {
  # shellcheck disable=SC2154
  install_ns="pre-${build_id}"
  kubectl create namespace "${install_ns}"

  job_type="github-actions"
  # shellcheck disable=SC2154
  kubectl label namespace "${install_ns}" trilio-label="${install_ns}" job-name="${job_name}" job-type=${job_type}

  restore_ns="${install_ns}-res"
  kubectl create namespace "${restore_ns}"
  kubectl label namespace "${restore_ns}" job-name="${JOB_NAME}" job-type=${job_type}

  helm_release_name="triliovault-${install_ns}"

  export INSTALL_NAMESPACE="${install_ns}"
  export BACKUP_NAMESPACE="${install_ns}"
  export RESTORE_NAMESPACE="${restore_ns}"
  export RESTORE_NAMESPACES_ALLOWED=("${BACKUP_NAMESPACE}" "${RESTORE_NAMESPACE}")
  export HELM_RELEASE_NAME="${helm_release_name}"
}

helm_install() {

  install_namespace=${INSTALL_NAMESPACE}
  restore_namespaces=("${RESTORE_NAMESPACES_ALLOWED[@]}")

  echo "Installing application in namespace - ${install_namespace}, with restore namespaces - ${restore_namespaces[*]}"

  # need comma separated list for restore namespaces
  restore_ns_comma=$(echo "${restore_namespaces[@]}" | tr ' ' ',')

  common_args="applicationScope=Namespaced"
  ARGS="imagePullPolicy=Always,restoreNamespaces={${restore_ns_comma[*]}},${common_args}"

  helm repo add k8s-triliovault-dev http://charts.k8strilio.net/trilio-dev/k8s-triliovault

  helm install --debug "${HELM_RELEASE_NAME}" --namespace "${install_namespace}" --set "${ARGS}" k8s-triliovault-dev/k8s-triliovault --wait --timeout=10m

  # add ingress public IP and HostName to /etc/hosts file for all the integration tests
  node_name=$(kubectl get po -o wide -n "${install_namespace}" | grep backend | awk '{print $7}')

  node_ip=$(kubectl get no "$node_name" -o=jsonpath='{.status.addresses[?(@.type=="ExternalIP")].address}')

  if [[ -n "${node_ip}" ]]; then
    sudo -- sh -c "echo \"$node_ip  ${INGRESS_HOST}\" >>/etc/hosts"
  fi
}

run_tests() {
  components=("$@")
  GO111MODULE=off go get -u github.com/onsi/ginkgo/ginkgo
  ginkgo -r -keepGoing "${components[@]}"
}

trap "cleanup" EXIT

prepare_namespaces
helm_install

run_tests "${COMPONENTS[@]}"
