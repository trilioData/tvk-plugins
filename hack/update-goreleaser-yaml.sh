#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC2154
echo "$release_preflight" "$release_log_collector"

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
goreleaser_yaml=$SRC_ROOT/.goreleaser.yml

if [[ $release_preflight == true ]]; then
  echo "adding preflight packages to goreleaser.yml"
  echo '  extra_files:
    - glob: build/preflight.tar.gz
    - glob: build/preflight-sha256.txt' >>"$goreleaser_yaml"
fi

if [[ $release_log_collector != true ]]; then
  echo "removing log-collector packages from goreleaser.yml"
  sed -i '/binary: log-collector/a \ \ skip: true' "$goreleaser_yaml"
fi

echo "updated $goreleaser_yaml"
cat "$goreleaser_yaml"
