#!/usr/bin/env bash

set -o errexit
set -eo pipefail

# shellcheck source=/dev/null
source tools/preflight/preflight.sh --source-only

catch() {
  if [ "$1" != "0" ]; then
    echo "Error - return code $1 occurred on line no. $2"
  fi
  cleanup
}

trap 'catch $? $LINENO' EXIT

take_input --storageclass csi-gce-pd --snapshotclass default-snapshot-class

testKubectl() {
  check_kubectl
  # shellcheck disable=SC2181
  if [ "$?" != "0" ]; then
    # shellcheck disable=SC2082
    echo "Error - checking kubectl, Expected 0 got ${$?}"
  fi
}

testKubectlAccess() {

  check_kubectl_access
  # shellcheck disable=SC2181
  if [ "$?" != "0" ]; then
    # shellcheck disable=SC2082
    echo "Error - checking kubectl access, Expected 0 got ${$?}"
  fi
}

testHelmVersion() {

  check_helm_version
  # shellcheck disable=SC2181
  if [ "$?" != "0" ]; then
    # shellcheck disable=SC2082
    echo "Error - checking helm tiller version, Expected 0 got ${$?}"
  fi
}

testK8sVersion() {

  check_kubernetes_version
  # shellcheck disable=SC2181
  if [ "$?" != "0" ]; then
    # shellcheck disable=SC2082
    echo "Error - checking kubernetes version, Expected 0 got ${$?}"
  fi
}

testK8sRBAC() {

  check_kubernetes_rbac
  # shellcheck disable=SC2181
  if [ "$?" != "0" ]; then
    # shellcheck disable=SC2082
    echo "Error - checking kubernetes RBAC, Expected 0 got ${$?}"
  fi
}

testStorageSnapshotClass() {

  check_storage_snapshot_class
  # shellcheck disable=SC2181
  if [ "$?" != "0" ]; then
    # shellcheck disable=SC2082
    echo "Error - checking storage snapshot class, Expected 0 got ${$?}"
  fi
}

testCSI() {

  check_csi
  # shellcheck disable=SC2181
  if [ "$?" != "0" ]; then
    # shellcheck disable=SC2082
    echo "Error - checking CSI Expected 0 got ${$?}"
  fi
}

testDNSResolution() {

  check_dns_resolution
  # shellcheck disable=SC2181
  if [ "$?" != "0" ]; then
    # shellcheck disable=SC2082
    echo "Error - checking DNS resolution, Expected 0 got ${$?}"
  fi
}

testVolumeSnapshot() {

  check_volume_snapshot
  # shellcheck disable=SC2181
  if [ "$?" != "0" ]; then
    # shellcheck disable=SC2082
    echo "Error - checking volume snapshot, Expected 0 got ${$?}"
  fi
}

testKubectl
testKubectlAccess
testHelmVersion
testK8sVersion
testK8sRBAC
testStorageSnapshotClass
testCSI
testDNSResolution
testVolumeSnapshot