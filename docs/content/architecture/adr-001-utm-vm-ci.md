---
title: "ADR-001: UTM VM CI Strategy"
date: 2026-03-31
status: Accepted
---

## Status

Accepted — in progress (Windows provisioned, Linux VMs not yet booted)

## Current State (2026-03-31)

| VM | State | Provisioned | GPU | CI run |
|----|-------|------------|-----|--------|
| windows | stopped | YES — mise + git + telegraf built | virtio-gpu-gl-pci | NOT YET |
| alpine | never created | NO | n/a | NOT YET |
| ubuntu | never created | NO | n/a | NOT YET |
| debian | never created | NO | n/a | NOT YET |

**What is working:**
- `utm:deps` / `utm:deps:delete` — install/uninstall UTM + Vagrant + plugin
- `utm:up` / `utm:down` — start/stop VMs
- `utm:wait` — poll until all builds finish inside VM (critical for Windows where telegraf takes ~9 min)
- `utm:diag` — show all 5 sources of truth side-by-side
- `utm:state:restore` — auto-heal vagrant id file before every lifecycle task
- `utm:clean:registry` — nuclear reconcile when state is badly broken
- `utm:copy:to` / `utm:copy:from` — copy files host↔VM
- `utm:windows:ci` — task exists, not yet run end-to-end
- `utm:linux:ci` — task exists, not yet run (no Linux VM ever booted)
- Windows provision: all bugs fixed (see decisions 3, 4, 6 below)

**What is NOT yet validated:**
- `VM=windows mise run utm:windows:ci` — full `mise run ci` inside Windows
- Any Linux VM boot + provision + CI run
- `utm:copy:to` / `utm:copy:from` — written but not tested

**Next steps:**
1. `VM=windows mise run utm:windows:ci` — validate full Windows CI
2. `VM=alpine mise run utm:up` — first Alpine boot (downloads box, runs linux-provision.sh)
3. `VM=alpine mise run utm:linux:ci` — validate Linux CI path

## Context

`plat-telemetry` must validate that `mise run ci` works correctly on Linux, macOS, and Windows. GitHub Actions covers Linux and macOS cheaply. Windows on ARM64 (Apple Silicon) is the problem: GitHub-hosted Windows runners are x86-64 only and cannot run the ARM64 binaries this project produces. A local VM solution is required.

Additionally, Linux VMs provide a faster, cheaper alternative to the Windows VM for rapid iteration — Alpine boots in seconds, has no licensing constraints, and exercises the same CI path.

## Decision

Use **UTM** (QEMU-based hypervisor for Apple Silicon) with **Vagrant** and the **vagrant_utm** provider to manage local VMs for CI validation. All VM lifecycle is driven by `mise run utm:<task>` so the interface is identical to every other workflow in the project.

## Architecture

### The 5 Sources of Truth

UTM VM state is spread across 5 independent stores that must stay consistent:

| # | Source | What it contains | How to query |
|---|--------|-----------------|--------------|
| 1 | `utmctl list` | Running UTM process view: UUID, status, name | `utmctl list` |
| 2 | UTM plist registry | Persistent registry in `com.utmapp.UTM` prefs | `defaults read com.utmapp.UTM Registry` |
| 3 | `.utm` bundles on disk | Actual VM files in `~/Library/.../Documents/` | `ls *.utm` |
| 4 | Vagrant local state | `.vagrant/machines/<VM>/utm/id` — UUID file | `cat .vagrant/machines/*/utm/id` |
| 5 | Vagrant global index | `~/.vagrant.d/data/machine-index/index` — JSON | `jq` |

When these drift apart (e.g. after a force-kill, partial clean, or UTM crash), vagrant commands fail with "VM not found". `mise run utm:diag` shows all 5 side-by-side. `utm:state:restore` auto-heals source #4 from source #1 before every lifecycle task.

### VM Inventory

| VM | Box | Communicator | Port | Memory |
|----|-----|-------------|------|--------|
| alpine | `utm/alpine-ce` | SSH | 2223 | 2 GB |
| ubuntu | `utm/ubuntu-24.04` | SSH | 2224 | 4 GB |
| debian | `utm/bookworm` | SSH | 2225 | 4 GB |
| windows | `utm/windows-11` | WinRM | 55985 | 8 GB |

### Task Hierarchy

```
utm:deps          — install UTM + Vagrant + vagrant_utm plugin
utm:deps:delete   — exact inverse of utm:deps
utm:up            — boot VM (VM=name); creates from box if new
utm:wait          — poll until all builds/installs finish inside VM
utm:down          — stop VM cleanly
utm:destroy       — delete VM and all state
utm:diag          — show all 5 sources of truth side-by-side
utm:state:restore — rebuild vagrant id file from utmctl if missing
utm:clean:registry — reconcile registry + clear all vagrant state
utm:linux:ci      — provision-with ci on a Linux VM
utm:windows:ci    — provision-with ci on Windows VM
utm:copy:to       — copy file from host into VM
utm:copy:from     — copy file from VM to host
utm:wait          — wait until VM is idle (all builds/installs done)
```

## Key Technical Decisions

### 1. GPU acceleration: `virtio-gpu-gl-pci` not `virtio-ramfb`

**Decision:** Windows VM uses `virtio-gpu-gl-pci` (host Metal/GPU).

**Why:** `virtio-ramfb` is a software framebuffer. With software rendering, QEMU can serialise the entire VM state to disk (suspend-to-disk). This creates hidden state in the qcow2 image that survives reboots and corrupts the "fresh start" assumption of CI. `virtio-gpu-gl-pci` uses the host GPU; GPU state is not serialisable, so QEMU cannot save VM state. This is the desired behaviour.

**Implementation:** Set in `config.plist` via a Vagrantfile `trigger.after :up` block that runs PlistBuddy on the host after the VM is first created from the box.

### 2. Registry parsing: `defaults read` vs PlistBuddy

**Decision:** Use `defaults read com.utmapp.UTM Registry` for diagnostics (UTM running); use `PlistBuddy + strings` for `utm:clean:registry` (UTM stopped).

**Why:** The UTM plist contains NSData Bookmark fields — binary blobs that contain UUID-like byte sequences. Grepping raw PlistBuddy output produces false UUID matches. `defaults read` goes through `cfprefsd` and returns plain text, so UUID matching is clean. When UTM is stopped, `cfprefsd` may not have flushed, so `defaults read` returns stale data; instead use `PlistBuddy | strings` and match the pattern `UUID = Dict` which only matches actual registry dict keys.

### 3. No hostname on Windows VM

**Decision:** Removed `vm.vm.hostname = "windows"` from Vagrantfile.

**Why:** Setting a hostname on Windows causes an immediate reboot mid-provision. After the reboot, WinRM takes 60–90 seconds to come back. The vagrant WinRM communicator timed out, which triggered vagrant's warden cleanup — destroying the newly-created VM. The hostname is not needed for CI (we connect via `127.0.0.1:55985`).

### 4. WinRM stderr is fatal under `ErrorActionPreference = Stop`

**Decision:** Native exe stderr is handled with `try/catch` (for warnings) and `ErrorActionPreference = Continue` + `$LASTEXITCODE` check (for commands where output matters).

**Why:** Vagrant's WinRM communicator runs scripts via `powershell.exe -OutputFormat Text -file script.ps1`. Under `ErrorActionPreference = Stop`, any native executable that writes to stderr generates a terminating `NativeCommandError`, causing the script to exit non-zero. Vagrant then reports provision failure. Affected commands:
- `git clone` — writes "Cloning into..." to stderr → fixed with `--quiet`
- `mise trust` — writes "No untrusted config files found" to stderr → fixed with `try/catch`
- `mise install` — warnings on stderr → fixed with `ErrorActionPreference = Continue`

`2>$null` does **not** fix this: it suppresses the text output but the error record is still generated and propagated to the outer `powershell.exe` process that WinRM is watching.

### 5. `utm:wait` before shutdown

**Decision:** `utm:wait` polls `go`, `msiexec`, `winget` (Windows) or `go`, `apt`, `apk` (Linux) until the VM is idle before allowing a clean shutdown.

**Why:** `mise install` installs tools and returns, but background processes it spawned (e.g. `go build` for telegraf, which takes ~9 minutes on Windows ARM64) keep running. Shutting down while `go build` is running corrupts the binary being compiled. `utm:wait` makes the completion state observable via a single mise task.

### 6. Vagrant file locking on Windows

**Decision:** Windows Defender must be configured to exclude `C:\tmp` before re-provisioning.

**Why:** Vagrant uploads provision scripts to `C:\tmp\vagrant-shell.ps1`. Windows Defender scans newly-created `.ps1` files, locking them for several seconds. The winrm-fs file transporter immediately hashes the file after upload; if Defender holds the lock, the hash fails with `IOException: file is being used by another process` and provision fails. The provision script adds `C:\tmp` to Defender exclusions. If a partial provision leaves a stale `vagrant-shell.ps1` locked by a still-running process, kill the process on the guest before retrying.

### 7. Linux provisioner in POSIX `sh`

**Decision:** `linux-provision.sh` uses `#!/usr/bin/env sh`, not `bash`.

**Why:** Alpine Linux does not ship bash by default. Using POSIX `sh` ensures the provisioner works identically on Alpine, Debian, and Ubuntu without an extra install step.

## Consequences

**Positive:**
- Full Windows ARM64 CI validation locally before push
- Linux VMs (especially Alpine) provide fast iteration for CI debugging
- All VM operations are `mise run utm:<task>` — no ad-hoc vagrant/utmctl commands needed
- `utm:diag` makes state problems immediately visible
- `utm:state:restore` self-heals the most common failure mode automatically

**Negative:**
- UTM 4.7.5 has a known SIGSEGV crash in SwiftUI on macOS 26 beta (HStack layout bug). This is an upstream bug; workaround is graceful `osascript quit` + restart in `utm:up` when utmctl returns XPC error -609.
- Windows VM first provision takes ~15 minutes (telegraf source build on ARM64).
- Vagrant's WinRM communicator is fragile: any stderr from native exes is treated as an error, and any long-running provisioner step that causes a reboot will break the WinRM connection.
- The vagrant_utm provider (`vagrant_utm-0.1.5`) has limited error recovery compared to the VMware or VirtualBox providers.
