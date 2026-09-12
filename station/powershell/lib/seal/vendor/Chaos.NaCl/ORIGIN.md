# Chaos.NaCl, vendored

Source, so the payload stays plain text you can read before you run it. A DLL
would pass the payload's size check and fail its second one, correctly.

| | |
|---|---|
| upstream | <https://github.com/NetSparkleUpdater/Chaos.NaCl> |
| commit | `5c50d743b46e14612b77a227bc0e9a2516744585`, 2025-11-26 |
| licence | MIT, in `LICENSE.md` beside this file |
| lineage | Christian Winnerlein's C# port of **djb's ref10 from SUPERCOP**, the reference implementation, maintained as the NetSparkle fork |

## What is here, and what is not

Taken: `Ed25519`, `Sha512`, `CryptoBytes`, the `Ed25519Ref10` core (which is
where `MontgomeryOperations.scalarmult` lives), and `Poly1305Donna`.

Left behind: `MontgomeryCurve25519`, `XSalsa20Poly1305`, `OneTimeAuth`,
`Poly1305`, `Ed25519Signer`, the project files and the assembly metadata.
Nothing here calls them.

## The one edit made to upstream source

**`Ed25519.KeyExchange` is deleted**, both overloads, and a comment in
`Ed25519.cs` says so at the point they were.

They return NaCl's `crypto_box_beforenm` - the raw X25519 secret run through
HSalsa20 - and not the raw RFC 7748 secret this seal derives its key from.
Against RFC 7748 §6.1 the public key matches and the secret does not, so a
station using them would enrol successfully and then fail to open every
message; or, if both ends used them, it would work while silently speaking a
protocol that is not the one Go speaks.

Deleted rather than left unused, because an unused wrong function in the
payload is one autocomplete away from being the used one, and no test catches a
call nobody has written yet. `tests/test-seal-vectors.ps1` asserts the method is
absent.

The raw one is `MontgomeryOperations.scalarmult`, in
`Internal/Ed25519Ref10/scalarmult.cs`. It clamps the scalar internally.

## What is checked, on every build

`tests/test-seal-ps1.sh` runs the vendored code against the standards' own
vectors - RFC 7748 §6.1 and RFC 8032 §7.1 TEST 1 - and refuses to pass on a
round trip. If a file here is ever touched, those say so.

It also compiles the whole tree as **C# 5 against net48**, because Windows
PowerShell 5.1's `Add-Type` uses the .NET Framework CodeDOM compiler. Every
file here is C# 5 already; that is a large part of why this library was chosen
and not the one the design first recommended.

## Refreshing it

Take the same files from a newer commit, delete `KeyExchange` again, update the
commit above, and run the vectors. If they still pass, the update is safe; if
they do not, the update is the problem.
