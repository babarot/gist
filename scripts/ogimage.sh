#!/usr/bin/env bash
# Draw the share images of the page (og:image) from scripts/ogimage/og.html
# with headless Chrome: docs/ogimage/THEME.png, 1200x630 each. The page uses
# one of them as its og:image (see docs/index.html).
#
# Usage: ogimage.sh [THEME ...]
#   THEME  dark or light; both when none
# CHROME names the Chrome or Chromium to run when it is not found.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
out_dir="$root/docs/ogimage"
themes=("$@")
[[ ${#themes[@]} -gt 0 ]] || themes=(dark light)

chrome="${CHROME:-}"
if [[ -z "$chrome" ]]; then
  for c in "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
    "/Applications/Chromium.app/Contents/MacOS/Chromium" \
    google-chrome chromium chromium-browser; do
    if command -v "$c" >/dev/null 2>&1 || [[ -x "$c" ]]; then
      chrome="$c"
      break
    fi
  done
fi
if [[ -z "$chrome" ]]; then
  echo "ogimage: no Chrome found; set CHROME to the browser to run" >&2
  exit 1
fi

mkdir -p "$out_dir"

for theme in "${themes[@]}"; do
  case "$theme" in
    dark | light) ;;
    *) echo "ogimage: no theme $theme; dark or light" >&2; exit 1 ;;
  esac
  # a profile for each run, so that one Chrome does not take over another;
  # the time budget lets the web fonts come in
  profile="$(mktemp -d)"
  "$chrome" --headless=new --disable-gpu --hide-scrollbars --no-first-run \
    --user-data-dir="$profile" --force-device-scale-factor=1 \
    --window-size=1200,630 --timeout=25000 --virtual-time-budget=6000 \
    --screenshot="$out_dir/$theme.png" \
    "file://$root/scripts/ogimage/og.html?v=$theme" 2>/dev/null
  rm -rf "$profile"
  echo "ogimage: wrote docs/ogimage/$theme.png"
done
