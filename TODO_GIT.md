# TODO: Git-Based Version Automation

## Goal

Automate version detection and **INCREMENTAL binary updates** using **GitHub webhooks** - NO POLLING!

**CRITICAL: This runs on BOTH DEV and USER machines.**
- **DEVs**: Webhook monitors UPSTREAM source repos (nats-io, arcopen, etc.)
  - When nats-io/nats-server releases → rebuild ONLY nats → package ONLY nats → release
  - When arcopen/arc releases → rebuild ONLY arc → package ONLY arc → release
  - **INCREMENTAL**: Only rebuild what actually changed upstream
- **USERs**: Webhook monitors plat-telemetry releases
  - Download ONLY updated binaries → hot-reload ONLY affected services
- **1,000s of deployed machines** get real-time INCREMENTAL updates
- **Zero manual intervention** - fully automated update pipeline
- **CI optimization**: Only rebuild subsystems whose upstream source changed

## Problem Statement

**Current approach (BROKEN):**
- CI rebuilds ALL binaries on EVERY commit (wasteful!)
- Each subsystem has a manually pinned `VERSION` in its Taskfile
- We manually check upstream repos for new releases
- We manually update VERSION and rebuild ALL binaries
- No automation for detecting when upstream changes
- **CI is dumb** - rebuilds nats even when only arc source changed

**What we need:**
- **INCREMENTAL builds** - only rebuild what changed upstream
- GitHub webhooks to monitor UPSTREAM source repos (nats-io, arcopen, etc.)
- When nats-io releases → rebuild ONLY nats, package ONLY nats
- When arcopen releases → rebuild ONLY arc, package ONLY arc
- CI should do the SAME - only rebuild changed subsystems
- USERs download ONLY updated binaries
- Hot-reload ONLY affected services
- Zero polling - event-driven architecture only
- **Works for 1,000s of deployed USER machines** + DEV environments

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

**SERVICE subsystem running on BOTH DEV and USER machines**

### Subsystem: `sync/`

**Purpose:** Synchronize with releases and auto-update binaries in real-time
**Category:** SERVICE (runs in Process Compose like `nats/`, `arc/`)
**Scope:** BOTH DEV and USER (1,000s of deployed machines)

**Single binary with two modes:**
```bash
sync/.bin/sync watch --port 8080  # Webhook server (runs as service)
sync/.bin/sync check              # Manual version check (testing only)
```

**Directory structure:**
```
sync/
├── .bin/
│   ├── sync         # Single binary, distributed to USERs
│   └── .version
├── .src/            # Source code (DEV only)
├── main.go
├── cmd/
│   ├── check.go     # Version checking via GitHub API
│   └── watch.go     # Webhook server via githubevents
├── pkg/
│   ├── checker/     # GitHub release detection
│   └── webhook/     # githubevents handling
└── Taskfile.yml     # Standard service tasks (bin:build, bin:download, run, health)
```

**Process Compose integration (DEV and USER):**
```yaml
sync:
  command: task sync:run
  readiness_probe:
    exec:
      command: task sync:health
    initial_delay_seconds: 3
    period_seconds: 10
  # No dependencies - runs independently
  # BOTH DEVs and USERs run this service
```

### Mode 1: Version Checker (Manual testing only)

**Responsibilities:**
1. Check GitHub releases API for new versions
2. Compare current vs latest versions
3. Output: JSON manifest of available updates

**Key principle: NO POLLING!**
- Only runs when explicitly called: `task sync:check`
- Primarily for manual testing
- Webhook is the PRIMARY mechanism

**Interface:**
```bash
# Manual check for updates (testing only)
task sync:check

# Output:
# {
#   "nats": {"current": "v2.10.24", "latest": "v2.10.25", "update_available": true},
#   "arc": {"current": "v1.0.0", "latest": "v1.0.0", "update_available": false}
# }
```

### Mode 2: GitHub Webhook Receiver (SERVICE - PRIMARY MECHANISM)

**Responsibilities:**
1. Run as continuous service in Process Compose (DEV and USER)
2. Receive GitHub webhook events from release repos
3. Validate HMAC signatures
4. Trigger updates automatically
5. Hot-reload services after update

**This is the PRIMARY mechanism and runs on ALL machines!**

**Webhook flow - DEV machine (INCREMENTAL builds from source):**
```bash
# 1. DEV has all services running (including sync webhook server)
task start:fg

# 2. Upstream GitHub pushes new tag to nats-io/nats-server v2.10.25
#    → Webhook POST to sync service at http://dev-machine:8080/webhook/nats

# 3. sync service validates HMAC signature

# 4. sync service runs internal check:
#    Detects nats v2.10.24 → v2.10.25 update available

# 5. sync service executes INCREMENTAL DEV workflow:
#    task nats:src:update     # Pull latest source (ONLY nats!)
#    task nats:bin:build      # Rebuild from source (ONLY nats!)
#    task bin:verify          # Verify checksum

# 6. sync service hot-reloads ONLY affected service:
#    task reload PROC=nats

# 7. DEV packages ONLY updated binary:
#    task nats:package        # Package ONLY nats binary
#    gh release create v1.2.3-nats .dist/nats-*.tar.gz
#    OR
#    gh release upload v1.2.3 .dist/nats-*.tar.gz  # Add to existing release

# 8. Webhook fires for plat-telemetry release
#    → Notifies USER machines that NATS binary updated
```

**Key insight: INCREMENTAL everything!**
- Webhook specifies WHICH subsystem (nats, arc, liftbridge, etc.)
- Update ONLY that subsystem's source
- Rebuild ONLY that subsystem's binary
- Package ONLY that subsystem
- Release ONLY that binary (or add to existing release)
- USERs download ONLY the updated binary
- Hot-reload ONLY the affected service

**Webhook flow - USER machine (INCREMENTAL binary downloads):**
```bash
# 1. USER has all services running (including sync webhook server)
task start:fg

# 2. DEV uploads nats binary to plat-telemetry release v1.2.3
#    → Webhook POST to sync service at http://user-machine:8080/webhook/nats

# 3. sync service validates HMAC signature

# 4. sync service runs internal check:
#    Detects nats binary updated in release

# 5. sync service executes INCREMENTAL USER workflow:
#    task nats:bin:download   # Download ONLY nats binary
#    task bin:verify          # Verify checksum

# 6. sync service hot-reloads ONLY affected service:
#    task reload PROC=nats

# Result: USER's machine updated ONLY nats automatically
#         arc, liftbridge, telegraf unchanged (no unnecessary downloads/reloads)
```

**Real-time INCREMENTAL updates to 1,000s of machines:**
- nats-io releases v2.10.25 → DEV rebuilds ONLY nats → packages ONLY nats → uploads to release
- plat-telemetry release updated → ALL USER machines download ONLY nats + reload ONLY nats
- arcopen releases v1.0.1 → DEV rebuilds ONLY arc → packages ONLY arc → uploads to release
- plat-telemetry release updated → ALL USER machines download ONLY arc + reload ONLY arc
- **INCREMENTAL**: Only affected subsystems rebuilt/downloaded/reloaded
- Entire pipeline automated, zero polling, instant targeted updates
- Works for desktops, servers, all deployed environments

**Setup required:**
- **DEVs**: Configure webhooks on upstream repos (nats-io, arcopen, etc.)
- **USERs**: Configure webhook on joeblew999/plat-telemetry releases
- Both: Set webhook secret in .env
- Both: Expose webhook endpoint (ngrok, cloudflare tunnel, or public IP)
- Both: `sync` runs as service in Process Compose

### Taskfile Integration

**New tasks in root Taskfile.yml:**

```yaml
sync:check:
  desc: Check subsystems for updates (manual testing only)
  cmds:
    - sync/.bin/sync check {{.SUBSYSTEM | default "all"}}

sync:update:
  desc: Update subsystems with available updates (called by webhook)
  cmds:
    - |
      # Called by webhook when updates detected
      # Detects DEV vs USER mode automatically
      UPDATES=$(sync/.bin/sync check --json)
      for subsystem in $(echo $UPDATES | jq -r '.[] | select(.update_available) | .name'); do
        echo "▶ Updating $subsystem..."

        # DEV mode: rebuild from source
        if [ -d "$subsystem/.src" ]; then
          task $subsystem:src:update
          task $subsystem:bin:build
        # USER mode: download pre-built binary
        else
          task $subsystem:bin:download
        fi

        task bin:verify
        task reload PROC=$subsystem
      done
```

**sync/Taskfile.yml (standard service pattern with BOTH workflows):**

```yaml
version: '3'

vars:
  BIN_NAME: sync
  BIN_DIR: .bin
  BIN_PATH: "{{.BIN_DIR}}/{{.BIN_NAME}}"
  UPSTREAM_REPO: https://github.com/joeblew999/plat-telemetry
  UPSTREAM_BRANCH: main

# src: tasks (source management - DEV only)
src:clone:
  desc: Clone sync source (DEV only)
  cmds:
    - git clone {{.UPSTREAM_REPO}} .src
    - cd .src && git checkout {{.UPSTREAM_BRANCH}}
  status:
    - test -d .src

src:update:
  desc: Update sync source (DEV only)
  dir: .src
  cmds:
    - git fetch origin
    - git checkout {{.UPSTREAM_BRANCH}}
    - git pull origin {{.UPSTREAM_BRANCH}}

# bin: tasks (binary artifacts)
bin:build:
  desc: Build sync binary from source (DEV)
  sources:
    - ".src/**/*.go"
    - ".src/go.mod"
    - ".src/go.sum"
  generates:
    - "{{.BIN_PATH}}"
    - "{{.BIN_DIR}}/.version"
  env:
    GOWORK: off
  cmds:
    - mkdir -p {{.BIN_DIR}}
    - cd .src && go build -o ../{{.BIN_PATH}} .
    - |
      # Generate .version file
      cd .src
      echo "commit: $(git rev-parse --short HEAD)" > ../{{.BIN_DIR}}/.version
      echo "timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)" >> ../{{.BIN_DIR}}/.version
      echo "checksum: $(shasum -a 256 ../{{.BIN_PATH}} | awk '{print $1}')" >> ../{{.BIN_DIR}}/.version

bin:download:
  desc: Download pre-built sync binary (USER)
  vars:
    RELEASE_REPO: '{{.RELEASE_REPO | default "joeblew99/plat-telemetry"}}'
    RELEASE_VERSION: '{{.RELEASE_VERSION | default "latest"}}'
  cmds:
    - mkdir -p {{.BIN_DIR}}
    - |
      # Download sync binary from GitHub release
      gh release download {{.RELEASE_VERSION}} \
        --repo {{.RELEASE_REPO}} \
        --pattern "sync-{{OS}}-{{ARCH}}.tar.gz" \
        --output - | tar xz -C {{.BIN_DIR}}
    - chmod +x {{.BIN_PATH}}

# Service tasks
deps:
  desc: Download Go dependencies (DEV only)
  dir: .src
  cmds:
    - go mod download
  status:
    - test -d .src/vendor || test -f .src/go.sum

ensure:
  desc: Ensure binary exists (build or download if missing)
  cmds:
    - |
      # DEV mode: build from source
      if [ -d .src ]; then
        task bin:build
      # USER mode: download binary
      else
        task bin:download
      fi
  status:
    - test -f {{.BIN_PATH}}

health:
  desc: Check webhook server health
  cmds:
    - curl -f http://localhost:8080/health || exit 1

run:
  desc: Run webhook server (called by Process Compose)
  deps: [ensure]
  cmds:
    - "{{.BIN_PATH}} watch --port 8080 --secret ${WEBHOOK_SECRET}"

test:
  desc: Run unit tests (DEV only)
  dir: .src
  cmds:
    - go test -v ./...

package:
  desc: Package sync binary for release (DEV only)
  vars:
    DIST_DIR: '{{.DIST_DIR | default "../.dist"}}'
  cmds:
    - mkdir -p {{.DIST_DIR}}
    - tar czf {{.DIST_DIR}}/sync-{{.OS}}-{{.ARCH}}.tar.gz -C {{.BIN_DIR}} {{.BIN_NAME}} .version

# clean: tasks
clean:
  desc: Clean built binaries
  cmds:
    - rm -rf {{.BIN_DIR}}

clean:data:
  desc: Clean runtime data (none for sync)
  cmds:
    - echo "No data to clean"

clean:src:
  desc: Clean source directory (DEV only)
  cmds:
    - rm -rf .src
```

**NO SCHEDULED CI CHECKS!**
- Webhooks notify instantly when releases happen (PRIMARY mechanism)
- Manual `task sync:check` for testing only
- CI only builds/tests/packages, NEVER polls versions
- Works for BOTH DEV (build from source) and USER (download binaries)

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
