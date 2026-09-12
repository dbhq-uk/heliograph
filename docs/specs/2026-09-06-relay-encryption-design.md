# C1 - the relay protocol and its cryptography

**Date:** 2026-09-06
**Status:** design, not yet approved
**Parent:** [2026-09-06-heliograph-next-design.md](2026-09-06-heliograph-next-design.md)

The relay is the only transport that needs no estate infrastructure at all: both sides dial out over ordinary HTTPS, and nothing has to be provisioned. That makes it the flagship, and it also makes DBHQ a party in every customer's estate. This document says exactly what that party can and cannot do.

## The requirement, stated correctly

The obvious framing is "the relay must not read your logs". That is the smaller half.

The question that decides whether this product is safe to exist is: **can a compromised relay make a station run something?**

If it can, then compromising `relay.heliograph.dbhq.uk` yields code execution inside every customer estate at once, through a channel the customer installed deliberately and trusts. That is a worse position than any log disclosure, and it is the failure that would end the project.

> **The hostname here is the one this document planned for, not the one that shipped.** The relay is at `heliograph-relay.dbhq.uk`; `relay.heliograph.dbhq.uk` has never resolved. The argument is unaffected and the name is left as written.

So the requirements are ordered:

| | requirement | |
|---|---|---|
| **R1** | the relay cannot cause a station to run anything | **authenticity, first** |
| R2 | the relay cannot tamper with a request, status, progress or log | integrity |
| R3 | the relay cannot read request, status, progress or log content | confidentiality |
| R4 | a credential stolen from one station cannot forge requests, or impersonate another station | containment |
| R5 | the relay cannot replay an old request | freshness |
| R6 | the relay cannot force a weaker or plaintext mode | no downgrade |
| R7 | DBHQ can support a precise, checkable claim about what it holds | auditability |

R1 is why a shared-passphrase design was rejected. A symmetric key that a station holds is a key that can forge control's instructions, so any station compromise becomes an estate compromise.

## Threat model

**Defended against**

- a curious or subpoenaed relay operator, including DBHQ
- a full compromise of the relay: server, database, object storage, TLS keys
- a malicious relay that actively forges, tampers, reorders, replays, drops or delays
- a station credential stolen from a machine you cannot reach and do not trust
- a stolen control credential, for everything except the estate it belongs to
- network observers, by TLS, and by the payload encryption underneath it

**Not defended against, and the docs must say so**

- an attacker with code execution on the control machine. They hold the control key
- an attacker with code execution on a station, for that station. They already run commands there, which is what a station is for
- traffic analysis. The relay sees timing, sizes and volume; see *What the relay sees*
- a step that prints a secret. `cap_redact` is a safety net, unchanged by any of this
- the operator, who can read anything the station can

## Why the station side is not pure bash

`secret.sh` establishes that openssl is an acceptable dependency, and it uses AES-256-CBC with PBKDF2 at 600k iterations. That construction is **unauthenticated**. For a value travelling one way through a private git repo, that is defensible. Through a relay that may tamper, it is not: an attacker who can flip ciphertext bits undetected defeats R2 and, for a request, R1.

The obvious repair is AEAD, and the obvious tool cannot do it:

```
$ openssl enc -aes-256-gcm ...
AEAD ciphers not supported by the enc utility
```

So a bash implementation would have to assemble X25519 key agreement via `openssl pkeyutl -derive`, an HKDF, AES-256-CBC, HMAC-SHA256 in encrypt-then-MAC order, and Ed25519 signatures, by hand, in shell, across openssl 1.1.1 and 3.x behaviour differences. Every primitive exists. Composing them correctly is where crypto bugs live, and a crypto bug here is silent.

**Decision: a small Go helper, `heliograph-seal`, does the cryptography. Nothing else.**

It does **not** do networking. The bash transport still runs `curl` and still handles the long-poll, so the network behaviour stays readable and debuggable in the shell, and the binary's trust surface is only the crypto.

```
heliograph-seal keygen                 write a keypair
heliograph-seal fingerprint            print this key's fingerprint
heliograph-seal enrol <token>          consume an enrolment token
heliograph-seal seal   --to <peer>     stdin plaintext  -> stdout sealed envelope
heliograph-seal open   --from <peer>   stdin envelope   -> stdout plaintext, or refuse
```

Two consequences worth having deliberately:

- **git, object store, file share and bundle stay pure bash.** Only the relay transport calls the helper. "It is just bash, you can read it before you run it" survives for every transport that does not need it
- **every other transport can adopt it later.** A sealed bundle on a USB stick and a sealed blob in someone else's S3 account are both obviously wanted, and both come free once this exists

## The construction

**Do not invent a protocol.** The single most valuable sentence in a cryptographic design review is that nothing here is bespoke.

| layer | |
|---|---|
| key agreement, encryption | the [age](https://age-encryption.org/v1) **construction**: X25519, HKDF-SHA256, ChaCha20-Poly1305 |
| origin authentication | Ed25519, over the plaintext, inside the encryption |
| transport | HTTPS, certificate verification on, no pinning |

**Corrected 2026-09-08.** This table said "via `filippo.io/age`" and "TLS 1.3", and `internal/seal` does neither. It uses the same primitives directly from the Go standard library - `crypto/ecdh`, `crypto/hkdf`, `crypto/ed25519`, `chacha20poly1305` - with its own envelope, and `filippo.io/age` is not a dependency at all. The HTTP client is the default one, so TLS 1.2 is the floor and 1.3 is negotiated when the server offers it; nothing enforces a minimum.

Neither is a defect, and both were worth correcting anyway. "Same construction, standard primitives, no invented protocol" is the claim that survives review; "we import the reference implementation" is a different and stronger claim that was not true, and a reviewer checking the imports would have found that out at the worst possible moment. If a TLS floor is wanted it should be set explicitly rather than described.

ChaCha20-Poly1305 rather than AES-GCM because station hardware is unknown and may lack AES-NI, where ChaCha is both faster and constant-time in software.

### Sign, then encrypt

The signature is over the plaintext and travels inside the encryption, so the relay cannot see who signed what, and a signature proves authorship rather than merely transmission.

Sign-then-encrypt has a known weakness - a recipient can re-encrypt a validly signed message to a third party, who then believes it was sent to them. The standard mitigation applies: **the recipient's key fingerprint is one of the signed fields**, so a forwarded message fails verification at the new recipient.

### The envelope

What the relay stores, and all it stores:

```
estate   e_7f3a9c21          routing only
seq      412                 monotonic, per direction
dir      c2s | s2c
bytes    18442
at       2026-09-06T10:15:00Z    the relay's own clock, for TTL only
body     <ciphertext>
```

What is inside, after `open`, with every field covered by the signature:

```
v          1
estate     e_7f3a9c21        bound: a cross-estate replay fails
station    st_4a91           bound: a cross-station replay fails
dir        c2s               bound: a reflected message fails
seq        412               bound: a replay fails
recipient  <fingerprint>     bound: surreptitious forwarding fails
sent       2026-09-06T10:15:00Z
kind       request | status | progress | log | payload
content    <the document, or the log bytes>
```

`open` refuses unless the signature verifies **and** `estate`, `station`, `dir` and `recipient` all match the identity it is running as. A refusal is a hard failure that publishes a status; it is never a warning and never a fallback.

### Freshness

`seq` is strictly increasing per estate, station and direction. Each side persists the highest value it has accepted and refuses anything at or below it.

A **gap** is tolerated, because the relay may legitimately drop or expire a message. A **regression** is refused. This is what stops the relay re-running an old destructive request by replaying it, which is the attack that matters most given `--allow-actions` and `CONFIRM=yes` are decided long before the request that uses them arrives.

`sent` is advisory. Clock skew on a locked-down station is common and must not be able to wedge the loop, so freshness rests on `seq`, not on time.

### No downgrade, ever

There is no opportunistic mode, no negotiation and no plaintext fallback. A station configured for the relay refuses any message it cannot verify and decrypt. `v` is a signed field, so the relay cannot offer an older version.

If `heliograph-seal` is missing or fails its checksum, the station **refuses to start** and says so. It does not fall back to sending in the clear.

## Enrolment

The chicken-and-egg is that control and station must learn each other's public keys through a channel the relay controls. The answer is that the operator already has to receive something to plant a station at all, so that string carries the trust.

```
control                             operator                     station
-------                             --------                     -------
heliograph init payments
  generate control keypair
  create estate at relay
  print enrolment token  ────────▶  receives it
                                    (the same channel that
                                     carries a repo URL today)
                                                        ────────▶ heliograph-seal enrol <token>
                                                                    verify ctrl pubkey hash
                                                                    generate station keypair
                                                                    publish station pubkey
  station fingerprint appears
  operator reads it back  ◀────────  reads fingerprint aloud
  confirm, or refuse
```

The token:

```
hg1_<estate>_<relay-host>_<control-pubkey-hash>_<one-time-secret>
```

It carries the **hash of control's public key**, so the relay cannot substitute its own. It is single use, expires in 24 hours by default, and consuming it is what lets a station publish its public key once.

The fingerprint read-back is the ceremony `secret.sh key show` already established in this product, so operators have met it before. It is what closes the residual risk that the enrolment token was intercepted before use.

`--trust-on-first-use` exists for automated planting where no human is in the loop, prints a loud warning, and records in the estate that this station was never verified.

## Key loss, and the failure that bricks an estate

**Losing the control private key means every log from every station in that estate is unreadable, permanently.** Nothing can recover it, by design. That is severe enough that the CLI must not let it happen quietly.

`heliograph init` refuses to complete until the control key has been backed up: written encrypted under a passphrase, and the path confirmed. `heliograph doctor` re-checks that the backup still exists and still decrypts.

Losing a station key is ordinary and cheap: revoke it and plant again.

## Revocation

Two mechanisms, because one of them relies on a party we do not trust.

1. **Control stops accepting.** Control removes the station's public key from the estate and rejects anything signed by it. This works even if the relay is hostile, and it is the one that counts
2. **The relay refuses the token.** Control asks the relay to invalidate that station's bearer token, which stops the traffic at the edge. A convenience and a cost control, not a security boundary

Revoking a station does not affect any other station, which is the containment R4 asks for.

## Relay tokens are not the security boundary

The station and control bearer tokens exist for routing, rate limiting and abuse control. They carry no confidentiality or authenticity role whatsoever.

**A stolen relay token yields denial of service and metadata, never content and never execution.** Saying so plainly in the docs matters, because "we use scoped tokens" is exactly the kind of claim that gets mistaken for the real protection.

## What the relay sees

Everything it has, in full:

| | |
|---|---|
| estate identifier | an opaque id, not a customer name |
| direction, sequence number | |
| ciphertext size | reveals roughly how long a log was |
| timing | when a request went out, when a log came back, so roughly how long a step took |
| station liveness | that *a* station is polling, not which host it is |
| bearer token identity | which credential is in use |

It does **not** see step names, host names, exit codes, log content, request environment, station state, or the operator's identity.

Size and timing are not concealed. Padding to buckets was considered and rejected for now: it costs bandwidth on links that are often poor, and the leak is coarse. If a customer needs it, it becomes an option rather than a default. The docs state the leak rather than implying it away.

## Retention

- ciphertext is deleted **on acknowledgement** by the recipient
- an unacknowledged message expires after **7 days**, then is deleted
- the hosted relay publishes these as its retention policy; a self-hosted relay makes both configurable
- nothing is archived, and there are no backups of message bodies. Losing an unacknowledged message costs a re-run, which is exactly the cost heliograph already accepts everywhere else

## The binary is a new trust surface, and must be treated as one

`heliograph-seal` runs on a machine nobody can reach, so it has to be verifiable without anyone present.

- reproducible builds, so a third party can rebuild the published artefact bit for bit
- signed releases, published checksums, and an SBOM
- the station **verifies the checksum before executing it**, and refuses to start on a mismatch
- buildable from source in one command, for estates that will not run a downloaded binary
- it is a separate, small, single-purpose artefact rather than a mode of the main CLI, so that what has to be audited stays small

## What DBHQ can honestly claim, and what it cannot

**Can claim.** Key generation happens on customer machines. No key material appears in any relay API, request or response. The relay's source is public and its builds are reproducible, so the deployed artefact can be checked against the source. The protocol is documented, uses no bespoke cryptography, and refuses to operate unencrypted.

**Cannot claim.** That the running relay is the published build - a customer verifies that by the station rejecting anything not correctly signed, not by trusting the operator. That timing and size are hidden; they are not. That a compromised control machine or station is protected; neither is. That an audit has been done - one is planned, and until it reports the claim is "designed to be auditable", not "audited".

The station verifying every message is what makes all of this hold **without** trusting the relay, which is the only form of the claim worth making.

## Checked against a real implementation

Paseo's relay is open source, and reading it was worth more than any amount of
further reasoning. The comparison is recorded because the differences are
decisions, not oversights on either side.

### What was taken from it

**Reject an all-zero shared secret.** A low-order or small-subgroup public key
makes X25519 produce all zeroes, and a peer supplying one could then read
everything. Paseo checks for it explicitly. Go's `crypto/ecdh` returns an error
for the same case, so it comes free here, but it is asserted in a test rather
than assumed: "the standard library handles it" is exactly the belief worth
measuring once.

**A random nonce even where a fixed one would be safe.** The key here is derived
from an ephemeral key that is fresh per message, so a zero nonce is sound. It is
also the sort of line that stops a review dead, correctly, and then costs the
reviewer an argument about ephemeral generation to clear. Twelve bytes buys
that argument away. Paseo does the same.

**Never infer a message's type from its contents.** Their note that "the
receiver never guesses whether authenticated plaintext is text or binary from
its byte contents" is the same principle as `kind` being a signed metadata field
here rather than something parsed out of the payload.

### Where the two designs genuinely differ

**Their phone is ephemeral-only; our control has a long-term identity.** In
Paseo the daemon holds a persistent keypair and the client generates a fresh one
per connection, so the daemon is authenticated to the client but the client is
not authenticated to the daemon. Their trust anchor is the pairing link, and
their security note says so plainly: *"Treat it like a password."* Anyone holding
it can drive the daemon.

Here a station is enrolled against a specific control public key and every
request must be **signed** by it. Possession of an enrolment token lets you
enrol once; it does not let you command a station afterwards. That is a
deliberately higher bar, and it is bought with a heavier enrolment: a token plus
a fingerprint read back aloud, rather than scanning a QR code.

The reason for the difference is what is on the other end. Paseo's daemon runs
on a machine its owner controls and can go and fix. A heliograph station runs
where nobody can reach it, so a credential that is enough to command it forever
is a credential that can never be rotated after a leak.

**Replay.** Paseo's security note is explicit that within a live session
"replay protection is not yet implemented". Sequence numbers are enforced here,
monotonically per estate, station and direction, because the attack is concrete:
`--allow-actions` and `CONFIRM=yes` are decided days before the request that
uses them, so a replayed request is a destructive step re-running with all its
gates already satisfied.

**Session versus message.** Theirs is a live WebSocket with one shared key per
session. Ours is store-and-forward, so each message carries its own ephemeral
key. Neither shape is better; they answer different questions.

## Open questions

1. **Multi-user control.** One estate, several engineers, each with their own key. Straightforward to add as a set of accepted control keys, but rotation and revocation need designing. Deferred to the point where a second person actually needs it
2. **Forward secrecy.** Rejected for now: a ratchet is real complexity, and delete-on-ack with a 7 day cap already bounds how much ciphertext exists to decrypt later. Revisit if retention ever grows
3. **Whether other transports adopt `heliograph-seal`.** The bundle case is compelling - a sealed tarball on a USB stick - and costs almost nothing once this exists. Not in C1's scope, but the interface should not preclude it
4. **A browser client for the future web UI.** Client-side decryption in WASM, the Bitwarden model. Explicitly out of scope here; the design does not block it and no work is done for it now
