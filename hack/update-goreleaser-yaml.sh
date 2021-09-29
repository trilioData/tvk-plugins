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
# shellcheck disable=SC2154
echo "release tvk-oneclick package:" "$release_tvk_oneclick"

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
goreleaser_yaml=$SRC_ROOT/.goreleaser.yml

if [[ $release_preflight == true || $release_cleanup == true || $release_tvk_oneclick == true ]]; then

  echo '  extra_files:' >>"$goreleaser_yaml"

  if [[ $release_preflight == true ]]; then
    echo "adding preflight packages to goreleaser.yml"
    echo '    - glob: build/preflight/preflight.tar.gz
    - glob: build/preflight/preflight-sha256.txt' >>"$goreleaser_yaml"
  fi

  if [[ $release_tvk_oneclick == true ]]; then
    echo "adding tvk-oneclick packages to goreleaser.yml"
    echo '    - glob: build/tvk-oneclick/tvk-oneclick.tar.gz
    - glob: build/tvk-oneclick/tvk-oneclick-sha256.txt' >>"$goreleaser_yaml"
  fi

  if [[ $release_cleanup == true ]]; then
    echo "adding cleanup packages to goreleaser.yml"
    echo '    - glob: build/cleanup/cleanup.tar.gz
    - glob: build/cleanup/cleanup-sha256.txt' >>"$goreleaser_yaml"
  fi

fi

if [[ $release_log_collector != true ]]; then
  echo "skip log-collector packages release from goreleaser.yml"
  sed -i '/binary: log-collector/a \ \ skip: true' "$goreleaser_yaml"
fi

if [[ $release_target_browser != true ]]; then
  echo "skip target-browser packages release from goreleaser.yml"
  sed -i '/binary: target-browser/a \ \ skip: true' "$goreleaser_yaml"
fi

echo "updated $goreleaser_yaml"
cat "$goreleaser_yaml"
