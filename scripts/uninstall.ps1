#Requires -Version 5.1
<#
.SYNOPSIS
    sota-headless uninstaller for Windows.
.DESCRIPTION
    Stops and removes the sota-headless Windows service, removes firewall rule,
    and cleans up installed files.
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\uninstall.ps1
#>

param(
    [string]$InstallDir = "$env:ProgramFiles\sota-headless",
    [string]$Lang = ""
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } elseif ($MyInvocation.MyCommand -and $MyInvocation.MyCommand.Path) { Split-Path -Parent $MyInvocation.MyCommand.Path } else { "." }
$installScript = Join-Path $scriptDir "install.ps1"

if (Test-Path $installScript) {
    & powershell -ExecutionPolicy Bypass -File $installScript -Uninstall -InstallDir $InstallDir -Lang $Lang
} else {
    Write-Error "install.ps1 not found in $scriptDir"
}
