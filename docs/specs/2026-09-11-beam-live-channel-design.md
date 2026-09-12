# The beam: the live channel, and the relayed beam

**Date:** 2026-09-11
**Status:** draft, for review
**Part of:** [the signalling toolkit](../plans/2026-09-11-signalling-toolkit-roadmap.md) (S4)

The beam is the third shape: a live, two-way connection held open between the
control node and the station, until it is torn down. It is the reverse
connection the raw-TCP transport was dropped for being, and this spec builds it
deliberately, off by default, sealed, signed, and honest about what it is. It
delivers the abstraction (`Channel`), the first carrier (the relayed beam, over
the relay server), and the station side that holds the line.

The direct beam - line of sight, peer to peer - is S6, and is another `Channel`
under the same abstraction. What rides the beam - a plain interactive shell, a
PTY, an `ssh` passthrough - is S5. This spec is the pipe and its security, not
its passengers.

## The layering

Four layers, each with one job, so the beam can gain carriers and passengers
without either learning about the other:

```
consumers        the shell, a PTY, ssh passthrough            (S3, S5)
  │
beam classes     step beam (gated per line) | raw beam (opaque stream)
  │
Noise            end-to-end handshake + AEAD frames, carrier-blind
  │
Channel          a duplex byte stream:  wss-over-relay | direct (S6)
```

- **`Channel`** is the carrier: a duplex byte stream, `Open`/`Send`/`Recv`/
  `Close`, and nothing about what the bytes mean. WebSocket-over-relay is the
  first implementation; the S6 direct beam is a second; the interface is what
  lets both exist without touching the layers above.
- **Noise** rides on top of any `Channel`. Because the end-to-end security is a
  layer above the carrier, the broker moves ciphertext and never holds a key -
  the same claim the relay makes for discrete messages, now for a stream.
- **beam classes** decide what the sealed stream carries, and are where the
  station keeps control (below).

## The `Channel` abstraction

Distinct from `Transport`, because a transport is discrete (`PutRequest`,
`ReadLog`) and a channel is live. A first cut:

- `Open(ctx) error` - establish the carrier and complete the Noise handshake,
  or fail before anything is trusted.
- `Send([]byte) error` / `Recv() ([]byte, error)` - sealed frames in each
  direction; or a single `io.ReadWriteCloser` once the handshake is done, which
  is what a PTY and `ssh` want.
- `Close() error` - tear down and forget the session keys.
- `Describe() string` - the carrier and the peer identity, by kind, never by
  key.

The CLI selects a beam estate the way it selects a transport; `internal/estate`
gains the beam, carrying the broker URL, the estate and station ids, and the
two identities the Noise handshake authenticates against.

## The carrier: WebSocket over the relay, first

The relayed beam extends the deployed relay server rather than standing up new
infrastructure:

- Each side opens a **`wss` connection to the broker over 443**. Both dial out;
  nothing accepts inbound, so the property that makes a beacon permissible - no
  inbound path - holds for the beam too.
- The broker pairs the two connections for an estate/station and **copies
  Noise-ciphertext frames between them**. It sees routing metadata and frame
  sizes, never plaintext, never a key.
- WebSocket is chosen for reliability through exactly the networks heliograph
  targets: outbound-only proxies that pass `wss` on 443 but frequently break
  gRPC/HTTP-2 streaming. "Cleaner protocol" loses to "actually connects" here.
- Keepalive, idle timeout and reconnection are the broker's, with the same
  ping/pong WebSocket already defines. An idle beam is torn down, not held for
  free.

A separate streaming carrier can be added later as another `Channel` with no
change above it; it is not built here, because `wss`-over-relay is the reliable
one and the direct beam (S6) is the more valuable second carrier.

## The station side needs a Go component

The feasibility flag, stated plainly because it moves a line the project has
held. **A bash station cannot hold a beam.** Noise and a `wss` connection are
not things `curl` and coreutils do; the relay already conceded this for
discrete messages, which is why `heliograph-seal` is a Go binary and why the
PowerShell relay transport is deferred on the same ground.

So the beam station side is a **Go component on the far side** - the same
trade as the relay's seal helper, one step further. This means:

- The beam is **not available on a pure-bash station.** An estate that forbids
  installing a binary gets the beacon and the flare, which do the whole job
  without a beam, and the docs say so where the beam is introduced.
- The component is the existing static binary or a small sibling, planted the
  way the station payload is, and it is the thing that speaks Noise and holds
  the `wss`. The bash loop is unchanged for the beacon and the flare.
- This is consistent with S1's honesty: the beam is the most capable shape and
  also the most demanding, and where it cannot be met the other two shapes are
  the answer.

## The station keeps control: the establishment gate and two classes

A beam holds the line open, so `run.sh` cannot read a mode header before each
command the way it does for a beacon or a flare. Control moves to
**establishment**, and the class of beam:

- **A beam is off unless the station is started with `--allow-beam`.** No flag,
  no beam - the same shape as `--allow-actions`. Establishment is signed by the
  station's identity, so the broker cannot conjure one.
- **The default class is a step beam.** Each line the caller sends is still a
  **mode-declared step run through `run.sh`**, so the per-command read-only /
  action gate and the per-command captured log survive, even though the channel
  is live. A step beam is a fast beacon, not a hole in the gate: a read-only
  station still refuses an action step over it.
- **A raw beam is opaque bytes, and needs more.** A PTY or an `ssh` passthrough
  (S5) is an opaque stream `run.sh` cannot see inside, so the per-command gate
  is gone and the only control is the grant. A raw beam therefore requires a
  **further explicit `--allow-raw-beam`**, is never the default, and the docs
  state it for what it is: interactive access to the account the station runs
  as, gated once, at the door.

This is the answer to S0's "what does a read-only beam mean" - a read-only beam
is a step beam on a read-only station, and it means exactly what read-only
means everywhere else, because the step still goes through `run.sh`.

## What the broker holds, and cannot

Stated so the claim is checkable, the way the relay's is:

- ciphertext frames and routing metadata (estate/station id, direction), for as
  long as a beam is up; nothing at rest after teardown.
- no key, ever - Noise keys are ephemeral per session and held only by the two
  ends.
- it cannot read the stream, and it cannot inject into it: a forged or
  reordered frame fails the Noise transport's authentication and the beam
  drops. A broker able to inject would be code execution inside the estate, the
  same bar the relay is held to.

## Non-goals

- No passengers - no shell, no PTY, no `ssh`. S5.
- No direct beam, no NAT traversal. S6.
- No PowerShell beam station in this spec; the Go component lands first, and
  the PowerShell question follows the relay's, unresolved on purpose.

## Done when

- A `Channel` opens a `wss`-over-relay beam, completes a Noise handshake, and
  fails closed if the peer identity does not verify.
- A step beam runs a read-only line and refuses an action line on a station
  started with `--allow-beam` but not `--allow-actions`.
- A raw beam refuses to establish without `--allow-raw-beam`, and carries an
  opaque byte stream when granted.
- The broker, given the session, cannot produce plaintext, and an injected
  frame drops the beam rather than being delivered - both proved in a test.
- An idle beam is torn down and its session keys forgotten.
