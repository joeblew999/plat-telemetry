---
title: "Developer Guide"
---

Build and develop the platform from source.

## Quick Start

```bash
# Clone all upstream sources at pinned versions
task src:clone

# Build all subsystems
task bin:build

# Run all services
task run
```

## Version Pinning

Versions are pinned in each subsystem's Taskfile:
- `NATS_VERSION=v2.10.24`
- `LB_VERSION=v1.10.0`
- `ARC_VERSION=main`
- `TG_VERSION=master`
- `GF_VERSION=11.4.0`

Override at runtime:
```bash
NATS_VERSION=v2.11.0 task nats:src:clone
```

## Building a Single Subsystem

For faster iteration:
```bash
task bin:build:one SUBSYSTEM=nats
```

## Directory Structure

Each subsystem follows this convention:
- `.src/` - Cloned source code
- `.bin/` - Compiled binaries
- `.data/` - Runtime data

## Creating a Release

1. Update version pins in Taskfiles
2. Commit and push
3. Tag a release:
```bash
git tag v0.1.2
git push origin v0.1.2
```

GitHub Actions will build and upload binaries automatically.
