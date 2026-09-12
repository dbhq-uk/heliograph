# SSH and interactive over the beam

**Date:** 2026-09-11
**Status:** draft, for review
**Part of:** [the signalling toolkit](../plans/2026-09-11-signalling-toolkit-roadmap.md) (S5)

The beam (S4) is a sealed byte stream. This spec is what rides it: a live
terminal, real `ssh` carried through it, and a gated jump host. It turns "you
cannot log in" into "you log in through heliograph" - the original ask - and
does it without heliograph ever accepting an inbound connection, because the
beam is dialled out from both ends.

All three passengers are **raw beams**, so all three require the station to
have been started with `--allow-raw-beam` (S4). The station keeps the last word
throughout: the control side can ask, the far-side component decides.

## Three passengers

### 1. The live PTY shell

`heliograph shell <estate>` against a beam estate upgrades the discrete REPL of
S3 into a real terminal. The far-side Go component allocates a PTY, runs the
station account's login shell in it, and pipes it over the raw beam. Unlike the
S3 shell - a step per line, gated through `run.sh` - this is a genuine session:
job control, `top` and `vi`, colours, a shell that remembers its own `cwd`. It
needs no `sshd` on the far side, which is the point of having it: the plainest
way to get a live shell where one is permitted.

The trade is the one S4 named - an opaque stream is not gated per command - so
the PTY shell is a raw beam and inherits that gate at the door.

### 2. ssh passthrough

`heliograph proxy <estate>` is a byte conduit: it reads stdin and writes
stdout, carrying them over a raw beam to the station's own `sshd` on
`127.0.0.1:22`. It is built to be an OpenSSH `ProxyCommand`:

```
ssh -o ProxyCommand="heliograph proxy payments" user@station
```

Now OpenSSH runs end to end - its own key exchange, host-key check and auth -
*inside* the Noise-sealed beam. Two layers, and each earns its place: the beam
authenticates the *station* (the machine is who it claims) and seals the
carrier; ssh authenticates the *account* (you are who you claim) the way the
estate's own ssh already does. The near side never had a route to port 22; the
beam gives it one that dials outward.

This needs an `sshd` on the far side, bound to localhost - common even where
inbound 22 is firewalled, which is often exactly why heliograph was needed. If
there is no `sshd`, the PTY shell is the answer instead.

### 3. The generic TCP forward, allowlisted

`heliograph proxy <estate> <host:port>` connects the beam to a destination
reachable *from the station* - `ssh -W` / `ssh -L` semantics, with the station
as the jump host:

```
ssh -o ProxyCommand="heliograph proxy payments sql01:22" user@sql01
heliograph proxy payments sql01:1433        # forward a database port
```

This is the most powerful passenger and the widest blast radius, so it is
gated where it belongs - **on the far side, by the operator**:

- The station forwards only to destinations the operator listed at start:
  `--forward sql01:22 --forward sql01:1433`. A destination not on the list is
  **refused by the station**, not filtered by the client, so onward reach is a
  deliberate, named grant and the control side cannot widen it.
- No allowlist entry, no forward. `--allow-raw-beam` permits a beam; it does
  not permit reaching the neighbours. Onward reach is its own decision.

## The station keeps the last word

Restating S4 for the passengers, because a live channel is where control is
easiest to lose:

- A beam does not establish at all without `--allow-raw-beam`.
- A forward reaches only an allowlisted destination; the far-side component
  enforces the list and rejects the rest.
- The PTY shell runs the station account's shell, so the account is the blast
  radius - the same bound the station has always had. It is not root; the loop
  already refuses to run as root and the beam component inherits that.

## Audit: a connection record, not content

An opaque stream cannot be captured line by line the way `run.sh` captures a
step, and S4 said so. What survives is a **connection record**, and the station
writes one for every raw beam: class (PTY, ssh, forward), destination for a
forward, the peer identity, open and close times, and bytes each way. It is not
the content, but it is evidence that a beam happened, to where, for how long,
and how much crossed - which is the most a live opaque channel can honestly
offer, and more than a raw tunnel offers at all. The record ships back the way
a captured log does, so it lands beside the estate's other evidence.

**It is emitted as a structured event, not only as a line of text.** A log line
has to be parsed back into fields by whoever consumes it, and every consumer
parses it slightly differently. A defined event - one schema, versioned - can
be shipped to a SIEM, counted, or aggregated across estates without anybody
writing a regular expression first. This is better engineering for a
self-hoster with Splunk as much as for anything hosted, which is why it is in
the open-source build rather than reserved. The station still writes the human
-readable record too; the event is in addition, not instead.

## The proxy subcommand behaves

`heliograph proxy` is designed to be driven by ssh, so it obeys the contract a
`ProxyCommand` must: the data path is stdin/stdout and nothing else, every
diagnostic goes to stderr, and it exits with a clear status when the beam drops
so ssh reports a real error rather than hanging. A stray byte on stdout would
corrupt the ssh stream, and the tests assert there is none.

## Non-goals

- No new carrier and no NAT traversal; the direct beam is S6. S5 rides whatever
  `Channel` S4 established.
- No inbound listener anywhere. The beam is dialled out; `heliograph proxy` is
  a client of it.
- No content capture of a raw beam - a connection record only, by design.

## Done when

- `heliograph shell` over a beam estate gives a working PTY: `vi`, colours and
  job control behave.
- `ssh -o ProxyCommand="heliograph proxy <estate>"` logs into the station's
  `sshd` end to end, the ssh session sealed inside the beam.
- `heliograph proxy <estate> sql01:22` reaches an allowlisted destination and
  is **refused** for one that is not, with the refusal coming from the station.
- Every raw beam leaves a connection record - class, destination, peer, times,
  bytes - beside the estate's logs, **both as readable text and as a versioned
  structured event**, and a consumer can ingest the event without parsing prose.
- `heliograph proxy` puts nothing but the tunnelled bytes on stdout, asserted
  by a test, and exits non-zero when the beam drops.
