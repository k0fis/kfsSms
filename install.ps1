#Requires -RunAsAdministrator
<#
.SYNOPSIS
  One-time installer for kfsSms on Windows.
.DESCRIPTION
  1. Creates C:\kfsSms
  2. Downloads Temurin JRE 21 (ZIP) from Adoptium API
  3. Downloads latest SmsApp JAR from GitHub Releases
  4. Creates config.yml template
  5. Copies kfsSms.bat wrapper
  6. Registers Task Scheduler task (run at startup as SYSTEM, highest privileges)
#>

param(
    [string]$InstallDir = "C:\kfsSms",
    [string]$GhOwner    = "k0fis",
    [string]$GhRepo     = "kfsSms"
)

$ErrorActionPreference = "Stop"

Write-Host "=== kfsSms Installer ===" -ForegroundColor Cyan

# 1. Create install directory
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
    Write-Host "Created $InstallDir"
} else {
    Write-Host "$InstallDir already exists"
}

# 2. Download Temurin JRE 21
$jreDir = Join-Path $InstallDir "jre"
if (-not (Test-Path $jreDir)) {
    Write-Host "Downloading Temurin JRE 21..."
    $adoptiumUrl = "https://api.adoptium.net/v3/binary/latest/21/ga/windows/x64/jre/hotspot/normal/eclipse?project=jdk"
    $jreZip = Join-Path $InstallDir "jre.zip"

    Invoke-WebRequest -Uri $adoptiumUrl -OutFile $jreZip -UseBasicParsing
    Write-Host "Extracting JRE..."
    Expand-Archive -Path $jreZip -DestinationPath $InstallDir -Force

    # Adoptium extracts to a versioned folder like jdk-21.0.x+y-jre
    $extracted = Get-ChildItem -Path $InstallDir -Directory | Where-Object { $_.Name -match "jdk-21.*-jre" } | Select-Object -First 1
    if ($extracted) {
        Rename-Item -Path $extracted.FullName -NewName "jre"
        Write-Host "JRE installed to $jreDir"
    } else {
        throw "Could not find extracted JRE directory"
    }

    Remove-Item $jreZip -Force
} else {
    Write-Host "JRE already present at $jreDir"
}

# 3. Download latest SmsApp JAR from GitHub Releases
Write-Host "Fetching latest release from GitHub..."
$releaseJson = Invoke-RestMethod -Uri "https://api.github.com/repos/$GhOwner/$GhRepo/releases/latest" -Headers @{ "User-Agent" = "kfsSms-installer" }
$jarAsset = $releaseJson.assets | Where-Object { $_.name -match "SmsApp-.*\.jar$" } | Select-Object -First 1

if (-not $jarAsset) {
    throw "No SmsApp JAR found in latest release"
}

$jarPath = Join-Path $InstallDir "SmsApp.jar"
Write-Host "Downloading $($jarAsset.name)..."
Invoke-WebRequest -Uri $jarAsset.browser_download_url -OutFile $jarPath -UseBasicParsing
Write-Host "JAR saved to $jarPath"

# 4. Create config.yml template
$configPath = Join-Path $InstallDir "config.yml"
if (-not (Test-Path $configPath)) {
    @"
sms:
  portName: "COM3"
  baudRate: 115200
  pollIntervalMs: 5000
  outgoingPollIntervalMs: 5000
  openModem: true
  sendMaxRetries: 3
  sendRetryDelayMs: 5000

api:
  baseUrl: "https://your-server:8081"
  user: "sms-user"
  password: "change-me"

cfg:
  terminate: ""

msisdn:
  pin: "1234"

logging:
  level: "INFO"
  packages:
    kfs.sc.sms: DEBUG
"@ | Set-Content -Path $configPath -Encoding UTF8
    Write-Host "Config template created at $configPath — EDIT BEFORE FIRST RUN!"
} else {
    Write-Host "config.yml already exists, skipping"
}

# 5. Copy kfsSms.bat
$batSource = Join-Path $PSScriptRoot "kfsSms.bat"
$batDest   = Join-Path $InstallDir "kfsSms.bat"
if (Test-Path $batSource) {
    Copy-Item -Path $batSource -Destination $batDest -Force
    Write-Host "Copied kfsSms.bat"
} else {
    Write-Host "WARNING: kfsSms.bat not found at $batSource — copy it manually"
}

# 6. Register Task Scheduler
$taskName = "kfsSms"
$existingTask = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue

if ($existingTask) {
    Write-Host "Task '$taskName' already registered, removing old..."
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
}

$action  = New-ScheduledTaskAction -Execute (Join-Path $InstallDir "kfsSms.bat") -WorkingDirectory $InstallDir
$trigger = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -RunLevel Highest -LogonType ServiceAccount
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)

Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Description "kfsSms — SMS gateway service"
Write-Host "Task Scheduler task '$taskName' registered (runs at startup as SYSTEM)"

Write-Host ""
Write-Host "=== Installation complete ===" -ForegroundColor Green
Write-Host "NEXT STEPS:"
Write-Host "  1. Edit $configPath (set COM port, server URL, credentials, PIN)"
Write-Host "  2. Test manually: cd $InstallDir && kfsSms.bat"
Write-Host "  3. Reboot to start automatically, or: Start-ScheduledTask -TaskName kfsSms"
