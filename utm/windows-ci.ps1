# Windows CI runner for plat-telemetry
# Called by: mise run utm:windows:ci  (via vagrant provision --provision-with ci)
# Must NOT be run privileged - mise.exe fails as SYSTEM

$ErrorActionPreference = "Stop"
$ProgressPreference    = "SilentlyContinue"

# Ensure git bash and mise are on PATH for this session
$env:PATH = "C:\Program Files\Git\bin;C:\ProgramData\mise\bin;" + $env:PATH

$repoDir = "C:\plat-telemetry"
$mise    = "C:\ProgramData\mise\bin\mise.exe"

if (-not (Test-Path $repoDir)) {
    Write-Error "FAIL repo not found at $repoDir - run 'VM=windows mise run utm:up' first"
    exit 1
}
if (-not (Test-Path $mise)) {
    Write-Error "FAIL mise not found at $mise - re-provision with 'VM=windows mise run utm:provision'"
    exit 1
}

Set-Location $repoDir

# Pull latest code so .mise.toml changes (new tools etc.) are reflected
# git writes remote fetch info to stderr; lower ErrorActionPreference so
# native command stderr does not throw under Stop mode
Write-Host "Pulling latest repo changes..."
$ErrorActionPreference = "Continue"
git reset --hard HEAD
git pull --quiet
$ErrorActionPreference = "Stop"
if ($LASTEXITCODE -ne 0) { Write-Error "git pull failed"; exit 1 }

Write-Host "=== Windows CI ==="
Write-Host "mise: $(& $mise --version)"
Write-Host "bash: $(bash --version 2>&1 | Select-Object -First 1)"
Write-Host "git:  $(git --version)"
Write-Host ""

& $mise trust
# Re-run install to pick up any new tools added to .mise.toml (e.g. ouch)
& $mise install

# Run CI - use doppler to inject secrets if DOPPLER_TOKEN is available
if ($env:DOPPLER_TOKEN) {
    Write-Host "Running 'mise run ci' via doppler (secrets injected)..."
    & doppler run -- $mise run ci
} else {
    Write-Host "WARN DOPPLER_TOKEN not set - running without secrets (ci:release will be skipped)"
    & $mise run ci
}
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
exit 0
