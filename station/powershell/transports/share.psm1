# =============================================================================
#  transports/share.psm1 - a directory both sides can see
# =============================================================================
# The cheapest transport there is, and it covers a real population. An estate
# that will not open an egress path, will not provision a storage account and
# will not permit a git host will quite often already have a share that both
# machines mount, because that is how everything else in the estate moves files.
#
# It needs no credential of its own: THE MOUNT IS THE CREDENTIAL, which is also
# its whole security model and worth being plain about. Anyone who can write to
# the share can queue a request, so the share must be as tightly scoped as the
# account the station runs as.
#
#   <SHARE_DIR>\<SHARE_SCOPE>\request          the control side writes it
#   <SHARE_DIR>\<SHARE_SCOPE>\status           this side writes it
#   <SHARE_DIR>\<SHARE_SCOPE>\ops-logs\*.txt   this side writes them
#
# The twin of transports/share.sh, and the layout is identical on purpose: a
# bash control node and a PowerShell station have to meet in the same directory.
# =============================================================================

Set-StrictMode -Version 2.0

$script:ShareDir = ''
$script:ShareScope = ''
$script:ShareBase = ''

# `heliograph-tmp-` rather than a bare `.tmp`: a half-written file in a shared
# directory is something a human will find, and the name should say whose it is
# and that it is disposable.
$script:TmpPrefix = 'heliograph-tmp-'

function Get-TpCapabilities { return 'request status progress live' }

function Initialize-Tp {
    if (-not (Test-TpNeed -Name 'SHARE_DIR' -Why 'the directory both sides mount')) { return $false }
    if (-not (Test-TpNeed -Name 'SHARE_SCOPE' -Why 'the scope, which is what a run is bound to - one directory per investigation, so two do not overwrite each other')) { return $false }

    $script:ShareDir = $env:SHARE_DIR
    $script:ShareScope = $env:SHARE_SCOPE

    # A WHITELIST, AND THE REASON IS NOT PATH TRAVERSAL ALONE.
    #
    # Refusing separators covers the obvious case. It does not cover the one
    # that bites: the scope is published as the status document's `branch:`
    # value, and that document is line-oriented `key: value`. A scope holding a
    # NEWLINE injects a second key, and the parser keeps the last - so a scope
    # of "x`nstate: running" makes an idle station report itself busy for ever.
    #
    # transports/share.sh and internal/transport/share.go refuse exactly this
    # set. Three copies of one rule, because none of them may assume another
    # configured it.
    if ($script:ShareScope -cnotmatch '^[A-Za-z0-9._-]+$' -or
        $script:ShareScope -eq '.' -or $script:ShareScope -eq '..' -or
        $script:ShareScope.StartsWith('-')) {
        Write-CapTpError "'$($script:ShareScope)' is not a usable SHARE_SCOPE. It becomes a directory name AND the status document's branch field, so it may hold only letters, digits, dot, hyphen and underscore, and may not begin with a hyphen"
        return $false
    }

    if (-not (Test-Path -LiteralPath $script:ShareDir -PathType Container)) {
        Write-CapTpError "the share at $($script:ShareDir) is not there. Mount it, or point SHARE_DIR at where it is mounted on THIS machine - the two sides need not agree on the path, only on the directory"
        return $false
    }

    # A LINKED SCOPE **OR** ops-logs IS REFUSED. The share is the security
    # boundary, and a link inside it points somewhere the mount's permissions
    # do not describe.
    #
    # BOTH COMPONENTS, because only checking the scope leaves the one that
    # actually receives logs unchecked: a linked `ops-logs` sends every
    # delivery somewhere the control side never reads, and Send-TpLog returns
    # true having written it there. transports/share.sh refuses either.
    #
    # ReparsePoint as well as LinkType, because on Windows a junction and a
    # mount point are neither symlinks nor hard links and LinkType alone does
    # not always name them.
    $scopePath = Join-Path $script:ShareDir $script:ShareScope
    foreach ($p in $scopePath, (Join-Path $scopePath 'ops-logs')) {
        if (-not (Test-Path -LiteralPath $p)) { continue }
        $item = Get-Item -LiteralPath $p -Force
        $isLink = $false
        if ($item.LinkType) { $isLink = $true }
        if ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) { $isLink = $true }
        if ($isLink) {
            Write-CapTpError "$p is a link or reparse point, and the share is the security boundary - it points somewhere the mount's permissions do not describe, so a delivery would land where the control side never reads"
            return $false
        }
    }

    $script:ShareBase = $scopePath

    # NOTHING IS CREATED HERE. `--check` promises to change nothing, and it
    # calls this. The publish path creates what it needs when it needs it.
    return $true
}

function Get-TpScope { return $script:ShareScope }
function Get-TpRevision { return "share $($script:ShareDir), scope $($script:ShareScope)" }

function Get-TpDescribe {
    # NO CREDENTIAL IN THE ANSWER, because both callers print this - the
    # preflight table and the station's startup banner, which goes to a log.
    # There is no credential here to leak, and saying so is the point: an
    # operator reading the table should learn that the mount IS the credential.
    return "file share $($script:ShareDir), scope $($script:ShareScope), credential: the mount itself"
}

# --- writing, atomically ------------------------------------------------------
# WRITE-THEN-RENAME, because a control side polling this directory can read it
# at any instant. A partially written status is a status with no `state:` yet,
# or worse a truncated one, and the reader would act on it.
function New-ShareTemp {
    param([Parameter(Mandatory = $true)][string] $Destination)
    $dir = Split-Path -Parent $Destination
    if (-not (Test-Path -LiteralPath $dir)) {
        [void](New-Item -ItemType Directory -Force -Path $dir -ErrorAction Stop)
    }
    $leaf = Split-Path -Leaf $Destination
    return (Join-Path $dir ($script:TmpPrefix + $leaf + '.' + [guid]::NewGuid().ToString('N')))
}

function Move-ShareInto {
    param([string] $Temp, [string] $Destination)
    # A RENAME ONTO AN EXISTING DIRECTORY puts the file INSIDE it on some
    # platforms and fails on others. Refused explicitly, or the station reports
    # a delivery the control side cannot see. transports/share.sh refuses the
    # same case for the same reason.
    if (Test-Path -LiteralPath $Destination -PathType Container) {
        Write-CapTpError "$Destination is a directory, so publishing there would put the file inside it"
        Remove-Item -LiteralPath $Temp -Force -ErrorAction SilentlyContinue
        return $false
    }
    try {
        # -Force, so a re-published status replaces the old one rather than
        # failing. Move-Item is a rename within a filesystem, which is atomic.
        Move-Item -LiteralPath $Temp -Destination $Destination -Force -ErrorAction Stop
        return $true
    } catch {
        Write-CapTpError "could not publish to $Destination : $($_.Exception.Message)"
        Remove-Item -LiteralPath $Temp -Force -ErrorAction SilentlyContinue
        return $false
    }
}

function Publish-ShareFile {
    param([string] $Source, [string] $Destination)
    try {
        $tmp = New-ShareTemp -Destination $Destination
        # Copy-CapSharedFile, NOT Copy-Item. This publishes a PARTIAL log while
        # the capture still holds it open for writing, and on Windows Copy-Item
        # opens with FileShare.Read - which does not tolerate an existing
        # writer, so every progress publication threw a sharing violation. On
        # Linux nothing enforces that and it worked perfectly.
        [void](Copy-CapSharedFile -Source $Source -Destination $tmp)
        return (Move-ShareInto -Temp $tmp -Destination $Destination)
    } catch {
        Write-CapTpError "could not stage $Source for $Destination : $($_.Exception.Message)"
        return $false
    }
}

function Publish-ShareBody {
    <#
      .SYNOPSIS
      Write text into place atomically. The status document's path.
      .DESCRIPTION
      `File.WriteAllText` rather than Set-Content, for the encoding: PowerShell
      5.1's Set-Content writes a BOM, and the control side parses this file as
      line-oriented `key: value` with the first key anchored. Three invisible
      bytes in front of `state:` and the first key is not found.

      NO TRAILING NEWLINE IS ADDED. The caller composes the whole document and
      transports/share.sh publishes it with `printf '%s'`, which adds none - so
      a byte-for-byte comparison of the two implementations' output is a
      meaningful test rather than one that has to normalise first.
    #>
    param([string] $Body, [string] $Destination)
    try {
        $tmp = New-ShareTemp -Destination $Destination
        [System.IO.File]::WriteAllText($tmp, $Body)
        return (Move-ShareInto -Temp $tmp -Destination $Destination)
    } catch {
        Write-CapTpError "could not stage a document for $Destination : $($_.Exception.Message)"
        return $false
    }
}

function Test-Tp {
    <#
      .SYNOPSIS
      Prove the write, and prove the WHOLE write.

      .DESCRIPTION
      A share mounted read-only, and one whose server has gone away leaving a
      stale handle, both stat perfectly and fail on the first write - which
      would be the log, an hour later, with nobody left to tell.

      CREATE, RENAME, THEN REMOVE, because publishing needs all three and an ACL
      can grant them separately. An SMB share that permits create and write but
      denies rename or delete - an ordinary way to configure a drop box - passes
      a check that only writes a file, and then fails on every publication at
      the rename.

      IT CREATES NO DIRECTORY: it probes the deepest one that already exists,
      because `--check` promises to change nothing.
    #>
    # THE DEEPEST DIRECTORY THAT ALREADY EXISTS, and ops-logs is deeper than
    # the scope. Probing the scope when `ops-logs` exists with a stricter ACL
    # approves a station that can write a status and cannot deliver a single
    # log - which is the one thing this check is for.
    #
    # IT CREATES NOTHING: `--check` promises that, so it probes what is there
    # rather than making the directory in order to test it.
    $dir = $script:ShareDir
    $what = 'the share root'
    if (Test-Path -LiteralPath $script:ShareBase -PathType Container) {
        $dir = $script:ShareBase
        $what = 'the scope directory'
    }
    $logs = Join-Path $script:ShareBase 'ops-logs'
    if (Test-Path -LiteralPath $logs -PathType Container) {
        $dir = $logs
        $what = 'the ops-logs directory, which is where a log actually lands'
    }

    $tmp = Join-Path $dir ($script:TmpPrefix + 'check.' + [guid]::NewGuid().ToString('N'))
    $dst = Join-Path $dir ('.heliograph-write-check.' + [guid]::NewGuid().ToString('N'))
    try {
        [System.IO.File]::WriteAllText($tmp, "heliograph write check`n")
        Move-Item -LiteralPath $tmp -Destination $dst -ErrorAction Stop
        Remove-Item -LiteralPath $dst -Force -ErrorAction Stop
        return $true
    } catch {
        Write-CapTpError "cannot create, rename and remove a file in $what ($dir): $($_.Exception.Message)"
        Write-CapTpError "  A station needs all three: it writes a temporary, renames it into place, and tidies up after itself."
        foreach ($p in $tmp, $dst) {
            if (Test-Path -LiteralPath $p) { Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue }
        }
        return $false
    }
}

function Send-TpLog {
    <#
      .SYNOPSIS
      Deliver the finished log. $true only when it is actually there.
      .DESCRIPTION
      The message argument is ignored: it is a git commit subject, and there is
      no history here to carry it. The signature matches the other transports so
      the caller does not branch.
    #>
    param(
        [Parameter(Mandatory = $true)][string] $LogPath,
        [string] $Message = ''
    )
    if (-not (Test-Path -LiteralPath $LogPath -PathType Leaf)) {
        Write-CapTpError "there is no log at $LogPath to deliver"
        return $false
    }
    $name = Split-Path -Leaf $LogPath
    return (Publish-ShareFile -Source $LogPath `
                              -Destination (Join-Path (Join-Path $script:ShareBase 'ops-logs') $name))
}

# =============================================================================
#  The RECEIVE half - what the loop polls and publishes
# =============================================================================
# `$null` MEANS FAILED. An empty string means "there is nothing queued", which
# is the ordinary state of a station that has just started.
#
# Collapsing the two is how a station goes permanently deaf without anybody
# being told: a share that can no longer be read - a permissions change, a
# stale SMB handle, an I/O error, or `request` having somehow become a
# directory - would be reported as an empty queue, and the station would poll it
# for ever reporting itself idle. transports/share.sh separates them by exit
# status for the same reason; here it is $null against ''.
#
# THE CALLER MUST TEST `$null -eq $body`, not `-not $body`, because '' is falsy
# too and the difference is the whole point. station.ps1 does.
# =============================================================================

function Receive-TpRequest {
    <#
      .SYNOPSIS
      The queued request document, '' when there is none, $null when the share
      could not be read.
    #>
    $req = Join-Path $script:ShareBase 'request'
    if (-not (Test-Path -LiteralPath $req)) { return '' }
    if (Test-Path -LiteralPath $req -PathType Container) {
        Write-CapTpError "$req is a directory, not a request. Nothing can be read from it, and this station will not run anything until that is fixed"
        return $null
    }
    try {
        # ReadAllText, NOT Get-Content: Get-Content returns an ARRAY of lines
        # and the caller wants a document. Worse, a one-line request would come
        # back as a bare string and a two-line one as an array, so the caller's
        # handling of it would differ by the number of lines in the file.
        return [System.IO.File]::ReadAllText($req)
    } catch {
        Write-CapTpError "the request at $req is there and cannot be read: $($_.Exception.Message)"
        return $null
    }
}

# The same read. There is no working tree here and nothing to disturb, so the
# live verb is the ordinary verb - which is what makes `cancel` reach a running
# step within one poll on a share.
function Receive-TpRequestLive { return (Receive-TpRequest) }

function Send-TpStatus {
    <#
      .SYNOPSIS
      Publish the status document. $true only when it landed.
      .DESCRIPTION
      AlsoFile is a partial log from a cancelled run. On git it is committed
      alongside the status so the cancellation is not stranded behind a dirty
      tree; here it is simply the last thing the far side will ever see of that
      run, and losing it is losing the only evidence there is.
    #>
    param(
        [Parameter(Mandatory = $true)][string] $Body,
        [string] $Message = '',
        [string] $AlsoFile = ''
    )
    if (-not (Publish-ShareBody -Body $Body -Destination (Join-Path $script:ShareBase 'status'))) {
        return $false
    }
    if ($AlsoFile -and (Test-Path -LiteralPath $AlsoFile -PathType Leaf)) {
        return (Publish-ShareFile -Source $AlsoFile `
                    -Destination (Join-Path (Join-Path $script:ShareBase 'ops-logs') (Split-Path -Leaf $AlsoFile)))
    }
    return $true
}

function Send-TpProgress {
    <#
      .SYNOPSIS
      Publish a snapshot of a running step's log, under its FINAL name.
      .DESCRIPTION
      The name is deliberate. The blob transport puts progress under a fixed
      `log` object because there the two are different objects; here the control
      side lists one directory and reads by name, so publishing progress under
      the final name means a reader following a long step watches it grow and
      then be completed in place - rather than finding a stale snapshot beside
      the real thing and having to work out which is which.
    #>
    param(
        [Parameter(Mandatory = $true)][string] $Body,
        [string] $Message = '',
        [string] $LogPath = ''
    )
    if (-not (Publish-ShareBody -Body $Body -Destination (Join-Path $script:ShareBase 'status'))) {
        return $false
    }
    if (-not $LogPath -or -not (Test-Path -LiteralPath $LogPath -PathType Leaf)) { return $true }
    return (Publish-ShareFile -Source $LogPath `
                -Destination (Join-Path (Join-Path $script:ShareBase 'ops-logs') (Split-Path -Leaf $LogPath)))
}

# Cannot self-update: nothing publishes a payload to a share, so `self` is
# absent from Get-TpCapabilities and the loop never calls this. Defined anyway,
# so that calling it would be a refusal rather than "the term is not recognised"
# - which would read as a broken payload rather than as a transport saying no.
#
# 0 = something changed, 1 = nothing to do, 2 = it could not be done. The same
# three the bash loop reads.
function Sync-TpSelf { return 1 }

function Test-TpPreflight {
    <#
      .SYNOPSIS
      The lines this transport contributes to the preflight table.
      .DESCRIPTION
      Returns an array of @{ Status; Label; Detail }. The caller prints them, so
      a transport says what is wrong with ITSELF rather than the preflight
      guessing on its behalf.
    #>
    $out = @()
    $out += @{ Status = 'ok'; Label = 'share'; Detail = "$($script:ShareDir), scope $($script:ShareScope)" }
    if (Test-Path -LiteralPath $script:ShareBase -PathType Container) {
        $out += @{ Status = 'ok'; Label = 'scope'; Detail = "$($script:ShareBase) exists" }
    } else {
        # NOT A FAILURE. A typo in SHARE_SCOPE is indistinguishable from a first
        # run, and the publish path creates it - so this is a fact worth knowing
        # rather than a refusal. A station that refused here could never start
        # on a fresh share.
        $out += @{ Status = 'warn'; Label = 'scope'; Detail = "$($script:ShareBase) does not exist yet. The first delivery creates it - but if that is a typo, the far side will be watching a directory nothing ever writes to" }
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
    'Sync-TpSelf'
)
