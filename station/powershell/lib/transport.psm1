# =============================================================================
#  lib/transport.psm1 - pick a transport, or say why not
# =============================================================================
# The station-side half of a channel. `TRANSPORT` names one, this loads it, and
# everything above talks to the same four functions whichever it is.
#
# WHAT IS HERE IS DELIVERY, AND ONLY DELIVERY:
#
#     Initialize-Tp     the variables this channel needs, checked and named
#     Test-Tp           it is reachable and the credential is accepted
#     Get-TpDescribe    what it is, with no credential in the answer
#     Send-TpLog        a finished log reaches the far side
#
# The RECEIVE half - fetching a request, publishing a status - is not here, and
# that is the same call as leaving the loop out of the last PR. Nothing can poll
# yet, so a `Receive-TpRequest` written now could not be tested end to end, and
# an untested request path is the one that decides what runs on somebody's
# machine. It arrives with the loop that can prove it.
#
# So a PowerShell station today can DELIVER and cannot RECEIVE. `run.ps1 <step>`
# captures and ships; a request still has to be handed over by whoever is
# driving. The preflight says exactly that rather than implying a round trip.
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
    'Test-TpNeed',
    'Write-CapTpError'
)
