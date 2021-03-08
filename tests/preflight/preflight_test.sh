#!/usr/bin/env bash

source ../../tools/preflight/preflight.sh --source-only

take_input --storageclass csi-gce-pd --snapshotclass default-snapshot-class
assertEquals 0 $?

testKubectl() {
 
  check_kubectl
  assertEquals 0 $?

}

testKubectlAccess() {
 
  check_kubectl_access 
  assertEquals 0 $?

}

testHelmTillerVersion() {
 
  check_helm_tiller_version 
  assertEquals 0 $?

}

testK8sVersion() {
 
  check_kubernetes_version
  assertEquals 0 $?

}

testK8sRBAC() {
 
  check_kubernetes_rbac
  assertEquals 0 $?

}

testFeatureGates() {
 
  check_feature_gates
  assertEquals 0 $?

}

testStorageSnapshotClass() {

  check_storage_snapshot_class
  assertEquals 0 $?

}

testCSI() {
 
  check_csi
  assertEquals 0 $?

}

testDNSResolution() {
 
  check_dns_resolution
  assertEquals 0 $?

}

testVolumeSnapshot() {

  check_volume_snapshot
  assertEquals 0 $?

}

# Load shUnit2.
. shunit2

