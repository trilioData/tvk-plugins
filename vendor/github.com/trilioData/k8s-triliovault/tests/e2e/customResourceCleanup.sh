#! /bin/bash

NAMESPACE=$1
UNIQUE_ID=$2
RESOURCE_YAML=$3
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck disable=SC2207
res_list=($(kubectl get pvc -n "${NAMESPACE}" | awk 'FNR > 1 {print $1}'))
for res in "${res_list[@]}"; do
  echo "Deleting pvcs "
  kubectl delete pvc -n "${NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s || true
  kubectl patch pvc -n "${NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true
done

kubectl delete -f "${SRC_DIR}"/../../test-data/CustomResource/"${RESOURCE_YAML}" -n "${NAMESPACE}"
kubectl delete all -l triliobackupall="${UNIQUE_ID}" -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l app=nginx-rc -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l app=nginx-rs -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l name=fluentd-elasticsearch -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l app=nginx-sts -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l app=nginx-deployment -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l app=nginx-rs -n "${NAMESPACE}" --force --grace-period=0 || true
