# TODO: Cloudflare Events — Remaining Work

## ✅ Done (in sync-cf/)
- CF audit log polling (`sync-cf poll`)
- CF notification webhook handler (`/webhook/cf`)
- cloudflared tunnel (`sync-cf tunnel`)
- CF logpush routes
- Audit log handler

## Remaining

### Phase 5: Unified Event Bus
Wire all sources (GitHub webhooks, CF webhooks, CF audit, GitHub poller) into a shared `events.jsonl` store.

```go
type Event struct {
    ID        string
    Source    string    // "github" | "cloudflare"
    Type      string    // e.g. "github.workflow_run.completed"
    Timestamp time.Time
    Payload   map[string]any
}
```

### Phase 6: Actions on Events
Map CF events to actions:
- Pages deploy → `mise run docs:build`
- R2 upload complete → notify / update download URLs
- Tunnel disconnected → reconnect

### Phase 7: cf-worker (Optional)
Edge fan-in Worker for high-volume events. Not needed until event volume warrants it.

## Questions Still Open
1. Which CF events actually matter? (audit logs, pages deploys, workers deploys, tunnel health)
2. Actions on events: log only, or trigger mise tasks?
3. cf-worker: now or later?
