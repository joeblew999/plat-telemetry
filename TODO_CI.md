# TODO: Release-First CI

## Vision
Binaries are the artifact. Releases are the cache.
- CI calls `ensure` → tries `bin:download` from GitHub Releases
- Cache hit: download pre-built binary (fast)
- Cache miss: build from source → upload to release (slow, once per version)

## ✅ Done
- `release:binary` task — package + upload single subsystem binary
- `release:all` task — loop all subsystems
- `ci:release` task — called from CI to upload binaries
- Per-subsystem `ensure` tasks (try download, fall back to build)

## Remaining

### 1. Add `ensure:all` task
Parallel ensure across all subsystems:
```toml
["ensure:all"]
description = "Ensure all binaries exist (download or build)"
depends = ["arc:ensure", "nats:ensure", "sync-gh:ensure", "sync-git:ensure", "sync-cf:ensure", "service:ensure"]
```

### 2. Update `ci:build` to use `ensure:all`
```toml
["ci:build"]
depends = ["ensure:all", "bin:verify"]
```
Currently builds from source every time. Should download if release exists.

### 3. CI workflow
Update `.github/workflows/ci.yml` — remove redundant build steps, rely on `mise run ci:build` (which uses ensure:all).

## Release Naming
```
# Per-subsystem binary releases (cache):
nats-server-darwin-arm64.tar.gz
arc-darwin-arm64.tar.gz
...

# Project tagged releases (distribution):
plat-telemetry-darwin-arm64.tar.gz  (all binaries bundled)
```

## Success Criteria
1. `mise run ensure:all` on fresh clone → downloads all from releases (< 1 min)
2. Version bump → builds only changed subsystem
3. CI after dev pre-release → downloads, no builds
