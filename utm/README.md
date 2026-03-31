# utm

VM management for macOS using UTM and vagrant_utm.

Using:
- https://github.com/naveenrajm7/vagrant_utm — Vagrant provider for UTM
- https://naveenrajm7.github.io/utm-gallery/ — Pre-built VM boxes (Linux + Windows)

So that we can run UTM for Windows and Linux, and run `mise run ci` (this repo and others) inside those VMs to validate cross-platform compatibility.

---

## The Problem

You write code that needs to work on Windows, but you develop on macOS. CI only tells you after a push. You want to know *before* pushing.

## The Solution: UTM + Vagrant + mise

```
Your Mac
  └── UTM (hypervisor, like VirtualBox but for Apple Silicon)
        ├── Alpine / Debian / Ubuntu  ← Linux testing via SSH
        └── Windows 11 ARM64          ← Windows CI via WinRM
              └── mise run ci         ← same command you run locally
```

---

## Prerequisites

1. **UTM** — Install from https://mac.getutm.app (free) or Mac App Store
2. **Vagrant** — `brew install --cask vagrant` (requires interactive sudo, run in terminal)
3. **vagrant_utm plugin** — installed automatically by `mise run utm:deps`

> Vagrant is distributed as a `.pkg` on macOS so it can't be installed via mise — it needs an interactive terminal for sudo.

---

## Quick Start

All tasks use `VM=<name>` as an environment variable prefix to select which VM to operate on.

### Linux (start here — Alpine is only 93MB)

```bash
# 1. Install deps
mise run utm:deps

# 2. Boot Alpine (downloads 93MB box on first run)
VM=alpine mise run utm:up

# 3. SSH in and verify
VM=alpine mise run utm:ssh

# 4. Stop it when done
VM=alpine mise run utm:down
```

### Windows CI

```bash
# 1. Boot Windows VM (downloads 5.7GB box on first run)
VM=windows mise run utm:up

# 2. Run CI tests inside Windows
mise run utm:windows:ci

# 3. Open an interactive shell
mise run utm:windows:shell
```

---

## How It Works

### Linux VM (Alpine / Debian / Ubuntu)

```
VM=alpine mise run utm:up
  └── vagrant up alpine
        └── downloads utm/alpine-ce from utm-gallery (first run only, 93MB)
        └── UTM starts the Alpine VM
        └── VM ready for SSH

VM=alpine mise run utm:ssh
  └── vagrant ssh alpine
        └── interactive shell inside the VM
```

### Windows VM

```
VM=windows mise run utm:up
  └── vagrant up windows
        └── downloads utm/windows-11 from utm-gallery (first run only, 5.7GB)
        └── UTM starts the Windows 11 ARM64 VM
        └── windows-provision.ps1 runs once:
              installs mise + git
              clones this repo to C:/plat-telemetry
              runs mise install (Go, nats-server, pitchfork, gh)

mise run utm:windows:ci
  └── vagrant winrm → cd C:/plat-telemetry && mise run ci
        └── same mise run ci you run on your Mac
```

---

## Why Each Tool

| Tool | Role |
|------|------|
| **UTM** | The hypervisor — runs VMs on Apple Silicon |
| **vagrant_utm** | Vagrant plugin that speaks to UTM instead of VirtualBox |
| **Vagrant** | Manages VM lifecycle idempotently (up/down/destroy) |
| **utm-gallery** | Pre-built Vagrant boxes for UTM (Linux + Windows 11) |
| **WinRM** | How Vagrant communicates with Windows guests (SSH equivalent) |

---

## Available VMs

Sorted by download size — start with Alpine to test your setup before committing to a larger download.

| Name | OS | Box | Size |
|------|----|-----|------|
| alpine | Alpine CE | utm/alpine-ce | 93MB |
| debian | Debian 12 (Bookworm) | utm/bookworm | 517MB |
| ubuntu | Ubuntu 24.04 | utm/ubuntu-24.04 | 2.6GB |
| windows | Windows 11 ARM64 | utm/windows-11 | 5.7GB |
| macos | macOS Sequoia | macos-sequoia-arm64 | manual |

---

## All Tasks

```bash
# Setup
mise run utm:deps                    # Verify UTM + Vagrant, install vagrant_utm plugin

# VM Lifecycle — VM=<name> is required for all lifecycle tasks
VM=alpine  mise run utm:up           # Start VM (downloads box on first run)
VM=alpine  mise run utm:down         # Stop VM
VM=alpine  mise run utm:destroy      # Destroy VM and its disk
VM=alpine  mise run utm:ssh          # SSH into Linux VM
VM=windows mise run utm:provision    # Re-run provisioners
VM=alpine  mise run utm:reload       # Reload with updated Vagrantfile
           mise run utm:status       # Show status of all VMs

# Box Management
mise run utm:box:download            # Pre-download all boxes
mise run utm:box:list                # List downloaded boxes
mise run utm:box:clean               # Remove all downloaded boxes

# Windows (WinRM-specific)
mise run utm:windows:ci              # Run 'mise run ci' inside Windows VM
mise run utm:windows:shell           # Open interactive WinRM PowerShell session

# Clean (destructive)
mise run utm:clean                   # Destroy ALL VMs
mise run utm:clean:all               # Destroy ALL VMs and remove ALL boxes
```

---

## Reusability (Other Repos)

The goal is for `utm/mise-tasks.toml` to work with any repo, not just this one. The provisioner clones from GitHub and runs `mise install && mise run ci` — so any repo that follows the same mise conventions can use this setup by pointing at their repo URL.

Future: extract `utm/` into a standalone mise plugin that any repo can include.

---

## NATS Integration (Phase 2)

Future work will add NATS-based fleet management:
- Subscribe to `utm.vm.{up,down,destroy,status}` subjects
- Control VMs remotely via NATS messages
- Track state in NATS KV store
- No public IP required — works over NATS leaf nodes
