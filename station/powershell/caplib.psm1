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

# --- building a command line -------------------------------------------------
# ProcessStartInfo.ArgumentList DOES NOT EXIST ON .NET FRAMEWORK, so it does not
# exist under Windows PowerShell 5.1 - which is the floor this module targets.
# It was added in .NET Core 2.1. Written against pwsh 7 and tested there, the
# first version used it and would have thrown "You cannot call a method on a
# null-valued expression" on the one platform this exists for.
#
# So: `Arguments`, built here, with the quoting rules CommandLineToArgvW
# actually applies. Those rules are not "wrap it in quotes": a backslash is
# literal EXCEPT immediately before a quote, where it must be doubled, and a
# run of backslashes at the end of an argument must be doubled because the
# closing quote follows it.
function ConvertTo-CapArgumentString {
    param([string[]] $Argv)
    $parts = New-Object System.Collections.Generic.List[string]
    foreach ($a in $Argv) {
        if ($a -eq '') { $parts.Add('""'); continue }
        if ($a -notmatch '[\s"]') { $parts.Add($a); continue }
        $sb = New-Object System.Text.StringBuilder
        [void]$sb.Append('"')
        $slashes = 0
        foreach ($ch in $a.ToCharArray()) {
            if ($ch -eq '\') {
                $slashes++
                continue
            }
            if ($ch -eq '"') {
                [void]$sb.Append('\' * ($slashes * 2 + 1))
                [void]$sb.Append('"')
            } else {
                [void]$sb.Append('\' * $slashes)
                [void]$sb.Append($ch)
            }
            $slashes = 0
        }
        [void]$sb.Append('\' * ($slashes * 2))
        [void]$sb.Append('"')
        $parts.Add($sb.ToString())
    }
    return ($parts -join ' ')
}

# POSIX single-quoting, for the `sh -c` path. Everything inside single quotes is
# literal except a single quote itself, which is closed, escaped and reopened.
function ConvertTo-CapPosixArgumentString {
    param([string[]] $Argv)
    $parts = New-Object System.Collections.Generic.List[string]
    foreach ($a in $Argv) {
        $parts.Add("'" + $a.Replace("'", "'\''") + "'")
    }
    return ($parts -join ' ')
}

function Test-CapWindows {
    # 5.1 does not define $IsWindows AT ALL, and under Set-StrictMode reading an
    # undefined variable is an error rather than $false - so the obvious test is
    # itself a 5.1-only failure. This asks the platform instead.
    return [System.Environment]::OSVersion.Platform -eq [System.PlatformID]::Win32NT
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

    # ONE PIPE, MERGED BY THE PLATFORM, exactly as the bash side does it.
    #
    # `cap_run` is `"$@" 2>&1 | ...`: the SHELL merges the two streams into one
    # before anything reads them. Everything downstream then sees a single
    # ordered stream, and that ordering is the child's own.
    #
    # Two pipes read from one thread cannot reproduce that, and the first
    # version of this function tried. Each pipe had one outstanding
    # ReadLineAsync and the loop took whichever completed first - so a line that
    # arrived while the loop was busy with the other stream sat in a completed
    # task, unstamped, until the loop came back for it. Under steady output on
    # one stream the other starves: its reader stops draining, its stamp drifts
    # from its arrival, and the child can block writing to it. The claim "the
    # stamp is taken the instant a line lands" was false in exactly the case
    # that matters - a busy step.
    #
    # Handler-based reading is not available either: a PowerShell scriptblock on
    # OutputDataReceived runs on a threadpool thread with no runspace and kills
    # the process outright ("There is no Runspace available to run scripts in
    # this thread"), and Register-ObjectEvent's -Action runs when the ENGINE is
    # free, which is the buffering defect relocated. Without Add-Type - which a
    # hardened estate may block - there is no way to put a reader on its own
    # thread.
    #
    # So the merge happens where bash does it: in the child. One pipe, one
    # blocking ReadLine on this thread, and the stamp taken the line after.
    $argv = @($FilePath) + $ArgumentList
    if (Test-CapWindows) {
        # /d skips AutoRun, which an estate may have set to something that
        # prints. /s makes cmd strip exactly the first and last quote and take
        # the rest verbatim, which is the only reliable way to pass a command
        # line through it.
        $shell = $env:ComSpec
        if (-not $shell) { $shell = 'cmd.exe' }
        $inner = ConvertTo-CapArgumentString -Argv $argv
        $arguments = '/d /s /c "' + $inner + ' 2>&1"'
    } else {
        $shell = '/bin/sh'
        $inner = ConvertTo-CapPosixArgumentString -Argv $argv
        $arguments = ConvertTo-CapArgumentString -Argv @('-c', ($inner + ' 2>&1'))
    }

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $shell
    $psi.Arguments = $arguments
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    # NOT redirected. There is nothing on it: the child merged it into stdout,
    # and a redirected pipe nobody reads is a pipe a child can block filling.
    $psi.RedirectStandardError = $false

    # THE DECODER, set explicitly.
    #
    # This chooses how BYTES ARE READ and it does not make the child emit UTF-8;
    # Microsoft is explicit that it cannot. What it fixes is the common case: a
    # console under an OEM codepage (437, 850, 932) whose default decoding turns
    # a UTF-8 sequence into mojibake before anything here sees the line. A
    # native program that genuinely emits OEM is a case this does not handle and
    # PLAN.md records it as such rather than pretending otherwise.
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8

    $proc = New-Object System.Diagnostics.Process
    $proc.StartInfo = $psi
    [void]$proc.Start()

    $writer = [System.IO.File]::AppendText($Path)
    try {
        $writer.AutoFlush = $true

        while ($true) {
            # BLOCKING, on this thread, so the stamp on the next line is the
            # moment this one returned and nothing can get between them.
            $line = $proc.StandardOutput.ReadLine()
            if ($null -eq $line) { break }
            $stamp = [DateTime]::UtcNow

            # THE TRAILING CR, kept as a belt to .NET's braces.
            #
            # .NET's StreamReader.ReadLine already treats CR, LF and CRLF as
            # terminators, so a trailing CR does not reach here - measured, not
            # assumed. bash's `read` splits on LF alone and DOES leave one,
            # which is why cap_run strips it. This stays because the guarantee
            # belongs to this function rather than to whichever reader it
            # happens to use, and it costs one comparison per line.
            #
            # It is also where the two implementations diverge on a BARE CR: a
            # progress bar writing "step 1`rstep 2`rstep 3`n" is one line with
            # embedded CRs to bash and three lines to .NET. PLAN.md records it.
            if ($line.EndsWith("`r")) { $line = $line.Substring(0, $line.Length - 1) }

            $line = $script:CapAnsi.Replace($line, '')
            $line = Invoke-CapRedact -Line $line

            $stamped = $stamp.ToString('HH:mm:ss') + ' | ' + $line
            Write-Host $stamped
            $writer.WriteLine($stamped)
        }
    } finally {
        $writer.Close()
    }

    # EOF on the pipe means the child closed it, which is not the same instant
    # as exiting - and ExitCode throws before the process has exited.
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
    'ConvertTo-CapArgumentString',
    'ConvertTo-CapPosixArgumentString',
    'Test-CapWindows',
    'Invoke-CapRedact',
    'Get-CapStamp',
    'Write-CapHeader',
    'Write-CapFooter',
    'Invoke-CapRun',
    'Get-CapHostname',
    'Get-CapUser'
)
