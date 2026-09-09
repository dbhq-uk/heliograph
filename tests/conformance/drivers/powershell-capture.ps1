# =============================================================================
#  powershell-capture.ps1 - one capture, for the conformance driver
# =============================================================================
# The suite is bash and the implementation under test is PowerShell, so the
# driver shells out and this is what it shells out to. Header, run, footer -
# exactly what run.ps1 will do in PR 10, and nothing else.
#
# It exits with the STEP's code, which is the property under test (p3). A
# wrapper that exited 0 because it itself succeeded would publish every failed
# run as a success.
# =============================================================================
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string] $LogPath,
    [Parameter(Mandatory = $true)][string] $Step,
    [string] $Label = 'conformance'
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here '../../../station/powershell/caplib.psm1') -Force

# The host that runs the step is THIS host, found from the running process
# rather than assumed to be on PATH. `powershell` and `pwsh` are different
# programs and an estate may have either; whichever is running this one is by
# definition present and is the one the station would use.
$shell = [System.Diagnostics.Process]::GetCurrentProcess().MainModule.FileName

Write-CapHeader -Path $LogPath -Label $Label
$rc = Invoke-CapRun -Path $LogPath -FilePath $shell -ArgumentList @('-NoProfile', '-File', $Step)
Write-CapFooter -Path $LogPath -ExitCode $rc
exit $rc
