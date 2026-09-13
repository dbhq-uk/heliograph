#!/usr/bin/env pwsh
# =============================================================================
#  trust-vectors.ps1 - the PowerShell trusted set, against bytes it did not make
# =============================================================================
#  The station's two implementations decide WHO MAY COMMAND A MACHINE. A control
#  side cannot tell which one answered, so if they disagreed, the same signed
#  change would be applied on one station and refused on another - and an estate
#  owner's audit would be right about half their estate with no way to know
#  which half.
#
#  So this does not round-trip anything. It reads tests/fixtures/trust-vectors.json,
#  written by the Go side, and checks that this implementation produces the same
#  canonical bytes, the same digests, the same signing input, and above all THE
#  SAME REFUSALS.
#
#  EVERY STAGE IS PINNED, not just the digest. A port that gets one thing wrong
#  should be told which thing, and the likeliest mistake is the least visible:
#  length prefixes counted in characters rather than in bytes, which is correct
#  for every ASCII name anybody tests with and wrong for the first estate that
#  has a person called josé in it.
#
#  Run directly, or through tests/test-trust-ps1.sh.
# =============================================================================
Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$root = Split-Path -Parent $here
Import-Module (Join-Path $root 'station/powershell/lib/seal.psm1') -Force
Import-Module (Join-Path $root 'station/powershell/lib/trust.psm1') -Force

function ToHex { param([byte[]] $b) return (([BitConverter]::ToString($b)) -replace '-', '').ToLowerInvariant() }

$script:pass = 0; $script:fail = 0
function Check {
    param([string] $name, [string] $want, [string] $got)
    if ($want -ceq $got) {
        $script:pass++; "ok   $name"
    } else {
        $script:fail++; "FAIL $name"; "     want $want"; "     got  $got"
    }
}
function CheckTrue {
    param([string] $name, [bool] $ok)
    if ($ok) { $script:pass++; "ok   $name" } else { $script:fail++; "FAIL $name" }
}

Initialize-Seal

$vecPath = Join-Path $root 'tests/fixtures/trust-vectors.json'
if (-not (Test-Path $vecPath)) {
    "FAIL the golden vectors are missing at $vecPath, so NOTHING here was asserted"
    exit 1
}
$V = Get-Content -Raw $vecPath | ConvertFrom-Json

Check 'the two implementations agree on the document version' `
    ([string] $V.version) ([string] 1)
Check 'and on the signing domain, which is what stops a signature being replayed as another kind' `
    $V.domain 'heliograph-trusted-set-change-v1'

# --- identities ---------------------------------------------------------------
# The fixture carries secrets, and they are published in a public repository and
# worth exactly nothing. If either side derived a different public half from the
# same secret, every signature below would differ for a reason that has nothing
# to do with the trusted set.
$ids = @{}
foreach ($name in @('anchor', 'alice', 'bob')) {
    $iv = $V.identities.$name
    $id = Import-SealIdentity $iv.secret
    $ids[$name] = $id
    Check "identity $name derives the same public half" $iv.public (Export-SealPublic $id)
    Check "identity $name has the same fingerprint" $iv.fingerprint (Get-SealFingerprint $id)
}

# --- the sets -------------------------------------------------------------------
$sets = @{}
foreach ($sv in $V.sets) {
    # PARSED FROM THE GO SIDE'S OWN TEXT. Building a set here and comparing would
    # only show that this implementation agrees with itself.
    $s = Import-TrustSet -Text $sv.marshalled
    $sets[$sv.name] = $s

    Check "set '$($sv.name)': the canonical bytes match" $sv.canonicalHex (ToHex (Get-TrustSetCanonical $s))
    Check "set '$($sv.name)': the digest matches" $sv.digest (Get-TrustSetDigest $s)
    Check "set '$($sv.name)': the published member line matches" $sv.membersLine (Get-TrustMembersLine $s)

    # AND IT RE-MARSHALS TO THE SAME TEXT. A set the station writes back after
    # applying a change has to be byte-identical to what Go would have written,
    # or the two stations publish different digests for identical trust and
    # `heliograph doctor` reports a divergence that is not one.
    Check "set '$($sv.name)': re-marshals byte for byte" $sv.marshalled (Export-TrustSet $s)
}

# --- the changes ------------------------------------------------------------------
foreach ($cv in $V.changes) {
    $c = Import-TrustChange -Text $cv.marshalled
    CheckTrue "change '$($cv.name)': parses out of the document" ($null -ne $c)
    if ($null -eq $c) { continue }

    Check "change '$($cv.name)': the canonical bytes match" $cv.canonicalHex (ToHex (Get-TrustChangeCanonical $c))

    # THE EXACT BYTES Ed25519 COVERS. This is the stage where a port that
    # concatenates instead of length-prefixing diverges with no other symptom,
    # and where counting characters rather than bytes hides until somebody has a
    # non-ASCII name.
    $input = Get-TrustSigningInput $V.domain (Get-TrustChangeCanonical $c)
    Check "change '$($cv.name)': the signing input matches" $cv.signingInputHex (ToHex $input)
    Check "change '$($cv.name)': the signature bytes match" $cv.signatureHex (ToHex $c.Sig)
    CheckTrue "change '$($cv.name)': and the signature verifies here" `
        ([Chaos.NaCl.Ed25519]::Verify($c.Sig, $input, $c.AuthorKey.SignPublic))

    # --- AND THE DECISION, WHICH IS THE POINT -----------------------------------
    $base = $sets[$cv.appliesTo]
    CheckTrue "change '$($cv.name)': the set it applies to is in the fixture" ($null -ne $base)
    if ($null -eq $base) { continue }

    $before = Get-TrustSetDigest $base
    $result = $null
    $refusal = $null
    try { $result = Invoke-TrustApply $base $c ([datetime]::Parse('2026-01-04T09:00:00Z').ToUniversalTime()) }
    catch { $refusal = $_.Exception.Message }

    if ($cv.refusal) {
        CheckTrue "change '$($cv.name)': REFUSED here too" ($null -ne $refusal)
        if ($refusal) {
            # THE WHOLE SENTENCE, not a substring. The refusal is published to a
            # far side that cannot log in, and two stations giving different
            # reasons for the same decision is a support call nobody can close.
            Check "change '$($cv.name)': and refused for the same stated reason" $cv.refusal $refusal
        }
        Check "change '$($cv.name)': and the set did not move" $before (Get-TrustSetDigest $base)
    } else {
        CheckTrue "change '$($cv.name)': applied here too" ($null -ne $result)
        if ($refusal) { "     refused with: $refusal" }
        if ($result) {
            Check "change '$($cv.name)': and produced the same set" $cv.resultDigest (Get-TrustSetDigest $result)
        }
        Check "change '$($cv.name)': and did not mutate the set it was applied to" $before (Get-TrustSetDigest $base)
    }
}

# --- the length-prefix trap, asserted rather than hoped for -----------------------
#
# Nothing in the fixture above has a non-ASCII name, because the Go side chooses
# the fixture and a fixture cannot cover what nobody thought of. This is the one
# case worth stating directly: a name that is more bytes than characters. If
# this implementation counted characters, every check above would still pass and
# the first estate with a person called josé would have a station that refuses
# every change from one of its two implementations.
$utf8Name = 'jos' + [char] 0x00E9
$u = [System.Text.Encoding]::UTF8.GetBytes($utf8Name)
CheckTrue 'the trap is real: the test name is more bytes than characters' `
    ($u.Length -gt $utf8Name.Length)
$parts = New-Object System.Collections.Generic.List[byte[]]
$probe = Get-TrustSigningInput 'd' ([System.Text.Encoding]::UTF8.GetBytes($utf8Name))
# The body length is the last 8-byte prefix before the body itself.
$bodyLen = 0
for ($i = 0; $i -lt 8; $i++) { $bodyLen = ($bodyLen * 256) + $probe[$probe.Length - $u.Length - 8 + $i] }
Check 'and the signing input length-prefixes in BYTES, not characters' `
    ([string] $u.Length) ([string] $bodyLen)

""
"trust-vectors.ps1: $script:pass passed, $script:fail failed"
if ($script:fail -gt 0) { exit 1 }
exit 0
