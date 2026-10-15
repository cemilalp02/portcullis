#!/usr/bin/env bash
# Runs Portcullis inside the GitHub Action (see action.yml and
# docs/GITHUB_ACTION.md). Inputs arrive as environment variables and are never
# interpolated into code, which rules out script injection.
#
# Pull requests: the base commit's .portcullis.yaml, baseline, context.lock
# and .portcullis/ directory are exported to a snapshot and passed with
# --base-snapshot, so a pull request cannot weaken the check that reviews it
# (REQUIREMENTS FR-44). The portcullis binary itself never runs git
# (ADR-0006); this script does, in a trusted step.
set -euo pipefail

bin="${PORTCULLIS_BIN:?PORTCULLIS_BIN is not set; run build.sh first}"
work="${RUNNER_TEMP:-/tmp}/portcullis"
mkdir -p "$work"
sarif="$work/portcullis.sarif"

path="${INPUT_PATH:-.}"
path="${path%/}"
if [ -z "$path" ]; then
  path="."
fi
prefix=""
if [ "$path" != "." ]; then
  prefix="$path/"
fi

common=(--trusted)
while IFS= read -r line || [ -n "$line" ]; do
  line="$(printf '%s' "$line" | tr -d '\r' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
  if [ -n "$line" ]; then
    common+=(--exclude "$line")
  fi
done <<<"${INPUT_EXCLUDE:-}"

if [ "${EVENT_NAME:-}" = "pull_request" ] && [ -n "${BASE_SHA:-}" ]; then
  if ! git cat-file -e "${BASE_SHA}^{commit}" 2>/dev/null; then
    if ! git fetch --no-tags --depth=1 origin "$BASE_SHA"; then
      echo "::error title=Portcullis::Cannot read the base commit $BASE_SHA. Use actions/checkout with fetch-depth: 0 (or keep its default credentials) so the base configuration can be compared."
      exit 2
    fi
  fi
  snapshot="$work/base"
  rm -rf "$snapshot"
  mkdir -p "$snapshot"
  for f in .portcullis.yaml .portcullis-baseline.yaml context.lock; do
    if git cat-file -e "${BASE_SHA}:${prefix}${f}" 2>/dev/null; then
      git show "${BASE_SHA}:${prefix}${f}" >"$snapshot/$f"
    fi
  done
  git ls-tree -r --name-only "$BASE_SHA" -- "${prefix}.portcullis/" | while IFS= read -r p; do
    rel="${p#"$prefix"}"
    mkdir -p "$snapshot/$(dirname "$rel")"
    git show "${BASE_SHA}:${p}" >"$snapshot/$rel"
  done
  common+=(--base-snapshot "$snapshot")
fi

scan=(scan --format github --sarif "$sarif" --path-prefix "$prefix")
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  scan+=(--summary "$GITHUB_STEP_SUMMARY")
fi
if [ -n "${INPUT_FAIL_ON:-}" ]; then
  scan+=(--fail-on "$INPUT_FAIL_ON")
fi

set +e
"$bin" "${scan[@]}" "${common[@]}" "$path"
code=$?
set -e

case "${INPUT_VERIFY_LOCK:-auto}" in
  true) verify=1 ;;
  false) verify=0 ;;
  *) if [ -f "$path/context.lock" ]; then verify=1; else verify=0; fi ;;
esac
if [ "$verify" = 1 ]; then
  set +e
  "$bin" verify "${common[@]}" "$path"
  vcode=$?
  set -e
  if [ "$vcode" -gt "$code" ]; then
    code=$vcode
  fi
fi

if [ ! -s "$sarif" ]; then
  sarif=""
fi
if [ -n "${GITHUB_OUTPUT:-}" ]; then
  {
    echo "exit-code=$code"
    echo "sarif-file=$sarif"
  } >>"$GITHUB_OUTPUT"
fi
echo "Portcullis finished with exit code $code"
