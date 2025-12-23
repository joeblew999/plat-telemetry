# CLAUDE

## 1. Core Principles

### 1.1 Behavior
- **DOG FOOD** - Do it yourself, don't tell the user to do things
- **ALWAYS RUNNING** - Keep `task start` or `task start:fg` running so you can't cheat
- **PROJECT ISOLATION** - Never touch the OS; use project-level encapsulation
- **LOCAL FIRST** - Never push to GitHub CI and pray; run `task ci` locally first
- **SINGLE CI** - Only ever have one GitHub workflow for CI

### 1.2 Philosophy
- **Task is the only interface** - DEV, USER, CI, services all use identical `task` commands
- **Idempotency everywhere** - Every task must be safe to run repeatedly
- **One workflow for all** - DEV builds from source, USER downloads binaries, but `task start` works for both

---

## 2. Taskfile Specification

### 2.1 Variable Naming

**ALL variables in subsystem Taskfiles MUST be prefixed with the subsystem name (uppercase).**

Task includes all subsystem Taskfiles into a single namespace. Unprefixed variables leak between subsystems and get overwritten by the last include.

```yaml
# GOOD - nats/Taskfile.yml
vars:
  NATS_BIN_NAME: nats-server
  NATS_UPSTREAM_REPO: https://github.com/nats-io/nats-server.git
  NATS_VERSION: '{{.NATS_VERSION | default "v2.10.24"}}'
  NATS_SRC: '{{.TASKFILE_DIR}}/.src'
  NATS_BIN: '{{.TASKFILE_DIR}}/.bin'
  NATS_BIN_PATH: '{{.NATS_BIN}}/{{.NATS_BIN_NAME}}'
  NATS_DATA: '{{.TASKFILE_DIR}}/.data'

# BAD - these WILL break when Taskfiles are included together
vars:
  BIN_NAME: nats-server      # WRONG - no prefix
  UPSTREAM_REPO: ...         # WRONG - no prefix
  VERSION: ...               # WRONG - no prefix
```

**Standard variable pattern:**
```yaml
vars:
  <PREFIX>_BIN_NAME: <binary-name>
  <PREFIX>_UPSTREAM_REPO: <git-url>
  <PREFIX>_VERSION: '{{.<PREFIX>_VERSION | default "<version>"}}'
  <PREFIX>_SRC: '{{.TASKFILE_DIR}}/.src'
  <PREFIX>_BIN: '{{.TASKFILE_DIR}}/.bin'
  <PREFIX>_BIN_PATH: '{{.<PREFIX>_BIN}}/{{.<PREFIX>_BIN_NAME}}'
  <PREFIX>_DATA: '{{.TASKFILE_DIR}}/.data'
  <PREFIX>_PORT: '{{.<PREFIX>_PORT | default "<default-port>"}}'  # For services with HTTP/RPC
```

**Port variable benefits:**
- Enables inspection via `task <subsystem>:config:port`
- Allows port conflict resolution
- Supports process killing by port: `lsof -ti:{{.<PREFIX>_PORT}} | xargs kill`
- Consistent pattern for health checks: `curl http://localhost:{{.<PREFIX>_PORT}}/health`

**Root-level variables** (defined in root Taskfile.yml only):
- `RELEASE_REPO` - GitHub repository for releases
- `RELEASE_VERSION` - Release version tag
- `DIST_DIR` - Directory for packaged release artifacts
- `SUBSYSTEMS_BUILD` - Space-separated list of subsystems that build binaries
- `SUBSYSTEMS_RELEASE` - Space-separated list of subsystems to release

### 2.2 Task Naming

**Semantics:**
| Task | Level | Purpose |
|------|-------|---------|
| `start` | root | Orchestrate all services via Process Compose |
| `run` | subsystem | Execute a pre-built binary |
| `dev:run` | subsystem | Go only: compile+execute from source (`go run`) |
| `bin:build` | subsystem | Compile source to binary |
| `test`, `health` | both | Short-lived commands that return immediately |

**Standard tasks per subsystem:**
- **src:** `src:clone`, `src:update`
- **bin:** `bin:build`, `bin:download`
- **dev:** `dev:run` (Go subsystems only)
- **service:** `config:version`, `deps`, `ensure`, `health`, `install`, `package`, `run`, `test`
- **clean:** `clean`, `clean:all`, `clean:data`, `clean:src`

**Root aggregator tasks:**
- `src:clone`, `src:update` - Manage all subsystem sources
- `bin:build`, `bin:download` - Build/download all binaries
- `package` - Package all binaries for release
- `test`, `deps` - Run for all subsystems
- `clean`, `clean:data`, `clean:src`, `clean:all` - Clean all subsystems
- `ci:dist` - Output DIST_DIR path for CI

### 2.3 Task Ordering

Within subsystem Taskfiles, tasks MUST appear in this order:
1. `src:` tasks
2. `bin:` tasks
3. `dev:` tasks
4. Service tasks (alphabetically)
5. `clean:` tasks (alphabetically)

### 2.4 Idempotency

Every task MUST be safe to run repeatedly:
```yaml
# Use status: to skip if already done
src:clone:
  status:
    - test -d {{.NATS_SRC}}
  cmds:
    - git clone ...

# Use sources:/generates: for incremental builds
bin:build:
  sources:
    - "{{.NATS_SRC}}/**/*.go"
  generates:
    - "{{.NATS_BIN_PATH}}"
  cmds:
    - go build ...

# Use deps: chains so tasks auto-satisfy dependencies
run:
  deps: [ensure]
  cmds:
    - '{{.NATS_BIN_PATH}} --config ...'
```

### 2.5 Sorting Rules

**Alphabetically sort in root files:**
- `includes:` section in Taskfile.yml
- `deps:` and `cmds:` lists that call multiple subsystems
- Process definitions in process-compose.yaml
- `depends_on:` lists in process-compose.yaml

---

## 3. Subsystem Template

### 3.1 Directory Structure
```
<subsystem>/
├── Taskfile.yml     # All tasks for this subsystem
├── .src/            # Cloned source code
├── .bin/            # Compiled binaries
│   ├── <binary>     # The binary
│   └── .version     # Version metadata
└── .data/           # Runtime data
```

### 3.2 Version File Format
```
commit: <short-sha>
timestamp: <ISO8601>
checksum: <SHA256>
```

### 3.3 Canonical Example

```yaml
version: '3'

vars:
  NATS_BIN_NAME: nats-server
  NATS_UPSTREAM_REPO: https://github.com/nats-io/nats-server.git
  NATS_VERSION: '{{.NATS_VERSION | default "v2.10.24"}}'
  NATS_SRC: '{{.TASKFILE_DIR}}/.src'
  NATS_BIN: '{{.TASKFILE_DIR}}/.bin'
  NATS_BIN_PATH: '{{.NATS_BIN}}/{{.NATS_BIN_NAME}}'
  NATS_DATA: '{{.TASKFILE_DIR}}/.data'

env:
  GOWORK: off

tasks:
  # src: tasks
  src:clone:
    desc: Clone upstream repository
    cmds:
      - git clone --branch {{.NATS_VERSION}} --depth 1 {{.NATS_UPSTREAM_REPO}} {{.NATS_SRC}}
    status:
      - test -d {{.NATS_SRC}}

  src:update:
    desc: Update to pinned version
    deps: [src:clone]
    dir: '{{.NATS_SRC}}'
    cmds:
      - git fetch --tags
      - git checkout {{.NATS_VERSION}}

  # bin: tasks
  bin:build:
    desc: Build binary from source
    deps: [src:clone]
    dir: '{{.NATS_SRC}}'
    status:
      - test -f {{.NATS_BIN_PATH}}
    cmds:
      - mkdir -p {{.NATS_BIN}}
      - go build -o {{.NATS_BIN_PATH}} .
      - |
        {
          echo "commit: $(git rev-parse --short HEAD)"
          echo "timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
          echo "checksum: $(shasum -a 256 {{.NATS_BIN_PATH}} | awk '{print $1}')"
        } > {{.NATS_BIN}}/.version

  bin:download:
    desc: Download pre-built binary
    vars:
      GOOS:
        sh: go env GOOS
      GOARCH:
        sh: go env GOARCH
    cmds:
      - mkdir -p {{.NATS_BIN}}
      - curl -L {{.RELEASE_URL}}/{{.NATS_BIN_NAME}}-{{.GOOS}}-{{.GOARCH}}.tar.gz | tar xz -C {{.NATS_BIN}}
    status:
      - test -f {{.NATS_BIN_PATH}}

  # dev: tasks
  dev:run:
    desc: Run from source (development)
    deps: [src:clone]
    dir: '{{.NATS_SRC}}'
    cmds:
      - go run . --config {{.TASKFILE_DIR}}/nats.conf

  # Service tasks (alphabetical)
  config:version:
    desc: Output pinned version (for sync poller)
    cmds:
      - echo "{{.NATS_VERSION}}"
    silent: true

  deps:
    desc: Download Go dependencies
    deps: [src:clone]
    dir: '{{.NATS_SRC}}'
    cmds:
      - go mod download

  ensure:
    desc: Ensure binary exists
    status:
      - test -f {{.NATS_BIN_PATH}}
    cmds:
      - task: bin:download

  health:
    desc: Check service health
    cmds:
      - curl -sf http://localhost:8222/healthz > /dev/null

  package:
    desc: Package binary for release
    vars:
      GOOS: '{{.GOOS | default OS}}'
      GOARCH: '{{.GOARCH | default ARCH}}'
      DIST: '{{.DIST | default .DIST_DIR}}'
    cmds:
      - tar -czvf {{.DIST}}/{{.NATS_BIN_NAME}}-{{.GOOS}}-{{.GOARCH}}.tar.gz -C {{.NATS_BIN}} {{.NATS_BIN_NAME}}

  run:
    desc: Run binary
    deps: [ensure]
    cmds:
      - mkdir -p {{.NATS_DATA}}
      - '{{.NATS_BIN_PATH}} --config nats.conf'

  test:
    desc: Run tests
    deps: [src:clone]
    dir: '{{.NATS_SRC}}'
    cmds:
      - go test -v ./...

  # clean: tasks (alphabetical)
  clean:
    desc: Clean build artifacts
    cmds:
      - rm -rf {{.NATS_BIN}}

  clean:all:
    desc: Clean everything
    cmds:
      - rm -rf {{.NATS_SRC}} {{.NATS_BIN}} {{.NATS_DATA}}

  clean:data:
    desc: Clean runtime data
    cmds:
      - rm -rf {{.NATS_DATA}}

  clean:src:
    desc: Clean source
    cmds:
      - rm -rf {{.NATS_SRC}}
```

---

## 4. Process Compose Specification

### 4.1 Delegation Rule

Process Compose ONLY orchestrates. All implementation lives in Taskfiles.

```yaml
# GOOD - delegates to task
nats:
  command: task nats:run
  readiness_probe:
    exec:
      command: task nats:health

# BAD - calls binary directly
nats:
  command: nats/.bin/nats-server --config nats/nats.conf
```

### 4.2 Required Fields

```yaml
<service>:
  command: task <subsystem>:run
  depends_on:
    <dependency>:
      condition: process_healthy
  readiness_probe:
    exec:
      command: task <subsystem>:health
    initial_delay_seconds: 2
    period_seconds: 5
```

---

## 5. Workflows

### 5.1 DEV Workflow (build from source)
```bash
task src:clone        # Clone all sources
task bin:build        # Build all binaries
task start:fg         # Start all services

# After code changes:
task nats:bin:build   # Rebuild specific subsystem
task reload PROC=nats # Hot-reload the service
```

### 5.2 USER Workflow (download binaries)
```bash
task bin:download     # Download all binaries
task start:fg         # Start all services

# After new release:
task nats:bin:download # Download updated binary
task reload PROC=nats  # Hot-reload the service
```

### 5.3 CI Workflow
```bash
task ci               # Run full CI locally (MUST pass before push)
task ci:build         # Build all binaries
task ci:test          # Run all tests
task ci:package       # Package for release
task ci:pages         # Build docs
```

### 5.4 Round-Trip Testing
Before pushing ANY changes:
1. `task ci` - validates build, tests, packaging
2. `task start:fg` in background - validates services stay healthy
3. `task test:reload:all` - validates hot-reload workflows
4. Only push after ALL local validation passes

---

## 6. Language-Specific

### 6.1 Go

**Set GOWORK at subsystem level:**
```yaml
env:
  GOWORK: off
```

**Never shell out to git binary.** Use `sync clone` and `sync pull` commands:
```yaml
# BAD - assumes git binary exists
src:clone:
  cmds:
    - git clone --branch {{.VERSION}} {{.UPSTREAM_REPO}} {{.SRC_DIR}}

# GOOD - uses sync binary with go-git
src:clone:
  cmds:
    - sync/.bin/sync clone {{.UPSTREAM_REPO}} {{.SRC_DIR}} {{.VERSION}}
```

---

## 7. Sync Subsystem (Automated Updates)

### 7.1 Monitoring Approach

| Method | Use Case | Trigger |
|--------|----------|---------|
| GitHub polling | Upstream repos (nats-io, influxdata) | Every 5 minutes |
| GitHub webhooks | Our repos (joeblew999/*) | Push events |

### 7.2 Update Flow
```
Polling/Webhook detects change
  → Maps repo to subsystem
  → task sync:update SUBSYSTEM=<name>
  → task <subsystem>:src:update (DEV) or task <subsystem>:bin:download (USER)
  → task <subsystem>:bin:build (DEV only)
  → task reload PROC=<subsystem>
```

### 7.3 Manual Commands
```bash
task sync:check                    # Check all subsystems
task sync:check SUBSYSTEM=nats     # Check specific subsystem
```

---

## 8. Validation Specification

This section defines mandatory requirements for subsystem Taskfiles to enable automated validation.

### 8.1 Subsystem Types

| Type | Description | Examples |
|------|-------------|----------|
| `go-upstream` | Clone upstream Go repo, build binary | nats, liftbridge, telegraf, arc, gh, pc |
| `go-inrepo` | Build Go binary from in-repo code | sync, service |
| `prebuilt` | Download pre-built binary (no source) | grafana |
| `tool` | Download external tool + deps management | utm |
| `docs` | Documentation/static site generator | docs |

### 8.2 Mandatory Variables

**All subsystem types MUST have:**
```yaml
vars:
  <PREFIX>_BIN_NAME: <string>           # Binary name
  <PREFIX>_BIN: '{{.TASKFILE_DIR}}/.bin'  # Binary directory
  <PREFIX>_BIN_PATH: '{{.<PREFIX>_BIN}}/{{.<PREFIX>_BIN_NAME}}'  # Full path (see exceptions)
```

**Exceptions:**
- `prebuilt` type may have nested paths if tarball extracts with subdirectories (e.g., `{{.GF_BIN}}/bin/{{.GF_BIN_NAME}}` for Grafana)

**Additional by type:**

| Type | Additional Mandatory Variables |
|------|-------------------------------|
| `go-upstream` | `<PREFIX>_UPSTREAM_REPO`, `<PREFIX>_VERSION`, `<PREFIX>_SRC`, `<PREFIX>_DATA` |
| `go-inrepo` | `<PREFIX>_DATA` (if runtime data needed) |
| `prebuilt` | `<PREFIX>_VERSION`, `<PREFIX>_DOWNLOAD_URL`, `<PREFIX>_DATA` |
| `tool` | `<PREFIX>_VERSION` |
| `docs` | `<PREFIX>_SRC`, `<PREFIX>_DIST`, `<PREFIX>_VERSION` |

### 8.3 Mandatory Tasks

**All subsystem types MUST have:**
```
ensure        # Ensure binary exists (idempotent)
clean         # Clean build artifacts
```

**By type:**

| Type | Mandatory Tasks |
|------|-----------------|
| `go-upstream` | `src:clone`, `src:update`, `bin:build`, `bin:download`, `dev:run`, `deps`, `run`, `test`, `package`, `clean:all`, `clean:data`, `clean:src` |
| `go-inrepo` | `bin:build`, `test` (if testable), `package` (if distributable) |
| `prebuilt` | `bin:download`, `run`, `clean:all`, `clean:data` |
| `tool` | `bin:download`, `deps` |
| `docs` | `bin:build` or `bin:download`, `run`, `build`, `test`, `clean:all`, `clean:bin`, `clean:src` |

**Notes:**
- `go-inrepo` subsystems may replace `run` with domain-specific tasks (e.g., `service` uses `install`, `start`, `stop`)
- `prebuilt` subsystems don't need `src:*` tasks since there's no source to clone

**For Process Compose services (subsystems in process-compose.yaml):**
```
health        # Health check for readiness probe
run           # Long-running service execution
```

### 8.4 Task Requirements

**`ensure` task MUST:**
- Have `status:` checking `test -f {{.<PREFIX>_BIN_PATH}}`
- Call `bin:download` or `bin:build` on failure

**`bin:build` task MUST (Go types):**
- Have `deps: [src:clone]` or equivalent
- Output binary to `{{.<PREFIX>_BIN_PATH}}`
- Create `.version` file with commit, timestamp, checksum

**`bin:download` task MUST:**
- Have `status:` checking binary exists
- Use `{{.<PREFIX>_BIN_NAME}}` in download URL for cross-platform
- Create `.version` file

**`run` task MUST:**
- Have `deps: [ensure]`
- Use `{{.<PREFIX>_BIN_PATH}}` to execute binary

**`package` task MUST:**
- Accept `GOOS`, `GOARCH`, `DIST` vars with defaults
- Use `{{.<PREFIX>_BIN_NAME}}` in archive filename
- Output to `{{.DIST}}/<PREFIX>_BIN_NAME}}-{{.GOOS}}-{{.GOARCH}}.tar.gz`

**`clean` tasks MUST:**
- Use prefixed variables (`{{.<PREFIX>_BIN}}`, `{{.<PREFIX>_SRC}}`, `{{.<PREFIX>_DATA}}`)

### 8.5 Environment Requirements

**Go subsystems MUST have:**
```yaml
env:
  GOWORK: off
```

### 8.6 Validation Checklist

Use this checklist to validate a subsystem Taskfile:

```
[ ] All variables use <PREFIX>_ naming
[ ] <PREFIX>_BIN_NAME defined
[ ] <PREFIX>_BIN defined as '{{.TASKFILE_DIR}}/.bin'
[ ] <PREFIX>_BIN_PATH defined as '{{.<PREFIX>_BIN}}/{{.<PREFIX>_BIN_NAME}}'
[ ] ensure task has status: and calls bin:download/build
[ ] run task has deps: [ensure]
[ ] run task uses {{.<PREFIX>_BIN_PATH}}
[ ] package task uses {{.<PREFIX>_BIN_NAME}} in filename
[ ] clean tasks use prefixed variables
[ ] Go subsystems have env: GOWORK: off
[ ] Tasks appear in correct order (src, bin, dev, service, clean)
```

---

## 9. Remote Taskfiles

Subsystem Taskfiles are published to GitHub Pages for remote consumption. This enables users to run tasks without cloning the repository.

### 9.1 How It Works

1. **Build copies Taskfiles**: `task docs:build` copies all subsystem Taskfiles to `docs/static/taskfiles/`
2. **Hugo serves them**: GitHub Pages serves files at `https://joeblew999.github.io/plat-telemetry/taskfiles/<subsystem>/Taskfile.yml`
3. **Users include remotely**: Remote Taskfiles can be included via HTTPS URLs

### 9.2 Usage

**For users (download and run remotely):**
```bash
# Download the remote Taskfile
curl -o Taskfile.yml https://joeblew999.github.io/plat-telemetry/Taskfile.remote.yml

# Enable remote taskfiles and run
TASK_X_REMOTE_TASKFILES=1 task --yes bin:download
TASK_X_REMOTE_TASKFILES=1 task --yes start
```

**Include in your own Taskfile:**
```yaml
version: '3'

includes:
  plat:
    taskfile: https://joeblew999.github.io/plat-telemetry/Taskfile.remote.yml

tasks:
  start:
    cmds:
      - task plat:start
```

### 9.3 Local Testing

Test remote Taskfiles locally while Hugo dev server is running:
```bash
# Start Hugo dev server
task docs:run &

# Copy taskfiles to static
task docs:taskfiles:copy

# Test remote include (uses localhost:1313)
TASK_X_REMOTE_TASKFILES=1 task -t Taskfile.remote-test.yml --list --insecure --yes
```

### 9.4 Files

| File | Purpose |
|------|---------|
| `docs/static/Taskfile.remote.yml` | User-facing remote Taskfile with HTTPS includes |
| `docs/static/taskfiles/<subsystem>/` | Copied subsystem Taskfiles |
| `Taskfile.remote-test.yml` | Local test file using localhost |

### 9.5 Requirements

- **Task experiment**: `TASK_X_REMOTE_TASKFILES=1` environment variable
- **Trust**: `--yes` flag or `--trusted-hosts` for non-interactive use
- **HTTPS**: Required for production (use `--insecure` for localhost HTTP testing)
