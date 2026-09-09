#!/usr/bin/env bash
# Renders og.html to ../assets/og.png at 1200x630 with headless Chrome.
#
# Run it after changing og.html and commit the PNG. It is not part of the
# build: Chrome is not on the Pages runner, and a picture that changes only
# when somebody means it to is the point.
set -euo pipefail
cd "$(dirname "$0")"
chrome=""
for c in google-chrome google-chrome-stable chromium chromium-browser; do
  command -v "$c" >/dev/null 2>&1 && { chrome="$c"; break; }
done
[ -n "$chrome" ] || { echo "no Chrome or Chromium on PATH" >&2; exit 1; }
"$chrome" --headless=new --disable-gpu --no-sandbox --hide-scrollbars \
  --window-size=1200,630 --screenshot="$PWD/../assets/og.png" \
  "file://$PWD/og.html" 2>/dev/null
ls -la ../assets/og.png
