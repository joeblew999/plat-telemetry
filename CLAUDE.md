# CLAUDE

## 1. Core Principles

### 1.1 Behavior
- **DOG FOOD** - Do it yourself, don't tell the user to do things
- **ALWAYS RUNNING** - Keep services running via `pitchfork start --all` so you can't cheat
- **PROJECT ISOLATION** - Never touch the OS; use project-level encapsulation
- **LOCAL FIRST** - Never push to GitHub CI and pray; run `mise run ci` locally first
- **SINGLE CI** - Only ever have one GitHub workflow for CI

### 1.2 Philosophy
- **Mise is the only interface** - DEV, USER, CI, services all use identical `mise run` commands
- **Pitchfork orchestrates** - All daemon lifecycle managed by pitchfork, not scripts
- **Idempotency everywhere** - Every task must be safe to run repeatedly
- **One workflow for all** - DEV builds from source, USER downloads binaries, `mise run start` works for both

---

## 2. Tool Stack

| Tool | Purpose | Replaces |
|------|---------|----------|
| **mise** | Tool management, task runner, env vars | go-task (Taskfile.yml) |
| **pitchfork** | Daemon orchestration, process lifecycle | process-compose |
| **ubi** (mise backend) | Install binaries from GitHub releases | Custom bin:download tasks |

### 2.1 mise

Mise is the single entry point for all operations:
- **Tool management**: `[tools]` section installs Go, gh, nats-server, pitchfork via ubi
- **Task runner**: `[tasks]` and included TOML files define all build/run/test/deploy tasks
- **Environment**: `[env]` manages all configuration variables
- **Idempotency**: `sources`/`outputs` for file-based caching; shell checks for state

### 2.2 pitchfork

Pitchfork manages all long-running services:
- **Configuration**: `pitchfork.toml` at project root
- **Readiness**: `ready_http`, `ready_port`, `ready_cmd`, `ready_delay`, `ready_output`
- **Dependencies**: `depends = ["nats"]` ensures startup order
- **mise integration**: `[settings.general] mise = true` wraps all commands with `mise x --`
- **Auto start/stop**: `auto = ["start", "stop"]` with shell hooks
- **Retry**: `retry = true` for crash recovery with exponential backoff

---

## 3. Mise Task Specification

### 3.1 File Structure

```
.mise.toml              # Root: tools, env, task_config, root tasks
pitchfork.toml          # Daemon orchestration
<subsystem>/
├── mise-tasks.toml     # All tasks for this subsystem
├── .src/               # Cloned source code
├── .bin/               # Compiled binaries (for source-built subsystems)
│   ├── <binary>        # The binary
│   └── .version        # Version metadata
└── .data/              # Runtime data
```

### 3.2 Task Naming

**Semantics:**
| Task | Level | Purpose |
|------|-------|---------|
| `start` | root | Start all daemons via pitchfork |
| `<sub>:run` | subsystem | Execute a pre-built binary (long-running) |
| `<sub>:dev:run` | subsystem | Go only: compile+execute from source (`go run`) |
| `<sub>:bin:build` | subsystem | Compile source to binary |
| `<sub>:test`, `<sub>:health` | subsystem | Short-lived commands |

**Standard tasks per subsystem:**
- **src:** `src:clone`, `src:update`
- **bin:** `bin:build`, `bin:download`
- **dev:** `dev:run` (Go subsystems only)
- **service:** `config:version`, `deps`, `ensure`, `health`, `run`, `test`, `package`
- **clean:** `clean`, `clean:all`, `clean:data`, `clean:src`

### 3.3 Task Format (TOML)

**In `.mise.toml` (root):**
```toml
[tasks."task:name"]
description = "What this task does"
depends = ["other:task"]
env = { KEY = "value" }
dir = "relative/path"
sources = ["src/**/*.go"]
outputs = ["bin/binary"]
run = "command here"
```

**In included files (`<subsystem>/mise-tasks.toml`):**
```toml
["subsystem:task:name"]
description = "What this task does"
depends = ["subsystem:other:task"]
run = "command here"
```

Note: Included files use top-level task definitions (no `[tasks.]` prefix).

### 3.4 Idempotency Patterns

```toml
# Shell check - skip if already done
["nats:src:clone"]
run = """
[ -d nats/.src ] && exit 0
git clone --branch "$NATS_VERSION" --depth 1 https://github.com/nats-io/nats-server.git nats/.src
"""

# File-based caching - skip if outputs newer than sources
["sync-git:bin:build"]
sources = ["sync-git/**/*.go", "sync-git/go.mod"]
outputs = ["sync-git/.bin/sync-git"]
run = "cd sync-git && go build -o .bin/sync-git ."

# Ensure pattern - check binary, build if missing
["arc:ensure"]
run = """
[ -f arc/.bin/arc ] && exit 0
mise run arc:bin:build
"""
```

### 3.5 Variable Pattern

All configuration uses environment variables in `[env]` section of `.mise.toml`:

```toml
[env]
NATS_VERSION = "v2.10.24"
ARC_VERSION = "main"
RELEASE_REPO = "joeblew999/plat-telemetry"
_.file = [".env"]  # Load additional vars from .env
```

Tasks reference variables as `$NATS_VERSION`, `$RELEASE_REPO`, etc.

### 3.6 Platform Support

For cross-platform tasks, use `run_windows` for Windows-specific commands:
```toml
["example:task"]
run = "bash script.sh"
run_windows = "powershell script.ps1"
```

---

## 4. Pitchfork Specification

### 4.1 Delegation Rule

Pitchfork ONLY orchestrates. All implementation lives in mise tasks.

```toml
# GOOD - delegates to mise task
[daemons.nats]
run = "mise run nats:run"
ready_http = "http://localhost:8222/healthz"

# BAD - calls binary directly
[daemons.nats]
run = "nats/.bin/nats-server --config nats/nats.conf"
```

### 4.2 Daemon Configuration

```toml
[daemons.<service>]
run = "mise run <subsystem>:run"
depends = ["<dependency>"]
ready_http = "http://localhost:<port>/health"  # or ready_port, ready_cmd, ready_delay
retry = true
auto = ["start", "stop"]
```

### 4.3 Readiness Checks

| Method | Config Key | Use Case |
|--------|-----------|----------|
| HTTP polling | `ready_http` | Services with health endpoints |
| TCP port | `ready_port` | Services listening on known ports |
| Shell command | `ready_cmd` | Custom health checks |
| Output matching | `ready_output` | Services that print "ready" to stdout |
| Delay | `ready_delay` | Simple timeout (default: 3s) |

### 4.4 Settings

```toml
[settings.general]
mise = true  # Wrap all daemon commands with `mise x --`
```

---

## 5. Subsystem Types

### 5.1 mise-managed (ubi)

Tools installed directly from GitHub releases. No build tasks needed.

| Tool | mise key | Binary |
|------|----------|--------|
| nats-server | `ubi:nats-io/nats-server` | `nats-server` (on PATH) |
| gh CLI | `ubi:cli/cli` | `gh` (on PATH) |
| pitchfork | `ubi:jdx/pitchfork` | `pitchfork` (on PATH) |

### 5.2 go-upstream (build from source)

Clone upstream Go repos, build binary locally. Used for custom forks.

Examples: arc, liftbridge, telegraf, docs (hugo extended)

Required tasks: `src:clone`, `src:update`, `bin:build`, `ensure`, `run`, `health`, `test`, `package`, `clean:*`

### 5.3 go-inrepo (in-repo code)

Build Go binary from code in the repository.

Examples: sync-git, sync-gh, sync-cf, sync-cf-worker, service

Required tasks: `bin:build`, `ensure`, `run` (if service), `test`, `package`, `clean`

---

## 6. Workflows

### 6.1 DEV Workflow (build from source)
```bash
mise install                  # Install tools (go, pitchfork, gh, nats-server)
mise run src:clone            # Clone all upstream sources
mise run bin:build            # Build all binaries
pitchfork start --all         # Start all services (or: mise run start)

# After code changes:
mise run nats:bin:build       # Rebuild specific subsystem
mise run reload nats          # Hot-reload the service
```

### 6.2 USER Workflow (download binaries)
```bash
mise install                  # Install tools (includes nats-server via ubi)
pitchfork start --all         # Start all services
```

### 6.3 CI Workflow
```bash
mise run ci                   # Run full CI locally (MUST pass before push)
mise run ci:build             # Build all binaries
mise run ci:test              # Run all tests
mise run ci:package           # Package for release
mise run ci:pages             # Build docs
```

### 6.4 Pitchfork Management
```bash
pitchfork start --all         # Start all daemons
pitchfork list                # Show status of all daemons
pitchfork logs nats --tail    # Follow nats logs
pitchfork stop nats           # Stop specific daemon
pitchfork stop --all          # Stop all daemons
pitchfork tui                 # Open terminal UI
```

---

## 7. Language-Specific

### 7.1 Go

**Set GOWORK at task level:**
```toml
["subsystem:bin:build"]
env = { GOWORK = "off" }
run = "go build ..."
```

Or globally in `.mise.toml`:
```toml
[env]
GOWORK = "off"
```

**Never shell out to git binary.** Use `sync-git` commands with go-git:
```toml
# GOOD - uses sync-git binary with go-git
["liftbridge:src:clone"]
depends = ["sync-git:ensure"]
run = 'sync-git/.bin/sync-git clone https://github.com/... liftbridge/.src "${LB_VERSION}"'
```

---

## 8. Validation Checklist

Use this checklist to validate a subsystem:

```
[ ] mise-tasks.toml exists in subsystem directory
[ ] All task names use <subsystem>: prefix
[ ] ensure task checks binary existence and builds if missing
[ ] run task depends on ensure
[ ] run task is referenced in pitchfork.toml (if service)
[ ] health task matches pitchfork readiness check
[ ] Go tasks have env = { GOWORK = "off" }
[ ] Tasks use idempotency (shell checks or sources/outputs)
[ ] clean tasks remove .bin, .src, .data as appropriate
```

---

## 9. CI Matrix

CI runs on three platforms via GitHub Actions:

```yaml
strategy:
  matrix:
    os: [ubuntu-latest, macos-latest, windows-latest]
```

- **Linux/macOS**: Full build + test + package + service start/stop
- **Windows**: Build + test + package (service orchestration TBD)
- All platforms use `jdx/mise-action@v2` for tool installation
- All commands use `mise run <task>` for consistency
