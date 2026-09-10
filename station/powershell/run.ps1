# =============================================================================
#  run.ps1 - the step runner.  ONE command for the operator to remember.
# =============================================================================
#
#     git pull; .\run.ps1                 # runs whatever step is currently set
#     git pull; .\run.ps1 <step>          # or name one explicitly
#     .\run.ps1 C:\path\to\probe.ps1      # or point at a step file directly
#     .\run.ps1 --list                    # what steps exist on this branch
#     .\run.ps1 --mode <step>             # what that step DECLARES itself to be
#     .\run.ps1 --file <step>             # which file that declaration came from
#
#  The PowerShell twin of run.sh, for the estate that has no bash. It is a
#  TWIN and not a port-in-spirit: the same three gates, the same exit codes,
#  the same log. A Windows box that has Git for Windows should keep running
#  run.sh - one implementation is better than two wherever there is a choice.
#
#  THE GATES IT CARRIES ARE 1, 2 AND 4. Gate 3 - the station must have been
#  started with --allow-actions - lives in the loop, which is station.ps1, and
#  the loop is a later PR. Nothing here silently stands in for it.
#
#  EXIT CODES ARE THE CONTRACT, and they are run.sh's:
#     2  unknown step, or a step file that cannot be run
#     3  the step declares no mode, an unrecognised mode, or is an action
#        without CONFIRM=yes
#     5  refused: this account is privileged
#     *  otherwise the step's own exit code, unchanged
# =============================================================================
# NO param() BLOCK AND NO [CmdletBinding()], deliberately.
#
# CmdletBinding adds the common parameters, and PowerShell binds them BEFORE
# this script sees anything - so `.\run.ps1 -Verbose` set a preference and ran
# the DEFAULT step, while run.sh called it an unknown step and exited 2. A step
# name that happens to start with a hyphen, or to abbreviate a common
# parameter, must reach the gates like any other.
#
# The automatic $args gets the words as typed, with no binding of any kind.
Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$RepoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $RepoRoot 'caplib.psm1') -Force

# ==============================================================
#  CURRENT STEP - Claude edits this line; the operator just pulls & runs
# ==============================================================
$DefaultStep = 'env'
# ==============================================================
#  Steps on this branch:
#    DIAGNOSTICS (read-only, safe to repeat)
#      env    control-node snapshot: OS, tools, auth, proxy, DNS      [default]
#    ACTIONS (change something - never make one the default step)
#      (none on main)
#
#    Which of the two a step is comes from the step's OWN FILE, not from this
#    table and not from its name: `# heliograph-mode: read-only` or `action` in
#    its first 30 lines. A step declaring neither will not run.
# ==============================================================
# CASE-SENSITIVE, because a PowerShell hashtable is not and run.sh's `case` is.
# `./run.ps1 ENV` resolving to the `env` step on one implementation and being an
# unknown step on the other is the kind of difference that makes a twin useless.
$StepTable = New-Object 'System.Collections.Hashtable' ([System.StringComparer]::Ordinal)
$StepTable['env'] = 'steps/env-snapshot.ps1'


function Write-Err { param([string] $Text) [Console]::Error.WriteLine($Text) }

# --- the arguments ------------------------------------------------------------
# `--mode <step>` answers what a step declares itself to be; `--file <step>`
# answers which file that declaration was read from. Both exit having touched
# nothing, and station.ps1 will ask through here rather than reading the step
# table itself - two copies of that mapping would drift the first time somebody
# registered a step that takes arguments.
$query = ''
$rest = @($args)
if ($rest.Count -gt 0) {
    # -ceq, because run.sh's `case` is case-sensitive and `--MODE` is not an
    # option there.
    if ($rest[0] -ceq '--mode' -or $rest[0] -ceq '--file') {
        $query = $rest[0].Substring(2)
        # `@()` WHEN THERE IS NOTHING LEFT. `1..0` in PowerShell is the sequence
        # 1,0 - it counts DOWN - so `$rest[1..0]` on a one-element array yields
        # two elements rather than none, and `.\run.ps1 --mode` was treated as
        # a step named `--mode`. run.sh shifts the option and answers about the
        # default step.
        if ($rest.Count -gt 1) { $rest = @($rest[1..($rest.Count - 1)]) } else { $rest = @() }
    }
}

$step = if ($rest.Count -gt 0 -and $rest[0]) { $rest[0] } else { $DefaultStep }

if ($step -ceq '--list' -or $step -ceq '-l') {
    # Read out of this file, so the table and the listing cannot disagree.
    $inBlock = $false
    foreach ($l in [System.IO.File]::ReadAllLines($MyInvocation.MyCommand.Path)) {
        if ($l -match '^#\s+Steps on this branch:') { $inBlock = $true }
        elseif ($inBlock -and $l -match '^# =+$') { break }
        if ($inBlock) { Write-Output ($l -replace '^#\s?', '') }
    }
    exit 0
}

# A STEP GIVEN AS A PATH IS RESOLVED BEFORE anything changes directory, or a
# relative path would resolve next to run.ps1 rather than next to the caller,
# and the caller would be told their own file is an unknown step.
#
# A name from the step table always wins - the table is consulted first - so a
# file that happens to be called `env` in the working directory cannot shadow
# the shipped step of that name.
$stepPath = ''
if (-not $StepTable.ContainsKey($step)) {
    try {
        $resolved = Resolve-Path -LiteralPath $step -ErrorAction Stop
        if (Test-Path -LiteralPath $resolved -PathType Leaf) { $stepPath = $resolved.Path }
    } catch {
        # Not a path. It will be reported as an unknown step below, which is
        # the honest answer: nothing here knows what it is.
    }
}

# --- pick the file the step runs from ----------------------------------------
if ($StepTable.ContainsKey($step)) {
    $stepFile = Join-Path $RepoRoot $StepTable[$step]
    if (-not (Test-Path -LiteralPath $stepFile -PathType Leaf)) {
        Write-Err "step '$step' is registered but its file is missing: $stepFile"
        exit 2
    }
} elseif ($stepPath) {
    $stepFile = $stepPath
} else {
    Write-Err "unknown step: $step"
    Write-Err "run '.\run.ps1 --list' to see the steps on this branch,"
    Write-Err "or give the path to a step file."
    exit 2
}

# --- GATE 1: a step declares what it is, and an undeclared step does not run --
# This gate is not a list of step NAMES, and the hole in that was not subtle:
# `cleanup-disk` matches no such list and would be waved through as a
# diagnostic, while a read-only step that happened to be called `deploy` would
# be gated for its spelling. A filename is not evidence about behaviour.
#
# Read from the file ABOUT TO BE EXECUTED, so a step submitted with a request
# refuses exactly as a committed one does.
$mode = ''
$lineNo = 0

# THE FILE IS READ AS BYTES FIRST, for two reasons that both end in the two
# runners disagreeing about the same file.
#
# A BOM. `File.ReadAllLines` detects one and strips it, so a declaration behind
# a BOM is accepted here - while run.sh's `sed` sees those bytes before the `#`,
# the anchor does not match, and the step is refused as undeclared. Editors on
# Windows write BOMs by default, so this is the common case rather than the
# exotic one. Refused here too, and SAID, because "your editor added three
# invisible bytes" is a fixable answer and "declares no mode" is not.
#
# UNREADABLE. Resolve-Path and Test-Path succeeding do not mean ReadAllLines
# will: an ACL, a lock, or a file replaced between the two throws, and the
# uncaught exception exits 1 - a code the contract does not define and the loop
# would misreport.
$headBytes = $null
try {
    $headBytes = [System.IO.File]::ReadAllBytes($stepFile)
} catch {
    Write-Err "cannot read step file: $stepFile"
    Write-Err "  $($_.Exception.Message)"
    exit 2
}
if ($headBytes.Length -ge 2) {
    $b0 = $headBytes[0]; $b1 = $headBytes[1]
    $b2 = if ($headBytes.Length -ge 3) { $headBytes[2] } else { 0 }
    if (($b0 -eq 0xEF -and $b1 -eq 0xBB -and $b2 -eq 0xBF) -or
        ($b0 -eq 0xFF -and $b1 -eq 0xFE) -or ($b0 -eq 0xFE -and $b1 -eq 0xFF)) {
        Write-Err "step '$step' ($stepFile) starts with a byte-order mark."
        Write-Err "  A BOM is three invisible bytes before the first character, and the"
        Write-Err "  declaration must be the first thing on its line for BOTH runners to"
        Write-Err "  read it. Save the file as UTF-8 without a BOM."
        exit 3
    }
}

$stepLines = $null
try {
    $stepLines = [System.IO.File]::ReadAllLines($stepFile)
} catch {
    Write-Err "cannot read step file: $stepFile"
    Write-Err "  $($_.Exception.Message)"
    exit 2
}

foreach ($l in $stepLines) {
    $lineNo++
    if ($lineNo -gt 30) { break }
    # -cmatch, case-sensitive, for the same reason as the switch below: the
    # bash side reads this with a case-sensitive sed, and a header spelled
    # `# HELIOGRAPH-MODE:` must mean the same thing to both or it means nothing.
    if ($l -cmatch '^#\s*heliograph-mode:\s*([A-Za-z-]+)') {
        $mode = $Matches[1]
        break
    }
}

if ($query -eq 'mode') {
    if ($mode) { Write-Output $mode } else { Write-Output 'undeclared' }
    exit 0
}
if ($query -eq 'file') {
    Write-Output $stepFile
    exit 0
}

# -CaseSensitive, because PowerShell's switch is not and bash's `case` is.
# Without it `# heliograph-mode: READ-ONLY` runs here and refuses with exit 3
# over there, from the same file. The gate is a declaration, and a declaration
# that means different things to two readers has not declared anything.
switch -CaseSensitive ($mode) {
    'read-only' { }
    'action' {
        # --- GATE 2: an action needs CONFIRM=yes ------------------------------
        # -cne, CASE-SENSITIVE, and that is not pedantry.
        #
        # PowerShell's -ne is case-INSENSITIVE by default, so `CONFIRM=YES`
        # satisfied this gate while bash's `[ "$CONFIRM" != "yes" ]` refuses
        # it. A state-changing step ran on one implementation and was refused
        # on the other, from the same request. Found by testing the four
        # spellings somebody actually types.
        #
        # The strict spelling is the point of the gate: `yes`, deliberately
        # typed, is different from a value that happens to look affirmative.
        if ($env:CONFIRM -cne 'yes') {
            Write-Err "step '$step' declares 'heliograph-mode: action' - it changes state."
            Write-Err "Re-run with: `$env:CONFIRM='yes'; .\run.ps1 $step"
            exit 3
        }
    }
    '' {
        Write-Err "step '$step' ($stepFile) declares no mode, so it will not run."
        Write-Err "Add one of these to the file, in its first 30 lines:"
        Write-Err "    # heliograph-mode: read-only     # measures, changes nothing"
        Write-Err "    # heliograph-mode: action        # changes state; needs CONFIRM=yes"
        exit 3
    }
    default {
        Write-Err "step '$step' ($stepFile) declares 'heliograph-mode: $mode', which is not a mode."
        Write-Err "The two accepted values are 'read-only' and 'action'. Nothing else is guessed at."
        exit 3
    }
}

# --- GATE 4: nothing runs as the privileged account ---------------------------
# AFTER the step has been validated, so an unknown step still says it is
# unknown whoever is asking, and BEFORE anything is captured, because a warning
# in a log is read after the run has already happened.
#
# The name stays ALLOW_ROOT rather than gaining a Windows synonym. It is the
# same gate, it is documented in one place, and an operator who has read
# /security should not have to learn a second spelling to switch it off.
if (-not (Test-CapPrivilegedAllowed)) {
    Write-Err 'refusing to run as a privileged account.'
    Write-Err '  This toolkit has no credentials of its own, so the account it runs as is'
    Write-Err '  the whole blast radius. As Administrator or SYSTEM that is the machine.'
    Write-Err '  Run it as an ordinary user, or set ALLOW_ROOT=1 if this image has no other.'
    exit 5
}

# --- the capture --------------------------------------------------------------
$outDir = if ($env:LOG_DIR) { $env:LOG_DIR } else { Join-Path $RepoRoot 'ops-logs' }
[void](New-Item -ItemType Directory -Force -Path $outDir)
$stamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ')

# The log is named for the step. A step given as a path would otherwise put its
# own directories into the filename, which lands the log outside outDir
# entirely. The banner and the header keep the full path, because there the
# caller wants to see exactly what ran.
$label = [System.IO.Path]::GetFileNameWithoutExtension($stepFile)
$out = Join-Path $outDir ("$label-$stamp.txt")

$shell = [System.Diagnostics.Process]::GetCurrentProcess().MainModule.FileName

# THE CHILD IS TOLD TO EMIT UTF-8, rather than only being decoded as it.
#
# caplib.psm1 sets StandardOutputEncoding, which chooses the DECODER and cannot
# make an arbitrary program emit UTF-8 - Microsoft is explicit about that. Here
# the child is not arbitrary: it is PowerShell, so it can be told. Under 5.1 on
# a console with an OEM codepage, a step printing a non-ASCII character
# otherwise lands in the log as mojibake, and the damage is done before
# anything downstream sees the line.
#
# `exit $LASTEXITCODE` at the end, or the wrapper's own success would replace
# the step's exit code - which is property 3, broken by the thing added to
# protect property 10.
$wrapper = @(
    '[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false);'
    "& '" + $stepFile.Replace("'", "''") + "';"
    'exit $LASTEXITCODE'
) -join ' '

Write-Host ''
Write-Host "==> STEP: $step"
Write-CapHeader -Path $out -Label "STEP: $step" -Context @("command: $shell -File $stepFile")
$rc = Invoke-CapRun -Path $out -FilePath $shell -ArgumentList @('-NoProfile', '-Command', $wrapper)
Write-CapFooter -Path $out -ExitCode $rc

# --- delivery ------------------------------------------------------------------
# THE FOOTER IS ALREADY WRITTEN, and always is before this point. Delivery ships
# a COMPLETE log or it ships nothing; a reader must never have to wonder whether
# the file they are holding is the whole run.
#
# AND A FAILED DELIVERY IS NEVER REPORTED AS A SUCCESS. It also never loses the
# log: the file is on local disk and its path is named, so somebody with access
# can still fetch it. That is property 9, and it is the one that let a real
# defect ship on the bash side - a station that captured perfectly and
# delivered nothing, invisibly, because the log was written correctly every
# time.
if ($env:PUSH -ceq '0') {
    Write-Host "PUSH=0 - captured locally, not delivered:"
    Write-Host "  $out"
} else {
    Import-Module (Join-Path $RepoRoot 'lib/transport.psm1') -Force
    if (Import-Tp) {
        if (Send-TpLog -LogPath $out -Message "step: $step ($stamp) exit=$rc ***NO_CI***") {
            Write-Host "delivered over '$(Get-TpName)': $(Split-Path -Leaf $out)"
        } else {
            # NAMED, both the file and the channel. Somebody with access can
            # still fetch it, and the reason is above this line.
            [Console]::Error.WriteLine("DELIVERY FAILED over '$(Get-TpName)'. The log is complete and is here:")
            [Console]::Error.WriteLine("  $out")
        }
    } else {
        [Console]::Error.WriteLine("no usable transport, so the log was NOT delivered. It is complete and is here:")
        [Console]::Error.WriteLine("  $out")
    }
}

$result = if ($rc -eq 0) { 'OK' } else { "FAILED (exit $rc)" }
Write-Host "DONE - step $step $result. See $out"
exit $rc
