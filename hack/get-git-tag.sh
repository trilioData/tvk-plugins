#!/usr/bin/env bash

# This script verifies that a preflight build can be installed to a system using
# itself as the documented installation method.

set -euo pipefail

get_git_tag() {
  # Copy and process plugins manifests
  # shellcheck disable=SC2046
  git_describe="$(git describe --tags --always)"
  if [[ ! "${git_describe}" =~ v.* ]]; then
    # if tag cannot be inferred (e.g. CI/CD), still provide a valid
    # version field for plugin.yaml
    git_describe="v0.0.0"
  fi

  git_version="${TAG_NAME:-$git_describe}"
  echo >&2 "current git version is $git_version"
}

get_git_tag
