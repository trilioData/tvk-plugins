#!/bin/bash

NAMESPACE=$1
UNIQUE_ID=$2
printf "Deleting operator restored resources in namespace: %s\\n" "${NAMESPACE}"

echo "Deleting mysqlcluster"
kubectl delete mysqlcluster -n "${NAMESPACE}" "${UNIQUE_ID}"-sample-mysqlcluster --force --grace-period=0 --timeout=5s || true
kubectl patch mysqlcluster -n "${NAMESPACE}" "${UNIQUE_ID}"-sample-mysqlcluster --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true

echo "Deleting helmrelease"
kubectl delete helmrelease -n "${NAMESPACE}" "${UNIQUE_ID}"-redis --force --grace-period=0 --timeout=5s || true
kubectl patch helmrelease -n "${NAMESPACE}" "${UNIQUE_ID}"-redis --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true

kubectl delete configmap --all -n "${NAMESPACE}"
kubectl delete sa "${UNIQUE_ID}"-helm-operator "${UNIQUE_ID}"-mysql-operator -n "${NAMESPACE}" || true
kubectl delete secret "${UNIQUE_ID}"-sample-mysql-cluster-secret "${UNIQUE_ID}"-mysql-operator-orc "${UNIQUE_ID}"-helm-operator-git-deploy redis-auth -n "${NAMESPACE}"
#kubectl delete secret --all -n "${NAMESPACE}"
kubectl delete svc --all -n "${NAMESPACE}" || true
kubectl delete sts --all -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete deploy --all -n "${NAMESPACE}" --force --grace-period=0 || true

# shellcheck disable=SC2207
res_list=($(kubectl get pvc -n "${NAMESPACE}" | awk 'FNR > 1 {print $1}'))
for res in "${res_list[@]}"; do
  echo "Deleting pvcs "
  kubectl delete pvc -n "${NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s || true
  kubectl patch pvc -n "${NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true
done

kubectl delete po --all -n "${NAMESPACE}" --force --grace-period=0 || true

printf "Deleted operator restored resources\\n"
