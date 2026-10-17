#!/usr/bin/env bash
# Tests install.sh against a fake release server (a curl stub) and a fake
# GitHub CLI (a gh stub); nothing goes to the network.
# Usage: bash scripts/action/install_test.sh
set -uo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/release/v0.0.1" "$work/stubs" "$work/pkg"

# Release v0.0.1: an archive whose "binary" prints a version, and its checksum
# in the format GoReleaser writes.
printf '#!/bin/sh\necho "portcullis v0.0.1"\n' >"$work/pkg/portcullis"
chmod +x "$work/pkg/portcullis"
archive="portcullis_0.0.1_linux_amd64.tar.gz"
tar -czf "$work/release/v0.0.1/$archive" -C "$work/pkg" portcullis
(cd "$work/release/v0.0.1" && sha256sum "$archive" | sed 's/ \*/  /' >checksums.txt)

cat >"$work/stubs/curl" <<'EOF'
#!/usr/bin/env bash
# Serves .../releases/download/<tag>/<file> from $FAKE_RELEASES.
head=0; out=""; url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift ;;
    -*I*) head=1 ;;
    http*) url="$1" ;;
  esac
  shift
done
src="$FAKE_RELEASES/${url#https://github.com/cemilalp02/portcullis/releases/download/}"
[ -f "$src" ] || exit 22
[ "$head" = 1 ] && exit 0
cp "$src" "$out"
EOF
cat >"$work/stubs/gh" <<'EOF'
#!/usr/bin/env bash
echo "gh $*" >>"$FAKE_GH_LOG"
exit "${FAKE_GH_EXIT:-0}"
EOF
chmod +x "$work/stubs/"*

passed=0
failed=0
rc=0
run() { # GH_CLI REF [FROM_SOURCE]
  : >"$work/out"
  : >"$work/env"
  : >"$work/ghlog"
  rm -rf "$work/tmp"
  mkdir -p "$work/tmp"
  PATH="$work/stubs:/usr/bin:/bin" GH_CLI="$1" FAKE_RELEASES="$work/release" FAKE_GH_LOG="$work/ghlog" FAKE_GH_EXIT="${FAKE_GH_EXIT:-0}" \
    RUNNER_TEMP="$work/tmp" RUNNER_OS=Linux RUNNER_ARCH=X64 GITHUB_OUTPUT="$work/out" GITHUB_ENV="$work/env" \
    ACTION_REF="$2" INPUT_FROM_SOURCE="${3:-false}" GH_TOKEN=test \
    bash "$here/install.sh" >"$work/log" 2>&1
  rc=$?
}
expect() { # NAME EXIT BUILD PATTERN
  local build
  build="$(sed -n 's/^build=//p' "$work/out" | tail -1)"
  if [ "$rc" = "$2" ] && [ "$build" = "$3" ] && grep -q -- "$4" "$work/log" "$work/env" "$work/ghlog"; then
    echo "ok   $1"
    passed=$((passed + 1))
  else
    echo "FAIL $1: exit $rc (want $2), build=$build (want $3), no match for: $4"
    sed 's/^/     /' "$work/log"
    failed=$((failed + 1))
  fi
}

run "$work/stubs/gh" main
expect "a branch ref builds from source" 0 true "is not a release tag"
run "$work/stubs/gh" v0.0.1 true
expect "from-source builds from source" 0 true "from-source is set"
run "$work/no-such-gh" v0.0.1
expect "without gh it builds from source" 0 true "GitHub CLI (gh) is not available"
run "$work/stubs/gh" v9.9.9
expect "a tag without a release archive builds from source" 0 true "has no portcullis_9.9.9_linux_amd64.tar.gz"
run "$work/stubs/gh" v0.0.1
expect "a verified release is installed" 0 false "PORTCULLIS_BIN=.*/bin/portcullis$"
expect "the provenance must come from the release workflow at this tag" 0 false \
  "--signer-workflow cemilalp02/portcullis/.github/workflows/release.yml --source-ref refs/tags/v0.0.1 --deny-self-hosted-runners"
FAKE_GH_EXIT=1 run "$work/stubs/gh" v0.0.1
expect "missing provenance stops the action" 1 "" "has no valid build provenance"
sed -i 's/^[0-9a-f]\{8\}/00000000/' "$work/release/v0.0.1/checksums.txt"
run "$work/stubs/gh" v0.0.1
expect "a checksum mismatch stops the action" 1 "" "does not match checksums.txt"

echo "$passed passed, $failed failed"
[ "$failed" = 0 ]
