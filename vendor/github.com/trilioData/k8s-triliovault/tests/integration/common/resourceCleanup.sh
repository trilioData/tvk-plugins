#!/bin/bash

RESOURCE=$1
NAMESPACE=$2

# shellcheck disable=SC2207
res_list=($(kubectl get "${RESOURCE}" -n "${NAMESPACE}" | awk 'FNR > 1 {print $1}'))
for res in "${res_list[@]}"; do
    echo "Deleting ${RESOURCE} ${res}"
    kubectl delete "${RESOURCE}" "${res}" -n "${NAMESPACE}" --force --grace-period=0 --timeout=5s || true
    kubectl patch "${RESOURCE}" "${res}" -n "${NAMESPACE}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true
done
