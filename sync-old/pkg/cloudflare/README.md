# Cloudflare Event Integration

This package provides Cloudflare event integration for the sync service.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         CLOUDFLARE                                      │
├─────────────┬───────────────┬─────────────────┬────────────────────────┤
│ Audit Logs  │ Notifications │    Logpush      │  Pages/Workers Hooks   │
│   (API)     │  (Webhooks)   │  (HTTP Push)    │     (Webhooks)         │
└──────┬──────┴───────┬───────┴────────┬────────┴───────────┬────────────┘
       │              │                │                    │
       │   ┌──────────┴────────────────┴────────────────────┘
       │   │
       │   ▼
       │   ┌────────────────────────────────────────────────────────────┐
       │   │              CF-WORKER (optional edge fan-in)              │
       │   │         github.com/syumai/workers - Go WASM Worker         │
       │   │                                                            │
       │   │  Endpoints:                                                │
       │   │    /webhook/pages   - Pages deploy hooks                   │
       │   │    /webhook/alert   - Notification webhooks                │
       │   │    /logpush         - Logpush HTTP destination             │
       │   └────────────────────────────┬───────────────────────────────┘
       │                                │
       │                                ▼
       │              ┌─────────────────────────────────────┐
       │              │      CLOUDFLARE TUNNEL              │
       │              │   (cloudflared quick tunnel)        │
       │              │   https://xxx.trycloudflare.com     │
       │              └─────────────────┬───────────────────┘
       │                                │
       ▼                                ▼
┌──────────────────────────────────────────────────────────────────────┐
│                        SYNC SERVICE                                  │
│                                                                      │
│  ┌────────────────┐    ┌─────────────────┐    ┌───────────────────┐ │
│  │  Audit Poller  │    │ Webhook Handler │    │  Event Handlers   │ │
│  │  (polls API)   │    │   (HTTP POST)   │    │   (callbacks)     │ │
│  └───────┬────────┘    └────────┬────────┘    └────────▲──────────┘ │
│          │                      │                      │            │
│          └──────────────────────┴──────────────────────┘            │
│                                 │                                    │
│                    Normalized Event{Type, Action, Resource}          │
└──────────────────────────────────────────────────────────────────────┘
```

## Components

### 1. Core Client (`cloudflare.go`)
Main client that normalizes events from all sources.

```go
client, _ := cloudflare.NewClient(cloudflare.Config{
    APIToken:  os.Getenv("CF_API_TOKEN"),
    AccountID: os.Getenv("CF_ACCOUNT_ID"),
})

client.OnAny(func(ctx context.Context, event cloudflare.Event) error {
    log.Printf("Event: %s %s/%s", event.Type, event.Resource, event.Action)
    return nil
})
```

### 2. Audit Log Polling (`audit.go`)
Polls the Cloudflare Audit Logs API for account changes.

```go
poller := client.StartAuditPolling(ctx, 1*time.Minute)
defer poller.Stop()
```

### 3. Webhook Handler (`webhook.go`)
HTTP handlers for Cloudflare notification webhooks and Logpush.

```go
mux := http.NewServeMux()
client.RegisterRoutes(mux, "/cloudflare", "webhook-secret")
```

### 4. Tunnel Integration (`tunnel.go`)
Manages cloudflared tunnels for exposing local webhook endpoints.

```go
tunnel := cloudflare.NewTunnel(cloudflare.TunnelConfig{
    LocalPort: 8080,
})
tunnel.Start(ctx)
log.Printf("Webhook URL: %s/webhook", tunnel.URL())
```

### 5. Sync Orchestrator (`sync.go`)
Ties everything together with a single Start/Stop interface.

```go
sync, _ := cloudflare.NewSync(cloudflare.SyncConfig{
    APIToken:           os.Getenv("CF_API_TOKEN"),
    AccountID:          os.Getenv("CF_ACCOUNT_ID"),
    WebhookPort:        8080,
    EnableTunnel:       true,
    EnableAuditPolling: true,
    AuditPollInterval:  1 * time.Minute,
})

sync.OnEvent(func(ctx context.Context, event cloudflare.Event) error {
    // Handle any CF event
    return nil
})

sync.Start(ctx)
defer sync.Stop(ctx)
```

## Event Types

| Type | Source | Description |
|------|--------|-------------|
| `audit_log` | Audit API polling | Account configuration changes |
| `alert` | Notification webhook | Generic alerts |
| `logpush` | Logpush HTTP push | Traffic/security logs |
| `pages_deploy` | Pages deploy hook | Pages deployment events |
| `workers_deploy` | Notification webhook | Workers deployment events |
| `tunnel` | Notification webhook | Tunnel health events |

## CF-Worker (Edge Event Aggregator)

For edge fan-in (receiving webhooks at Cloudflare edge before forwarding), see `../../../cf-worker/`.

```bash
cd cf-worker
task deps           # Download dependencies
task bin:build      # Build WASM
task dev:run        # Local development
task deploy         # Deploy to Cloudflare
```

## Configuration

### Environment Variables

```bash
# Required for audit log polling
CF_API_TOKEN=xxx           # Cloudflare API token
CF_ACCOUNT_ID=xxx          # Cloudflare account ID

# For CF-Worker forwarding
SYNC_ENDPOINT=https://xxx  # Where worker forwards events
SYNC_TOKEN=xxx             # Auth token for sync service
```

### API Token Scopes

For full functionality, your API token needs:
- `Account:Audit Logs:Read` - For audit log polling
- `Account:Account Settings:Read` - For account info
- `Account:Notifications:Read` - For notification history
- `Account:Notifications:Write` - For webhook destination setup

## Usage with Sync Service

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/joeblew99/plat-telemetry/sync/pkg/cloudflare"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sync, err := cloudflare.NewSync(cloudflare.SyncConfig{
        APIToken:           os.Getenv("CF_API_TOKEN"),
        AccountID:          os.Getenv("CF_ACCOUNT_ID"),
        WebhookPort:        8080,
        EnableTunnel:       true,
        EnableAuditPolling: true,
        AuditPollInterval:  1 * time.Minute,
    })
    if err != nil {
        log.Fatal(err)
    }

    sync.OnEvent(func(ctx context.Context, event cloudflare.Event) error {
        log.Printf("[CF] %s: %s on %s", event.Type, event.Action, event.Resource)
        // Trigger sync action based on event...
        return nil
    })

    if err := sync.Start(ctx); err != nil {
        log.Fatal(err)
    }

    log.Printf("Cloudflare sync running. Tunnel URL: %s", sync.TunnelURL())

    // Wait for shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    log.Println("Shutting down...")
    sync.Stop(ctx)
}
```
