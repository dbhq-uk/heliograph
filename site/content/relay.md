# The relay

The only transport that needs no estate infrastructure at all. No git host, no
storage account, no VNet, no inbound rule. Both sides dial **out** over ordinary
HTTPS and meet at a server neither of them trusts.

## Status, plainly

| | |
|---|---|
| station side | complete. Fetches requests, publishes status and progress, delivers the finished log |
| relay server | [dbhq-uk/heliograph-relay](https://github.com/dbhq-uk/heliograph-relay), **deployed at `heliograph-relay.dbhq.uk`** |
| control side | `heliograph init --transport relay`, and a round trip in CI drives all three halves |

**It works end to end, over the deployed relay.** On 2026-09-09 a sealed
request went out through `heliograph-relay.dbhq.uk`, a station picked it up, ran
the step and sealed the log, and the CLI read it back with its timestamps and
its footer intact.

Two things prove it, and they prove different things. A round trip in CI drives
the real binary, a real `start.sh` and a real `heliograph-seal` against a local
relay - that is what shows the two halves agree. A second check, on `main` only,
asks the **deployed** relay whether it still answers this credential - that is
what catches the TLS, the custom domain, the routing and the token going wrong
independently of this repository, which they can and which is silent.

## Setting one up

```bash
heliograph init payments --transport relay \
  --dir https://heliograph-relay.dbhq.uk \
  --relay-estate payments --scope db-a

heliograph plant -e payments        # what to send the operator
heliograph relay peer -e payments <the line they send back>

export HELIOGRAPH_RELAY_TOKEN=...   # the CONTROL token
heliograph send steps/probe.sh
```

Full flags on [the CLI page](/cli#relay-estates). The one step that matters
most is the last one there: **compare the two fingerprints over a channel the
operator already trusts.** It is the only step a machine cannot do for you.

### Running your own

The relay is a queue with a token check and no keys, so hosting one is a small
thing to own:

```bash
CTL=$(head -c 32 /dev/urandom | base64)
STN=$(head -c 32 /dev/urandom | base64)
docker run -p 8080:8080 \
  -e HELIOGRAPH_RELAY_ESTATES="payments:$CTL:$STN" \
  ghcr.io/dbhq-uk/heliograph-relay:latest
```

Put it behind something that terminates TLS. `heliograph init` refuses a plain
`http://` URL for anything but loopback, because the token travels as a bearer
header on every request.

**Keep the estates value somewhere you can read it back.** Cloudflare secrets
are write-only, so a hosted relay whose token nobody recorded can only be
re-issued - which means re-enrolling every station on it.

## The threat model, which is the whole point

**The relay is outside the trust boundary in both directions.**

A relay that could read your logs would be a privacy problem - unacceptable for
customers who cannot let a third party read what their machines print, who are the customers.

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

**Two processes take them, so a lock is not optional.** The loop publishes the
status; the runner is a separate child process and publishes the finished log.
Both took a number from a value loaded when they started, so they collided - and
the receiver dropped the second as a replay, correctly, because that is what a
replay looks like. The log arrived and the run reported `running` for ever, with
nothing erroring anywhere. Numbers are now taken under a `mkdir` lock, and the
state file is written by taking the higher of each field so a stale writer can
never wind the counter back.

### A relay is a queue, not a store

It deletes on collection and expires after seven days. So the control node
**keeps what it collects**, and one consequence is worth knowing: whatever asks
the relay a question collects everything waiting, including messages it was not
asking about. That is why `heliograph doctor` proves its credential by
collecting properly rather than by a throwaway request, and why the station's
preflight probes a routing key nothing uses - an earlier version proved the
token by reading this station's own request queue, which silently ate whatever
request was waiting every time somebody started a station.

## Two tokens, two scopes

Because the station token sits on a machine you do not trust and cannot reach.

| token | may |
|---|---|
| station | read requests, write status and logs, for one estate |
| control | write requests, read logs, for one estate |

## Why the bash station needs a binary, and the PowerShell one does not

`heliograph-seal` is the single exception to *nothing is installed on the far
side*, and it applies to the **bash** station only. It is argued for explicitly
rather than smuggled in.

`openssl enc` refuses AEAD ciphers outright. A shell implementation would have
to hand-assemble encrypt-then-MAC and key agreement across openssl 1.1.1 and
3.x behaviour differences - which is where crypto bugs live, and where they are
silent.

So `heliograph-seal` does the sealing and **no networking at all**. `curl` stays
in the shell, where its behaviour can be read and debugged. Every other bash
transport stays pure bash and always will.

The binary must be present and executable or the station refuses to start:
there is no plaintext fallback and no degraded mode.

**The checksum is only enforced if you set one.** With `RELAY_SEAL_SHA256`, a
binary that does not match refuses to run. Without it the station prints a
warning and carries on, which is weaker than this page used to claim. Set it.

### On the PowerShell station there is no binary and no checksum

The [PowerShell station](/windows#the-relay-works-here-and-needs-nothing-installed)
does the same construction in **managed C# that ships as source** inside the
payload, compiled by `Add-Type` when the transport loads. `RELAY_SEAL` and
`RELAY_SEAL_SHA256` do not apply there; everything else on this page does.

That is not a shortcut around the argument above. It is the same reasoning
reaching a different answer, because the constraint is different: a shell
cannot do AEAD, and .NET can be given the code to. An estate that refuses a
native binary is exactly the estate that payload exists for, so a relay that
needed one would not have been a relay for them at all.

The two seals are **compared rather than assumed to agree**. `internal/seal`
emits golden vectors from fixed keys, and the PowerShell side has to reproduce
them byte for byte at every stage - canonical metadata, shared secret, derived
key, signature, sealed message - as well as open what Go sealed and refuse six
tampered variants. Each primitive is separately checked against its own
standard's vectors, because a round trip is satisfied by two implementations
that agree with each other and with nothing else.

## Configuring a station

```bash
TRANSPORT=relay
RELAY_URL=https://heliograph-relay.dbhq.uk
RELAY_ESTATE=payments
RELAY_STATION=sql01
RELAY_TOKEN=<the station-scoped token>
RELAY_IDENTITY=/path/to/station.key      # this station's key
RELAY_PEER=/path/to/control.pub          # the control side's public identity
RELAY_SEAL_SHA256=<checksum from the release>   # bash station only
```

`RELAY_SEAL_SHA256` is optional and should not be - see above. On the
PowerShell station it does not exist, because neither does the binary.

A request may **not** set anything that configures capture, delivery, redaction
or identity - including every `RELAY_*` variable above. `RELAY_PEER` is the
sharpest case: it is what an incoming request is verified against *and* what an
outgoing log is sealed to, so a request that could choose it could choose who
reads the log. Reserved by prefix rather than by name, and tested. See
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

## Known limits, before you rely on it

Two, both found by adversarial review and neither yet fixed. They are here
rather than in an issue tracker because somebody evaluating this transport
needs them before they choose it.

**A cancelled run's partial log does not arrive.** The station passes it
alongside the cancellation status, and only the git transport honours that.
On the relay the cancellation arrives and the partial evidence stays on the
station.

**Sequence numbers can collide between the loop and the runner.** The loop
loads its counter once at start; the runner delivers the finished log from its
own process and advances the counter there. The loop does not reload before
publishing `idle`, so it can emit a number the runner has already used - and
the receiver drops anything at or below what it has already accepted, by
design, because that is the replay defence. One of the two messages can be
lost.

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
