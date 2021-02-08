#!/usr/bin/env bash

# This script verifies that a preflight and log-collector builds can be installed to a system using
# krew local testing method

set -euo pipefail

[[ -n "${DEBUG:-}" ]] && set -x

SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$SCRIPT_PATH"/test-preflight-plugin-locally.sh
"$SCRIPT_PATH"/test-log-collector-plugin-locally.sh
