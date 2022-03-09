#!/usr/bin/env bash
set -o errexit
set -o pipefail
set -x

COMPONENTS=("$@")

run_tests() {
  components=("$@")

  GO111MODULE=on go install github.com/onsi/ginkgo/ginkgo@v1.16.4
  ginkgo -r -keepGoing "${components[@]}"
}

# change permission of kubeconfig file to suppress it's warning
sudo chmod 600 "${KUBECONFIG}" || true

# run test suite
run_tests "${COMPONENTS[@]}"
