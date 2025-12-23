# TODO: Release-First CI Strategy

## The Problem

Currently CI builds everything from source every time. This is:
- Slow (10+ minutes)
- Wasteful (rebuilding unchanged binaries)
- Not aligned with the "binary release" philosophy

## The Vision

**Binaries are the artifact. Releases are the cache.**

1. Dev bumps version in Taskfile → pushes
2. CI calls `ensure` which tries `bin:download`
3. `bin:download` checks GitHub Releases for that version
4. **Cache hit**: Download pre-built binary → fast
5. **Cache miss**: Build from source → upload to release → slow (but only once per version)

This means:
- Same code works on laptop and CI
- First build after version bump is slow (builds + releases)
- All subsequent builds are fast (downloads from release)
- Dev can pre-release from laptop to make CI fast

## Current State

| Component | Status | Notes |
|-----------|--------|-------|
| `ensure` tasks | ✅ Have fallback | `task bin:download \|\| task bin:build` |
| `bin:download` tasks | ⚠️ Exist but fail | No matching release for current versions |
| `bin:build` tasks | ✅ Work | Build from source |
| `package` tasks | ✅ Work | Create tarballs |
| GitHub Releases | ⚠️ Exist | Have old versions only |
| CI workflow | ❌ Wrong | Runs `task ci` which always builds |

## Implementation Plan

### Phase 1: Version-Aware Downloads

Each subsystem needs version-aware `bin:download`:

```yaml
bin:download:
  desc: Download binary matching pinned version
  vars:
    VERSION: '{{.NATS_VERSION}}'
  cmds:
    - |
      # Try to download from our releases first
      if gh release download "{{.VERSION}}" \
           --repo {{.RELEASE_REPO}} \
           --pattern "{{.NATS_BIN_NAME}}-{{.GOOS}}-{{.GOARCH}}.tar.gz" \
           --output - 2>/dev/null | tar xz -C {{.NATS_BIN}}; then
        echo "Downloaded {{.NATS_BIN_NAME}} {{.VERSION}} from release"
      else
        # Fall back to upstream release if available
        curl -L "{{.UPSTREAM_RELEASE_URL}}" | tar xz -C {{.NATS_BIN}}
      fi
  status:
    - test -f {{.NATS_BIN_PATH}}
    - grep -q "version: {{.VERSION}}" {{.NATS_BIN}}/.version
```

Key: The `status:` check verifies BOTH that binary exists AND version matches.

### Phase 2: Auto-Release on Build

When CI builds from source (cache miss), it should release:

```yaml
bin:build:
  desc: Build binary and optionally release
  cmds:
    - go build -o {{.BIN_PATH}} .
    - |
      # Write version file
      echo "version: {{.VERSION}}" > {{.BIN}}/.version
      echo "commit: $(git rev-parse --short HEAD)" >> {{.BIN}}/.version
      echo "checksum: $(shasum -a 256 {{.BIN_PATH}} | awk '{print $1}')" >> {{.BIN}}/.version
    - |
      # If in CI and this is a new version, release it
      if [ -n "${GITHUB_ACTIONS:-}" ]; then
        task: release:binary
      fi
```

### Phase 3: Simplified CI Workflow

```yaml
# .github/workflows/ci.yml
jobs:
  build:
    steps:
      - run: task ensure:all  # Downloads if release exists, builds if not
      - run: task test

  # Only needed if ensure:all did any builds
  release:
    needs: build
    if: # new binaries were built
    steps:
      - run: task release:upload
```

### Phase 4: Version Manifest

Single source of truth for all versions:

```yaml
# versions.yml (or in root Taskfile.yml vars)
versions:
  nats: v2.10.24
  liftbridge: v1.10.0
  telegraf: v1.33.0
  arc: v0.1.0       # Our code - we control versions
  sync: v0.1.0      # Our code
  docs: v0.140.2    # Hugo version
  pc: v1.43.0       # Process Compose
```

When any version bumps:
1. CI detects cache miss (no release for that version)
2. Builds affected subsystem(s)
3. Creates release with new binaries
4. Future CI runs download from release

## Tasks To Complete

**Subsystem Taskfiles: NO CHANGES NEEDED**

The subsystem Taskfiles already have all the right tasks:
- ✅ `ensure` - tries download, falls back to build
- ✅ `bin:download` - downloads from release
- ✅ `bin:build` - builds from source, writes .version
- ✅ `package` - creates tarball
- ✅ `config:version` - outputs pinned version

**All changes are in ROOT Taskfile only. Minimal changes needed.**

### Changes to Root Taskfile.yml

**1. ADD `ensure:all` task:**
```yaml
ensure:all:
  desc: Ensure all binaries exist (parallel, download or build)
  deps:
    - arc:ensure
    - docs:ensure
    - gh:ensure
    - liftbridge:ensure
    - nats:ensure
    - pc:ensure
    - sync:ensure
    - telegraf:ensure
```

**2. CHANGE `ci:build` (one line change):**
```yaml
ci:build:
  desc: CI build step - ensure all binaries
  cmds:
    - task: ensure:all      # ← CHANGED from bin:build + sync:bin:build
    - task: bin:verify
```

**3. ADD `ci:release` task:**
```yaml
ci:release:
  desc: CI release step - upload binaries to GitHub releases
  cmds:
    - task: release:all
```

**4. ADD `release:binary` and `release:all` tasks:**
```yaml
release:binary:
  desc: "Release subsystem binary (usage: task release:binary SUBSYSTEM=nats)"
  vars:
    SUBSYSTEM: '{{.SUBSYSTEM}}'
  cmds:
    - task: '{{.SUBSYSTEM}}:package'
    - |
      VERSION=$(task {{.SUBSYSTEM}}:config:version)
      RELEASE_TAG="{{.SUBSYSTEM}}-${VERSION}"
      gh release view "$RELEASE_TAG" >/dev/null 2>&1 || \
        gh release create "$RELEASE_TAG" --title "{{.SUBSYSTEM}} ${VERSION}" --notes "Binary release"
      gh release upload "$RELEASE_TAG" .dist/*-{{OS}}-{{ARCH}}.tar.gz --clobber

release:all:
  desc: Release all subsystem binaries
  cmds:
    - for: [arc, liftbridge, nats, pc, sync, telegraf]
      task: release:binary
      vars:
        SUBSYSTEM: '{{.ITEM}}'
```

**5. UPDATE `ci:` to include ci:release:**
```yaml
ci:
  desc: Main CI entry point - runs build, test, package, release, pages
  cmds:
    - task: ci:build
    - task: ci:test
    - task: ci:package
    - task: ci:release    # ← ADD this line
    - task: ci:pages
```

### CI Workflow (.github/workflows/ci.yml)

**SIMPLIFY** - Remove GitHub Actions, just call task commands:

```yaml
name: CI

on:
  push:
    branches: [main]
    tags: ['v*']
  pull_request:

permissions:
  contents: write
  pages: write
  id-token: write

jobs:
  ci:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}

    environment:
      name: ${{ github.ref == 'refs/heads/main' && 'github-pages' || '' }}

    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install Task
        run: go install github.com/go-task/task/v3/cmd/task@latest

      # Same commands as local dev!
      - run: task ci:build      # ensure:all + verify
      - run: task ci:test       # start, test, stop
      - run: task ci:package    # create tarballs
      - run: task ci:release    # upload to GitHub releases (uses gh CLI)
      - run: task ci:pages      # build docs
        if: github.ref == 'refs/heads/main' && runner.os == 'Linux'
```

**Key insight**: `gh` CLI is pre-installed on GitHub runners, so `task ci:release` just works - it calls `gh release create` and `gh release upload` directly.

No need for:
- ❌ `actions/upload-artifact`
- ❌ `softprops/action-gh-release`
- ❌ Separate release job
- ❌ Artifact download/merge logic

**One workflow, same as local.** `task ci` does everything.

## The Full Cycle: Tags, Packages, and Releases

There are TWO types of releases:

### 1. Per-Subsystem Binary Releases (Cache)

These are the "cache" releases - one per subsystem version:

```
Release: nats-v2.10.24
Assets:
  - nats-server-darwin-arm64.tar.gz
  - nats-server-darwin-amd64.tar.gz
  - nats-server-linux-arm64.tar.gz
  - nats-server-linux-amd64.tar.gz

Release: arc-v0.1.0
Assets:
  - arc-darwin-arm64.tar.gz
  - arc-linux-amd64.tar.gz
  ...
```

**Trigger**: When `ensure` builds from source (cache miss), it packages and uploads.

### 2. Project Tagged Releases (Distribution)

These are full project releases with ALL binaries for a platform:

```
Release: v1.0.0 (project version)
Assets:
  - plat-telemetry-darwin-arm64.tar.gz  (contains all binaries)
  - plat-telemetry-darwin-amd64.tar.gz
  - plat-telemetry-linux-arm64.tar.gz
  - plat-telemetry-linux-amd64.tar.gz
```

**Trigger**: `git tag v1.0.0 && git push --tags`

### The Cycle

```
┌─────────────────────────────────────────────────────────────────┐
│                    DEV LOCAL CI (task ci)                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Dev runs: task ci                                              │
│                        │                                         │
│                        ▼                                         │
│  1. task ci:build                                               │
│     → ensure:all (download or build each subsystem)             │
│     → bin:verify                                                │
│                        │                                         │
│                        ▼                                         │
│  2. task ci:test                                                │
│     → Start services, run tests, stop                           │
│                        │                                         │
│                        ▼                                         │
│  3. task ci:package                                             │
│     → Package all binaries to .dist/                            │
│                        │                                         │
│                        ▼                                         │
│  4. task ci:release (OPTIONAL - dev can skip or run)            │
│     → Upload binaries to per-subsystem releases                 │
│     → Makes NEXT local run AND GitHub CI FAST                   │
│                        │                                         │
│                        ▼                                         │
│  5. task ci:pages                                               │
│     → Build docs                                                │
│                                                                  │
│  ✅ Dev pushes to main (confident CI will be fast)              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                     GITHUB CI WORKFLOW                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. CI runs: task ci:build                                      │
│     → ensure:all downloads from releases (dev pre-released!)    │
│     → FAST! No builds needed.                                   │
│                        │                                         │
│                        ▼                                         │
│  2. CI runs: task ci:test                                       │
│     → Tests pass                                                │
│                        │                                         │
│                        ▼                                         │
│  3. CI runs: task ci:pages (main branch only)                   │
│     → Deploys docs to GitHub Pages                              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    TAGGED RELEASE WORKFLOW                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Dev tags: git tag v1.0.0 && git push --tags                 │
│                        │                                         │
│                        ▼                                         │
│  2. CI detects tag push, runs release job:                      │
│     → task ensure:all (downloads all binaries - FAST)           │
│     → task package:bundle (creates platform bundle)             │
│     → gh release create v1.0.0 --generate-notes                 │
│     → Uploads plat-telemetry-{os}-{arch}.tar.gz                 │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Key Insight: Dev Releases Make Everything Fast

When dev runs `task ci` locally and releases binaries:

1. **Next local `task ci`** → Downloads instead of builds (fast)
2. **GitHub CI** → Downloads instead of builds (fast)
3. **Other devs** → Download instead of build (fast)
4. **Users** → Download pre-built binaries (fast)

**One build, many downloads.** The first person to build a version pays the cost, everyone else benefits.

### Key Tasks Needed

```yaml
# Release a single subsystem's binary (dev runs this after building new version)
release:binary:
  desc: Upload subsystem binary to its version release
  vars:
    SUBSYSTEM: '{{.SUBSYSTEM}}'
  cmds:
    - |
      VERSION=$(task {{.SUBSYSTEM}}:config:version)
      RELEASE_TAG="{{.SUBSYSTEM}}-${VERSION}"

      # Create release if it doesn't exist
      gh release view "$RELEASE_TAG" >/dev/null 2>&1 || \
        gh release create "$RELEASE_TAG" --title "{{.SUBSYSTEM}} ${VERSION}" --notes "Binary release for {{.SUBSYSTEM}} ${VERSION}"

      # Upload binary
      gh release upload "$RELEASE_TAG" .dist/{{.SUBSYSTEM}}-*.tar.gz --clobber

# Package all binaries into a platform bundle (for tagged releases)
package:bundle:
  desc: Create platform bundle with all binaries
  vars:
    GOOS: '{{.GOOS | default OS}}'
    GOARCH: '{{.GOARCH | default ARCH}}'
  cmds:
    - mkdir -p .dist/bundle
    - |
      # Collect all binaries into bundle
      for subsystem in arc liftbridge nats pc sync telegraf; do
        cp $subsystem/.bin/* .dist/bundle/
      done
    - tar -czvf .dist/plat-telemetry-{{.GOOS}}-{{.GOARCH}}.tar.gz -C .dist/bundle .
```

## Release Naming Convention

```
# For each version tag (e.g., v1.0.0), release contains:
arc-darwin-arm64.tar.gz
arc-darwin-amd64.tar.gz
arc-linux-arm64.tar.gz
arc-linux-amd64.tar.gz
nats-server-darwin-arm64.tar.gz
nats-server-darwin-amd64.tar.gz
...

# Each tarball contains:
<binary>
.version
```

## Testing the Strategy

```bash
# Simulate CI behavior locally
export GITHUB_ACTIONS=true

# Clean slate
task clean:all

# This should:
# 1. Try to download each binary from release
# 2. Fall back to building if no release
# 3. Report what was downloaded vs built
task ensure:all

# Verify
task bin:verify
```

## Success Criteria

1. **Fresh clone + `task ensure:all`** → Downloads all from releases (< 1 min)
2. **Version bump + `task ensure:all`** → Builds only changed subsystem
3. **CI after version bump** → Builds once, releases, subsequent runs download
4. **Dev pre-release** → `task release:upload` from laptop, CI downloads
