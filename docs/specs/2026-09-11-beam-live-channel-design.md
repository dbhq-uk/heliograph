# The beam: the live channel, and the relayed beam

**Date:** 2026-09-11
**Status:** draft, for review
**Part of:** [the signalling toolkit](../plans/2026-09-11-signalling-toolkit-roadmap.md) (S4)

> **The carrier order was inverted on 2026-09-12, and this document is corrected
> rather than annotated, because it is a draft that has not been built from.**
> As written it made `wss`-over-relay the first and only `Channel`. It reached
> that by comparing WebSocket against gRPC and HTTP/2 streaming, which are not
> the alternative: the alternative is the long poll the relay already uses and
> has already proved through these networks. Against that baseline `wss` is
> strictly less robust, so the **long-poll-framed carrier is carrier one** and
> `wss` is a per-estate optimisation with silent fallback. The reasoning is in
> "The carrier" below and is recorded there so it does not get re-argued.
> Reference: heliograph-io/heliograph-cloud#36.

The beam is the third shape: a live, two-way connection held open between the
control node and the station, until it is torn down. It is the reverse
connection the raw-TCP transport was dropped for being, and this spec builds it
deliberately, off by default, sealed, signed, and honest about what it is. It
delivers the abstraction (`Channel`), the first carrier (the relayed beam, over
the relay server, long-poll-framed), and the station side that holds the line.

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
Channel          a duplex byte stream:  long-poll-framed over the relay
                                        | wss, negotiated | direct (S6)
```

- **`Channel`** is the carrier: a duplex byte stream, `Open`/`Send`/`Recv`/
  `Close`, and nothing about what the bytes mean. The long-poll-framed carrier
  over the relay is the first implementation; `wss` over the same relay is a
  second, negotiated per estate; the S6 direct beam is a third. The interface is
  what lets all three exist without touching the layers above, and it is what
  makes `wss` an optimisation rather than a rewrite.
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

## The carrier: the long poll first, `wss` as an optimisation

The relayed beam extends the deployed relay server rather than standing up new
infrastructure. Both carriers below are the same broker, the same estate and
station ids, and the same Noise frames; only the shape on the wire differs.

```
   carrier 1   long-poll-framed    proven through these networks already.
                                   Higher latency. Always available
   carrier 2   wss                 negotiated per estate. Probe once,
                                   remember the answer, fall back silently
   carrier 3   direct (S6)         later, and gated separately
```

**Carrier one is a long poll, framed.** Each side sends Noise-ciphertext frames
as ordinary HTTP requests and collects the other side's with a held-open GET,
exactly as the discrete relay transport already does. Both dial out; nothing
accepts inbound, so the property that makes a beacon permissible - no inbound
path - holds for the beam too. The broker pairs the two sides for an
estate/station and copies frames between them, seeing routing metadata and
frame sizes, never plaintext and never a key.

**Why this order, which is the correction.** An earlier revision of this spec
made `wss` the first and only carrier, on the grounds that it is reliable
"through exactly the networks heliograph targets: outbound-only proxies that
pass `wss` on 443 but frequently break gRPC/HTTP-2 streaming". That sentence is
true and it compares `wss` against the wrong alternative. gRPC and HTTP/2
streaming were never the alternative. **The alternative is the long poll the
relay already uses and has already proved through these networks**, and against
that baseline `wss` is strictly less robust:

| the path | can an intermediary see the `Upgrade` header? | so `wss` |
|---|---|---|
| `ws://` on port 80 | yes, in cleartext | is routinely stripped |
| `wss://` through a CONNECT-tunnelling proxy | no, it is inside TLS | works fine |
| `wss://` through a TLS-intercepting proxy | yes, the proxy terminates TLS and reads the handshake | may be stripped, **and there is no client-side fix** |

A long poll is an ordinary HTTP request that happens to take a while. No
intermediary has to understand it, there is no protocol upgrade to strip, and
nothing has to be configured on the proxy. It is the most boring thing on the
wire, which is exactly why it survives.

**And the estates heliograph exists for are the ones most likely to run TLS
interception.** A corporate root CA and a DPI box are standard in a
change-controlled, bastion-only estate with no route in. So `wss` works in most
networks and
fails in the ones that matter most, silently, with no remedy the client side
can apply - only the network administrator can permit it. A carrier that fails
there fails on exactly the estates this exists for.

**Carrier two is `wss`, negotiated per estate.** Probe once, remember the
answer, and **fall back silently** to carrier one on failure. It is an
optimisation on latency, not a capability: nothing above the `Channel` may
behave differently depending on which carrier is under it, and no feature may
be gated on the upgrade succeeding. Keepalive, idle timeout and reconnection
are the broker's; an idle beam is torn down, not held for free.

**What this costs.** Poll-interval latency in the worst estates and nothing in
the best ones. A step beam tolerates that and a PTY does not: a step beam is
discrete framed requests, so extra latency is a slower shell rather than a
broken one, while a raw beam PTY over a long poll would be unusable. That is a
further argument for the step beam being the default and the raw beam being
exceptional, which is what is already designed below.

**What it buys.** The beam ships on infrastructure already proved end to end
against the deployed relay, rather than on a carrier whose viability in the
target market is an open question. Measuring how often `wss` survives in real
estates is still worth doing, because it sizes how often the optimisation
applies, but it **no longer blocks the beam**: carrier one does not depend on
the answer.

One operational note for whenever `wss` does ship: Cloudflare requires
WebSockets to be enabled under Network -> WebSockets, or `Upgrade` headers are
not forwarded and the handshake fails with "101 not received".

The direct beam (S6) is a third `Channel` with no change above it, and is the
more valuable next carrier once carrier one is proved.

## The station side needs a Go component

The feasibility flag, stated plainly because it moves a line the project has
held. **A bash station cannot hold a beam.** The blocker is **Noise**, not the
carrier: `curl` and coreutils do a long poll perfectly well - the bash relay
transport already does exactly that - but they do not do a Noise handshake and
AEAD framing, and neither do they do a `wss` connection. The relay already
conceded the crypto half for discrete messages, which is why `heliograph-seal`
is a Go binary and why the PowerShell relay transport was deferred on the same
ground. Making the long poll carrier one narrows the reason but does not remove
it.

So the beam station side is a **Go component on the far side** - the same
trade as the relay's seal helper, one step further. This means:

- The beam is **not available on a pure-bash station.** An estate that forbids
  installing a binary gets the beacon and the flare, which do the whole job
  without a beam, and the docs say so where the beam is introduced.
- The component is the existing static binary or a small sibling, planted the
  way the station payload is, and it is the thing that speaks Noise and holds
  the carrier, whichever one was negotiated. The bash loop is unchanged for the
  beacon and the flare.
- This is consistent with S1's honesty: the beam is the most capable shape and
  also the most demanding, and where it cannot be met the other two shapes are
  the answer.

### Reproducible, or it is not readable

Publishing the source is **not sufficient** here, and saying so is the point.
"You can read it before you run it" has been the proposition, and nobody reads
a binary. So the component ships with a **reproducible build**: deterministic
output (`-trimpath`, `CGO_ENABLED=0`, a pinned toolchain), verification steps a
third party can actually run, and checksums published with each release the way
the CLI's already are. An operator who reviewed the source can then prove the
binary in front of them came from it.

This is the price of moving off pure bash, and it is **gated**: the beam does
not ship without it. The first version of this that gets security-reviewed is
the one that sets the impression, and it needs an answer to "how do I know this
is the source I read".

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

## Two seams, designed in now

Both are interfaces in the open-source build, not features withheld from it.
They exist so that a hosted layer - or anyone else - extends this rather than
forks it, and both are cheap now and expensive to retrofit.

- **A policy hook at the broker.** Establishment decisions are taken through a
  defined interface rather than hardcoded, so a deployment can refuse a class
  of beam for an estate. S6 already needs this to enforce estate-wide
  relayed-only, so the seam is required whatever else is built on it. The
  open-source broker ships a policy that reads local configuration; nothing is
  gated behind a service.
- **An identity-provider interface.** The station and control identities are
  resolved through an interface rather than being fixed to a single operator
  key on disk. The default implementation is exactly today's behaviour - one
  key, one operator, no service - so a self-hoster sees no change. It exists
  because the relay spec already deferred multi-user control, and retrofitting
  identity after single-operator keys are baked through the handshake is the
  expensive kind of change.

**The line these seams respect:** every shape and every passenger - beacon,
flare, relayed and direct beam, the PTY, `ssh` passthrough, the forward - stays
in the open-source build, along with the crypto, the key handling, the gates,
and the *generation* of audit. The seams exist for multi-user, multi-estate and
governance concerns, never to move a gap-crossing capability behind one.

## Non-goals

- No passengers - no shell, no PTY, no `ssh`. S5.
- No direct beam, no NAT traversal. S6.
- No PowerShell beam station in this spec; the Go component lands first, and
  the PowerShell question follows the relay's, unresolved on purpose.

## Done when

- A `Channel` opens a **long-poll-framed** beam over the relay, completes a
  Noise handshake, and fails closed if the peer identity does not verify. This
  is carrier one and it is what the beam ships on.
- `wss` is negotiated per estate, **falls back silently** to the long poll when
  the upgrade does not survive, and nothing above the `Channel` behaves
  differently for either - proved by running the beam's own tests over both
  carriers rather than by inspection.
- A step beam runs a read-only line and refuses an action line on a station
  started with `--allow-beam` but not `--allow-actions`.
- A raw beam refuses to establish without `--allow-raw-beam`, and carries an
  opaque byte stream when granted.
- The broker, given the session, cannot produce plaintext, and an injected
  frame drops the beam rather than being delivered - both proved in a test.
- An idle beam is torn down and its session keys forgotten.
- **The station component builds reproducibly**: a third party following the
  published steps produces a binary matching the released checksum. The beam
  does not ship until this passes.
- Establishment runs through the policy hook, and identities resolve through
  the identity-provider interface, with the default implementations behaving
  exactly as a single-operator station does today.
