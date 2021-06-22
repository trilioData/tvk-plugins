#!/bin/bash
set -ex
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "Installing application in namespace - ${INSTALL_NAMESPACE}"

export INSTALL_NAMESPACE="triliovault-integration"
kubectl create namespace "${INSTALL_NAMESPACE}"
echo "Add the Trilio Helm repository"
helm repo add triliovault-operator http://charts.k8strilio.net/trilio-stable/k8s-triliovault-operator
helm repo add triliovault http://charts.k8strilio.net/trilio-stable/k8s-triliovault
helm repo update
echo "Install TrilioVault operator helm chart"
helm install  tvm --wait triliovault-operator/k8s-triliovault-operator --version=v2.1.0 --namespace "${INSTALL_NAMESPACE}"
echo "List TrilioVault operator helm release name"
helm list

echo "Verify TrilioVault operator pods are running"
kubectl get pods -l release=triliovault-operator

echo "Install TVK "
kubectl apply -f "${SRC_DIR}/tests/e2e/test-data/triliovault-manager.yaml/" --namespace "${INSTALL_NAMESPACE}"


# NOTE: need sleep for resources to start k8s-triliovault-admission-webhook
sleep 60

echo "Verify TrilioVaultManager CR are running"
kubectl get pods --namespace "${INSTALL_NAMESPACE}"

echo "Create Target"
kubectl apply -f "${SRC_DIR}/tests/e2e/test-data/target.yaml/" --namespace "${INSTALL_NAMESPACE}"
# NOTE: need sleep for target to come available state
sleep 30
echo "Verify target is in available state"
kubectl get target --namespace "${INSTALL_NAMESPACE}"
