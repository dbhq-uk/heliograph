# =============================================================================
#  seal.psm1 - the end-to-end encryption, in PowerShell
# =============================================================================
#  The twin of `internal/seal/seal.go`, and of the `heliograph-seal` binary the
#  bash station shells out to. This payload exists for estates that will not let
#  you install a native binary, so the relay transport needs the construction
#  here or it does not exist for them at all.
#
#  IT IS A SECOND IMPLEMENTATION OF A CRYPTO FORMAT, which is the dangerous
#  kind of thing to write. Two implementations that have never been compared
#  are two formats, and the failure is silent: a station enrols, reports
#  success, and cannot open a single message. So this file is written against
#  `tests/fixtures/seal-vectors.json` - fixed bytes emitted by the Go side -
#  and every stage is checked separately, not just the output. See
#  `tests/test-seal-ps1.sh`.
#
#  THE CONSTRUCTION, and every line of it is load-bearing:
#
#    sig   = Ed25519(sender, canonical(meta) || plaintext)
#    eph   = a fresh X25519 keypair, per message
#    key   = HKDF-SHA256(ikm  = X25519(eph, recipient),
#                        salt = ephPublic || recipientPublic,
#                        info = "heliograph-seal-v1" || canonical(meta))
#    out   = senderSignPublic || ephPublic || nonce
#            || ChaCha20-Poly1305(key, nonce, sig || plaintext, aad=canonical)
#
#  SIGN THEN ENCRYPT, so the signature travels inside the encryption and the
#  relay cannot see who signed what. Its known weakness - a recipient
#  re-encrypting a validly signed message to a third party - is closed by
#  making the RECIPIENT'S FINGERPRINT a signed field.
#
#  THE METADATA IS LENGTH-PREFIXED, never delimited, with a big-endian uint64
#  before each field. A delimiter can appear inside a value, and then two
#  different metadatas serialise identically - which is how a signature comes
#  to cover something other than what it appears to.
#
#  WHERE THE PRIMITIVES COME FROM, and why not from .NET:
#
#    X25519, Ed25519   vendored Chaos.NaCl, which is djb's ref10. .NET has
#                      neither, on any version - dotnet/runtime#63174 is
#                      api-approved and milestoned Future.
#    Poly1305          vendored Chaos.NaCl (poly1305-donna). The 130-bit
#                      arithmetic is the part with a subtle failure mode.
#    ChaCha20, framing lib/seal/ChaCha20Poly1305.cs, written here. .NET
#                      Framework has no ChaCha20-Poly1305 - it arrived in .NET
#                      5 - and every managed library that does is built on
#                      Span<T>, which .NET Framework does not have.
#    HKDF-SHA256       lib/seal/Hkdf.cs, written here. Also .NET 5 and later.
#
#  ADD-TYPE IS REQUIRED, and Constrained Language Mode refuses it. That is not
#  an extra cost: CLM already stops the whole station, because the capture is
#  mostly .NET method calls, and start.ps1 checks for it before anything else.
#  An estate in CLM has no station at all, relay or otherwise.
# =============================================================================
Set-StrictMode -Version 2.0

$script:SealRoot = Join-Path $PSScriptRoot 'seal'

# Compiled once per process. Add-Type throws on a second load of the same type,
# and the loop calls into this on every poll.
#
# -Path AND NOT -TypeDefinition. Each file is its own compilation unit, so the
# `using` clauses stay at the top of their own file where C# requires them;
# concatenating sixty files puts the second file's `using` after the first
# file's namespace and nothing compiles.
function Initialize-Seal {
    if ('Heliograph.Seal.ChaCha20Poly1305' -as [type]) { return }
    if (-not (Test-Path -LiteralPath $script:SealRoot -PathType Container)) {
        throw "the seal source is missing from $script:SealRoot. Re-plant this station: the relay transport cannot work without it"
    }
    $cs = @(Get-ChildItem -LiteralPath $script:SealRoot -Filter *.cs -Recurse |
            ForEach-Object { $_.FullName })
    if ($cs.Count -eq 0) { throw "no C# sources under $script:SealRoot" }
    Add-Type -Path $cs -ErrorAction Stop
}

# --- bytes, and the two encodings the format uses ----------------------------

# base64.RawURLEncoding: URL alphabet, NO padding. Not a detail - a `+` where
# the Go side wrote a `-` produces an identity that decodes to different bytes
# and a fingerprint that matches nothing.
function ConvertTo-SealBase64 {
    param([Parameter(Mandatory = $true)][byte[]] $Bytes)
    return ([Convert]::ToBase64String($Bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_'))
}

function ConvertFrom-SealBase64 {
    param([Parameter(Mandatory = $true)][string] $Text)
    $s = $Text.Trim().Replace('-', '+').Replace('_', '/')
    switch ($s.Length % 4) {
        2 { $s += '==' }
        3 { $s += '=' }
        1 { throw 'not a usable base64url string' }
    }
    return [Convert]::FromBase64String($s)
}

function Join-SealBytes {
    param([byte[][]] $Parts)
    $n = 0
    foreach ($p in $Parts) { if ($p) { $n += $p.Length } }
    $out = New-Object byte[] $n
    $o = 0
    foreach ($p in $Parts) {
        if (-not $p -or $p.Length -eq 0) { continue }
        [Array]::Copy($p, 0, $out, $o, $p.Length)
        $o += $p.Length
    }
    return , $out
}

function Get-SealSlice {
    param([byte[]] $Bytes, [int] $Offset, [int] $Length)
    $out = New-Object byte[] $Length
    [Array]::Copy($Bytes, $Offset, $out, 0, $Length)
    return , $out
}

# Big-endian, because the seal's canonical metadata is. RFC 8439's own length
# fields inside the AEAD are little-endian, and they are handled in the C#. The
# two are a metre apart and opposite, which is exactly why both have vectors.
function ConvertTo-SealUInt64BE {
    param([UInt64] $Value)
    $b = New-Object byte[] 8
    for ($i = 0; $i -lt 8; $i++) { $b[$i] = [byte](($Value -shr (8 * (7 - $i))) -band 0xFF) }
    return , $b
}

# --- identities ---------------------------------------------------------------

$script:SealVersion = 1
$script:X25519Base = [byte[]] @(9) + (New-Object byte[] 31)

function Get-SealX25519Public {
    param([byte[]] $Secret)
    Initialize-Seal
    $pub = New-Object byte[] 32
    # scalarmult, NOT KeyExchange. See vendor/Chaos.NaCl/ORIGIN.md - the other
    # one is crypto_box_beforenm and is deleted from the vendored source.
    [Chaos.NaCl.Internal.Ed25519Ref10.MontgomeryOperations]::scalarmult(
        $pub, 0, $Secret, 0, $script:X25519Base, 0)
    return , $pub
}

# An identity is 64 bytes: the X25519 secret then the Ed25519 SEED.
#
# TWO KEYPAIRS AND NOT ONE. X25519 cannot sign, Ed25519 cannot agree a key, and
# converting between them is the sort of clever that gets a design an
# unfavourable audit.
function Import-SealIdentity {
    param([Parameter(Mandatory = $true)][string] $Encoded)
    Initialize-Seal
    $b = ConvertFrom-SealBase64 $Encoded
    if ($b.Length -ne 64) { throw "an identity is 64 bytes, got $($b.Length)" }
    $enc = Get-SealSlice $b 0 32
    $seed = Get-SealSlice $b 32 32
    return [pscustomobject]@{
        EncSecret  = $enc
        EncPublic  = (Get-SealX25519Public $enc)
        SignSeed   = $seed
        SignSecret = [Chaos.NaCl.Ed25519]::ExpandedPrivateKeyFromSeed($seed)
        SignPublic = [Chaos.NaCl.Ed25519]::PublicKeyFromSeed($seed)
    }
}

function Import-SealPublic {
    param([Parameter(Mandatory = $true)][string] $Encoded)
    $b = ConvertFrom-SealBase64 $Encoded
    if ($b.Length -ne 64) { throw "a public identity is 64 bytes, got $($b.Length)" }
    return [pscustomobject]@{
        EncPublic  = (Get-SealSlice $b 0 32)
        SignPublic = (Get-SealSlice $b 32 32)
    }
}

function Export-SealPublic {
    param([Parameter(Mandatory = $true)] $Identity)
    return ConvertTo-SealBase64 (Join-SealBytes @($Identity.EncPublic, $Identity.SignPublic))
}

# --- the files on disk, whose shape is set by internal/seal/file.go -----------
#
# An identity file is JSON: {"version":1,"secret":"...","public":"..."}. It is
# NOT the bare base64 string, and reading it as one produces "the input is not a
# valid Base-64 string" from a file that is perfectly valid - which reads as a
# corrupt key rather than as a parser looking at the wrong thing.
#
# ConvertFrom-Json rather than a regex, but the property is fetched defensively:
# under Set-StrictMode a missing property throws rather than returning $null,
# and "the key file is wrong" must not arrive as a PropertyNotFound.
function Import-SealIdentityFile {
    param([Parameter(Mandatory = $true)][string] $Path)
    $txt = [System.IO.File]::ReadAllText($Path).Trim()
    if (-not $txt.StartsWith('{')) {
        throw "$Path is not an identity file. An identity is JSON with a `"secret`" field, written by 'heliograph-seal keygen --out <file>'"
    }
    $o = $txt | ConvertFrom-Json
    $secret = if ($o.PSObject.Properties['secret']) { $o.secret } else { '' }
    if (-not $secret) {
        throw "$Path holds a public identity and no secret: it is a PEER file, not an identity"
    }
    return Import-SealIdentity $secret
}

# A peer file may be a bare public identity on one line, or the JSON an identity
# file happens to be. Both are accepted for the reason file.go accepts both:
# somebody sends the whole file rather than the one line, and refusing a peer
# that is sitting right there helps nobody. The JSON is read for its PUBLIC half
# only.
function Import-SealPublicFile {
    param([Parameter(Mandatory = $true)][string] $Path)
    $txt = [System.IO.File]::ReadAllText($Path).Trim()
    if ($txt.StartsWith('{')) {
        $o = $txt | ConvertFrom-Json
        $pub = if ($o.PSObject.Properties['public']) { $o.public } else { '' }
        if ($pub) { return Import-SealPublic $pub }
        throw "$Path is JSON with no `"public`" field, so there is nothing to verify against"
    }
    return Import-SealPublic $txt
}

function New-SealIdentity {
    Initialize-Seal
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $b = New-Object byte[] 64
        $rng.GetBytes($b)
        return ConvertTo-SealBase64 $b
    } finally { $rng.Dispose() }
}

# Twelve hex characters in groups of four: long enough not to be collided
# deliberately, short enough to read aloud down a phone. That reading-aloud is
# the ceremony that closes the residual risk in enrolment, so the grouping is
# part of the format and not decoration.
function Get-SealFingerprint {
    param([Parameter(Mandatory = $true)] $Public)
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $data = Join-SealBytes @(
            [System.Text.Encoding]::UTF8.GetBytes('heliograph-identity-v1'),
            $Public.EncPublic, $Public.SignPublic)
        $h = $sha.ComputeHash($data)
    } finally { $sha.Dispose() }
    $hex = (([BitConverter]::ToString($h)) -replace '-', '').ToLowerInvariant().Substring(0, 12)
    return "$($hex.Substring(0,4))-$($hex.Substring(4,4))-$($hex.Substring(8,4))"
}

# --- the metadata, and why it is shaped like this ----------------------------
#
# Every field is signed, and each one answers one attack:
#
#   Estate     a message from one estate replayed into another
#   Station    a message for one station replayed to a different one
#   Dir        a message reflected back the way it came
#   Seq        an old request replayed to re-run something destructive
#   Recipient  surreptitious forwarding, sign-then-encrypt's known weakness
#
# There is no timestamp, deliberately: the recipient has to reconstruct this
# exactly in order to verify, and it cannot know a clock it did not read.
function New-SealMeta {
    param(
        [Parameter(Mandatory = $true)][string] $Estate,
        [Parameter(Mandatory = $true)][string] $Station,
        [Parameter(Mandatory = $true)][string] $Dir,
        [UInt64] $Seq = 0,
        [Parameter(Mandatory = $true)][string] $Kind,
        [Parameter(Mandatory = $true)][string] $Recipient
    )
    if ($Dir -cne 'c2s' -and $Dir -cne 's2c') { throw "direction must be c2s or s2c, got '$Dir'" }
    if (-not $Estate -or -not $Station -or -not $Recipient) {
        throw 'a message must name its estate, station and recipient'
    }
    if (-not $Kind) { throw 'a message must say what kind it is' }
    return [pscustomobject]@{
        Estate = $Estate; Station = $Station; Dir = $Dir
        Seq = $Seq; Kind = $Kind; Recipient = $Recipient
    }
}

function Get-SealCanonical {
    param([Parameter(Mandatory = $true)] $Meta)
    $parts = New-Object System.Collections.Generic.List[byte[]]
    $parts.Add((ConvertTo-SealUInt64BE ([UInt64] $script:SealVersion)))
    # LENGTH IN BYTES, not in characters. A station named `café-01` is eight
    # bytes and seven characters, and a port that counts characters produces a
    # signature the Go side rejects on exactly the estates most likely to have
    # one.
    $add = {
        param([string] $s)
        $u = [System.Text.Encoding]::UTF8.GetBytes($s)
        $parts.Add((ConvertTo-SealUInt64BE ([UInt64] $u.Length)))
        $parts.Add($u)
    }
    & $add $Meta.Estate
    & $add $Meta.Station
    & $add $Meta.Dir
    $parts.Add((ConvertTo-SealUInt64BE ([UInt64] $Meta.Seq)))
    & $add $Meta.Kind
    & $add $Meta.Recipient
    return Join-SealBytes $parts.ToArray()
}

function Get-SealKey {
    param([byte[]] $Shared, [byte[]] $EphPublic, [byte[]] $RecipientPublic, $Meta)
    $salt = Join-SealBytes @($EphPublic, $RecipientPublic)
    $info = Join-SealBytes @(
        [System.Text.Encoding]::UTF8.GetBytes('heliograph-seal-v1'),
        (Get-SealCanonical $Meta))
    return , ([Heliograph.Seal.Hkdf]::DeriveKey($Shared, $salt, $info, 32))
}

# --- seal and open -------------------------------------------------------------

# EphemeralSecret and Nonce exist for the vectors and for nothing else. A caller
# who could choose the nonce could reuse one, and nonce reuse under a fixed key
# reads the plaintext straight out - so they are not in the transport's path and
# the test that uses them says why.
function Protect-Seal {
    param(
        [Parameter(Mandatory = $true)] $From,
        [Parameter(Mandatory = $true)] $To,
        [Parameter(Mandatory = $true)] $Meta,
        [byte[]] $Plaintext = @(),
        [byte[]] $EphemeralSecret,
        [byte[]] $Nonce
    )
    Initialize-Seal
    $fp = Get-SealFingerprint $To
    if ($fp -cne $Meta.Recipient) {
        # Caught here rather than at the far side, where the refusal happens on
        # a machine nobody can reach.
        throw "Meta.Recipient is '$($Meta.Recipient)' but the key given is '$fp'"
    }

    $canon = Get-SealCanonical $Meta
    $sig = [Chaos.NaCl.Ed25519]::Sign((Join-SealBytes @($canon, $Plaintext)), $From.SignSecret)

    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        if (-not $EphemeralSecret) {
            $EphemeralSecret = New-Object byte[] 32
            $rng.GetBytes($EphemeralSecret)
        }
        if (-not $Nonce) {
            $Nonce = New-Object byte[] 12
            $rng.GetBytes($Nonce)
        }
    } finally { $rng.Dispose() }

    $ephPub = Get-SealX25519Public $EphemeralSecret
    $shared = New-Object byte[] 32
    [Chaos.NaCl.Internal.Ed25519Ref10.MontgomeryOperations]::scalarmult(
        $shared, 0, $EphemeralSecret, 0, $To.EncPublic, 0)
    # An all-zero shared secret is what a low-order or small-subgroup public key
    # produces. Go's ECDH returns an error for it; here it is checked by hand,
    # because nothing else will.
    if (-not (Test-SealNonZero $shared)) { throw 'unusable recipient key: the shared secret is all zeroes' }

    $key = Get-SealKey $shared $ephPub $To.EncPublic $Meta
    $body = Join-SealBytes @($sig, $Plaintext)
    $ct = [Heliograph.Seal.ChaCha20Poly1305]::Encrypt($key, $Nonce, $body, $canon)

    return Join-SealBytes @($From.SignPublic, $ephPub, $Nonce, $ct)
}

function Unprotect-Seal {
    param(
        [Parameter(Mandatory = $true)] $To,
        [Parameter(Mandatory = $true)] $ExpectFrom,
        [Parameter(Mandatory = $true)] $Meta,
        [Parameter(Mandatory = $true)][byte[]] $Sealed
    )
    Initialize-Seal
    $hdr = 32 + 32 + 12
    if ($Sealed.Length -lt ($hdr + 16 + 64)) { throw 'message is too short to be one' }

    $claimed = Get-SealSlice $Sealed 0 32
    $ephPub = Get-SealSlice $Sealed 32 32
    $nonce = Get-SealSlice $Sealed 64 12
    $ct = Get-SealSlice $Sealed $hdr ($Sealed.Length - $hdr)

    # Constant time, and BEFORE any cryptography is attempted with it. The value
    # is public; a comparison that returns early is a habit worth not having in
    # this file.
    if (-not [Heliograph.Seal.ChaCha20Poly1305]::ConstantTimeEquals($claimed, $ExpectFrom.SignPublic)) {
        throw 'the message claims a different sender than the one expected'
    }

    $shared = New-Object byte[] 32
    [Chaos.NaCl.Internal.Ed25519Ref10.MontgomeryOperations]::scalarmult(
        $shared, 0, $To.EncSecret, 0, $ephPub, 0)
    if (-not (Test-SealNonZero $shared)) { throw 'unusable ephemeral key' }

    $canon = Get-SealCanonical $Meta
    $key = Get-SealKey $shared $ephPub $To.EncPublic $Meta
    $body = [Heliograph.Seal.ChaCha20Poly1305]::Decrypt($key, $nonce, $ct, $canon)
    if ($null -eq $body) {
        # Deliberately unspecific. Which field disagreed is information a relay
        # probing the protocol would like, and nobody honest needs.
        throw 'could not open this message: it was not sealed for this identity, or it has been altered'
    }
    if ($body.Length -lt 64) { throw 'message carries no signature' }

    $sig = Get-SealSlice $body 0 64
    $plaintext = Get-SealSlice $body 64 ($body.Length - 64)
    if (-not [Chaos.NaCl.Ed25519]::Verify($sig, (Join-SealBytes @($canon, $plaintext)), $ExpectFrom.SignPublic)) {
        throw 'the signature does not verify: this message was not written by the expected sender'
    }
    return , $plaintext
}

function Test-SealNonZero {
    param([byte[]] $Bytes)
    $acc = 0
    foreach ($b in $Bytes) { $acc = $acc -bor $b }
    return ($acc -ne 0)
}

Export-ModuleMember -Function @(
    'Initialize-Seal',
    'ConvertTo-SealBase64', 'ConvertFrom-SealBase64',
    'New-SealIdentity', 'Import-SealIdentity', 'Import-SealPublic', 'Export-SealPublic',
    'Import-SealIdentityFile', 'Import-SealPublicFile',
    'Get-SealFingerprint', 'Get-SealX25519Public',
    'New-SealMeta', 'Get-SealCanonical', 'Get-SealKey', 'Join-SealBytes', 'Get-SealSlice',
    'Protect-Seal', 'Unprotect-Seal'
)
