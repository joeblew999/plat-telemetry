# CLAUDE

## Philosophy

**Taskfiles are the source of truth.** Everything runs through `task` - DEV, CI, OPS use identical commands.

**Idempotency everywhere.** Every task should be safe to run repeatedly:
- Use `status:` to skip if already done
- Use `sources:/generates:` for incremental builds
- Use `deps:` chains so tasks auto-satisfy dependencies

**One workflow for all users.** DEV builds from source, USER downloads binaries, but `task run` works for both via `ensure` task that auto-downloads if binary missing.

**Process Compose orchestrates, Task executes.** PC handles process lifecycle, health checks, dependencies. Task handles build/download/ensure logic.

**Run once, react to changes.** Start services with `task run` in one terminal. In another terminal, run `task build` (dev) or `task bin:download` (user) - binaries change, PC detects and restarts. All idempotent - run any command any time.

**Docs are a subsystem too.** Hugo docs live in `docs/`, run with `task docs:dev`. Same philosophy - edit markdown, Hugo hot-reloads. Treat docs like code.

## Conventions

Directories per subsystem:
- `.src/` - Source code
- `.bin/` - Binaries
- `.data/` - Runtime data

Standard tasks:
- `ensure` - Download binary if missing (idempotent)
- `build` - Build from source (deps on src:clone)
- `run` - Run service (deps on ensure)
- `bin:download` - Download pre-built binary

Use `GOWORK: off` for Go builds.
