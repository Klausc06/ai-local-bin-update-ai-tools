#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
dist_dir="$repo_root/dist"
binary="update-ai-tools"

VERSION="${VERSION:-$(git -C "$repo_root" describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)}"
LDFLAGS="${LDFLAGS:--s -w -X 'update-ai-tools/internal/app.version=$VERSION'}"

targets=(
  "darwin/arm64"
  "darwin/amd64"
  "linux/arm64"
  "linux/amd64"
  "windows/amd64"
)

mkdir -p "$dist_dir"

for target in "${targets[@]}"; do
  goos="${target%/*}"
  goarch="${target#*/}"
  out="$dist_dir/$binary-$goos-$goarch"
  if [[ "$goos" == "windows" ]]; then
    out="$out.exe"
  fi

  printf 'building %s/%s -> %s (%s)\n' "$goos" "$goarch" "${out/#$repo_root\//}" "$VERSION"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOCACHE="${GOCACHE:-$repo_root/.gocache}" \
    go build -ldflags "$LDFLAGS" -trimpath -o "$out" "$repo_root/cmd/update-ai-tools"
done

printf 'release binaries written to %s\n' "${dist_dir/#$HOME/~}"
