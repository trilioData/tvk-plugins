#!/usr/bin/env bash

set -e -o pipefail

echo >&2 "Creating Target Browser plugin manifest yaml"

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
cd "$SRC_ROOT"

# get current git tag
# shellcheck disable=SC1090
source "$SRC_ROOT"/hack/get-git-tag.sh

build_dir="dist"

# consistent timestamps for files in build dir to ensure consistent checksums
while IFS= read -r -d $'\0' f; do
  echo "modifying atime/mtime for $f"
  TZ=UTC touch -at "0001010000" "$f"
  TZ=UTC touch -mt "0001010000" "$f"
done < <(find $build_dir -print0)

checksum_cmd="shasum -a 256"
if hash sha256sum 2>/dev/null; then
  checksum_cmd="sha256sum"
fi

tvk_target_browser_yaml="tvk-target-browser.yaml"
cp .krew/$tvk_target_browser_yaml $build_dir/$tvk_target_browser_yaml

tvk_target_browser_yaml=$build_dir/$tvk_target_browser_yaml

# shellcheck disable=SC2154
target_browser_tar_archive="target-browser_${git_version}_linux_amd64.tar.gz"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${target_browser_tar_archive}" | awk '{print $1;}')"
sed -i "s/TARGET_BROWSER_LINUX_TAR_CHECKSUM/${tar_checksum}/g" $tvk_target_browser_yaml

target_browser_tar_archive="target-browser_${git_version}_darwin_amd64.tar.gz"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${target_browser_tar_archive}" | awk '{print $1;}')"
sed -i "s/TARGET_BROWSER_DARWIN_TAR_CHECKSUM/${tar_checksum}/g" $tvk_target_browser_yaml

target_browser_tar_archive="target-browser_${git_version}_windows_amd64.zip"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${target_browser_tar_archive}" | awk '{print $1;}')"
sed -i "s/TARGET_BROWSER_WINDOWS_TAR_CHECKSUM/${tar_checksum}/g" $tvk_target_browser_yaml

sed -i "s/TARGET_BROWSER_VERSION/$git_version/g" $tvk_target_browser_yaml

echo >&2 "Written out $tvk_target_browser_yaml"
