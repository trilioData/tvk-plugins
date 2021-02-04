#!/usr/bin/env bash

set -euo pipefail

[[ -n "${DEBUG:-}" ]] && set -x

gopath="$(go env GOPATH)"

install_shfmt() {
  shfmt_dir="$(mktemp -d)"
  trap 'rm -rf -- ${shfmt_dir}' EXIT

  cd "${shfmt_dir}"
  go mod init foo
  go get mvdan.cc/sh/v3/cmd/shfmt@v3.0.0
  cd -
}

# install shfmt that ensures consistent format in shell scripts
if ! [[ -x "${gopath}/bin/shfmt" ]]; then
  echo >&2 'Installing shfmt'
  install_shfmt
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
shfmt_out="$("$gopath"/bin/shfmt -l -i=2 "${script_dir}")"
if [[ -n "${shfmt_out}" ]]; then
  echo >&2 "The following shell scripts need to be formatted, run: 'shfmt -w -i=2 ${script_dir}'"
  echo >&2 "${shfmt_out}"
  exit 1
fi

# run shell lint
SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"

find "${SRC_ROOT}" -type f -name "*.sh" -not -path "*/vendor/*" -exec "shellcheck" {} +

echo >&2 "No issues detected!"
