# TODO: xplat Integration

Integration plan for using xplat as the single binary for plat-telemetry.

## What is xplat?

xplat is a cross-platform Taskfile bootstrapper that embeds:
- **Task** (taskfile runner)
- **Process-Compose** (process orchestration)
- **Cross-platform utilities** (rm, cp, mv, glob, etc.)

Repository: https://github.com/joeblew999/xplat

## Why xplat?

Currently plat-telemetry requires users to install:
1. `task` - for running Taskfiles
2. `process-compose` - for service orchestration
3. Platform-specific shell commands

With xplat, users only need ONE binary that provides everything.

## Integration Plan

### Phase 1: Add xplat as subsystem

```yaml
# xplat/Taskfile.yml
vars:
  XPLAT_BIN_NAME: xplat
  XPLAT_UPSTREAM_REPO: https://github.com/joeblew999/xplat.git
  XPLAT_VERSION: '{{.XPLAT_VERSION | default "v0.2.0"}}'
  XPLAT_SRC: '{{.TASKFILE_DIR}}/.src'
  XPLAT_BIN: '{{.TASKFILE_DIR}}/.bin'
  XPLAT_BIN_PATH: '{{.XPLAT_BIN}}/{{.XPLAT_BIN_NAME}}'
```

**Tasks:**
- [ ] Create xplat/Taskfile.yml following canonical pattern
- [ ] Add to SUBSYSTEMS_BUILD and SUBSYSTEMS_RELEASE
- [ ] Add xplat releases to GitHub

### Phase 2: Replace task/pc calls

Currently:
```yaml
run:
  cmds:
    - task pc:run
```

With xplat:
```yaml
run:
  cmds:
    - xplat/.bin/xplat process up
```

**Benefits:**
- Single binary for all platforms
- Embedded cross-platform utilities
- No external dependencies

### Phase 3: Package management

xplat has a built-in package manager:
```bash
xplat pkg install nats        # Install from registry
xplat pkg install liftbridge  # Install from registry
```

This could replace our bin:download pattern for common packages.

### Phase 4: Bootstrap flow

Users would bootstrap with just:
```bash
# Download xplat for their platform
curl -L https://github.com/joeblew999/xplat/releases/download/v0.2.0/xplat-$(go env GOOS)-$(go env GOARCH).tar.gz | tar xz

# Run everything
./xplat task start
```

## xplat Commands Mapping

| Current Pattern | xplat Equivalent |
|-----------------|------------------|
| `task start` | `xplat dev up` or `xplat process` |
| `task stop` | `xplat dev down` or `xplat process down` |
| `task logs PROC=x` | `xplat dev logs x` |
| `task status` | `xplat dev status` |
| Process Compose TUI | `xplat process` (embedded) |

## Considerations

1. **xplat binary releases** - Currently v0.2.0 has no binary assets. Need to set up CI to build/release binaries.

2. **Task version compatibility** - xplat embeds Task. Ensure compatibility with our Taskfiles.

3. **Process Compose version** - xplat embeds Process Compose. Check feature parity.

4. **Gradual migration** - Can run both during transition:
   - Keep pc/Taskfile.yml working
   - Add xplat as optional alternative
   - Eventually make xplat the default

## Status

- [ ] Phase 1: Add xplat subsystem
- [ ] Phase 2: Replace task/pc calls
- [ ] Phase 3: Package management integration
- [ ] Phase 4: Bootstrap flow

## References

- [xplat v0.2.0 release](https://github.com/joeblew999/xplat/releases/tag/v0.2.0)
- [xplat README](https://github.com/joeblew999/xplat/blob/main/README.md)
