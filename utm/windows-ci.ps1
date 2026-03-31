# Windows CI runner for plat-telemetry
# Called by: mise run utm:windows:ci  (via vagrant provision --provision-with ci)
# Must NOT be run privileged - mise.exe fails as SYSTEM
#
# Note: $ErrorActionPreference is NOT set to Stop here.
# WinRM treats any native command stderr as failure under Stop mode.
# Instead we use explicit $LASTEXITCODE checks after critical commands.

$ProgressPreference = "SilentlyContinue"

# Ensure git bash and mise are on PATH for this session
$env:PATH = "C:\Program Files\Git\bin;C:\ProgramData\mise\bin;" + $env:PATH

$repoDir = "C:\plat-telemetry"
$mise    = "C:\ProgramData\mise\bin\mise.exe"

if (-not (Test-Path $repoDir)) {
    Write-Host "FAIL repo not found at $repoDir - run 'VM=windows mise run utm:up' first"
    exit 1
}
if (-not (Test-Path $mise)) {
    Write-Host "FAIL mise not found at $mise - re-provision with 'VM=windows mise run utm:provision'"
    exit 1
}

Set-Location $repoDir

# Pull latest code so .mise.toml changes (new tools etc.) are reflected
# --quiet suppresses remote fetch info that git writes to stderr
Write-Host "Pulling latest repo changes..."
git reset --hard HEAD
git pull --quiet
if ($LASTEXITCODE -ne 0) { Write-Host "FAIL git pull failed"; exit 1 }

Write-Host "=== Windows CI ==="
Write-Host "mise: $(& $mise --version)"
Write-Host "bash: $(bash --version 2>&1 | Select-Object -First 1)"
Write-Host "git:  $(git --version)"
Write-Host ""

& $mise trust 2>$null
& $mise install 2>$null

# Run CI - use doppler to inject secrets if DOPPLER_TOKEN is available
# MISE_LOG_LEVEL=error suppresses mise's task-tracing lines ([ci] $ ...)
# that go to stderr; WinRM treats any stderr as failure.
if ($env:DOPPLER_TOKEN) {
    Write-Host "Running 'mise run ci' via doppler (secrets injected)..."
    & doppler run -- $mise run ci 2>$null
} else {
    Write-Host "WARN DOPPLER_TOKEN not set - running without secrets (ci:release will be skipped)"
    & $mise run ci 2>$null
}
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
exit 0
