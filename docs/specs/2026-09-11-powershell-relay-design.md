# The PowerShell relay: what I got wrong, what is actually in the way

**Status: research and a recommendation. Nothing is built.**

The PowerShell station ships git and share. The [transports
page](../../site/content/transports.md), the [Windows
page](../../site/content/windows.md) and `PLAN.md` all say it has no relay
because the relay needs `heliograph-seal`, a native Go binary, and the estates
this payload exists for will not let you install one.

**That reasoning was wrong**, and this document exists to correct it before
anybody builds on it.

## What I claimed, and what is actually true

I said the alternative to the binary was hand-rolling four primitives in
PowerShell, which would be worse than not having the transport. That rested on
an unstated assumption - that the primitives are not available to a .NET
Framework process - and I did not check it.

### .NET itself: I was right, and it does not matter

Measured on this machine, PowerShell 7.6.5 / .NET 10.0.11:

| | .NET 10 | Windows PowerShell 5.1 (.NET Framework) |
|---|---|---|
| ChaCha20-Poly1305 | present | absent (.NET 5+) |
| HKDF | present | absent (.NET 5+) |
| X25519 | absent | absent |
| Ed25519 | absent | absent |

`dotnet/runtime#63174`, *"[API Proposal]: Add support for Ed25519"*, is
**`api-approved`** and milestoned **Future** - designed, agreed, not shipped and
not scheduled. The original proposal `#14741` was closed as completed in favour
of it. So Ed25519 is coming to .NET and is not there yet, on any version.

### Windows itself has three of the four

Asked before reaching for a library, and it should have been asked first.

| primitive | CNG identifier | supported from |
|---|---|---|
| ChaCha20-Poly1305 | `BCRYPT_CHACHA20_POLY1305_ALGORITHM`, `L"CHACHA20_POLY1305"` | Windows 10 |
| HKDF | `BCRYPT_HKDF_ALGORITHM`, `L"HKDF"` | Windows 10 |
| X25519 | `BCRYPT_ECDH_ALGORITHM` with `BCRYPT_ECC_CURVE_NAME` set to `BCRYPT_ECC_CURVE_25519` - name `curve25519`, 255 bits | Windows 10 / Server 2016 |
| **Ed25519** | **not in the list** | - |

Server 2016 is older than the floor this payload already targets, so on any
machine that can run this station at all, three of the four primitives are in
the operating system. No package, no assembly, nothing to get approved.

**Ed25519 is the only gap.** It is not an algorithm identifier, and
`BCRYPT_ECDSA_ALGORITHM` over `curve25519` is not a substitute: ECDSA on that
curve is a different signature scheme from Ed25519 and would not verify against
anything the Go side produces.

**THIS IS DOCUMENTED API SURFACE, NOT A MEASUREMENT.** Everything above comes
from Microsoft's own reference pages; none of it has been run, because there is
no Windows here. Two things in particular have to be proved on a real machine
before anybody builds on them:

- whether CNG's `curve25519` ECDH yields the **raw** RFC 7748 shared secret.
  `BCryptSecretAgreement` returns a handle and `BCryptDeriveKey` decides what
  comes out; `BCRYPT_KDF_RAW_SECRET` is the one that must be used and must
  match what Go's `crypto/ecdh` produces for the same keys.
- whether the ChaCha20-Poly1305 provider takes the nonce and AAD the way RFC
  8439 specifies, byte for byte.

Getting either wrong is silent. This document recommends measuring them as the
first task, not trusting this table - which is the whole lesson of the section
above it.

### BouncyCastle: all four, pure managed, measured

`bcgit/bc-csharp` - MIT, 1,920 stars, last pushed 2026-09-07 - carries
`Ed25519`, `X25519`, `ChaCha20Poly1305` and `HkdfBytesGenerator`. The
`BouncyCastle.Cryptography` 2.7.0 package targets **`net461`, `netstandard2.0`
and `net6.0`**, and the repository contains **no committed native binaries**: it
is managed code, one assembly, 8.3 MB packaged.

`net461` and `netstandard2.0` are both loadable by Windows PowerShell 5.1.

Run from PowerShell against the `netstandard2.0` assembly:

```
X25519 shared secrets agree : True
HKDF-SHA256 derived 32 bytes: True
ChaCha20-Poly1305 round trip : True
Ed25519 sign and verify      : True
```

**So a PowerShell relay does not need a native binary**, whichever route is
taken. Every primitive the seal uses is available to the floor version.

### So the only library needed is an Ed25519

CNG covers three. The question is what supplies the fourth, and the candidates
were measured **against the standards' own test vectors** rather than
round-tripped against themselves.

| | DLL | licence | targets | Ed25519 vs RFC 8032 |
|---|---|---|---|---|
| **`Rebex.Elliptic.Ed25519`** | 172 KB | **public domain** | net20, net40, netstandard2.0 | **matches** |
| **`NetSparkleUpdater.Chaos.NaCl`** | 172 KB | **MIT**, maintained | net462, netstandard2.0 | **matches** |
| `BouncyCastle.Cryptography` | 8.3 MB pkg | MIT | net461, netstandard2.0 | matches |
| `NaCl.Core` | 0.23 MB pkg | MIT | net45, netstandard2.0 | **has no Ed25519** - ChaCha20-Poly1305 and Poly1305 only |
| original `Chaos.NaCl` | source | **none declared**, no commit since 2021 | - | - |

Both of the top two are the same code: Christian Winnerlein's C# port of djb's
**ref10** from SUPERCOP, which is the reference implementation. Rebex's licence
file says so in as many words. That is good provenance for the one primitive on
the signature path, and either is about **48 times smaller than BouncyCastle**.

Measured, not assumed - RFC 8032 section 7.1 TEST 1, the empty message:

```
Chaos.NaCl  public key matches RFC 8032 : True
Chaos.NaCl  signature matches RFC 8032  : True
```

### A trap in the same library, and the reason to measure against a standard

`Chaos.NaCl` also exposes `MontgomeryCurve25519`, which looks like an X25519 to
reach for if CNG disappoints. **Its `KeyExchange` is not X25519.**

Against RFC 7748 section 6.1:

```
public key matches RFC 7748        : True
KeyExchange == RAW X25519 secret   : False
  KeyExchange gave : 1b27556473e985d462cd51197a9a46c76009549eac6474f206c4ee0844f68389
  RFC 7748 raw is  : 4a5d9d5ba4ce2de1728e3bf480350f25e07e21c947d19e3376f09b3c1e161742
```

It returns NaCl's `crypto_box_beforenm` - the raw secret run through HSalsa20 -
which is correct for NaCl and wrong for this. The public key is right, so
enrolment would succeed and every message would then fail to open, or, if both
sides used the same library, would work while diverging from the protocol.

**BouncyCastle's `X25519Agreement` does give the raw secret**, and matches RFC
7748 exactly. So the fallback, if CNG's `curve25519` turns out not to yield a
raw secret, is BouncyCastle rather than Chaos.NaCl.

This is why the vectors matter. An earlier version of this document said all
four primitives "work", on the strength of a round trip - two sides of the same
library agreeing with each other. That is not the property. The property is
agreeing with **Go**, and only a standard's own vectors show it.

## What is actually in the way

Not "the crypto does not exist". Three things, in descending order of how much
they should worry anybody.

### 1. Byte-compatibility with the Go implementation

The seal is not four primitives in a bag. `internal/seal/seal.go` also
specifies:

- **Sign-then-encrypt**, so the signature travels inside the encryption
- **The recipient's fingerprint is a signed field**, which is the standard
  mitigation for sign-then-encrypt's re-encryption weakness
- **Length-prefixed canonical metadata** - a big-endian `uint64` length before
  each field, never a delimiter, because a delimiter that can appear inside a
  value lets two different messages serialise identically
- **`base64.RawURLEncoding`** for identities and fingerprints
- **A signed version field**, with no negotiation and no plaintext fallback

None of that is hard. All of it is exacting, and a mistake in any of it is
silent - which is the argument that produced the Go binary in the first place.
The framing is plain byte manipulation and reproduces fine in PowerShell; what
it needs is **cross-implementation test vectors**, which do not exist yet.

### 2. Ed25519, and only Ed25519

CNG has the other three. Whatever is chosen for the signature is the one piece
of third-party or hand-written crypto on the path, and it is the piece where
being wrong is worst: a forged request is code execution inside the estate.

RFC 8032 ships official test vectors, so this is provable rather than trusted -
which is what makes vendoring one implementation acceptable where vendoring four
would not be.

### 3. The payload stops being plain text, if a library is shipped

The proposition is *"plain text you can read before you run it"*, and CI
enforces it: no Go, no binary and no package under `station/`.
`station/embed_test.go` caps the embedded payload at 4 MB and refuses anything
that is not part of the station - a guard written for a different reason that
would reject an 8.3 MB third-party assembly on both counts, correctly.

The CNG route avoids this entirely, which is most of why it is the
recommendation. A vendored Ed25519 is source, and readable, so it keeps the
property; an 8.3 MB assembly does not.

## Recommendation

**Use Windows CNG for X25519, HKDF and ChaCha20-Poly1305, and solve Ed25519 on
its own.** That is one gap rather than four, and it leaves the payload with no
third-party assembly to ship, approve or keep up to date.

Build it in this order, and do not start at the transport.

1. **Prove the three CNG primitives on a real Windows machine**, against Go's
   output for the same inputs. Specifically: `BCRYPT_KDF_RAW_SECRET` from a
   `curve25519` secret agreement must equal what `crypto/ecdh` gives. If it does
   not, this recommendation collapses and BouncyCastle is the answer - so this
   is the first task and not an afterthought.
2. **Golden test vectors from `internal/seal`**, emitted from fixed keys. The
   PowerShell implementation is written against those, and the Go side verifies
   what PowerShell produced. Two implementations of a crypto format that have
   never been compared are two formats.
3. **Decide Ed25519**, which is the only real choice left:

   | | |
   |---|---|
   | **`Rebex.Elliptic.Ed25519`** - 172 KB, public domain, targets back to net20 | the recommendation. One primitive, ref10 lineage, verified against RFC 8032 here |
   | **`NetSparkleUpdater.Chaos.NaCl`** - 172 KB, MIT, maintained | the same code with a declared licence and recent commits; pick this if "public domain" is harder to get past legal than MIT |
   | **Vendor the source** rather than the assembly | keeps the payload readable text, which is the property the whole product rests on. Both are permissively licensed, so this is allowed; it is more code to carry and the same vectors prove it |
   | **Require BouncyCastle after all** | 8.3 MB, and then the other three may as well come from it too. This is also the fallback if CNG's curve25519 does not yield a raw secret, because BouncyCastle's X25519 **does** match RFC 7748 and Chaos.NaCl's does not |
   | **Change the signature algorithm in a seal v2** | `Version` is already a signed field with no negotiation, so a v2 is possible - but it is a protocol change on both sides and the deployed estate's identities are Ed25519. Not worth it to avoid one primitive |

4. **`lib/seal.psm1`**, the construction alone - no networking, exactly as
   `heliograph-seal` does sealing and leaves curl in the shell.
5. **`transports/relay.psm1`**, which is then an ordinary transport.
6. **Conformance over the relay**, with the existing stub, on both editions.

**One thing this does not escape.** P/Invoke into `bcrypt.dll` needs
`Add-Type`, and Constrained Language Mode refuses it. That is not an extra cost:
CLM already stops the whole station, because the capture is mostly .NET method
calls and `start.ps1` checks for it first. An estate in CLM has no station at
all, relay or otherwise.

## What I would tell a reviewer

The correction matters more than the plan. I recommended deferring this on a
reason that does not hold, and the reason did not hold because I asserted the
capability of a platform instead of measuring it - having just spent a day
finding defects that all had the same shape. The measurement took four minutes.
