#!/usr/bin/env bash

# Purpose: Helper script for running pre-flight checks before installing K8s-Triliovault application.

set -o pipefail

TIMESTAMP=$(date +%F_%H-%M-%S)

# COLOUR CONSTANTS
GREEN='\033[0;32m'
GREEN_BOLD='\033[1;32m'
LIGHT_BLUE='\033[1;34m'
RED='\033[0;31m'
RED_BOLD='\033[1;31m'
BROWN='\033[0;33m'
NC='\033[0m'
BLUE='\033[1;36m'

CHECK='\xE2\x9C\x94'
CROSS='\xE2\x9D\x8C'

MIN_HELM_VERSION="3.0.0"
STORAGE_SNAPSHOT_CLASS_CHECK_SUCCESS=true
MIN_K8S_VERSION="1.18.0"
PREFLIGHT_RUN_SUCCESS=true

# shellcheck disable=SC2018
RANDOM_STRING=$(
  LC_ALL=C tr -dc a-z </dev/urandom | head -c 6
  echo ''
)
SOURCE_POD="source-pod-${RANDOM_STRING}"
SOURCE_PVC="source-pvc-${RANDOM_STRING}"
RESTORE_POD="restored-pod-${RANDOM_STRING}"
RESTORE_PVC="restored-pvc-${RANDOM_STRING}"
VOLUME_SNAP_SRC="snapshot-source-pvc-${RANDOM_STRING}"
UNMOUNTED_RESTORE_POD="unmounted-restored-pod-${RANDOM_STRING}"
UNMOUNTED_RESTORE_PVC="unmounted-restored-pvc-${RANDOM_STRING}"
UNMOUNTED_VOLUME_SNAP_SRC="unmounted-source-pvc-${RANDOM_STRING}"
DNS_UTILS="dnsutils-${RANDOM_STRING}"
LABEL_K8S_PART_OF="app.kubernetes.io/part-of"
LABEL_K8S_PART_OF_VALUE="k8s-triliovault"

LOG_FILE=preflight-log-${TIMESTAMP}.log

echolog() (
  echo -n "[$(date +'%F %H:%M:%S')] " >>"${LOG_FILE}" 2>&1
  echo -e "${BLUE}[ INFO ]${NC}" "$@" | tee -a "${LOG_FILE}"
  echo
)

print_help() {
  echo "
--------------------------------------------------------------
Usage:
kubectl tvk-preflight --storageclass <storage_class_name> --snapshotclass <volume_snapshot_class>
Params:
  --storageclass  name of storage class being used in k8s cluster
  --local-registry name of the local registry to get images from (OPTIONAL)
  --service-account name of the service account (OPTIONAL)
  --image-pull-secret name of the secret configured for authentication (OPTIONAL)
  --snapshotclass name of volume snapshot class being used in k8s cluster (OPTIONAL)
  --kubeconfig  path to kube config (OPTIONAL)
--------------------------------------------------------------
"
}

take_input() {
  if [[ -z "${1}" ]]; then
    echolog "Error: --storageclass needed to run pre flight checks!"
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
        echolog "Error: flag --storageclass value may not be empty!"
        print_help
        exit 1
      fi
      ;;
    --snapshotclass)
      if [[ -n "$2" ]]; then
        SNAPSHOT_CLASS=$2
        shift 2
      else
        echolog "Error: flag --snapshotclass value may not be empty. Either set the value or skip this flag!"
        print_help
        exit 1
      fi
      ;;
    --kubeconfig)
      if [[ -n "$2" ]]; then
        KUBECONFIG_PATH=$2
        shift 2
      else
        echolog "Error: flag --kubeconfig value may not be empty. Either set the value or skip this flag!"
        print_help
        exit 1
      fi
      ;;
    --local-registry)
      if [[ -n "$2" ]]; then
        LOCAL_REGISTRY=$2
        shift 2
      else
        echolog "Error: flag --local-registry value may not be empty. Either set the value or skip this flag!"
        print_help
        exit 1
      fi
      ;;
    --image-pull-secret)
      if [[ -n "$2" ]]; then
        IMAGE_PULL_SECRET=$2
        shift 2
      else
        echolog "Error: flag --image-pull-secret value may not be empty. Either set the value or skip this flag!"
        print_help
        exit 1
      fi
      ;;
    --service-account)
      if [[ -n "$2" ]]; then
        SERVICE_ACCOUNT_NAME=$2
        shift 2
      else
        echolog "Error: flag --service-account value may not be empty. Either set the value or skip this flag!"
        print_help
        exit 1
      fi
      ;;
    -h | --help)
      print_help
      exit
      ;;
    *)
      echolog "Error: wrong input parameter $1 passed. Check Usage!"
      print_help
      exit 1
      ;;
    esac
  done
  if [[ -z "${STORAGE_CLASS}" ]]; then
    echolog "Error: --storageclass flag needed to run pre flight checks!"
    print_help
    exit 1
  fi
  if [[ -z "${LOCAL_REGISTRY}" && -n "${IMAGE_PULL_SECRET}" ]]; then
    echolog "Error: Cannot Give Pull Secret if local-registry is not provided!"
    exit 1
  fi
}

check_kubectl() {
  echo
  echolog "${LIGHT_BLUE}Checking for kubectl...${NC}\n"
  local exit_status=0
  if ! command -v "kubectl" &>/dev/null; then
    echolog "${RED} ${CROSS} Unable to find kubectl${NC}\n"
    exit_status=1
  else
    echo "kubectl found at path -" "$(! command -v "kubectl")" >>"${LOG_FILE}" 2>&1
    echolog "${GREEN} ${CHECK} Found kubectl${NC}\n"
  fi
  return ${exit_status}
}

check_kubectl_access() {
  local exit_status=0
  if [[ -n ${KUBECONFIG_PATH} ]]; then
    export KUBECONFIG=${KUBECONFIG_PATH}
  fi
  echolog "${LIGHT_BLUE}Checking access to the Kubernetes context $(kubectl config current-context)...${NC}\n"

  if [[ $(kubectl get ns default) ]]; then
    echolog "${GREEN} ${CHECK} Able to access the default Kubernetes namespace${NC}\n"
  else
    echolog "${RED} ${CROSS} Unable to access the default Kubernetes namespace${NC}\n"
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
  echolog "${LIGHT_BLUE}Checking for required Helm version (>= v${MIN_HELM_VERSION})...${NC}\n"
  local exit_status=0

  # Abort successfully in case of OCP setup
  if [[ $(check_if_ocp) == "Y" ]]; then
    echolog "${GREEN} ${CHECK} Helm not needed for OCP clusters${NC}\n"
    return ${exit_status}
  fi

  if ! command -v "helm" &>/dev/null; then
    echolog "${RED} ${CROSS} Unable to find helm${NC}\n"
    exit_status=1
  else
    echo "helm found at path -" "$(! command -v "helm")" >>"${LOG_FILE}" 2>&1
    echolog "${GREEN} ${CHECK} Found helm${NC}\n"
  fi

  local helm_version
  helm_version=$(helm version --template "{{ .Version }}")
  if [[ ${helm_version} != "<no value>" ]]; then
    echolog "${GREEN} ${CHECK} Helm version ${helm_version} meets minimum required version v${MIN_HELM_VERSION}${NC}\n"
  fi

  return ${exit_status}
}

check_kubernetes_version() {
  echolog "${LIGHT_BLUE}Checking for required Kubernetes version (>= v${MIN_K8S_VERSION})...${NC}\n"
  local exit_status=0
  local k8s_version
  k8s_version=$(kubectl version --short | grep Server | awk '{print $3}')

  if version_gt_eq "${k8s_version:1}" "${MIN_K8S_VERSION}"; then
    echolog "${GREEN} ${CHECK} Kubernetes version (${k8s_version}) meets minimum requirements${NC}\n"
  else
    echolog "${RED} ${CROSS} Kubernetes version (${k8s_version}) does not meet minimum requirements${NC}\n"
    exit_status=1
  fi
  return ${exit_status}
}

check_kubernetes_rbac() {
  echolog "${LIGHT_BLUE}Checking if Kubernetes RBAC is enabled...${NC}\n"
  local exit_status=0
  # The below shellcheck conflicts with pipefail
  # shellcheck disable=SC2143
  if [[ $(kubectl api-versions | grep rbac.authorization.k8s.io) ]]; then
    echolog "${GREEN} ${CHECK} Kubernetes RBAC is enabled${NC}\n"
  else
    echolog "${RED} ${CROSS} Kubernetes RBAC is not enabled${NC}\n"
    exit_status=1
  fi
  return ${exit_status}
}

check_storage_snapshot_class() {
  echolog "${LIGHT_BLUE}Checking if a StorageClass and VolumeSnapshotClass are present...${NC}\n"
  local exit_status=0
  # shellcheck disable=SC2143
  if [[ $(kubectl get storageclass | grep -E "(^|\s)${STORAGE_CLASS}($|\s)") ]]; then
    echolog "${GREEN} ${CHECK} Storage class \"${STORAGE_CLASS}\" found${NC}\n"
  else
    echolog "${RED} ${CROSS} Storage class \"${STORAGE_CLASS}\" not found${NC}\n"
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
      echolog "${RED} ${CROSS} Volume snapshot class having same driver as StorageClass's provisioner=$provisioner not found in cluster${NC}\n"
      exit_status=1
      return ${exit_status}
    else
      echolog "${GREEN} ${CHECK} Extracted volume snapshot class \"${SNAPSHOT_CLASS}\" found in cluster${NC}\n"
      echolog "${GREEN} ${CHECK} Volume snapshot class \"${SNAPSHOT_CLASS}\" driver matches with given StorageClass's provisioner=$provisioner${NC}\n"
      return
    fi
  fi

  # shellcheck disable=SC2143
  if [[ $(kubectl get volumesnapshotclass | grep -E "(^|\s)${SNAPSHOT_CLASS}($|\s)") ]]; then
    echolog "${GREEN} ${CHECK} Volume snapshot class \"${SNAPSHOT_CLASS}\" found in cluster${NC}\n"
    # shellcheck disable=SC1009
    if [[ $(kubectl get volumesnapshotclass "${SNAPSHOT_CLASS}" -oyaml | grep -E "(^)driver: ${provisioner}") ]]; then
      echolog "${GREEN} ${CHECK} Volume snapshot class \"${SNAPSHOT_CLASS}\" driver matches with given StorageClass's provisioner=$provisioner${NC}\n"
    else
      echolog "${RED} ${CROSS} Volume snapshot class \"${SNAPSHOT_CLASS}\" driver does not match with given StorageClass's provisioner=$provisioner${NC}\n"
      exit_status=1
    fi
  else
    echolog "${RED} ${CROSS} Volume snapshot class \"${SNAPSHOT_CLASS}\" not found in cluster${NC}\n"
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

  echolog "${LIGHT_BLUE}Checking if CSI APIs are installed in cluster...${NC}\n"

  for api in "${common_required_apis[@]}"; do
    # shellcheck disable=SC2143
    if [[ $(kubectl get crds | grep "${api}") ]]; then
      echolog "${GREEN} ${CHECK} Found ${api}${NC}\n"
    else
      echolog "${RED} ${CROSS} Not Found ${api}${NC}\n"
      exit_status=1
    fi
  done
  return ${exit_status}
}

check_dns_resolution() {
  if [[ -n "${LOCAL_REGISTRY}" ]]; then
    IMG_PATH=${LOCAL_REGISTRY}
  else
    IMG_PATH="gcr.io/kubernetes-e2e-test-images"
  fi
  echolog "${LIGHT_BLUE}Checking if DNS resolution working in K8s cluster...${NC}\n"
  local exit_status=1

  {
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${DNS_UTILS}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
    ${LABEL_K8S_PART_OF}: ${LABEL_K8S_PART_OF_VALUE}
spec:
  imagePullSecrets:
    - name: ${IMAGE_PULL_SECRET}
  serviceAccountName: ${SERVICE_ACCOUNT_NAME}
  containers:
  - name: dnsutils
    image: ${IMG_PATH}/dnsutils:1.3
    command:
      - sleep
      - "3600"
    imagePullPolicy: IfNotPresent
    resources:
      requests:
        memory: "64Mi"
        cpu: "250m"
      limits:
        memory: "128Mi"
        cpu: "500m"
  restartPolicy: Always
EOF
    echo "Waiting for dns pod ${DNS_UTILS} to become 'Ready'" >>"${LOG_FILE}" 2>&1
    kubectl wait --for=condition=ready --timeout=3m pod/"${DNS_UTILS}" >>"${LOG_FILE}" 2>&1
    # shellcheck disable=SC2181
    if [[ $? -eq 0 ]]; then
      n=0
      until [ "$n" -ge 3 ]; do
        kubectl exec -it "${DNS_UTILS}" -- nslookup kubernetes.default
        # shellcheck disable=SC2181
        if [ $? -eq 0 ]; then
          exit_status=0
          break
        else
          echo "Retrying to check dns resolution for service kubernetes.default" >>"${LOG_FILE}" 2>&1
          n=$((n + 1))
          sleep 2
        fi
      done
    fi

  } >>"${LOG_FILE}" 2>&1

  # shellcheck disable=SC2181
  if [[ $exit_status -eq 0 ]]; then
    echolog "${GREEN} ${CHECK} Able to resolve DNS \"kubernetes.default\" service inside pods${NC}\n"
  else
    echolog "${RED} ${CROSS} Could not resolve DNS \"kubernetes.default\" service inside pod${NC}\n"
  fi
  kubectl delete --force --grace-period=0 --timeout=5s pod "${DNS_UTILS}" >>"${LOG_FILE}" 2>&1
  return ${exit_status}
}

check_volume_snapshot() {
  if [[ -n "${LOCAL_REGISTRY}" ]]; then
    IMG_PATH="${LOCAL_REGISTRY}/busybox"
  else
    IMG_PATH="busybox"
  fi
  echolog "${LIGHT_BLUE}Checking if volume snapshot and restore enabled in K8s cluster...${NC}\n"
  local err_status=1
  local success_status=0
  local retries=60
  local sleep=5

  echolog "${BROWN} Creating source pod and pvc for volume-snapshot check${NC}\n"

  # shellcheck disable=SC2129
  cat <<EOF | kubectl apply -f - >>"${LOG_FILE}" 2>&1
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: ${SOURCE_PVC}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
    ${LABEL_K8S_PART_OF}: ${LABEL_K8S_PART_OF_VALUE}
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
    ${LABEL_K8S_PART_OF}: ${LABEL_K8S_PART_OF_VALUE}
spec:
  imagePullSecrets:
  - name: ${IMAGE_PULL_SECRET}
  serviceAccountName: ${SERVICE_ACCOUNT_NAME}
  containers:
  - name: busybox
    image: ${IMG_PATH}
    command: ["/bin/sh", "-c"]
    args: ["touch /demo/data/sample-file.txt && sleep 3000"]
    resources:
      requests:
        memory: "64Mi"
        cpu: "250m"
      limits:
        memory: "128Mi"
        cpu: "500m"
    volumeMounts:
    - name: source-data
      mountPath: /demo/data
  volumes:
  - name: source-data
    persistentVolumeClaim:
      claimName: ${SOURCE_PVC}
      readOnly: false
EOF

  echo "Waiting for source pod ${SOURCE_POD} to become 'Ready'" >>"${LOG_FILE}" 2>&1
  kubectl wait --for=condition=ready --timeout=3m pod/"${SOURCE_POD}" >>"${LOG_FILE}" 2>&1
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echolog "${GREEN} ${CHECK} Created source pod and pvc${NC}\n"
  else
    echolog "${RED} ${CROSS} Error creating source pod and pvc${NC}\n"
    return ${err_status}
  fi

  exec_status=1
  n=0
  echo "Checking for file -> /demo/data/sample-file.txt in source pod ${SOURCE_POD}" >>"${LOG_FILE}" 2>&1
  until [ "$n" -ge 3 ]; do
    kubectl exec -it "${SOURCE_POD}" -- ls /demo/data/sample-file.txt >>"${LOG_FILE}" 2>&1
    # shellcheck disable=SC2181
    if [ $? -eq 0 ]; then
      exec_status=0
      break
    else
      echo "Retrying exec to check if file is created in pod ${SOURCE_POD} of source PVC" >>"${LOG_FILE}" 2>&1
      n=$((n + 1))
      sleep 2
    fi
  done

  api_service=$(kubectl get apiservices)
  snapshotVersion=""
  # shellcheck disable=SC2143
  if [[ $(echo "$api_service" | grep "v1.snapshot.storage.k8s.io") ]]; then
    snapshotVersion="v1"
  elif [[ $(echo "$api_service" | grep "v1beta1.snapshot.storage.k8s.io") ]]; then
    snapshotVersion="v1beta1"
  else
    echolog "${RED} ${CROSS} Volume snapshot crd version [v1 or v1beta1] not found in cluster${NC}\n"
    return ${err_status}
  fi

  echolog "${BROWN} Creating volume snapshot from source pvc${NC}\n"

  # shellcheck disable=SC2006
  cat <<EOF | kubectl apply -f - >>"${LOG_FILE}" 2>&1
apiVersion: snapshot.storage.k8s.io/${snapshotVersion}
kind: VolumeSnapshot
metadata:
  name: ${VOLUME_SNAP_SRC}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
    ${LABEL_K8S_PART_OF}: ${LABEL_K8S_PART_OF_VALUE}
spec:
  volumeSnapshotClassName: ${SNAPSHOT_CLASS}
  source:
    persistentVolumeClaimName: ${SOURCE_PVC}
EOF

  # shellcheck disable=SC2181
  if [[ $? -ne 0 ]]; then
    echolog "${RED_BOLD} ${CROSS} Error creating volume snapshot from source pvc${NC}\n"
    return ${err_status}
  fi

  while true; do
    if [[ ${retries} -eq 0 ]]; then
      echolog "${RED_BOLD} ${CROSS} Volume snapshot from source pvc not readyToUse (waited 300 sec)${NC}\n"
      return ${err_status}
    fi
    # shellcheck disable=SC2143
    if [[ $(kubectl get volumesnapshot "${VOLUME_SNAP_SRC}" -o yaml | grep 'readyToUse: true') ]]; then
      echolog "${GREEN} ${CHECK} Created volume snapshot from source pvc and is readyToUse${NC}\n"
      break
    else
      echo "Waiting for Volume snapshot from source pvc be become 'readyToUse:true'" >>"${LOG_FILE}" 2>&1
      sleep "${sleep}"
      ((retries--))
      continue
    fi
  done

  echolog "${BROWN} Creating restore pod from volume snapshot${NC}\n"

  # shellcheck disable=SC2129
  cat <<EOF | kubectl apply -f - >>"${LOG_FILE}" 2>&1
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: ${RESTORE_PVC}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
    ${LABEL_K8S_PART_OF}: ${LABEL_K8S_PART_OF_VALUE}
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
    ${LABEL_K8S_PART_OF}: ${LABEL_K8S_PART_OF_VALUE}
spec:
  imagePullSecrets:
    - name: ${IMAGE_PULL_SECRET}
  serviceAccountName: ${SERVICE_ACCOUNT_NAME}
  containers:
  - name: busybox
    image: ${IMG_PATH}
    args:
    - sleep
    - "3600"
    resources:
      requests:
        memory: "64Mi"
        cpu: "250m"
      limits:
        memory: "128Mi"
        cpu: "500m"
    volumeMounts:
    - name: source-data
      mountPath: /demo/data
  volumes:
  - name: source-data
    persistentVolumeClaim:
      claimName: ${RESTORE_PVC}
      readOnly: false
EOF

  echo "Waiting for restore pod ${RESTORE_POD} to become 'Ready'" >>"${LOG_FILE}" 2>&1
  kubectl wait --for=condition=ready --timeout=3m pod/"${RESTORE_POD}" >>"${LOG_FILE}" 2>&1
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echolog "${GREEN} ${CHECK} Created restore pod from volume snapshot${NC}\n"
  else
    echolog "${RED_BOLD} ${CROSS} Error creating pod and pvc from volume snapshot${NC}\n"
    return ${err_status}
  fi

  exec_status=1
  n=0
  echo "Checking for file -> /demo/data/sample-file.txt in restore pod ${RESTORE_POD}" >>"${LOG_FILE}" 2>&1
  until [ "$n" -ge 3 ]; do
    kubectl exec -it "${RESTORE_POD}" -- ls /demo/data/sample-file.txt >>"${LOG_FILE}" 2>&1
    # shellcheck disable=SC2181
    if [ $? -eq 0 ]; then
      exec_status=0
      break
    else
      echo "Retrying exec to check data from Restored pod ${RESTORE_POD} from volume snapshot of source PVC" >>"${LOG_FILE}" 2>&1
      n=$((n + 1))
      sleep 2
    fi
  done

  # shellcheck disable=SC2181
  if [[ $exec_status -eq 0 ]]; then
    echolog "${GREEN} ${CHECK} Restored pod has expected data${NC}\n"
  else
    echolog "${RED_BOLD} ${CROSS} Restored pod does not have expected data${NC}\n"
    return ${err_status}
  fi

  kubectl delete --force --grace-period=0 --timeout=5s --ignore-not-found=true pod/"${SOURCE_POD}" >>"${LOG_FILE}" 2>&1
  kubectl patch po "${SOURCE_POD}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' >>"${LOG_FILE}" 2>&1 || true
  kubectl get pod "${SOURCE_POD}" >>"${LOG_FILE}" 2>&1
  # shellcheck disable=SC2181
  if [[ $? -ne 0 ]]; then
    echolog "${GREEN} ${CHECK} Deleted source pod${NC}\n"
  else
    echolog "${RED} ${CROSS} Error cleaning up source pod${NC}\n"
    exit_status=1
  fi

  echolog "${BROWN} Creating volume snapshot from unmounted source pvc${NC}\n"

  # shellcheck disable=SC2143
  cat <<EOF | kubectl apply -f - >>"${LOG_FILE}" 2>&1
apiVersion: snapshot.storage.k8s.io/${snapshotVersion}
kind: VolumeSnapshot
metadata:
  name: ${UNMOUNTED_VOLUME_SNAP_SRC}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
    ${LABEL_K8S_PART_OF}: ${LABEL_K8S_PART_OF_VALUE}
spec:
  volumeSnapshotClassName: ${SNAPSHOT_CLASS}
  source:
    persistentVolumeClaimName: ${SOURCE_PVC}
EOF

  # shellcheck disable=SC2181
  if [[ $? -ne 0 ]]; then
    echolog "${RED_BOLD} ${CROSS} Error creating volume snapshot from unmounted source pvc${NC}\n"
    return ${err_status}
  fi

  while true; do
    if [[ ${retries} -eq 0 ]]; then
      echolog "${RED_BOLD} ${CROSS} Volume snapshot from source pvc not readyToUse (waited 150 sec)${NC}\n"
      return ${err_status}
    fi
    # shellcheck disable=SC2143
    if [[ $(kubectl get volumesnapshot "${UNMOUNTED_VOLUME_SNAP_SRC}" -o yaml | grep 'readyToUse: true') ]]; then
      echolog "${GREEN} ${CHECK} Created volume snapshot from unmounted source pvc and is readyToUse${NC}\n"
      break
    else
      echo "Waiting for Volume snapshot from unmounted pvc be become 'readyToUse:true'" >>"${LOG_FILE}" 2>&1
      sleep "${sleep}"
      ((retries--))
      continue
    fi
  done

  echolog "${BROWN} Creating restore pod from volume snapshot of unmounted pv${NC}\n"

  # shellcheck disable=SC2129
  cat <<EOF | kubectl apply -f - >>"${LOG_FILE}" 2>&1
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: ${UNMOUNTED_RESTORE_PVC}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
    ${LABEL_K8S_PART_OF}: ${LABEL_K8S_PART_OF_VALUE}
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: ${STORAGE_CLASS}
  resources:
    requests:
      storage: 1Gi
  dataSource:
    kind: VolumeSnapshot
    name: ${UNMOUNTED_VOLUME_SNAP_SRC}
    apiGroup: snapshot.storage.k8s.io
---
apiVersion: v1
kind: Pod
metadata:
  name: ${UNMOUNTED_RESTORE_POD}
  labels:
    trilio: tvk-preflight
    preflight-run: ${RANDOM_STRING}
    ${LABEL_K8S_PART_OF}: ${LABEL_K8S_PART_OF_VALUE}
spec:
  imagePullSecrets:
    - name: ${IMAGE_PULL_SECRET}
  serviceAccountName: ${SERVICE_ACCOUNT_NAME}
  containers:
  - name: busybox
    image: ${IMG_PATH}
    args:
    - sleep
    - "3600"
    resources:
      requests:
        memory: "64Mi"
        cpu: "250m"
      limits:
        memory: "128Mi"
        cpu: "500m"
    volumeMounts:
    - name: source-data
      mountPath: /demo/data
  volumes:
  - name: source-data
    persistentVolumeClaim:
      claimName: ${UNMOUNTED_RESTORE_PVC}
      readOnly: false
EOF

  echo "Waiting for restore pod ${UNMOUNTED_RESTORE_POD} from volume snapshot of unmounted pv to become 'Ready'" >>"${LOG_FILE}" 2>&1
  kubectl wait --for=condition=ready --timeout=3m pod/"${UNMOUNTED_RESTORE_POD}" >>"${LOG_FILE}" 2>&1
  # shellcheck disable=SC2181
  if [[ $? -eq 0 ]]; then
    echolog "${GREEN} ${CHECK} Created restore pod from volume snapshot of unmounted pv${NC}\n"
  else
    echolog "${RED_BOLD} ${CROSS} Error creating pod and pvc from volume snapshot of unmounted pv${NC}\n"
    return ${err_status}
  fi

  exec_status=1
  n=0
  until [ "$n" -ge 3 ]; do
    kubectl exec -it "${UNMOUNTED_RESTORE_POD}" -- ls /demo/data/sample-file.txt >>"${LOG_FILE}" 2>&1
    # shellcheck disable=SC2181
    if [ $? -eq 0 ]; then
      exec_status=0
      break
    else
      echo "Retrying exec to check data from Restored pod ${UNMOUNTED_RESTORE_POD} from volume snapshot of unmounted pv" >>"${LOG_FILE}" 2>&1
      n=$((n + 1))
      sleep 2
    fi
  done

  # shellcheck disable=SC2181
  if [[ $exec_status -eq 0 ]]; then
    echolog "${GREEN} ${CHECK} Restored pod from volume snapshot of unmounted pv has expected data${NC}\n"
  else
    echolog "${RED_BOLD} ${CROSS} Restored pod from volume snapshot of unmounted pv does not have expected data${NC}\n"
    return ${err_status}
  fi

  return ${success_status}
}

cleanup() {
  local exit_status=0

  echolog "${LIGHT_BLUE} Cleaning up residual resources...${NC}\n"

  declare -a pvc=("${SOURCE_PVC}" "${RESTORE_PVC}" "${UNMOUNTED_RESTORE_PVC}")
  for res in "${pvc[@]}"; do
    echo "Cleaning PVC - ${res}" >>"${LOG_FILE}" 2>&1
    kubectl delete pvc -n "${NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s >>"${LOG_FILE}" 2>&1 || true
    kubectl patch pvc -n "${NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' >>"${LOG_FILE}" 2>&1 || true
    echo >>"${LOG_FILE}" 2>&1
  done

  declare -a vsnaps=("${VOLUME_SNAP_SRC}" "${UNMOUNTED_VOLUME_SNAP_SRC}")
  for res in "${vsnaps[@]}"; do
    echo >>"${LOG_FILE}" 2>&1
    echo "Cleaning VolumeSnapshot - ${res}" >>"${LOG_FILE}" 2>&1
    kubectl delete volumesnapshot -n "${NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s >>"${LOG_FILE}" 2>&1 || true
    kubectl patch volumesnapshot -n "${NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' >>"${LOG_FILE}" 2>&1 || true
    echo >>"${LOG_FILE}" 2>&1
  done

  declare -a pods=("${SOURCE_POD}" "${RESTORE_POD}" "${UNMOUNTED_RESTORE_POD}")
  for res in "${pods[@]}"; do
    echo >>"${LOG_FILE}" 2>&1
    echo "Deleting pod - ${res}" >>"${LOG_FILE}" 2>&1
    kubectl delete po -n "${NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s >>"${LOG_FILE}" 2>&1 || true
    kubectl patch po -n "${NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' >>"${LOG_FILE}" 2>&1 || true
    echo >>"${LOG_FILE}" 2>&1
  done

  echo "Cleaning all resources related to label - preflight-run:" "${RANDOM_STRING}" >>"${LOG_FILE}" 2>&1
  kubectl delete all -l preflight-run="${RANDOM_STRING}" --force --grace-period=0 --timeout=5s >>"${LOG_FILE}" 2>&1 || true

  echolog "${GREEN} ${CHECK} Cleaned up all the resources${NC}\n"

  sed -i -r "s/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[m|K]//g" "${LOG_FILE}" || true

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
echolog "${GREEN_BOLD}--- Running Pre-flight Checks ---${NC}\n"
echolog "${BROWN}Writing logs to file -> ${LOG_FILE}${NC}\n"
echolog "${BROWN}Might take a few minutes to execute all Pre-flight Checks...${NC}\n"

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
  echolog "${LIGHT_BLUE}Skipping 'VOLUME_SNAPSHOT' check as 'STORAGE_SNAPSHOT_CLASS' preflight check failed${NC}\n"
fi

#Print status of Pre-flight checks
if [ $PREFLIGHT_RUN_SUCCESS == "true" ]; then
  echolog "${GREEN_BOLD}All Pre-flight Checks Succeeded!${NC}\n"
else
  echolog "${RED_BOLD}Some Pre-flight Checks Failed!${NC}\n"
  exit 1
fi
