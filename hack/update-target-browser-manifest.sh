#!/bin/bash

set -e -o pipefail

if [[ -z "${TARGET_BROWSER_VERSION}" ]]; then
  echo >&2 "TARGET_BROWSER_VERSION (required) is not set"
  exit 1
else
  echo "TARGET_BROWSER_VERSION is set to ${TARGET_BROWSER_VERSION}"
fi

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
# shellcheck disable=SC2164
cd "$SRC_ROOT"

plugins_dir="$SRC_ROOT"/plugins
build_dir="$SRC_ROOT"/build
template_manifest_dir="$SRC_ROOT"/.krew
target_browser_yaml="tvk-target-browser.yaml"
target_browser_template_manifest=$template_manifest_dir/$target_browser_yaml

mkdir -p "${build_dir}"

# shellcheck disable=SC2086
cp "$target_browser_template_manifest" $build_dir/$target_browser_yaml
target_browser_template_manifest=$build_dir/$target_browser_yaml

repoURL=$(git config --get remote.origin.url)
target_browser_sha256_file="tvk-plugins-sha256.txt"
target_browser_sha256_filePath=$build_dir/$target_browser_sha256_file

target_browser_sha256_URI="$repoURL/releases/download/${TARGET_BROWSER_VERSION}/$target_browser_sha256_file"

curl -fsSL "$target_browser_sha256_URI" >"${target_browser_sha256_filePath}"

if [ -s "${target_browser_sha256_filePath}" ]; then
  echo "File ${target_browser_sha256_filePath} successfully downloaded and contains data"
else
  echo "File ${target_browser_sha256_filePath} does not contain any data. Exiting..."
  exit 1
fi

target_browser_linux_sha=$(awk '/target-browser/ && /linux/ { print $1 }' "$target_browser_sha256_filePath")
# shellcheck disable=SC2086
target_browser_darwin_sha=$(awk '/target-browser/ && /darwin/ { print $1 }' $target_browser_sha256_filePath)
# shellcheck disable=SC2086
target_browser_windows_sha=$(awk '/target-browser/ && /windows/ { print $1 }' $target_browser_sha256_filePath)

sed -i "s/TARGET_BROWSER_VERSION/$TARGET_BROWSER_VERSION/g" "$target_browser_template_manifest"

sed -i "s/TARGET_BROWSER_LINUX_TAR_CHECKSUM/$target_browser_linux_sha/g" "$target_browser_template_manifest"
sed -i "s/TARGET_BROWSER_DARWIN_TAR_CHECKSUM/$target_browser_darwin_sha/g" "$target_browser_template_manifest"
sed -i "s/TARGET_BROWSER_WINDOWS_TAR_CHECKSUM/$target_browser_windows_sha/g" "$target_browser_template_manifest"

cp "$build_dir"/$target_browser_yaml "$plugins_dir"/$target_browser_yaml
echo >&2 "Updated target-browser plugin manifest '$target_browser_yaml' with 'version=$TARGET_BROWSER_VERSION' and new sha256sum"
