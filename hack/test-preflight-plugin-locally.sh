#!/usr/bin/env bash

# This script verifies that a preflight build can be installed to a system using
# krew local testing method

set -euo pipefail

[[ -n "${DEBUG:-}" ]] && set -x

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
cd "$SRC_ROOT"

build_dir="dist"

# get current git tag
# shellcheck disable=SC1090
source "$SRC_ROOT"/hack/get-git-tag.sh

preflight_manifest="${build_dir}/tvk-preflight.yaml"
if [[ ! -f "${preflight_manifest}" ]]; then
  echo >&2 "Could not find manifest ${preflight_manifest}."
  exit 1
fi

# shellcheck disable=SC2154
preflight_tar_archive="preflight_${git_version}_linux_amd64.tar.gz"
preflight_archive_path="${build_dir}/${preflight_tar_archive}"
if [[ ! -f "${preflight_archive_path}" ]]; then
  echo >&2 "Could not find archive ${preflight_archive_path}."
  exit 1
fi

# test for linux OS
kubectl krew install --manifest=$preflight_manifest --archive="$preflight_archive_path"
kubectl krew uninstall tvk-preflight

preflight_tar_archive="preflight_${git_version}_linux_arm64.tar.gz"
preflight_archive_path="${build_dir}/${preflight_tar_archive}"
if [[ ! -f "${preflight_archive_path}" ]]; then
  echo >&2 "Could not find archive ${preflight_archive_path}."
  exit 1
fi

KREW_OS=linux KREW_ARCH=arm64 kubectl krew install --manifest=$preflight_manifest --archive="$preflight_archive_path"
KREW_OS=linux KREW_ARCH=arm64 kubectl krew uninstall tvk-preflight

preflight_tar_archive="preflight_${git_version}_linux_arm.tar.gz"
preflight_archive_path="${build_dir}/${preflight_tar_archive}"
if [[ ! -f "${preflight_archive_path}" ]]; then
  echo >&2 "Could not find archive ${preflight_archive_path}."
  exit 1
fi

KREW_OS=linux KREW_ARCH=arm kubectl krew install --manifest=$preflight_manifest --archive="$preflight_archive_path"
KREW_OS=linux KREW_ARCH=arm kubectl krew uninstall tvk-preflight

preflight_tar_archive="preflight_${git_version}_darwin_amd64.tar.gz"
preflight_archive_path="${build_dir}/${preflight_tar_archive}"
if [[ ! -f "${preflight_archive_path}" ]]; then
  echo >&2 "Could not find archive ${preflight_archive_path}."
  exit 1
fi

KREW_OS=darwin KREW_ARCH=amd64 kubectl krew install --manifest=$preflight_manifest --archive="$preflight_archive_path"
KREW_OS=darwin KREW_ARCH=amd64 kubectl krew uninstall tvk-preflight

preflight_tar_archive="preflight_${git_version}_darwin_arm64.tar.gz"
preflight_archive_path="${build_dir}/${preflight_tar_archive}"
if [[ ! -f "${preflight_archive_path}" ]]; then
  echo >&2 "Could not find archive ${preflight_archive_path}."
  exit 1
fi

KREW_OS=darwin KREW_ARCH=arm64 kubectl krew install --manifest=$preflight_manifest --archive="$preflight_archive_path"
KREW_OS=darwin KREW_ARCH=arm64 kubectl krew uninstall tvk-preflight

preflight_tar_archive="preflight_${git_version}_windows_amd64.zip"
preflight_archive_path="${build_dir}/${preflight_tar_archive}"
if [[ ! -f "${preflight_archive_path}" ]]; then
  echo >&2 "Could not find archive ${preflight_archive_path}."
  exit 1
fi

KREW_OS=windows KREW_ARCH=amd64 kubectl krew install --manifest=$preflight_manifest --archive="$preflight_archive_path"
KREW_OS=windows KREW_ARCH=amd64 kubectl krew uninstall tvk-preflight

preflight_tar_archive="preflight_${git_version}_windows_arm64.zip"
preflight_archive_path="${build_dir}/${preflight_tar_archive}"
if [[ ! -f "${preflight_archive_path}" ]]; then
  echo >&2 "Could not find archive ${preflight_archive_path}."
  exit 1
fi

KREW_OS=windows KREW_ARCH=arm64 kubectl krew install --manifest=$preflight_manifest --archive="$preflight_archive_path"
KREW_OS=windows KREW_ARCH=arm64 kubectl krew uninstall tvk-preflight

preflight_tar_archive="preflight_${git_version}_windows_arm.zip"
preflight_archive_path="${build_dir}/${preflight_tar_archive}"
if [[ ! -f "${preflight_archive_path}" ]]; then
  echo >&2 "Could not find archive ${preflight_archive_path}."
  exit 1
fi

KREW_OS=windows KREW_ARCH=arm kubectl krew install --manifest=$preflight_manifest --archive="$preflight_archive_path"
KREW_OS=windows KREW_ARCH=arm kubectl krew uninstall tvk-preflight

echo >&2 "Successfully tested preflight plugin locally"
