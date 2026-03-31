# Windows VM provisioner for plat-telemetry CI validation
# Executed by Vagrant when the Windows VM first boots
# Goal: install mise + git, clone the repo, run mise install

$ErrorActionPreference = "Stop"
$ProgressPreference    = "SilentlyContinue"  # speeds up Invoke-WebRequest

Write-Host "=== plat-telemetry Windows Provisioner ==="

# ── Helpers ───────────────────────────────────────────────────────────────────

function Refresh-Path {
    $env:PATH = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
                [System.Environment]::GetEnvironmentVariable("Path", "User")
}

# ── 1. Install mise ───────────────────────────────────────────────────────────

# ── 1a. Visual C++ 2019+ Redistributable (required by mise and Go) ───────────

if (-not (Test-Path "C:\Windows\System32\VCRUNTIME140.dll")) {
    Write-Host "Installing Visual C++ 2019+ Redistributable (ARM64)..."
    [System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
    $wc = New-Object System.Net.WebClient
    $wc.Headers.Add("User-Agent", "PowerShell")
    $vcUrl = "https://aka.ms/vs/17/release/vc_redist.arm64.exe"
    $vcDest = "$env:TEMP\vc_redist.arm64.exe"
    $wc.DownloadFile($vcUrl, $vcDest)
    Start-Process -FilePath $vcDest -ArgumentList "/quiet /norestart" -Wait
    Remove-Item $vcDest
    Write-Host "OK Visual C++ redistributable installed"
} else {
    Write-Host "OK Visual C++ redistributable already present"
}

$miseBin = "C:\ProgramData\mise\bin\mise.exe"
if (Get-Command mise -ErrorAction SilentlyContinue) {
    Write-Host "OK mise already installed: $(mise --version)"
} elseif (Test-Path $miseBin) {
    $env:PATH = "C:\ProgramData\mise\bin;$env:PATH"
    Write-Host "OK mise found at $miseBin"
} else {
    Write-Host "Installing mise from GitHub releases..."
    # Bypass cert validation for PS 5.1 (VM may have stale root certs)
    [System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
    $wc = New-Object System.Net.WebClient
    $wc.Headers.Add("User-Agent", "PowerShell")
    $json    = $wc.DownloadString("https://api.github.com/repos/jdx/mise/releases/latest")
    $release = $json | ConvertFrom-Json
    $asset   = $release.assets | Where-Object { $_.name -like "*windows-arm64.zip" } | Select-Object -First 1
    if (-not $asset) {
        $asset = $release.assets | Where-Object { $_.name -like "*windows-x64.zip" } | Select-Object -First 1
    }
    $zipPath    = "$env:TEMP\mise.zip"
    $extractDir = "$env:TEMP\mise-extract"
    $wc.DownloadFile($asset.browser_download_url, $zipPath)
    Remove-Item $extractDir -Recurse -Force -ErrorAction SilentlyContinue
    Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force
    Remove-Item $zipPath
    # Find mise.exe wherever it landed in the zip
    $miseExe = Get-ChildItem -Path $extractDir -Recurse -Filter "mise.exe" | Select-Object -First 1
    if (-not $miseExe) { throw "mise.exe not found in zip" }
    New-Item -ItemType Directory -Force -Path "C:\ProgramData\mise\bin" | Out-Null
    Copy-Item $miseExe.FullName "C:\ProgramData\mise\bin\mise.exe" -Force
    Remove-Item $extractDir -Recurse -Force
    $sysPath = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($sysPath -notlike "*ProgramData\mise\bin*") {
        [System.Environment]::SetEnvironmentVariable("Path", "C:\ProgramData\mise\bin;$sysPath", "Machine")
    }
    $env:PATH = "C:\ProgramData\mise\bin;$env:PATH"
    Write-Host "OK mise installed: $(& 'C:\ProgramData\mise\bin\mise.exe' --version)"
}

# ── 2. Install git ────────────────────────────────────────────────────────────

if (Get-Command git -ErrorAction SilentlyContinue) {
    Write-Host "OK git already installed: $(git --version)"
} else {
    Write-Host "Installing git via winget..."
    $result = winget install --id Git.Git -e --source winget `
        --accept-package-agreements --accept-source-agreements --silent 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "winget failed, falling back to direct download..."
        $gitUrl = "https://github.com/git-for-windows/git/releases/download/v2.53.0.windows.2/Git-2.53.0.2-arm64.exe"
        Invoke-WebRequest -Uri $gitUrl -OutFile "$env:TEMP\git-installer.exe"
        Start-Process -FilePath "$env:TEMP\git-installer.exe" `
            -ArgumentList "/VERYSILENT /NORESTART /COMPONENTS=icons,ext\reg\shellhere,assoc,assoc_sh" `
            -Wait
        Remove-Item "$env:TEMP\git-installer.exe"
    }
    Refresh-Path
    Write-Host "OK git installed: $(git --version)"
}

# ── 3. Clone / update repo ────────────────────────────────────────────────────

$repoDir = "C:\plat-telemetry"
$repoUrl = "https://github.com/joeblew999/plat-telemetry.git"

if (Test-Path "$repoDir\.git") {
    Write-Host "Updating repository..."
    Set-Location $repoDir
    git pull
} else {
    Write-Host "Cloning repository..."
    if (Test-Path $repoDir) { Remove-Item $repoDir -Recurse -Force }
    git clone $repoUrl $repoDir
    Set-Location $repoDir
}

Write-Host "OK repo at $repoDir"

# ── 3b. Add Git bash to system PATH (required for mise task shebangs) ─────────

$gitBash = "C:\Program Files\Git\bin"
if (Test-Path "$gitBash\bash.exe") {
    $sysPath = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($sysPath -notlike "*Git\bin*") {
        Write-Host "Adding Git bash to system PATH..."
        [System.Environment]::SetEnvironmentVariable("Path", "$gitBash;$sysPath", "Machine")
    }
    $env:PATH = "$gitBash;$env:PATH"
    Write-Host "OK bash: $(bash --version | Select-Object -First 1)"
} else {
    Write-Host "WARN git bash not found at $gitBash — mise tasks with bash shebangs will fail"
}

# ── 3c. Create mise wrapper in git-bash usr/bin so bash scripts can find mise ──
# mise tasks use #!/usr/bin/env bash shebangs; the spawned bash process does not
# inherit the Windows system PATH, so mise.exe must be accessible via a wrapper
# placed on git-bash's own PATH (C:\Program Files\Git\usr\bin is always on it).

$gitUsrBin = "C:\Program Files\Git\usr\bin"
if (Test-Path $gitUsrBin) {
    $miseWrapper = "#!/bin/sh`nexec /c/ProgramData/mise/bin/mise.exe `"`$@`"`n"
    [System.IO.File]::WriteAllText("$gitUsrBin\mise", $miseWrapper, [System.Text.Encoding]::ASCII)
    Write-Host "OK mise wrapper created at $gitUsrBin\mise"
} else {
    Write-Host "WARN $gitUsrBin not found — bash scripts may not find mise"
}

# ── 4. Install mise tools (go, nats-server, pitchfork, gh) ───────────────────

Set-Location $repoDir
Write-Host "Running mise trust + install..."
$env:PATH = "C:\ProgramData\mise\bin;C:\Program Files\Git\bin;$env:PATH"
& "C:\ProgramData\mise\bin\mise.exe" trust
& "C:\ProgramData\mise\bin\mise.exe" install

Write-Host ""
Write-Host "=== Provisioning complete ==="
Write-Host ""
Write-Host "To run CI tests: mise run utm:windows:ci"
Write-Host "Or from inside VM: cd C:\plat-telemetry && mise run ci"
