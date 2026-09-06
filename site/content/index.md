# heliograph

**Remote, captured, auditable execution on a machine you cannot log into.**

Someone can reach the machine. You cannot, and you are the one who knows what to
ask it. heliograph runs that gap as a loop rather than a relay: you push a step,
it runs on the far side, and the whole run comes back as a log with every line
timestamped in UTC, whether it passed or failed.

If you can SSH in yourself, do that instead. This is for when you cannot.

## Does this sound familiar

- No SSH access to production, and you are not going to be given any
- Air-gapped, or behind a bastion, a jump host or a VPN you are not on
- A client-owned estate where only their staff can log in
- Blocked by policy rather than capability: regulated, restricted, change-controlled
- The fourth round of "can you run this and paste the output", and what came back was a screenshot of half a terminal

## The three components

| | |
|---|---|
| **control** | your machine: the `heliograph` CLI, and you |
| **transport** | the channel: git, file share, bundle, and more coming |
| **station** | the far side: the box, and the loop running on it |

## Two properties that make a log-only loop workable

**Every line carries a UTC timestamp**, so a hang shows up as a *gap*. After the
fact, in an untimed log, a hang and slow progress are indistinguishable. This is
the single most useful property of these logs, and `heliograph logs --gaps`
turns reading it from a discipline into arithmetic.

**The exit code survives and the log ships even on failure**, so a failed run
reads as clearly as a successful one and a round trip is never wasted.

## What it will not do

Give you access you do not have. It does not tunnel, proxy or hold a connection
open to a host you control, and there is nothing here to punch through a
firewall with. A raw TCP transport was considered and dropped for exactly that
reason: a persistent reverse connection is a C2 channel by any blue team's
definition, and that sentence is a large part of why this class of tool is
permitted in regulated estates.

Every command runs on the far side because someone with legitimate access chose
to run it.
