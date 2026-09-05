#Requires -Version 5.1
<#
.SYNOPSIS
    sota-headless installer & service manager for Windows.
.DESCRIPTION
    Installs sota-headless as a native Windows service, configures firewall,
    and sets up subscriptions for Mihomo, Clash, sing-box, v2ray, etc.
.PARAMETER AccessKey
    Sota Connect access key (UUID).
.PARAMETER Version
    Version tag to install (default: "latest").
.PARAMETER InstallDir
    Target directory (default: "$env:ProgramFiles\sota-headless").
.PARAMETER Listen
    Listen address (default: "0.0.0.0:16698").
.PARAMETER Lang
    Language: 'ru' for Russian, 'en' for English (prompts if omitted).
.PARAMETER Uninstall
    Stops and removes the service, firewall rule, and files.
.EXAMPLE
    irm https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.ps1 | iex
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\install.ps1
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\install.ps1 -Uninstall
#>

param(
    [string]$AccessKey = $(if ($env:SOTA_ACCESS_KEY) { $env:SOTA_ACCESS_KEY } else { "" }),
    [string]$Version = $(if ($env:SOTA_VERSION) { $env:SOTA_VERSION } else { "latest" }),
    [string]$InstallDir = "$env:ProgramFiles\sota-headless",
    [string]$Listen = "0.0.0.0:16698",
    [string]$Lang = "",
    [switch]$Upx = $(if ($env:SOTA_UPX -eq "1" -or $env:SOTA_UPX -eq "true") { $true } else { $false }),
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$REPO = "paintingpromisesss/sota_headless"
$SERVICE_NAME = "sota-headless"
$FIREWALL_RULE_NAME = "sota-headless-http"
$PORT = 16698

# ── Formatting helpers ────────────────────────────────────────────────────────
function Write-Sep {
    Write-Host "────────────────────────────────────────────" -ForegroundColor DarkGray
}

function Write-Hdr([string]$text) {
    Write-Host ""
    Write-Host "  $text" -ForegroundColor Yellow
    Write-Sep
}

function Write-Info([string]$text) {
    Write-Host "  • $text" -ForegroundColor Gray
}

function Write-Ok([string]$text) {
    Write-Host "  ✓ $text" -ForegroundColor Green
}

function Write-WarnMsg([string]$text) {
    Write-Host "  ! $text" -ForegroundColor Yellow
}

function Write-ErrMsg([string]$text) {
    Write-Host "  ✗ $text" -ForegroundColor Red
    exit 1
}

function Ask-Input([string]$prompt) {
    Write-Host "  → $prompt " -ForegroundColor Yellow -NoNewline
    return Read-Host
}

# ── Check Administrator privileges ───────────────────────────────────────────
function Assert-Administrator {
    $currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    $isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $isAdmin) {
        if ($LangUI -eq "ru") {
            Write-ErrMsg "Этот скрипт должен быть запущен от имени Администратора.`n    Откройте PowerShell от имени Администратора (ПКМ → 'Запуск от имени администратора') и повторите."
        } else {
            Write-ErrMsg "This script must be run as Administrator.`n    Please run PowerShell as Administrator (Right click → 'Run as Administrator') and try again."
        }
    }
}

# ── Language selection ────────────────────────────────────────────────────────
function Select-Language {
    if ($Lang -in @("ru", "en")) {
        return $Lang
    }

    Write-Host ""
    Write-Host "════════════════════════════════════════════" -ForegroundColor Yellow
    Write-Host "    sota-headless  ·  Windows installer    " -ForegroundColor Yellow
    Write-Host "════════════════════════════════════════════" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "  [1] English (default)" -ForegroundColor Gray
    Write-Host "  [2] Русский" -ForegroundColor Gray
    Write-Host ""

    $choice = Ask-Input "Select language / Выберите язык [1/2]:"
    switch ($choice.Trim()) {
        "2"  { return "ru" }
        "ru" { return "ru" }
        "RU" { return "ru" }
        default { return "en" }
    }
}

# ── Detect architecture ───────────────────────────────────────────────────────
function Get-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    if ($env:PROCESSOR_ARCHITEW6432) {
        $arch = $env:PROCESSOR_ARCHITEW6432
    }
    switch -Regex ($arch) {
        "ARM64" { return "windows-arm64.exe" }
        default { return "windows-amd64.exe" }
    }
}

# ── Uninstall routine ─────────────────────────────────────────────────────────
function Invoke-UninstallRoutine {
    Write-Hdr $(if ($LangUI -eq "ru") { "Удаление sota-headless" } else { "Uninstalling sota-headless" })

    # Stop service
    $svc = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($svc) {
        Write-Info $(if ($LangUI -eq "ru") { "Остановка службы $SERVICE_NAME..." } else { "Stopping service $SERVICE_NAME..." })
        if ($svc.Status -eq "Running") {
            Stop-Service -Name $SERVICE_NAME -Force -ErrorAction SilentlyContinue
        }
        Write-Info $(if ($LangUI -eq "ru") { "Удаление службы $SERVICE_NAME..." } else { "Removing service $SERVICE_NAME..." })
        & sc.exe delete $SERVICE_NAME | Out-Null
        Write-Ok $(if ($LangUI -eq "ru") { "Служба удалена" } else { "Service removed" })
    } else {
        Write-Info $(if ($LangUI -eq "ru") { "Служба $SERVICE_NAME не найдена" } else { "Service $SERVICE_NAME not found" })
    }

    # Remove firewall rule
    $fw = Get-NetFirewallRule -Name $FIREWALL_RULE_NAME -ErrorAction SilentlyContinue
    if ($fw) {
        Write-Info $(if ($LangUI -eq "ru") { "Удаление правила брандмауэра..." } else { "Removing firewall rule..." })
        Remove-NetFirewallRule -Name $FIREWALL_RULE_NAME -ErrorAction SilentlyContinue
        Write-Ok $(if ($LangUI -eq "ru") { "Правило брандмауэра удалено" } else { "Firewall rule removed" })
    }

    # Clean files
    if (Test-Path $InstallDir) {
        $promptMsg = if ($LangUI -eq "ru") {
            "Удалить каталог установки и конфигурацию ($InstallDir)? [Y/n]:"
        } else {
            "Remove installation directory and config ($InstallDir)? [Y/n]:"
        }
        $remove = Ask-Input $promptMsg
        if ($remove -eq "" -or $remove -match "^[yYдД]") {
            Remove-Item -Path $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
            Write-Ok $(if ($LangUI -eq "ru") { "Файлы удалены" } else { "Files removed" })
        } else {
            Write-Info $(if ($LangUI -eq "ru") { "Файлы сохранены в $InstallDir" } else { "Files preserved in $InstallDir" })
        }
    }

    Write-Host ""
    Write-Host "════════════════════════════════════════════" -ForegroundColor Yellow
    Write-Host $(if ($LangUI -eq "ru") { "    Удаление завершено!    " } else { "    Uninstall completed!    " }) -ForegroundColor Yellow
    Write-Host "════════════════════════════════════════════" -ForegroundColor Yellow
    Write-Host ""
    exit 0
}

# ── Main installation logic ───────────────────────────────────────────────────
$global:LangUI = Select-Language
Assert-Administrator

if ($Uninstall) {
    Invoke-UninstallRoutine
}

if ($env:SOTA_VERSION -and $Version -eq "latest") {
    $Version = $env:SOTA_VERSION
}

$targetBinary = Get-Architecture
if ($Upx -or $env:SOTA_UPX -eq "1" -or $env:SOTA_UPX -eq "true") {
    $targetBinary = $targetBinary -replace '\.exe$', '-upx.exe'
}
$isUpgrade = $false
$configFile = Join-Path $InstallDir "sota-headless.env"
$stateDir = Join-Path $InstallDir "state"
$deviceJson = Join-Path $stateDir "device.json"
$binDst = Join-Path $InstallDir "sota-headless.exe"

Write-Info $(if ($LangUI -eq "ru") {
    "Архитектура: $targetBinary (версия: $Version)"
} else {
    "Architecture: $targetBinary (version: $Version)"
})

# ── Check existing installation ───────────────────────────────────────────────
$existingSvc = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
if ($existingSvc) {
    $isUpgrade = $true
    Write-WarnMsg $(if ($LangUI -eq "ru") {
        "Служба $SERVICE_NAME уже установлена — выполняется обновление"
    } else {
        "Service $SERVICE_NAME is already installed — performing upgrade"
    })
}

$oldHwid = ""
$oldDeviceName = ""
$oldKey = ""
$oldListen = $Listen
$oldTtl = "30m"
$oldLogLevel = "info"

if (Test-Path $configFile) {
    Get-Content $configFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -match "^SOTA_ACCESS_KEY=(.+)$") { $oldKey = $Matches[1].Trim() }
        if ($line -match "^SOTA_HWID=(.+)$") { $oldHwid = $Matches[1].Trim() }
        if ($line -match "^SOTA_DEVICE_NAME=(.+)$") { $oldDeviceName = $Matches[1].Trim() }
        if ($line -match "^SOTA_LISTEN=(.+)$") { $oldListen = $Matches[1].Trim() }
        if ($line -match "^SOTA_CACHE_TTL=(.+)$") { $oldTtl = $Matches[1].Trim() }
        if ($line -match "^SOTA_LOG_LEVEL=(.+)$") { $oldLogLevel = $Matches[1].Trim() }
    }
}

if (Test-Path $deviceJson) {
    Write-Info $(if ($LangUI -eq "ru") {
        "Найден существующий device.json — идентификатор устройства сохранён"
    } else {
        "Found existing device.json — will preserve device identity"
    })
}

# ── Access Key Dialog ─────────────────────────────────────────────────────────
Write-Hdr $(if ($LangUI -eq "ru") { "Ключ доступа Sota" } else { "Sota access key" })

if (-not $AccessKey) {
    if ($oldKey) {
        Write-Info $(if ($LangUI -eq "ru") { "Текущий ключ: $oldKey" } else { "Current key: $oldKey" })
        Write-Host $(if ($LangUI -eq "ru") {
            "  Оставьте пустым, чтобы сохранить текущий ключ, или введите новый."
        } else {
            "  Leave blank to keep it, or enter a new key to replace."
        }) -ForegroundColor DarkGray

        $inputKey = Ask-Input $(if ($LangUI -eq "ru") { "Новый ключ (Enter чтобы оставить):" } else { "New access key (Enter to keep):" })
        if ([string]::IsNullOrWhiteSpace($inputKey)) {
            $AccessKey = $oldKey
            Write-Info $(if ($LangUI -eq "ru") { "Сохранён текущий ключ доступа" } else { "Keeping existing access key" })
        } else {
            $AccessKey = $inputKey.Trim()
            Write-Ok $(if ($LangUI -eq "ru") { "Ключ доступа обновлён" } else { "Access key updated" })
        }
    } else {
        Write-Host $(if ($LangUI -eq "ru") {
            "  Формат UUID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
        } else {
            "  UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
        }) -ForegroundColor DarkGray

        $inputKey = Ask-Input $(if ($LangUI -eq "ru") { "Ключ доступа:" } else { "Access key:" })
        if ([string]::IsNullOrWhiteSpace($inputKey)) {
            Write-ErrMsg $(if ($LangUI -eq "ru") { "Ключ доступа не может быть пустым" } else { "Access key cannot be empty" })
        }
        $AccessKey = $inputKey.Trim()
        Write-Ok $(if ($LangUI -eq "ru") { "Ключ доступа установлен" } else { "Access key set" })
    }
}

if ($AccessKey -notmatch "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$") {
    Write-WarnMsg $(if ($LangUI -eq "ru") {
        "Формат ключа выглядит нестандартно — продолжаем"
    } else {
        "Key format looks unusual — continuing anyway"
    })
}

# ── Stop existing service if upgrading ───────────────────────────────────────
if ($isUpgrade -and $existingSvc.Status -eq "Running") {
    Write-Info $(if ($LangUI -eq "ru") { "Остановка существующей службы..." } else { "Stopping existing service..." })
    Stop-Service -Name $SERVICE_NAME -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
}

# ── Create installation directories ──────────────────────────────────────────
if (-not (Test-Path $InstallDir)) {
    New-Item -Path $InstallDir -ItemType Directory -Force | Out-Null
}
if (-not (Test-Path $stateDir)) {
    New-Item -Path $stateDir -ItemType Directory -Force | Out-Null
}

# ── Download or copy binary ──────────────────────────────────────────────────
Write-Hdr $(if ($LangUI -eq "ru") { "Загрузка файлов" } else { "Downloading" })

# Check for local binary in build directory or script parent directory
$localBin = ""
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
if ($scriptDir) {
    $candidates = @(
        (Join-Path $scriptDir "..\sota-headless.exe"),
        (Join-Path $scriptDir "..\sota-headless-$targetBinary"),
        (Join-Path $scriptDir "sota-headless.exe")
    )
    foreach ($c in $candidates) {
        if (Test-Path $c) {
            $localBin = (Resolve-Path $c).Path
            break
        }
    }
}

if ($localBin -and -not ($Version -ne "latest" -and $Version -notmatch "^v?local")) {
    Write-Info $(if ($LangUI -eq "ru") {
        "Используется локальный бинарный файл: $localBin"
    } else {
        "Using local binary: $localBin"
    })
    Copy-Item -Path $localBin -Destination $binDst -Force
} else {
    $downloadUrl = if ($Version -eq "latest") {
        "https://github.com/${REPO}/releases/latest/download/sota-headless-$targetBinary"
    } else {
        $tag = if ($Version.StartsWith("v")) { $Version } else { "v$Version" }
        "https://github.com/${REPO}/releases/download/$tag/sota-headless-$targetBinary"
    }

    Write-Info $(if ($LangUI -eq "ru") { "Загрузка sota-headless-$targetBinary ..." } else { "Fetching sota-headless-$targetBinary ..." })
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13
    $tempFile = Join-Path $env:TEMP "sota-headless-temp.exe"
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempFile -UseBasicParsing
        Move-Item -Path $tempFile -Destination $binDst -Force
    } catch {
        Write-ErrMsg $(if ($LangUI -eq "ru") { "Ошибка загрузки: $downloadUrl`n$($_)" } else { "Download failed: $downloadUrl`n$($_)" })
    }
}

Write-Ok $(if ($LangUI -eq "ru") { "Бинарный файл → $binDst" } else { "Binary → $binDst" })

# ── Write configuration ──────────────────────────────────────────────────────
$configLines = @(
    "# sota-headless configuration",
    "# Updated: $(Get-Date -Format 'yyyy-MM-ddTHH:mm:ssK')",
    "",
    "SOTA_ACCESS_KEY=$AccessKey",
    "SOTA_BASE_DIR=$InstallDir",
    "SOTA_LISTEN=$oldListen",
    "SOTA_CACHE_TTL=$oldTtl",
    "SOTA_LOG_LEVEL=$oldLogLevel"
)

if ($oldHwid) {
    $configLines += "SOTA_HWID=$oldHwid"
}
if ($oldDeviceName) {
    $configLines += "SOTA_DEVICE_NAME=$oldDeviceName"
}

[System.IO.File]::WriteAllLines($configFile, $configLines, [System.Text.Encoding]::UTF8)
Write-Ok $(if ($LangUI -eq "ru") { "Конфигурация сохранена в $configFile" } else { "Config saved to $configFile" })

# ── Windows Service registration ──────────────────────────────────────────────
Write-Hdr $(if ($LangUI -eq "ru") { "Настройка службы Windows" } else { "Configuring Windows service" })

$quotedBin = "`"$binDst`""
if (-not (Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue)) {
    New-Service -Name $SERVICE_NAME `
        -BinaryPathName $quotedBin `
        -DisplayName "Sota Headless Subscription Provider" `
        -StartupType Automatic `
        -Description "Generates Sota Connect subscription links for Mihomo, sing-box, v2ray" | Out-Null

    # Configure restart on failure (5s, 10s, 30s)
    & sc.exe failure $SERVICE_NAME reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null
    Write-Ok $(if ($LangUI -eq "ru") { "Служба $SERVICE_NAME успешно создана" } else { "Service $SERVICE_NAME created successfully" })
} else {
    # Ensure binary path and startup type are up to date
    & sc.exe config $SERVICE_NAME binPath= $quotedBin start= auto | Out-Null
    Write-Ok $(if ($LangUI -eq "ru") { "Конфигурация службы $SERVICE_NAME обновлена" } else { "Service $SERVICE_NAME configuration updated" })
}

# ── Windows Firewall rule ─────────────────────────────────────────────────────
$listenPort = 16698
if ($oldListen -match ":(\d+)$") {
    $listenPort = [int]$Matches[1]
}

if (-not (Get-NetFirewallRule -Name $FIREWALL_RULE_NAME -ErrorAction SilentlyContinue)) {
    try {
        New-NetFirewallRule -Name $FIREWALL_RULE_NAME `
            -DisplayName "sota-headless (TCP $listenPort)" `
            -Direction Inbound `
            -Protocol TCP `
            -LocalPort $listenPort `
            -Action Allow `
            -Profile Any | Out-Null
        Write-Ok $(if ($LangUI -eq "ru") { "Брандмауэр Windows: открыт входящий порт TCP $listenPort" } else { "Windows Firewall: allowed inbound TCP $listenPort" })
    } catch {
        Write-WarnMsg $(if ($LangUI -eq "ru") { "Не удалось настроить брандмауэр автоматически" } else { "Failed to configure firewall automatically" })
    }
} else {
    Write-Ok $(if ($LangUI -eq "ru") { "Правило брандмауэра для порта TCP $listenPort активно" } else { "Firewall rule for TCP $listenPort is active" })
}

# ── Start Service ─────────────────────────────────────────────────────────────
Write-Hdr $(if ($LangUI -eq "ru") { "Запуск службы" } else { "Starting service" })

Start-Service -Name $SERVICE_NAME
Start-Sleep -Seconds 2

# Verify health endpoint
$healthOk = $false
for ($i = 0; $i -lt 5; $i++) {
    try {
        $resp = Invoke-RestMethod -Uri "http://127.0.0.1:$listenPort/health" -TimeoutSec 2 -UseBasicParsing
        if ($resp -and $resp.ok) {
            $healthOk = $true
            break
        }
    } catch {
        Start-Sleep -Seconds 1
    }
}

if ($healthOk) {
    Write-Ok $(if ($LangUI -eq "ru") { "Служба запущена и отвечает на запросы!" } else { "Service is up and responding!" })
} else {
    Write-WarnMsg $(if ($LangUI -eq "ru") {
        "Служба запущена, но /health пока не ответил — возможно, идёт инициализация`n  Проверка логов в Event Viewer (Журналы Windows → Приложение)"
    } else {
        "Service started but /health not responding yet — may still be initializing`n  Check logs in Windows Event Viewer (Windows Logs → Application)"
    })
}

# ── Detect Local IP ───────────────────────────────────────────────────────────
$lanIp = "127.0.0.1"
try {
    $ipObj = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {
        $_.InterfaceAlias -notlike "*Loopback*" -and
        $_.IPAddress -notlike "169.254*" -and
        $_.IPAddress -ne "127.0.0.1"
    } | Select-Object -First 1
    if ($ipObj -and $ipObj.IPAddress) {
        $lanIp = $ipObj.IPAddress
    }
} catch {}

# ── Print Completion Summary ──────────────────────────────────────────────────
Write-Host ""
Write-Host "════════════════════════════════════════════" -ForegroundColor Yellow
Write-Host $(if ($LangUI -eq "ru") { "    Готово!    " } else { "    Done!    " }) -ForegroundColor Yellow
Write-Host "════════════════════════════════════════════" -ForegroundColor Yellow
Write-Host ""

if ($LangUI -eq "ru") {
    Write-Host "  Ссылки на подписки:" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  Mihomo / Clash / Zashboard" -ForegroundColor Yellow
    Write-Host "  http://${lanIp}:${listenPort}/sub/mihomo" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Sing-box outbounds" -ForegroundColor Yellow
    Write-Host "  http://${lanIp}:${listenPort}/sub/singbox" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Base64 (v2rayN, Nekoray, v2rayNG)" -ForegroundColor Yellow
    Write-Host "  http://${lanIp}:${listenPort}/sub/base64" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Прямые ссылки vless://" -ForegroundColor Yellow
    Write-Host "  http://${lanIp}:${listenPort}/sub/vless" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Проверка: curl http://127.0.0.1:${listenPort}/health" -ForegroundColor Gray
    Write-Host "  Статус:   Get-Service sota-headless" -ForegroundColor Gray
    Write-Host "  Перезапуск: Restart-Service sota-headless" -ForegroundColor Gray
    Write-Host "  Остановка:  Stop-Service sota-headless" -ForegroundColor Gray
    Write-Host '  Удаление: powershell -ExecutionPolicy Bypass -File scripts\uninstall.ps1' -ForegroundColor Gray
    Write-Host ""
} else {
    Write-Host "  Subscription URLs:" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  Mihomo / Clash / Zashboard" -ForegroundColor Yellow
    Write-Host "  http://${lanIp}:${listenPort}/sub/mihomo" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Sing-box outbounds" -ForegroundColor Yellow
    Write-Host "  http://${lanIp}:${listenPort}/sub/singbox" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Base64 (v2rayN, Nekoray, v2rayNG)" -ForegroundColor Yellow
    Write-Host "  http://${lanIp}:${listenPort}/sub/base64" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Plain vless:// links" -ForegroundColor Yellow
    Write-Host "  http://${lanIp}:${listenPort}/sub/vless" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "  Health:  curl http://127.0.0.1:${listenPort}/health" -ForegroundColor Gray
    Write-Host "  Status:  Get-Service sota-headless" -ForegroundColor Gray
    Write-Host "  Restart: Restart-Service sota-headless" -ForegroundColor Gray
    Write-Host "  Stop:    Stop-Service sota-headless" -ForegroundColor Gray
    Write-Host '  Uninstall: powershell -ExecutionPolicy Bypass -File scripts\uninstall.ps1' -ForegroundColor Gray
    Write-Host ""
}
