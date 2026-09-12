#!/usr/bin/env pwsh
# =============================================================================
#  seal-vectors.ps1 - the PowerShell seal, against fixed bytes it did not make
# =============================================================================
#  Two sets of vectors, and neither is a round trip.
#
#  1. THE STANDARDS' OWN, for each primitive: RFC 7748 6.1, RFC 8032 7.1 TEST 1,
#     RFC 8439 2.3.2 / 2.4.2 / 2.5.2 / 2.8.2, and RFC 5869 TC1.
#
#  2. THE GO SIDE'S, from tests/fixtures/seal-vectors.json, for the whole
#     construction: it opens what Go sealed, and it re-seals with Go's own
#     ephemeral key and nonce and must produce Go's bytes exactly.
#
#  A ROUND TRIP WOULD PASS ALL OF THIS AND PROVE NOTHING. This repository has
#  the receipt: Chaos.NaCl's MontgomeryCurve25519.KeyExchange round-trips
#  perfectly with itself, returns crypto_box_beforenm rather than the raw RFC
#  7748 secret, and would have produced a station that enrolled cleanly and
#  could not open one message.
#
#  Run directly, or through tests/test-seal-ps1.sh which also compiles the
#  whole tree as C# 5 against net48 - the compiler Windows PowerShell 5.1 uses.
# =============================================================================
Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$root = Split-Path -Parent $here
Import-Module (Join-Path $root 'station/powershell/lib/seal.psm1') -Force

# NOT NAMED H. `h` is the built-in alias for Get-History, and a function named
# H is shadowed by it in a way that reports "cannot convert to System.Int64"
# from a line that contains no numbers.
function FromHex {
    param([string] $s)
    $s = $s -replace '[^0-9a-fA-F]', ''
    $b = New-Object byte[] ($s.Length / 2)
    for ($i = 0; $i -lt $b.Length; $i++) { $b[$i] = [Convert]::ToByte($s.Substring($i * 2, 2), 16) }
    return , $b
}
function ToHex { param([byte[]] $b) return (([BitConverter]::ToString($b)) -replace '-', '').ToLowerInvariant() }

$script:pass = 0; $script:fail = 0
function Check {
    param([string] $name, [string] $want, [string] $got)
    if ($want.ToLowerInvariant() -ceq $got.ToLowerInvariant()) {
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

# =============================================================================
#  1. EACH PRIMITIVE, AGAINST ITS OWN STANDARD
# =============================================================================
$alicePriv = FromHex '77076d0a7318a57d3c16c17251b26645df4c2f87ebc0992ab177fba51db92c2a'
$bobPub = FromHex 'de9edb7d7b7dc1b4d35b61c2ece435373f8343c85b78674dadfc7e146f882b4f'
$q = New-Object byte[] 32
[Chaos.NaCl.Internal.Ed25519Ref10.MontgomeryOperations]::scalarmult($q, 0, $alicePriv, 0, $bobPub, 0)
Check 'RFC 7748 6.1   X25519 shared secret' '4a5d9d5ba4ce2de1728e3bf480350f25e07e21c947d19e3376f09b3c1e161742' (ToHex $q)
Check 'RFC 7748 6.1   X25519 public key' '8520f0098930a754748b7ddcb43ef75a0dbf3a0d26381af4eba4a98eaa9b4e6a' `
  (ToHex (Get-SealX25519Public $alicePriv))

# THE TRAP, PINNED BY ABSENCE. See vendor/Chaos.NaCl/ORIGIN.md. An unused wrong
# function is one autocomplete away from being the used one.
CheckTrue 'Ed25519.KeyExchange is not in the payload (it is crypto_box_beforenm, not X25519)' `
  (-not ([Chaos.NaCl.Ed25519].GetMethods() | Where-Object { $_.Name -ceq 'KeyExchange' }))

$seed = FromHex '9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60'
Check 'RFC 8032 7.1   Ed25519 public key' 'd75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a' `
  (ToHex ([Chaos.NaCl.Ed25519]::PublicKeyFromSeed($seed)))
$expanded = [Chaos.NaCl.Ed25519]::ExpandedPrivateKeyFromSeed($seed)
$sig = [Chaos.NaCl.Ed25519]::Sign((New-Object byte[] 0), $expanded)
Check 'RFC 8032 7.1   Ed25519 signature' ('e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e0652249015' +
    '55fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b') (ToHex $sig)

$k = FromHex '000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f'
Check 'RFC 8439 2.3.2 ChaCha20 block' ('10f1e7e4d13b5915500fdd1fa32071c4c7d1f4c733c068030422aa9ac3d46c4e' +
    'd282644 6079faa0914c2d705d98b02a2b5129cd1de164eb9cbd083e8a2503c4e'.Replace(' ', '')) `
  (ToHex ([Heliograph.Seal.ChaCha20Poly1305]::Block($k, 1, (FromHex '000000090000004a00000000'))))

$pt = [System.Text.Encoding]::ASCII.GetBytes(
    "Ladies and Gentlemen of the class of '99: If I could offer you only one tip for the future, sunscreen would be it.")
Check 'RFC 8439 2.4.2 ChaCha20 keystream' ('6e2e359a2568f98041ba0728dd0d6981e97e7aec1d4360c20a27afccfd9fae0b' +
    'f91b65c5524733ab8f593dabcd62b3571639d624e65152ab8f530c359f0861d807ca0dbf500d6a61' +
    '56a38e088a22b65e52bc514d16ccf806818ce91ab77937365af90bbf74a35be6b40b8eedf2785e42' +
    '874d') (ToHex ([Heliograph.Seal.ChaCha20Poly1305]::Xor($k, 1, (FromHex '000000000000004a00000000'), $pt)))

Check 'RFC 8439 2.5.2 Poly1305 tag' 'a8061dc1305136c6c22b8baf0c0127a9' `
  (ToHex ([Heliograph.Seal.ChaCha20Poly1305]::Poly1305Mac(
        (FromHex '85d6be7857556d337f4452fe42d506a80103808afb0db2fd4abff6af4149f51b'),
        [System.Text.Encoding]::ASCII.GetBytes('Cryptographic Forum Research Group'))))

$aeadKey = FromHex '808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9f'
$aeadNonce = FromHex '070000004041424344454647'
$aad = FromHex '50515253c0c1c2c3c4c5c6c7'
$out = [Heliograph.Seal.ChaCha20Poly1305]::Encrypt($aeadKey, $aeadNonce, $pt, $aad)
Check 'RFC 8439 2.8.2 AEAD ciphertext and tag' ('d31a8d34648e60db7b86afbc53ef7ec2a4aded51296e08fea9e2b5a736ee62d6' +
    '3dbea45e8ca9671282fafb69da92728b1a71de0a9e060b2905d6a5b67ecd3b3692ddbd7f2d778b8c' +
    '9803aee328091b58fab324e4fad675945585808b4831d7bc3ff4def08e4b7a9de576d26586cec64b' +
    '61161ae10b594f09e26a7e902ecbd0600691') (ToHex $out)
CheckTrue 'RFC 8439 2.8.2 AEAD decrypts back' `
  ((ToHex ([Heliograph.Seal.ChaCha20Poly1305]::Decrypt($aeadKey, $aeadNonce, $out, $aad))) -ceq (ToHex $pt))
$out[0] = $out[0] -bxor 1
CheckTrue 'RFC 8439 2.8.2 a tampered ciphertext returns null, not a plaintext' `
  ($null -eq [Heliograph.Seal.ChaCha20Poly1305]::Decrypt($aeadKey, $aeadNonce, $out, $aad))

Check 'RFC 5869 TC1   HKDF-SHA256 OKM' '3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf34007208d5b887185865' `
  (ToHex ([Heliograph.Seal.Hkdf]::DeriveKey((FromHex ('0b' * 22)), (FromHex '000102030405060708090a0b0c'),
            (FromHex 'f0f1f2f3f4f5f6f7f8f9'), 42)))

# =============================================================================
#  2. THE WHOLE CONSTRUCTION, AGAINST GO'S BYTES
# =============================================================================
$vf = Join-Path $root 'tests/fixtures/seal-vectors.json'
if (-not (Test-Path -LiteralPath $vf -PathType Leaf)) {
    "FAIL the Go vectors are missing from $vf"
    $script:fail++
} else {
    # ReadAllText, NOT `Get-Content -Raw`. On Windows PowerShell 5.1
    # Get-Content defaults to the ANSI CODEPAGE, so the UTF-8 bytes c3 a9 in
    # `café-01` came back as the two Windows-1252 characters `Ã©` and were then
    # re-encoded to c3 83 c2 a9. The canonical metadata was two bytes longer
    # than Go's, the derived key differed, and the signature differed - a
    # perfect mojibake cascade, on Windows only, in the one vector case with a
    # non-ASCII field. .NET's ReadAllText is UTF-8 with BOM detection on both
    # editions, and it is what the station itself uses to read a key file.
    $v = [System.IO.File]::ReadAllText($vf) | ConvertFrom-Json
    CheckTrue 'the vectors are for the version this module speaks' ($v.version -eq 1)

    $ids = @{
        control = Import-SealIdentity $v.control.secret
        station = Import-SealIdentity $v.station.secret
    }
    # THE IDENTITY DERIVATION FIRST. If the public keys differ, everything below
    # differs for one reason, and it is this one.
    Check 'identity: control X25519 public' $v.control.encPublicHex (ToHex $ids.control.EncPublic)
    Check 'identity: control Ed25519 public' $v.control.signPublicHex (ToHex $ids.control.SignPublic)
    Check 'identity: control encoded public' $v.control.public (Export-SealPublic $ids.control)
    Check 'identity: control fingerprint' $v.control.fingerprint (Get-SealFingerprint $ids.control)
    Check 'identity: station fingerprint' $v.station.fingerprint (Get-SealFingerprint $ids.station)

    foreach ($c in $v.cases) {
        $m = New-SealMeta -Estate $c.meta.estate -Station $c.meta.station -Dir $c.meta.dir `
            -Seq ([UInt64] $c.meta.seq) -Kind $c.meta.kind -Recipient $c.meta.recipient
        $from = $ids[$c.from]; $to = $ids[$c.to]
        $plain = FromHex $c.plaintextHex

        # EVERY STAGE, because "the ciphertext differs" is not a debuggable
        # statement about six chained primitives.
        Check "$($c.name): canonical metadata" $c.canonicalHex (ToHex (Get-SealCanonical $m))

        $eph = FromHex $c.ephemeralSecretHex
        Check "$($c.name): ephemeral public" $c.ephemeralPublicHex (ToHex (Get-SealX25519Public $eph))

        $shared = New-Object byte[] 32
        [Chaos.NaCl.Internal.Ed25519Ref10.MontgomeryOperations]::scalarmult(
            $shared, 0, $eph, 0, $to.EncPublic, 0)
        Check "$($c.name): X25519 shared secret" $c.sharedSecretHex (ToHex $shared)
        Check "$($c.name): derived key" $c.keyHex (ToHex (Get-SealKey $shared (Get-SealX25519Public $eph) $to.EncPublic $m))

        $sig = [Chaos.NaCl.Ed25519]::Sign(
            (Join-SealBytes @((Get-SealCanonical $m), $plain)), $from.SignSecret)
        Check "$($c.name): Ed25519 signature" $c.signatureHex (ToHex $sig)

        # AND THE WHOLE MESSAGE, byte for byte, with Go's own ephemeral key and
        # nonce. This is the assertion the file exists for.
        $sealed = Protect-Seal -From $from -To $to -Meta $m -Plaintext $plain `
            -EphemeralSecret $eph -Nonce (FromHex $c.nonceHex)
        Check "$($c.name): the sealed message matches Go byte for byte" $c.sealedHex (ToHex $sealed)

        # And the other direction: open what Go sealed.
        #
        # CAUGHT RATHER THAN THROWN. A refusal here is a failed assertion, not a
        # crashed test run: the file's whole value is telling you WHICH stage
        # disagreed, and a first mismatch that aborts the run hides the other
        # five cases and every refusal below.
        try {
            $opened = Unprotect-Seal -To $to -ExpectFrom $from -Meta $m -Sealed (FromHex $c.sealedHex)
            Check "$($c.name): opens Go's own bytes" $c.plaintextHex (ToHex $opened)
        } catch {
            $script:fail++
            "FAIL $($c.name): opens Go's own bytes"
            "     refused with: $($_.Exception.Message)"
        }
    }

    # THE REFUSALS. A vector file of things that must succeed teaches a port to
    # accept everything.
    foreach ($r in $v.mustRefuse) {
        $m = New-SealMeta -Estate $r.meta.estate -Station $r.meta.station -Dir $r.meta.dir `
            -Seq ([UInt64] $r.meta.seq) -Kind $r.meta.kind -Recipient $r.meta.recipient
        $refused = $false
        try {
            $null = Unprotect-Seal -To $ids.station -ExpectFrom $ids.control -Meta $m -Sealed (FromHex $r.sealedHex)
        } catch { $refused = $true }
        CheckTrue "refuses: $($r.name)" $refused
    }
}

''
"seal-vectors.ps1: $($script:pass) passed, $($script:fail) failed"
if ($script:fail -gt 0) { exit 1 }
exit 0
