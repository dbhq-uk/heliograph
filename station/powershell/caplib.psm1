# =============================================================================
#  caplib.psm1 - the capture, in PowerShell, under the conformance contract
# =============================================================================
# A second implementation of the capture is permitted ONLY while it passes
# tests/conformance. That is the whole rule, and it replaced a blanket
# prohibition: the argument against a port was always about UNTESTED drift.
#
# This file is the capture and nothing else. No transport, no gates, no loop.
#
# WHY A PORT AT ALL. A Windows box with Git for Windows can run the bash
# station and should - one implementation is better than two wherever there is
# a choice. This exists for the estate that has no bash and will not be given
# any, where the alternative is not a bash station but no station.
#
# WHAT MUST NEVER APPEAR ON THE CAPTURE PATH
#
# This is the PowerShell spelling of the busybox failure the whole suite exists
# for. Each of these collects the stream before handing it on, so every line of
# a block gets the timestamp of the flush, and the log reads perfectly while
# being useless - a hang and slow progress become indistinguishable, which is
# the single property these logs exist for.
#
#   $( )                 collects the whole stream into an array first
#   Out-String           without -Stream, joins everything into one string
#   Select-Object        buffers to count
#   Sort-Object          cannot emit until it has seen the last item
#   Group-Object         same
#   -Wait                on Get-Content, and on Start-Process
#
# The stamp is taken WHERE THE LINE IS READ, and never later.
# Anything that moves the stamp away from the read is the same defect wearing
# different clothes.
#
# FIVE FURTHER TRAPS, each the PowerShell equivalent of something the bash side
# already paid for. They are handled below and named where they are handled:
# ErrorRecord objects from 2>&1; $LASTEXITCODE clobbered or unset; the console
# output encoding and the OEM codepage; the trailing CR; and 5.1's
# SecurityProtocol defaulting below TLS 1.2.
# =============================================================================

Set-StrictMode -Version 2.0

# 5.1 defaults SecurityProtocol to SSL3/TLS1.0, and every modern endpoint has
# turned those off - so a station that needs to reach anything over HTTPS fails
# with a connection error that names TLS nowhere. Set once, at import, because
# it is process-wide state and setting it per-call is how one path gets missed.
# Guarded, because PowerShell 7 on Linux has no such default to fix.
try {
    if ([Net.ServicePointManager]::SecurityProtocol -band [Net.SecurityProtocolType]::Tls12) {
        # already usable
    } else {
        [Net.ServicePointManager]::SecurityProtocol =
            [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    }
} catch {
    # Not fatal. A station on a private git remote never opens a TLS socket,
    # and refusing to load the capture over it would be the wrong trade.
}

# --- redaction ----------------------------------------------------------------
# ONE RULE PER LINE, IN THE SAME ORDER AS caplib.sh's cap_redact.
#
# Neither redactor is the specification. tests/fixtures/redaction-corpus.txt is,
# and tests/test-redaction-corpus.sh proves every rule in it is load-bearing:
# delete any one and the corpus fails. Add a rule here and there must be a
# corpus line that ONLY it catches, or nothing is testing it.
#
# .NET regex rather than sed, so the syntax differs in three ways worth
# knowing: \b is the same, (?i) replaces sed's I flag, and a POSIX class like
# [[:space:]] must be written out.
$script:CapRedactRules = @(
    @{ P = '(?i)((password|passwd|pwd|secret|token|api[_-]?key|client_secret|accountkey|sas|connectionstring)["'']?\s*[:=]\s*["'']?)[^"''\s,;}]+'; R = '${1}***REDACTED***' }
    @{ P = '(://[^/@:\s]*):[^/@\s]*@'; R = '${1}:***REDACTED***@' }
    @{ P = '(?i)(https?://)[^/@:?#,\s]*@'; R = '${1}***REDACTED***@' }
    @{ P = '(Bearer\s+)[A-Za-z0-9._~+/-]{16,}=*'; R = '${1}***REDACTED***' }
    @{ P = '(Basic\s+)[A-Za-z0-9+/]{16,}=*'; R = '${1}***REDACTED***' }
    @{ P = '(?i)(Authorization:\s*[A-Za-z]+\s+)\S{8,}'; R = '${1}***REDACTED***' }
    @{ P = '\b(gh[pousr]_[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{16,})'; R = '***REDACTED***' }
    @{ P = '\b(AKIA|ASIA)[0-9A-Z]{16}\b'; R = '***REDACTED***' }
    @{ P = '\bxox[abposr]-[A-Za-z0-9-]{10,}'; R = '***REDACTED***' }
    @{ P = '\bglpat-[A-Za-z0-9_-]{16,}'; R = '***REDACTED***' }
    @{ P = '\bsk-[A-Za-z0-9_-]{20,}'; R = '***REDACTED***' }
    @{ P = '\beyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}'; R = '***REDACTED***' }
    @{ P = '-----BEGIN [A-Z ]*PRIVATE KEY-----'; R = '***REDACTED PRIVATE KEY***' }
)

# ANSI colour, stripped for the same reason the bash side strips it: a log full
# of escape sequences is unreadable in a file and unsearchable with grep.
$script:CapAnsi = [regex]'\x1b\[[0-9;]*[mGKHF]'

function Invoke-CapRedact {
    <#
      .SYNOPSIS
      One line in, one redacted line out. Never a stream: this is called from
      inside the read loop, once per line, so that nothing is ever collected.
    #>
    param([Parameter(Mandatory = $true)][AllowEmptyString()][string] $Line)

    if ($env:REDACT -eq '0') { return $Line }
    $out = $Line
    foreach ($rule in $script:CapRedactRules) {
        $out = [regex]::Replace($out, $rule.P, $rule.R)
    }
    return $out
}

function Get-CapStamp {
    # UTC, always, and HH:MM:SS to match the bash side byte for byte. A log
    # stamped in local time is read against the wrong hour by everybody who did
    # not run it, and these logs exist to be read by somebody else.
    return [DateTime]::UtcNow.ToString('HH:mm:ss')
}

function Write-CapHeader {
    param(
        [Parameter(Mandatory = $true)][string] $Path,
        [Parameter(Mandatory = $true)][string] $Label,
        [string[]] $Context = @()
    )
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add('============================================================')
    $lines.Add(" $Label")
    $lines.Add(" started UTC : " + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ'))
    $lines.Add(" control node: " + (Get-CapHostname))
    $lines.Add(" user        : " + (Get-CapUser))
    $lines.Add(" git branch  : " + (Get-CapGit @('rev-parse', '--abbrev-ref', 'HEAD')))
    $lines.Add(" git commit  : " + (Get-CapGit @('log', '--oneline', '-1')))
    foreach ($c in $Context) {
        if ($c) { $lines.Add(" context     : $c") }
    }
    $lines.Add('============================================================')
    $lines.Add('')
    Set-CapContent -Path $Path -Lines $lines
}

function Write-CapFooter {
    param(
        [Parameter(Mandatory = $true)][string] $Path,
        [Parameter(Mandatory = $true)][int] $ExitCode
    )
    $result = if ($ExitCode -eq 0) { 'OK' } else { 'FAILED' }
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add('')
    $lines.Add('============================================================')
    $lines.Add(" finished UTC : " + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ'))
    $lines.Add(" exit code    : $ExitCode")
    $lines.Add(" RESULT       : $result")
    $lines.Add('============================================================')
    foreach ($l in $lines) { Write-Host $l }
    Add-CapContent -Path $Path -Lines $lines
}

function Invoke-CapRun {
    <#
      .SYNOPSIS
      Run a command, stamping every line as it is READ, and return its REAL
      exit code.

      .DESCRIPTION
      The one function this module exists for. Everything about how it is
      written is downstream of one requirement: a line must be stamped at the
      moment it arrives, or a hang is invisible.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string] $Path,
        [Parameter(Mandatory = $true)][string] $FilePath,
        [string[]] $ArgumentList = @()
    )

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $FilePath
    foreach ($a in $ArgumentList) { [void]$psi.ArgumentList.Add($a) }
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true

    # THE ENCODING, set explicitly on both streams.
    #
    # A console under an OEM codepage (437, 850, 932) decodes a UTF-8 byte
    # sequence as mojibake, and the damage is done before anything here sees
    # the line - so no amount of care later recovers it. The step's output is
    # evidence from a machine nobody can reach twice.
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $psi.StandardErrorEncoding = [System.Text.Encoding]::UTF8

    $proc = New-Object System.Diagnostics.Process
    $proc.StartInfo = $psi
    [void]$proc.Start()

    # TWO PIPES, ONE THREAD, AND THE STAMP TAKEN THE INSTANT A LINE LANDS.
    #
    # stdout and stderr are separate pipes and must both be drained or a child
    # that fills one while we block on the other deadlocks. The obvious answer -
    # OutputDataReceived with a scriptblock handler - does not work and fails in
    # a way worth recording: the handler runs on a threadpool thread with no
    # runspace, and PowerShell terminates the whole process with "There is no
    # Runspace available to run scripts in this thread". Not an exception a
    # caller can catch. A capture that crashes the station is worse than one
    # that buffers.
    #
    # Register-ObjectEvent does have a runspace, but its -Action runs when the
    # ENGINE gets round to it, so the stamp would say when PowerShell was free
    # rather than when the line arrived - which is the buffering defect exactly,
    # just relocated.
    #
    # So: one ReadLineAsync per pipe, WaitAny for whichever completes first, and
    # the stamp taken here, in this thread, immediately. No handlers, no extra
    # runspaces, and nothing between the read and the clock.
    $writer = [System.IO.File]::AppendText($Path)
    try {
        $writer.AutoFlush = $true

        $tOut = $proc.StandardOutput.ReadLineAsync()
        $tErr = $proc.StandardError.ReadLineAsync()

        while (($null -ne $tOut) -or ($null -ne $tErr)) {
            $pending = New-Object 'System.Collections.Generic.List[System.Threading.Tasks.Task]'
            $which = New-Object 'System.Collections.Generic.List[string]'
            if ($null -ne $tOut) { $pending.Add($tOut); $which.Add('out') }
            if ($null -ne $tErr) { $pending.Add($tErr); $which.Add('err') }

            $i = [System.Threading.Tasks.Task]::WaitAny($pending.ToArray())
            $stamp = [DateTime]::UtcNow
            $line = $pending[$i].Result

            if ($null -eq $line) {
                # EOF on that pipe. Retired rather than re-issued, or the loop
                # spins on a completed task for as long as the other one runs.
                if ($which[$i] -eq 'out') { $tOut = $null } else { $tErr = $null }
                continue
            }

            # THE TRAILING CR. ReadLine splits on the newline and keeps
            # everything before it, so a CRLF stream leaves a CR on the end of
            # every value: invisible in a terminal, wrong in the file, and it
            # silently breaks any later grep anchored with $ - which is exactly
            # what somebody reads these logs with.
            if ($line.EndsWith("`r")) { $line = $line.Substring(0, $line.Length - 1) }

            $line = $script:CapAnsi.Replace($line, '')
            $line = Invoke-CapRedact -Line $line

            $stamped = $stamp.ToString('HH:mm:ss') + ' | ' + $line
            Write-Host $stamped
            $writer.WriteLine($stamped)

            if ($which[$i] -eq 'out') {
                $tOut = $proc.StandardOutput.ReadLineAsync()
            } else {
                $tErr = $proc.StandardError.ReadLineAsync()
            }
        }
    } finally {
        $writer.Close()
    }

    # Both pipes are at EOF, so the child has closed them, but closing a pipe
    # and exiting are not the same instant and ExitCode throws before exit.
    $proc.WaitForExit()

    # THE REAL EXIT CODE, taken from the process rather than from
    # $LASTEXITCODE.
    #
    # $LASTEXITCODE is set only by a NATIVE call and is left untouched by a
    # pure-PowerShell step, so reading it would report whatever the previous
    # native command in this session happened to return. It is also clobbered
    # by the next native call, and there are several between here and any
    # caller. The bash side has the same defect in a different shape - $? is
    # the exit of the last stage of the pipeline, which is why cap_run reads
    # PIPESTATUS[0] - and getting it wrong publishes every failed run as a
    # success, the most expensive defect available here.
    return $proc.ExitCode
}

# --- the small platform answers ----------------------------------------------
# Each is a function rather than an inline call so the capture path has no
# `if ($IsWindows)` in it. PowerShell 5.1 does not define $IsWindows at all,
# which is itself a trap: under Set-StrictMode it is an error, not $false.

function Get-CapHostname {
    try {
        $n = [System.Net.Dns]::GetHostEntry([string]::Empty).HostName
        if ($n) { return $n }
    } catch {
        # DNS may be unreachable, which is ordinary on an isolated estate and
        # is not a reason to fail a capture.
    }
    return [System.Environment]::MachineName
}

function Get-CapUser {
    if ($env:USERNAME) { return $env:USERNAME }
    if ($env:USER) { return $env:USER }
    return [System.Environment]::UserName
}

function Get-CapGit {
    param([string[]] $CliArgs)
    try {
        $out = & git @CliArgs 2>$null
        if ($LASTEXITCODE -ne 0) { return '' }
        # -join, because git log --oneline -1 is one line but git is entitled
        # to wrap and a header with a newline in it is a corrupt header.
        return (@($out) -join ' ').Trim()
    } catch {
        # No git is not a fault: a station on the share or the relay transport
        # need never have it installed.
        return ''
    }
}

# --- writing ------------------------------------------------------------------
# UTF-8 WITHOUT A BOM, and this is not cosmetic.
#
# PowerShell 5.1's `Out-File -Encoding utf8` writes a BOM, and Set-Content does
# too. A BOM at the head of a log is three bytes before the first `=` of the
# header: `grep '^===='` misses the first line, and the .station-env reader on
# the bash side refuses a BOM outright. Nothing in this toolkit writes one.

function Get-CapEncoding {
    return New-Object System.Text.UTF8Encoding($false)
}

function Set-CapContent {
    param([string] $Path, [System.Collections.Generic.List[string]] $Lines)
    [System.IO.File]::WriteAllLines($Path, $Lines, (Get-CapEncoding))
}

function Add-CapContent {
    param([string] $Path, [System.Collections.Generic.List[string]] $Lines)
    [System.IO.File]::AppendAllLines($Path, $Lines, (Get-CapEncoding))
}

Export-ModuleMember -Function @(
    'Invoke-CapRedact',
    'Get-CapStamp',
    'Write-CapHeader',
    'Write-CapFooter',
    'Invoke-CapRun',
    'Get-CapHostname',
    'Get-CapUser'
)
