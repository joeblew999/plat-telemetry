# TODO

---

faster CI !! 

   - uses: WillAbides/setup-go-faster@v1
    - uses: actions/setup-go@v6
      with:
        go-version: 'stable'


---

The binaries are unsigned. We need to solve this because our DEVS and USERS need this to compensate. 

On Linxu and Darwin there are standard ways to allow unsigned binaries to run. We need to look at this !!

---

## ✅ VERIFIED: USER Binary Update Workflow (PC stays running)

**The correct flow - tested and working:**

```bash
# Prerequisites: Process Compose already running
task start:fg
# (runs in background, PC maintains Unix socket at pc/.pc.sock)

# USER updates a single binary (e.g., new nats release available)
# In real scenario: task nats:bin:download RELEASE_VERSION=v0.1.3
# For testing: rm nats/.bin/nats-server && task nats:bin:build

# Hot-reload the updated service (PC stays running!)
task reload PROC=nats

# Result: NATS restarted with new binary (PID changed)
#         Dependent services (liftbridge, telegraf) automatically reconnect
#         Process Compose orchestrator never stopped
```

**Key insight:** Process Compose **does NOT** auto-detect binary changes. Users must explicitly run `task reload PROC=<name>` after updating a binary.

**Test results:**
- ✅ Binary updated: `nats-server` timestamp changed from 13:26 → 13:32
- ✅ Process restarted: PID changed from 77776 → 79112
- ✅ New binary loaded: confirmed by new process start time in logs
- ✅ Dependencies reconnected: liftbridge reconnected all 5 NATS connections
- ✅ PC orchestrator: never stopped, maintained socket control throughout

**What was WRONG in previous test:**
```bash
# ❌ WRONG: This restarts PC, defeating the whole purpose
task clean         # Deleted binaries
task bin:download  # Re-downloaded
task start:fg      # RESTARTED PC <-- wrong!
```

The point is that PC should be **always running** - only individual services get reloaded.



## ✅ COMPLETED: Automated Regression Testing for Binary Hot-Reload

**Comprehensive chaos testing system implemented - see [TODO_REGRESSION.md](TODO_REGRESSION.md)**

What's implemented:
- ✅ Version tracking (commit hash + timestamp + SHA256 checksum)
- ✅ Snapshot/verify system for before/after comparison
- ✅ Single service reload tests (nats, arc, liftbridge, telegraf)
- ✅ Multi-service reload tests (parallel, cascade, all-at-once)
- ✅ Rapid reload stress testing (10 consecutive reloads)
- ✅ Failure scenario testing (corrupt binary recovery)
- ✅ Full regression suite with detailed reporting
- ✅ CI integration (runs on every commit to main and every PR)
- ✅ Live feedback with phase control tasks

Run tests:
```bash
# Full suite (~5min)
task test:reload:all

# Individual phases
task test:reload:phase3  # Single service reloads (~1min)
task test:reload:phase4  # Multi-service reloads (~2min)
task test:reload:phase5  # Rapid reload stress test (~1min)
task test:reload:phase6  # Failure scenarios (~30s)
```

Test results prove:
- PID changes → process restarted
- Checksum changes → new binary loaded
- Timestamp changes → rebuild occurred
- Health checks → service stability maintained

**Everything works flawlessly. Binary hot-reload verified end-to-end.**

## Next

- Architecture docs page (currently placeholder)
- Add more platforms: linux/arm64, darwin/amd64
- Tag v0.1.2 with all fixes
- Phase 7: Data integrity tests (metrics injection during reload)

---

Any code thats not our code can be WAY smarter in terms of decideding then it needs to rebuild

---

Also, we can go further, Why cant we , from taskfile, poll for the third party repos changing ? the ci just has a cron to call the same taskfile.

Each taskfile can easiyl have this ans it will all line up into the manifest and sha etc ?
