#!/usr/bin/env bash

set -euo pipefail

get_git_tag() {
  # shellcheck disable=SC2046
  # shellcheck disable=SC2006
  git_describe="$(git describe --abbrev=0 --always --tags)"
  if [[ ! "${git_describe}" =~ v.* ]]; then
    # if tag cannot be inferred, still provide a valid version field for plugin yamls
    git_describe="v0.0.0"
  fi

  git_version="${TAG_NAME:-$git_describe}"
  echo >&2 "current git version is $git_version"
}

get_git_tag
