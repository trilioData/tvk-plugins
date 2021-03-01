#! /bin/bash

UNIQUE_ID=$1
NAMESPACE=$2
NAMESPACE=${NAMESPACE:=default}

SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "unlabeling the ${NAMESPACE} namespace"
kubectl label ns "${NAMESPACE}" trilio-label-


all_releases=("$(helm ls -n "${NAMESPACE}" | awk 'FNR > 1 {print $1}')")
for rel in "${all_releases[@]}"; do
  echo "found release ${rel}"
  trilio_rel="triliovault"
  if [[ "${rel}" == *"${trilio_rel}"* ]]; then
    continue
  fi
  echo "Deleting helm release ${rel}"
  helm delete -n "${NAMESPACE}" "${rel}" || true
done

all_res=("pvc" "backup" "restore" "backupplan" "rc" "daemonset" "networkpolicy" "jobs")
for r in "${all_res[@]}"; do
  # shellcheck disable=SC2207
  res_list=($(kubectl get "${r}" -n "${NAMESPACE}" | awk 'FNR > 1 {print $1}'))
  for res in "${res_list[@]}"; do
    echo "Deleting ${r}"
    kubectl delete "${r}" -n "${NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s || true
    kubectl patch "${r}" -n "${NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true
  done
done

kubectl delete target --all -n "${NAMESPACE}"

kubectl delete policy --all -n "${NAMESPACE}"

kubectl delete clusterrole "${UNIQUE_ID}"-mysql-operator "${UNIQUE_ID}"-helm-operator

kubectl delete clusterrolebinding "${UNIQUE_ID}"-mysql-operator "${UNIQUE_ID}"-helm-operator

kubectl delete jobs --all -n "${NAMESPACE}" --force --grace-period=0

res_list=("$(kubectl get jobs -n "${NAMESPACE}" | awk 'FNR > 1 {print $1}')")
for res in "${res_list[@]}"; do
  echo "Deleting jobs "
  kubectl delete jobs -n "${NAMESPACE}" "${res}" --force --grace-period=0 --timeout=5s || true
  kubectl patch jobs -n "${NAMESPACE}" "${res}" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true
done

echo "Deleting mysqlcluster"
kubectl delete mysqlcluster -n "${NAMESPACE}" "${UNIQUE_ID}"-sample-mysqlcluster --force --grace-period=0 --timeout=5s || true
kubectl patch mysqlcluster -n "${NAMESPACE}" "${UNIQUE_ID}"-sample-mysqlcluster --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true

echo "Deleting helmrelease"
kubectl delete helmrelease -n "${NAMESPACE}" "${UNIQUE_ID}"-redis --force --grace-period=0 --timeout=5s || true
kubectl patch helmrelease -n "${NAMESPACE}" "${UNIQUE_ID}"-redis --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true

kubectl delete sa "${UNIQUE_ID}"-mysql-operator "${UNIQUE_ID}"-helm-operator -n "${NAMESPACE}" --force --grace-period=0

kubectl delete secret redis-auth "${UNIQUE_ID}"-helm-operator-git-deploy "${UNIQUE_ID}"-sample-mysql-cluster-secret "${UNIQUE_ID}"-mysql-operator-orc -n "${NAMESPACE}" --force --grace-period=0

kubectl delete statefulset.apps/"${UNIQUE_ID}"-mysql-operator -n "${NAMESPACE}" --force --grace-period=0

kubectl delete deployment.apps/"${UNIQUE_ID}"-helm-operator -n "${NAMESPACE}" --force --grace-period=0

kubectl delete svc "${UNIQUE_ID}"-helm-operator "${UNIQUE_ID}"-mysql "${UNIQUE_ID}"-mysql-operator-0-svc "${UNIQUE_ID}"-mysql-operator -n "${NAMESPACE}" --force --grace-period=0

kubectl delete cm "${UNIQUE_ID}"-helm-operator-kube-config "${UNIQUE_ID}"-mysql-operator-orc -n "${NAMESPACE}" --force --grace-period=0

kubectl delete -f "${SRC_DIR}"/../../test-data/CustomResource/customResourceWithPVC.yaml -n "${NAMESPACE}" --force --grace-period=0

kubectl delete all -l triliobackupall="${UNIQUE_ID}" -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l app=nginx-rc -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l app=nginx-rs -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l name=fluentd-elasticsearch -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l app=nginx-sts -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l app=nginx-deployment -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete all -l release=mysql -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete po "${UNIQUE_ID}"-mysql-operator-0 -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete po "${UNIQUE_ID}"-mysql-operator-1 -n "${NAMESPACE}" --force --grace-period=0 || true
kubectl delete po "${UNIQUE_ID}"-sample-mysqlcluster-mysql-0 -n "${NAMESPACE}" --force --grace-period=0 || true

echo "labeled back the ${NAMESPACE} namespace"
kubectl label namespace "${NAMESPACE}" trilio-label="${NAMESPACE}" || true
