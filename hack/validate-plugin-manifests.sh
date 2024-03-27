#!/usr/bin/env bash

set -euo pipefail

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"

# install plugin validate-krew-manifest
export GOBIN=$HOME/bin
if ! [[ -x "$GOBIN/validate-krew-manifest" ]]; then
  go install sigs.k8s.io/krew/cmd/validate-krew-manifest@v0.4.4
fi

# validate plugin manifests
for file in "$SRC_ROOT"/plugins/*; do
  "$GOBIN"/validate-krew-manifest -manifest "$file"
  echo >&2 "Successfully validated plugin manifest $file"
done
unset GOBIN
