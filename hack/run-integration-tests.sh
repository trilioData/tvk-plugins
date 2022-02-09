#!/usr/bin/env bash
set -o errexit
set -o pipefail
set -x

COMPONENTS=("$@")

export STORAGE_CLASS="csi-gce-pd"
export APP_SCOPE="Namespaced"
export JOB_TYPE="github-actions"
export UPDATE_INGRESS="true"

# shellcheck disable=SC2018
random_string=$(LC_ALL=C head -c 128 /dev/urandom | tr -dc 'a-z' | fold -w 6 | head -n 1)
# shellcheck disable=SC2154
install_ns="plugins-${build_id}-${random_string}"
helm_release_name="triliovault-${install_ns}"

export INGRESS_HOST="${install_ns}.k8s-tvk.com"
export INSTALL_NAMESPACE="${install_ns}"
export BACKUP_NAMESPACE="${install_ns}"
export HELM_RELEASE_NAME="${helm_release_name}"

cleanup_namespace() {
  local rc=$?
  kubectl delete ns "${INSTALL_NAMESPACE}" --request-timeout 2m || true
  exit ${rc}
}

cleanup() {
  local rc=$?

  kubectl delete target --all -n "${INSTALL_NAMESPACE}" || true

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

  exit ${rc}
}

prepare_namespaces() {
  kubectl create namespace "${INSTALL_NAMESPACE}"
  # shellcheck disable=SC2154
  kubectl label namespace "${INSTALL_NAMESPACE}" trilio-label="${INSTALL_NAMESPACE}" job-name="${job_name}" job-type=${JOB_TYPE}
}

helm_install() {
  echo "Installing TVK application in namespace - ${INSTALL_NAMESPACE}"

  common_args="applicationScope=Namespaced"
  resources_args="web-backend.resources.limits.memory=1024Mi,web-backend.livenessProbeEnable=false,web-backend.resources.requests.memory=10Mi,control-plane.resources.limits.memory=1024Mi"
  ARGS="imagePullPolicy=Always,${common_args},${resources_args}"

  DEV_REPO="http://charts.k8strilio.net/trilio-dev/k8s-triliovault"
  helm repo add k8s-triliovault-dev "${DEV_REPO}"

  helm install --debug "${HELM_RELEASE_NAME}" --namespace "${INSTALL_NAMESPACE}" --set "${ARGS}" k8s-triliovault-dev/k8s-triliovault --wait --timeout=10m

  if [[ -n "${UPDATE_INGRESS}" ]]; then
    selector=$(kubectl get svc k8s-triliovault-ingress-gateway -n "${INSTALL_NAMESPACE}" -o wide | awk '{print $NF}' | tail -n +2)
    node=$(kubectl get pods -o wide -l "$selector" -n "${INSTALL_NAMESPACE}" | awk '{print $7}' | tail -n +2)
    instance_info=$(gcloud compute instances describe "$node" --zone "${GKE_ZONE}" --format=json | jq '.| "\(.tags.items[0]) \(.networkInterfaces[].network)"')
    IFS=" " read -r -a node_port <<<"$(kubectl get svc k8s-triliovault-ingress-gateway -n "${INSTALL_NAMESPACE}" --template='{{range .spec.ports}}{{print "\n" .nodePort}}{{end}}' | tr '\n' ' ')"
    node_external_ip=$(kubectl get no "$node" -o=jsonpath='{.status.addresses[?(@.type=="ExternalIP")].address}')
    port=""
    for ((c = 0; c < ${#node_port}; c++)); do
      if [[ ${node_port[$c]} != "" ]]; then
        port+="tcp:${node_port[$c]},"
      fi
    done
    gcloud compute firewall-rules create "${JOB_TYPE}"-"${INSTALL_NAMESPACE}" --allow="$port" --source-ranges="0.0.0.0/0" --target-tags="$(echo "${instance_info}" | awk '{print $1}' | sed 's/"//g')" --network="$(echo "${instance_info}" | awk '{print $2}' | sed 's/"//g' | awk -F'/' '{print $NF}')"
    if [[ -n "${node_external_ip}" ]]; then
      sudo -- bash -c "echo \"${node_external_ip} ${INGRESS_HOST}\" >>/etc/hosts"
    else
      exit 1
    fi
  fi

}

run_tests() {
  components=("$@")

  # will be required to run test-cases
  sudo apt-get install -y nfs-common

  GO111MODULE=on go install github.com/onsi/ginkgo/ginkgo@v1.16.4
  ginkgo -r -keepGoing "${components[@]}"
}

if [[ "${job_name}" == "target-browser" ]]; then
  trap "cleanup" EXIT
else
  trap "cleanup_namespace" EXIT
fi

# change permission of kubeconfig file to suppress it's warning
sudo chmod 600 "${KUBECONFIG}" || true

# creates ns to run test suite
prepare_namespaces

# install TVK helm chart for target-browser test job
if [[ "${job_name}" == "target-browser" ]]; then
  helm_install
fi

# run test suite
run_tests "${COMPONENTS[@]}"
