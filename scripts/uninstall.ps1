#Requires -Version 5.1
<#
.SYNOPSIS
    sota-headless uninstaller for Windows.
.DESCRIPTION
    Stops and removes the sota-headless Windows service, removes firewall rule,
    and cleans up installed files.
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File uninstall.ps1
    irm https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/uninstall.ps1 | iex
#>

param(
    [string]$InstallDir = $(if ($env:SOTA_BASE_DIR) { $env:SOTA_BASE_DIR } else { "$env:ProgramFiles\sota-headless" }),
    [string]$Lang = $(if ($env:SOTA_LANG) { $env:SOTA_LANG } else { "" }),
    [switch]$Force = $(if ($env:SOTA_FORCE -eq "1" -or $env:SOTA_FORCE -eq "true") { $true } else { $false })
)

$SERVICE_NAME = "sota-headless"
$FIREWALL_RULE_NAME = "sota-headless"

function Write-Ok([string]$text) {
    Write-Host "  ✓ $text" -ForegroundColor Green
}
function Write-Info([string]$text) {
    Write-Host "  • $text" -ForegroundColor Cyan
}
function Write-WarnMsg([string]$text) {
    Write-Host "  ! $text" -ForegroundColor Yellow
}
function Write-ErrMsg([string]$text) {
    Write-Host "  ✗ $text" -ForegroundColor Red
    exit 1
}
function Write-Hdr([string]$text) {
    Write-Host ""
    Write-Host "  $text" -ForegroundColor Cyan
    Write-Host "  $('-' * ($text.Length + 4))" -ForegroundColor DarkGray
}

function Assert-Administrator {
    $currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        Write-ErrMsg "This script must be run as Administrator (Right click → 'Run as Administrator')"
    }
}

Assert-Administrator

# Language detection
$langUI = $Lang
if (-not $langUI) {
    $culture = [System.Globalization.CultureInfo]::CurrentUICulture.TwoLetterISOLanguageName
    $langUI = if ($culture -eq "ru") { "ru" } else { "en" }
}

Write-Host ""
Write-Host "════════════════════════════════════════════" -ForegroundColor Yellow
Write-Host $(if ($langUI -eq "ru") { "sota-headless  ·  Удаление из Windows" } else { "sota-headless  ·  Windows uninstaller" }) -ForegroundColor Yellow
Write-Host "════════════════════════════════════════════" -ForegroundColor Yellow

# 1. Stop and remove service
$svc = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
if ($svc) {
    Write-Info $(if ($langUI -eq "ru") { "Остановка службы $SERVICE_NAME..." } else { "Stopping service $SERVICE_NAME..." })
    if ($svc.Status -eq "Running") {
        Stop-Service -Name $SERVICE_NAME -Force -ErrorAction SilentlyContinue
    }
    Write-Info $(if ($langUI -eq "ru") { "Удаление службы $SERVICE_NAME..." } else { "Removing service $SERVICE_NAME..." })
    & sc.exe delete $SERVICE_NAME | Out-Null
    Write-Ok $(if ($langUI -eq "ru") { "Служба удалена" } else { "Service removed" })
} else {
    Write-Info $(if ($langUI -eq "ru") { "Служба $SERVICE_NAME не найдена" } else { "Service $SERVICE_NAME not found" })
}

# 2. Remove firewall rule
$fw = Get-NetFirewallRule -Name $FIREWALL_RULE_NAME -ErrorAction SilentlyContinue
if ($fw) {
    Write-Info $(if ($langUI -eq "ru") { "Удаление правила брандмауэра..." } else { "Removing firewall rule..." })
    Remove-NetFirewallRule -Name $FIREWALL_RULE_NAME -ErrorAction SilentlyContinue
    Write-Ok $(if ($langUI -eq "ru") { "Правило брандмауэра удалено" } else { "Firewall rule removed" })
}

# 3. Clean files
if (Test-Path $InstallDir) {
    $remove = $true
    if (-not $Force) {
        $promptMsg = if ($langUI -eq "ru") {
            "Удалить каталог установки и конфигурацию ($InstallDir)? [Y/n]: "
        } else {
            "Remove installation directory and config ($InstallDir)? [Y/n]: "
        }
        Write-Host -NoNewline "  ? $promptMsg" -ForegroundColor Cyan
        $ans = [Console]::ReadLine()
        if ($ans -and $ans -notmatch "^[yYдД]") {
            $remove = $false
        }
    }
    if ($remove) {
        Remove-Item -Path $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Ok $(if ($langUI -eq "ru") { "Файлы удалены" } else { "Files removed" })
    } else {
        Write-Info $(if ($langUI -eq "ru") { "Файлы сохранены в $InstallDir" } else { "Files preserved in $InstallDir" })
    }
}

Write-Host ""
Write-Host "════════════════════════════════════════════" -ForegroundColor Yellow
Write-Host $(if ($langUI -eq "ru") { "    Удаление завершено!    " } else { "    Uninstall completed!    " }) -ForegroundColor Yellow
Write-Host "════════════════════════════════════════════" -ForegroundColor Yellow
Write-Host ""
