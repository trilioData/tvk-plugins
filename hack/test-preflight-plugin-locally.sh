#!/usr/bin/env bash

# This script verifies that a preflight build can be installed to a system using
# krew local testing method

set -euo pipefail

[[ -n "${DEBUG:-}" ]] && set -x

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
cd "$SRC_ROOT"

build_dir="build"

preflight_manifest="${build_dir}/tvk-preflight.yaml"
if [[ ! -f "${preflight_manifest}" ]]; then
  echo >&2 "Could not find manifest ${preflight_manifest}."
  exit 1
fi

preflight_archive="${build_dir}/preflight.tar.gz"
if [[ ! -f "${preflight_archive}" ]]; then
  echo >&2 "Could not find archive ${preflight_archive}."
  exit 1
fi

kubectl krew install --manifest=$preflight_manifest --archive=$preflight_archive
kubectl krew uninstall tvk-preflight

echo >&2 "Successfully tested preflight plugin locally"
