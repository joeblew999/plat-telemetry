# CLAUDE

## Philosophy

**Taskfiles are the source of truth.** Everything runs through `task` - DEV, CI, OPS use identical commands.

**Idempotency everywhere.** Every task should be safe to run repeatedly:
- Use `status:` to skip if already done
- Use `sources:/generates:` for incremental builds
- Use `deps:` chains so tasks auto-satisfy dependencies

**One workflow for all users.** DEV builds from source, USER downloads binaries, but `task run` works for both via `ensure` task that auto-downloads if binary missing.

**Process Compose orchestrates, Task executes.** PC handles process lifecycle, health checks, dependencies. Task handles build/download/ensure logic.

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
