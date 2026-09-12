# =============================================================================
#  service.ps1 - make the loop outlive the session, with no bash anywhere
# =============================================================================
#     .\service.ps1 install              # survive logout and reboot
#     .\service.ps1 install -- --once    # args after -- go to the loop
#     .\service.ps1 install -Force       # install anyway, over a refusal
#     .\service.ps1 status
#     .\service.ps1 logs
#     .\service.ps1 stop
#     .\service.ps1 uninstall
#
#  The twin of station/bash/service.ps1, and NOT the same file. That one starts
#  `station.ps1` in the bash payload - the LAUNCHER, which finds Git for Windows
#  and hands over to start.sh - and it refuses to install without a start.sh
#  beside it. On this payload there is no bash and no start.sh, so it would
#  refuse every time. Until this file existed, `--flavour powershell` planted no
#  way to survive a logout at all.
#
#  WHY A SCHEDULED TASK rather than a service. A Windows service needs
#  installation rights and a wrapper for a script; a scheduled task needs
#  neither, runs as the operator, and can be told to run whether that operator
#  is logged on or not, which is the whole requirement.
#
#  IT REGISTERS start.ps1, NOT station.ps1. The preflight runs on every start,
#  which is the point: a machine that has since had Constrained Language Mode
#  applied, or lost its share mount, should refuse and say why rather than
#  starting a loop that cannot work. start.ps1 hands over to the loop itself.
#
#  THE HARD PART IS NOT THE TASK. It is that a detached task starts with a
#  FRESH ENVIRONMENT and inherits nothing from the shell that registered it -
#  so the transport's variables, and any credential, simply are not there. The
#  loop then starts, polls happily, and cannot deliver a single log; the far
#  side waits for hours. That is what `.station-env-ps` below is for, and it is
#  the reason this file is not fifty lines long.
# =============================================================================
Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$RepoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path

# Overridable for the reason the bash side's is: one transport repo per
# investigation is ordinary, a fixed name would let the second install silently
# replace the first, and CI must not unregister a task somebody is relying on.
$TaskName = if ($env:HELIOGRAPH_SERVICE_NAME) { $env:HELIOGRAPH_SERVICE_NAME } else { 'heliograph-ps' }
$LogFile = Join-Path $RepoRoot '.station-service.log'
$EnvFile = Join-Path $RepoRoot '.station-env-ps'

# --- what the task has to be told --------------------------------------------
# EVERY VARIABLE A TRANSPORT OR THE LOOP READS, because the task inherits none
# of them.
#
# A LIST RATHER THAN "EVERYTHING IN THE ENVIRONMENT", deliberately. Copying the
# whole environment into a file on disk would put PATH, TEMP, and whatever else
# the operator happens to have exported into a file that also holds a token -
# and the file would then differ on every machine, which makes "what is this
# station configured with" unanswerable.
#
# It is grouped so that adding a transport means adding its variables here and
# nowhere else.
$CarriedVars = @(
    # which channel, and the loop's own knobs
    'TRANSPORT', 'INTERVAL', 'ALLOW_ACTIONS', 'ALLOW_ROOT', 'REQUIRE_PIN',
    'PROGRESS_EVERY', 'ACTION_ENV',
    # git
    'REPO_ROOT', 'GIT_TOKEN', 'GIT_TOKEN_FILE', 'GIT_TOKEN_USER', 'GIT_AUTH_HEADER',
    'GIT_AUTHOR_NAME', 'GIT_AUTHOR_EMAIL',
    # share
    'SHARE_DIR', 'SHARE_SCOPE'
)

# Which of those are secret, for the reporting below. Named rather than guessed
# from the value, because a station that is unsure whether it holds a credential
# should say the cautious thing.
$SecretVars = @('GIT_TOKEN', 'GIT_AUTH_HEADER')

function Assert-Prereqs {
    foreach ($f in 'start.ps1', 'station.ps1', 'run.ps1', 'caplib.psm1') {
        if (-not (Test-Path -LiteralPath (Join-Path $RepoRoot $f))) {
            throw "service.ps1: no $f beside this script. Run it from inside a planted station."
        }
    }
    # A CLEAR REFUSAL RATHER THAN A CONFUSING ONE. This payload has no bash, so
    # somebody who reached for it expecting the launcher should be told which
    # file they want rather than watching a task fail at boot.
    if (Test-Path -LiteralPath (Join-Path $RepoRoot 'start.sh')) {
        Write-Warning "there is a start.sh here too, so this repo carries both payloads."
        Write-Warning "  This installs the POWERSHELL loop. For the bash one, use the"
        Write-Warning "  service.ps1 that came with it, and give them different"
        Write-Warning "  HELIOGRAPH_SERVICE_NAME values or the second will replace the first."
    }
}

# --- carrying the configuration across the gap -------------------------------
function Write-EnvFile {
    <#
      .SYNOPSIS
      Record the transport's variables where a detached task can read them.

      .DESCRIPTION
      PLAIN `KEY=value`, ONE PER LINE, AND NOT POWERSHELL. A `.ps1` here would
      be a file the loop executes at every start, sitting in a directory the far
      side can write to on some transports. Configuration must not be code.

      Values are written verbatim and read verbatim - no quoting, no escaping,
      no evaluation - so a token containing a quote, a space or a backslash
      survives. That is why the format is one variable per line with the first
      `=` as the separator: it needs no parser that could be wrong.
    #>
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add('# heliograph station environment, written by service.ps1.')
    $lines.Add('# KEY=value, one per line, read verbatim. Not executed.')
    $lines.Add('# Delete this file to make the station forget its configuration.')
    $carried = @()
    foreach ($n in $CarriedVars) {
        $v = [System.Environment]::GetEnvironmentVariable($n)
        if (-not $v) { continue }
        # A NEWLINE WOULD FORGE A SECOND VARIABLE, which is the same injection
        # the share transport refuses in a scope name. Refused rather than
        # stripped: silently changing a credential is worse than not writing it.
        if ($v.Contains("`n") -or $v.Contains("`r")) {
            throw "$n contains a newline, which cannot be carried in this file. Fix the value and run this again."
        }
        $lines.Add("$n=$v")
        $carried += $n
    }
    [System.IO.File]::WriteAllLines($EnvFile, $lines, (New-Object System.Text.UTF8Encoding($false)))
    Protect-EnvFile
    return $carried
}

function Protect-EnvFile {
    <#
      .SYNOPSIS
      Make the env file readable by this account and nobody else.
      .DESCRIPTION
      IT HOLDS A TOKEN. The bash side's `.station-env` is chmod 600 and this is
      the Windows equivalent: inheritance off, every inherited rule dropped, one
      rule granting the owner. Without breaking inheritance first, removing the
      rules does nothing - they come back from the parent directory, which on a
      transport repo may be readable by anyone who can read the repo.
    #>
    try {
        $acl = Get-Acl -LiteralPath $EnvFile
        $acl.SetAccessRuleProtection($true, $false)
        foreach ($r in @($acl.Access)) { [void]$acl.RemoveAccessRule($r) }
        $me = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
        $acl.AddAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule(
                    $me, 'FullControl', 'None', 'None', 'Allow')))
        Set-Acl -LiteralPath $EnvFile -AclObject $acl
    } catch {
        # SAID, NOT SWALLOWED. A file holding a token whose permissions could
        # not be tightened is something the operator has to know about, and it
        # is not a reason to refuse the install - the alternative is no station.
        Write-Warning "could not restrict permissions on $EnvFile : $($_.Exception.Message)"
        Write-Warning "  It may hold a token. Check who can read it."
    }
}

function Test-Configuration {
    <#
      .SYNOPSIS
      $true when a detached task would actually be able to deliver.
      .DESCRIPTION
      THE CREDENTIAL IS WHERE AN UNATTENDED LOOP FAILS, and it fails silently:
      the loop starts, polls, captures a perfect log and cannot deliver it. The
      point of checking here is that somebody is still watching.

      NOTHING HERE MAY Write-Output, AND THAT IS NOT STYLE. Everything a
      PowerShell function writes to the output stream IS part of its return
      value, so one informational `Write-Output "transport: share"` made this
      return the ARRAY ("transport: share", $false) - and `-not $array` on two
      elements is $false, so the refusal below never fired. The installer then
      wrote its config file and went on to register a task for a station that
      could not deliver.

      Caught by the assertion that a refused install leaves no config file
      behind. Status goes to Write-Host and problems to Write-Warning; neither
      is on the output stream.
    #>
    $transport = if ($env:TRANSPORT) { $env:TRANSPORT } else { 'git' }
    Write-Host "transport: $transport"

    if ($transport -ceq 'share') {
        $ok = $true
        foreach ($n in 'SHARE_DIR', 'SHARE_SCOPE') {
            if (-not [System.Environment]::GetEnvironmentVariable($n)) {
                Write-Warning "$n is not set in this shell, so the task will not have it either."
                $ok = $false
            }
        }
        if ($ok -and -not (Test-Path -LiteralPath $env:SHARE_DIR -PathType Container)) {
            Write-Warning "SHARE_DIR ($($env:SHARE_DIR)) is not mounted here. A task at boot may run before it is."
        }
        return $ok
    }

    if ($transport -cne 'git') {
        # Another transport settles its own requirements, and start.ps1 will
        # report them at every start. Nothing here can second-guess it.
        return $true
    }

    $url = ''
    try { $url = (& git -C $RepoRoot remote get-url origin 2>$null | Select-Object -First 1) } catch { $url = '' }
    if (-not $url) {
        Write-Warning "there is no origin remote. Git is the transport, so there is nowhere to push a log."
        return $false
    }
    if ($url -notmatch '^https?://') { return $true }   # ssh uses a key; a path needs nothing

    # A FILE SURVIVES A DETACHED START. An environment variable in this shell
    # does not - and that is the whole trap, so it is named rather than hinted
    # at. It IS carried into the env file below, which is why this is a warning
    # about where the secret now lives rather than a refusal.
    foreach ($p in @($env:GIT_TOKEN_FILE, (Join-Path $RepoRoot '.git-token'), (Join-Path $HOME '.git-token'))) {
        if ($p -and (Test-Path -LiteralPath $p)) { return $true }
    }
    if ($env:GIT_TOKEN -or $env:GIT_AUTH_HEADER) {
        Write-Warning "the credential is in this shell's environment and will be COPIED into"
        Write-Warning "  $EnvFile so the task can read it. That file is restricted to this"
        Write-Warning "  account. If you would rather it lived somewhere else, write it to a"
        Write-Warning "  file and set GIT_TOKEN_FILE instead:"
        Write-Warning "      `$env:GIT_TOKEN | Out-File -NoNewline -Encoding ascii `"`$HOME\.git-token`""
        return $true
    }
    Write-Warning "no credential found at all. The loop would start and be unable to push."
    return $false
}

# --- the verbs ----------------------------------------------------------------
function Install-Service {
    param([switch] $Force, [string[]] $StartArgs = @())
    Assert-Prereqs

    $ok = Test-Configuration
    if (-not $ok -and -not $Force) {
        throw "Refusing to install a loop that cannot deliver. Fix the above, or pass -Force if you know better."
    }

    $carried = Write-EnvFile
    Write-Output "carried into $($EnvFile | Split-Path -Leaf): $(if ($carried) { $carried -join ', ' } else { '(nothing - the task will use defaults)' })"
    foreach ($n in $carried) {
        if ($SecretVars -ccontains $n) { Write-Output "  $n is a credential, and that file is restricted to this account" }
    }

    # -Command rather than -File, because a task's output goes nowhere by
    # default and a loop you cannot read is barely better than one that died.
    # *>&1 catches every stream, including the information stream the preflight
    # writes its table to.
    $tail = ''
    if ($StartArgs.Count) {
        $tail = ' ' + (($StartArgs | ForEach-Object { "'" + ($_ -replace "'", "''") + "'" }) -join ' ')
    }
    $inner = "& '$RepoRoot\start.ps1'$tail *>&1 | Out-File -FilePath '$LogFile' -Encoding utf8 -Append"
    $action = New-ScheduledTaskAction -Execute 'powershell.exe' `
        -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command `"$inner`"" `
        -WorkingDirectory $RepoRoot

    # AtStartup, so it comes back after a reboot with nobody logged in.
    $trigger = New-ScheduledTaskTrigger -AtStartup

    # S4U runs the task whether the operator is logged on or not and stores no
    # password. It gets no network CREDENTIALS, which does not matter here: the
    # transport authenticates with a token or a key, not the Windows identity.
    $principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" `
        -LogonType S4U -RunLevel Limited

    # TWO SETTINGS THAT ARE NOT INCIDENTAL.
    #
    # ExecutionTimeLimit Zero means no limit. The default is three days, after
    # which Windows stops the task - and a loop that quietly stops after three
    # days is precisely the failure this file exists to prevent.
    #
    # RestartCount matters MORE here than on the bash side. A PowerShell station
    # that updates itself EXITS 75 rather than re-executing, because PowerShell
    # has no exec and a respawn is killed by the Job Object the cancel depends
    # on. Without a restart policy, a self-update stops the station instead of
    # replacing it - it would look exactly like a station that finished.
    $settings = New-ScheduledTaskSettingsSet `
        -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
        -ExecutionTimeLimit ([TimeSpan]::Zero) `
        -RestartCount 5 -RestartInterval (New-TimeSpan -Minutes 1) `
        -MultipleInstances IgnoreNew `
        -StartWhenAvailable

    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
    Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
        -Principal $principal -Settings $settings `
        -Description "heliograph PowerShell station ($RepoRoot)" | Out-Null

    Start-ScheduledTask -TaskName $TaskName
    Start-Sleep -Seconds 3

    Write-Output ''
    Write-Output "installed: scheduled task '$TaskName'"
    if ($StartArgs.Count) { Write-Output "arguments: $($StartArgs -join ' ')" }
    Write-Output 'mechanism: scheduled task, runs whether logged on or not, starts at boot'
    Write-Output "           and restarts if the loop exits, which is how a self-update takes effect"
    Write-Output "log      : $LogFile"
    Write-Output ''
    Write-Output '  .\service.ps1 status     what it is doing'
    Write-Output '  .\service.ps1 logs       follow the log'
    Get-Status
}

function Get-Status {
    $t = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if (-not $t) {
        Write-Output 'not installed. Run: .\service.ps1 install'
        return
    }
    $info = Get-ScheduledTaskInfo -TaskName $TaskName
    Write-Output "task     : $TaskName"
    Write-Output "state    : $($t.State)"
    Write-Output "last run : $($info.LastRunTime)  result=$($info.LastTaskResult)"
    Write-Output "next run : $($info.NextRunTime)"
    Write-Output "log      : $LogFile"
    Write-Output "config   : $EnvFile"
    # 267009 is "currently running", which reads like an error code to anybody
    # who has not looked it up.
    if ($info.LastTaskResult -eq 267009) {
        Write-Output '           (267009 means it is running now, not a failure)'
    }
    # 75 IS THE STATION ASKING TO BE RESTARTED, not a fault. Said here because
    # an operator reading a non-zero result assumes the worst, and this is the
    # one code that means the opposite.
    if ($info.LastTaskResult -eq 75) {
        Write-Output '           (75 means the station updated itself and asked to be restarted)'
    }
}

function Show-Logs {
    if (-not (Test-Path -LiteralPath $LogFile)) {
        throw "no log yet at $LogFile. Has it started? Run .\service.ps1 status"
    }
    Get-Content -LiteralPath $LogFile -Tail 50 -Wait
}

function Stop-Loop {
    $t = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if (-not $t) { Write-Output 'nothing was running'; return }
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    Write-Output "stopped '$TaskName'"
}

function Uninstall-Service {
    Stop-Loop
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
    Write-Output "removed the scheduled task '$TaskName'"
    # THE CONFIG FILE GOES TOO, because it may hold a token and leaving a
    # credential behind after an uninstall is not tidying up. The log stays:
    # it is the evidence, and it is what somebody reads to find out why they
    # are uninstalling.
    if (Test-Path -LiteralPath $EnvFile) {
        Remove-Item -LiteralPath $EnvFile -Force -ErrorAction SilentlyContinue
        Write-Output "removed $EnvFile, which may have held a credential"
    }
    Write-Output "the log is left at $LogFile"
}

$argv = @($args)
$verb = if ($argv.Count) { $argv[0] } else { '' }
switch -CaseSensitive ($verb) {
    'install' {
        $rest = @($argv | Select-Object -Skip 1 | Where-Object { $_ -cne '-Force' -and $_ -cne '--' })
        Install-Service -Force:($argv -ccontains '-Force') -StartArgs $rest
    }
    'status' { Get-Status }
    'logs' { Show-Logs }
    'stop' { Stop-Loop }
    'uninstall' { Uninstall-Service }
    default {
        Write-Output 'service.ps1 - make the PowerShell loop outlive the session'
        Write-Output ''
        Write-Output '  .\service.ps1 install     survive logout and reboot, and start now'
        Write-Output '  .\service.ps1 status'
        Write-Output '  .\service.ps1 logs'
        Write-Output '  .\service.ps1 stop'
        Write-Output '  .\service.ps1 uninstall'
        Write-Output ''
        Write-Output 'It carries the transport variables from THIS shell into a restricted'
        Write-Output 'file, because a scheduled task inherits none of them.'
        exit 2
    }
}
