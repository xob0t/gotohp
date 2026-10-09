#!/usr/bin/env bash
# Recompute vendorHash (Go modules) and npmDepsHash (frontend) in nix/hashes.json.
# Needs: nix (flakes enabled), jq, git. New files must be `git add`ed first,
# because flakes only see tracked files.
set -euo pipefail
cd "$(dirname "$0")/.."

hashes=nix/hashes.json
fake="sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
backup=$(mktemp)
cp "$hashes" "$backup"
trap 'cp "$backup" "$hashes"' ERR

set_hash() { # key value
  local tmp
  tmp=$(mktemp)
  jq --arg k "$1" --arg v "$2" '.[$k] = $v' "$hashes" >"$tmp"
  mv "$tmp" "$hashes"
}

update() { # key flake-attr
  set_hash "$1" "$fake"
  local new
  new=$(nix build ".#$2" --no-link 2>&1 | sed -n 's/^ *got: *//p' | head -n1 || true)
  if [ -z "$new" ]; then
    echo "could not determine $1 (is the build failing for another reason? run: nix build .#$2)" >&2
    return 1
  fi
  set_hash "$1" "$new"
  echo "$1 = $new"
}

# gotohp-cli needs no frontend or GTK, so it is the cheapest way to get the vendor hash.
update vendorHash gotohp-cli
update npmDepsHash frontend
