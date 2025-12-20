---
title: "Users Guide"
---

# Users Guide

Run the pre-built platform without building from source.

## Quick Start

```bash
# Download pre-built binaries
task bin:download

# Run all services
task run
```

## Version Management

Check installed versions:
```bash
task manifest
```

Upgrade to a specific release:
```bash
RELEASE_VERSION=v0.1.1 task bin:download
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| NATS | 4222 | Messaging |
| Liftbridge | 9292 | Stream processing |
| Arc | 8080 | Telemetry storage |
| Telegraf | - | Metrics collection |
| Grafana | 3000 | Visualization |

## Configuration

Each subsystem has a config file in its directory:
- `nats/nats.conf`
- `liftbridge/` (uses defaults)
- `arc/arc.toml`
- `telegraf/telegraf.conf`
- `grafana/grafana.ini`
