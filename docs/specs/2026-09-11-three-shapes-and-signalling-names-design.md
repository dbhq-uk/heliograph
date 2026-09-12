# Three shapes on one medium, and the end of "regulated"

**Date:** 2026-09-11
**Status:** draft, for review
**Part of:** [the signalling toolkit](../plans/2026-09-11-signalling-toolkit-roadmap.md) (S1)

The foundation of the program. Before a line of the beam is written, the words
have to change: the two shapes become three, they take their names from
signalling, and the positioning that said heliograph refuses to be a tunnel is
retired, because the beam is one. This spec is prose, generated content and a
rename strategy. It writes no Go and no bash of the transports themselves; that
is S2 and S4.

## What it delivers

1. The three shapes named on one medium - **beacon**, **flare**, **beam** -
   and the "two shapes" model replaced with three, in the one generated source
   and in the prose that reads from it.
2. Every "regulated estate" reference removed from user-facing content, and the
   framing rebuilt on capability rather than compliance.
3. The "what it will not do" refusal rewritten into an honest characterisation
   of the beam - what it is, its blast radius, when to reach for it, and when
   an estate should forbid it.
4. The rename strategy that lets the code and the environment variables follow
   later without breaking a station in the field.

## The names

Decided in brainstorming. The axis is *what is held, and for how long*, and the
words come from signalling because a heliograph is a signalling device.

| shape | was | now | held | analogy |
|---|---|---|---|---|
| dead drop | pigeonhole | **beacon** | a message, at rest | a signal lit where both can see it, collected by looking later |
| direct call | intercom | **flare** | one transaction | fired straight at a reachable endpoint, one burst, an answer, gone |
| open line | *(new)* | **beam** | the connection | a steady beam held on the far station, live and two-way |

The beam has two variants, and the metaphor names them: a **relayed beam** is
bounced off a relay the way light bounces off a mirror (brokered, both sides
dial out); a **direct beam** is line of sight (a direct peer connection). Their
designs are S4 and S6.

**The docs get one short "three ways across" section**, in plain language, on
the site and in the README. The approved copy is the brainstorm's simple
explanation: beacon is *like leaving a note where you agreed*, flare is *like
knocking and waiting on the step*, beam is *like a phone call left off the
hook*. One line ties it together - a beacon holds a message, a flare is a
single exchange, a beam holds the connection itself.

## Removing "regulated"

Fourteen references in all, and **every one of them goes**. Nine are
user-facing, across seven files:

| file | line | now says | becomes |
|---|---|---|---|
| README.md | 35 | "regulated, restricted, change-controlled" | "restricted, change-controlled, or reached only through people who can" |
| README.md | 179 | "permitted in regulated estates" | folded into the rewritten "what it will do, and what that costs" section below |
| site/content/index.md | 21 | "regulated, restricted, change-controlled" | as README:35 |
| site/content/index.md | 66 | "permitted in regulated estates" | as README:179 |
| site/content/security.md | 5 | "regulated estate" | "an estate you cannot log into" |
| site/content/claude-code.md | 66 | "regulated, restricted, change-controlled" | as README:35 |
| site/content/relay.md | 71 | "regulated customers, who are the customers" | "customers who cannot let a third party read what their machines print" |
| site/content/dbhq.md | 41 | "in regulated estates" | "in estates with no route in" |
| site/content/roadmap.md | 64 | "the regulated market" | "the enterprise git market" |

The principle: describe the *situation* (no route in, policy blocks access,
someone else holds the keys) rather than the *label* (regulated). The tool was
never really about regulation; it was about the gap, and the gap is what the
words should name.

**The two historical specs are scrubbed as well.** They mention "regulated" as
rationale for decisions made then, and the word goes from those too, so a
repository-wide grep is clean and the CI guard needs no carve-out. Decided
deliberately: a documented exception is a thing that has to be explained every
time somebody greps, and the sentences read the same once the label is gone -
what they argue is a property of the estate, not of its regulator.

| file | line | now says | becomes |
|---|---|---|---|
| `2026-09-06-heliograph-next-design.md` | 129 | "permitted in regulated estates" | "permitted in the estates it targets" |
| `2026-09-06-heliograph-next-design.md` | 152 | "an unacceptable trust ask for regulated customers, who are the customers" | "an unacceptable trust ask for customers who cannot let a third party read what their machines print, who are the customers" |
| `2026-09-06-heliograph-next-design.md` | 443 | "why regulated estates permit this" | "why these estates permit this" |
| `2026-09-10-new-transports-and-stations-design.md` | 133 | "Every regulated estate that refuses" | "Every estate that refuses" |
| `2026-09-10-new-transports-and-stations-design.md` | 380 | "the regulated market" | "the enterprise market" |

Only the label is removed. The arguments those passages make are left standing,
including the two this programme overturns - `2026-09-06:129` dropping the
reverse connection, and `:443` leaving open whether the station ever becomes a
Go binary. S4 answers both, and a reader comparing them can see that it did.

## The refusal becomes a characterisation

Today four places say heliograph does not tunnel and that a reverse connection
was dropped for being a C2 channel:

- README.md, "What it will not do" (the raw TCP paragraph)
- site/content/security.md
- `docs/specs/2026-09-10-new-transports-and-stations-design.md:53` - the
  "raw TCP, reverse tunnel | never" survey row
- `docs/specs/2026-09-06-heliograph-next-design.md:129` - "TCP is dropped"

The beam is that connection. These do not get deleted - deleting them would
lose the argument a reader needs to make the decision. They get rewritten from
*"we refuse this"* to *"this is what it is, and here is the switch"*:

- **README "What it will not do" becomes "What the beam is, and what it
  costs".** It states plainly: the beam holds a live, two-way connection to a
  machine you cannot otherwise reach; that is a tunnel, and a blue team will
  read a held-open channel as one; it is **off unless explicitly enabled**, on
  both sides; and where an estate forbids a reverse connection, the beacon and
  the flare still do the whole job without one. The honesty that made the old
  paragraph trusted is the point - the new one is not spin, it is the same
  plainness pointed at a capability the tool now has.
- **security.md gains a beam section** covering the blast radius (a live
  channel is arbitrary interactive access for whoever holds the line), the
  enable gates (off by default, explicit on both ends, signed establishment),
  and the guidance that the beam is the shape you turn on deliberately and
  close when done.
- **The survey row** changes from "never" to a pointer at S4/S6, keeping the
  reason the raw form was refused (an unauthenticated, always-on reverse
  connection) and distinguishing it from the beam (off by default, sealed,
  signed, and torn down when idle).

This spec writes the words. The gates those words describe are built in S4 and
S5; S1 must not claim a switch that does not exist yet, so the beam prose uses
the project's existing "written, not yet proven" convention until S4 lands.

## The generated source, and the prose that reads it

The matrix is generated from one place, `cmd/heliograph-site/main.go`, so the
model changes there first and the pages cannot disagree with it.

- **`cmd/heliograph-site/main.go`** - the shape vocabulary moves from two to
  three. Wherever the code enumerates or labels "pigeonhole" and "intercom",
  it gains "beam", and the descriptions become the beacon/flare/beam ones.
- **site/content/matrix.md** - "Every transport is one of two shapes" becomes
  three; the table gains the beam row; "Everything shipped is a pigeonhole"
  becomes "Everything shipped today is a beacon", with the flare and the beam
  named as what is arriving.
- **site/content/transports.md** - the pigeonhole-versus-intercom opening
  becomes the three-shape one, and "All six below are pigeonholes" becomes
  "All six below are beacons".
- **The navigation** - the `/intercom` page is renamed `/flare` with a
  redirect from the old path, so no external link 404s.

## The rename strategy

Only the user-facing half is in S1. The code and the environment variables are
a compatibility surface and move in their own PR, still under S1's number but
separately, so a review can reason about breakage on its own:

- **Now (S1 docs PR).** Docs, site, CLI help and any new CLI surface use
  beacon, flare and beam. `/intercom` redirects to `/flare`.
- **Next (S1 rename PR).** `pigeonhole.sh` -> `beacon.sh`, `intercom.sh` ->
  `flare.sh`, `intercom.py` -> `flare.py`, and the `PIGEONHOLE_*` / `INTERCOM_*`
  environment variables gain `BEACON_*` / `FLARE_*` names. **The old names keep
  working**: the station reads the new name first, falls back to the old, and
  warns once when it uses the fallback. The Azure templates and
  `skill_coherence_test.go` move to the new names. A deprecation window is
  stated in the docs and the old names are removed only after it, in a later
  release - not in this program.

## Non-goals

- No control-side transport work. Wiring the relay into CLI selection and
  building the flare transport is S2.
- No shell, no channel, no beam implementation. S3, S4, S5, S6.
- No removal of the old environment-variable names. Aliases only.

## Done when

- **No file in the repository contains "regulat"** - a repository-wide grep in
  CI stays green, with no exclusions (the guard belongs beside the existing
  site guards).
- The matrix renders three shapes from the one generated source, and
  `matrix.md`, `transports.md` and the shape descriptions agree with it.
- The README and `security.md` describe the beam honestly, marked as written
  rather than proven, with no claim of a gate that S4 has not built.
- `/intercom` redirects to `/flare`; no internal link points at a dead path.
- The rename PR leaves every `PIGEONHOLE_*` and `INTERCOM_*` name working, with
  a warning, proved by a conformance test that sets only the old names.
