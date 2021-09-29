#!/usr/bin/env bash

# This script verifies that a target-browser build can be installed to a system using
# krew local testing method

set -euo pipefail

[[ -n "${DEBUG:-}" ]] && set -x

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
cd "$SRC_ROOT"

build_dir="dist"

# get current git tag
# shellcheck disable=SC1090
source "$SRC_ROOT"/hack/get-git-tag.sh

target_browser_manifest="${build_dir}/tvk-target-browser.yaml"
if [[ ! -f "${target_browser_manifest}" ]]; then
  echo >&2 "Could not find manifest ${target_browser_manifest}."
  exit 1
fi

# shellcheck disable=SC2154
target_browser_tar_archive="target-browser_${git_version}_linux_amd64.tar.gz"
target_browser_archive_path="${build_dir}/${target_browser_tar_archive}"
if [[ ! -f "${target_browser_archive_path}" ]]; then
  echo >&2 "Could not find archive ${target_browser_archive_path}."
  exit 1
fi

kubectl krew install --manifest=$target_browser_manifest --archive="$target_browser_archive_path"
kubectl krew uninstall tvk-target-browser

target_browser_tar_archive="target-browser_${git_version}_linux_arm64.tar.gz"
target_browser_archive_path="${build_dir}/${target_browser_tar_archive}"
if [[ ! -f "${target_browser_archive_path}" ]]; then
  echo >&2 "Could not find archive ${target_browser_archive_path}."
  exit 1
fi

KREW_OS=linux KREW_ARCH=arm64 kubectl krew install --manifest=$target_browser_manifest --archive="$target_browser_archive_path"
KREW_OS=linux KREW_ARCH=arm64 kubectl krew uninstall tvk-target-browser

target_browser_tar_archive="target-browser_${git_version}_linux_arm.tar.gz"
target_browser_archive_path="${build_dir}/${target_browser_tar_archive}"
if [[ ! -f "${target_browser_archive_path}" ]]; then
  echo >&2 "Could not find archive ${target_browser_archive_path}."
  exit 1
fi

KREW_OS=linux KREW_ARCH=arm kubectl krew install --manifest=$target_browser_manifest --archive="$target_browser_archive_path"
KREW_OS=linux KREW_ARCH=arm kubectl krew uninstall tvk-target-browser

target_browser_tar_archive="target-browser_${git_version}_darwin_amd64.tar.gz"
target_browser_archive_path="${build_dir}/${target_browser_tar_archive}"
if [[ ! -f "${target_browser_archive_path}" ]]; then
  echo >&2 "Could not find archive ${target_browser_archive_path}."
  exit 1
fi

KREW_OS=darwin KREW_ARCH=amd64 kubectl krew install --manifest=$target_browser_manifest --archive="$target_browser_archive_path"
KREW_OS=darwin KREW_ARCH=amd64 kubectl krew uninstall tvk-target-browser

target_browser_tar_archive="target-browser_${git_version}_darwin_arm64.tar.gz"
target_browser_archive_path="${build_dir}/${target_browser_tar_archive}"
if [[ ! -f "${target_browser_archive_path}" ]]; then
  echo >&2 "Could not find archive ${target_browser_archive_path}."
  exit 1
fi

KREW_OS=darwin KREW_ARCH=arm64 kubectl krew install --manifest=$target_browser_manifest --archive="$target_browser_archive_path"
KREW_OS=darwin KREW_ARCH=arm64 kubectl krew uninstall tvk-target-browser

target_browser_tar_archive="target-browser_${git_version}_windows_amd64.zip"
target_browser_archive_path="${build_dir}/${target_browser_tar_archive}"
if [[ ! -f "${target_browser_archive_path}" ]]; then
  echo >&2 "Could not find archive ${target_browser_archive_path}."
  exit 1
fi

KREW_OS=windows KREW_ARCH=amd64 kubectl krew install --manifest=$target_browser_manifest --archive="$target_browser_archive_path"
KREW_OS=windows KREW_ARCH=amd64 kubectl krew uninstall tvk-target-browser

target_browser_tar_archive="target-browser_${git_version}_windows_arm64.zip"
target_browser_archive_path="${build_dir}/${target_browser_tar_archive}"
if [[ ! -f "${target_browser_archive_path}" ]]; then
  echo >&2 "Could not find archive ${target_browser_archive_path}."
  exit 1
fi

KREW_OS=windows KREW_ARCH=arm64 kubectl krew install --manifest=$target_browser_manifest --archive="$target_browser_archive_path"
KREW_OS=windows KREW_ARCH=arm64 kubectl krew uninstall tvk-target-browser

target_browser_tar_archive="target-browser_${git_version}_windows_arm.zip"
target_browser_archive_path="${build_dir}/${target_browser_tar_archive}"
if [[ ! -f "${target_browser_archive_path}" ]]; then
  echo >&2 "Could not find archive ${target_browser_archive_path}."
  exit 1
fi

KREW_OS=windows KREW_ARCH=arm kubectl krew install --manifest=$target_browser_manifest --archive="$target_browser_archive_path"
KREW_OS=windows KREW_ARCH=arm kubectl krew uninstall tvk-target-browser

echo >&2 "Successfully tested target-browser plugin locally"
