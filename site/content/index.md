# heliograph

**Remote, captured, auditable execution on a machine you cannot log into.**

A CLI, and a [Claude Code skill](/claude-code). Someone can reach the machine.
You cannot, and you are the one who knows what to ask it. heliograph runs that
gap as a loop rather than a relay: you push a step, it runs on the far side,
and the whole run comes back as a log with every line timestamped in UTC,
whether it passed or failed.

If you can SSH in yourself, do that instead. This is for when you cannot.

```diagram loop
You push a step; the station runs it; the log comes back. The operator runs one command, once.
```

## Does this sound familiar

- No SSH access to production, and you are not going to be given any
- Air-gapped, or behind a bastion, a jump host or a VPN you are not on
- A client-owned estate where only their staff can log in
- Blocked by policy rather than capability: restricted, change-controlled, somebody else's sign-off
- The fourth round of "can you run this and paste the output", and what came back was a screenshot of half a terminal

## For an AI agent that cannot reach the machine

Claude Code is excellent on a box it can run commands on. On a production
machine behind a bastion, in a client-owned estate, or behind a change-control
policy, it cannot run anything at all.

heliograph gives it a way to ask: publish a step, and read back a log with every
line timestamped, whether the run passed or failed. The operator runs one
command, once, and stops being anybody's terminal.

[How it works with Claude Code](/claude-code), or drive it as typed tools from
any agent through the [MCP server](/mcp).

## The three components

| | |
|---|---|
| **control** | your machine: the `heliograph` CLI, the [skill](/claude-code) that drives it, and you |
| **transport** | the channel: git, relay, file share, bundle, object store |
| **station** | the far side: plain bash - or plain PowerShell, for an estate with no bash - planted by `heliograph bootstrap`, and the loop running on it |

One repository carries all three, and the boundary is the gap: everything
under `station/` runs on the far side and depends on nothing - bash 4+, git,
GNU coreutils. No Go will ever cross it, and CI enforces that.

## Two properties that make a log-only loop workable

**Every line carries a UTC timestamp**, so a hang shows up as a *gap*. After the
fact, in an untimed log, a hang and slow progress are indistinguishable. This is
the single most useful property of these logs, and `heliograph logs --gaps`
turns reading it from a discipline into arithmetic.

**The exit code survives and the log ships even on failure**, so a failed run
reads as clearly as a successful one and a round trip is never wasted.

## What it will not do

Give you access you do not have. Two of its three shapes, the beacon and the
flare, never tunnel, proxy or hold a connection open to a host you control,
and there is nothing in either to punch through a firewall with. A raw TCP
transport was considered and dropped for exactly that reason: an
unauthenticated, always-on reverse connection is a C2 channel by any blue
team's definition, and that sentence is a large part of why this class of tool
is permitted in the estates it targets.

The third shape, the beam, is a live connection and does hold a line open -
deliberately, off by default, and under [its own
controls](/security#the-beam-and-the-blast-radius-of-a-held-open-line). Where
an estate forbids a reverse connection outright, the beacon and the flare still
do the whole job without one.

Every command runs on the far side because someone with legitimate access chose
to run it.
