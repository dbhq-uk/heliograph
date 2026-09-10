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
    [string] $Label = 'conformance',
    # Written before the step starts, so a canceller has something to aim at
    # even if the capture never produces a line.
    [string] $HandleFile = ''
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here '../../../station/powershell/caplib.psm1') -Force
Import-Module (Join-Path $here '../../../station/powershell/lib/cancel.psm1') -Force

# THIS PROCESS AND EVERYTHING IT STARTS DIE TOGETHER. On Windows that is a Job
# Object with KILL_ON_JOB_CLOSE; elsewhere the caller has already put us in our
# own process group with setsid. Either way a canceller only needs this pid.
[void](Enter-CapKillGroup)
if ($HandleFile) {
    # Before the step, deliberately. A capture that hangs on its first line
    # still has to be cancellable, and a handle written afterwards would not be
    # there yet.
    [System.IO.File]::WriteAllText($HandleFile, "$PID")
}

# The host that runs the step is THIS host, found from the running process
# rather than assumed to be on PATH. `powershell` and `pwsh` are different
# programs and an estate may have either; whichever is running this one is by
# definition present and is the one the station would use.
$shell = [System.Diagnostics.Process]::GetCurrentProcess().MainModule.FileName

Write-CapHeader -Path $LogPath -Label $Label
$rc = Invoke-CapRun -Path $LogPath -FilePath $shell -ArgumentList @('-NoProfile', '-File', $Step)
Write-CapFooter -Path $LogPath -ExitCode $rc
exit $rc
