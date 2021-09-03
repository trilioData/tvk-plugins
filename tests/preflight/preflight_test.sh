#!/usr/bin/env bash

set -o pipefail

PREFLIGHT_TESTS_SUCCESS=true

# shellcheck source=/dev/null
source tools/preflight/preflight.sh --source-only

# change permission of kubeconfig file to suppress it's warning
sudo chmod 600 "${KUBECONFIG}"

trap 'cleanup' EXIT

take_input --storageclass csi-gce-pd --snapshotclass default-snapshot-class

testKubectl() {
  check_kubectl
  rc=$?
  # shellcheck disable=SC2181
  if [ $rc != "0" ]; then
    # shellcheck disable=SC2082
    echo "Failed - test check_kubectl, Expected 0 got $rc"
  fi
  return $rc
}

testKubectlAccess() {

  check_kubectl_access
  rc=$?
  # shellcheck disable=SC2181
  if [ $rc != "0" ]; then
    # shellcheck disable=SC2082
    echo "Failed - test check_kubectl_access, Expected 0 got $rc"
  fi
  return $rc
}

testHelmVersion() {

  check_helm_version
  rc=$?
  # shellcheck disable=SC2181
  if [ $rc != "0" ]; then
    # shellcheck disable=SC2082
    echo "Failed - test check_helm_version, Expected 0 got $rc"
  fi
  return $rc
}

testK8sVersion() {

  check_kubernetes_version
  rc=$?
  # shellcheck disable=SC2181
  if [ $rc != "0" ]; then
    # shellcheck disable=SC2082
    echo "Failed - test check_kubernetes_version, Expected 0 got $rc"
  fi
  return $rc
}

testK8sRBAC() {

  check_kubernetes_rbac
  rc=$?
  # shellcheck disable=SC2181
  if [ $rc != "0" ]; then
    # shellcheck disable=SC2082
    echo "Failed - test check_kubernetes_rbac, Expected 0 got ${rc}"
  fi
  return $rc
}

testStorageSnapshotClass() {

  check_storage_snapshot_class
  rc=$?
  # shellcheck disable=SC2181
  if [ $rc != "0" ]; then
    # shellcheck disable=SC2082
    echo "Failed - test check_storage_snapshot_class, Expected 0 got $rc"
  fi
  return $rc
}

testCSI() {

  check_csi
  rc=$?
  # shellcheck disable=SC2181
  if [ $rc != "0" ]; then
    # shellcheck disable=SC2082
    echo "Failed - test check_csi, Expected 0 got $rc"
  fi
  return $rc
}

testDNSResolution() {

  check_dns_resolution
  rc=$?
  # shellcheck disable=SC2181
  if [ $rc != "0" ]; then
    # shellcheck disable=SC2082
    echo "Failed - test check_dns_resolution, Expected 0 got $rc"
  fi
  return $rc
}

testVolumeSnapshot() {

  check_volume_snapshot
  rc=$?
  # shellcheck disable=SC2181
  if [ $rc != "0" ]; then
    # shellcheck disable=SC2082
    echo "Failed - test testVolumeSnapshot, Expected 0 got $rc"
  fi
  return $rc
}

testKubectl
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_TESTS_SUCCESS=false
fi

testKubectlAccess
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_TESTS_SUCCESS=false
fi

testHelmVersion
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_TESTS_SUCCESS=false
fi

testK8sVersion
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_TESTS_SUCCESS=false
fi

testK8sRBAC
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_TESTS_SUCCESS=false
fi

testStorageSnapshotClass
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_TESTS_SUCCESS=false
fi

testCSI
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_TESTS_SUCCESS=false
fi

testDNSResolution
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_TESTS_SUCCESS=false
fi

testVolumeSnapshot
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_TESTS_SUCCESS=false
fi

# Check status of Pre-flight test-cases
if [ $PREFLIGHT_TESTS_SUCCESS == "true" ]; then
  echo -e "All Pre-flight tests Passed!"
else
  echo -e "Some Pre-flight Checks Failed!"
  exit 1
fi
