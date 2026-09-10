# =============================================================================
#  steps/env-snapshot.ps1 - what is this control node, really?
# =============================================================================
# heliograph-mode: read-only
# The correct first step of ANY investigation on a box you cannot log into. It
# answers the questions that otherwise get assumed and turn out to be wrong:
# which host, which user, which PowerShell, is there a proxy in the way, what
# does DNS look like, and - the two that decide whether a station can run here
# at all - what is the LANGUAGE MODE and what execution policy is in force.
#
# Read-only. Never prompts. Safe to run repeatedly.
#
#     .\run.ps1 env                  # captured
#     .\steps\env-snapshot.ps1       # straight to the terminal
# =============================================================================
Set-StrictMode -Version 2.0

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here '../lib/probe.psm1') -Force

Write-Output 'ENV SNAPSHOT - control node baseline'

# --- identity ----------------------------------------------------------------
Invoke-Probe 'host' {
    [System.Net.Dns]::GetHostEntry([string]::Empty).HostName
}
Invoke-Probe 'os' {
    [System.Environment]::OSVersion.VersionString
    [System.Environment]::Is64BitOperatingSystem
    if (Get-Command Get-CimInstance -ErrorAction SilentlyContinue) {
        # THE WHOLE OBJECT, formatted as a list. Format-Table would truncate
        # every value to the terminal width and the build number is what
        # identifies a patched image - which is the entire reason for asking.
        Get-CimInstance Win32_OperatingSystem -ErrorAction SilentlyContinue |
            Format-List Caption, Version, BuildNumber, OSArchitecture, InstallDate
    }
}
Invoke-Probe 'clock (UTC - check for skew against your own)' {
    [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
}
# WindowsIdentity IS WINDOWS-ONLY and throws "Windows Principal functionality
# is not supported on this platform" everywhere else - which this step reported
# as a failed probe on Linux until it was run there. A step that fails on the
# platform it is being developed on is a step nobody trusts.
Invoke-Probe 'user' {
    "name : " + [System.Environment]::UserName
    if ([System.Environment]::OSVersion.Platform -eq [System.PlatformID]::Win32NT) {
        $id = [System.Security.Principal.WindowsIdentity]::GetCurrent()
        "full : $($id.Name)"
        if ($id.User) { "sid  : $($id.User.Value)" }
        $p = New-Object System.Security.Principal.WindowsPrincipal($id)
        "admin: " + $p.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)
    } else {
        & id 2>&1
    }
}

# --- the two that decide whether a station can run here at all ---------------
# CONSTRAINED LANGUAGE MODE is the one that stops a PowerShell station dead.
# Under it, .NET method calls and type literals are refused - which is most of
# caplib.psm1 - and the failure reads as a syntax error in somebody else's file
# rather than as a policy. Asking here, in the first step, is what turns an
# afternoon into a line in a log.
Invoke-Probe 'language mode' {
    $mode = $ExecutionContext.SessionState.LanguageMode
    "LanguageMode: $mode"
    if ("$mode" -ne 'FullLanguage') {
        'WARNING: not FullLanguage. A PowerShell station cannot capture under'
        '         ConstrainedLanguage: .NET method calls are refused, and that'
        '         is most of caplib.psm1. Use the bash station, or have the'
        '         policy relaxed for this account.'
    }
}
# A GPO-set execution policy OVERRIDES -ExecutionPolicy Bypass, so a station
# that launches fine by hand can refuse to launch from a scheduled task. The
# per-scope list is the answer; the effective value alone does not say who set
# it and therefore does not say whether it can be changed.
Invoke-Probe 'execution policy, by scope' {
    Get-ExecutionPolicy -List | Format-Table -AutoSize | Out-String -Width 200
}

Invoke-Probe 'powershell' {
    $PSVersionTable | Format-List | Out-String -Width 200
}

# --- resources ---------------------------------------------------------------
Invoke-ProbeOptional 'disk' {
    Get-PSDrive -PSProvider FileSystem |
        Select-Object Name, @{n = 'UsedGB'; e = { [math]::Round($_.Used / 1GB, 1) } },
                            @{n = 'FreeGB'; e = { [math]::Round($_.Free / 1GB, 1) } }, Root |
        Format-Table -AutoSize | Out-String -Width 200
}

# --- tooling -----------------------------------------------------------------
# Versions, not just presence. A floated tool version is a recurring cause of
# "it works here and not there", and the version is the only thing that settles
# it. Optional, because a station on the share or relay transport needs no git.
foreach ($tool in 'git', 'curl', 'ssh', 'python3', 'dotnet') {
    Invoke-ProbeOptional "tool: $tool" {
        # -CommandType Application, AND the resolved path is what gets run.
        #
        # Windows PowerShell 5.1 defines `curl` as an ALIAS for
        # Invoke-WebRequest, so a bare `Get-Command curl` finds the cmdlet and
        # `& curl --version` calls it - which fails with a parameter binding
        # error and reports "curl is broken here" about a curl.exe that may not
        # even be installed. `wget` and `ls` are the same story.
        # Indexed, rather than the cmdlet that takes the first of a pipeline.
        # CI refuses that cmdlet in a step outright and is right to: telling
        # "pick one command" from "cut the evidence" needs a reader, and a gate
        # that needs a reader is a gate that gets argued with. The comment
        # avoids the literal too, or the gate would fire on this explanation.
        $found = @(Get-Command $tool -CommandType Application -ErrorAction SilentlyContinue)
        if ($found.Count -eq 0) { throw "not on PATH as an executable" }
        $c = $found[0]
        "path   : $($c.Source)"
        & $c.Source --version 2>&1
    }.GetNewClosure()
}

# --- the way out --------------------------------------------------------------
# A proxy nobody mentioned is the commonest reason a station cannot reach its
# transport, and the commonest reason nobody spots it is that it is set for the
# interactive user and not for the service account the station runs as.
Invoke-Probe 'proxy environment' {
    $any = $false
    foreach ($v in 'HTTP_PROXY', 'HTTPS_PROXY', 'NO_PROXY', 'http_proxy', 'https_proxy', 'no_proxy') {
        $val = [System.Environment]::GetEnvironmentVariable($v)
        if ($val) { "$v = $val"; $any = $true }
    }
    if (-not $any) { 'no proxy variables set in this environment' }
}
Invoke-ProbeOptional 'winhttp proxy (the machine-wide one, which the env does not show)' {
    & netsh winhttp show proxy 2>&1
}
Invoke-ProbeOptional 'dns servers' {
    [System.Net.NetworkInformation.NetworkInterface]::GetAllNetworkInterfaces() |
        Where-Object { $_.OperationalStatus -eq 'Up' } |
        ForEach-Object {
            $p = $_.GetIPProperties()
            foreach ($d in $p.DnsAddresses) { "$($_.Name): $d" }
        }
}

# --- provenance ---------------------------------------------------------------
Invoke-ProbeOptional 'this checkout' {
    & git log --oneline -3 2>&1
    & git status --short --branch 2>&1
}

Write-ProbeSummary
exit (Get-ProbeExitCode)
