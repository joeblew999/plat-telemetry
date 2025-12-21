# TODO: Git-Based Version Automation

## Goal

Automate upstream version detection and binary rebuilds using git repository monitoring instead of manual version pinning.

## Problem Statement

**Current approach:**
- Each subsystem has a manually pinned `VERSION` in its Taskfile
- We manually check upstream repos for new releases
- We manually update VERSION and rebuild binaries
- No automation for detecting when upstream changes

**What we need:**
- Auto-detect when upstream repos have new commits/tags
- Trigger rebuilds automatically when new versions available
- Use git metadata as the source of truth for versioning
- Enable CI to poll upstream repos on a schedule

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

### Component 1: Version Poller (Go Binary)

**Name:** `version-poller`
**Location:** `vp/` (new subsystem)

**Responsibilities:**
1. Poll upstream git repos for changes (cron-like)
2. Compare upstream HEAD vs local .src HEAD
3. Detect new tags/releases
4. Output: JSON manifest of available updates

**Interface:**
```bash
# Check all subsystems for updates
vp/.bin/version-poller check-all

# Output:
# {
#   "nats": {"current": "abc123", "latest": "def456", "update_available": true},
#   "arc": {"current": "xyz789", "latest": "xyz789", "update_available": false}
# }
```

**Implementation:**
- Uses go-git to avoid git binary dependency
- Reads subsystem Taskfiles to find upstream repos
- Caches results to avoid excessive GitHub API calls
- Outputs structured data for Taskfile consumption

### Component 2: GitHub Webhook Receiver (Optional)

**Name:** `gh-webhook`
**Location:** `ghw/` (new subsystem)

**Responsibilities:**
1. Receive GitHub webhook events
2. Validate signatures
3. Trigger version poller on relevant events
4. Enable real-time updates instead of polling

**Interface:**
```bash
# Start webhook server
ghw/.bin/gh-webhook serve --port 8080 --secret $WEBHOOK_SECRET

# Receives POST from GitHub on:
# - push events
# - release events
# - tag creation
```

### Component 3: Taskfile Integration

**New tasks in root Taskfile.yml:**

```yaml
version:check:
  desc: Check all subsystems for upstream updates
  cmds:
    - vp/.bin/version-poller check-all

version:update:
  desc: Update subsystems with available updates
  cmds:
    - |
      UPDATES=$(vp/.bin/version-poller check-all --json)
      for subsystem in $(echo $UPDATES | jq -r '.[] | select(.update_available) | .name'); do
        echo "Updating $subsystem..."
        task $subsystem:src:update
        task $subsystem:bin:build
        task reload PROC=$subsystem
      done

version:auto:
  desc: Automatically update and reload changed subsystems
  cmds:
    - task: version:check
    - task: version:update
```

**CI Integration (.github/workflows/version-check.yml):**

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

### Phase 3: CI Automation
**Goal:** Scheduled version checks in CI

**Tasks:**
1. Add `.github/workflows/version-check.yml`
2. Implement auto-PR creation for updates
3. Run regression tests on version updates
4. Add notifications (Discord/Slack)

**Success criteria:**
- CI checks versions every 6 hours
- Creates PR when updates available
- Regression tests run before merge

### Phase 4: GitHub Webhooks (Optional)
**Goal:** Real-time version updates

**Tasks:**
1. Implement webhook receiver
2. Deploy to server/cloud function
3. Configure webhooks on upstream repos
4. Trigger version poller on events

**Success criteria:**
- Receives GitHub webhook events
- Triggers version check on push
- Near-instant update detection

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

The version poller reads these to know:
- Where to check for updates
- What branch to track
- Whether to follow tags/releases or latest commits

## Benefits

1. **Automatic version detection** - No manual checking
2. **Faster security updates** - Detect CVE fixes immediately
3. **Consistent versioning** - Git as single source of truth
4. **CI integration** - Scheduled checks and auto-PRs
5. **Real-time updates** - Optional webhooks for instant detection
6. **Zero manual intervention** - Fully automated pipeline

## Migration Path

### Week 1: Foundation
- Create `vp/` subsystem
- Implement basic git polling
- Test with one subsystem (NATS)

### Week 2: Integration
- Add `version:check` and `version:update` tasks
- Integrate with existing version verification
- Test hot-reload workflow

### Week 3: CI Automation
- Add scheduled version checks
- Implement auto-PR creation
- Run regression tests on updates

### Week 4: Production
- Enable for all subsystems
- Add monitoring/alerting
- Document workflow in CLAUDE.md

## Open Questions

1. **Rate limiting** - How to avoid GitHub API limits?
   - Cache results locally
   - Use conditional requests (If-None-Match)
   - Personal access token with higher limits

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
