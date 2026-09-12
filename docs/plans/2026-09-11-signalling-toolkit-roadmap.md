# The signalling toolkit: three shapes, and the beam

**Date:** 2026-09-11
**Status:** draft, for review

This is the coordinating document for a program of six designs. Each design
becomes its own spec and its own PR (or small run of PRs); this file is how
they stay aware of each other. It names the shared vocabulary, the order, the
dependencies, and the questions each spec still has to answer. Detail lives in
the specs; this file holds the shape of the whole thing and nothing else.

## What changes, in one paragraph

Until now every way across the gap left a *message* somewhere and collected it
later, and the product defined itself by the one way across it refused to be: a
tunnel. This program adds the way across that holds the *connection* open - the
**beam** - renames the three shapes onto heliograph's own medium (light), and
drops the compliance-label framing throughout. The proposition stops being
"the remote-execution tool that is not a C2 channel" and becomes "every way
across the gap, each one honestly described, pick the one your estate allows".
The beam is the biggest of these and the one that most needs saying plainly:
it is a held-open channel, and the docs will say so.

## The three shapes, on one medium

The taxonomy is one axis - **what is held, and for how long** - and the names
now come from signalling, which is what a heliograph is.

| shape | was | now | what is held | who reaches whom | latency |
|---|---|---|---|---|---|
| dead drop | pigeonhole | **beacon** | a *message*, at rest | both dial one agreed place | poll interval |
| direct call | intercom | **flare** | a *transaction*, for one exchange | caller reaches the far endpoint | synchronous |
| open line | *(new)* | **beam** | the *connection itself* | rendezvous, or direct | real-time |

- **beacon** - a signal lit at a place both can see, and collected by looking
  later. The far side already holds the code, so a request only names it: the
  gate is strongest here. git, the relay, a file share, a storage account.
- **flare** - fired straight at a reachable endpoint, one burst, an immediate
  answer, then gone. The caller ships the code, so the mode header becomes a
  claim and the gate moves to the endpoint (the key and the allowlist).
- **beam** - a steady beam held on the far station, live and two-way, until it
  is torn down. Two variants, and the optical metaphor names both:
  - **relayed beam** - bounced off a relay, as a mirror bounces light. Both
    sides dial out; no inbound path. The brokered topology.
  - **direct beam** - line of sight. A direct peer connection, NAT traversal
    included. The P2P topology.

## Two interfaces

The beam does not fit the transport interface, and that fact shapes the whole
program.

- **`Transport`** (exists, `internal/transport/transport.go`) - discrete
  request and response. `PutRequest`, `FetchStatus`, `ReadLog`. **beacon** and
  **flare** are transports.
- **`Channel`** (new, S4) - a live bidirectional stream. **beam** is a channel.
  The shell and the SSH passthrough sit on top of one or the other, and only
  the beam ones need the channel.

## The specs

Seven documents including this one. "Ready" means the design decisions are
settled and the spec can be written now; "needs a design pass" means real
architecture is still open and the spec is a brainstorm of its own.

| id | spec | delivers | depends on | state |
|---|---|---|---|---|
| S0 | *this file* | the program, the vocabulary, the order | - | written |
| S1 | three shapes and the signalling names | rename to beacon/flare/beam, remove the compliance-label framing, rewrite the anti-tunnel prose into an honest beam characterisation, one generated matrix source, rename-with-aliases strategy | - | ready |
| S2 | discrete transports, completed | wire the existing relay into CLI selection; build the **flare** control-side Go transport; close or document the `ListLogs` gap | - | ready |
| S3 | `heliograph shell` | a REPL that authors an ephemeral step per line over any transport; read-only by default; Ctrl-C cancels; emulated cwd and env; wired to OS OpenSSH as a forced command | S2 | ready |
| S4 | the beam: the live channel and the relayed beam | the `Channel` interface; Noise; the relayed-beam broker (both sides dial out `wss`, sealed stream, no inbound); a Go station component; the establishment gate with step-beam and raw-beam classes | S1 | drafted |
| S5 | SSH and interactive over the beam | a live PTY shell; real `ssh` passthrough via `heliograph proxy <station>` as a `ProxyCommand`; a gated generic TCP forward (jump host) with an operator destination allowlist; a connection-record audit | S3, S4 | drafted |
| S6 | the direct beam | the P2P variant: a WebRTC data channel over ICE with STUN, sealed signalling through the broker, transparent fallback to the relayed beam (no TURN), broker-enforced estate policy | S4 | drafted |

Dependency order: S1 and S2 start immediately and in parallel. S3 follows S2.
S4 follows S1. S5 follows S3 and S4. S6 follows S4.

```
S1 ─────────────► S4 ──────┐
                   │        ├─► S5
S2 ────► S3 ───────┼────────┘
                   └─► S6
```

## Tracking

Umbrella issue **#85**, with one issue per design so a priority can be linked
to rather than remembered, the way the rest of the register works.

| S1 | S2 | S3 | S4 | S5 | S6 | cross-cutting |
|---|---|---|---|---|---|---|
| #86 | #87 | #89 | #90 | #91 | #92 | #93 |

**#93 raised two requirements that belong to no single design, and both are now
decided and folded into the specs that own them:**

- **Reproducible builds** and published checksums for the beam's Go component -
  the transparency answer to putting a binary where bash used to be readable.
  **Gated**: it is in S4's done-when, so the beam does not ship without it.
- **Three seams**, all adopted: a **broker policy hook** and an
  **identity-provider interface** in S4, and a **structured audit event
  stream** in S5. They are interfaces in the open-source build, with default
  implementations that behave exactly as a single-operator station does today -
  not features withheld from it. Every shape and passenger, the crypto, the
  gates and the generation of audit stay open; the seams serve multi-user,
  multi-estate and governance concerns only.

## The open questions the later specs must answer

Named here so they are not rediscovered, and so S4-S6 are recognised as design
work rather than transcription.

- **Sealing a live stream (S4).** The relay spec sealed discrete messages and
  [deferred forward secrecy](../specs/2026-09-06-relay-encryption-design.md)
  on the grounds that delete-on-ack and a 7-day cap bound the ciphertext. A
  beam is a long-lived stream, which reopens that: a per-message ephemeral key
  does not obviously carry to a stream, and a session key exchange or a ratchet
  may be needed. This is the hardest question in the program.
- **What the broker holds (S4).** The relay is store-and-forward and blind. A
  relayed-beam broker forwards a live stream. It must stay as blind - it sees
  ciphertext frames and routing metadata, nothing more - and the spec has to
  show that it cannot become a channel the broker can read or inject into.
- **The gate on a held-open channel (S4, S5).** beacon and flare gate per
  request. A beam holds the line open, so "read-only unless the operator said
  otherwise" needs a new expression: what does a read-only beam even mean, and
  how does the station keep the last word once the line is live.
- **NAT traversal without becoming a hole-punch product (S6).** A direct beam
  is exactly the thing the docs called a firewall hole-punch. S6 has to state
  the trade in full, gate it hard, and fall back to the relayed beam when
  direct fails - not to TURN, which would be a second relay doing the job S4's
  broker already does - so the estate that forbids the direct beam still has
  the relayed one.

## The rename is not just words

`pigeonhole` and `intercom` are filenames and environment-variable contracts -
`pigeonhole.sh`, `intercom.sh`, `intercom.py`, `PIGEONHOLE_*`, `INTERCOM_URL`,
the `/intercom` site page, `skill_coherence_test.go`. Renaming those breaks
deployed stations and the Azure templates that reference them. So the rename
runs in two parts, and only the first is in S1:

1. **User-facing now (S1).** The docs, the site, the CLI help and any new CLI
   surface use beacon, flare and beam. This is where a reader meets the words.
2. **Internal later, with aliases (its own PR under S1).** The scripts and the
   environment variables are renamed with the old names kept as accepted
   aliases and a deprecation window, so no station in the field stops working
   the day the rename lands. `PIGEONHOLE_ALLOW_ACTIONS` keeps working; the
   station warns and reads `BEACON_ALLOW_ACTIONS` first.

## Where this is recorded

`PLAN.md` is the register and carries a one-line pointer to this file. Each
spec updates `PLAN.md` as its PRs land, the same as every other line of work.
