#!/bin/bash

set -e -o pipefail

if [[ -z "${PREFLIGHT_VERSION}" ]]; then
  echo >&2 "PREFLIGHT_VERSION (required) is not set"
  exit 1
else
  echo "PREFLIGHT_VERSION is set to ${PREFLIGHT_VERSION}"
fi

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
# shellcheck disable=SC2164
cd "$SRC_ROOT"

plugins_dir="$SRC_ROOT"/plugins
build_dir="$SRC_ROOT"/build
template_manifest_dir="$SRC_ROOT"/.krew
preflight_yaml="tvk-preflight.yaml"
preflight_template_manifest=$template_manifest_dir/$preflight_yaml

mkdir -p "${build_dir}"

# shellcheck disable=SC2086
cp "$preflight_template_manifest" $build_dir/$preflight_yaml
preflight_template_manifest=$build_dir/$preflight_yaml

repoURL=$(git config --get remote.origin.url)
preflightSha256File="preflight-sha256.txt"

preflightSha256URI="$repoURL/releases/download/${PREFLIGHT_VERSION}/$preflightSha256File"
preflightSha256FilePath=$build_dir/$preflightSha256File

curl -fsSL "$preflightSha256URI" >"${preflightSha256FilePath}"

if [ -s "${preflightSha256FilePath}" ]; then
  echo "File ${preflightSha256FilePath} successfully downloaded and contains data"
else
  echo "File ${preflightSha256FilePath} does not contain any data. Exiting..."
  exit 1
fi

preflight_linux_amd64_sha=$(awk '/preflight/ && /linux_amd64/ { print $1 }' "$preflightSha256FilePath")
# shellcheck disable=SC2086
preflight_linux_arm64_sha=$(awk '/preflight/ && /linux_arm64/ { print $1 }' "$preflightSha256FilePath")
# shellcheck disable=SC2086
preflight_linux_arm_sha=$(awk '/preflight/ && /linux_arm.tar.gz/ { print $1 }' "$preflightSha256FilePath")
# shellcheck disable=SC2086
preflight_darwin_amd64_sha=$(awk '/preflight/ && /darwin_amd64/ { print $1 }' "$preflightSha256FilePath")
# shellcheck disable=SC2086
preflight_darwin_arm64_sha=$(awk '/preflight/ && /darwin_arm64/ { print $1 }' $preflightSha256FilePath)
# shellcheck disable=SC2086
preflight_windows_amd64_sha=$(awk '/preflight/ && /windows_amd64/ { print $1 }' $preflightSha256FilePath)
# shellcheck disable=SC2086
preflight_windows_arm64_sha=$(awk '/preflight/ && /windows_arm64/ { print $1 }' $preflightSha256FilePath)
# shellcheck disable=SC2086
preflight_windows_arm_sha=$(awk '/preflight/ && /windows_arm.zip/ { print $1 }' $preflightSha256FilePath)

sed -i "s/PREFLIGHT_VERSION/$PREFLIGHT_VERSION/g" "$preflight_template_manifest"

sed -i "s/PREFLIGHT_LINUX_AMD64_TAR_CHECKSUM/$preflight_linux_amd64_sha/g" "$preflight_template_manifest"
sed -i "s/PREFLIGHT_LINUX_ARM64_TAR_CHECKSUM/$preflight_linux_arm64_sha/g" "$preflight_template_manifest"
sed -i "s/PREFLIGHT_LINUX_ARM_TAR_CHECKSUM/$preflight_linux_arm_sha/g" "$preflight_template_manifest"
sed -i "s/PREFLIGHT_DARWIN_AMD64_TAR_CHECKSUM/$preflight_darwin_amd64_sha/g" "$preflight_template_manifest"
sed -i "s/PREFLIGHT_DARWIN_ARM64_TAR_CHECKSUM/$preflight_darwin_arm64_sha/g" "$preflight_template_manifest"
sed -i "s/PREFLIGHT_WINDOWS_AMD64_TAR_CHECKSUM/$preflight_windows_amd64_sha/g" "$preflight_template_manifest"
sed -i "s/PREFLIGHT_WINDOWS_ARM64_TAR_CHECKSUM/$preflight_windows_arm64_sha/g" "$preflight_template_manifest"
sed -i "s/PREFLIGHT_WINDOWS_ARM_TAR_CHECKSUM/$preflight_windows_arm_sha/g" "$preflight_template_manifest"

cp "$build_dir"/$preflight_yaml "$plugins_dir"/$preflight_yaml
echo >&2 "Updated preflight plugin manifest '$preflight_yaml' with 'version=$PREFLIGHT_VERSION' and new sha256sum"
