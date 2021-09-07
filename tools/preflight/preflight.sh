#!/usr/bin/env bash

# Purpose: Helper script for running pre-flight checks before installing K8s-Triliovault application.

set -o pipefail

# COLOUR CONSTANTS
GREEN='\033[0;32m'
GREEN_BOLD='\033[0;32m\e[1m'
LIGHT_BLUE='\033[1;34m'
RED='\033[0;31m'
RED_BOLD='\033[0;31m\e[1m'
BROWN='\033[0;33m'
NC='\033[0m'

CHECK='\xE2\x9C\x94'
CROSS='\xE2\x9D\x8C'

MIN_HELM_VERSION="3.0.0"
MIN_K8S_VERSION="1.18.0"
PREFLIGHT_RUN_SUCCESS=true
STORAGE_SNAPSHOT_CLASS_CHECK_SUCCESS=true

# shellcheck disable=SC2018
RANDOM_STRING=$(
  tr -dc a-z </dev/urandom | head -c 6
  echo ''
)
SOURCE_POD="source-pod-${RANDOM_STRING}"
SOURCE_PVC="source-pvc-${RANDOM_STRING}"
RESTORE_POD="restored-pod-${RANDOM_STRING}"
RESTORE_PVC="restored-pvc-${RANDOM_STRING}"
VOLUME_SNAP_SRC="snapshot-source-pvc-${RANDOM_STRING}"
UNUSED_RESTORE_POD="unused-restored-pod-${RANDOM_STRING}"
UNUSED_RESTORE_PVC="unused-restored-pvc-${RANDOM_STRING}"
UNUSED_VOLUME_SNAP_SRC="unused-source-pvc-${RANDOM_STRING}"
DNS_UTILS="dnsutils-${RANDOM_STRING}"

print_help() {
  echo "
--------------------------------------------------------------
Usage:
kubectl tvk-preflight --storageclass <storage_class_name> --snapshotclass <volume_snapshot_class>
Params:
	--storageclass	name of storage class being used in k8s cluster
	--snapshotclass name of volume snapshot class being used in k8s cluster (OPTIONAL)
	--kubeconfig	path to kube config (OPTIONAL)
--------------------------------------------------------------
"
}

take_input() {
  if [[ -z "${1}" ]]; then
    echo "Error: --storageclass needed to run pre flight checks!"
    print_help
    exit 1
  fi
  while [ -n "$1" ]; do
    case "$1" in
    --storageclass)
      if [[ -n "$2" ]]; then
        STORAGE_CLASS=$2
        shift 2
      else
        echo "Error: flag --storageclass value may not be empty!"
        print_help
        exit 1
      fi
      ;;
    --snapshotclass)
      if [[ -n "$2" ]]; then
        SNAPSHOT_CLASS=$2
        shift 2
      else
        echo "Error: flag --snapshotclass value may not be empty. Either set the value or skip this flag!"
        print_help
        exit 1
      fi
      ;;
    --kubeconfig)
      if [[ -n "$2" ]]; then
        KUBECONFIG_PATH=$2
        shift 2
      else
        echo "Error: flag --kubeconfig value may not be empty. Either set the value or skip this flag!"
        print_help
        exit 1
      fi
      ;;
    -h | --help)
      print_help
      exit
      ;;
    *)
      echo "Error: wrong input parameter $1 passed. Check Usage!"
      print_help
      exit 1
      ;;
    esac
  done
  if [[ -z "${STORAGE_CLASS}" ]]; then
    echo "Error: --storageclass flag needed to run pre flight checks!"
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
  if [[ "${sorted_version}" != "$1" || "${sorted_version}" == "$2" ]]; then
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

check_helm_version() {
  echo -e "${LIGHT_BLUE}Checking for required Helm version (>= v${MIN_HELM_VERSION})...${NC}\n"
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

  local helm_version
  helm_version=$(helm version --template "{{ .Version }}")
  if [[ ${helm_version} != "<no value>" ]]; then
    echo -e "${GREEN} ${CHECK} Helm version ${helm_version} meets minimum required version v${MIN_HELM_VERSION}${NC}\n"
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

  # shellcheck disable=SC1083
  provisioner=$(kubectl get sc "${STORAGE_CLASS}" -oyaml | grep -E '(^)provisioner:(\s)' | awk '{print $2}')

  if [[ -z "${SNAPSHOT_CLASS}" ]]; then
    # shellcheck disable=SC1083
    vsList=$(kubectl get volumesnapshotclass | awk '{if(NR>1)print $1}')
    if [[ -n "${vsList}" ]]; then
      # shellcheck disable=SC2162
      while read -r vs; do
        vsMeta=$(kubectl get volumesnapshotclass "$vs" -o yaml)
        # shellcheck disable=SC2143
        if [[ $(echo "$vsMeta" | grep -E "(^)driver: ${provisioner}($)") ]]; then
          # shellcheck disable=SC2143
          if [[ $(kubectl get volumesnapshotclass "$vs" -o yaml | grep -E "(^|\s)snapshot.storage.kubernetes.io/is-default-class: \"true\"($|\s)") ]]; then
            SNAPSHOT_CLASS=$vs
            break
          fi
          SNAPSHOT_CLASS=$vs
        fi
      done <<<"$vsList"
    fi

    if [[ -z "${SNAPSHOT_CLASS}" ]]; then
      echo -e "${RED} ${CROSS} Volume snapshot class having same driver as StorageClass's provisioner=$provisioner not found in cluster${NC}\n"
      exit_status=1
      return ${exit_status}
    else
      echo -e "${GREEN} ${CHECK} Extracted volume snapshot class \"${SNAPSHOT_CLASS}\" found in cluster${NC}\n"
      echo -e "${GREEN} ${CHECK} Volume snapshot class \"${SNAPSHOT_CLASS}\" driver matches with given StorageClass's provisioner=$provisioner${NC}\n"
      return
    fi
  fi

  # shellcheck disable=SC2143
  if [[ $(kubectl get volumesnapshotclass | grep -E "(^|\s)${SNAPSHOT_CLASS}($|\s)") ]]; then
    echo -e "${GREEN} ${CHECK} Volume snapshot class \"${SNAPSHOT_CLASS}\" found in cluster${NC}\n"
    # shellcheck disable=SC1009
    if [[ $(kubectl get volumesnapshotclass "${SNAPSHOT_CLASS}" -oyaml | grep -E "(^)driver: ${provisioner}($)") ]]; then
      echo -e "${GREEN} ${CHECK} Volume snapshot class \"${SNAPSHOT_CLASS}\" driver matches with given StorageClass's provisioner=$provisioner${NC}\n"
    else
      echo -e "${RED} ${CROSS} Volume snapshot class \"${SNAPSHOT_CLASS}\" driver does not match with given StorageClass's provisioner=$provisioner${NC}\n"
      exit_status=1
    fi
  else
    echo -e "${RED} ${CROSS} Volume snapshot class \"${SNAPSHOT_CLASS}\" not found in cluster${NC}\n"
    exit_status=1
  fi

  return ${exit_status}
}

check_csi() {
  local exit_status=0

  common_required_apis=(
    "volumesnapshotclasses.snapshot.storage.k8s.io"
    "volumesnapshotcontents.snapshot.storage.k8s.io"
    "volumesnapshots.snapshot.storage.k8s.io"
  )

  echo -e "${LIGHT_BLUE}Checking if CSI APIs are installed in cluster...${NC}\n"

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
  name: ${DNS_UTILS}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
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

  kubectl wait --for=condition=ready --timeout=5m pod/"${DNS_UTILS}" &>/dev/null
  kubectl exec -it "${DNS_UTILS}" -- nslookup kubernetes.default &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Able to resolve DNS \"kubernetes.default\" service inside pods${NC}\n"
  else
    echo -e "${RED} ${CROSS} Could not resolve DNS \"kubernetes.default\" service inside pod${NC}\n"
    exit_status=1
  fi
  kubectl delete pod "${DNS_UTILS}" &>/dev/null
  return ${exit_status}
}

check_volume_snapshot() {
  echo -e "${LIGHT_BLUE}Checking if volume snapshot and restore enabled in K8s cluster...${NC}\n"
  local err_status=1
  local success_status=0
  local retries=45
  local sleep=5

  echo -e "${BROWN} Creating source pod and pvc for volume-snapshot check${NC}\n"

  cat <<EOF | kubectl apply -f - &>/dev/null
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: ${SOURCE_PVC}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
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
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
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

  kubectl wait --for=condition=ready --timeout=5m pod/"${SOURCE_POD}" &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Successfully created source pod [Ready] and pvc ${NC}\n"
  else
    echo -e "${RED} ${CROSS} Error waiting for source pod and pvc to be in [Ready] state${NC}\n"
    return ${err_status}
  fi

  api_service=$(kubectl get apiservices)
  snapshotVersion=""
  # shellcheck disable=SC2143
  if [[ $(echo "$api_service" | grep "v1.snapshot.storage.k8s.io") ]]; then
    snapshotVersion="v1"
  elif [[ $(echo "$api_service" | grep "v1beta1.snapshot.storage.k8s.io") ]]; then
    snapshotVersion="v1beta1"
  else
    echo -e "${RED} ${CROSS} Volume snapshot crd version [v1 or v1beta1] not found in cluster${NC}\n"
    return ${err_status}
  fi

  echo -e "${BROWN} Creating volume snapshot from source pvc${NC}\n"

  # shellcheck disable=SC2006
  cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: snapshot.storage.k8s.io/${snapshotVersion}
kind: VolumeSnapshot
metadata:
  name: ${VOLUME_SNAP_SRC}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
spec:
  volumeSnapshotClassName: ${SNAPSHOT_CLASS}
  source:
    persistentVolumeClaimName: ${SOURCE_PVC}
EOF

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

  echo -e "${BROWN} Creating restore pod from volume snapshot${NC}\n"

  cat <<EOF | kubectl apply -f - &>/dev/null
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: ${RESTORE_PVC}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
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
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
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

  kubectl wait --for=condition=ready --timeout=5m pod/"${RESTORE_POD}" &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Successfully created restore pod [Ready] from volume snapshot${NC}\n"
  else
    echo -e "${RED_BOLD} ${CROSS} Error waiting for restore pod and pvc from volume snapshot to be in [Ready] state${NC}\n"
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

  echo -e "${BROWN} Creating volume snapshot from unused source pvc${NC}\n"

  # shellcheck disable=SC2143
  cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: snapshot.storage.k8s.io/${snapshotVersion}
kind: VolumeSnapshot
metadata:
  name: ${UNUSED_VOLUME_SNAP_SRC}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
spec:
  volumeSnapshotClassName: ${SNAPSHOT_CLASS}
  source:
    persistentVolumeClaimName: ${SOURCE_PVC}
EOF

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

  echo -e "${BROWN} Creating restore pod from volume snapshot of unused pv${NC}\n"

  cat <<EOF | kubectl apply -f - &>/dev/null
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: ${UNUSED_RESTORE_PVC}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
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
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
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

  kubectl wait --for=condition=ready --timeout=5m pod/"${UNUSED_RESTORE_POD}" &>/dev/null
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN} ${CHECK} Successfully created restore pod [Ready] from volume snapshot of unused pv${NC}\n"
  else
    echo -e "${RED_BOLD} ${CROSS} Error waiting for restore pod and pvc from volume snapshot of unused pv to be in [Ready] state${NC}\n"
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

  return ${success_status}
}

cleanup() {
  local exit_status=0

  echo -e "\n${LIGHT_BLUE}Cleaning up residual resources...${NC}"

  declare -a pvc=("${SOURCE_PVC}" "${RESTORE_PVC}" "${UNUSED_RESTORE_PVC}")
  for res in "${pvc[@]}"; do
    kubectl delete pvc -n "${NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s &>/dev/null || true
    kubectl patch pvc -n "${NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' &>/dev/null || true
  done

  declare -a vsnaps=("${VOLUME_SNAP_SRC}" "${UNUSED_VOLUME_SNAP_SRC}")
  for res in "${vsnaps[@]}"; do
    kubectl delete volumesnapshot -n "${NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s &>/dev/null || true
    kubectl patch volumesnapshot -n "${NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' &>/dev/null || true
  done

  kubectl delete --force --grace-period=0 pod "${SOURCE_POD}" "${RESTORE_POD}" "${UNUSED_RESTORE_POD}" &>/dev/null || true

  kubectl delete all -l preflight-run="${RANDOM_STRING}" --force --grace-period=0 &>/dev/null || true

  echo -e "\n${GREEN} ${CHECK} Cleaned up all the resources${NC}\n"

  return ${exit_status}
}

export -f check_kubectl
export -f check_kubectl_access
export -f check_helm_version
export -f check_kubernetes_version
export -f check_kubernetes_rbac
export -f check_storage_snapshot_class
export -f check_csi
export -f check_dns_resolution
export -f check_volume_snapshot
export -f cleanup

# --- End Definitions Section ---
# check if we are being sourced by another script or shell
[[ "${#BASH_SOURCE[@]}" -gt "1" ]] && { return 0; }
# --- Begin Code Execution Section ---

take_input "$@"

echo
echo -e "${GREEN_BOLD}--- Running Pre-flight Checks Before Installing Triliovault for Kubernetes ---${NC}\n"
echo -e "${BROWN}Might take a few minutes...${NC}\n"

trap "cleanup" EXIT

check_kubectl
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_RUN_SUCCESS=false
fi

check_kubectl_access
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_RUN_SUCCESS=false
fi

check_helm_version
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_RUN_SUCCESS=false
fi

check_kubernetes_version
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_RUN_SUCCESS=false
fi

check_kubernetes_rbac
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_RUN_SUCCESS=false
fi

check_storage_snapshot_class
retCode=$?
if [[ retCode -ne 0 ]]; then
  STORAGE_SNAPSHOT_CLASS_CHECK_SUCCESS=false
  PREFLIGHT_RUN_SUCCESS=false
fi

check_csi
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_RUN_SUCCESS=false
fi

check_dns_resolution
retCode=$?
if [[ retCode -ne 0 ]]; then
  PREFLIGHT_RUN_SUCCESS=false
fi

if [ $STORAGE_SNAPSHOT_CLASS_CHECK_SUCCESS == "true" ]; then
  check_volume_snapshot
  retCode=$?
  if [[ retCode -ne 0 ]]; then
    PREFLIGHT_RUN_SUCCESS=false
  fi
else
  echo -e "${LIGHT_BLUE}Skipping 'VOLUME_SNAPSHOT' check as 'STORAGE_SNAPSHOT_CLASS' preflight check failed${NC}\n"
fi

# Print status of Pre-flight checks
if [ $PREFLIGHT_RUN_SUCCESS == "true" ]; then
  echo -e "\n${GREEN_BOLD}All Pre-flight Checks Succeeded!${NC}\n"
else
  echo -e "\n${RED_BOLD}Some Pre-flight Checks Failed!${NC}\n"
  exit 1
fi
