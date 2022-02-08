#!/usr/bin/env bash

set -e -o pipefail

echo >&2 "Creating Preflight plugin manifest yaml"

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

tvk_preflight_yaml="tvk-preflight.yaml"
cp .krew/$tvk_preflight_yaml $build_dir/$tvk_preflight_yaml

tvk_preflight_yaml=$build_dir/$tvk_preflight_yaml

# shellcheck disable=SC2154
preflight_tar_archive="preflight_${git_version}_linux_amd64.tar.gz"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${preflight_tar_archive}" | awk '{print $1;}')"
sed -i "s/PREFLIGHT_LINUX_AMD64_TAR_CHECKSUM/${tar_checksum}/g" $tvk_preflight_yaml

preflight_tar_archive="preflight_${git_version}_linux_arm64.tar.gz"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${preflight_tar_archive}" | awk '{print $1;}')"
sed -i "s/PREFLIGHT_LINUX_ARM64_TAR_CHECKSUM/${tar_checksum}/g" $tvk_preflight_yaml

preflight_tar_archive="preflight_${git_version}_linux_arm.tar.gz"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${preflight_tar_archive}" | awk '{print $1;}')"
sed -i "s/PREFLIGHT_LINUX_ARM_TAR_CHECKSUM/${tar_checksum}/g" $tvk_preflight_yaml

preflight_tar_archive="preflight_${git_version}_darwin_amd64.tar.gz"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${preflight_tar_archive}" | awk '{print $1;}')"
sed -i "s/PREFLIGHT_DARWIN_AMD64_TAR_CHECKSUM/${tar_checksum}/g" $tvk_preflight_yaml

preflight_tar_archive="preflight_${git_version}_darwin_arm64.tar.gz"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${preflight_tar_archive}" | awk '{print $1;}')"
sed -i "s/PREFLIGHT_DARWIN_ARM64_TAR_CHECKSUM/${tar_checksum}/g" $tvk_preflight_yaml

preflight_tar_archive="preflight_${git_version}_windows_amd64.zip"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${preflight_tar_archive}" | awk '{print $1;}')"
sed -i "s/PREFLIGHT_WINDOWS_AMD64_TAR_CHECKSUM/${tar_checksum}/g" $tvk_preflight_yaml

preflight_tar_archive="preflight_${git_version}_windows_arm64.zip"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${preflight_tar_archive}" | awk '{print $1;}')"
sed -i "s/PREFLIGHT_WINDOWS_ARM64_TAR_CHECKSUM/${tar_checksum}/g" $tvk_preflight_yaml

preflight_tar_archive="preflight_${git_version}_windows_arm.zip"
tar_checksum="$(eval "${checksum_cmd[@]}" "$build_dir/${preflight_tar_archive}" | awk '{print $1;}')"
sed -i "s/PREFLIGHT_WINDOWS_ARM_TAR_CHECKSUM/${tar_checksum}/g" $tvk_preflight_yaml

sed -i "s/PREFLIGHT_VERSION/$git_version/g" $tvk_preflight_yaml

echo >&2 "Written out $tvk_preflight_yaml"
