#!/bin/bash

#Usage:
# mysqlOperator.sh delete <release-name> <namespace>
# mysqlOperator.sh upgrade <release-name> <namespace>
# mysqlOperator.sh rollback <release-name> <namespace>
# mysqlOperator.sh install <release-name> <namespace> <extra> <install> <args>

set -ex

CHART_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")/mysql-operator-chart" && pwd)"

if [[ $# -ne 3 ]] && [[ $1 != "install" ]]; then
  echo "Incorrect input params"
  exit 1
else
  RELEASE_NAME=$2
  OPERATOR_NAMESPACE=$3
fi

if [[ $1 == "install" ]] && [[ $# -gt 3 ]]; then
  INSTALL_ARGS=${*:4:$#}
fi

function deleteMysqlCR() {
  res_list=("$(kubectl get mysqlcluster -n "${OPERATOR_NAMESPACE}" | awk 'FNR > 1 {print $1}')")
  for res in "${res_list[@]}"; do
    echo "Deleting mysqlcluster ${res}"
    kubectl delete mysqlcluster -n "${OPERATOR_NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s || true
    kubectl patch mysqlcluster -n "${OPERATOR_NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true
  done
}

# shellcheck disable=SC2086
function install() {
  echo "Installing mysql operator with release name ${RELEASE_NAME} in namespace: ${OPERATOR_NAMESPACE}"
  helm install "${RELEASE_NAME}" -n "${OPERATOR_NAMESPACE}" "${CHART_PATH}" --set orchestrator.persistence.storageClass="${STORAGE_CLASS}" \
      --set operatorNamespace="${OPERATOR_NAMESPACE}" $INSTALL_ARGS
}

function delete() {
  echo "deleting helm ${RELEASE_NAME}"
  deleteMysqlCR

  helm delete "${RELEASE_NAME}" -n "${OPERATOR_NAMESPACE}" || true

  # shellcheck disable=SC2207
  res_list=($(kubectl get pvc -n "${OPERATOR_NAMESPACE}" | awk 'FNR > 1 {print $1}'))
  for res in "${res_list[@]}"; do
    echo "Deleting pvcs "
    kubectl delete pvc -n "${OPERATOR_NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s || true
    kubectl patch pvc -n "${OPERATOR_NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true
  done
}

function upgrade() {
  echo "upgrading helm release"
  helm upgrade "${RELEASE_NAME}" "${CHART_PATH}" -n "${OPERATOR_NAMESPACE}" --set orchestrator.persistence.storageClass="${STORAGE_CLASS}",operatorNamespace="${OPERATOR_NAMESPACE}",replicas=2
}

function rollback() {
  echo "rolling back helm release to previous version"
  helm rollback "${RELEASE_NAME}" -n "${OPERATOR_NAMESPACE}" || true
}

case $1 in
    "install") install
    ;;
    "upgrade") upgrade
    ;;
    "delete") delete
    ;;
    "rollback") rollback
    ;;
esac
