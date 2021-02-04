#!/usr/bin/env bash

set -euo pipefail

install_kubectl_if_needed() {
  if hash kubectl 2>/dev/null; then
    echo >&2 "using kubectl from the host system and not reinstalling"
  else
    local bin_dir
    bin_dir="$(go env GOPATH)/bin"
    local -r kubectl_version='v1.18.8'
    local -r kubectl_path="${bin_dir}/kubectl"
    local goos goarch kubectl_url
    goos="$(go env GOOS)"
    goarch="$(go env GOARCH)"
    kubectl_url="https://storage.googleapis.com/kubernetes-release/release/${kubectl_version}/bin/${goos}/${goarch}/kubectl"

    echo >&2 "kubectl not detected in environment, downloading ${kubectl_url}"
    mkdir -p "${bin_dir}"
    curl --fail --show-error --silent --location --output "$kubectl_path" "${kubectl_url}"
    chmod +x "$kubectl_path"
    echo >&2 "installed kubectl to ${kubectl_path}"
  fi
}

install_helm_if_needed() {

  if hash helm 2>/dev/null; then
    echo >&2 "using helm from the host system and not reinstalling"
  else
    echo >&2 "helm not detected in environment, installing..."

    local -r HELM_VERSION="v3.5.1"
    curl -Lo /tmp/helm-linux-amd64.tar.gz https://storage.googleapis.com/kubernetes-helm/helm-${HELM_VERSION}-linux-amd64.tar.gz
    tar -xvf /tmp/helm-linux-amd64.tar.gz -C /tmp/
    chmod +x /tmp/linux-amd64/helm && sudo mv /tmp/linux-amd64/helm /usr/local/bin/
    helm init --client-only
    helm version
    echo >&2 "installed helm"
  fi
}

install_krew_if_needed() {
  if hash kubectl krew 2>/dev/null; then
    echo >&2 "using kew from the host system and not reinstalling"
  else
    set -x
    cd "$(mktemp -d)" &&
      curl -fsSLO "https://github.com/kubernetes-sigs/krew/releases/latest/download/krew.tar.gz" &&
      tar zxvf krew.tar.gz &&
      KREW=./krew-"$(uname | tr '[:upper:]' '[:lower:]')_$(uname -m | sed -e 's/x86_64/amd64/' -e 's/arm.*$/arm/')" &&
      "$KREW" install krew
    sudo mv ~/.krew/bin/kubectl-krew /usr/local/bin/
    kubectl krew
  fi
}

install_kubectl_if_needed
install_helm_if_needed
install_krew_if_needed
