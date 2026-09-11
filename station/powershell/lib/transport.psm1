# =============================================================================
#  lib/transport.psm1 - pick a transport, or say why not
# =============================================================================
# The station-side half of a channel. `TRANSPORT` names one, this loads it, and
# everything above talks to the same four functions whichever it is.
#
# THE WHOLE CONTRACT, both halves:
#
#   the channel
#     Get-TpCapabilities     which of the verbs below this channel has
#     Initialize-Tp          the variables it needs, checked and named
#     Test-Tp                it is reachable and the credential is accepted
#     Get-TpScope            what a run is bound to: a branch, a directory
#     Get-TpRevision         which payload this is
#     Get-TpDescribe         what it is, with NO credential in the answer
#     Test-TpPreflight       the lines it contributes to the preflight table
#
#   sending - run.ps1's half
#     Send-TpLog             a finished log reaches the far side
#
#   receiving - station.ps1's half
#     Receive-TpRequest      the queued request, or '' , or $null for failed
#     Receive-TpRequestLive  the same, read without disturbing a running step
#     Send-TpStatus          what this station is doing, published
#     Send-TpProgress        a snapshot of a log while the step is still running
#     Sync-TpSelf            bring a newer payload in: 0 changed, 1 no, 2 failed
#
# A CAPABILITY IS A PROMISE THE LOOP HOLDS THE TRANSPORT TO. `self` and `live`
# are both optional and the loop asks before it calls: a share cannot
# self-update because nothing publishes a payload to a share, and declaring a
# verb with no implementation behind it is worse than not having it - the loop
# calls what is declared.
#
# THE NAME IS VALIDATED BEFORE IT BECOMES A PATH. It selects a file that gets
# imported, so a name from a config file is a name that chooses code to run.
# =============================================================================

Set-StrictMode -Version 2.0

$script:TpModule = $null
$script:TpName = ''

function Get-TpNames {
    <#
      .SYNOPSIS
      Every transport this payload actually ships, read from disk.
      .DESCRIPTION
      Listed rather than hardcoded, so a payload that is missing one says so
      instead of offering it and failing on the import.
    #>
    $dir = Join-Path (Split-Path -Parent $PSScriptRoot) 'transports'
    if (-not (Test-Path -LiteralPath $dir -PathType Container)) { return @() }
    return @(Get-ChildItem -LiteralPath $dir -Filter '*.psm1' -File |
            ForEach-Object { $_.BaseName } | Sort-Object)
}

function Import-Tp {
    <#
      .SYNOPSIS
      Load the transport named by $Name (default: $env:TRANSPORT, then git).
      Returns $true when it loaded AND initialised.
    #>
    param([string] $Name = '')

    if (-not $Name) { $Name = $env:TRANSPORT }
    if (-not $Name) { $Name = 'git' }

    # A WHITELIST, because this name becomes a filename that gets IMPORTED.
    # `../../evil` or an absolute path would run somebody else's code with the
    # station's credentials, and the name can come from a .station-env that the
    # far side may have written. Lowercase letters, digits and hyphen is the
    # same set transports/ is allowed to contain.
    if ($Name -cnotmatch '^[a-z0-9-]+$') {
        Write-CapTpError "'$Name' is not a usable transport name. It becomes a filename that gets imported, so only lowercase letters, digits and hyphens are accepted. Set TRANSPORT to one of: $((Get-TpNames) -join ', ')"
        return $false
    }

    $dir = Join-Path (Split-Path -Parent $PSScriptRoot) 'transports'
    $path = Join-Path $dir "$Name.psm1"
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        Write-CapTpError "no transport named '$Name' in $dir. This payload ships: $((Get-TpNames) -join ', '). Set TRANSPORT to one of those, or re-run the bootstrap if the directory is missing entirely"
        return $false
    }

    try {
        # -Global, because Import-Module inside a module function imports into
        # THAT MODULE'S session state and nowhere else. Without it the transport
        # loaded, initialised and reported success, and every one of its
        # functions was invisible to the caller - which reads as "the transport
        # is fine, and Send-TpLog does not exist".
        $script:TpModule = Import-Module $path -Force -Global -PassThru -ErrorAction Stop
        $script:TpName = $Name
    } catch {
        Write-CapTpError "the '$Name' transport would not load: $($_.Exception.Message)"
        return $false
    }

    if (-not (Initialize-Tp)) { return $false }
    return $true
}

function Get-TpName { return $script:TpName }

function Test-TpCapability {
    <#
      .SYNOPSIS
      $true when the loaded transport declares the named capability.

      .DESCRIPTION
      READ ONCE, AT START, by the caller - not per poll. A station that cannot
      self-update is a working station; one that finds that out when an update
      is needed, with nobody on this side to tell, has cost a round trip.

      The match is on WHOLE WORDS, which is why the haystack and the needle are
      both padded with spaces. `history` contains `stor`... nothing, but
      `request` is a substring of nothing here only by luck, and a capability
      list is exactly the kind of string that grows a member which contains
      another. transports/git.sh pads the same way for the same reason.
    #>
    param([Parameter(Mandatory = $true)][string] $Name)
    $caps = ''
    try { $caps = Get-TpCapabilities } catch { return $false }
    if (-not $caps) { return $false }
    return (" $caps ").Contains(" $Name ")
}

function Write-CapTpError {
    param([string] $Text)
    [Console]::Error.WriteLine("station: $Text")
}

# --- cap_need, in PowerShell --------------------------------------------------
# A transport says what it needs and WHY, once, in one place. The bash side
# learned this the hard way: `${VAR:?}` exits the whole shell rather than
# failing the function, so a missing variable killed the station instead of
# being reported through the preflight table.
function Test-TpNeed {
    <#
      .SYNOPSIS
      $true when the named environment variable is set. Otherwise reports what
      it is for and returns $false.
    #>
    param(
        [Parameter(Mandatory = $true)][string] $Name,
        [Parameter(Mandatory = $true)][string] $Why
    )
    $v = [System.Environment]::GetEnvironmentVariable($Name)
    if ($v) { return $true }
    Write-CapTpError "$Name is not set, and the '$($script:TpName)' transport needs it: $Why"
    return $false
}

Export-ModuleMember -Function @(
    'Get-TpNames',
    'Import-Tp',
    'Get-TpName',
    'Test-TpCapability',
    'Test-TpNeed',
    'Write-CapTpError'
)
