# The relay

The only transport that needs no estate infrastructure at all. No git host, no
storage account, no VNet, no inbound rule. Both sides dial **out** over ordinary
HTTPS and meet at a server neither of them trusts.

## Status, plainly

| | |
|---|---|
| station side | complete. Fetches requests, publishes status and progress, delivers the finished log |
| relay server | [dbhq-uk/heliograph-relay](https://github.com/dbhq-uk/heliograph-relay), deployed |
| control side | implemented in `internal/transport`, and **no CLI command can select it** |

So it is not usable end to end yet, and this page describes it as designed so
the design can be reviewed. Teaching `heliograph init` to select it is on
[the roadmap](https://github.com/dbhq-uk/heliograph/blob/main/docs/plans/2026-09-08-powershell-and-docs-roadmap.md);
until then, every example on this page is station-side configuration.

## The threat model, which is the whole point

**The relay is outside the trust boundary in both directions.**

A relay that could read your logs would be a privacy problem - unacceptable for
regulated customers, who are the customers.

A relay that could **forge a request** would have code execution inside every
estate at once, through a channel the estate installed deliberately. That is
the one that matters, and everything below exists to make it impossible rather
than merely against the rules.

## How that is achieved

Content is encrypted with a key held only on control and station. The relay
stores and forwards ciphertext it cannot read, and every message is signed by
its origin and verified before it is acted on.

**Sign then encrypt**, in that order: the signature travels *inside* the
encryption, so the relay cannot see who signed what, and cannot strip or swap a
signature it cannot reach.

Nothing bespoke:

| | |
|---|---|
| key agreement | X25519 |
| key derivation | HKDF-SHA256 |
| encryption | ChaCha20-Poly1305 |
| signing | Ed25519 |

That is the age construction with signing added. ChaCha rather than AES-GCM
because station hardware is unknown and may lack AES-NI, where ChaCha is both
faster and constant-time in software.

Every message binds the estate, the station, the direction, the kind and a
sequence number **inside the signature**, so a relay cannot replay a message,
reflect one back, or forward one from a different estate.

### Sequence numbers are the replay defence

They are persisted on both sides. Losing that state is not merely inconvenient:
a reset would let the relay replay everything it has ever seen. The station
keeps them in a local, gitignored file.

## Two tokens, two scopes

Because the station token sits on a machine you do not trust and cannot reach.

| token | may |
|---|---|
| station | read requests, write status and logs, for one estate |
| control | write requests, read logs, for one estate |

## Why this one transport needs a binary

`heliograph-seal`, and it is the single exception to *nothing is installed on
the far side*. It is argued for explicitly rather than smuggled in.

`openssl enc` refuses AEAD ciphers outright. A shell implementation would have
to hand-assemble encrypt-then-MAC and key agreement across openssl 1.1.1 and
3.x behaviour differences - which is where crypto bugs live, and where they are
silent.

So `heliograph-seal` does the sealing and **no networking at all**. `curl` stays
in the shell, where its behaviour can be read and debugged. Every other
transport stays pure bash and always will.

The binary must be present and executable or the station refuses to start:
there is no plaintext fallback and no degraded mode.

**The checksum is only enforced if you set one.** With `RELAY_SEAL_SHA256`, a
binary that does not match refuses to run. Without it the station prints a
warning and carries on, which is weaker than this page used to claim. Set it.

## Configuring a station

```bash
TRANSPORT=relay
RELAY_URL=https://relay.heliograph.dbhq.uk
RELAY_ESTATE=payments
RELAY_STATION=sql01
RELAY_TOKEN=<the station-scoped token>
RELAY_IDENTITY=/path/to/station.key      # this station's key
RELAY_PEER=/path/to/control.pub          # the control side's public identity
RELAY_SEAL_SHA256=<checksum from the release>
```

`RELAY_SEAL_SHA256` is optional and should not be - see above.

A request may **not** set `TRANSPORT`, `PUSH`, `REDACT` or `LOG_DIR`. See
[security](/security).

## Self-hosting

The same server, one container, no keys:

```bash
docker run -p 8080:8080 \
  -e HELIOGRAPH_RELAY_ESTATES="payments:$CONTROL_PUB:$STATION_PUB" \
  ghcr.io/dbhq-uk/heliograph-relay:latest
```

It holds public identities so it can route, and nothing it could decrypt
anything with.

## Long-poll, not WebSocket

A station runs behind a corporate proxy that may strip the upgrade header, and a
transport that fails on those estates fails on exactly the estates this is for.
The long poll holds for 25 seconds server-side, so a client must wait longer
than that or it times out its own successful poll.

## What DBHQ can and cannot claim

**Can:** the hosted relay cannot read your content, and cannot cause a station
to run anything. The server source is public precisely so that is checkable
rather than trusted, and it holds no keys.

**Cannot:** that the relay learns nothing at all. It sees message sizes,
timings, which estate is active and when. Traffic analysis is not addressed and
is not claimed to be.

The full account is in
[`docs/specs/2026-09-06-relay-encryption-design.md`](https://github.com/dbhq-uk/heliograph/blob/main/docs/specs/2026-09-06-relay-encryption-design.md),
including what was corrected in it and why.
