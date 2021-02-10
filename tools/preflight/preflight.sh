#!/usr/bin/env bash

# Purpose: Helper script for running pre-flight checks before installing K8s-Triliovault application.

# COLOUR CONSTANTS
GREEN='\033[0;32m'
GREEN_BOLD='\033[0;32m\e[1m'
LIGHT_BLUE='\033[1;34m'
RED='\033[0;31m'
RED_BOLD='\033[0;31m\e[1m'
NC='\033[0m'

CHECK='\xE2\x9C\x94'
CROSS='\xE2\x9D\x8C'

MIN_HELM_VERSION="2.11.0"
MIN_K8S_VERSION="1.13.0"

SOURCE_POD="source-pod"
SOURCE_PVC="source-pvc"
RESTORE_POD="restored-pod"
RESTORE_PVC="restored-pvc"
VOLUME_SNAP_SRC="snapshot-source-pvc"
UNUSED_RESTORE_POD="unused-restored-pod"
UNUSED_RESTORE_PVC="unused-restored-pvc"
UNUSED_VOLUME_SNAP_SRC="unused-source-pvc"

print_help() {
  echo "Usage:
kubectl tvk-preflight --storageclass <storage_class_name> --snapshotclass <volume_snapshot_class>
Params:
	--storageclass	name of storage class being used in k8s cluster
	--snapshotclass name of volume snapshot class being used in k8s cluster
	--kubeconfig	path to kube config (OPTIONAL)
"
}

take_input() {
  if [[ -z "${1}" ]]; then
    echo "Error: --storageclass and --snapshotclass flags are needed to run pre flight checks!"
    print_help
    exit 1
  fi
  while true; do
    case "$1" in
    --storageclass)
      if [[ "$2" =~ -- ]]; then
        STORAGE_CLASS=""
        shift
      else
        STORAGE_CLASS=$2
        shift 2
      fi
      ;;
    --snapshotclass)
      if [[ "$2" =~ -- ]]; then
        SNAPSHOT_CLASS=""
        shift
      else
        SNAPSHOT_CLASS=$2
        shift 2
      fi
      ;;
    --kubeconfig)
      if [[ "$2" =~ -- ]]; then
        KUBECONFIG_PATH=""
        shift
      else
        KUBECONFIG_PATH=$2
        shift 2
      fi
      ;;
    -h | --help)
      print_help
      exit
      ;;
    *)
      shift
      break
      ;;
    esac
  done
  if [[ -z "${STORAGE_CLASS}" || -z "${SNAPSHOT_CLASS}" ]]; then
    echo "Error: --storageclass and --snapshotclass, both flags are needed to run pre flight checks!"
    print_help
    exit 1
  fi
}

check_kubectl() {
  echo
  echo -e "${LIGHT_BLUE}Checking for kubectl...${NC}\n"
  local exit_status=0
  if ! command -v "kubectl" >/dev/null 2>&1; then
    echo -e "${RED} ${CROSS} Unable to find kubectl${NC}\n"
    exit_status=1
  else
    echo -e "${GREEN} ${CHECK} Found kubectl${NC}\n"
  fi
  return ${exit_status}
}

check_kubectl_access() {
  local exit_status=0
  if [[ -n ${KUBECONFIG_PATH} ]]; then
    export KUBECONFIG=${KUBECONFIG_PATH}
  fi
  echo -e "${LIGHT_BLUE}Checking access to the Kubernetes context $(kubectl config current-context)...${NC}\n"

  if [[ $(kubectl get ns default) ]]; then
    echo -e "${GREEN} ${CHECK} Able to access the default Kubernetes namespace${NC}\n"
  else
    echo -e "${RED} ${CROSS} Unable to access the default Kubernetes namespace${NC}\n"
    exit_status=1
  fi
  return ${exit_status}
}

version_gt_eq() {
  local sorted_version
  sorted_version=$(echo -e '%s\n' "$@" | sort -V | head -n 1)
  if [[ "${sorted_version}" != "$1" || "${sorted_version}" = "$2" ]]; then
    return 0
  fi
  return 1
}

check_if_ocp() {
  # Check if the k8s cluster is upstream or OCP
  local is_ocp="N"
  #shellcheck disable=SC2143
  if [[ $(kubectl api-resources | grep openshift.io) ]]; then
    is_ocp="Y"
  fi
  echo "${is_ocp}"
}

check_helm_tiller_version() {
  echo -e "${LIGHT_BLUE}Checking for required Helm Tiller version (>= v${MIN_HELM_VERSION})...${NC}\n"
  local exit_status=0

  # Abort successfully in case of OCP setup
  if [[ $(check_if_ocp) == "Y" ]]; then
    echo -e "${GREEN} ${CHECK} Helm not needed for OCP clusters${NC}\n"
    return ${exit_status}
  fi

  if ! command -v "helm" >/dev/null 2>&1; then
    echo -e "${RED} ${CROSS} Unable to find helm${NC}\n"
    exit_status=1
  else
    echo -e "${GREEN} ${CHECK} Found helm${NC}\n"
  fi

  # Abort if Helm 3
  local helm_version
  helm_version=$(helm version --template "{{ .Version }}")
  if [[ ${helm_version} != "<no value>" ]]; then
    echo -e "${GREEN} ${CHECK} No Tiller needed with Helm ${helm_version}${NC}\n"
    return ${exit_status}
  fi
  helm_version=$(helm version --template "{{ .Server.SemVer }}")
  if version_gt_eq "${helm_version:1}" "${MIN_HELM_VERSION}"; then
    echo -e "${GREEN} ${CHECK} Tiller version (${helm_version}) meets minimum requirements${NC}\n"
  else
    echo -e "${RED} ${CROSS} Tiller version (${helm_version}) does not meet minimum requirements${NC}\n"
    exit_status=1
  fi
  return ${exit_status}
}

check_kubernetes_version() {
  echo -e "${LIGHT_BLUE}Checking for required Kubernetes version (>= v${MIN_K8S_VERSION})...${NC}\n"
  local exit_status=0
  local k8s_version
  k8s_version=$(kubectl version --short | grep Server | awk '{print $3}')

  if version_gt_eq "${k8s_version:1}" "${MIN_K8S_VERSION}"; then
    echo -e "${GREEN} ${CHECK} Kubernetes version (${k8s_version}) meets minimum requirements${NC}\n"
  else
    echo -e "${RED} ${CROSS} Kubernetes version (${k8s_version}) does not meet minimum requirements${NC}\n"
    exit_status=1
  fi
  return ${exit_status}
}

check_kubernetes_rbac() {
  echo -e "${LIGHT_BLUE}Checking if Kubernetes RBAC is enabled...${NC}\n"
  local exit_status=0
  # The below shellcheck conflicts with pipefail
  # shellcheck disable=SC2143
  if [[ $(kubectl api-versions | grep rbac.authorization.k8s.io) ]]; then
    echo -e "${GREEN} ${CHECK} Kubernetes RBAC is enabled${NC}\n"
  else
    echo -e "${RED} ${CROSS} Kubernetes RBAC is not enabled${NC}\n"
    exit_status=1
  fi
  return ${exit_status}
}

check_storage_snapshot_class() {
  echo -e "${LIGHT_BLUE}Checking if a StorageClass and VolumeSnapshotClass are present...${NC}\n"
  local exit_status=0
  # shellcheck disable=SC2143
  if [[ $(kubectl get storageclass | grep -E "(^|\s)${STORAGE_CLASS}($|\s)") ]]; then
    echo -e "${GREEN} ${CHECK} Storage class \"${STORAGE_CLASS}\" found${NC}\n"
  else
    echo -e "${RED} ${CROSS} Storage class \"${STORAGE_CLASS}\" not found${NC}\n"
    exit_status=1
  fi
  # shellcheck disable=SC2143
  if [[ $(kubectl get volumesnapshotclass | grep -E "(^|\s)${SNAPSHOT_CLASS}($|\s)") ]]; then
    echo -e "${GREEN} ${CHECK} Volume snapshot class \"${SNAPSHOT_CLASS}\" found${NC}\n"
  else
    echo -e "${RED} ${CROSS} Volume snapshot class \"${SNAPSHOT_CLASS}\" not found${NC}\n"
    exit_status=1
  fi
  # shellcheck disable=SC2143
  if [[ $(kubectl get apiservices | grep "v1beta1.snapshot.storage.k8s.io") ]]; then
    echo -e "${GREEN} ${CHECK} Snapshot api is in beta. No need to have default class annotation on volume snapshot class${NC}\n"
  else
    # shellcheck disable=SC2143
    if [[ $(kubectl get volumesnapshotclass -o yaml | grep "is-default-class: \"true\"") ]]; then
      echo -e "${GREEN} ${CHECK} Snapshot api is in alpha. Found a snapshot class marked as default${NC}\n"
    else
      echo -e "${RED} ${CROSS} Snapshot api is in alpha. No snapshot class is marked default${NC}\n"
      exit_status=1
    fi
  fi
  return ${exit_status}
}

check_feature_gates() {
  local exit_status=0
  local k8s_version
  local features=()

  # specially handle this for GKE alpha clusters
  gke_all_alpha_feature="AllAlpha=true"

  k8s_version=$(kubectl version --short | grep Server | awk '{print $3}')
  echo -e "${LIGHT_BLUE}Checking if needed features are available in k8s cluster for version ${k8s_version}...${NC}\n"

  k8s_version=$(echo "${k8s_version}" | cut -d '.' -f2)
  if [[ "${k8s_version}" == "13" ]]; then
    features=("CSIBlockVolume" "CSIDriverRegistry" "CSINodeInfo" "VolumeSnapshotDataSource")
  else
    features=("VolumeSnapshotDataSource")
  fi

  if [[ "${k8s_version}" -lt "15" ]]; then
    features+=("CustomResourceWebhookConversion")
  fi

  if [[ ${k8s_version} -ge "17" ]]; then
    echo -e "${GREEN} ${CHECK} No feature gates needed${NC}\n"
    return ${exit_status}
  fi

  if [[ $(check_if_ocp) == "Y" ]]; then
    features_enabled=$(kubectl get cm -n openshift-kube-apiserver -oyaml | grep feature)
  else
    features_enabled=$(kubectl get po -n kube-system -oyaml | grep feature)
  fi

  for feat in "${features[@]}"; do
    # shellcheck disable=SC2143
    if [[ $(echo "${features_enabled}" | grep "${feat}") || $(echo "${features_enabled}" | grep "${gke_all_alpha_feature}") ]]; then
      echo -e "${GREEN} ${CHECK} Found ${feat}${NC}\n"
    else
      echo -e "${RED} ${CROSS} Not found ${feat}${NC}\n"
      exit_status=1
    fi
  done
  return ${exit_status}
}

check_csi() {
  local exit_status=0
  readonly apis_for_k8s_13=("csidrivers.csi.storage.k8s.io" "csinodeinfos.csi.storage.k8s.io")

  common_required_apis=(
    "volumesnapshotclasses.snapshot.storage.k8s.io"
    "volumesnapshotcontents.snapshot.storage.k8s.io"
    "volumesnapshots.snapshot.storage.k8s.io"
  )

  echo -e "${LIGHT_BLUE}Checking if CSI APIs are installed in cluster...${NC}\n"

  k8s_version=$(kubectl version --short | grep Server | awk '{print $3}' | cut -d '.' -f2)
  if [[ "${k8s_version}" == 13 ]]; then
    # shellcheck disable=SC2206
    common_required_apis+=(${apis_for_k8s_13[@]})
  fi

  for api in "${common_required_apis[@]}"; do
    # shellcheck disable=SC2143
    if [[ $(kubectl get crds | grep "${api}") ]]; then
      echo -e "${GREEN} ${CHECK} Found ${api}${NC}\n"
    else
      echo -e "${RED} ${CROSS} Not Found ${api}${NC}\n"
      exit_status=1
    fi
  done
  return ${exit_status}
}

check_dns_resolution() {
  echo -e "${LIGHT_BLUE}Checking if DNS resolution working in K8s cluster...${NC}\n"
  local exit_status=0

  cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: dnsutils
spec:
  containers:
  - name: dnsutils
    image: gcr.io/kubernetes-e2e-test-images/dnsutils:1.3
    command:
      - sleep
      - "3600"
    imagePullPolicy: IfNotPresent
  restartPolicy: Always
EOF

  set +o errexit
  kubectl wait --for=condition=ready --timeout=2m pod/dnsutils &>/dev/null
  kubectl exec -it dnsutils -- nslookup kubernetes.default &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Able to resolve DNS \"kubernetes.default\" service inside pods${NC}\n"
  else
    echo -e "${RED} ${CROSS} Could not resolve DNS \"kubernetes.default\" service inside pod${NC}\n"
    exit_status=1
  fi
  kubectl delete pod dnsutils &>/dev/null
  set -o errexit
  return ${exit_status}
}

check_volume_snapshot() {
  echo -e "${LIGHT_BLUE}Checking if volume snapshot and restore enabled in K8s cluster...${NC}\n"
  local err_status=1
  local success_status=0
  local retries=30
  local sleep=5
  set +o errexit

  cat <<EOF | kubectl apply -f - &>/dev/null
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: ${SOURCE_PVC}
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: ${STORAGE_CLASS}
  resources:
    requests:
      storage: 1Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: ${SOURCE_POD}
spec:
  containers:
  - name: busybox
    image: busybox
    command: ["/bin/sh", "-c"]
    args: ["touch /demo/data/sample-file.txt && sleep 3000"]
    volumeMounts:
    - name: source-data
      mountPath: /demo/data
  volumes:
  - name: source-data
    persistentVolumeClaim:
      claimName: ${SOURCE_PVC}
      readOnly: false
EOF

  kubectl wait --for=condition=ready --timeout=2m pod/source-pod &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Created source pod and pvc${NC}\n"
  else
    echo -e "${RED} ${CROSS} Error creating source pod and pvc${NC}\n"
    return ${err_status}
  fi

  # shellcheck disable=SC2143
  if [[ $(kubectl get apiservices | grep "v1beta1.snapshot.storage.k8s.io") ]]; then
    cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: snapshot.storage.k8s.io/v1beta1
kind: VolumeSnapshot
metadata:
  name: ${VOLUME_SNAP_SRC}
spec:
  volumeSnapshotClassName: ${SNAPSHOT_CLASS}
  source:
    persistentVolumeClaimName: ${SOURCE_PVC}
EOF
  else
    cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: snapshot.storage.k8s.io/v1alpha1
kind: VolumeSnapshot
metadata:
  name: ${VOLUME_SNAP_SRC}
spec:
  snapshotClassName: ${SNAPSHOT_CLASS}
  source:
    kind: PersistentVolumeClaim
    name: ${SOURCE_PVC}
EOF
  fi
  # shellcheck disable=SC2181
  if [[ $? -ne 0 ]]; then
    echo -e "${RED_BOLD} ${CROSS} Error creating volume snapshot from source pvc${NC}\n"
    return ${err_status}
  fi

  while true; do
    if [[ ${retries} -eq 0 ]]; then
      echo -e "${RED_BOLD} ${CROSS} Volume snapshot from source pvc not readyToUse (waited 150 sec)${NC}\n"
      return ${err_status}
    fi
    # shellcheck disable=SC2143
    if [[ $(kubectl get volumesnapshot "${VOLUME_SNAP_SRC}" -o yaml | grep 'readyToUse: true') ]]; then
      echo -e "${GREEN} ${CHECK} Created volume snapshot from source pvc and is readyToUse${NC}\n"
      break
    else
      sleep "${sleep}"
      ((retries--))
      continue
    fi
  done

  cat <<EOF | kubectl apply -f - &>/dev/null
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: ${RESTORE_PVC}
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: ${STORAGE_CLASS}
  resources:
    requests:
      storage: 1Gi
  dataSource:
    kind: VolumeSnapshot
    name: ${VOLUME_SNAP_SRC}
    apiGroup: snapshot.storage.k8s.io
---
apiVersion: v1
kind: Pod
metadata:
  name: ${RESTORE_POD}
spec:
  containers:
  - name: busybox
    image: busybox
    args:
    - sleep
    - "3600"
    volumeMounts:
    - name: source-data
      mountPath: /demo/data
  volumes:
  - name: source-data
    persistentVolumeClaim:
      claimName: ${RESTORE_PVC}
      readOnly: false
EOF

  kubectl wait --for=condition=ready --timeout=2m pod/"${RESTORE_POD}" &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Created restore pod from volume snapshot${NC}\n"
  else
    echo -e "${RED_BOLD} ${CROSS} Error creating pod and pvc from volume snapshot${NC}\n"
    return ${err_status}
  fi

  kubectl exec -it "${RESTORE_POD}" -- ls /demo/data/sample-file.txt &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Restored pod has expected data${NC}\n"
  else
    echo -e "${RED_BOLD} ${CROSS} Restored pod does not have expected data${NC}\n"
    return ${err_status}
  fi

  kubectl delete --ignore-not-found=true pod/"${SOURCE_POD}" &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Deleted source pod${NC}\n"
  else
    echo -e "${RED_BOLD} ${CROSS} Error cleaning up source pod${NC}\n"
    exit_status=1
  fi

  # shellcheck disable=SC2143
  if [[ $(kubectl get apiservices | grep "v1beta1.snapshot.storage.k8s.io") ]]; then
    cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: snapshot.storage.k8s.io/v1beta1
kind: VolumeSnapshot
metadata:
  name: ${UNUSED_VOLUME_SNAP_SRC}
spec:
  volumeSnapshotClassName: ${SNAPSHOT_CLASS}
  source:
    persistentVolumeClaimName: ${SOURCE_PVC}
EOF
  else
    cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: snapshot.storage.k8s.io/v1alpha1
kind: VolumeSnapshot
metadata:
  name: ${UNUSED_VOLUME_SNAP_SRC}
spec:
  snapshotClassName: ${SNAPSHOT_CLASS}
  source:
    kind: PersistentVolumeClaim
    name: ${SOURCE_PVC}
EOF
  fi
  # shellcheck disable=SC2181
  if [[ $? -ne 0 ]]; then
    echo -e "${RED_BOLD} ${CROSS} Error creating volume snapshot from unused source pvc${NC}\n"
    return ${err_status}
  fi

  while true; do
    if [[ ${retries} -eq 0 ]]; then
      echo -e "${RED_BOLD} ${CROSS} Volume snapshot from source pvc not readyToUse (waited 150 sec)${NC}\n"
      return ${err_status}
    fi
    # shellcheck disable=SC2143
    if [[ $(kubectl get volumesnapshot "${UNUSED_VOLUME_SNAP_SRC}" -o yaml | grep 'readyToUse: true') ]]; then
      echo -e "${GREEN} ${CHECK} Created volume snapshot from unused source pvc and is readyToUse${NC}\n"
      break
    else
      sleep "${sleep}"
      ((retries--))
      continue
    fi
  done

  cat <<EOF | kubectl apply -f - &>/dev/null
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: ${UNUSED_RESTORE_PVC}
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: ${STORAGE_CLASS}
  resources:
    requests:
      storage: 1Gi
  dataSource:
    kind: VolumeSnapshot
    name: ${UNUSED_VOLUME_SNAP_SRC}
    apiGroup: snapshot.storage.k8s.io
---
apiVersion: v1
kind: Pod
metadata:
  name: ${UNUSED_RESTORE_POD}
spec:
  containers:
  - name: busybox
    image: busybox
    args:
    - sleep
    - "3600"
    volumeMounts:
    - name: source-data
      mountPath: /demo/data
  volumes:
  - name: source-data
    persistentVolumeClaim:
      claimName: ${UNUSED_RESTORE_PVC}
      readOnly: false
EOF

  kubectl wait --for=condition=ready --timeout=2m pod/"${UNUSED_RESTORE_POD}" &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Created restore pod from volume snapshot of unused pv${NC}\n"
  else
    echo -e "${RED_BOLD} ${CROSS} Error creating pod and pvc from volume snapshot of unused pv${NC}\n"
    return ${err_status}
  fi

  kubectl exec -it "${UNUSED_RESTORE_POD}" -- ls /demo/data/sample-file.txt &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Restored pod from volume snapshot of unused pv has expected data${NC}\n"
  else
    echo -e "${RED_BOLD} ${CROSS} Restored pod from volume snapshot of unused pv does not have expected data${NC}\n"
    return ${err_status}
  fi

  set -o errexit
  return ${success_status}
}

cleanup() {
  local exit_status=0

  echo -e "${LIGHT_BLUE}Cleaning up...${NC}\n"

  kubectl delete --ignore-not-found=true pod/"${SOURCE_POD}" pod/"${RESTORE_POD}" pod/"${UNUSED_RESTORE_POD}" pvc/"${SOURCE_PVC}" \
    pvc/"${RESTORE_PVC}" pvc/"${UNUSED_RESTORE_PVC}" volumesnapshot/"${VOLUME_SNAP_SRC}" volumesnapshot/"${UNUSED_VOLUME_SNAP_SRC}" &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "\n${GREEN} ${CHECK} Cleaned up all the resources${NC}\n"
  else
    echo -e "${RED_BOLD} ${CROSS} Error cleaning up intermediate resources${NC}\n"
    exit_status=1
  fi

  return ${exit_status}
}

exit_trap() {
  local rc=$?
  if [ ${rc} -eq 0 ]; then
    echo -e "\n${GREEN_BOLD}All pre-flight checks succeeded!${NC}\n"
  else
    echo -e "\n${RED_BOLD}Pre-flight checks failed!${NC}\n"
  fi
  exit ${rc}
}

take_input "$@"

echo
echo -e "${GREEN_BOLD}Running pre-flight checks before installing K8s Triliovault. Might take a few minutes...${NC}\n"

set -o errexit
set -o pipefail

trap "exit_trap" EXIT

check_kubectl
check_kubectl_access
check_helm_tiller_version
check_kubernetes_version
check_kubernetes_rbac
check_feature_gates
check_storage_snapshot_class
check_csi
check_dns_resolution
check_volume_snapshot
cleanup
