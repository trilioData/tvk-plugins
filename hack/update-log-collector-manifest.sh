#!/bin/bash

set -e -o pipefail

if [[ -z "${LOG_COLLECTOR_VERSION}" ]]; then
  echo >&2 "LOG_COLLECTOR_VERSION (required) is not set"
  exit 1
else
  echo "LOG_COLLECTOR_VERSION is set to ${LOG_COLLECTOR_VERSION}"
fi

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
# shellcheck disable=SC2164
cd "$SRC_ROOT"

plugins_dir="$SRC_ROOT"/plugins
build_dir="$SRC_ROOT"/build
template_manifest_dir="$SRC_ROOT"/.krew
log_collector_yaml="tvk-log-collector.yaml"
log_collector_template_manifest=$template_manifest_dir/$log_collector_yaml

# shellcheck disable=SC2086
cp "$log_collector_template_manifest" $build_dir/$log_collector_yaml
log_collector_template_manifest=$build_dir/$log_collector_yaml

repoURL=$(git config --get remote.origin.url)
log_collector_sha256_file="log-collector-sha256.txt"

log_collector_sha256_URI="$repoURL/releases/download/${LOG_COLLECTOR_VERSION}/$log_collector_sha256_file"

curl -fsSL "$log_collector_sha256_URI" >"$build_dir"/$log_collector_sha256_file

log_collector_sha256_filePath=$build_dir/$log_collector_sha256_file

log_collector_linux_sha=$(awk '/linux/{ print $1 }' "$log_collector_sha256_filePath")
# shellcheck disable=SC2086
log_collector_darwin_sha=$(awk '/darwin/{ print $1 }' $log_collector_sha256_filePath)
# shellcheck disable=SC2086
log_collector_windows_sha=$(awk '/windows/{ print $1 }' $log_collector_sha256_filePath)

sed -i "s/LOG_COLLECTOR_VERSION/$LOG_COLLECTOR_VERSION/g" "$log_collector_template_manifest"

sed -i "s/LOG_COLLECTOR_LINUX_TAR_CHECKSUM/$log_collector_linux_sha/g" "$log_collector_template_manifest"
sed -i "s/LOG_COLLECTOR_DARWIN_TAR_CHECKSUM/$log_collector_darwin_sha/g" "$log_collector_template_manifest"
sed -i "s/LOG_COLLECTOR_WINDOWS_TAR_CHECKSUM/$log_collector_windows_sha/g" "$log_collector_template_manifest"

cp "$build_dir"/$log_collector_yaml "$plugins_dir"/$log_collector_yaml
echo >&2 "Updated log-collector plugin manifest '$log_collector_yaml' with 'version=$LOG_COLLECTOR_VERSION' and new sha256sum"
