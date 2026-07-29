#!/bin/bash

# Cleans up all Trilio resources from the k8s cluster
# IMP - Runs on the sourced kubeconfig (refers to .kube/config)
# Please make sure that correct kubeconfig is used or sourced
# Deleted resources can not be recovered

CLEANUP_RUN_SUCCESS=true

check_cluster_connectivity() {
  local output
  output=$(kubectl cluster-info --request-timeout=10s 2>&1)
  local ret=$?
  if [ "${ret}" -ne 0 ]; then
    echo "Cluster connectivity check failed: ${output}"
    return 1
  fi
  echo "Cluster connectivity check passed"
  return 0
}

check_helm_connectivity() {
  local output
  if ! command -v helm >/dev/null 2>&1; then
    echo "helm is not installed but is required for TVM/operator cleanup (-t flag)"
    return 1
  fi
  output=$(helm list -A 2>&1)
  local ret=$?
  if [ "${ret}" -ne 0 ]; then
    echo "Helm connectivity check failed: ${output}"
    return 1
  fi
  echo "Helm connectivity check passed"
  return 0
}

handle_delete_with_finalizer_fallback() {
  local resource=$1
  local name=$2
  local ns=$3
  local exit_status_var=$4
  local patch_args=()

  if [ -n "${ns}" ]; then
    patch_args=(-n "${ns}")
  fi

  echo "Failed deleting ${resource} ${name}${ns:+ in namespace ${ns}}"
  echo "Patching ${resource} ${name}${ns:+ in ${ns}}"
  kubectl patch "${resource}" "${name}" -p '{"metadata":{"finalizers":[]}}' --type=merge "${patch_args[@]}"
  # If delete was previously denied, clearing finalizers alone is not enough — delete again.
  kubectl delete "${resource}" "${name}" --force --grace-period=0 --timeout=5s "${patch_args[@]}" 2>/dev/null
  kubectl wait --for=delete "${resource}/${name}" "${patch_args[@]}" --timeout=10s 2>/dev/null
  if (kubectl get "${resource}" "${name}" "${patch_args[@]}" 2>/dev/null); then
    echo "Failed deleting ${resource} ${name}${ns:+ in ${ns}}"
    eval "${exit_status_var}=1"
  else
    echo "Deleted ${resource} ${name}${ns:+ in ${ns}}"
  fi
}

# Returns 0 if kind is cluster-scoped (no namespace).
is_cluster_scoped_resource() {
  local res=$1
  local namespaced

  namespaced=$(kubectl api-resources --api-group=triliovault.trilio.io --no-headers 2>/dev/null |
    awk -v kind="${res}" 'BEGIN{IGNORECASE=1} tolower($NF)==tolower(kind){print $(NF-1); exit}')

  if [ -n "${namespaced}" ]; then
    [ "${namespaced}" = "false" ]
    return $?
  fi

  # Fallback when api-resources lookup fails (e.g. CRD already gone)
  [[ "${res}" == *Cluster* || "${res}" == "ContinuousRestorePlan" || "${res}" == "ConsistentSet" ]]
}

check_if_ocp() {
  # Check if the k8s cluster is upstream or OCP
  local is_ocp="False"
  if (kubectl api-resources | grep -q openshift.io); then
    is_ocp="True"
  fi
  echo "${is_ocp}"
}

delete_tvk_res() {
  # Same for OCP & Upstream
  # Check all the namespaces for restores, delete the restores
  local exit_status=0
  local retValue

  for res in ${TVK_resources}; do
    if is_cluster_scoped_resource "${res}"; then
      # Cluster-scoped resources have no namespace
      if (kubectl get "${res}" --no-headers 2>/dev/null); then
        for name in $(kubectl get "${res}" --no-headers 2>/dev/null | awk '{print $1}' | uniq); do
          echo "Deleting cluster-scoped ${res} ${name}"
          kubectl delete "${res}" "${name}" --force --grace-period=0 --timeout=5s
          retValue=$?
          if [ "${retValue}" -ne 0 ]; then
            handle_delete_with_finalizer_fallback "${res}" "${name}" "" exit_status
          fi
        done
      else
        echo "Resource ${res} does not exist on the cluster"
        echo
      fi
    else
      # Namespaced resources
      if (kubectl get "${res}" -A --no-headers 2>/dev/null); then
        # Fetch non-duplicate namespace for the given resource
        for ns in $(kubectl get "${res}" -A --no-headers 2>/dev/null | awk '{print $1}' | uniq); do
          # Fetch given resource name
          for name in $(kubectl get "${res}" -n "${ns}" --no-headers 2>/dev/null | awk '{print $1}' | uniq); do
            echo "Deleting ${res} ${name} in namespace ${ns} "
            if [ "${res}" == "FileRecoveryVM" ]; then
              kubectl patch "${res}" "${name}" -p '{"metadata":{"annotations":{"triliovault.trilio.io/request-for-delete":"true"}}}' --type=merge -n "${ns}"
              retValue=$?
              if [ "${retValue}" -ne 0 ]; then
                echo "Failed to patch FileRecoveryVM ${name} in namespace ${ns}"
                exit_status=1
              fi
            fi
            kubectl delete "${res}" "${name}" --force --grace-period=0 --timeout=5s -n "${ns}"
            retValue=$?
            if [ "${retValue}" -ne 0 ]; then
              handle_delete_with_finalizer_fallback "${res}" "${name}" "${ns}" exit_status
            fi
          done
        done
      else
        echo "Resource ${res} does not exist on the cluster"
        echo
      fi
    fi
  done
  return ${exit_status}
}

delete_tvk_op() {
  # Check if the k8s cluster is upstream or OCP
  local exit_status=0
  local retValue
  local tvkcsversion

  if [[ $(check_if_ocp) == "True" ]]; then
    echo "This is OCP Cluster"
    # Delete k8s-triliovault operator
    if (kubectl get subscription k8s-triliovault -n openshift-operators >/dev/null 2>&1); then
      echo "Uninstalling k8s-triliovault operator"
      kubectl delete subscription k8s-triliovault --force --grace-period=0 --timeout=5s -n openshift-operators
      retValue=$?
      if [ "${retValue}" -ne 0 ]; then
        handle_delete_with_finalizer_fallback "subscription" "k8s-triliovault" "openshift-operators" exit_status
      fi
    fi

    # Delete k8s-triliovault clusterserviceversion
    tvkcsversion=$(kubectl get clusterserviceversion --no-headers -n openshift-operators 2>/dev/null | grep k8s-triliovault | awk '{print $1}')
    if [ -n "${tvkcsversion}" ]; then
      echo "Deleting k8s-triliovault clusterserviceversion"
      kubectl delete clusterserviceversion "${tvkcsversion}" --force --grace-period=0 --timeout=5s -n openshift-operators
      retValue=$?
      if [ "${retValue}" -ne 0 ]; then
        handle_delete_with_finalizer_fallback "clusterserviceversion" "${tvkcsversion}" "openshift-operators" exit_status
      fi
    fi

  fi

  # For Upstream OR in case if TVK installed on OCP using "helm"
  # Delete Triliovault-manager and Triliovault-operator using helm/label
  # Fetch non-deuplicate namespace
  local tvm_ns
  tvm_ns=$(helm list -A | grep -v REVISION | grep 'triliovault' | awk '{print $2}' | uniq)
  if [ -n "${tvm_ns}" ]; then
    for ns in ${tvm_ns}; do
      local tvm_name
      local tvm
      local tvo
      local tvkcron

      # Deleting Trilivault-manager CR
      tvm_name=$(kubectl get triliovaultmanager --no-headers -n "${ns}" 2>/dev/null | awk '{print $1}')
      if [ -n "${tvm_name}" ]; then
        echo "Deleting triliovaultmanager CR ${tvm_name} in namespace ${ns}"
        kubectl delete triliovaultmanager "${tvm_name}" --force --grace-period=0 --timeout=5s -n "${ns}"
        retValue=$?
        if [ "${retValue}" -ne 0 ]; then
          handle_delete_with_finalizer_fallback "triliovaultmanager" "${tvm_name}" "${ns}" exit_status
        fi
      fi

      # Uninstall Trilivault-manager
      tvm=$(helm list -n "${ns}" | grep -v REVISION | grep 'triliovault-[0-9]' | awk '{print $1}')
      if [ -n "${tvm}" ]; then
        echo "Uninstalling Trilivault-manager helm chart in namespace ${ns}"
        helm uninstall "${tvm}" -n "${ns}"
        retValue=$?
        if [ "${retValue}" -ne 0 ]; then
          exit_status=1
        fi
      fi

      # Uninstall Trilivault-operator
      tvo=$(helm list -n "${ns}" | grep -v REVISION | grep triliovault-o | awk '{print $1}')
      if [ -n "${tvo}" ]; then
        echo "Uninstalling Trilivault-operator in namespace ${ns}"
        helm uninstall "${tvo}" -n "${ns}"
        retValue=$?
        if [ "${retValue}" -ne 0 ]; then
          exit_status=1
        fi
      fi

      # Delete k8s-triliovault-resource-cleaner cronjob
      tvkcron=$(kubectl get cronjob --no-headers -n "${ns}" 2>/dev/null | grep k8s-triliovault | awk '{print $1}')
      if [ -n "${tvkcron}" ]; then
        echo "Deleting k8s-triliovault-resource-cleaner cronjob in namespace ${ns}"
        kubectl delete cronjob "${tvkcron}" --force --grace-period=0 --timeout=5s -n "${ns}"
        retValue=$?
        if [ "${retValue}" -ne 0 ]; then
          handle_delete_with_finalizer_fallback "cronjob" "${tvkcron}" "${ns}" exit_status
        fi
      fi
    done
  fi

  return ${exit_status}
}

delete_tvk_crd() {
  # Same for OCP & Upstream
  # Delete Triliovault CRDs
  local exit_status=0
  local retValue

  for tvkcrd in $(kubectl get crd --no-headers 2>/dev/null | grep triliovault | awk '{print $1}'); do
    # Delete
    echo "Deleting crd ${tvkcrd}"
    kubectl delete crd "${tvkcrd}" --force --grace-period=0 --timeout=5s
    retValue=$?
    if [ "${retValue}" -ne 0 ]; then
      handle_delete_with_finalizer_fallback "crd" "${tvkcrd}" "" exit_status
    fi
  done
  return ${exit_status}
}

print_usage() {
  echo "
--------------------------------------------------------------
tvk-cleanup - Cleans up Triliovault Custom reources and CRDs
Usage:
kubectl tvk-cleanup [options] [arguments]
Options:
        -h, --help                show brief help
        -n, --noninteractive      run script in non-interactive mode
        -c, --crd                 delete Triliovault CRDs
        -t, --tvm                 delete Triliovault Manager or Operator
        -r, --resources \"resource1 resource2..\"
                                  specify list of Triliovault CRs to delete
                                  If not provided, all Triliovault CRs (listed below) will be deleted
                                  e.g. Restore Backup Backupplan Hook Target Policy License
--------------------------------------------------------------
"
}

for arg in "$@"; do
  if [[ "${arg}" == "--source-only" ]]; then
    return 0 2>/dev/null || exit 0
  fi
done

# Main script starts here
# Check the options provided
if [ $# -eq 0 ]; then
  print_usage
  exit 1
fi

while test $# -gt 0; do
  case "$1" in
  -h | --help)
    print_usage
    exit 0
    ;;
  -n | --noninteractive)
    export Non_interact=True
    echo "Flag set to run cleanup in non-interactive mode"
    echo
    ;;
  -c | --crd)
    export Delete_CRD=True
    echo "Flag set to delete Triliovault CRDs"
    echo
    ;;
  -t | --tvm)
    export Delete_TVM=True
    echo "Flag set to delete Triliovault Manager or Operator"
    echo
    ;;
  -r | --resources)
    shift
    if [[ "$*" == -* || $# -eq 0 ]]; then
      export TVK_resources="ClusterRestore ClusterBackup ClusterSnapshot ClusterBackupPlan Restore Backup Snapshot Backupplan ConsistentSet ContinuousRestorePlan FileRecoveryVM Hook ClusterHook Policy ClusterPolicy License Target ClusterTarget"
      echo "No resources specified, will be deleting all resources listed below"
      echo ${TVK_resources}
      echo
      continue
    else
      export TVK_resources=$1
      echo "Resource list: ${TVK_resources}"
      echo
    fi
    ;;
  *)
    echo "Incorrect option, check usage below..."
    echo
    print_usage
    exit 1
    ;;
  esac
  shift
done

echo "##################### DISCLAIMER ############################"
echo "# This script deletes all the Triliovault Custom Resources, #"
echo "# Triliovault Manager application and CRDs from all the     #"
echo "# namespaces. Once deleted, these can not be recovered.     #"
echo "# Please select the options carefully.                      #"
echo "#############################################################"
echo

if [[ ${Non_interact} != "True" ]]; then
  echo -n "Do you want to continue: y/n? "
  read -r start
  if [[ ${start} != "Y" && ${start} != "y" ]]; then
    echo "Exiting..............................."
    echo
    exit 0
  fi
fi

if [[ -z "${TVK_resources}" && -z "${Delete_TVM}" && -z "${Delete_CRD}" ]]; then
  echo "No resources selected for cleanup, please check usage below"
  print_usage
  exit
fi

echo "Checking cluster connectivity..."
if ! check_cluster_connectivity; then
  echo "Aborting cleanup due to cluster connectivity failure"
  exit 1
fi
echo

if [ "${Delete_TVM}" ]; then
  echo "Checking helm connectivity..."
  if ! check_helm_connectivity; then
    echo "Aborting cleanup due to helm connectivity failure"
    exit 1
  fi
  echo
fi

echo "Starting Cleanup..............................."
echo

if [ -n "${TVK_resources}" ]; then
  echo "Deleting Triliovault resources: "
  echo "${TVK_resources}"
  echo
  # Delete Triliovault resource
  delete_tvk_res
  retValue=$?
  if [ "${retValue}" -ne 0 ]; then
    CLEANUP_RUN_SUCCESS=false
  fi
fi

if [ ${Delete_TVM} ]; then
  echo "Deleting Triliovault Manager or Operator"
  echo
  # Delete Triliovault Manager or Operator
  delete_tvk_op
  retValue=$?
  if [ "${retValue}" -ne 0 ]; then
    CLEANUP_RUN_SUCCESS=false
  fi
fi

if [ ${Delete_CRD} ]; then
  echo "Deleting Triliovault CRDs"
  echo
  # Delete CRDs
  delete_tvk_crd
  retValue=$?
  if [ "${retValue}" -ne 0 ]; then
    CLEANUP_RUN_SUCCESS=false
  fi
fi

# Print status of cleanup
if [ $CLEANUP_RUN_SUCCESS == "true" ]; then
  echo "Cleanup completed successfully!!"
else
  echo "Cleanup failed!!"
  exit 1
fi
