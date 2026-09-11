# =============================================================================
#  transports/git.psm1 - the transport repo itself
# =============================================================================
# The default, and the one most estates already have: a private repo both sides
# can reach. The control side commits a request, the station commits the log,
# and git history IS the audit trail - which no other transport here gives you
# for free.
#
# EVERY git COMMAND THE STATION ISSUES IS IN THIS FILE. That rule is inherited
# from transports/git.sh and it is what keeps the rest of the payload able to
# run on a channel that is not git at all. Three commands once lived in
# start.sh, and that is exactly what made start.sh refuse to start a station
# with no git.
#
# THE CREDENTIAL NEVER REACHES argv. `git -c http.extraHeader=...` puts the
# token in the process table, where every other user on the box can read it out
# of `ps`. It goes through GIT_CONFIG_* environment variables instead, which is
# the same arrangement transports/git.sh uses and for the same reason.
# =============================================================================

Set-StrictMode -Version 2.0

$script:RepoRoot = ''
$script:Branch = ''
$script:WriteCheckRef = 'refs/heads/heliograph-write-check'

# $LASTEXITCODE IS UNSET UNTIL THE FIRST NATIVE CALL, and under
# Set-StrictMode reading an unset variable THROWS rather than yielding $null.
# So a module that asks about it before running anything native dies on a line
# that looks like an ordinary check. caplib.psm1 documents the other half of
# this trap - that it is also left stale by anything non-native.
function Get-LastExit {
    if (Test-Path 'variable:global:LASTEXITCODE') { return $global:LASTEXITCODE }
    return 0
}

function Get-TpCapabilities { return 'request status progress live self history' }

function Initialize-Tp {
    $script:RepoRoot = if ($env:REPO_ROOT) { $env:REPO_ROOT } else { (Get-Location).Path }

    if (-not (Get-Command git -CommandType Application -ErrorAction SilentlyContinue)) {
        Write-CapTpError "git is not on PATH, and it IS this transport. Install it, or set TRANSPORT to a channel that does not need it: $((Get-TpNames) -join ', ')"
        return $false
    }

    $script:Branch = (& git -C $script:RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Select-Object -First 1)
    if ((Get-LastExit) -ne 0 -or -not $script:Branch) {
        Write-CapTpError "$($script:RepoRoot) is not a git checkout, so the git transport has nothing to push to. Clone the transport repo, or set TRANSPORT to another channel"
        return $false
    }
    if ($script:Branch -ceq 'HEAD') {
        # A DETACHED HEAD IS REFUSED, because the branch is which machine this
        # station answers for. Pushing from a detached HEAD goes nowhere useful
        # and the operator would be waiting on a log that is committed to
        # nothing.
        Write-CapTpError "detached HEAD. The branch is which machine this station answers for, so there is nowhere for a log to go. Check out the task branch first"
        return $false
    }
    return $true
}

function Get-TpScope { return $script:Branch }
function Get-TpRevision { return "git $($script:Branch)" }

function Get-TpDescribe {
    <#
      .SYNOPSIS
      What this transport is, with NO CREDENTIAL in the answer.
      .DESCRIPTION
      Both callers print this - the preflight table and the startup banner,
      which goes into a log that gets committed and pushed. A token in a remote
      URL is the commonest way one ends up in a repository for ever, so the URL
      is masked here rather than at each call site.
    #>
    $url = (& git -C $script:RepoRoot remote get-url origin 2>$null | Select-Object -First 1)
    if ((Get-LastExit) -ne 0 -or -not $url) { $url = '(no remote named origin)' }
    return "git $(Hide-GitCredential $url) on $($script:Branch)"
}

function Hide-GitCredential {
    <#
      .SYNOPSIS
      Mask BOTH userinfo components of a URL, not just the password.
      .DESCRIPTION
      `https://<token>:x-oauth-basic@host` is a documented git form: the SECRET
      IS THE USERNAME and the password is a fixed placeholder. Masking only the
      password prints the token in full, in the shape most likely to carry a
      real credential. So the username goes too. caplib.sh's cap_mask_url makes
      the same trade for the same reason.
    #>
    param([string] $Url)
    if (-not $Url) { return $Url }
    $out = [regex]::Replace($Url, '(://[^/@:]*):[^/@]*@', '${1}:***@')
    $out = [regex]::Replace($out, '(?i)(https?://)[^/@:?#,]*@', '${1}***@')
    return $out
}

# --- the credential, out of argv ---------------------------------------------
function Get-GitAuthHeader {
    <#
      .SYNOPSIS
      The Authorization header value, or '' when there is no credential.
      .DESCRIPTION
      Three sources, in the same order transports/git.sh takes them:
      GIT_AUTH_HEADER verbatim, GIT_TOKEN, then the FIRST LINE of the file named
      by GIT_TOKEN_FILE - first line, because a secret mounted as a file
      usually has a trailing newline and sometimes has more than one line.
    #>
    if ($env:GIT_AUTH_HEADER) { return $env:GIT_AUTH_HEADER }

    $tok = ''
    if ($env:GIT_TOKEN) {
        $tok = $env:GIT_TOKEN
    } elseif ($env:GIT_TOKEN_FILE) {
        try {
            # An unreadable file yields no token, which is handled below. The
            # realistic mistake is GIT_TOKEN_FILE naming a mounted secrets
            # DIRECTORY rather than a file inside it.
            # ReadAllLines strips the terminator and nothing else, which is
            # what `sed -n 1p` does on the bash side. `.Trim()` was wrong here:
            # it also removes leading and trailing bytes OF THE TOKEN, and a
            # token that differs by one byte fails authentication with a
            # message that says nothing about whitespace.
            $lines = [System.IO.File]::ReadAllLines($env:GIT_TOKEN_FILE)
            if ($lines.Count -gt 0) { $tok = $lines[0] }
        } catch {
            return ''
        }
    }
    if (-not $tok) { return '' }

    $user = if ($env:GIT_TOKEN_USER) { $env:GIT_TOKEN_USER } else { '' }
    $pair = "$user`:$tok"
    $b64 = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($pair))
    return "Basic $b64"
}

function Test-GitEnvConfig {
    <#
      .SYNOPSIS
      $true when this git understands GIT_CONFIG_COUNT/KEY/VALUE (2.31+).
      .DESCRIPTION
      OLDER GIT SILENTLY IGNORES THEM. Not an error - it just runs
      unauthenticated, so the push fails with a credential message while the
      preflight says a credential is configured. caplib.sh checks the same
      version for the same reason.
    #>
    $v = (& git --version 2>$null | Select-Object -First 1)
    if ((Get-LastExit) -ne 0 -or -not $v) { return $false }
    if ($v -notmatch 'git version (\d+)\.(\d+)') { return $false }
    $maj = [int]$Matches[1]; $min = [int]$Matches[2]
    if ($maj -gt 2) { return $true }
    return ($maj -eq 2 -and $min -ge 31)
}

function Invoke-CapGit {
    <#
      .SYNOPSIS
      git, with the credential supplied through the environment.
      Returns the combined output; sets $script:GitExit.

      .DESCRIPTION
      GIT_CONFIG_COUNT/KEY/VALUE rather than `git -c`, because `-c` puts the
      header in argv where `ps` shows it to every user on the machine.

      APPENDED AT THE NEXT FREE INDEX rather than hard-coded at slot 0: an
      operator on a locked-down node may already export their own GIT_CONFIG_*
      (an ambient http.proxy, sslCAInfo, safe.directory). Slot 0 would
      overwrite theirs and truncate the rest off the list, and every
      authenticated push would start failing with a network error on a machine
      nobody can log into to diagnose.
    #>
    param([Parameter(ValueFromRemainingArguments = $true)][string[]] $GitArgs)

    $hdr = Get-GitAuthHeader
    $saved = @{}
    $useEnv = $false
    if ($hdr) { $useEnv = Test-GitEnvConfig }
    if ($hdr -and -not $useEnv) {
        # THE LESSER EVIL, and the same trade caplib.sh makes. On git older than
        # 2.31 the environment mechanism does not exist, so the choice is
        # `-c http.extraHeader=` - which puts the header in argv where `ps` can
        # read it - or no credential at all, which means no delivery. A log that
        # never arrives is worse than a token visible to whoever is already on
        # this machine, and the preflight reports which is in force.
        try {
            $out = & git -C $script:RepoRoot -c "http.extraHeader=Authorization: $hdr" @GitArgs 2>&1 |
                   ForEach-Object { "$_" }
            $script:GitExit = Get-LastExit
            return ($out -join "`n")
        } catch {
            $script:GitExit = 1
            return "$($_.Exception.Message)"
        }
    }
    if ($hdr) {
        $n = 0
        if ($env:GIT_CONFIG_COUNT -match '^\d+$') { $n = [int]$env:GIT_CONFIG_COUNT }
        foreach ($k in 'GIT_CONFIG_COUNT', "GIT_CONFIG_KEY_$n", "GIT_CONFIG_VALUE_$n") {
            $saved[$k] = [System.Environment]::GetEnvironmentVariable($k)
        }
        [System.Environment]::SetEnvironmentVariable("GIT_CONFIG_KEY_$n", 'http.extraHeader')
        [System.Environment]::SetEnvironmentVariable("GIT_CONFIG_VALUE_$n", "Authorization: $hdr")
        [System.Environment]::SetEnvironmentVariable('GIT_CONFIG_COUNT', "$($n + 1)")
    }
    try {
        $out = & git -C $script:RepoRoot @GitArgs 2>&1 | ForEach-Object { "$_" }
        $script:GitExit = Get-LastExit
        return ($out -join "`n")
    } finally {
        foreach ($k in $saved.Keys) {
            [System.Environment]::SetEnvironmentVariable($k, $saved[$k])
        }
    }
}

function Test-Tp {
    <#
      .SYNOPSIS
      The remote answers AND the credential is accepted.
      .DESCRIPTION
      READ AND WRITE, separately. `ls-remote` proves reachability and a read
      credential; a push credential is a different grant on most hosts, and a
      station that can read and not write captures a perfect log it can never
      deliver. `push --dry-run` asks the question without changing anything.
    #>
    $null = Invoke-CapGit ls-remote --heads origin
    if ($script:GitExit -ne 0) {
        Write-CapTpError "the remote did not answer, or refused this credential. './start.ps1 --check' reports which credential is in force"
        return $false
    }
    # A REF THAT DOES NOT EXIST, and the choice is load-bearing rather than
    # arbitrary. `push --dry-run origin HEAD` is refused LOCALLY as a
    # non-fast-forward the moment origin holds a commit this checkout lacks -
    # which is the ordinary state every time a station starts, after any push
    # by anyone. The credential is fine and the message blames it.
    #
    # A ref that does not exist cannot be a non-fast-forward, --dry-run creates
    # nothing, and the push still negotiates with git-receive-pack, which is
    # the service write access is granted on. transports/git.sh uses this exact
    # ref name, and it is fixed rather than generated so it is greppable in a
    # git host's audit log.
    $null = Invoke-CapGit push --dry-run origin "HEAD:$script:WriteCheckRef"
    if ($script:GitExit -ne 0) {
        Write-CapTpError "the remote is readable but refused a push. A read credential and a write credential are different grants on most hosts, and a station that cannot push captures logs it can never deliver"
        return $false
    }
    return $true
}

function Send-TpLog {
    <#
      .SYNOPSIS
      Commit the log and push it. $true only when the far side has it.
      .DESCRIPTION
      A FAILED PUSH MUST NEVER LOSE THE LOG, and must never be reported as a
      success. The commit happens first, so the file is safe locally whatever
      the network does, and the caller is told plainly where it is.

      IT PUSHES BARE, RESOLVING THROUGH UPSTREAM, and Push-GitScope - which the
      status path uses - names origin and the branch explicitly. That is a real
      inconsistency and it is left here on purpose.

      caplib.sh's cap_push, which is the bash tp_put_log, pushes bare too. A
      branch whose upstream is some other remote therefore delivers logs
      somewhere the control side never reads, on BOTH implementations, and
      reports success. Fixing it on one side would make the twins disagree about
      where a log goes, which is the one thing they may not do. It is recorded
      in PLAN.md to be fixed on both sides together, with a test that watches
      the remote rather than the exit code.
    #>
    param(
        [Parameter(Mandatory = $true)][string] $LogPath,
        [string] $Message = ''
    )
    if (-not (Test-Path -LiteralPath $LogPath -PathType Leaf)) {
        Write-CapTpError "there is no log at $LogPath to deliver"
        return $false
    }
    if (-not $Message) { $Message = "step log $(Split-Path -Leaf $LogPath) ***NO_CI***" }

    # -f because ops-logs/ is gitignored in some payloads: the log is the point,
    # and an ignore rule written for a working tree must not silence delivery.
    $null = Invoke-CapGit add -f -- $LogPath
    if ($script:GitExit -ne 0) {
        Write-CapTpError "could not stage $LogPath"
        return $false
    }

    $null = Invoke-CapGit diff --cached --quiet -- $LogPath
    if ($script:GitExit -eq 0) {
        # Nothing staged: this exact content is already committed. Not a
        # failure - re-delivering an identical log is a no-op, not an error.
        return $true
    }

    $who = Get-CapUser
    $name = if ($env:GIT_AUTHOR_NAME) { $env:GIT_AUTHOR_NAME } else { $who }
    $mail = if ($env:GIT_AUTHOR_EMAIL) { $env:GIT_AUTHOR_EMAIL } else { "$who@localhost" }
    $null = Invoke-CapGit -c "user.name=$name" -c "user.email=$mail" commit -q -m $Message -- $LogPath
    if ($script:GitExit -ne 0) {
        Write-CapTpError "could not commit $LogPath"
        return $false
    }

    # A rebase left half-applied hands the operator a checkout mid-rebase with
    # no idea why, and the next run fails before it starts - on a machine where
    # nobody can investigate that.
    $null = Invoke-CapGit pull --rebase --quiet
    if ($script:GitExit -ne 0) {
        $null = Invoke-CapGit rebase --abort
    }

    $null = Invoke-CapGit push --quiet
    if ($script:GitExit -eq 0) { return $true }

    # A branch created on the control node has no upstream, and a bare push
    # refuses rather than guessing. Without this the very first run on a new
    # branch would capture the log and then fail to ship it.
    $null = Invoke-CapGit push --quiet -u origin HEAD
    if ($script:GitExit -eq 0) { return $true }

    Write-CapTpError "the push failed. The log is COMMITTED LOCALLY and is here:"
    Write-CapTpError "  $LogPath"
    Write-CapTpError "  If that push fails on authentication: an ssh remote needs a key"
    Write-CapTpError "  'ssh-add -l' can list; an https remote needs GIT_TOKEN, or"
    Write-CapTpError "  GIT_TOKEN_FILE naming a file whose first line is the token."
    Write-CapTpError "  '.\start.ps1 --check' reports which credential is in force here."
    return $false
}

# =============================================================================
#  The RECEIVE half - what the loop polls and publishes
# =============================================================================
# `station/request` and `station/status`, and NOT the `agent/` spellings that
# transports/git.sh also reads.
#
# That compat path exists because a transport repo is a SEPARATE repo on a
# machine nobody here can reach, so it does not get upgraded when this one does
# and an operator who bootstrapped before the rename still has `agent/request`
# in their checkout. It cannot arise here: those paths are chosen by the CONTROL
# side, and the only binary that can plant a PowerShell station at all is one
# that postdates the rename. Carrying a second pair of paths into a new
# implementation would be dead code with a removal date, so it is said here
# instead.
# =============================================================================

$script:RequestPath = 'station/request'
$script:StatusPath = 'station/status'

function Receive-TpRequest {
    <#
      .SYNOPSIS
      The queued request document, '' when there is none, $null when the
      remote could not be reached.

      .DESCRIPTION
      READS THE WORKING TREE, having fetched. That is not an oversight and it is
      what transports/git.sh does: the fetch proves the remote is reachable, and
      Sync-TpSelf - which the loop calls immediately afterwards - is what brings
      the tree forward. A request therefore takes one poll to be seen, and the
      alternative is reading `origin/<branch>` here and running a step against a
      payload the station has not pulled yet.

      `$null` MEANS FAILED and '' means nothing is queued. The caller must test
      `$null -eq $body`: '' is falsy too, and an unreachable remote reported as
      an empty queue is a station that polls a dead link for ever calling itself
      idle.
    #>
    $null = Invoke-CapGit fetch --quiet origin $script:Branch
    if ($script:GitExit -ne 0) { return $null }
    $req = Join-Path $script:RepoRoot $script:RequestPath
    if (-not (Test-Path -LiteralPath $req -PathType Leaf)) { return '' }
    try {
        return [System.IO.File]::ReadAllText($req)
    } catch {
        Write-CapTpError "the request at $req is there and cannot be read: $($_.Exception.Message)"
        return $null
    }
}

function Receive-TpRequestLive {
    <#
      .SYNOPSIS
      The request as it is on the REMOTE right now. Used while a step runs.
      .DESCRIPTION
      NEVER PULLS AND NEVER REBASES. The step is appending to its log through an
      open descriptor; a rebase would rewrite that file underneath it and the
      appends would carry on at a stale offset, corrupting the evidence this
      read only meant to observe. `git show origin/<branch>:<path>` touches no
      working tree at all.
    #>
    $null = Invoke-CapGit fetch --quiet origin $script:Branch
    if ($script:GitExit -ne 0) { return $null }
    $body = Invoke-CapGit show "origin/$($script:Branch):$($script:RequestPath)"
    if ($script:GitExit -ne 0) { return '' }
    return $body
}

function Sync-TpSelf {
    <#
      .SYNOPSIS
      Bring a newer payload into the working tree.
      0 = something changed, 1 = nothing did, 2 = it could not be done.
      .DESCRIPTION
      A 2 is not fatal. A station that cannot update itself is still a working
      station, and the loop reports it and carries on rather than dying on a
      machine nobody can reach.
    #>
    $before = Invoke-CapGit rev-parse HEAD
    if ($script:GitExit -ne 0) { return 2 }
    $after = Invoke-CapGit rev-parse "origin/$($script:Branch)"
    if ($script:GitExit -ne 0) { return 2 }
    if ($before -ceq $after) { return 1 }

    $null = Invoke-CapGit pull --rebase --quiet
    if ($script:GitExit -eq 0) { return 0 }

    # A rebase left half-applied hands the operator a checkout mid-rebase with
    # no idea why, and every later poll fails before it starts.
    $null = Invoke-CapGit rebase --abort
    return 2
}

function Send-TpStatus {
    <#
      .SYNOPSIS
      Commit and push the status document. $true only when the far side has it.
      .DESCRIPTION
      AlsoFile is a partial log from a cancelled run, and it is not decoration.
      A killed step leaves its log modified in the working tree, and progress
      pushes have made that file TRACKED - so unless it goes in this commit,
      every later `pull --rebase` refuses on a dirty tree and the station wedges
      with the cancellation never reaching the far side. transports/git.sh
      carries the same argument, found by cancelling a run that had been
      publishing progress.
    #>
    param(
        [Parameter(Mandatory = $true)][string] $Body,
        [string] $Message = '',
        [string] $AlsoFile = ''
    )
    if (-not $Message) { $Message = 'station: status ***NO_CI***' }
    if ($AlsoFile -and -not (Test-Path -LiteralPath $AlsoFile -PathType Leaf)) { $AlsoFile = '' }
    if (-not (Write-GitStatusFile -Body $Body)) { return $false }

    $paths = @($script:StatusPath)
    if ($AlsoFile) { $paths += $AlsoFile }

    $null = Invoke-CapGit add -f -- @paths
    if ($script:GitExit -ne 0) {
        Write-CapTpError "could not stage $($script:StatusPath)"
        return $false
    }
    $null = Invoke-CapGit diff --cached --quiet -- @paths
    # Nothing staged: this exact status is already committed. Publishing the
    # same document twice is a no-op, not a failure.
    if ($script:GitExit -eq 0) { return $true }

    if (-not (Invoke-GitStatusCommit -Message $Message -Paths $paths)) { return $false }

    $null = Invoke-CapGit pull --rebase --quiet
    if ($script:GitExit -ne 0) { $null = Invoke-CapGit rebase --abort }

    return (Push-GitScope)
}

function Send-TpProgress {
    <#
      .SYNOPSIS
      Publish a snapshot of a running step's log.
      .DESCRIPTION
      PUSHES BUT NEVER PULLS OR REBASES, deliberately - see Receive-TpRequestLive
      for why a rebase underneath a running step corrupts the log it is
      observing. A rejected push is simply retried next cycle, and the final
      delivery reconciles properly.
    #>
    param(
        [Parameter(Mandatory = $true)][string] $Body,
        [string] $Message = '',
        [string] $LogPath = ''
    )
    if (-not $Message) { $Message = 'station: progress ***NO_CI***' }
    if (-not (Write-GitStatusFile -Body $Body)) { return $false }

    $paths = @($script:StatusPath)
    if ($LogPath -and (Test-Path -LiteralPath $LogPath -PathType Leaf)) { $paths += $LogPath }

    $null = Invoke-CapGit add -f -- @paths
    if ($script:GitExit -ne 0) { return $false }
    $null = Invoke-CapGit diff --cached --quiet -- @paths
    if ($script:GitExit -eq 0) { return $true }

    if (-not (Invoke-GitStatusCommit -Message $Message -Paths $paths)) { return $false }
    return (Push-GitScope)
}

function Write-GitStatusFile {
    param([string] $Body)
    $dst = Join-Path $script:RepoRoot $script:StatusPath
    try {
        $dir = Split-Path -Parent $dst
        if (-not (Test-Path -LiteralPath $dir)) {
            [void](New-Item -ItemType Directory -Force -Path $dir -ErrorAction Stop)
        }
        # WriteAllText, not Set-Content: 5.1's Set-Content writes a BOM, and the
        # control side anchors the first key of this document.
        [System.IO.File]::WriteAllText($dst, $Body)
        return $true
    } catch {
        Write-CapTpError "could not write $dst : $($_.Exception.Message)"
        return $false
    }
}

function Invoke-GitStatusCommit {
    param([string] $Message, [string[]] $Paths)
    $who = Get-CapUser
    $name = if ($env:GIT_AUTHOR_NAME) { $env:GIT_AUTHOR_NAME } else { $who }
    $mail = if ($env:GIT_AUTHOR_EMAIL) { $env:GIT_AUTHOR_EMAIL } else { "$who@localhost" }
    $null = Invoke-CapGit -c "user.name=$name" -c "user.email=$mail" commit -q -m $Message -- @Paths
    if ($script:GitExit -ne 0) {
        Write-CapTpError "could not commit the status document"
        return $false
    }
    return $true
}

function Push-GitScope {
    <#
      .SYNOPSIS
      Push to origin and THIS branch, named explicitly.
      .DESCRIPTION
      NOT a bare `git push`. Once the branch is which MACHINE this station
      answers for, resolving the destination through upstream configuration is
      too implicit: a branch tracking some other remote would take every status
      this station publishes somewhere the control side never reads, and report
      success. transports/git.sh publishes status the same way, and for the same
      reason.

      Send-TpLog does NOT do this, and that divergence is deliberate rather than
      an oversight - see the note above it.
    #>
    $null = Invoke-CapGit push --quiet origin "HEAD:$($script:Branch)"
    if ($script:GitExit -eq 0) { return $true }
    Write-CapTpError "could not push the status to origin/$($script:Branch)"
    return $false
}

function Test-TpPreflight {
    $out = @()
    $out += @{ Status = 'ok'; Label = 'branch'; Detail = $script:Branch }

    $url = (& git -C $script:RepoRoot remote get-url origin 2>$null | Select-Object -First 1)
    if ((Get-LastExit) -ne 0 -or -not $url) {
        $out += @{ Status = 'FAIL'; Label = 'remote'; Detail = "no remote named 'origin'. Git is the transport, so there is nowhere to push a log. Add one: git remote add origin <url>" }
        return $out
    }
    $out += @{ Status = 'ok'; Label = 'remote'; Detail = (Hide-GitCredential $url) }

    # WHICH CREDENTIAL, by source, and never the credential itself. An operator
    # debugging a refused push needs to know which of the three is in force -
    # that is usually the whole answer.
    if ($env:GIT_AUTH_HEADER) {
        $out += @{ Status = 'ok'; Label = 'credential'; Detail = 'GIT_AUTH_HEADER, used verbatim' }
    } elseif ($env:GIT_TOKEN) {
        $out += @{ Status = 'ok'; Label = 'credential'; Detail = "GIT_TOKEN as an HTTP basic header (user '$(if ($env:GIT_TOKEN_USER) { $env:GIT_TOKEN_USER } else { '<empty>' })')" }
    } elseif ($env:GIT_TOKEN_FILE) {
        if (Test-Path -LiteralPath $env:GIT_TOKEN_FILE -PathType Leaf) {
            $out += @{ Status = 'ok'; Label = 'credential'; Detail = "the first line of $($env:GIT_TOKEN_FILE)" }
        } else {
            $out += @{ Status = 'FAIL'; Label = 'credential'; Detail = "GIT_TOKEN_FILE names $($env:GIT_TOKEN_FILE), which is not a readable file. A mounted secrets DIRECTORY rather than the file inside it is the usual mistake" }
        }
    } else {
        $out += @{ Status = 'warn'; Label = 'credential'; Detail = 'none set. An ssh remote uses your agent; an https remote will be refused unless the host needs no credential' }
    }
    return $out
}

Export-ModuleMember -Function @(
    'Get-TpCapabilities',
    'Initialize-Tp',
    'Get-TpScope',
    'Get-TpRevision',
    'Get-TpDescribe',
    'Test-Tp',
    'Send-TpLog',
    'Test-TpPreflight',
    # The receive half.
    'Receive-TpRequest',
    'Receive-TpRequestLive',
    'Send-TpStatus',
    'Send-TpProgress',
    'Sync-TpSelf',
    'Hide-GitCredential',
    'Get-GitAuthHeader',
    'Get-LastExit',
    'Test-GitEnvConfig',
    # EXPORTED because it is this transport's single git entry point - the rule
    # this file is built on is that every git command the station issues is in
    # here, and the loop will need to issue some. It is also the only way to ask
    # BEHAVIOURALLY whether the credential reached git, rather than grepping the
    # source and matching a comment.
    'Invoke-CapGit'
)
