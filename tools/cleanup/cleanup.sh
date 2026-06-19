#!/bin/bash

# Cleans up all Trilio resources from the k8s cluster
# IMP - Runs on the sourced kubeconfig (refers to .kube/config)
# Please make sure that correct kubeconfig is used or sourced
# Deleted resources can not be recovered

CLEANUP_RUN_SUCCESS=true

is_kubectl_connectivity_error() {
  echo "$1" | grep -qiE 'unable to connect to the server|connection refused|connection timed out|no route to host|network is unreachable|i/o timeout|context deadline exceeded|tls: |x509: |EOF|net/http: request canceled|error dialing|dial tcp|the kubernetes api server|couldn'\''t get current server API group list'
}

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

# Returns 0 if deleted, 1 if still exists, 2 on connectivity error
verify_resource_deleted() {
  local resource=$1
  local name=$2
  local ns=${3:-}
  local output
  local ret

  if [ -n "${ns}" ]; then
    output=$(kubectl get "${resource}" "${name}" -n "${ns}" 2>&1)
  else
    output=$(kubectl get "${resource}" "${name}" 2>&1)
  fi
  ret=$?
  if [ "${ret}" -eq 0 ]; then
    return 1
  fi
  if is_kubectl_connectivity_error "${output}"; then
    echo "Failed to verify deletion of ${resource} ${name}: cluster connectivity error"
    echo "${output}"
    return 2
  fi
  return 0
}

handle_delete_with_finalizer_fallback() {
  local resource=$1
  local name=$2
  local ns=$3
  local exit_status_var=$4
  local patch_args=()
  local verify_ret
  local patch_output
  local patch_ret

  if [ -n "${ns}" ]; then
    patch_args=(-n "${ns}")
  fi

  echo "Failed deleting ${resource} ${name}${ns:+ in namespace ${ns}}"
  echo "Patching ${resource} ${name}${ns:+ in ${ns}}"
  patch_output=$(kubectl patch "${resource}" "${name}" -p '{"metadata":{"finalizers":[]}}' --type=merge "${patch_args[@]}" 2>&1)
  patch_ret=$?
  if [ "${patch_ret}" -ne 0 ] && is_kubectl_connectivity_error "${patch_output}"; then
    echo "Failed to patch ${resource} ${name}: cluster connectivity error"
    echo "${patch_output}"
    eval "${exit_status_var}=1"
    return
  fi

  verify_resource_deleted "${resource}" "${name}" "${ns}"
  verify_ret=$?
  if [ "${verify_ret}" -eq 1 ]; then
    echo "Failed deleting ${resource} ${name}${ns:+ in ${ns}}"
    eval "${exit_status_var}=1"
  elif [ "${verify_ret}" -eq 2 ]; then
    eval "${exit_status_var}=1"
  else
    echo "Deleted ${resource} ${name}${ns:+ in ${ns}}"
  fi
}

check_if_ocp() {
  # Check if the k8s cluster is upstream or OCP
  local is_ocp="False"
  local output
  local ret

  output=$(kubectl api-resources 2>&1)
  ret=$?
  if [ "${ret}" -ne 0 ]; then
    if is_kubectl_connectivity_error "${output}"; then
      echo "Failed to detect cluster type: ${output}" >&2
      echo "Error"
      return
    fi
  fi
  if echo "${output}" | grep -q openshift.io; then
    is_ocp="True"
  fi
  echo "${is_ocp}"
}

delete_tvk_res() {
  # Same for OCP & Upstream
  # Check all the namespaces for restores, delete the restores
  local exit_status=0
  local get_output
  local get_ret
  local retValue

  for res in ${TVK_resources}; do
    get_output=$(kubectl get "${res}" -A --no-headers 2>&1)
    get_ret=$?
    if [ "${get_ret}" -ne 0 ]; then
      if is_kubectl_connectivity_error "${get_output}"; then
        echo "Failed to list ${res}: cluster connectivity error"
        echo "${get_output}"
        exit_status=1
        continue
      fi
      echo "Resource ${res} does not exist on the cluster"
      echo
      continue
    fi

    if [ -z "${get_output}" ]; then
      echo "No ${res} resources found on the cluster"
      echo
      continue
    fi

    # Fetch non-deuplicate namespace for the given resource
    for ns in $(echo "${get_output}" | awk '{print $1}' | uniq); do
      local ns_output
      local ns_ret

      ns_output=$(kubectl get "${res}" -n "${ns}" --no-headers 2>&1)
      ns_ret=$?
      if [ "${ns_ret}" -ne 0 ]; then
        if is_kubectl_connectivity_error "${ns_output}"; then
          echo "Failed to list ${res} in namespace ${ns}: cluster connectivity error"
          echo "${ns_output}"
          exit_status=1
          continue
        fi
        continue
      fi

      # Fetch given resource name
      for name in $(echo "${ns_output}" | awk '{print $1}' | uniq); do
        # Delete
        echo "Deleting ${res} ${name} in namespace ${ns} "
        kubectl delete "${res}" "${name}" --force --grace-period=0 --timeout=5s -n "${ns}"
        retValue=$?
        if [ "${retValue}" -ne 0 ]; then
          handle_delete_with_finalizer_fallback "${res}" "${name}" "${ns}" exit_status
        fi
      done
    done
  done
  return ${exit_status}
}

delete_tvk_op() {
  # Check if the k8s cluster is upstream or OCP
  local exit_status=0
  local ocp_result
  local sub_output
  local sub_ret
  local retValue
  local helm_output
  local helm_ret
  local tvm_output
  local tvm_ret
  local cron_output
  local cron_ret

  ocp_result=$(check_if_ocp)
  if [[ "${ocp_result}" == "Error" ]]; then
    exit_status=1
    return ${exit_status}
  fi

  if [[ "${ocp_result}" == "True" ]]; then
    echo "This is OCP Cluster"
    # Delete k8s-triliovault operator
    sub_output=$(kubectl get subscription k8s-triliovault -n openshift-operators 2>&1)
    sub_ret=$?
    if [ "${sub_ret}" -eq 0 ]; then
      echo "Uninstalling k8s-triliovault operator"
      kubectl delete subscription k8s-triliovault --force --grace-period=0 --timeout=5s -n openshift-operators
      retValue=$?
      if [ "${retValue}" -ne 0 ]; then
        handle_delete_with_finalizer_fallback "subscription" "k8s-triliovault" "openshift-operators" exit_status
      fi
    elif is_kubectl_connectivity_error "${sub_output}"; then
      echo "Failed to check k8s-triliovault subscription: cluster connectivity error"
      echo "${sub_output}"
      exit_status=1
    fi

    # Delete k8s-triliovault clusterserviceversion
    local csv_output
    local tvkcsversion

    csv_output=$(kubectl get clusterserviceversion --no-headers -n openshift-operators 2>&1)
    csv_ret=$?
    if [ "${csv_ret}" -ne 0 ]; then
      if is_kubectl_connectivity_error "${csv_output}"; then
        echo "Failed to list clusterserviceversions: cluster connectivity error"
        echo "${csv_output}"
        exit_status=1
      fi
    else
      tvkcsversion=$(echo "${csv_output}" | grep k8s-triliovault | awk '{print $1}')
      if [ -n "${tvkcsversion}" ]; then
        echo "Deleting k8s-triliovault clusterserviceversion"
        kubectl delete clusterserviceversion "${tvkcsversion}" --force --grace-period=0 --timeout=5s -n openshift-operators
        retValue=$?
        if [ "${retValue}" -ne 0 ]; then
          handle_delete_with_finalizer_fallback "clusterserviceversion" "${tvkcsversion}" "openshift-operators" exit_status
        fi
      fi
    fi

  fi

  # For Upstream OR in case if TVK installed on OCP using "helm"
  # Delete Triliovault-manager and Triliovault-operator using helm/label
  # Fetch non-deuplicate namespace
  helm_output=$(helm list -A 2>&1)
  helm_ret=$?
  if [ "${helm_ret}" -ne 0 ]; then
    echo "Failed to list helm releases: ${helm_output}"
    exit_status=1
    return ${exit_status}
  fi

  local tvm_ns
  tvm_ns=$(echo "${helm_output}" | grep -v REVISION | grep 'triliovault' | awk '{print $2}' | uniq)
  if [ -n "${tvm_ns}" ]; then
    for ns in ${tvm_ns}; do
      local tvm_name
      local tvm
      local tvo

      # Deleting Trilivault-manager CR
      tvm_output=$(kubectl get triliovaultmanager --no-headers -n "${ns}" 2>&1)
      tvm_ret=$?
      if [ "${tvm_ret}" -eq 0 ]; then
        tvm_name=$(echo "${tvm_output}" | awk '{print $1}')
        if [ -n "${tvm_name}" ]; then
          echo "Deleting triliovaultmanager CR ${tvm_name} in namespace ${ns}"
          kubectl delete triliovaultmanager "${tvm_name}" --force --grace-period=0 --timeout=5s -n "${ns}"
          retValue=$?
          if [ "${retValue}" -ne 0 ]; then
            handle_delete_with_finalizer_fallback "triliovaultmanager" "${tvm_name}" "${ns}" exit_status
          fi
        fi
      elif is_kubectl_connectivity_error "${tvm_output}"; then
        echo "Failed to get triliovaultmanager in namespace ${ns}: cluster connectivity error"
        echo "${tvm_output}"
        exit_status=1
      fi

      # Uninstall Trilivault-manager
      local helm_ns_output
      local helm_ns_ret

      helm_ns_output=$(helm list -n "${ns}" 2>&1)
      helm_ns_ret=$?
      if [ "${helm_ns_ret}" -ne 0 ]; then
        echo "Failed to list helm releases in namespace ${ns}: ${helm_ns_output}"
        exit_status=1
        continue
      fi
      tvm=$(echo "${helm_ns_output}" | grep -v REVISION | grep 'triliovault-[0-9]' | awk '{print $1}')
      if [ -n "${tvm}" ]; then
        echo "Uninstalling Trilivault-manager helm chart in namespace ${ns}"
        helm uninstall "${tvm}" -n "${ns}"
        retValue=$?
        if [ "${retValue}" -ne 0 ]; then
          echo "Failed uninstalling Trilivault-manager helm chart in namespace ${ns}"
          exit_status=1
        fi
      fi

      # Uninstall Trilivault-operator
      helm_ns_output=$(helm list -n "${ns}" 2>&1)
      helm_ns_ret=$?
      if [ "${helm_ns_ret}" -ne 0 ]; then
        echo "Failed to list helm releases in namespace ${ns}: ${helm_ns_output}"
        exit_status=1
        continue
      fi
      tvo=$(echo "${helm_ns_output}" | grep -v REVISION | grep triliovault-o | awk '{print $1}')
      if [ -n "${tvo}" ]; then
        echo "Uninstalling Trilivault-operator in namespace ${ns}"
        helm uninstall "${tvo}" -n "${ns}"
        retValue=$?
        if [ "${retValue}" -ne 0 ]; then
          echo "Failed uninstalling Trilivault-operator in namespace ${ns}"
          exit_status=1
        fi
      fi

      # Delete k8s-triliovault-resource-cleaner cronjob
      local tvkcron

      cron_output=$(kubectl get cronjob --no-headers -n "${ns}" 2>&1)
      cron_ret=$?
      if [ "${cron_ret}" -eq 0 ]; then
        tvkcron=$(echo "${cron_output}" | grep k8s-triliovault | awk '{print $1}')
        if [ -n "${tvkcron}" ]; then
          echo "Deleting k8s-triliovault-resource-cleaner cronjob in namespace ${ns}"
          kubectl delete cronjob "${tvkcron}" --force --grace-period=0 --timeout=5s -n "${ns}"
          retValue=$?
          if [ "${retValue}" -ne 0 ]; then
            handle_delete_with_finalizer_fallback "cronjob" "${tvkcron}" "${ns}" exit_status
          fi
        fi
      elif is_kubectl_connectivity_error "${cron_output}"; then
        echo "Failed to list cronjobs in namespace ${ns}: cluster connectivity error"
        echo "${cron_output}"
        exit_status=1
      fi
    done
  fi

  return ${exit_status}
}

delete_tvk_crd() {
  # Same for OCP & Upstream
  # Delete Triliovault CRDs
  local exit_status=0
  local crd_output
  local crd_ret
  local retValue

  crd_output=$(kubectl get crd --no-headers 2>&1)
  crd_ret=$?
  if [ "${crd_ret}" -ne 0 ]; then
    if is_kubectl_connectivity_error "${crd_output}"; then
      echo "Failed to list CRDs: cluster connectivity error"
      echo "${crd_output}"
      return 1
    fi
    echo "No CRDs found on the cluster"
    return 0
  fi

  for tvkcrd in $(echo "${crd_output}" | grep triliovault | awk '{print $1}'); do
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
      export TVK_resources="ClusterRestore ClusterBackup ClusterBackupPlan Restore Backup Backupplan Hook ClusterHook Target ClusterTarget Policy ClusterPolicy License"
      echo "No resources specified, will be deleting all resources listed below"
      echo "ClusterRestore ClusterBackup ClusterBackupPlan Restore Backup Backupplan Hook ClusterHook Target ClusterTarget Policy ClusterPolicy License"
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
