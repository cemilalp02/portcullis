#!/usr/bin/env bash
# Prints the CHANGELOG.md section of a release, for the GitHub release notes.
# Usage: notes.sh v0.1.0 [CHANGELOG.md]
# Fails when the changelog has no section for the version, so a tag cannot be
# released without its entry.
set -euo pipefail

tag="${1:?usage: notes.sh vX.Y.Z [CHANGELOG.md]}"
file="${2:-CHANGELOG.md}"
version="${tag#v}"

notes="$(awk -v v="$version" '
  /^## \[/ { if (found) exit; if (index($0, "## [" v "]") == 1) { found = 1; next } }
  found { print }
' "$file")"

if [ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]; then
  echo "notes.sh: no section for $version in $file" >&2
  exit 1
fi
printf '%s\n' "$notes"
