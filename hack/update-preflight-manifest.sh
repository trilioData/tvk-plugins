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

# shellcheck disable=SC2086
cp "$preflight_template_manifest" $build_dir/$preflight_yaml
preflight_template_manifest=$build_dir/$preflight_yaml

repoURL=$(git config --get remote.origin.url)
preflightSha256File="preflight-sha256.txt"

preflightSha256URI="$repoURL/releases/download/${PREFLIGHT_VERSION}/$preflightSha256File"

curl -fsSL "$preflightSha256URI" >"$build_dir"/$preflightSha256File

preflightSha256FilePath=$build_dir/$preflightSha256File

preflight_sha=$(awk '{print $1}' "$preflightSha256FilePath")

sed -i "s/PREFLIGHT_VERSION/$PREFLIGHT_VERSION/g" "$preflight_template_manifest"
sed -i "s/PREFLIGHT_TAR_CHECKSUM/$preflight_sha/g" "$preflight_template_manifest"

cp "$build_dir"/$preflight_yaml "$plugins_dir"/$preflight_yaml
echo >&2 "Updated preflight plugin manifest '$preflight_yaml' with 'version=$PREFLIGHT_VERSION' and new sha256sum"
