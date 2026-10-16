#!/usr/bin/env bash
# Installs Portcullis for the GitHub Action (see action.yml).
#
# At a release tag (uses: cemilalp02/portcullis@v0.1.0) it downloads the
# release archive for the runner and runs it only after two checks:
#   1. the archive's SLSA build provenance was made for this tag by this
#      repository's release workflow on a GitHub-hosted runner
#      (gh attestation verify), and
#   2. its SHA-256 matches the release's checksums.txt.
# A failed check fails the step; it never falls back to an unverified binary.
#
# At any other ref (a branch or commit SHA), with from-source: true, when the
# tag has no release archive, or when the GitHub CLI is missing, it writes
# build=true to the step outputs and build.sh compiles the action's own
# source instead. On success it exports PORTCULLIS_BIN.
set -euo pipefail

repo="cemilalp02/portcullis"
ref="${ACTION_REF:-}"
gh="${GH_CLI:-gh}" # the GitHub CLI; tests point this elsewhere
out="${RUNNER_TEMP:-/tmp}/portcullis"
mkdir -p "$out"

from_source() {
  echo "Building Portcullis from source: $1"
  echo "build=true" >>"${GITHUB_OUTPUT:-/dev/null}"
  exit 0
}

if [ "${INPUT_FROM_SOURCE:-false}" = "true" ]; then
  from_source "from-source is set"
fi
if ! [[ "$ref" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  from_source "the action ref '${ref:-none}' is not a release tag"
fi
if ! command -v "$gh" >/dev/null 2>&1; then
  from_source "the GitHub CLI (gh) is not available to verify the release"
fi

case "${RUNNER_OS:-$(uname -s)}" in
  Linux) os=linux ;;
  macOS | Darwin) os=darwin ;;
  Windows | MINGW* | MSYS*) os=windows ;;
  *) from_source "no release archive for the runner OS '${RUNNER_OS:-}'" ;;
esac
case "${RUNNER_ARCH:-$(uname -m)}" in
  X64 | x86_64 | amd64) arch=amd64 ;;
  ARM64 | arm64 | aarch64) arch=arm64 ;;
  *) from_source "no release archive for the runner architecture '${RUNNER_ARCH:-}'" ;;
esac

ext="tar.gz"
if [ "$os" = "windows" ]; then
  ext="zip"
fi
archive="portcullis_${ref#v}_${os}_${arch}.${ext}"
base="https://github.com/$repo/releases/download/$ref"

cd "$out"
if ! curl -fsSIL --retry 3 -o /dev/null "$base/$archive"; then
  from_source "release $ref has no $archive"
fi
curl -fsSL --retry 3 -o "$archive" "$base/$archive"
curl -fsSL --retry 3 -o checksums.txt "$base/checksums.txt"

echo "Verifying the build provenance of $archive"
if ! "$gh" attestation verify "$archive" --repo "$repo" \
  --signer-workflow "$repo/.github/workflows/release.yml" \
  --source-ref "refs/tags/$ref" --deny-self-hosted-runners >/dev/null; then
  echo "::error::$archive has no valid build provenance from the $repo release workflow for $ref"
  exit 1
fi

# "<sha256>  <file>"; a leading "*" marks binary mode in some sha256sum output.
want="$(awk -v f="$archive" '{ n = $2; sub(/^\*/, "", n) } n == f { print $1 }' checksums.txt)"
if command -v sha256sum >/dev/null 2>&1; then
  got="$(sha256sum "$archive" | awk '{ print $1 }')"
else
  got="$(shasum -a 256 "$archive" | awk '{ print $1 }')"
fi
if [ -z "$want" ] || [ "$got" != "$want" ]; then
  echo "::error::the SHA-256 of $archive does not match checksums.txt"
  exit 1
fi

rm -rf bin
mkdir bin
if [ "$ext" = "zip" ]; then
  unzip -q "$archive" -d bin
else
  tar -xzf "$archive" -C bin
fi
bin="$out/bin/portcullis"
if [ "$os" = "windows" ]; then
  bin="$bin.exe"
fi
"$bin" version
echo "build=false" >>"${GITHUB_OUTPUT:-/dev/null}"
if [ -n "${GITHUB_ENV:-}" ]; then
  echo "PORTCULLIS_BIN=$bin" >>"$GITHUB_ENV"
fi
