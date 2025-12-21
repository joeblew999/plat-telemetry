# CLAUDE

## Behavior Rules

STOP TELLING the User to do things. DOG FOOD your own shit! Stop being so lazy!

ALWAYS be RUNNING `task start` OR `task start:attach` so you can't CHEAT!

STOP touching the OS. Use project-level encapsulation, because hundreds of AIs are crawling over the OS changing things!

NEVER EVER JUST PUSH TO GITHUB CI and PRAY!

ONLY ever have a single GitHub workflow for CI.

## Architecture Principles

**Taskfiles are the source of truth.** Everything runs through `task` - DEV, CI, OPS use identical commands.

**Root-level configuration variables** (defined in root Taskfile.yml):
- `RELEASE_REPO` - GitHub repository for releases (e.g., `joeblew999/plat-telemetry`)
- `RELEASE_VERSION` - Release version tag (e.g., `latest`, `v1.0.0`)
- `DIST_DIR` - Directory for packaged release artifacts (defaults to `{{.ROOT_DIR}}/.dist`)

**process-compose.yaml delegates to Taskfiles.** Process Compose only orchestrates (process dependencies and restart policies). All implementation details live in subsystem Taskfiles:
- `command:` calls `task <subsystem>:run` (execute binary)
- `readiness_probe:` calls `task <subsystem>:health` (returns immediately)

**Idempotency everywhere.** Every task must be safe to run repeatedly:
- Use `status:` to skip if already done
- Use `sources:/generates:` for incremental builds
- Use `deps:` chains so tasks auto-satisfy dependencies

**One workflow for all users.** DEV builds from source, USER downloads binaries, but `task start` works for both via `ensure` task that auto-downloads if binary missing.

## Directory Structure

Per subsystem:
- `.src/` - Source code
- `.bin/` - Binaries
- `.data/` - Runtime data

## Task Naming

**Semantics - clear distinction between orchestration and execution:**
- `start` - Orchestrate all services (root level only, delegates to Process Compose)
- `run` - Execute a pre-built binary (subsystem level)
- `dev:run` - Rapid development with `go run` (Go subsystems only, compile+execute from source)
- `bin:build` - Compile source to binary (`go build`)
- `test`, `health` - Short-lived commands that return immediately

**Standard tasks per subsystem:**
- **src:** tasks - `src:clone`, `src:update`
- **bin:** tasks - `bin:build` (compile to binary), `bin:download` (download pre-built)
- **dev:** tasks - `dev:run` (Go subsystems: run from source with `go run`)
- **Service tasks** - `deps`, `ensure`, `health`, `install`, `package`, `run`, `test`
- **clean:** tasks - `clean`, `clean:all`, `clean:data`, `clean:src`

**Task ordering within subsystem Taskfiles:**
1. src: tasks (source management)
2. bin: tasks (binary artifacts)
3. dev: tasks (development mode)
4. Service tasks (alphabetically sorted)
5. clean: tasks (alphabetically sorted)

**Root-level aggregator tasks** (delegate to all subsystems):
- `src:clone` - Clone all subsystem sources (parallel via `deps:`)
- `src:update` - Update all subsystem sources (sequential via `cmds:`)
- `bin:build` - Build all subsystems from source (parallel via `deps:`)
- `bin:download` - Download all pre-built binaries (parallel via `deps:`)
- `package` - Package all binaries for release (sequential via `for:` loop with vars)
- `test` - Run tests for all subsystems (sequential)
- `deps` - Download dependencies for all subsystems (sequential)
- `clean`, `clean:data`, `clean:src`, `clean:all` - Clean tasks (sequential)

**CI-specific tasks (root Taskfile only):**
- `ci:dist` - Output the DIST_DIR path for CI to query (maintains "Taskfiles are the source of truth")

## Sorting Rules

**Alphabetically sort everything in root files:**
- Taskfile.yml: `includes:` section and all `deps:`/`cmds:` lists that call multiple subsystems
- process-compose.yaml: all process definitions and all `depends_on:` lists

This makes scanning and finding subsystems easy and keeps everything predictable.

## Go-Specific

Use `GOWORK: off` for Go builds.
