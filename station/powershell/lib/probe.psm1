# =============================================================================
#  lib/probe.psm1 - helpers for STEP SCRIPTS
# =============================================================================
# Step scripts print to stdout and nothing else. The runner owns the log file,
# the timestamps and the delivery. That separation is what lets you run any step
# standalone while you are developing it:
#
#     .\steps\env-snapshot.ps1     # straight to the terminal, no log
#     .\run.ps1 env                # same thing, captured
#
# The PowerShell twin of lib/probe.sh, and the two rules it exists to enforce
# are the same:
#
#   A PROBE NEVER ABORTS THE STEP. A diagnostic wants the whole picture, not
#   just the first thing that breaks. Every failure is recorded and printed and
#   the step carries on - so `$ErrorActionPreference = 'Stop'` has no business
#   in a step script.
#
#   A PROBE NEVER TRUNCATES ITS OWN OUTPUT. What a probe prints IS the evidence,
#   so a `| Select-Object -First 20` in one cuts the line somebody needed, and
#   the loss is invisible in the log. CI greps the bash steps for exactly that
#   and this file is why the same rule can be stated for PowerShell.
# =============================================================================

Set-StrictMode -Version 2.0

$script:ProbeFailures = 0
$script:ProbeTotal = 0

function Write-ProbeSection {
    param([Parameter(Mandatory = $true)][string] $Title)
    Write-Output ''
    Write-Output "---------- $Title ----------"
}

function Invoke-Probe {
    <#
      .SYNOPSIS
      Run a scriptblock, print everything it produces, record a failure without
      aborting.

      .DESCRIPTION
      Output is written through as it is produced. Nothing here collects the
      stream: the capture upstream stamps each line as it arrives, and a probe
      that buffered would hand it a block of lines all at once - which is the
      one defect the whole capture exists to prevent, introduced downstream of
      every guard against it.
    #>
    param(
        [Parameter(Mandatory = $true)][string] $Label,
        [Parameter(Mandatory = $true)][scriptblock] $Body,
        [switch] $Optional
    )

    $script:ProbeTotal++
    Write-ProbeSection $Label

    # RESET FIRST. $LASTEXITCODE is set only by a NATIVE call and is left
    # untouched by anything else, so a probe that runs no native command
    # inherits whatever the PREVIOUS one returned - and reports a failure it
    # had nothing to do with. Two probes in the first version of this file did
    # exactly that: they printed their answer and were marked FAILED because a
    # `git` three probes earlier had exited 1.
    #
    # This is the same trap caplib.psm1 documents and avoids by reading the
    # Process object instead. Here there is no process to read, so it is reset
    # instead - and a body that runs nothing native leaves it at 0, which is
    # the truth.
    $global:LASTEXITCODE = 0
    $failed = $false
    $code = 0
    try {
        # 2>&1 so a command's own diagnosis reaches the log next to its output.
        # A probe that keeps stdout and loses stderr keeps the symptom and
        # discards the reason.
        & $Body 2>&1 | ForEach-Object { Write-Output $_ }
        if ($null -ne $global:LASTEXITCODE -and $global:LASTEXITCODE -ne 0) {
            $failed = $true
            $code = $global:LASTEXITCODE
        }
    } catch {
        $failed = $true
        $code = 1
        Write-Output $_.Exception.Message
    }

    if ($failed) {
        if ($Optional) {
            Write-Output "   ^ absent or unavailable  ($Label)"
        } else {
            $script:ProbeFailures++
            Write-Output "   ^ FAILED: exit $code  ($Label)"
        }
    }
}

function Invoke-ProbeOptional {
    param(
        [Parameter(Mandatory = $true)][string] $Label,
        [Parameter(Mandatory = $true)][scriptblock] $Body
    )
    Invoke-Probe -Label $Label -Body $Body -Optional
}

function Write-ProbeSummary {
    <#
      .SYNOPSIS
      Print the tally. Prints only - it deliberately returns nothing.

      .DESCRIPTION
      EVERYTHING A POWERSHELL FUNCTION WRITES TO THE OUTPUT STREAM IS ITS
      RETURN VALUE, and that is the trap this signature exists to avoid.

      The first version returned the exit code and printed the tally with
      Write-Output, so a step ending `exit (Write-ProbeSummary)` captured BOTH
      into the parentheses: the summary never reached the log, and `exit` was
      handed an array. The log ended with the last probe and no tally at all,
      which reads exactly like a step that was cut off.

      So printing and the exit code are two functions. A caller cannot get one
      by accident while meaning the other.
    #>
    Write-ProbeSection 'summary'
    Write-Output "$script:ProbeTotal probes, $script:ProbeFailures failed"
}

function Get-ProbeExitCode {
    <#
      .SYNOPSIS
      The exit code the step should use, and nothing else on the output stream.
      .DESCRIPTION
      Non-zero when a REQUIRED probe failed, so a reader who only sees the
      footer still learns that something was wrong. Optional probes never
      count: "this box has no systemd" is an answer, not a failure.
    #>
    if ($script:ProbeFailures -gt 0) { return 1 }
    return 0
}

Export-ModuleMember -Function @(
    'Write-ProbeSection',
    'Invoke-Probe',
    'Invoke-ProbeOptional',
    'Write-ProbeSummary',
    'Get-ProbeExitCode'
)
