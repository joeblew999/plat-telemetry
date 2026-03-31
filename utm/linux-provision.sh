#!/usr/bin/env sh
# Linux VM provisioner for plat-telemetry CI validation
# Works on ubuntu, debian, alpine — uses sh (POSIX) for alpine compatibility
# Executed by Vagrant on first boot: mise run utm:provision VM=alpine

set -e

echo "=== plat-telemetry Linux Provisioner ==="

REPO_URL="https://github.com/joeblew999/plat-telemetry.git"
REPO_DIR="$HOME/plat-telemetry"

# -- 1. Install git ------------------------------------------------------------

if command -v git >/dev/null 2>&1; then
  echo "OK git: $(git --version)"
else
  echo "Installing git..."
  if command -v apk >/dev/null 2>&1; then
    sudo apk add --no-cache git
  elif command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update -qq && sudo apt-get install -y -q git
  fi
  echo "OK git: $(git --version)"
fi

# -- 2. Install mise -----------------------------------------------------------

if command -v mise >/dev/null 2>&1; then
  echo "OK mise: $(mise --version)"
else
  echo "Installing mise..."
  curl -fsSL https://mise.run | sh
  export PATH="$HOME/.local/bin:$PATH"
  echo "OK mise: $(mise --version)"
fi

export PATH="$HOME/.local/bin:$PATH"

# -- 3. Clone / update repo ----------------------------------------------------

if [ -d "$REPO_DIR/.git" ]; then
  echo "Updating repository..."
  git -C "$REPO_DIR" pull --quiet
else
  echo "Cloning repository..."
  git clone "$REPO_URL" "$REPO_DIR"
fi
echo "OK repo at $REPO_DIR"

# -- 4. Install mise tools -----------------------------------------------------

cd "$REPO_DIR"
mise trust --quiet
mise install
echo "OK mise tools installed"

echo ""
echo "=== Provisioning complete ==="
echo "To run CI: VM=alpine mise run utm:linux:ci"
echo "Or from inside VM: cd ~/plat-telemetry && mise run ci"
