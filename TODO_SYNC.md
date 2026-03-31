# TODO: sync-gh Remaining Work

## ✅ Done
- `sync-gh check` — version check
- `sync-gh poll` — poll upstream repos
- `sync-gh poll-taskfiles` — poll Taskfile/mise version pins
- `sync-gh webhook` — webhook server (catches all events)
- `sync-gh tunnel` — smee.io SSE client (pure Go)
- `sync-gh tunnel-setup` — create smee channel + configure GitHub webhook
- `sync-gh clone/pull/fetch/checkout/tags` — git ops via go-git
- `sync-gh state` — capture GitHub state
- Phase 6a: `sync version-file` command
- Phase 6b: All git operation commands (clone, pull, fetch, checkout, tags)

## Remaining

### Phase 1: Additional Webhook Handlers
Register handlers for events we're not yet consuming:
- `OnWorkflowRunEventCompleted` — know when CI finishes
- `OnPageBuildEventAny` — know when docs deploy
- `OnDeploymentStatusEventAny` — deployment tracking

### Phase 2: Event Storage
- Define `Event` struct
- Append-only `sync-gh/.data/events.jsonl`
- `sync-gh events` and `sync-gh events tail` commands

### Phase 3: Package Restructure (optional)
Current `pkg/` has some empty packages (`gitops/`, `updater/`, `version/`). Clean up or consolidate.

### Phase 4: Status Command
`sync-gh status` — compare local HEAD, releases, pages build, last workflow run against GitHub state.

### Phase 5: NATS Integration (optional)
Publish events to NATS subjects (`plat.events.github.*`) for distributed event streaming.

### Phase 6c: GitHub Actions Optimization
Use `sync-git clone` instead of `actions/checkout@v4` in CI — faster, no git dependency, same tool as local.

### Update Subsystem Tasks
Replace `git clone` shell calls with `sync-git clone` in arc, liftbridge, telegraf, docs src:clone tasks.
