# Service

## Status: IMPLEMENTED

The plat-telemetry stack runs as a system service using `github.com/kardianos/service`.

## How it works

1. **Service binary** (`service/.bin/plat-telemetry-svc`) wraps `task start:fg`
2. Installs as **LaunchAgent** on macOS (user-level, runs when logged in)
3. Would install as **systemd user service** on Linux
4. launchd/systemd manages process lifecycle - no more orphan processes

## Usage

```bash
# Build the service binary
task service:bin:build

# Install as system service
task service:install

# Start the service
task service:start

# Check status
task service:status

# Stop the service
task service:stop

# Uninstall
task service:uninstall
```

## Why kardianos/service?

- Cross-platform (macOS, Linux, Windows)
- Handles launchd PATH issues (Homebrew not in PATH)
- Proper process group cleanup when service stops
- Idempotent commands (safe to run multiple times)

## Files

- `service/main.go` - Service wrapper using kardianos/service
- `service/Taskfile.yml` - Task wrappers for service management
- `~/Library/LaunchAgents/plat-telemetry.plist` - Generated plist (macOS)
