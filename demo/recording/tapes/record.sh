#!/usr/bin/env bash
# Build the demo for Linux, record the tapes with vhs.sh beside it, and
# copy the GIFs next to the README's images in docs.
#
# Usage: record.sh [NAME ...]
set -euo pipefail

root="$(cd "$(dirname "$0")/../../.." && pwd)"
dir="$root/demo/recording/tapes"

if ! docker info > /dev/null 2>&1; then
  echo "Docker is not running; start it and try again." >&2
  exit 1
fi

arch="$(docker version --format '{{.Server.Arch}}')"
echo "Building the demo for linux/$arch ..."
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$dir/.gist-demo" "$root/demo/recording"
trap 'rm -f "$dir/.gist-demo"' EXIT

"$dir/vhs.sh" "$@"

names=("$@")
if [[ ${#names[@]} -eq 0 ]]; then
  for tape in "$dir"/*.tape; do
    names+=("$(basename "$tape" .tape)")
  done
fi
for name in "${names[@]}"; do
  cp "$dir/$name.gif" "$root/docs/$name.gif"
  echo "docs/$name.gif"
done
