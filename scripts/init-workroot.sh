#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SEED_ROOT="$ROOT_DIR/seed/assets"
WORK_ROOT="${1:-${WORK_ROOT:-$HOME/.dasi/ppt}}"

if [ ! -d "$SEED_ROOT" ]; then
  printf 'Seed directory does not exist: %s\n' "$SEED_ROOT" >&2
  exit 1
fi

if find "$SEED_ROOT" -type l -print -quit | grep -q .; then
  printf 'Seed directory must not contain symbolic links: %s\n' "$SEED_ROOT" >&2
  exit 1
fi

mkdir -p "$WORK_ROOT/db" "$WORK_ROOT/projects" "$WORK_ROOT/assets"

while IFS= read -r -d '' source; do
  relative="${source#"$SEED_ROOT"/}"
  destination="$WORK_ROOT/assets/$relative"
  if [ ! -e "$destination" ] && [ ! -L "$destination" ]; then
    mkdir -p "$(dirname "$destination")"
    cp "$source" "$destination"
  fi
done < <(find "$SEED_ROOT" -type f ! -name '.DS_Store' -print0)
