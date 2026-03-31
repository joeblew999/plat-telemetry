# TODO

## Open Items

### Faster CI
Switch to `WillAbides/setup-go-faster` instead of `actions/setup-go`:
```yaml
- uses: WillAbides/setup-go-faster@v1
```

### Binary Signing
Binaries are unsigned. DEVs and USERs need a way to run them without OS prompts.
On Linux and Darwin there are standard ways to allow unsigned binaries — needs investigation.

### Additional Platforms
- linux/arm64
- darwin/amd64

### Phase 7: Data Integrity Tests
Metrics injection during reload — verify zero data loss across reload boundary.
See [TODO_REGRESSION.md](TODO_REGRESSION.md) for details.

---

## ✅ Completed

- v0.1.2 tagged
- Binary hot-reload verified end-to-end (pitchfork stays running, individual services reload)
- Automated regression testing phases 1-6 (see [TODO_REGRESSION.md](TODO_REGRESSION.md))
- release:binary, release:all, ci:release tasks implemented
- service/ subsystem (kardianos/service, LaunchAgent/systemd)
- sync-gh, sync-git, sync-cf subsystems
