# =============================================================================
#  powershell-stop.ps1 - cancel a capture, and prove it is gone
# =============================================================================
# The conformance suite hands the driver a handle and asks it to cancel. On
# Windows there is no `kill -TERM -- -pid`, so this is where the platform's own
# answer lives: see station/powershell/lib/cancel.psm1.
#
# Exits 0 only when the process is CONFIRMED dead. A cancel that was merely
# signalled leaves the suite racing a step still writing to the log it is about
# to read.
# =============================================================================
[CmdletBinding()]
param([Parameter(Mandatory = $true)][string] $HandleFile)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here '../../../station/powershell/lib/cancel.psm1') -Force

$raw = (Get-Content -LiteralPath $HandleFile -Raw).Trim()
if (-not $raw) { Write-Error 'the handle file is empty'; exit 1 }
if (Stop-CapTree -ProcessId ([int]$raw)) { exit 0 }
exit 1
