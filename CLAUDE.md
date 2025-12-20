# CLAUDE

## Directory Conventions

Each subsystem uses these standard directories:

- `.src/` - Source code (cloned from upstream or downloaded)
- `.bin/` - Compiled binaries or downloaded executables
- `.data/` - Runtime data (databases, logs, state)

All paths should use `{{.TASKFILE_DIR}}` prefix for absolute paths when called via includes.

## Taskfile Conventions

Each subsystem Taskfile should have these standard tasks:

- `src:clone` - Clone/download source
- `src:update` - Update source
- `build` - Build binary
- `run` - Run service
- `clean` - Clean `.bin/`
- `clean:data` - Clean `.data/`
- `clean:src` - Clean `.src/`
- `clean:all` - Clean everything

Use `GOWORK: off` for all Go builds.

## Process Compose

Keep the process compose file ordered alphabetically.

## CI Philosophy

**Taskfiles are the source of truth.** CI workflows should be thin wrappers that just call `task` commands.

- NO logic in CI workflows - put it in Taskfiles
- Test locally with same commands CI uses
- Use `:one` variants for fast iteration: `task src:clone:one SUBSYSTEM=nats`
- CI just runs: `task src:clone && task bin:build`

This ensures DEV, CI, and OPS all use identical commands.
