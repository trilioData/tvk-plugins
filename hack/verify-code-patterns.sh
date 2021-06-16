#!/usr/bin/env bash

set -euo pipefail

# checks code patterns and fails if criteria does not meet

# Disallow usage of ioutil.TempDir in tests in favor of testutil.
#shellcheck disable=SC2063
out="$(grep --include '*_test.go' --exclude-dir 'vendor/' -EIrn 'ioutil.\TempDir' || true)"
if [[ -n "$out" ]]; then
  echo >&2 "You used ioutil.TempDir in tests, use 'testutil.NewTempDir()' instead:"
  echo >&2 "$out"
  exit 1
fi

# Do not use glog/klog in test code
#shellcheck disable=SC2063
out="$(grep --include '*_test.go' --exclude-dir 'vendor/' -EIrn '[kg]log\.' || true)"
if [[ -n "$out" ]]; then
  echo >&2 "You used glog or klog in tests, use 't.Logf' instead:"
  echo >&2 "$out"
  exit 1
fi

# Do not initialize index.{Plugin,Platform} structs in test code.
#shellcheck disable=SC2063
out="$(grep --include '*_test.go' --exclude-dir 'vendor/' -EIrn '[^]](index\.)(Plugin|Platform){' || true)"
if [[ -n "$out" ]]; then
  echo >&2 "Do not use index.Platform or index.Plugin structs directly in tests,"
  echo >&2 "use testutil.NewPlugin() or testutil.NewPlatform() instead:"
  echo >&2 "-----"
  echo >&2 "$out"
  exit 1
fi
