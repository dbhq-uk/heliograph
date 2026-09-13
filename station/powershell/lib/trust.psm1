# =============================================================================
#  trust.psm1 - the set of keys this station accepts a request from
# =============================================================================
#  The PowerShell twin of internal/trust. It parses a trusted set, verifies a
#  signed change against it, and applies it - and it needs NOTHING INSTALLED to
#  do so, because Ed25519 verification is already here in managed C# (see
#  lib/seal.psm1 and vendor/Chaos.NaCl). The bash station has to shell out to
#  heliograph-seal for the same work; this one does not.
#
#  WHY THERE ARE TWO IMPLEMENTATIONS AT ALL, AND WHAT KEEPS THEM HONEST.
#
#  A control side cannot tell which implementation answered a request. If these
#  two disagreed about who may command a station, the same signed change would
#  be applied on one machine and refused on another, and the estate owner's
#  audit would be right about half their estate. The twins have diverged on a
#  gate before - three case-sensitivity differences in run.ps1, two of them in a
#  security check - so agreement is asserted rather than assumed:
#  tests/test-trust-ps1.sh drives BOTH against the same fixtures and compares
#  digests, refusals and applied results byte for byte.
#
#  NO BESPOKE CRYPTOGRAPHY. Ed25519 through Chaos.NaCl, which is djb's ref10,
#  over the same length-prefixed canonical encoding the Go side builds. The
#  encoding is not cryptography: it exists so two different documents can never
#  serialise to the same bytes.
#
#  LENGTH IN BYTES, NOT IN CHARACTERS, everywhere below. Get-SealCanonical
#  carries the same warning and the same reason: a member named `josé` is five
#  bytes and four characters, and counting characters produces a digest the Go
#  side does not recognise on exactly the estates most likely to have one.
# =============================================================================

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# The seal, imported here rather than assumed, and WITHOUT -Force.
#
# This module needs Export-SealPublic, Import-SealPublic, Get-SealFingerprint,
# Join-SealBytes and Chaos.NaCl. A module CAN see commands the caller imported
# globally, so leaving this out appears to work - right up until something
# imports trust.psm1 without having imported seal.psm1 first.
#
# -Force IS THE TRAP, and it cost a debugging round. `Import-Module seal -Force`
# from in here UNLOADS the copy the caller already imported and loads a fresh
# one, so the caller's own `Initialize-Seal` stops resolving the moment it
# imports this module. The symptom is "The term 'Initialize-Seal' is not
# recognized" reported from the caller's line, with nothing wrong at that line.
if (-not (Get-Module -Name 'seal')) {
    Import-Module (Join-Path $PSScriptRoot 'seal.psm1') -DisableNameChecking
}

$script:TrustVersion = 1
$script:TrustDomain = 'heliograph-trusted-set-change-v1'
$script:TrustSetTag = 'heliograph-trusted-set-v1'
$script:DocumentPrefix = 'heliograph-document-v1'

# --- the canonical encoder ----------------------------------------------------

# THE COMMA IS LOAD-BEARING, and its absence cost a debugging round.
#
# PowerShell ENUMERATES a collection returned from a function. An empty List
# therefore returns nothing at all, so `$parts = New-TrustPartList` binds $null
# and the first Add-TrustString fails with "You cannot call a method on a
# null-valued expression" - reported from inside the helper, with nothing wrong
# at the line that called it. seal.psm1 comma-wraps every byte[] it returns for
# the same reason.
function New-TrustPartList { return , (New-Object System.Collections.Generic.List[byte[]]) }

# Big-endian, and PRIVATE TO THIS MODULE rather than borrowed from seal.psm1.
#
# seal.psm1 has the identical function and does not export it. Exporting it
# would widen a module every station imports in order to save nine lines here,
# and the two are allowed to differ later: the seal's canonical metadata and the
# trusted set's are separate formats that happen to agree today.
function ConvertTo-TrustUInt64BE {
    param([UInt64] $Value)
    $b = New-Object byte[] 8
    for ($i = 0; $i -lt 8; $i++) { $b[$i] = [byte](($Value -shr (8 * (7 - $i))) -band 0xFF) }
    return , $b
}

function Add-TrustString {
    param($Parts, [AllowEmptyString()][string] $Value)
    $u = [System.Text.Encoding]::UTF8.GetBytes($Value)
    $Parts.Add((ConvertTo-TrustUInt64BE ([UInt64] $u.Length)))
    if ($u.Length -gt 0) { $Parts.Add($u) }
}

function Add-TrustNumber {
    param($Parts, [UInt64] $Value)
    $Parts.Add((ConvertTo-TrustUInt64BE $Value))
}

# Get-TrustSigningInput is seal.SignDocument's input, byte for byte.
#
# The constant prefix is what stops a signature made over a trusted-set change
# ever verifying as a sealed message. Protect-Seal's signed input begins with
# eight bytes of the seal version, which is 1; this one begins with eight bytes
# holding the length of the prefix below, which is 22. They cannot collide.
function Get-TrustSigningInput {
    param([string] $Domain, [byte[]] $Body)
    $parts = New-TrustPartList
    Add-TrustString $parts $script:DocumentPrefix
    Add-TrustString $parts $Domain
    $parts.Add((ConvertTo-TrustUInt64BE ([UInt64] $Body.Length)))
    if ($Body.Length -gt 0) { $parts.Add($Body) }
    return Join-SealBytes $parts.ToArray()
}

# --- members ------------------------------------------------------------------

function Test-TrustName {
    param([string] $Name)
    if (-not $Name) { return $false }
    if ($Name.Length -gt 64) { return $false }
    return ($Name -cmatch '^[A-Za-z0-9._-]+$')
}

function New-TrustMember {
    param(
        [string] $Name, $Public,
        [AllowEmptyString()][string] $Added = '',
        [AllowEmptyString()][string] $AddedBy = '',
        [AllowEmptyString()][string] $Revoked = '',
        [AllowEmptyString()][string] $RevokedBy = ''
    )
    return [pscustomobject]@{
        Name = $Name; Public = $Public
        Added = $Added; AddedBy = $AddedBy
        Revoked = $Revoked; RevokedBy = $RevokedBy
    }
}

function Test-TrustMemberActive { param($Member) return [string]::IsNullOrEmpty($Member.Revoked) }

# Test-TrustSamePublic compares BOTH halves.
#
# A member is identified by the pair. Comparing only the signing key would let a
# key be re-added under a different encryption half, and that is a different
# party holding the same authority to author.
function Test-TrustSamePublic {
    param($A, $B)
    if (-not $A -or -not $B) { return $false }
    if ($A.EncPublic.Length -ne $B.EncPublic.Length) { return $false }
    if ($A.SignPublic.Length -ne $B.SignPublic.Length) { return $false }
    for ($i = 0; $i -lt $A.EncPublic.Length; $i++) { if ($A.EncPublic[$i] -ne $B.EncPublic[$i]) { return $false } }
    for ($i = 0; $i -lt $A.SignPublic.Length; $i++) { if ($A.SignPublic[$i] -ne $B.SignPublic[$i]) { return $false } }
    return $true
}

function Get-TrustEveryone {
    param($Set)
    $all = New-Object System.Collections.Generic.List[object]
    [void] $all.Add($Set.Anchor)
    foreach ($m in $Set.Members) { [void] $all.Add($m) }
    # Comma-wrapped for the reason New-TrustPartList is. A set with an anchor
    # and exactly one member would otherwise come back as two loose objects, and
    # one with an anchor alone as a bare object rather than a collection.
    return , $all
}

function Find-TrustMember {
    param($Set, $Public)
    foreach ($m in (Get-TrustEveryone $Set)) {
        if (Test-TrustSamePublic $m.Public $Public) { return $m }
    }
    return $null
}

# Find-TrustMemberBySigning matches on the Ed25519 half alone.
#
# IT EXISTS FOR ONE PURPOSE: turning a failed verification into a sentence that
# names somebody. It decides nothing, and every caller has already exhausted
# verification against the active members before reaching it. Using it to
# authorise would mean accepting a party holding a signing key and a DIFFERENT
# encryption key, which is a different party.
function Find-TrustMemberBySigning {
    param($Set, [byte[]] $SignPublic)
    foreach ($m in (Get-TrustEveryone $Set)) {
        if ($m.Public.SignPublic.Length -ne $SignPublic.Length) { continue }
        $same = $true
        for ($i = 0; $i -lt $SignPublic.Length; $i++) {
            if ($m.Public.SignPublic[$i] -ne $SignPublic[$i]) { $same = $false; break }
        }
        if ($same) { return $m }
    }
    return $null
}

# --- the set ------------------------------------------------------------------

function Get-TrustMemberCanonical {
    param($Parts, $Member)
    Add-TrustString $Parts $Member.Name
    Add-TrustString $Parts (Export-SealPublic $Member.Public)
    Add-TrustString $Parts $Member.Added
    Add-TrustString $Parts $Member.AddedBy
    Add-TrustString $Parts $Member.Revoked
    Add-TrustString $Parts $Member.RevokedBy
}

function Get-TrustSetCanonical {
    param($Set)
    $parts = New-TrustPartList
    Add-TrustString $parts $script:TrustSetTag
    Add-TrustNumber $parts ([UInt64] $Set.Version)
    Add-TrustString $parts $Set.Estate
    Add-TrustString $parts $Set.Station
    Add-TrustNumber $parts ([UInt64] $Set.Serial)
    Get-TrustMemberCanonical $parts $Set.Anchor
    Add-TrustNumber $parts ([UInt64] $Set.Members.Count)
    foreach ($m in $Set.Members) { Get-TrustMemberCanonical $parts $m }
    return Join-SealBytes $parts.ToArray()
}

# Get-TrustSetDigest is what a change names as the set it applies to, and what
# the station publishes so the owner can compare without reading every key.
#
# UTC IS NOT IN IT. The digest has to be reproducible on the control node from a
# copy of the same set, and when a particular machine last wrote the file is not
# a property of the set. Including it would make every station report a
# different digest for identical trust.
function Get-TrustSetDigest {
    param($Set)
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try { $h = $sha.ComputeHash((Get-TrustSetCanonical $Set)) } finally { $sha.Dispose() }
    return (([BitConverter]::ToString($h)) -replace '-', '').ToLowerInvariant()
}

function ConvertTo-TrustDash { param([AllowEmptyString()][string] $Value) if ($Value) { return $Value } return '-' }
function ConvertFrom-TrustDash { param([string] $Value) if ($Value -ceq '-') { return '' } return $Value }

function Format-TrustMember {
    param($Member)
    return (@(
        $Member.Name, (Export-SealPublic $Member.Public),
        (ConvertTo-TrustDash $Member.Added), (ConvertTo-TrustDash $Member.AddedBy),
        (ConvertTo-TrustDash $Member.Revoked), (ConvertTo-TrustDash $Member.RevokedBy)
    ) -join ' ')
}

# Export-TrustSet writes the same `key: value` shape as every other document
# that crosses the gap. The estate owner reads this with `cat` on a machine we
# cannot reach, which is the whole point of publishing it.
#
# LF LINE ENDINGS, EXPLICITLY. PowerShell writes CRLF by default and the Go side
# writes LF, and a set that round-trips through the two would parse identically
# and DIGEST DIFFERENTLY - so the station would publish a digest the control
# node reads as a divergence, which is the alarm this mechanism exists to raise,
# fired by a line ending.
function Export-TrustSet {
    param($Set)
    $sb = New-Object System.Text.StringBuilder
    [void] $sb.Append("version: $($Set.Version)`n")
    [void] $sb.Append("estate: $($Set.Estate)`n")
    [void] $sb.Append("station: $($Set.Station)`n")
    [void] $sb.Append("serial: $($Set.Serial)`n")
    [void] $sb.Append("utc: $($Set.UTC)`n")
    [void] $sb.Append("digest: $(Get-TrustSetDigest $Set)`n")
    [void] $sb.Append("anchor: $(Format-TrustMember $Set.Anchor)`n")
    foreach ($m in $Set.Members) { [void] $sb.Append("member: $(Format-TrustMember $m)`n") }
    return $sb.ToString()
}

function Split-TrustField {
    param([string] $Line)
    $i = $Line.IndexOf(':')
    if ($i -lt 0) { return $null }
    return [pscustomobject]@{
        Key = $Line.Substring(0, $i).Trim()
        Value = $Line.Substring($i + 1).Trim()
    }
}

function Import-TrustMemberLine {
    param([string] $Value)
    $f = $Value -split '\s+' | Where-Object { $_ -ne '' }
    if ($f.Count -ne 6) {
        throw "trust: a member line is six fields (name key added added-by revoked revoked-by), got $($f.Count): '$Value'"
    }
    if (-not (Test-TrustName $f[0])) {
        throw "trust: '$($f[0])' is not a usable member name: letters, digits, dot, hyphen and underscore only"
    }
    return (New-TrustMember -Name $f[0] -Public (Import-SealPublic $f[1]) `
        -Added (ConvertFrom-TrustDash $f[2]) -AddedBy (ConvertFrom-TrustDash $f[3]) `
        -Revoked (ConvertFrom-TrustDash $f[4]) -RevokedBy (ConvertFrom-TrustDash $f[5]))
}

# Import-TrustSet reads a set back.
#
# UNLIKE A REQUEST, AN UNKNOWN VERSION IS AN ERROR. A request a station half
# understands runs a step and produces a log somebody can read; a trusted set a
# station half understands decides who may command the machine, and the failure
# mode of guessing is trusting a key nobody meant to trust.
function Import-TrustSet {
    param([Parameter(Mandatory = $true)][string] $Text)
    $set = [pscustomobject]@{
        Version = 0; Estate = ''; Station = ''; Serial = [UInt64] 0; UTC = ''
        Anchor = $null
        Members = (New-Object System.Collections.Generic.List[object])
    }
    foreach ($line in ($Text -split "`r?`n")) {
        $kv = Split-TrustField $line
        if (-not $kv) { continue }
        switch ($kv.Key) {
            'version' { $set.Version = [int] $kv.Value }
            'estate'  { $set.Estate = $kv.Value }
            'station' { $set.Station = $kv.Value }
            'serial'  {
                $n = [UInt64] 0
                if (-not [UInt64]::TryParse($kv.Value, [ref] $n)) {
                    throw "trust: serial '$($kv.Value)' is not a number, and the serial is what stops a change being replayed"
                }
                $set.Serial = $n
            }
            'utc'     { $set.UTC = $kv.Value }
            'digest'  { }   # recomputed rather than trusted: it describes the file it is in
            'anchor'  { $set.Anchor = Import-TrustMemberLine $kv.Value }
            'member'  { [void] $set.Members.Add((Import-TrustMemberLine $kv.Value)) }
        }
    }
    if ($set.Version -ne $script:TrustVersion) {
        throw "trust: this set is version $($set.Version) and this build reads version $($script:TrustVersion): refusing to guess at who may command this station"
    }
    if (-not $set.Anchor) {
        throw 'trust: this set has no anchor, and a set without one has nothing that survives a compromised key'
    }
    if (-not $set.Estate -or -not $set.Station) {
        throw 'trust: this set names no estate or no station, so it cannot be bound to one machine'
    }
    return $set
}

function Import-TrustSetFile {
    param([string] $Path)
    return (Import-TrustSet -Text ([System.IO.File]::ReadAllText($Path)))
}

# Save-TrustSet replaces the file atomically.
#
# WRITE-THEN-RENAME. A truncated trusted set does not parse, and a station that
# cannot parse its trusted set accepts nothing - on a machine nobody can log
# into. The relay's sequence file taught this repository that lesson once
# already, where a truncated one read as sequence zero and accepted every replay
# the relay had ever seen.
function Save-TrustSet {
    param([string] $Path, $Set)
    $tmp = "$Path.$PID.tmp"
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($tmp, (Export-TrustSet $Set), $utf8)
    Move-Item -LiteralPath $tmp -Destination $Path -Force
}

# --- a change -----------------------------------------------------------------

function Get-TrustChangeCanonical {
    param($Change)
    $parts = New-TrustPartList
    Add-TrustNumber $parts ([UInt64] $Change.Version)
    Add-TrustString $parts $Change.Estate
    Add-TrustString $parts $Change.Station
    Add-TrustNumber $parts ([UInt64] $Change.Serial)
    Add-TrustString $parts $Change.Prev
    Add-TrustString $parts $Change.UTC
    Add-TrustString $parts $Change.Op
    Add-TrustString $parts $Change.Name
    Add-TrustString $parts (Export-SealPublic $Change.Subject)
    Add-TrustString $parts $Change.Author
    Add-TrustString $parts (Export-SealPublic $Change.AuthorKey)
    return Join-SealBytes $parts.ToArray()
}

function Import-TrustNamedKey {
    param([string] $Value)
    $f = $Value -split '\s+' | Where-Object { $_ -ne '' }
    if ($f.Count -ne 2) { throw "expected '<name> <key>', got '$Value'" }
    return [pscustomobject]@{ Name = $f[0]; Public = (Import-SealPublic $f[1]) }
}

# Import-TrustChange reads a change out of a document, which may be a whole
# request: the keys are prefixed `trust-` precisely so a change can ride inside
# one. Returns $null when there is no change here, which is the ordinary case
# for every request that is only a step.
function Import-TrustChange {
    param([Parameter(Mandatory = $true)][string] $Text)
    $c = [pscustomobject]@{
        Version = 0; Estate = ''; Station = ''; Serial = [UInt64] 0
        Prev = ''; UTC = ''; Op = ''; Name = ''
        Subject = $null; Author = ''; AuthorKey = $null; Sig = $null
    }
    $seen = $false
    foreach ($line in ($Text -split "`r?`n")) {
        $kv = Split-TrustField $line
        if (-not $kv) { continue }
        if (-not $kv.Key.StartsWith('trust-')) { continue }
        $seen = $true
        switch ($kv.Key) {
            'trust-version' { $c.Version = [int] $kv.Value }
            'trust-estate'  { $c.Estate = $kv.Value }
            'trust-station' { $c.Station = $kv.Value }
            'trust-serial'  {
                $n = [UInt64] 0
                if (-not [UInt64]::TryParse($kv.Value, [ref] $n)) { throw "trust: serial '$($kv.Value)' is not a number" }
                $c.Serial = $n
            }
            'trust-prev'    { $c.Prev = $kv.Value }
            'trust-utc'     { $c.UTC = $kv.Value }
            'trust-op'      { $c.Op = $kv.Value }
            'trust-subject' {
                $nk = Import-TrustNamedKey $kv.Value
                $c.Name = $nk.Name; $c.Subject = $nk.Public
            }
            'trust-author'  {
                $nk = Import-TrustNamedKey $kv.Value
                $c.Author = $nk.Name; $c.AuthorKey = $nk.Public
            }
            'trust-sig'     { $c.Sig = ConvertFrom-SealBase64 $kv.Value }
        }
    }
    if (-not $seen) { return $null }
    return $c
}

function Test-TrustChangeShape {
    param($Change)
    if ($Change.Version -ne $script:TrustVersion) {
        throw "trust: this change is version $($Change.Version) and this build reads version $($script:TrustVersion)"
    }
    if (-not $Change.Estate -or -not $Change.Station) {
        throw 'trust: a change must name its estate and station, or one machine''s change applies at another'
    }
    if ($Change.Op -cne 'add' -and $Change.Op -cne 'revoke') {
        throw "trust: '$($Change.Op)' is not an operation: a change adds or revokes"
    }
    if (-not (Test-TrustName $Change.Name)) {
        throw "trust: '$($Change.Name)' is not a usable member name: letters, digits, dot, hyphen and underscore only"
    }
    if (-not (Test-TrustName $Change.Author)) {
        throw "trust: '$($Change.Author)' is not a usable member name: letters, digits, dot, hyphen and underscore only"
    }
    if (-not $Change.Prev) {
        throw 'trust: a change must name the set it was authored against, or it can be applied to a set that has moved on'
    }
    if (-not $Change.Subject) {
        throw 'trust: a change must carry the subject''s key, so that a revocation names a key rather than a name somebody could reuse'
    }
    if (-not $Change.AuthorKey) {
        throw 'trust: a change must carry the author''s key, which is what it is verified against'
    }
}

# --- applying one -------------------------------------------------------------

# Invoke-TrustApply verifies a change against a set and RETURNS A NEW SET.
#
# IT NEVER MUTATES THE ONE IT WAS GIVEN. A refused change must leave the set
# exactly as it was, and a function that edits in place and then reports a
# refusal has already half-applied by the time the caller reads it.
#
# The order of the checks is chosen, not incidental, and it is the same order
# internal/trust uses. Cheap, unambiguous bindings first - target, ordering,
# fork - so a document a hostile transport made up costs no cryptography. The
# signature before any decision about WHAT the change does, so an unsigned
# document never reaches the code that would apply it. The anchor rule after the
# signature and before the mutation, because it must hold even for a change a
# trusted key signed: that is the entire point of it.
#
# A refusal is thrown with a one-line message, because station.ps1 publishes it
# as the status reason and a newline there would forge a second key.
function Invoke-TrustApply {
    param($Set, $Change, [datetime] $Now)

    Test-TrustChangeShape $Change

    if ($Change.Estate -cne $Set.Estate -or $Change.Station -cne $Set.Station) {
        throw ("trust: this change was authored for a different estate or station: it names " +
               "$($Change.Estate)/$($Change.Station) and this station is $($Set.Estate)/$($Set.Station)")
    }

    # THE REPLAY DEFENCE, AND IT SURVIVES A RESTART BECAUSE THE SERIAL IS IN THE
    # SET ON DISK. An in-memory window forgets on restart, and a station restarts
    # whenever the machine does - which is exactly when nobody is watching.
    if ($Change.Serial -ne ($Set.Serial + 1)) {
        throw ("trust: this change is not the next one: it is serial $($Change.Serial) and this " +
               "station is at $($Set.Serial), so the next one is $($Set.Serial + 1)")
    }

    $digest = Get-TrustSetDigest $Set
    if ($Change.Prev -cne $digest) {
        throw ("trust: this change was authored against a different set: it was authored against " +
               "set $($Change.Prev.Substring(0, [Math]::Min(12, $Change.Prev.Length))) and this " +
               "station holds $($digest.Substring(0, 12))")
    }

    $author = Find-TrustMember $Set $Change.AuthorKey
    if (-not $author) {
        throw ("trust: this change was signed by a key this station does not trust: the signing key " +
               "is $(Get-SealFingerprint $Change.AuthorKey), which is in neither the anchor nor any member")
    }
    if (-not (Test-TrustMemberActive $author)) {
        # NAMES THE REVOKED KEY. "not trusted" reads as a broken enrolment and
        # sends somebody to re-plant a station that is working perfectly.
        throw ("trust: this change was signed by a revoked key: $($author.Name) " +
               "($(Get-SealFingerprint $author.Public)) was revoked on $($author.Revoked) by $($author.RevokedBy)")
    }
    if ($author.Name -cne $Change.Author) {
        throw ("trust: this change was signed by a key this station does not trust: the signature is " +
               "$($author.Name)'s and the change claims to be from '$($Change.Author)'")
    }

    $input = Get-TrustSigningInput $script:TrustDomain (Get-TrustChangeCanonical $Change)
    if (-not $Change.Sig -or $Change.Sig.Length -ne 64 -or
        -not [Chaos.NaCl.Ed25519]::Verify($Change.Sig, $input, $Change.AuthorKey.SignPublic)) {
        throw ("trust: the signature on this change does not verify: it claims to be from " +
               "$($Change.Author) ($(Get-SealFingerprint $Change.AuthorKey))")
    }

    # --- THE ANCHOR RULE -----------------------------------------------------
    #
    # This is the assertion the whole claim rests on. A compromised key may evict
    # every other engineer; it may not evict the anchor, so the estate owner is
    # never locked out of their own machine by anything remote, and recovery is a
    # signed change from whoever holds the anchor rather than a site visit.
    #
    # CHECKED ON BOTH THE NAME AND THE KEY. Matching only the name lets a change
    # revoke the anchor's KEY under a different name; matching only the key lets
    # a change add a second member CALLED the anchor, after which "revoke owner"
    # is ambiguous and resolves in whichever direction the reader was not
    # expecting.
    #
    # There is no flag, no override and no privileged author that gets past this.
    # Not even the anchor's own key: the anchor changes with a command run on the
    # machine, and nothing arriving over a transport reaches it.
    #
    # THE SENTENCE IS WORD FOR WORD THE GO SIDE'S, including the double quotes
    # Go's %q produces. tests/trust-vectors.ps1 compares the WHOLE refusal, not
    # a substring, because two stations giving different reasons for the same
    # decision is a support call nobody can close - and it found exactly that:
    # this message named `heliograph-seal trust anchor`, a binary this station
    # does not have and never will.
    if ($Change.Name -ceq $Set.Anchor.Name) {
        throw ('trust: the anchor is changeable only on the machine: "' + $Change.Name + '" is the anchor, ' +
               "and it changes only on the machine itself, with the station's own trust anchor command")
    }
    if (Test-TrustSamePublic $Change.Subject $Set.Anchor.Public) {
        throw ("trust: the anchor is changeable only on the machine: " +
               "$(Get-SealFingerprint $Set.Anchor.Public) is the anchor's key, " +
               "and it changes only on the machine itself, with the station's own trust anchor command")
    }

    $members = New-Object System.Collections.Generic.List[object]
    foreach ($m in $Set.Members) {
        [void] $members.Add((New-TrustMember -Name $m.Name -Public $m.Public `
            -Added $m.Added -AddedBy $m.AddedBy -Revoked $m.Revoked -RevokedBy $m.RevokedBy))
    }

    $stamp = $Change.UTC
    if (-not $stamp) { $stamp = $Now.ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ') }

    if ($Change.Op -ceq 'add') {
        foreach ($m in $members) {
            if (Test-TrustSamePublic $m.Public $Change.Subject) {
                if (-not (Test-TrustMemberActive $m)) {
                    # A revoked key stays revoked. Re-adding one would let a single
                    # compromised key resurrect the leaver whose access was removed,
                    # which is the flow revocation exists for.
                    throw ("trust: that key was revoked and may not be added again: $($m.Name) " +
                           "($(Get-SealFingerprint $m.Public)) was revoked on $($m.Revoked): a returning member enrols a new key")
                }
                throw "trust: that name or key is already in the set: $($m.Name) already holds $(Get-SealFingerprint $m.Public)"
            }
            if ($m.Name -ceq $Change.Name -and (Test-TrustMemberActive $m)) {
                throw "trust: that name or key is already in the set: '$($Change.Name)' is already a member"
            }
        }
        [void] $members.Add((New-TrustMember -Name $Change.Name -Public $Change.Subject `
            -Added $stamp -AddedBy $Change.Author))
    } else {
        $found = $false
        foreach ($m in $members) {
            if (-not (Test-TrustSamePublic $m.Public $Change.Subject)) { continue }
            $found = $true
            if (-not (Test-TrustMemberActive $m)) {
                throw ("trust: that key is not in the set: $($m.Name) ($(Get-SealFingerprint $Change.Subject)) " +
                       "was already revoked on $($m.Revoked) by $($m.RevokedBy)")
            }
            $m.Revoked = $stamp
            $m.RevokedBy = $Change.Author
        }
        if (-not $found) {
            throw "trust: that key is not in the set: no member holds $(Get-SealFingerprint $Change.Subject)"
        }
    }

    return [pscustomobject]@{
        Version = $Set.Version; Estate = $Set.Estate; Station = $Set.Station
        Serial = $Change.Serial
        UTC = $Now.ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
        Anchor = $Set.Anchor
        Members = $members
    }
}

# Get-TrustMembersLine is the one-line audit view the status carries.
#
# ONE LINE, because the status document is `key: value` read with sed on the far
# side, and a newline in a value forges a second key.
function Get-TrustMembersLine {
    param($Set)
    $parts = @()
    foreach ($m in (Get-TrustEveryone $Set)) {
        $s = "$($m.Name)=$(Get-SealFingerprint $m.Public)"
        if (-not (Test-TrustMemberActive $m)) { $s += ' REVOKED' }
        $parts += $s
    }
    return ($parts -join ' ')
}

Export-ModuleMember -Function @(
    'Import-TrustSet', 'Import-TrustSetFile', 'Export-TrustSet', 'Save-TrustSet',
    'Get-TrustSetDigest', 'Get-TrustSetCanonical', 'Get-TrustSigningInput',
    'Import-TrustChange', 'Get-TrustChangeCanonical',
    'Invoke-TrustApply', 'Get-TrustMembersLine', 'Get-TrustEveryone',
    'Find-TrustMember', 'Find-TrustMemberBySigning', 'Test-TrustMemberActive',
    'Test-TrustSamePublic', 'Test-TrustName'
)
