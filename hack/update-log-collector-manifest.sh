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

mkdir -p "${build_dir}"

# shellcheck disable=SC2086
cp "$log_collector_template_manifest" $build_dir/$log_collector_yaml
log_collector_template_manifest=$build_dir/$log_collector_yaml

repoURL=$(git config --get remote.origin.url)
log_collector_sha256_file="tvk-plugins-sha256.txt"
log_collector_sha256_filePath=$build_dir/$log_collector_sha256_file

log_collector_sha256_URI="$repoURL/releases/download/${LOG_COLLECTOR_VERSION}/$log_collector_sha256_file"

curl -fsSL "$log_collector_sha256_URI" >"${log_collector_sha256_filePath}"

if [ -s "${log_collector_sha256_filePath}" ]; then
  echo "File ${log_collector_sha256_filePath} successfully downloaded and contains data"
else
  echo "File ${log_collector_sha256_filePath} does not contain any data. Exiting..."
  exit 1
fi

log_collector_linux_amd64_sha=$(awk '/log-collector/ && /linux_amd64/ { print $1 }' "$log_collector_sha256_filePath")
# shellcheck disable=SC2086
log_collector_linux_arm64_sha=$(awk '/log-collector/ && /linux_arm64/ { print $1 }' $log_collector_sha256_filePath)
# shellcheck disable=SC2086
log_collector_linux_arm_sha=$(awk '/log-collector/ && /linux_arm.tar.gz/ { print $1 }' $log_collector_sha256_filePath)
# shellcheck disable=SC2086
log_collector_darwin_amd64_sha=$(awk '/log-collector/ && /darwin_amd64/ { print $1 }' $log_collector_sha256_filePath)
# shellcheck disable=SC2086
log_collector_darwin_arm64_sha=$(awk '/log-collector/ && /darwin_arm64/ { print $1 }' $log_collector_sha256_filePath)
# shellcheck disable=SC2086
log_collector_windows_amd64_sha=$(awk '/log-collector/ && /windows_amd64/ { print $1 }' $log_collector_sha256_filePath)
# shellcheck disable=SC2086
log_collector_windows_arm64_sha=$(awk '/log-collector/ && /windows_arm64/ { print $1 }' $log_collector_sha256_filePath)
# shellcheck disable=SC2086
log_collector_windows_arm_sha=$(awk '/log-collector/ && /windows_arm.zip/ { print $1 }' $log_collector_sha256_filePath)

sed -i "s/LOG_COLLECTOR_VERSION/$LOG_COLLECTOR_VERSION/g" "$log_collector_template_manifest"

sed -i "s/LOG_COLLECTOR_LINUX_AMD64_TAR_CHECKSUM/$log_collector_linux_amd64_sha/g" "$log_collector_template_manifest"
sed -i "s/LOG_COLLECTOR_LINUX_ARM64_TAR_CHECKSUM/$log_collector_linux_arm64_sha/g" "$log_collector_template_manifest"
sed -i "s/LOG_COLLECTOR_LINUX_ARM_TAR_CHECKSUM/$log_collector_linux_arm_sha/g" "$log_collector_template_manifest"
sed -i "s/LOG_COLLECTOR_DARWIN_AMD64_TAR_CHECKSUM/$log_collector_darwin_amd64_sha/g" "$log_collector_template_manifest"
sed -i "s/LOG_COLLECTOR_DARWIN_ARM64_TAR_CHECKSUM/$log_collector_darwin_arm64_sha/g" "$log_collector_template_manifest"
sed -i "s/LOG_COLLECTOR_WINDOWS_AMD64_TAR_CHECKSUM/$log_collector_windows_amd64_sha/g" "$log_collector_template_manifest"
sed -i "s/LOG_COLLECTOR_WINDOWS_ARM64_TAR_CHECKSUM/$log_collector_windows_arm64_sha/g" "$log_collector_template_manifest"
sed -i "s/LOG_COLLECTOR_WINDOWS_ARM_TAR_CHECKSUM/$log_collector_windows_arm_sha/g" "$log_collector_template_manifest"

cp "$build_dir"/$log_collector_yaml "$plugins_dir"/$log_collector_yaml
echo >&2 "Updated log-collector plugin manifest '$log_collector_yaml' with 'version=$LOG_COLLECTOR_VERSION' and new sha256sum"
