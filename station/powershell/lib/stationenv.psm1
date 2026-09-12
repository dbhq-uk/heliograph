# =============================================================================
#  stationenv.psm1 - the configuration a detached start has no other way to get
# =============================================================================
#  A SCHEDULED TASK INHERITS NOTHING from the shell that registered it. Without
#  this, a service-managed station starts with no TRANSPORT, no SHARE_DIR and no
#  credential - so it polls happily, captures a perfect log, and cannot deliver
#  it. The far side waits for hours and nothing reports a fault.
#
#  `service.ps1` writes the file. The bash payload solves the same problem with
#  `.station-env`, which is a bash file it sources; this one is `KEY=value`, one
#  per line, READ AND NEVER EXECUTED. Configuration must not be code - this file
#  lives in a directory the far side can write to on some transports.
#
#  IT LIVES IN A MODULE BECAUSE TWO ENTRY POINTS NEED IT, AND THE FIRST ONE IS
#  NOT THE LOOP. `start.ps1` preflights before it hands over to `station.ps1`,
#  so a reader that only `station.ps1` calls runs too late to be any use: the
#  preflight refuses on the account and on the transport, prints a table naming
#  neither the config file nor the task, and the loop it would have configured
#  never starts. CI found exactly that - a scheduled task installed with
#  `TRANSPORT=share ALLOW_ROOT=1` in its config file, refused for being an
#  Administrator running the `git` transport.
#
#  IT USES CMDLETS, NOT .NET. `[System.Environment]` and `[System.IO.File]` are
#  refused under Constrained Language Mode, and `start.ps1` reads this file
#  before it has reported what the language mode even is. The whole point of
#  that table is to name the policy plainly; it cannot do that if loading the
#  config throws first. Methods on a string are permitted under CLM, so the
#  parsing below is fine - only the two type literals had to go.
# =============================================================================
Set-StrictMode -Version 2.0

# Import-CapStationEnv <payload root>
#
# THE ENVIRONMENT WINS. A variable already set is left alone, so running the
# station by hand overrides whatever the service was installed with, and the
# operator debugging it does not have to find and edit a dotfile first. A task
# starts with a clean environment, so there the file always applies.
#
# The value is taken VERBATIM after the first `=`. No quoting, no unescaping,
# nothing to get wrong - a token containing a quote, a space or a backslash
# survives, which is the only property this format needs.
#
# Returns the names it set, so a caller can say what it read. Nothing is
# written to the output stream except that list.
function Import-CapStationEnv {
    param([Parameter(Mandatory = $true)][string] $Root)

    $file = Join-Path $Root '.station-env-ps'
    $applied = @()
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { return $applied }

    try {
        # -Encoding UTF8, EXPLICITLY. Windows PowerShell 5.1's Get-Content
        # defaults to the ANSI CODEPAGE, so a value holding any byte above 0x7F
        # comes back as Windows-1252 characters - a token with a non-ASCII
        # character in it authenticates as something else, and the failure is a
        # 401 on a machine nobody can log into. `service.ps1` writes this file
        # as UTF-8 without a BOM.
        #
        # Still a cmdlet rather than [System.IO.File], for the CLM reason at
        # the top of this file.
        foreach ($line in @(Get-Content -LiteralPath $file -Encoding UTF8 -ErrorAction Stop)) {
            if (-not $line -or $line.StartsWith('#')) { continue }
            $eq = $line.IndexOf('=')
            if ($eq -lt 1) { continue }
            $name = $line.Substring(0, $eq)
            # `Test-Path Env:\X` rather than a lookup, so a variable set to the
            # empty string still counts as set. An operator who exports
            # GIT_TOKEN= to unset it means it.
            if (Test-Path -LiteralPath "Env:\$name") { continue }
            Set-Item -LiteralPath "Env:\$name" -Value $line.Substring($eq + 1)
            $applied += $name
        }
    } catch {
        # NOT FATAL, AND SAID. A station that cannot read its own configuration
        # fails a moment later with a better message than anything this line
        # could produce - but silence here would make that message look like a
        # transport fault rather than a permissions problem on one file.
        Write-Error "could not read $file : $($_.Exception.Message)" -ErrorAction Continue
    }
    return $applied
}

# Where the file is, whether or not it exists. `service.ps1` and the preflight
# both name it to the operator, and they must name the same path.
function Get-CapStationEnvPath {
    param([Parameter(Mandatory = $true)][string] $Root)
    return (Join-Path $Root '.station-env-ps')
}

Export-ModuleMember -Function @(
    'Import-CapStationEnv',
    'Get-CapStationEnvPath'
)
