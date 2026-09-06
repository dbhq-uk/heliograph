# AGENTS.md

Guidance for AI agents (and people) working in this repository.

## What this is

The **heliograph product**: a control-side CLI, a relay, and the transports
between them. Remote, captured, auditable execution on a machine nobody present
can debug.

It is **not** the skill. That is
[`dbhq-uk/heliograph-skill`](https://github.com/dbhq-uk/heliograph-skill), it is
plain bash, and it works today.

## The boundary that must not be crossed

`heliograph-skill` promises: plain bash, no interpreter, no packages, nothing to
install on the far side. That is what lets it run on a locked-down box where
installing anything is its own change request.

**Never propose Go, a binary, or any new dependency for that repository.** If
work here needs a change there, it goes as a PR to that repo on its own merits,
and it is bash. The two repositories exist precisely so this pressure never has
to be argued about case by case.

The line runs along the gap:

| | |
|---|---|
| far side, `heliograph-skill` | bash and PowerShell, zero dependencies |
| near side, here | Go, and whatever the near side can afford |

The one exception is `heliograph-seal`, the crypto helper for the relay
transport, and it is argued for explicitly in the C1 spec rather than assumed.
It is a single-purpose signed binary, checksum-verified before execution, and
only the relay transport uses it. Every other transport stays pure bash.

## The constraints inherited from the skill

These came from investigations that went wrong first. They are not negotiable
here either.

**1. Every captured line carries a UTC timestamp.** In an untimed log a hang and
slow progress are indistinguishable, and a gap in the timestamp column is the
only way to tell which operation stalled and for how long. Do not strip it, do
not batch output and stamp it at the end, and do not buffer a command's output
before printing it: every line then carries the same time, which is worse than
no timestamp because it looks like one.

**2. A failed run still ships, and a failed push never loses a log.** Each round
trip through an operator is expensive and none may be wasted by tooling that
only reports success.

**3. One specification of the capture pattern**, whose executable form is the
conformance suite. Implementations are permitted only while they pass it.

**4. Read-only until earned, and every gate fails closed.** A step declares
itself; one that declares nothing does not run. `ALLOW_ACTIONS` defaults to 0.
`--allow-payload` defaults to 0.

**5. Never truncate.** No `head`, no `tail -20`, no `2>/dev/null` on the thing
being diagnosed. This is why cloud run-command is a way to *plant* a station and
never a transport: Azure caps returned output at 4,096 bytes.

## The two rules specific to this repository

**The relay is outside the trust boundary, in both directions.** It may not read
content and it may not cause a station to run anything. Confidentiality is a
privacy feature; authenticity is what stops the relay being a weapon. Any change
that lets the relay see plaintext, or that would let a forged message reach a
station, is wrong regardless of what it buys.

**No bespoke cryptography.** age primitives and Ed25519, through vetted
libraries. If a design seems to need something novel, the design is wrong.

## House style

British English, plain hyphens, **no em dashes**, no trailing full stops on
headings. Say what happened and what to do about it.

Commit messages and PR descriptions say what changed and *what it cost* - the
measurement, the failure it prevents, the thing that was tried and did not work.
A commit message that only restates the diff is a wasted one.

## Validating a change

Nothing is built yet. As code lands, this section gets the exact commands. Until
then: specs are reviewed by reading them, and `docs/specs/` is the source of
truth for what is being built.
