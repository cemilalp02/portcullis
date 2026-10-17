#!/bin/sh
# Assemble the project website (site/) with the fonts and images it shares
# with the HTML report and the README into one directory for GitHub Pages.
# Usage: scripts/site/build.sh [out-dir]   (default: _site)
set -eu
root=$(cd "$(dirname "$0")/../.." && pwd)
out=${1:-"$root/_site"}
mkdir -p "$out"
find "$out" -mindepth 1 -delete
mkdir -p "$out/fonts" "$out/assets"
cp "$root/site/index.html" "$root/site/style.css" "$root/site/app.js" "$out/"
cp "$root/internal/report/html/fonts/"*.woff2 "$root/internal/report/html/fonts/LICENSE.txt" "$out/fonts/"
cp "$root/docs/assets/brand/favicon.svg" "$out/assets/favicon.svg"
cp "$root/docs/assets/report-light.png" "$root/docs/assets/report-dark.png" "$out/assets/"
cp "$root/docs/assets/social.png" "$out/assets/social.png"
echo "site written to $out"
