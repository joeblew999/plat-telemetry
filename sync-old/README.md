# sync

**Why this exists:** Know when third-party subsystems (NATS, Liftbridge, etc.) have source changes AND when our own binary releases are published - automatically apply updates. Now with Cloudflare event integration.

## Problem

We integrate third-party subsystems (nats-server, liftbridge, telegraf, arc):
- **DEV problem**: When nats-io/nats-server pushes new code, we want to know and rebuild ONLY nats (not everything)
- **USER problem**: When we publish new binaries, users want to know and auto-update without git
- **Wasteful**: Current CI rebuilds ALL binaries on every commit (expensive, slow)
- **Cloudflare gap**: No visibility into CF Pages deploys, Workers updates, audit logs

## Solution

The sync subsystem monitors:

### 1. GitHub Events
- **Polling mode**: GitHub API polls every 5 minutes (for repos we don't control)
- **Webhook mode**: GitHub webhook fires when they push code (if configured)
- We rebuild ONLY that subsystem (INCREMENTAL)
- Hot-reload the updated binary

### 2. Our Releases (joeblew999/plat-telemetry)
- GitHub webhook fires when we publish binaries
- USERs auto-download ONLY updated subsystems
- No git binary required (embeds go-git/v5)

### 3. Cloudflare Events (NEW)
- **Audit log polling**: Know when DNS, configs change
- **Notification webhooks**: Pages deploys, Workers deploys, alerts
- **Logpush endpoint**: High-volume log data
- **cloudflared tunnel**: Expose local server to CF (alternative to smee.io)

## Commands

```bash
# Run as service (webhook server + tunnels)
sync serve                                # Main service (see env vars below)

# GitHub commands
sync check                                # Check current versions
sync poll                                 # Poll upstream repos (5min interval)
sync webhook                              # Webhook server only
sync tunnel new                           # Auto-create smee channel
sync tunnel https://smee.io/xxx           # Use existing smee channel
sync tunnel-setup owner/repo              # Create smee + configure GitHub webhook

# Cloudflare commands
sync cf tunnel [port]                     # Start cloudflared quick tunnel (default: 9090)
sync cf poll [interval]                   # Poll CF audit logs (default: 1m)
sync cf webhook [port]                    # Start CF webhook server only
sync cf check                             # Check if cloudflared is installed
sync cf install                           # Install cloudflared

# Git operations (no git binary needed)
sync clone <url> <path> [version]
sync pull <path>
sync fetch <path> [--tags]
sync checkout <path> <ref>
sync tags <path>
```

## Environment Variables

```bash
# Server
PORT=9090                          # HTTP server port

# Tunnel (choose one)
TUNNEL_TYPE=cloudflared            # cloudflared | smee | none
SMEE_URL=https://smee.io/xxx       # Only if TUNNEL_TYPE=smee

# GitHub
GITHUB_TOKEN=xxx                   # For API polling (optional)
GITHUB_WEBHOOK_SECRET=xxx          # For webhook signature verification

# Cloudflare
CF_ACCOUNT_ID=xxx                  # Required for CF features
CF_API_TOKEN=xxx                   # For audit log polling
CF_WEBHOOK_SECRET=xxx              # For webhook signature verification

# Features
SYNC_ENABLE_CF=true                # Enable Cloudflare integration
SYNC_ENABLE_CF_AUDIT=true          # Enable CF audit log polling
SYNC_POLL_INTERVAL=5m              # Polling interval
```

## Local Development

### GitHub Webhooks (smee.io)

```bash
# Terminal 1: Start webhook server
task sync:run

# Terminal 2: One-time setup
task sync:tunnel:setup REPO=joeblew999/plat-telemetry

# Terminal 2: Start tunnel
task sync:tunnel SMEE_URL=https://smee.io/YOUR_CHANNEL
```

### Cloudflare Webhooks (cloudflared)

```bash
# Start with cloudflared tunnel (no account needed for quick tunnels)
TUNNEL_TYPE=cloudflared task sync:run

# Or just the tunnel
sync cf tunnel 9090
# → Prints: https://xxx.trycloudflare.com
# Configure this URL in Cloudflare Notifications destination
```

### All-in-One (GitHub + Cloudflare)

```bash
# With cloudflared as primary tunnel
export TUNNEL_TYPE=cloudflared
export CF_ACCOUNT_ID=xxx
export CF_API_TOKEN=xxx
export SYNC_ENABLE_CF_AUDIT=true

sync serve
# → GitHub webhooks: https://xxx.trycloudflare.com/webhook
# → CF webhooks: https://xxx.trycloudflare.com/cf/webhook
```

## Architecture

```
sync/
├── cmd/                    # CLI commands
│   ├── serve.go           # Main service entry
│   ├── cf.go              # Cloudflare commands
│   ├── tunnel.go          # smee.io tunnel
│   └── ...
├── pkg/
│   ├── cloudflare/        # CF integration
│   │   ├── audit.go       # Audit log polling
│   │   ├── webhook.go     # Notification handlers
│   │   └── tunnel.go      # cloudflared management
│   ├── events/            # Unified event bus
│   │   └── events.go      # GitHub + CF → normalized events
│   ├── github/            # GitHub webhook handling
│   ├── server/            # HTTP server
│   └── tunnel/            # smee.io tunnel
└── main.go                # CLI entry point
```

## Event Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                       sync serve                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  SOURCES                          EVENT BUS         ACTIONS     │
│  ───────                          ─────────         ───────     │
│                                                                  │
│  GitHub Webhook ──────┐                                         │
│  /webhook/github      │                                         │
│                       │           ┌─────────┐                   │
│  CF Notifications ────┼──────────►│  Event  │──► Log           │
│  /cf/webhook          │           │   Bus   │──► State Update  │
│                       │           │         │──► Reload        │
│  CF Logpush ──────────┤           └─────────┘──► Notify        │
│  /cf/logpush/*        │                 ▲                       │
│                       │                 │                       │
│  GitHub Poller ───────┘                 │                       │
│  (every 5 min)                          │                       │
│                                         │                       │
│  CF Audit Poller ───────────────────────┘                       │
│  (every 1 min)                                                  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/health` | Health check |
| `/status` | Service status (JSON) |
| `/webhook` | GitHub webhooks |
| `/webhook/github` | GitHub webhooks (explicit) |
| `/cf/webhook` | Cloudflare notification webhooks |
| `/cf/logpush/*` | Cloudflare Logpush HTTP destination |

## Integration

- Webhook server on port 9090
- Poller service runs continuously (configurable interval)
- Taskfile tasks: `sync:check`, `sync:update`
- Process Compose services: `sync` (webhooks), `sync-poller` (polling)
- Triggers `task reload PROC=<subsystem>` for hot-reload

## cf-worker (Edge Event Aggregator)

For high-volume CF events or edge processing, deploy the Go WASM worker:

```bash
# Build and deploy
task cf-worker:deploy

# Set sync endpoint (your cloudflared tunnel URL)
task cf-worker:secret:set NAME=SYNC_ENDPOINT
# Enter: https://xxx.trycloudflare.com/cf/webhook
```

The worker aggregates events at the edge and forwards to your sync service.

See [CLAUDE.md](../CLAUDE.md) for full documentation.
