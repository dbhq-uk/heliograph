# =============================================================================
#  start.ps1 - the preflight.  Run this FIRST on a Windows control node.
# =============================================================================
#
#     .\start.ps1 --check     # changes nothing at all, and says what would work
#     .\start.ps1             # preflight, then hand over to the loop
#
#  The PowerShell twin of start.sh, and the table it prints is deliberately the
#  same shape: one line per check, `ok` / `warn` / `FAIL`, and every FAIL names
#  what to do about it. This output is often the only thing the person on the
#  far side has to work from, so a refusal that does not say what to change is
#  a wasted round trip through somebody who cannot debug the machine.
#
#  IT ANSWERS THE TWO QUESTIONS THAT DECIDE THIS ON A REAL ESTATE, and neither
#  is guessable from a version number:
#
#    CONSTRAINED LANGUAGE MODE   the capture cannot work under it. .NET method
#                                calls and type literals are refused, and that
#                                is most of caplib.psm1. The failure otherwise
#                                reads as a syntax error in somebody else's
#                                file rather than as a policy decision.
#
#    A GPO-SET EXECUTION POLICY  overrides `-ExecutionPolicy Bypass`. A station
#                                that launches fine by hand then refuses to
#                                launch from a scheduled task, and the reason
#                                is in a scope the effective value does not
#                                name.
#
#  --check CHANGES NOTHING. It is what gets run on a node where nobody is
#  permitted to alter anything yet, so the answer to "will this work here" can
#  be had before asking for permission.
# =============================================================================
Set-StrictMode -Version 2.0

$RepoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path

$CheckOnly = $false
$Rest = @()
foreach ($a in $args) {
    if ($a -ceq '--check') { $CheckOnly = $true } else { $Rest += $a }
}

$script:Failed = 0

# report <ok|warn|FAIL> <label> <detail>
# A warn is a fact worth knowing that does not stop a run. A FAIL always names
# what to do about it. Same widths as start.sh's, so an operator who has seen
# one table can read the other.
function report {
    param([string] $Status, [string] $Label, [string] $Detail)
    if ($Status -ceq 'FAIL') { $script:Failed++ }
    Write-Output ('{0,-4}  {1,-12}  {2}' -f $Status, $Label, $Detail)
}

Write-Output "heliograph preflight on $([System.Environment]::MachineName)"
Write-Output ''

# --- the interpreter ----------------------------------------------------------
# 5.1 is the floor. It is what ships with Windows and what a locked-down estate
# has, so it is the version this implementation is written against - not the one
# it tolerates.
$v = $PSVersionTable.PSVersion
if ($v.Major -gt 5 -or ($v.Major -eq 5 -and $v.Minor -ge 1)) {
    report ok 'powershell' "$v ($($PSVersionTable.PSEdition))"
} else {
    report FAIL 'powershell' "need 5.1 or newer, found $v. Windows PowerShell 5.1 ships with Windows Management Framework 5.1; install it, or use the bash station"
}

# --- THE ONE THAT STOPS A STATION DEAD ---------------------------------------
$mode = "$($ExecutionContext.SessionState.LanguageMode)"
if ($mode -ceq 'FullLanguage') {
    report ok 'language' 'FullLanguage, so the capture can call .NET'
} elseif ($mode -ceq 'ConstrainedLanguage') {
    report FAIL 'language' 'ConstrainedLanguage. The capture cannot work: .NET method calls and type literals are refused, and that is most of caplib.psm1. This is set by AppLocker/WDAC or by __PSLockDownPolicy. Have the policy relaxed for this account, or run the bash station instead - one implementation is better than two anyway'
} else {
    report FAIL 'language' "LanguageMode is '$mode', which is neither FullLanguage nor ConstrainedLanguage. The capture needs FullLanguage. Ask whoever set the policy"
}

# --- the other one ------------------------------------------------------------
# THE PER-SCOPE LIST, not the effective value. `Get-ExecutionPolicy` alone
# returns one word and does not say who set it, so it cannot distinguish "the
# default, which -ExecutionPolicy Bypass overrides" from "a GPO, which it does
# not". The difference decides whether this is fixable by the operator.
try {
    $policies = @(Get-ExecutionPolicy -List -ErrorAction Stop)
    $gpo = @($policies | Where-Object {
            $_.Scope -in 'MachinePolicy', 'UserPolicy' -and
            "$($_.ExecutionPolicy)" -notin 'Undefined', 'Bypass', 'Unrestricted'
        })
    if ($gpo.Count -eq 0) {
        report ok 'exec policy' 'no group policy forcing one, so -ExecutionPolicy Bypass works'
    } else {
        $named = ($gpo | ForEach-Object { "$($_.Scope)=$($_.ExecutionPolicy)" }) -join ', '
        report FAIL 'exec policy' "set by group policy ($named), which OVERRIDES -ExecutionPolicy Bypass. A station launched by hand may work while the same station launched from a scheduled task refuses. Either have the policy changed, or sign the payload with a certificate the policy trusts"
    }
} catch {
    report warn 'exec policy' "could not be read ($($_.Exception.Message)). On a non-Windows PowerShell there is no such policy and nothing to check"
}

# --- what the cancel will actually be able to do ------------------------------
# Reported rather than assumed, because the answer differs per estate and the
# operator is the one who finds out. See lib/cancel.psm1.
Import-Module (Join-Path $RepoRoot 'lib/cancel.psm1') -Force
$strategy = Enter-CapKillGroup
switch ($strategy) {
    'job-object' {
        report ok 'cancel' 'a Job Object with KILL_ON_JOB_CLOSE, so a cancel takes the whole tree and nothing can escape it'
    }
    'taskkill' {
        report warn 'cancel' 'Add-Type is blocked here, so a cancel falls back to taskkill /T /F. That walks the child tree at the moment it runs, so a process started immediately after can survive. Everything else works; if cancels matter on this estate, ask for Add-Type to be permitted'
    }
    default {
        report ok 'cancel' "$strategy - a kill reaches the step's whole process group"
    }
}

# --- the clock ----------------------------------------------------------------
# Every line of every log carries one, and the reader is somewhere else. A
# station whose clock is wrong produces logs nobody can line up with anything.
report ok 'clock' "$([DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ'))  <- compare this with a clock you trust"

# --- the account IS the blast radius ------------------------------------------
Import-Module (Join-Path $RepoRoot 'caplib.psm1') -Force
$who = Get-CapUser
if (-not (Test-CapPrivileged)) {
    report ok 'user' "$who - not privileged, so the blast radius is this account"
} elseif ($env:ALLOW_ROOT -ceq '1') {
    report warn 'user' "$who is privileged, permitted by ALLOW_ROOT=1. Every step will run with the whole machine in reach"
} else {
    report FAIL 'user' "$who is an Administrator or SYSTEM, and the runner refuses that: this toolkit has no credentials of its own, so the account it runs as is the whole blast radius. Run as an ordinary user, or set ALLOW_ROOT=1 if this image has no other"
}

# --- somewhere to put a log ---------------------------------------------------
$logDir = if ($env:LOG_DIR) { $env:LOG_DIR } else { Join-Path $RepoRoot 'ops-logs' }
try {
    if (-not (Test-Path -LiteralPath $logDir)) {
        if ($CheckOnly) {
            report warn 'ops-logs' "$logDir does not exist yet. --check creates nothing; a real start would make it"
        } else {
            [void](New-Item -ItemType Directory -Force -Path $logDir -ErrorAction Stop)
        }
    }
    if (Test-Path -LiteralPath $logDir) {
        # WRITTEN TO, not stat'ed. A directory that exists and cannot be written
        # to looks identical until the first log fails to arrive, an hour later,
        # with nobody left to tell.
        $probe = Join-Path $logDir '.heliograph-write-check'
        [System.IO.File]::WriteAllText($probe, 'x')
        Remove-Item -LiteralPath $probe -Force
        report ok 'ops-logs' 'writable'
    }
} catch {
    report FAIL 'ops-logs' "$logDir is not writable, so a capture would have nowhere to go: $($_.Exception.Message)"
}

# --- the payload is all here --------------------------------------------------
foreach ($need in 'run.ps1', 'caplib.psm1', 'lib/probe.psm1', 'lib/cancel.psm1') {
    if (Test-Path -LiteralPath (Join-Path $RepoRoot $need) -PathType Leaf) {
        report ok 'payload' "$need"
    } else {
        report FAIL 'payload' "$need is missing from $RepoRoot. Re-run the bootstrap: this payload is incomplete and the loop would fail on the first request"
    }
}

# --- the transport ------------------------------------------------------------
# SAID, not skipped. A preflight that printed nothing about the transport would
# read as "the transport is fine", and the honest answer today is that this
# implementation has none - so a station here captures locally and delivers
# nothing. Anybody who needs delivery on Windows should run the bash station,
# which has all four transports.
report warn 'transport' 'the PowerShell station has no transport yet, so a step is captured to ops-logs and NOT delivered. For a Windows station that ships its logs, use the bash station via station.ps1 with Git for Windows'

Write-Output ''
if ($script:Failed -gt 0) {
    Write-Output "preflight: $($script:Failed) blocking problem(s) above. Not starting the station."
    exit 1
}
report ok 'preflight' 'clear'

if ($CheckOnly) {
    Write-Output ''
    Write-Output '--check: nothing was changed, and nothing was started.'
    exit 0
}

# --- hand over ----------------------------------------------------------------
# There is no loop to hand over TO yet, and saying so is better than starting
# something that cannot poll. run.ps1 is what works today.
Write-Output ''
Write-Output 'The loop is not implemented for PowerShell yet - there is no transport for it to poll.'
Write-Output 'What works today is a step, run by hand or by a scheduled task:'
Write-Output ''
Write-Output '    .\run.ps1 env'
Write-Output ''
exit 0
