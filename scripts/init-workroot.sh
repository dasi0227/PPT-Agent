#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RESOURCE_WORK_ROOT="${1:-${WORK_ROOT:-$HOME/.dasi/ppt}}"
case "$RESOURCE_WORK_ROOT" in
  /*) ;;
  *) RESOURCE_WORK_ROOT="$PWD/$RESOURCE_WORK_ROOT" ;;
esac
cd "$ROOT_DIR/backend"
exec go run ./cmd/init-resources --work-root "$RESOURCE_WORK_ROOT" --seed-root "$ROOT_DIR/seed"
