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

### BouncyCastle: all four, pure managed, measured

`bcgit/bc-csharp` - MIT, 1,920★, last pushed 2026-09-07 - carries `Ed25519`,
`X25519`, `ChaCha20Poly1305` and `HkdfBytesGenerator`. The
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

**So a PowerShell relay does not need a native binary.** Every primitive the
seal uses is available to the floor version, in managed code, under a licence
that permits redistribution.

## What is actually in the way

Three things, none of which is "the crypto does not exist".

### 1. The payload stops being plain text

The proposition is *"plain text you can read before you run it"*, and CI
enforces it: no Go, no binary and no package under `station/`. `station/embed_test.go`
caps the embedded payload at 4 MB and refuses anything that is not part of the
station - a guard written for a different reason that would reject an 8.3 MB
third-party assembly on both counts.

That guard is right, and this would be the second deliberate exception to it.
The first - `heliograph-seal` - was argued for explicitly rather than smuggled
in, and this needs the same treatment rather than a quiet limit bump.

**A managed DLL is a genuinely easier ask than a native one.** It is MIT, it is
already present in a great many estates, and it comes from a publisher a change
board has heard of. That is a different conversation from an unsigned executable
we compiled ourselves, and it is the part I got wrong.

### 2. Add-Type, and the estate that blocks it

Loading the assembly needs `Add-Type -Path` or
`[Reflection.Assembly]::LoadFrom`. **Constrained Language Mode refuses both.**

This is not an additional obstacle, and it is worth being precise rather than
alarmed: CLM already stops the whole PowerShell station dead, because the
capture is mostly .NET method calls. `start.ps1` checks for it first and says
so. An estate in CLM has no station at all, relay or otherwise.

So CLM does not argue against the relay. It argues for the preflight naming the
assembly as one more thing it checks before starting.

### 3. Byte-compatibility with the Go implementation is the real work

The seal is not just four primitives. `internal/seal/seal.go` also specifies:

- **Sign-then-encrypt**, so the signature travels inside the encryption
- **The recipient's fingerprint is a signed field**, which is the standard
  mitigation for sign-then-encrypt's re-encryption weakness
- **Length-prefixed canonical metadata** - big-endian `uint64` length before
  each field, never a delimiter, because a delimiter that can appear inside a
  value lets two different messages serialise identically
- **`base64.RawURLEncoding`** for identities and fingerprints
- **A signed version field**, with no negotiation and no plaintext fallback

None of that is hard. All of it is exacting, and a mistake in any of it is
silent - which is the same argument that produced the Go binary in the first
place. The framing is plain byte manipulation and reproduces fine in PowerShell;
what it needs is **cross-implementation test vectors**, not cleverness.

## Recommendation

Build it, in this order, and do not start at the transport.

1. **Test vectors first.** `internal/seal` gains a golden-vector test that emits
   a fixed set of sealed messages from known keys. The PowerShell
   implementation is written against those vectors before it is wired to
   anything, and the Go side verifies what PowerShell produced. Two
   implementations of a crypto format that have never been compared are two
   formats.
2. **`lib/seal.psm1`**, the construction alone - no networking, exactly as
   `heliograph-seal` does sealing and leaves curl in the shell.
3. **`transports/relay.psm1`**, which is then an ordinary transport.
4. **Conformance over the relay**, with the existing stub, on both editions.

**How to ship BouncyCastle is the decision this document cannot make**, because
it changes what the payload is. The options, with what each costs:

| | |
|---|---|
| **Require it to be present** and refuse to start without it | keeps the payload plain text; makes the relay unavailable until somebody installs a DLL, which is the problem we started with |
| **Ship the assembly in the payload** | works immediately; an 8.3 MB binary blob in a payload whose proposition is that you can read it |
| **Vendor the needed source** (MIT permits it) and compile with `Add-Type` at runtime | payload stays readable text; several thousand lines of somebody else's crypto to carry, and a compile step at station start |

My recommendation is the **first**, with the preflight naming the assembly and
where to get it - because it preserves the property the whole product rests on,
and because an estate that will permit a relay at all is an estate having a
conversation about egress anyway. The third is the interesting one if that
proves too slow in practice, and it should not be reached for first.

## What I would tell a reviewer

The correction matters more than the plan. I recommended deferring this on a
reason that does not hold, and the reason did not hold because I asserted the
capability of a platform instead of measuring it - having just spent a day
finding defects that all had the same shape. The measurement took four minutes.
