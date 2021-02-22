#!/usr/bin/env bash

# This script verifies that a log-collector build can be installed to a system using
# krew local testing method

set -euo pipefail

[[ -n "${DEBUG:-}" ]] && set -x

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
cd "$SRC_ROOT"

build_dir="dist"

# get current git tag
# shellcheck disable=SC1090
source "$SRC_ROOT"/hack/get-git-tag.sh

log_collector_manifest="${build_dir}/tvk-log-collector.yaml"
if [[ ! -f "${log_collector_manifest}" ]]; then
  echo >&2 "Could not find manifest ${log_collector_manifest}."
  exit 1
fi

# shellcheck disable=SC2154
log_collector_tar_archive="log-collector_${git_version}_linux_amd64.tar.gz"
log_collector_archive_path="${build_dir}/${log_collector_tar_archive}"
if [[ ! -f "${log_collector_archive_path}" ]]; then
  echo >&2 "Could not find archive ${log_collector_archive_path}."
  exit 1
fi

kubectl krew install --manifest=$log_collector_manifest --archive="$log_collector_archive_path"
kubectl krew uninstall tvk-log-collector

log_collector_tar_archive="log-collector_${git_version}_darwin_amd64.tar.gz"
log_collector_archive_path="${build_dir}/${log_collector_tar_archive}"
if [[ ! -f "${log_collector_archive_path}" ]]; then
  echo >&2 "Could not find archive ${log_collector_archive_path}."
  exit 1
fi

KREW_OS=darwin KREW_ARCH=amd64 kubectl krew install --manifest=$log_collector_manifest --archive="$log_collector_archive_path"
KREW_OS=darwin KREW_ARCH=amd64 kubectl krew uninstall tvk-log-collector

log_collector_tar_archive="log-collector_${git_version}_windows_amd64.zip"
log_collector_archive_path="${build_dir}/${log_collector_tar_archive}"
if [[ ! -f "${log_collector_archive_path}" ]]; then
  echo >&2 "Could not find archive ${log_collector_archive_path}."
  exit 1
fi

KREW_OS=windows KREW_ARCH=amd64 kubectl krew install --manifest=$log_collector_manifest --archive="$log_collector_archive_path"
KREW_OS=windows KREW_ARCH=amd64 kubectl krew uninstall tvk-log-collector

echo >&2 "Successfully tested log-collector plugin locally"
