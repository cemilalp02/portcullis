#!/usr/bin/env bash
# Builds Portcullis from the action's own source tree when install.sh did not
# install a verified release binary, and exports PORTCULLIS_BIN for the
# following steps.
set -euo pipefail

: "${ACTION_PATH:?}"
out="${RUNNER_TEMP:-/tmp}/portcullis"
mkdir -p "$out"
bin="$out/portcullis"
if [ "${RUNNER_OS:-}" = "Windows" ]; then
  bin="$bin.exe"
fi

pkg=github.com/cemilalp02/portcullis/internal/version
(
  cd "$ACTION_PATH"
  CGO_ENABLED=0 go build -trimpath -ldflags "-X $pkg.version=action-${ACTION_REF:-dev}" -o "$bin" ./cmd/portcullis
)
"$bin" version
if [ -n "${GITHUB_ENV:-}" ]; then
  echo "PORTCULLIS_BIN=$bin" >>"$GITHUB_ENV"
fi
