packer {
  required_plugins {
    utm = {
      version = ">=v0.0.2"
      source  = "github.com/naveenrajm7/utm"
    }
  }
}

# ── Variables ─────────────────────────────────────────────────────────────────

variable "iso_url" {
  type        = string
  description = "Path or URL to Windows 11 ARM64 ISO"
  # Download: https://www.microsoft.com/software-download/windowsinsiderpreviewiso
}

variable "iso_checksum" {
  type        = string
  description = "SHA256 checksum of the ISO (or 'none' to skip)"
  default     = "none"
}

variable "vm_name" {
  type    = string
  default = "packer-windows-11-arm64"
}

variable "memory" {
  type    = number
  default = 8192
}

variable "cpus" {
  type    = number
  default = 4
}

variable "disk_size" {
  type        = number
  default     = 65536 # 64 GB in MB
}

# ── Source ────────────────────────────────────────────────────────────────────

source "utm-iso" "windows" {
  vm_name  = var.vm_name
  iso_url  = var.iso_url
  iso_checksum = var.iso_checksum

  # Apple Virtualization backend for ARM64 (Apple Silicon)
  vm_backend = "apple"
  uefi_boot  = true

  memory    = var.memory
  cpus      = var.cpus
  disk_size = var.disk_size

  # autounattend.xml is packed into a CD and attached as drive D:
  # Windows Setup detects it automatically and runs unattended
  cd_files = ["${path.root}/autounattend.xml"]

  # WinRM communicator — enabled by autounattend.xml FirstLogonCommands
  communicator   = "winrm"
  winrm_username = "vagrant"
  winrm_password = "vagrant"
  winrm_timeout  = "4h"
  winrm_use_ssl  = false
  winrm_insecure = true
  winrm_port     = 5985

  boot_wait        = "60s"
  shutdown_command = "shutdown /s /t 10 /f /d p:4:1"
  shutdown_timeout = "30m"
}

# ── Build ─────────────────────────────────────────────────────────────────────

build {
  sources = ["source.utm-iso.windows"]

  # Install mise + git after Windows finishes initial setup
  provisioner "powershell" {
    pause_before = "2m"
    inline = [
      "winget install --id jdx.mise -e --accept-package-agreements --accept-source-agreements --silent",
      "$env:PATH = [System.Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [System.Environment]::GetEnvironmentVariable('Path','User')",
      "Write-Host \"mise: $(mise --version)\"",
    ]
  }

  provisioner "powershell" {
    inline = [
      "winget install --id Git.Git -e --source winget --accept-package-agreements --accept-source-agreements --silent",
      "$env:PATH = [System.Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [System.Environment]::GetEnvironmentVariable('Path','User')",
      "Write-Host \"git: $(git --version)\"",
    ]
  }

  # Package as Vagrant box using packer-plugin-utm's own post-processor
  post-processor "utm-vagrant" {
    compression_level = 9
    output            = "${path.root}/windows-11-arm64.box"
  }
}
