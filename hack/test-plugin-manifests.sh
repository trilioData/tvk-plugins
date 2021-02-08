#!/usr/bin/env bash

set -euo pipefail

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"

# install plugin validate-krew-manifest
export GOBIN=$HOME/bin
go get sigs.k8s.io/krew/cmd/validate-krew-manifest@master
unset GOBIN

# validate plugin manifests
for file in "$SRC_ROOT"/plugins/*; do
  "$HOME"/bin/validate-krew-manifest -manifest "$file"
  echo >&2 "Successfully validated plugin manifest $file"
done
