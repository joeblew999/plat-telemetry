#!/usr/bin/env sh
# Linux CI runner for plat-telemetry
# Called by: mise run utm:linux:ci (via vagrant provision --provision-with ci)
# Works on ubuntu, debian, alpine

set -e

export PATH="$HOME/.local/bin:$PATH"

REPO_DIR="$HOME/plat-telemetry"

if [ ! -d "$REPO_DIR" ]; then
  echo "FAIL repo not found at $REPO_DIR — run 'VM=<name> mise run utm:provision' first"
  exit 1
fi

if ! command -v mise >/dev/null 2>&1; then
  echo "FAIL mise not found — run 'VM=<name> mise run utm:provision' first"
  exit 1
fi

cd "$REPO_DIR"

echo "Pulling latest repo changes..."
git pull --quiet

echo "Running mise install..."
mise install --quiet

if [ -n "${DOPPLER_TOKEN:-}" ]; then
  echo "Running 'mise run ci' via doppler (secrets injected)..."
  doppler run -- mise run ci
else
  echo "WARN DOPPLER_TOKEN not set — running without secrets (ci:release will be skipped)"
  mise run ci
fi
