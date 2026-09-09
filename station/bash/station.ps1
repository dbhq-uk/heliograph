# =============================================================================
#  station.ps1 - start the heliograph loop on a Windows control node
# =============================================================================
#     .\station.ps1                 # preflight, then run the station
#     .\station.ps1 --check         # preflight only, change nothing
#     .\station.ps1 -- --once       # everything after -- goes to station.sh
#
#  THIS IS A LAUNCHER, NOT A PORT. It finds the bash that Git for Windows
#  already installed and hands over to start.sh. It reimplements nothing.
#
#  Why not a real PowerShell port. The capture pattern lives in caplib.sh and
#  there is exactly one of it: run.sh and caprun.sh own the log, the timestamps
#  and the push, and every step is bash. Porting would mean a second
#  implementation of the thing this toolkit is most careful about, and the two
#  would drift. The parts that look easiest to port are the ones that would
#  hurt: `sed -u` keeps the capture unbuffered so a line is stamped when it is
#  produced, and losing that gives you a log where every line carries the same
#  time, which reads like a working log while destroying the only property that
#  makes these logs worth having.
#
#  Git for Windows ships bash 4.4 or newer with GNU coreutils and a sed that
#  honours -u. Git is the transport, so a control node without git cannot
#  participate at all: the dependency is already paid for.
#
#  WHAT THIS DOES NOT SOLVE. The far side still needs bash to run the steps.
#  This makes a Windows machine able to HOST the loop; it does not make the
#  steps run natively on Windows. lib/remote.sh already reaches Windows hosts
#  over SSH and PowerShell, and that is the right shape for a Windows target.
# =============================================================================

$ErrorActionPreference = 'Stop'

# Find bash. Order matters: an explicit override first, then the registry,
# which is authoritative for where Git for Windows actually landed, then the
# usual paths, then PATH.
#
# PATH IS LAST ON PURPOSE. WSL puts a bash.exe in System32 that is not Git
# bash: it launches a Linux distribution, or fails with an install prompt if
# none exists. If a lookup found that first, the failure would be a confusing
# WSL message rather than anything about heliograph.
# Dot-sourced, not defined here, because service.ps1 needs the same answer and a
# second copy would be a second answer - see lib/Find-GitBash.ps1.
. (Join-Path (Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'lib') 'Find-GitBash.ps1')


$bash = Find-GitBash
Write-Host "station.ps1: using $bash"

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path

if (-not (Test-Path (Join-Path $repoRoot 'start.sh'))) {
    throw @"
station.ps1: no start.sh beside this script.

This has to run from inside a transport repo. Clone the repo the far side gave
you and run it from there, or bootstrap one with
station/bootstrap.sh.
"@
}

# Hand over. start.sh owns the preflight, the credential checks, the branch
# checkout and the handover to station.sh, exactly as on any other host.
#
# The path is converted to the form bash understands, and every argument is
# passed through untouched.
$repoUnix = $repoRoot -replace '\\', '/' -replace '^([A-Za-z]):', '/$1'
$argline = ($args | ForEach-Object { "'" + ($_ -replace "'", "'\''") + "'" }) -join ' '

# .station-env, if there is one, and `set -a` so it is EXPORTED.
#
# A relay, share or blob station is configured entirely by variables, and a
# scheduled task inherits nothing from the shell that registered it - the same
# fact service.ps1 has warned about for GIT_TOKEN since it was written. Windows
# has no systemd EnvironmentFile to reach for, so the file is sourced here, in
# the one place every Windows start goes through.
#
# `set -a` is the load-bearing half. Sourcing alone sets SHELL variables, and
# start.sh execs station.sh, which execs run.sh - each one a new process, which
# inherits environment variables and not shell ones. That exact omission made
# the launchd and setsid paths read the file and discard every value.
#
# LOADED FOR A MANUAL RUN TOO, deliberately. On Windows there is no habit of
# exporting variables in a profile before running something, so a station
# behaving differently by hand than under its task would be the surprise, not
# the consistency. It is the operator's own file, in their own payload.
$env_prefix = "set -a; [ -r ./.station-env ] && . ./.station-env; set +a; "

& $bash -lc "cd '$repoUnix' && $env_prefix exec ./start.sh $argline"
exit $LASTEXITCODE
