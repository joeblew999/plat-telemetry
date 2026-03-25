# Cloudflare Events Integration Plan

## Overview

Add Cloudflare event feedback loop to the sync subsystem - know when CF operations complete and react accordingly.

**Relationship to TODO_CF.md:** That file covers R2 migration for binaries. This file covers CF *events* (webhooks, audit logs, etc.).

---

## Current State

### sync/ subsystem handles:
- GitHub webhooks (push, release, etc.) via githubevents
- GitHub API polling for version changes
- smee.io tunnel for local webhook development
- Triggers `task reload PROC=<subsystem>` on changes

### What we're adding:
- Cloudflare audit log polling (config changes)
- Cloudflare notification webhooks (alerts, deploys)
- cloudflared tunnel option (alternative to smee.io)
- Edge fan-in Worker (optional, for high volume)

---

## Architecture Decision

**Recommendation: Extend sync subsystem (not separate cf subsystem)**

```
sync/
├── cmd/
│   ├── serve.go          # Main entry - unified server
│   ├── webhook.go         # GitHub webhook only (existing)
│   ├── poll.go            # GitHub polling (existing)
│   └── cf.go              # NEW: CF-specific commands
├── pkg/
│   ├── github/            # RENAME from webhook/ - GitHub-specific
│   │   └── webhook.go
│   ├── cloudflare/        # NEW: CF event handling
│   │   ├── cloudflare.go
│   │   ├── audit.go
│   │   ├── webhook.go
│   │   └── tunnel.go
│   ├── events/            # NEW: Unified event bus
│   │   └── events.go
│   ├── server/            # Unified HTTP server
│   │   └── server.go
│   └── tunnel/            # EXTEND: Support smee + cloudflared
│       └── tunnel.go
```

---

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
│  /webhook/cf          │           │   Bus   │──► State Update  │
│                       │           │         │──► Reload        │
│  CF Logpush ──────────┤           └─────────┘──► Notify        │
│  /logpush/*           │                 ▲                       │
│                       │                 │                       │
│  GitHub Poller ───────┘                 │                       │
│  (every 5 min)                          │                       │
│                                         │                       │
│  CF Audit Poller ───────────────────────┘                       │
│  (every 1 min)                                                  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Normalized Event Type

```go
package events

type Source string
const (
    SourceGitHub     Source = "github"
    SourceCloudflare Source = "cloudflare"
)

type Event struct {
    ID        string                 // Unique event ID
    Source    Source                 // github, cloudflare
    Type      string                 // push, release, audit_log, pages_deploy
    Timestamp time.Time

    // GitHub-specific
    Repo      string                 // owner/repo
    Ref       string                 // refs/heads/main

    // Cloudflare-specific
    AccountID string
    ZoneID    string

    // Common
    Action    string                 // What happened
    Resource  string                 // What was affected
    Actor     string                 // Who did it
    Metadata  map[string]interface{} // Extra data
}
```

---

## Configuration

### Environment Variables

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
CF_API_TOKEN=xxx                   # For audit log polling
CF_ACCOUNT_ID=xxx                  # Required for CF features
CF_WEBHOOK_SECRET=xxx              # For webhook signature verification

# Features
SYNC_ENABLE_GH_POLL=true           # GitHub API polling
SYNC_ENABLE_CF_AUDIT=true          # CF audit log polling
SYNC_POLL_INTERVAL=5m              # Polling interval
```

### Config File (optional)

```json
// sync/.data/config.json
{
  "tunnel_type": "cloudflared",
  "smee_url": "https://smee.io/xxx",
  "github": {
    "repos": ["joeblew999/plat-telemetry"],
    "poll_enabled": true
  },
  "cloudflare": {
    "account_id": "xxx",
    "audit_poll_enabled": true,
    "poll_interval": "1m"
  }
}
```

---

## CLI Commands

### Existing (keep as-is):
```bash
sync serve              # Main service
sync webhook            # GitHub webhook server only
sync tunnel <url>       # smee.io tunnel
sync poll               # GitHub polling
sync check              # Check versions
```

### New commands:
```bash
# CF-specific
sync cf webhook         # CF webhook server only
sync cf poll            # CF audit log polling only
sync cf tunnel          # cloudflared tunnel (auto quick tunnel)
sync cf tunnel <name>   # Named cloudflared tunnel

# Unified
sync serve --cf         # Enable CF features
sync serve --all        # Enable all features (GH + CF)
```

---

## Implementation Phases

### Phase 1: Dependencies & Compile Check
- [ ] Add `cloudflare-go/v6` to go.mod
- [ ] Verify `sync/pkg/cloudflare/` compiles
- [ ] Add `bufio` import to tunnel.go (for parseOutput)
- [ ] Run `go mod tidy`

### Phase 2: Wire CF Webhook to Server
- [ ] Add `/webhook/cf` route in `server/server.go`
- [ ] Add `/logpush/*` routes
- [ ] Test with `curl -X POST http://localhost:9090/webhook/cf`
- [ ] Verify events are logged

### Phase 3: Add cloudflared Tunnel Option
- [ ] Add `TUNNEL_TYPE` env var handling in server.go
- [ ] Implement cloudflared tunnel start/stop
- [ ] Add `sync cf tunnel` command
- [ ] Test: `TUNNEL_TYPE=cloudflared sync serve`

### Phase 4: Add CF Audit Polling
- [ ] Implement audit poller with real CF API calls
- [ ] Add `sync cf poll` command
- [ ] Integrate into `sync serve` when CF enabled
- [ ] Test with real CF account

### Phase 5: Event Bus Integration
- [ ] Create `sync/pkg/events/events.go`
- [ ] Refactor GitHub webhook to emit Events
- [ ] Refactor CF handlers to emit Events
- [ ] Add event handlers for logging, state updates

### Phase 6: Actions on Events
- [ ] Map CF events to subsystem actions
- [ ] e.g., Pages deploy → `task docs:build`
- [ ] e.g., R2 upload complete → notify
- [ ] Add action configuration

### Phase 7: cf-worker (Optional)
- [ ] Finalize cf-worker implementation
- [ ] Add `cf-worker` to root Taskfile includes
- [ ] Document when/how to use
- [ ] Add deployment tasks

---

## Questions to Answer Together

### 1. Which CF events do you need?

| Event | Use Case | Priority |
|-------|----------|----------|
| Audit logs | Know when DNS/config changes | ? |
| Pages deploys | Know when docs deployed | ? |
| Workers deploys | Know when workers updated | ? |
| Tunnel health | Know when tunnel goes down | ? |
| R2 operations | Know when binaries uploaded | ? |

### 2. What actions on CF events?

| Event | Action |
|-------|--------|
| Pages deploy success | Log, notify? |
| R2 upload complete | Update download URLs? |
| Audit log: DNS change | Alert? |
| Tunnel disconnected | Reconnect? |

### 3. Tunnel preference?

- **cloudflared only** - More robust, but requires CF account
- **smee.io fallback** - Keep for quick GitHub webhook testing
- **Both available** - Config chooses which to use

### 4. cf-worker needed?

- **Yes** - Want edge aggregation, high volume events
- **No** - Direct webhooks via cloudflared tunnel sufficient
- **Later** - Start without, add if needed

---

## Files Created (Draft)

```
sync/pkg/cloudflare/
├── cloudflare.go    # Core client, event types
├── audit.go         # Audit log polling
├── webhook.go       # Notification/logpush handlers
├── tunnel.go        # cloudflared management
├── sync.go          # Orchestrator
└── README.md        # Architecture docs

cf-worker/
├── main.go          # Go Worker (syumai/workers)
├── go.mod           # Module definition
├── wrangler.toml    # Worker config
├── Makefile         # Build WASM
└── Taskfile.yml     # Task integration
```

---

## Next: Your Input

Please answer:

1. **Priority events?** (audit logs, pages deploys, etc.)
2. **Actions on events?** (log only, reload, notify?)
3. **Tunnel preference?** (cloudflared, smee, both?)
4. **cf-worker now or later?**

Then I'll proceed with the implementation phases.
