# TODO: Incremental Git-Based Updates — Remaining Work

## ✅ Done
- `sync-gh/` — GitHub webhook receiver, API polling, smee.io tunnel, tunnel-setup, state capture
- `sync-git/` — Pure Go git ops (clone, pull, fetch, checkout, tags, version-file) — no git binary needed
- `sync-cf/` — CF audit polling, CF webhooks, cloudflared tunnel
- go-git and githubevents libraries integrated
- `sync version-file` replaces shell-based `.version` creation in tasks

## Remaining

### Update Subsystem Tasks to Use sync-git
Tasks still use shell `git` binary for src:clone and version tracking. Replace:
```toml
# OLD (requires git binary)
["arc:src:clone"]
run = "git clone ..."

# NEW (pure Go, no dependency)
["arc:src:clone"]
depends = ["sync-git:ensure"]
run = 'sync-git/.bin/sync-git clone https://... arc/.src "${ARC_VERSION}"'
```

Subsystems to update: arc, liftbridge, telegraf, docs

### True Incremental CI
CI still rebuilds all subsystems on every commit. Goal: only rebuild subsystems whose upstream source actually changed.

Mechanism: `sync-gh` webhook detects which upstream repo fired → triggers only that subsystem's build.

### Zero-Polling
CI has a cron (`*/15 * * * *`) for polling. Once webhooks are the primary mechanism, remove the cron.
