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
