# TODO: sync Subsystem Evolution

## Current State

### What Exists

| Component | Purpose | Status |
|-----------|---------|--------|
| `sync check` | One-shot version check | Works |
| `sync poll` | Poll upstream repos hourly | Works |
| `sync poll-taskfiles` | Poll Taskfile version pins | Works |
| `sync webhook` | Webhook server (catches ALL events) | Works |
| `sync tunnel` | smee.io client (pure Go, no npm) | Works |
| `sync tunnel-setup` | Create channel + configure GitHub webhook | Works |
| `sync clone/pull` | Git ops via go-git | Works |

### Current Webhook Handlers

```go
// webhook.go - catch ALL events
handler.OnBeforeAny(func(ctx, deliveryID, eventName, event) error {
    log.Printf("📨 Event: %s [delivery: %s]", eventName, deliveryID)
    return nil
})
```

### Dependencies Already Available

```go
// go.mod
github.com/cbrgm/githubevents/v2  // Webhook event handling
github.com/google/go-github/v80   // GitHub API client
github.com/go-git/go-git/v5       // Pure Go git
```

---

## Available GitHub Webhook Events

The `githubevents` library supports ALL these events. We just need to register handlers:

| Event | Trigger | Use Case |
|-------|---------|----------|
| `workflow_run` | CI workflow completes | Know when CI finished |
| `workflow_job` | Individual job starts/completes | Granular CI tracking |
| `deployment` | Deployment created | Track deployments |
| `deployment_status` | Deployment status changes | Know deploy success/fail |
| `page_build` | Pages build attempted | Know when pages built |
| `release` | Release activity | **Already have** (published only) |
| `push` | Code pushed | **Already have** |
| `check_run` | Check run activity | CI check results |
| `check_suite` | Check suite activity | CI suite results |

**Key insight:** We don't need to poll for outbound results - GitHub will **push webhook events** to us when:
- CI workflow completes (`workflow_run`)
- Pages builds (`page_build`)
- Releases published (`release`)

---

## Proposed Package Redesign

### Current Structure (messy)

```
sync/
├── cmd/
│   ├── check.go
│   ├── gitops.go
│   ├── poll.go
│   ├── poll-taskfiles.go
│   └── watch.go
├── pkg/
│   ├── checker/      # version checking
│   ├── gitops/       # (empty?)
│   ├── poller/       # upstream polling
│   ├── taskfile-poller/
│   ├── updater/      # (empty?)
│   ├── version/      # (empty?)
│   └── webhook/      # webhook handlers
└── main.go
```

### Proposed Structure (clean)

```
sync/
├── cmd/
│   └── sync/
│       └── main.go           # CLI entry point
├── internal/
│   ├── events/
│   │   ├── event.go          # Event type definition
│   │   ├── store.go          # Event storage (file/NATS)
│   │   └── emitter.go        # Emit events
│   ├── github/
│   │   ├── client.go         # go-github wrapper
│   │   ├── webhook.go        # Webhook handlers
│   │   └── poller.go         # API polling
│   ├── git/
│   │   └── ops.go            # go-git operations
│   └── taskfile/
│       └── version.go        # Taskfile version reading
├── pkg/
│   └── sync/
│       └── sync.go           # Public API (if needed)
└── Taskfile.yml
```

---

## Event System Design

### Event Type

```go
type Event struct {
    ID        string            `json:"id"`
    Timestamp time.Time         `json:"timestamp"`
    Type      string            `json:"type"`      // e.g., "github.workflow_run.completed"
    Source    string            `json:"source"`    // "webhook" | "poll" | "local"
    Repo      string            `json:"repo"`      // e.g., "joeblew999/plat-telemetry"
    Payload   map[string]any    `json:"payload"`   // event-specific data
}
```

### Event Types We Care About

| Type | Source | Trigger |
|------|--------|---------|
| `github.push` | webhook | Code pushed |
| `github.release.published` | webhook | Release created |
| `github.workflow_run.completed` | webhook | CI finished |
| `github.workflow_run.failed` | webhook | CI failed |
| `github.page_build.success` | webhook | Pages deployed |
| `github.page_build.failed` | webhook | Pages failed |
| `upstream.update.available` | poll | Upstream has new version |
| `taskfile.version.changed` | poll | Taskfile version pin changed |

### Event Storage

**Phase 1: File-based (simple)**
```
sync/.data/events.jsonl   # Append-only JSON lines
```

**Phase 2: NATS JetStream (distributed)**
```
Subject: plat.events.github.*
Stream: PLAT_EVENTS
```

---

## New Webhook Handlers

Add to `webhook.go`:

```go
// CI workflow completed
handler.OnWorkflowRunEventCompleted(func(ctx context.Context, deliveryID string, eventName string, event *github.WorkflowRunEvent) error {
    run := event.GetWorkflowRun()
    log.Printf("✅ CI completed: %s [%s] - %s",
        run.GetName(),
        run.GetConclusion(),  // success, failure, cancelled
        run.GetHTMLURL())

    emitEvent("github.workflow_run.completed", map[string]any{
        "workflow": run.GetName(),
        "conclusion": run.GetConclusion(),
        "run_id": run.GetID(),
        "url": run.GetHTMLURL(),
    })
    return nil
})

// Pages build
handler.OnPageBuildEventAny(func(ctx context.Context, deliveryID string, eventName string, event *github.PageBuildEvent) error {
    build := event.GetBuild()
    log.Printf("📄 Pages build: %s", build.GetStatus())

    emitEvent("github.page_build", map[string]any{
        "status": build.GetStatus(),  // built, errored
        "error": build.GetError().GetMessage(),
    })
    return nil
})

// Deployment status (for environments)
handler.OnDeploymentStatusEventAny(func(ctx context.Context, deliveryID string, eventName string, event *github.DeploymentStatusEvent) error {
    status := event.GetDeploymentStatus()
    log.Printf("🚀 Deployment: %s -> %s",
        event.GetDeployment().GetEnvironment(),
        status.GetState())

    emitEvent("github.deployment.status", map[string]any{
        "environment": event.GetDeployment().GetEnvironment(),
        "state": status.GetState(),  // pending, success, failure, error
        "url": status.GetTargetURL(),
    })
    return nil
})
```

---

## New CLI Commands

```bash
# Existing
sync check              # Check versions
sync poll               # Poll upstream
sync poll-taskfiles     # Poll Taskfile versions
sync webhook            # Webhook server (catches ALL events)
sync clone/pull         # Git ops

# New
sync events             # List recent events (last 20)
sync events tail        # Stream events (like tail -f)
sync events tail --type=github.workflow_run.*  # Filter
sync status             # Compare local vs GitHub state
```

---

## Implementation Phases

### Phase 1: Add Missing Webhook Handlers
- [ ] `OnWorkflowRunEventCompleted` - CI finished
- [ ] `OnWorkflowRunEventRequested` - CI started
- [ ] `OnPageBuildEventAny` - Pages build
- [ ] `OnDeploymentStatusEventAny` - Deployment status
- [ ] Test with actual GitHub webhooks

### Phase 2: Event Storage
- [ ] Define `Event` struct
- [ ] Implement file-based event store (`.data/events.jsonl`)
- [ ] Emit events from all handlers
- [ ] `sync events` and `sync events tail` commands

### Phase 3: Package Restructure
- [ ] Move to `internal/` structure
- [ ] Consolidate duplicate code
- [ ] Clean up empty packages

### Phase 4: Status Command
- [ ] `sync status` - show local vs GitHub state
- [ ] Compare: HEAD, releases, pages, workflow runs

### Phase 5: NATS Integration (optional)
- [ ] Publish events to NATS subjects
- [ ] Enable distributed event streaming

---

## Webhook Setup (AUTOMATED)

No manual configuration needed! Use the built-in smee.io tunnel:

```bash
# One-time setup: creates smee channel + configures GitHub webhook
sync tunnel-setup joeblew999/plat-telemetry

# Or via task:
task sync:tunnel:setup REPO=joeblew999/plat-telemetry
```

This automatically:
1. Generates a random smee.io channel URL
2. Calls `gh api` to create the webhook in the GitHub repo
3. Prints the smee URL to use with `sync tunnel`

Then run the tunnel to receive events locally:
```bash
# Terminal 1: webhook server
task sync:run

# Terminal 2: smee tunnel
task sync:tunnel SMEE_URL=https://smee.io/YOUR_CHANNEL
```

**How it works:**
- smee.io is a free, open-source webhook relay service from GitHub
- Our `sync tunnel` is a pure Go SSE client (~100 lines, no npm)
- GitHub → smee.io → sync tunnel → sync webhook → logs events

---

## Questions

1. **Start with Phase 1?** Add the webhook handlers first, test they work?

2. **Event storage format?** JSON lines file is simplest. OK for now?

3. **Package restructure priority?** Do it now or after handlers work?

---

## Sources

- [GitHub Webhook Events and Payloads](https://docs.github.com/en/webhooks/webhook-events-and-payloads)
- [githubevents library](https://github.com/cbrgm/githubevents)

---

## Phase 6: sync as Foundation (Bootstrap Dependency)

### Vision

sync becomes the **first subsystem** that must be installed before anything else. It provides:

1. **Git Operations** - No git binary needed (go-git)
2. **Version Management** - Create `.version` files with commit/timestamp/checksum
3. **GitHub API** - Fetch releases, create webhooks, check workflow status
4. **Event Bus** - Receive and emit events for automation

### Why sync First?

Currently, Taskfiles assume `git` binary exists:
```yaml
# BAD - requires git binary
src:clone:
  cmds:
    - git clone --branch {{.VERSION}} {{.UPSTREAM_REPO}} {{.SRC}}

bin:build:
  cmds:
    - echo "commit: $(git rev-parse --short HEAD)" > .version
```

With sync as foundation:
```yaml
# GOOD - uses sync binary (pure Go, no dependencies)
src:clone:
  cmds:
    - sync clone {{.UPSTREAM_REPO}} {{.SRC}} {{.VERSION}}

bin:build:
  cmds:
    - sync version-file {{.BIN}}/.version --src={{.SRC}}
```

### New gitops Package Capabilities

Extend `pkg/gitops/` to handle all git-like operations:

```go
package gitops

// Existing
func Clone(url, path, version string) error
func Pull(path string) (string, error)
func GetCommitHash(path string) (string, error)

// New - replace git binary usage
func Checkout(path, ref string) error                    // git checkout
func Fetch(path string, tags bool) error                 // git fetch
func GetTags(path string) ([]string, error)              // git tag -l
func GetBranch(path string) (string, error)              // git branch --show-current
func Status(path string) (Status, error)                 // git status

// New - replace shell commands in Taskfiles
func WriteVersionFile(versionPath, srcPath string) error // creates .version file
func ReadVersionFile(path string) (Version, error)       // reads .version file

// Version metadata
type Version struct {
    Commit    string    `yaml:"commit"`
    Timestamp time.Time `yaml:"timestamp"`
    Checksum  string    `yaml:"checksum"`
}
```

### New CLI Commands

```bash
# Git operations (already have)
sync clone <url> <path> [version]
sync pull <path>

# New git operations
sync checkout <path> <ref>           # checkout tag/branch
sync fetch <path> [--tags]           # fetch remote
sync status <path>                   # show status

# Version file operations (replace shell in Taskfiles)
sync version-file <path>             # create .version file
sync version-file <path> --read      # read and output .version
sync version-file <path> --json      # output as JSON
```

### GitHub Actions Without git

Currently CI uses:
```yaml
- uses: actions/checkout@v4  # pulls 100MB of git
```

With sync:
```yaml
- name: Download sync
  run: curl -L .../sync-linux-amd64.tar.gz | tar xz

- name: Clone repo
  run: ./sync clone $GITHUB_REPOSITORY . $GITHUB_SHA
```

Benefits:
- Faster CI (no git, just HTTP download)
- Smaller container images
- Same tool for DEV/USER/CI

### Bootstrap Flow

```
1. User downloads sync binary (single file, ~10MB)
   curl -L .../sync-linux-amd64.tar.gz | tar xz

2. sync clones/downloads all other subsystems
   sync clone https://github.com/nats-io/nats-server nats/.src v2.10.24

3. Task builds binaries using sync for version metadata
   task nats:bin:build  # uses sync internally

4. sync monitors for updates and triggers rebuilds
   sync serve  # webhook server + event bus
```

### Migration Path

Phase 6a: Add version-file command ✅ DONE
- [x] `sync version-file <path>` creates .version from adjacent .src
- [x] `sync version-file <path> --src=<dir>` for explicit source dir
- [x] `sync version-file <path> --version=<tag>` for downloaded binaries
- [x] `sync version-file <path> --read` to read existing .version
- [ ] Update subsystem Taskfiles to use `sync version-file`

Phase 6b: Remove git binary dependency ✅ COMMANDS READY
- [x] `sync clone` - Pure Go clone via go-git
- [x] `sync pull` - Pure Go pull
- [x] `sync fetch` - Pure Go fetch with --tags support
- [x] `sync checkout` - Pure Go checkout tag/branch/commit
- [x] `sync tags` - List all tags
- [ ] Update subsystem Taskfiles to use sync commands

Phase 6c: GitHub Actions optimization
- [ ] Download sync binary as first step
- [ ] Use `sync clone` instead of `actions/checkout`
- [ ] Test CI speed improvement

### Current sync Commands

```bash
# Service commands
sync serve                          # Run webhook server + tunnel
sync state [repo] [--show]          # Capture/display GitHub state
sync check                          # Check for upstream updates
sync poll                           # Poll upstream repos
sync poll-taskfiles                 # Poll Taskfile version pins
sync webhook                        # Webhook server only
sync tunnel <url|new> [target]      # smee.io tunnel
sync tunnel-setup <owner/repo>      # Setup GitHub webhook

# Git operations (pure Go, no git binary needed)
sync clone <url> <path> [version]   # Clone repository
sync pull <path>                    # Pull updates
sync fetch <path> [--tags]          # Fetch from origin
sync checkout <path> <ref>          # Checkout tag/branch/commit
sync tags <path>                    # List all tags

# Version file operations
sync version-file <path>            # Create .version from .src
sync version-file <path> --src=<d>  # Use specific source dir
sync version-file <path> --version=v1.0.0  # Use version tag
sync version-file <path> --read     # Read existing .version
sync version-file <path> --json     # Output as JSON
```

---

## Package Structure (Updated)

After refactoring, cmd/ is now thin wrappers:

```
sync/
├── cmd/
│   ├── check.go          # calls pkg/checker
│   ├── gitops.go         # calls pkg/gitops (clone, pull, fetch, checkout, tags)
│   ├── poll.go           # calls pkg/poller
│   ├── poll-taskfiles.go # calls pkg/taskfile-poller
│   ├── serve.go          # calls pkg/server
│   ├── state.go          # calls pkg/github
│   ├── tunnel.go         # calls pkg/tunnel
│   ├── version.go        # calls pkg/version
│   └── webhook.go        # calls pkg/webhook
├── pkg/
│   ├── checker/          # version checking logic
│   ├── github/           # GitHub API (state capture)
│   ├── gitops/           # go-git operations (clone, pull, fetch, checkout, tags)
│   ├── poller/           # GitHub API polling
│   ├── server/           # combined service (webhook + tunnel)
│   ├── taskfile-poller/  # Taskfile version polling
│   ├── tunnel/           # smee.io SSE client
│   ├── version/          # .version file creation/reading
│   └── webhook/          # GitHub webhook handlers
└── main.go
```

All business logic is in `pkg/`. cmd/ just parses args and calls pkg functions.
