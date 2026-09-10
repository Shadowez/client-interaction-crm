#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Refusing to package a dirty worktree. Commit or stash changes first." >&2
  exit 1
fi

version="$(node -p "require('./package.json').version")"
output_dir="${1:-installer/dist}"
mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"
asset="client-interaction-crm-app-v${version}.zip"

git archive --format=zip --prefix=app/ HEAD -o "$output_dir/$asset"
(
  cd "$output_dir"
  sha256sum "$asset" > "$asset.sha256"
)

echo "Created: $output_dir/$asset"
echo "Checksum: $output_dir/$asset.sha256"
