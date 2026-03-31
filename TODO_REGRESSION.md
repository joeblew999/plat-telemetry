# Regression Testing: Binary Hot-Reload

## Status

### ✅ Complete (Phases 1-6, 8)
- Version tracking: commit hash + timestamp + SHA256 in `.bin/.version`
- Snapshot/verify system (before/after comparison)
- Single service reload tests: `mise run test:reload:phase3`
- Multi-service reload tests: `mise run test:reload:phase4`
- Rapid reload stress test (10 consecutive): `mise run test:reload:phase5`
- Failure scenarios (corrupt binary recovery): `mise run test:reload:phase6`
- Full regression suite with reporting: `mise run test:reload:all`
- CI integration (runs on every commit to main and every PR)

### ⏳ Not Implemented

**Phase 7: Data Integrity Tests**

Verify zero data loss during reload by injecting metrics while telegraf/arc reload:

```bash
# Send metrics continuously in background
while true; do
  echo "cpu,host=test value=$RANDOM" | curl -XPOST localhost:8186/write --data-binary @-
  sleep 0.1
done &

# Reload service mid-write
mise run reload telegraf

# Verify all metrics arrived
```

Tasks to implement:
- `test:reload:data:telegraf` — metrics injection during telegraf reload
- `test:reload:data:arc` — query arc during reload

**Phase 6 gaps:**
- `test:reload:fail:socket` — PC socket unavailable mid-reload
- `test:reload:fail:deadlock` — reload dependency cycle detection

## Running Tests

```bash
# Full suite
mise run test:reload:all

# Individual phases
mise run test:reload:phase3   # Single service reloads (~1min)
mise run test:reload:phase4   # Multi-service reloads (~2min)
mise run test:reload:phase5   # Rapid reload stress (~1min)
mise run test:reload:phase6   # Failure scenarios (~30s)
```
