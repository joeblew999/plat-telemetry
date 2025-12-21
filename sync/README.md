# sync

**Why this exists:** Know when third-party subsystems (NATS, Liftbridge, etc.) have source changes AND when our own binary releases are published - automatically apply updates.

## Problem

We integrate third-party subsystems (nats-server, liftbridge, telegraf, arc):
- **DEV problem**: When nats-io/nats-server pushes new code, we want to know and rebuild ONLY nats (not everything)
- **USER problem**: When we publish new binaries, users want to know and auto-update without git
- **Wasteful**: Current CI rebuilds ALL binaries on every commit (expensive, slow)

## Solution

The sync subsystem monitors BOTH:

1. **Upstream source repos** (nats-io/nats-server, liftbridge-io/liftbridge, etc.)
   - **Polling mode**: GitHub API polls every 5 minutes (for repos we don't control)
   - **Webhook mode**: GitHub webhook fires when they push code (if configured)
   - We rebuild ONLY that subsystem (INCREMENTAL)
   - Hot-reload the updated binary

2. **Our own releases** (joeblew999/plat-telemetry)
   - GitHub webhook fires when we publish binaries
   - USERs auto-download ONLY updated subsystems
   - No git binary required (embeds go-git/v5)

## Commands

```bash
# Check current versions
sync check

# Poll upstream repos for updates (5 minute interval)
sync poll

# Webhook server (for repos we control)
sync watch

# Git operations (no git binary needed)
sync clone <url> <path> [version]
sync pull <path>
```

## Architecture

- **cmd/** - Thin CLI layer (argument parsing, user feedback)
- **pkg/checker/** - Version comparison logic
- **pkg/gitops/** - Git operations via go-git/v5
- **pkg/poller/** - GitHub API polling via go-github/v80
- **pkg/webhook/** - GitHub webhook handlers via githubevents/v2

## Integration

- Webhook server on port 9090
- Poller service runs continuously (5 minute interval)
- Taskfile tasks: `sync:check`, `sync:update`
- Process Compose services: `sync` (webhooks), `sync-poller` (polling)
- Triggers `task reload PROC=<subsystem>` for hot-reload

See [CLAUDE.md](../CLAUDE.md) for full documentation.
