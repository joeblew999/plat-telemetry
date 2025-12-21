# TODO: Git-Based Version Automation

## Goal

Automate upstream version detection and binary rebuilds using **git fetch** and **GitHub webhooks** - NO POLLING!

## Problem Statement

**Current approach:**
- Each subsystem has a manually pinned `VERSION` in its Taskfile
- We manually check upstream repos for new releases
- We manually update VERSION and rebuild binaries
- No automation for detecting when upstream changes

**What we need:**
- Use git fetch to check upstream state (on-demand, not polling)
- GitHub webhooks to trigger checks when upstream actually changes
- Use git metadata as the source of truth for versioning
- Zero polling - event-driven architecture only

## Tools to Use

### 1. go-git (https://github.com/go-git/go-git)
**Purpose:** Pure Go implementation of git
**Use cases:**
- Clone/fetch upstream repositories
- Query commit history, tags, branches
- Compare local vs remote versions
- No `git` binary dependency

**Example:**
```go
import "github.com/go-git/go-git/v5"

// Check if upstream has new commits
repo, _ := git.PlainOpen("nats/.src")
remote, _ := repo.Remote("origin")
refs, _ := remote.List(&git.ListOptions{})
// Compare with local HEAD
```

### 2. githubevents (https://github.com/cbrgm/githubevents)
**Purpose:** GitHub webhook event handling
**Use cases:**
- Listen for push events on upstream repos
- Trigger rebuilds on new releases
- React to tag creation events
- Enable real-time version updates

**Example:**
```go
import "github.com/cbrgm/githubevents/githubevents"

handler := githubevents.New(secret)
handler.OnPushEvent(func(event PushEvent) error {
    // Upstream pushed new code
    // Trigger rebuild
})
```

## Architecture

### Component 1: Version Checker (Go Binary) - ON-DEMAND ONLY

**Name:** `version-checker`
**Location:** `vc/` (new subsystem)

**Responsibilities:**
1. Run `git fetch` on .src repos (on-demand, when triggered)
2. Compare local HEAD vs origin/main
3. Detect new tags/releases
4. Output: JSON manifest of available updates

**Key principle: NO POLLING!**
- Only runs when explicitly called: `task version:check`
- Only runs when webhook receives event
- Uses `git fetch` to query remote state
- Zero background processes, zero cron jobs

**Interface:**
```bash
# Manually check for updates (rare, for testing)
task version:check

# Output:
# {
#   "nats": {"current": "abc123", "latest": "def456", "update_available": true},
#   "arc": {"current": "xyz789", "latest": "xyz789", "update_available": false}
# }
```

**Implementation:**
- Uses go-git for `git fetch` operations
- Compares local .src HEAD with remote refs
- No caching needed - git fetch is fast
- Outputs structured data for automation

### Component 2: GitHub Webhook Receiver (PRIMARY TRIGGER)

**Name:** `gh-webhook`
**Location:** `ghw/` (new subsystem)

**Responsibilities:**
1. Receive GitHub webhook events from upstream repos
2. Validate HMAC signatures
3. Trigger `task version:check` on push/release events
4. Trigger rebuilds automatically

**This is the PRIMARY mechanism - not "optional"!**

**Interface:**
```bash
# Start webhook server (runs via Process Compose)
ghw/.bin/gh-webhook serve --port 8080 --secret $WEBHOOK_SECRET

# Webhook flow:
# 1. GitHub pushes to nats-io/nats-server
# 2. Webhook POST to our server
# 3. Validates signature
# 4. Runs: task version:check SUBSYSTEM=nats
# 5. If update available: task nats:src:update nats:bin:build
# 6. Runs: task reload PROC=nats
```

**Setup required:**
- Configure webhooks on upstream repos (or fork them and watch forks)
- Set webhook secret in .env
- Expose webhook endpoint (ngrok for dev, proper domain for prod)

### Component 3: Taskfile Integration

**New tasks in root Taskfile.yml:**

```yaml
version:check:
  desc: Check subsystems for upstream updates (git fetch, NO polling)
  cmds:
    - vc/.bin/version-checker check {{.SUBSYSTEM | default "all"}}

version:update:
  desc: Update subsystems with available updates
  cmds:
    - |
      # This runs AFTER webhook trigger or manual check
      UPDATES=$(vc/.bin/version-checker check --json)
      for subsystem in $(echo $UPDATES | jq -r '.[] | select(.update_available) | .name'); do
        echo "▶ Updating $subsystem..."
        task $subsystem:src:update
        task $subsystem:bin:build
        task bin:verify
        task reload PROC=$subsystem
      done

version:watch:
  desc: Start webhook server to watch upstream repos
  cmds:
    - task ghw:run  # Runs webhook server via Process Compose
```

**NO SCHEDULED CI CHECKS!**
- Webhooks notify us instantly when upstream changes
- Manual `task version:check` for testing only
- CI only builds/tests, doesn't poll versions

```yaml
name: Version Check

on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours
  workflow_dispatch:

jobs:
  check-versions:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Check for updates
        run: task version:check

      - name: Create PR if updates available
        if: updates_available
        run: |
          task version:update
          # Create PR with changes
```

## Implementation Phases

### Phase 1: Version Poller Foundation
**Goal:** Basic git-based version detection

**Tasks:**
1. Create `vp/` subsystem directory structure
2. Implement go-git based poller
3. Add `version:check` task to root Taskfile
4. Test with NATS subsystem

**Files:**
- `vp/main.go` - Entry point
- `vp/poller/poller.go` - Git polling logic
- `vp/config/config.go` - Subsystem configuration
- `vp/Taskfile.yml` - Build tasks

**Success criteria:**
- `task version:check` shows current vs latest versions
- Uses go-git (no git binary needed)
- Outputs JSON for automation

### Phase 2: Automatic Updates
**Goal:** Auto-update subsystems when upstream changes

**Tasks:**
1. Implement `version:update` task
2. Add dry-run mode for safety
3. Integration with reload system
4. Test hot-reload after auto-update

**Success criteria:**
- `task version:update` rebuilds changed subsystems
- `task reload` works with auto-updated binaries
- All version verification passes

### Phase 3: GitHub Webhooks (PRIMARY)
**Goal:** Event-driven version updates - NO POLLING!

**Tasks:**
1. Implement webhook receiver using githubevents
2. Add to Process Compose (runs as service)
3. Configure webhooks on upstream repos (or forks)
4. Test webhook → version:check → rebuild → reload flow

**Success criteria:**
- Webhook server runs as PC service
- Receives push/release events from upstream
- Triggers `git fetch` + rebuild automatically
- Hot-reloads updated service
- Zero polling, instant updates

### Phase 4: Webhook Deployment
**Goal:** Production webhook hosting

**Options:**
1. **Self-hosted:** Run webhook server alongside PC
   - Pros: Full control, zero cost
   - Cons: Need public endpoint (ngrok/cloudflare tunnel)

2. **Cloud function:** Deploy to Cloudflare Workers/AWS Lambda
   - Pros: Auto-scaling, built-in HTTPS
   - Cons: Needs to trigger our server somehow

3. **GitHub Actions:** Webhook triggers workflow dispatch
   - Pros: No hosting needed
   - Cons: Still need to notify our server

**Recommended: Self-hosted + Cloudflare Tunnel**
- Free, permanent public URL
- Webhook server runs in PC
- Zero polling, instant updates

## Version Manifest Schema

**File:** `.version-manifest.json`

```json
{
  "last_check": "2025-12-21T15:45:00Z",
  "subsystems": {
    "nats": {
      "name": "nats",
      "upstream_repo": "https://github.com/nats-io/nats-server",
      "current_commit": "1d6f7ea",
      "current_tag": "v2.10.24",
      "latest_commit": "abc1234",
      "latest_tag": "v2.10.25",
      "update_available": true,
      "last_updated": "2025-12-21T08:35:43Z"
    },
    "arc": {
      "name": "arc",
      "upstream_repo": "https://github.com/arcopen/arc",
      "current_commit": "2d51120",
      "latest_commit": "2d51120",
      "update_available": false,
      "last_updated": "2025-12-21T08:36:02Z"
    }
  }
}
```

## Configuration Per Subsystem

Each subsystem Taskfile should declare its upstream source:

```yaml
vars:
  UPSTREAM_REPO: https://github.com/nats-io/nats-server
  UPSTREAM_BRANCH: main
  VERSION_STRATEGY: tags  # or 'commits' or 'releases'
```

The version checker reads these to know:
- Where to run `git fetch`
- What branch to track
- Whether to follow tags/releases or latest commits

## Benefits

1. **Event-driven updates** - Webhooks notify instantly, ZERO POLLING
2. **Faster security updates** - Know within seconds of upstream push
3. **Consistent versioning** - Git as single source of truth
4. **Resource efficient** - No cron jobs, no background polling
5. **Real-time updates** - Webhooks are the PRIMARY mechanism
6. **Zero manual intervention** - Fully automated end-to-end
7. **Integrates with hot-reload** - Auto rebuild + reload on upstream changes

## Migration Path

### Week 1: Foundation
- Create `vc/` subsystem
- Implement `git fetch` based checking
- Test with one subsystem (NATS)

### Week 2: Webhook Server
- Create `ghw/` subsystem using githubevents
- Add to Process Compose
- Test webhook → version:check flow

### Week 3: Integration
- Add `version:update` automation
- Integrate with existing `bin:verify` system
- Test webhook → rebuild → reload pipeline

### Week 4: Production
- Setup Cloudflare Tunnel for public endpoint
- Configure webhooks on upstream repos
- Add monitoring/alerting
- Document workflow in CLAUDE.md

## Open Questions

1. **Webhook source** - Watch upstream or forks?
   - **Option A:** Configure webhooks on upstream repos (nats-io/nats-server)
     - Pros: Direct notification
     - Cons: Need permission from maintainers
   - **Option B:** Fork repos and watch our forks
     - Pros: Full control
     - Cons: Need to keep forks in sync
   - **Recommended:** Start with forks, request upstream webhooks later

2. **Rate limiting** - Not an issue with webhooks!
   - Webhooks push to us (not polling)
   - `git fetch` only runs when notified
   - Zero API rate limit concerns

2. **Breaking changes** - How to detect incompatible updates?
   - Semver analysis (major version bumps)
   - Run regression tests before accepting update
   - Manual approval for major versions

3. **Multiple upstreams** - Some subsystems track forks?
   - Support multiple remotes in config
   - Priority order for version selection
   - Override mechanism per subsystem

4. **Rollback** - What if update breaks things?
   - Keep previous binary in .bin/.previous/
   - Automated rollback on test failure
   - Manual rollback command

## References

- go-git: https://github.com/go-git/go-git
- githubevents: https://github.com/cbrgm/githubevents
- GitHub Webhooks: https://docs.github.com/en/webhooks
- Semantic Versioning: https://semver.org/

## Next Actions

1. Create `vp/` directory structure
2. Add go-git dependency to go.mod
3. Implement basic poller for NATS
4. Test `task version:check` locally
5. Integrate with existing version system
