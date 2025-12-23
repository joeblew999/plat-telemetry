# utm

VM management for macOS using UTM and vagrant_utm.

## Goals

- Idempotent VM lifecycle (like Terraform)
- Fleet management via NATS JetStream
- Cross-platform: Windows, macOS, Linux guests on Mac hosts
- Integration with plat-telemetry for observability

## Prerequisites

1. **UTM** - Install from https://mac.getutm.app (free) or Mac App Store
2. **Vagrant** - `brew install vagrant`
3. **vagrant_utm plugin** - Installed automatically by `task utm:deps`

## Quick Start

```bash
# Install dependencies (UTM check + vagrant_utm plugin)
task utm:deps

# Start Ubuntu VM
task utm:up VM=ubuntu

# SSH into VM
task utm:ssh VM=ubuntu

# Stop VM
task utm:down VM=ubuntu

# Destroy VM
task utm:destroy VM=ubuntu
```

## Available VMs

| Name | OS | Box | Status |
|------|-----|-----|--------|
| ubuntu | Ubuntu 24.04 | naveenrajm7/ubuntu-24.04-aarch64 | Ready |
| debian | Debian 13 | naveenrajm7/debian-13-aarch64 | Ready |
| alpine | Alpine 3.21 | naveenrajm7/alpine-3.21-aarch64 | Ready |
| windows | Windows 11 | Custom box required | Manual setup |
| macos | macOS Sequoia | Custom box required | Manual setup |

## Tasks

```bash
# Dependencies
task utm:deps           # Install vagrant_utm plugin, check UTM

# VM Lifecycle
task utm:up             # Start all VMs
task utm:up VM=ubuntu   # Start specific VM
task utm:down           # Stop all VMs
task utm:destroy        # Destroy all VMs
task utm:status         # Show VM status
task utm:ssh VM=ubuntu  # SSH into VM

# Provisioning
task utm:provision      # Run provisioners
task utm:reload         # Reload with updated Vagrantfile

# Box Management
task utm:box:download   # Pre-download boxes
task utm:box:list       # List downloaded boxes

# Clean
task utm:clean          # Destroy all VMs
task utm:clean:boxes    # Remove downloaded boxes
task utm:clean:all      # Destroy VMs and remove boxes
```

## Windows/macOS Custom Boxes

Linux boxes are available from [utm-gallery](https://naveenrajm7.github.io/utm-gallery/).

For Windows and macOS, you need to create custom boxes:

### Option 1: Packer (Recommended)

Use [packer-plugin-utm](https://github.com/naveenrajm7/packer-plugin-utm) to build boxes from ISOs.

### Option 2: Manual

1. Create VM manually in UTM
2. Package as Vagrant box:
   ```bash
   vagrant package --base "VM Name" --output windows-11-arm64.box
   vagrant box add windows-11-arm64 windows-11-arm64.box
   ```

## NATS Integration (Phase 2)

Future work will add NATS-based fleet management:
- Subscribe to `utm.vm.{up,down,destroy,status}` subjects
- Control VMs remotely via NATS messages
- Track state in NATS KV store
- No public IP required - works over NATS leaf nodes

## References

- [UTM](https://mac.getutm.app/) - Virtual machine manager for macOS
- [vagrant_utm](https://naveenrajm7.github.io/vagrant_utm/) - Vagrant provider for UTM
- [utm-gallery](https://naveenrajm7.github.io/utm-gallery/) - Pre-built VM boxes
- [packer-plugin-utm](https://github.com/naveenrajm7/packer-plugin-utm) - Packer plugin for UTM
