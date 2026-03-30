#!/usr/bin/env bash
set -euo pipefail

# Install mise, which then installs all other tools (go, task) via .mise.toml
curl https://mise.jdx.dev/install.sh | sh
mise trust
mise install --yes
