#!/usr/bin/env bash
set -o errexit
set -o pipefail
set -x

# shellcheck disable=SC2018
random_string=$(LC_ALL=C head -c 128 /dev/urandom | tr -dc 'a-z' | fold -w 6 | head -n 1)
# shellcheck disable=SC2154
install_ns="plugins-${build_id}-${random_string}"

COMPONENTS=("$@")

export KUBEBUILDER_VERSION="2.3.2"
export INSTALL_NAMESPACE="${install_ns}"
export JOB_TYPE="github-actions"

prepare_namespaces() {
  kubectl create namespace "${INSTALL_NAMESPACE}"
  # shellcheck disable=SC2154
  kubectl label namespace "${INSTALL_NAMESPACE}" trilio-label="${INSTALL_NAMESPACE}" job-name="${job_name}" job-type=${JOB_TYPE}
}

run_tests() {
  components=("$@")

  GO111MODULE=on go install github.com/onsi/ginkgo/ginkgo@v1.16.4
  ginkgo -r -keepGoing "${components[@]}"
}

# install kubebuilder for env tests
os=$(go env GOOS) &&
  arch=$(go env GOARCH) &&
  curl -sL https://github.com/kubernetes-sigs/kubebuilder/releases/download/v"${KUBEBUILDER_VERSION}"/kubebuilder_"${KUBEBUILDER_VERSION}"_"${os}"_"${arch}".tar.gz | tar -xz -C /tmp/ &&
  sudo mv /tmp/kubebuilder_"${KUBEBUILDER_VERSION}"_"${os}"_"${arch}" /usr/local/kubebuilder &&
  rm -rf /tmp/kubebuilder_"${KUBEBUILDER_VERSION}"_"${os}"_"${arch}"

# change permission of kubeconfig file to suppress it's warning
sudo chmod 600 "${KUBECONFIG}" || true

# create namespaces
prepare_namespaces

# run test suite
run_tests "${COMPONENTS[@]}"
