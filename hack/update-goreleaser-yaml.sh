#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC2154
echo "release preflight package:" "$release_preflight"
# shellcheck disable=SC2154
echo "release log-collector package:" "$release_log_collector"
# shellcheck disable=SC2154
echo "release target-browser package:" "$release_target_browser"
# shellcheck disable=SC2154
echo "release cleanup package:" "$release_cleanup"

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
goreleaser_yaml=$SRC_ROOT/.goreleaser.yml

if [[ $release_preflight == true ]]; then
  echo "adding preflight packages to goreleaser.yml"
  echo '  extra_files:
    - glob: build/preflight.tar.gz
    - glob: build/preflight-sha256.txt' >>"$goreleaser_yaml"
fi

if [[ $release_log_collector != true ]]; then
  echo "skip log-collector packages release from goreleaser.yml"
  sed -i '/binary: log-collector/a \ \ skip: true' "$goreleaser_yaml"
fi

if [[ $release_target_browser != true ]]; then
  echo "skip target-browser packages release from goreleaser.yml"
  sed -i '/binary: target-browser/a \ \ skip: true' "$goreleaser_yaml"
fi

if [[ $release_cleanup == true ]]; then
  echo "adding cleanup packages to goreleaser.yml"
  echo '  extra_files:
    - glob: build/cleanup.tar.gz
    - glob: build/cleanup-sha256.txt' >>"$goreleaser_yaml"
fi

echo "updated $goreleaser_yaml"
cat "$goreleaser_yaml"
