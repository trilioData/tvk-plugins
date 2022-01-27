#!/usr/bin/env bash
set -o errexit
set -o pipefail
set -x

COMPONENTS=("$@")

run_tests() {
  components=("$@")

  GO111MODULE=off go get -u github.com/onsi/ginkgo/ginkgo
  ginkgo -r -keepGoing "${components[@]}"
}

run_tests "${COMPONENTS[@]}"
