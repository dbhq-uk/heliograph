# The direct beam: peer to peer, line of sight

**Date:** 2026-09-11
**Status:** draft, for review
**Part of:** [the signalling toolkit](../plans/2026-09-11-signalling-toolkit-roadmap.md) (S6)

The relayed beam (S4) sends every byte through the broker. The direct beam is
line of sight: the two ends find a path straight to each other and the broker
carries only the handshake that set it up. It is faster, it costs the broker no
bandwidth, and it is the hole-punch S1 characterises without flinching. It is
another `Channel` under S4's abstraction, so nothing above it - Noise, the beam
classes, the S5 passengers - learns that anything changed.

## What it is

- A **WebRTC data channel over ICE**, using **STUN** for candidate discovery.
  ICE finds a working path through NAT the way any WebRTC peer does.
- **Noise rides the data channel**, exactly as it rides the `wss` carrier in
  S4. DTLS protects the hop, but the E2E security does not trust it, the STUN
  server or the signalling: the peer identity is authenticated by the Noise
  handshake, end to end, or the beam does not come up.
- **No TURN.** When ICE cannot find a direct path - symmetric NAT, a firewall
  that blocks it - the fallback is the **relayed beam** we already built. TURN
  and the relayed broker do the same job, so there is one relay, not two, and
  nothing to deploy, credential or pay bandwidth for that S4 does not already
  provide.

## Signalling, through the sealed broker

ICE needs a signalling channel to exchange the SDP offer and answer and the ICE
candidates. It reuses the broker both ends already dial for the relayed beam:

- Offer, answer and candidates travel as **sealed, signed messages** through
  the broker, so the broker cannot read them or substitute its own - a broker
  that could rewrite the candidates could steer the beam to a peer it controls,
  which would defeat the point. The same trust bar S4 sets for frames applies
  to the negotiation that precedes them.
- Once the data channel is up, **no bytes cross the broker at all**. For a
  relayed fallback, the broker carries frames as in S4; for a direct beam, it
  saw only the handshake.

## The fallback is transparent

A consumer asks for a beam; it gets one. The direct `Channel` tries ICE within
a bounded time and, on failure, **falls back to the relayed `Channel`** behind
the same interface. The S5 passengers - the PTY shell, `ssh` passthrough, the
gated forward - do not know or care which carrier they ended up on; they see a
duplex stream either way. The connection record (S5) notes which carrier was
used, because "direct or relayed" is exactly the kind of fact a later reader
wants.

## Gating: the org holds the hole-punch

The direct beam is the most sensitive shape, so the decision to allow it sits
in two places, and the higher one wins:

- **Per station:** a direct beam needs `--allow-direct-beam`, on top of
  `--allow-raw-beam` for the raw passengers. No flag, no direct path - the
  station falls back to relayed or refuses, per how it was started.
- **Per estate, at the broker:** an estate configured **relayed-only** has the
  broker **refuse to relay ICE signalling** for it. No signalling, no direct
  beam, whatever a station's flags say. So a security team sets "never
  hole-punch, relayed beams only" in one place and no station in the estate can
  override it. This is the control that makes the direct beam adoptable: the
  people who would forbid it can forbid it centrally.

## Honestly, in the docs (S1)

The direct beam exposes each peer's address to the other and traverses NAT to
build a connection a blue team will read as hole-punching, because it is. S1's
characterisation says so, names the estate-wide "relayed-only" switch as the
answer for an org that will not permit it, and points out that the beacon, the
flare and the relayed beam all work without it. The direct beam is a throughput
and latency optimisation for estates that permit a direct path, not a
requirement for any capability - every passenger works over the relayed beam.

## Non-goals

- No TURN, now or as a follow-up; the relayed beam is the fallback.
- No new passengers; S5 owns those and rides this `Channel` unchanged.
- No change to Noise, the beam classes or the establishment gate; a direct beam
  is still a beam, still `--allow-beam`, still step-beam by default.

## Done when

- Two peers that can reach each other directly bring up a data channel via ICE
  and STUN, complete the Noise handshake, and carry an S5 passenger over it.
- A pair that cannot find a direct path falls back to the relayed beam with no
  change visible to the passenger, and the connection record says "relayed".
- A tampered ICE candidate from the broker fails the sealed-signalling check
  and the negotiation aborts rather than connecting to the wrong peer.
- An estate marked relayed-only never negotiates a direct beam, proved with a
  station that has `--allow-direct-beam` set and still ends up relayed.
