# The PowerShell relay: what I got wrong, and what the seal actually needs

**Status: research and a recommendation. Nothing is built.**

The PowerShell station ships git and share. The [transports
page](../../site/content/transports.md), the [Windows
page](../../site/content/windows.md) and `PLAN.md` all said it has no relay
because the relay needs `heliograph-seal`, a native Go binary, and the estates
this payload exists for will not let you install one.

**That reasoning was wrong**, and this document exists to correct it before
anybody builds on it.

## What I claimed, and what is actually true

I said the alternative to the binary was hand-rolling four primitives in
PowerShell, which would be worse than not having the transport. That rested on
an unstated assumption - that the primitives are not available to a .NET
Framework process - and I did not check it.

### .NET itself does not have them, and that turned out not to matter

Measured on this machine, PowerShell 7.6.5 / .NET 10.0.11:

| | .NET 10 | Windows PowerShell 5.1 (.NET Framework) |
|---|---|---|
| ChaCha20-Poly1305 | present | absent (.NET 5+) |
| HKDF | present | absent (.NET 5+) |
| X25519 | absent | absent |
| Ed25519 | absent | absent |

`dotnet/runtime#63174`, *"[API Proposal]: Add support for Ed25519"*, is
**`api-approved`** and milestoned **Future** - designed, agreed, not shipped and
not scheduled. The original proposal `#14741` was closed in favour of it. So
Ed25519 is coming to .NET and is not there yet, on any version.

## All four are available in managed C#, and all four were verified

Not round-tripped. Checked against **the standards' own test vectors**, from
PowerShell, **on Linux** - which matters for a reason given below.

| primitive | where it comes from | licence | verified against |
|---|---|---|---|
| **X25519, raw** | `Chaos.NaCl` → `MontgomeryOperations.scalarmult` | MIT | **RFC 7748 §6.1** |
| **Ed25519** | `Chaos.NaCl` → `Ed25519` | MIT | **RFC 8032 §7.1 TEST 1** |
| **ChaCha20-Poly1305** | `NaCl.Core` | MIT | **RFC 8439 §2.8.2** |
| **HKDF-SHA256** | about 25 lines over `HMACSHA256`, which .NET Framework has | ours | **RFC 5869 TC1** |

```
X25519  scalarmult matches RFC 7748  : True
Ed25519 public key matches RFC 8032  : True
Ed25519 signature matches RFC 8032   : True
ChaCha20-Poly1305 ciphertext + tag   : True
HKDF-SHA256 PRK and OKM              : True
```

**The weight is 200 KB**: `Chaos.NaCl.dll` is 168 KB and `NaCl.Core.dll` is 32
KB. BouncyCastle would do all four in one package and is **8.3 MB**.

`Chaos.NaCl` is Christian Winnerlein's C# port of **djb's ref10 from SUPERCOP** -
the reference implementation - maintained as the NetSparkle fork, MIT, with
recent commits. `Rebex.Elliptic.Ed25519` is the same code with a thin wrapper
and a public-domain licence, if MIT is the harder conversation.

**NO P/INVOKE AND NO CNG, WHICH IS WORTH AS MUCH AS THE SIZE.** Everything above
is portable managed code, so it behaves identically on Linux - and the seal
becomes testable on an ordinary CI runner instead of only on a Windows one.
Given how much of this station's trouble has been Windows-only and silent, that
is the most valuable property on this page.

### Windows CNG would also do three of the four, and is now optional

For completeness, because it was the previous recommendation. Microsoft
documents `BCRYPT_CHACHA20_POLY1305_ALGORITHM` and `BCRYPT_HKDF_ALGORITHM` from
Windows 10, and X25519 through `BCRYPT_ECDH_ALGORITHM` with
`BCRYPT_ECC_CURVE_25519` from Windows 10 / Server 2016. Ed25519 is **not** a CNG
algorithm, and `BCRYPT_ECDSA_ALGORITHM` over `curve25519` is a different
signature scheme that would verify against nothing the Go side produces.

That route needs P/Invoke, works only on Windows, and leaves one primitive to
find anyway. **It is not the recommendation, and it is not a blocker either** -
which is the useful part, because whether
`BCryptDeriveKey(..., BCRYPT_KDF_RAW_SECRET, ...)` yields the raw RFC 7748
secret is a question that cannot be answered without a Windows machine, and the
decision no longer waits on it.

Measured here, so the limit is stated rather than assumed: `CngKey::Create`
throws *"Windows Cryptography Next Generation (CNG) is not supported on this
platform"*, and .NET 10's portable `ECDiffieHellman` refuses `curve25519` on
Linux outright.

## Two traps, and why the vectors are the whole point

### `Chaos.NaCl.MontgomeryCurve25519.KeyExchange` is not X25519

It is the obvious method to reach for and it is wrong. Against RFC 7748 §6.1:

```
public key matches RFC 7748        : True
KeyExchange == RAW X25519 secret   : False
  KeyExchange gave : 1b27556473e985d462cd51197a9a46c76009549eac6474f206c4ee0844f68389
  RFC 7748 raw is  : 4a5d9d5ba4ce2de1728e3bf480350f25e07e21c947d19e3376f09b3c1e161742
```

It returns NaCl's `crypto_box_beforenm` - the raw secret run through HSalsa20 -
which is correct for NaCl and wrong for this. **The public key is right**, so
enrolment would succeed and every message would then fail to open; or, if both
sides used the same library, it would work while silently diverging from the
protocol.

`MontgomeryOperations.scalarmult` is the raw one. It is public, and it clamps
the scalar internally - verified by passing both a clamped and an unclamped
scalar and getting the same, correct answer.

### An earlier version of this document claimed all four "work"

On the strength of a **round trip**: two sides of one library agreeing with each
other. That is not the property. The property is agreeing with **Go**, and only
a standard's own vectors show it. The first trap above passes a round trip
perfectly.

And when the HKDF vector failed, the bug was in **my transcription** - a 21-byte
IKM where RFC 5869 says 22 - not in the code. A round trip would have reported
success either way.

## What is actually in the way

Not the crypto. Two things.

### 1. Byte-compatibility with the Go implementation

The seal is not four primitives in a bag. `internal/seal/seal.go` also
specifies:

- **Sign-then-encrypt**, so the signature travels inside the encryption
- **The recipient's fingerprint is a signed field**, the standard mitigation for
  sign-then-encrypt's re-encryption weakness
- **Length-prefixed canonical metadata** - a big-endian `uint64` length before
  each field, never a delimiter, because a delimiter that can appear inside a
  value lets two different messages serialise identically
- **`base64.RawURLEncoding`** for identities and fingerprints
- **A signed version field**, with no negotiation and no plaintext fallback

None of that is hard. All of it is exacting, and a mistake in any of it is
silent - which is the argument that produced the Go binary. What it needs is
**cross-implementation test vectors**, which do not exist yet.

### 2. Whether the payload ships an assembly or its source

The proposition is *"plain text you can read before you run it"*, and CI
enforces it: `station/embed_test.go` caps the embedded payload at 4 MB and
refuses anything that is not part of the station. Two 200 KB DLLs would pass the
size check and fail the second, correctly - a binary in the payload is exactly
what that guard is for.

**Vendoring the source keeps the property**, and both libraries permit it. The
cost is measured: **5,203 lines** for Chaos.NaCl's Ed25519 and Montgomery core,
10,672 for the whole library, plus NaCl.Core's ChaCha20-Poly1305. That is a real
weight, and it is readable C# implementing a published standard with published
vectors, which is a different thing from an opaque blob.

## Recommendation

**Use the managed libraries, vendored as source. Do not write the curve
arithmetic, and do not reach for CNG.**

Writing X25519 or Ed25519 by hand means constant-time field arithmetic with a
silent, catastrophic failure mode and no upside over djb's own reference
implementation - which is what `Chaos.NaCl` already is.

Build it in this order, and **the transport is last**:

1. **Golden test vectors from `internal/seal`**, emitted from fixed keys and
   committed. Two implementations of a crypto format that have never been
   compared are two formats.
2. **Vendor `Chaos.NaCl`'s Ed25519 + Montgomery core and `NaCl.Core`'s
   ChaCha20-Poly1305**, with their licences, and the four RFC vectors above as
   tests that run on every build. If a vendored file is ever touched, the
   vectors say so.
3. **`lib/seal.psm1`** - the construction alone, no networking, exactly as
   `heliograph-seal` does sealing and leaves curl in the shell. Written against
   the vectors from step 1, and verified in both directions: Go opens what
   PowerShell sealed, and PowerShell opens what Go sealed.
4. **`transports/relay.psm1`**, which is then an ordinary transport.
5. **Conformance over the relay**, with the existing stub, on both editions.

Steps 1 to 3 are testable on Linux, which is the point of choosing managed code
over CNG.

**One thing this does not escape.** `Add-Type` is needed to compile or load the
vendored code, and Constrained Language Mode refuses it. That is not an extra
cost: CLM already stops the whole station, because the capture is mostly .NET
method calls and `start.ps1` checks for it first. An estate in CLM has no
station at all, relay or otherwise.

## Reproducing the measurements

Every figure above came from running the libraries from PowerShell against the
published vectors. To repeat it: fetch `NetSparkleUpdater.Chaos.NaCl` and
`NaCl.Core` from NuGet, extract the `netstandard2.0` assemblies, and check

- `MontgomeryOperations.scalarmult` against RFC 7748 §6.1
- `Ed25519.PublicKeyFromSeed` and `Ed25519.Sign` against RFC 8032 §7.1 TEST 1
- `ChaCha20Poly1305.Encrypt` against RFC 8439 §2.8.2
- an HKDF over `HMACSHA256` against RFC 5869 Test Case 1

Those four become the vendored code's tests in step 2, so the measurement stops
being a note in a document and starts being something CI keeps true.

## What I would tell a reviewer

The correction matters more than the plan. I recommended deferring this on a
reason that does not hold, and the reason did not hold because I asserted the
capability of a platform instead of measuring it - having just spent a day
finding defects that all had that shape. The measurement took four minutes.

Then I named one library and stopped, which was the same mistake one level down.
Then I reported four primitives as working on the strength of a round trip,
which is the same mistake a third time, in the place it would have been most
expensive.
