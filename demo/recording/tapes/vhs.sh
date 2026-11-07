#!/usr/bin/env bash
# Record the tapes in this directory with VHS in Docker, then make each MP4 a
# GIF with ffmpeg. The GIF is made apart because VHS's own GIF output is
# unreliable when several containers run at once on macOS.
#
# Usage: vhs.sh [NAME ...]
#   NAME  a tape to record, without .tape; all of them when none
set -euo pipefail

dir="$(cd "$(dirname "$0")" && pwd)"
image="ghcr.io/charmbracelet/vhs:v0.12.1"

tapes=()
for name in "$@"; do
  if [[ ! -f "$dir/$name.tape" ]]; then
    echo "no tape named $name (looked for $dir/$name.tape)" >&2
    exit 2
  fi
  tapes+=("$dir/$name.tape")
done
if [[ ${#tapes[@]} -eq 0 ]]; then
  tapes=("$dir"/*.tape)
fi

echo "Recording ${#tapes[@]} tape(s) ..."
pids=()
names=()
for tape in "${tapes[@]}"; do
  name="$(basename "$tape" .tape)"
  names+=("$name")
  echo "  > $name"
  docker run --rm --memory=2g --cpus=2 -v "$dir":/data -w /data "$image" "$name.tape" > /dev/null &
  pids+=($!)
done
failures=0
for i in "${!pids[@]}"; do
  if wait "${pids[$i]}"; then
    echo "  ok ${names[$i]}"
  else
    echo "  failed ${names[$i]}" >&2
    failures=$((failures + 1))
  fi
done
if [[ $failures -gt 0 ]]; then
  exit 1
fi

# The VHS image has ffmpeg, so no other image is pulled
echo "Converting MP4 to GIF ..."
for name in "${names[@]}"; do
  docker run --rm -v "$dir":/data -w /data --entrypoint ffmpeg "$image" -y -loglevel error -i "$name.mp4" \
    -vf "fps=12,scale=1200:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=256[p];[s1][p]paletteuse=dither=sierra2_4a" \
    "$name.gif"
  echo "  ok $name.gif"
done
