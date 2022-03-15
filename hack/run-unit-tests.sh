#!/usr/bin/env bash
set -o errexit
set -o pipefail
set -x

COMPONENTS=("$@")

export KUBEBUILDER_VERSION="2.3.2"

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

# run test suite
run_tests "${COMPONENTS[@]}"
