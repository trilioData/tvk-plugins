#!/usr/bin/env bash

set -euo pipefail

[[ -n "${DEBUG:-}" ]] && set -x

# install shfmt that ensures consistent format in shell scripts
go_path="$(go env GOPATH)"
if ! [[ -x "$go_path/bin/shfmt" ]]; then
  echo >&2 'Installing shfmt'
  GO111MODULE=off go get -u -v mvdan.cc/sh/v3/cmd/shfmt
fi

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"

# run shell fmt
shfmt_out="$(shfmt -l -i=2 "${SRC_ROOT}/hack" "${SRC_ROOT}/tools/" "${SRC_ROOT}/tests/")"
if [[ -n "${shfmt_out}" ]]; then
  echo >&2 "The following shell scripts need to be formatted, run: 'shfmt -w -i=2 ${SRC_ROOT}/hack ${SRC_ROOT}/tools/ ${SRC_ROOT}/tests/'"
  echo >&2 "${shfmt_out}"
  exit 1
fi

# run shell lint
find "${SRC_ROOT}" -type f -name "*.sh" -not -path "*/vendor/*" -exec "shellcheck" {} +

echo >&2 "shell-lint: No issues detected!"
