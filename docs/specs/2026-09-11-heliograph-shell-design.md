# heliograph shell, and the SSH front door

**Date:** 2026-09-11
**Status:** draft, for review
**Part of:** [the signalling toolkit](../plans/2026-09-11-signalling-toolkit-roadmap.md) (S3)

You SSH into the control node you already own, land in a heliograph prompt, and
every line you type runs on the station and streams its log back. It is not a
tunnel and it holds nothing open to the far side: each line is one independent,
gated, logged step - `send` and `watch` in a loop, reached over SSH. The held
connection is to your own box.

This is the discrete shell, over a `Transport`. The live version - a real PTY
and an `ssh` passthrough over the beam - is S5, and arrives when the beam does.

## The shape

`heliograph shell <estate>` opens a REPL. For each line:

1. Author an **ephemeral step**: write the line to a temp step file with a mode
   header the shell sets (`# heliograph-mode: read-only` by default).
2. `PutRequest` it over the estate's transport, with a fresh `NewID`.
3. Stream the captured log back and print it, then prompt again.

The history of the session is the sequence of steps and their logs, and it is
kept - see the transcript below.

## Which transports it runs over

An interactive shell needs the typed line to reach the far side *as code*, and
it needs the answer back fast enough to feel like a shell. Only a carrier that
**ships the script with the request** does both:

- **flare (now).** The script travels in the request body (S2), and the answer
  comes back in seconds. This is the shell's home.
- **beam (S5).** The live channel is the best fit of all; S5 gives the shell a
  streaming backend over it.

**A shell over a beacon is a non-goal for v1**, and the reason is honest rather
than lazy: a beacon runs a *planted* step named by the request, so an ad-hoc
line would have to be committed to the transport repo per keystroke-worth of
command, and the answer waits a poll interval. That is a bad shell - noisy
history, multi-second echoes - and where a beacon is the only carrier, the
right tool is a planted step and `send`, not a prompt pretending to be
interactive. The shell says so and exits cleanly if pointed at a beacon estate.

## The mode gate does not move

The shell is control-side convenience; it grants the far side nothing new.

- **Read-only by default.** Every authored step declares `read-only`, so a
  stock station runs inspection commands and refuses anything that would change
  state.
- **`--allow-actions` marks the steps `action`** - but the station still
  refuses an action unless it was started with `--allow-actions` (or the flare
  endpoint with `FLARE_ALLOW_ACTIONS=1`). An action shell therefore needs the
  switch on *both* ends. The station keeps the last word, exactly as today.
- Because the typed line is caller-authored, its mode header is a *claim*, the
  same trade the flare already makes and states. On a beacon it would be
  committed and readable; on a flare the endpoint's key and allowlist are the
  gates. The shell repeats this in one line at start-up so the operator knows
  what they enabled.

## Session state, emulated

A shell keeps `cwd` and environment between commands; a heliograph step runs
fresh each time. The shell emulates the common case client-side:

- `cd` and `export` are intercepted and tracked locally, then **prepended to
  each authored step** so `cd /var/log; tail -n 50 syslog` behaves.
- What it cannot carry is stated at start-up: no subshell state, no `source`,
  no shell functions or aliases - because each step is a fresh bash on the far
  side, not a continued session. Best-effort, and honest about the edge.

## Interrupt is cancel

`Ctrl-C` while a line is running sends a `Cancel` for that run's id, using the
tree-cancel the loop already has, and returns to the prompt. It does not kill
the shell; a second `Ctrl-C` at an idle prompt exits. The far side stops what it
was running and publishes the partial log, which the shell prints - a killed
step's log is evidence of how far it got, the same rule the capture keeps
everywhere.

## The transcript is kept on the control side

A heliograph log is evidence, and a shell session is a sequence of them. The
shell writes each step and its log to a local transcript directory
(`ops-logs/shell-<UTC>/NNNN-<cmd>.txt`), regardless of transport. This matters
because the transports differ: over a beacon on git the commit history is a
durable transcript already, but over the relay the messages are deleted on
acknowledgement, so the control side is the only place a record survives. The
shell keeps one everywhere so the record does not depend on which carrier was
used.

## The SSH front door

"SSH into heliograph" is standard OS configuration, not new code in heliograph.
The control node is a machine you own and already reach; heliograph only
supplies the program that runs when you land.

- heliograph ships `heliograph shell <estate>`.
- You wire it to OpenSSH as a **forced command** for a dedicated user:
  `command="heliograph shell payments"` on the key in `authorized_keys`, or the
  user's login shell set to a small wrapper that execs it. OpenSSH owns the
  keys, the MFA, the connection logging and the rate limiting; heliograph opens
  **no listener** and grows no auth code.
- `ssh helio@control` then drops the caller straight into the payments shell,
  every line running on that estate's station.
- The docs carry the `authorized_keys` line and an `sshd` note, and the
  security page states the obvious: the account this runs as on the control
  node is the blast radius on the *near* side, and it should be a dedicated,
  unprivileged one whose only power is to reach the estate.

## Non-goals

- No live PTY, no `ssh` passthrough - that is S5, over the beam.
- No shell over a beacon (documented above).
- heliograph does not run an SSH server. OS OpenSSH does the front door.

## Done when

- `heliograph shell <flare-estate>` runs a typed line on the station and prints
  its log, with `cd`/`export` carried across lines.
- A read-only shell against a stock station refuses a state-changing line, and
  the same line runs only when both ends allow actions.
- `Ctrl-C` cancels an in-flight line and returns to the prompt; the partial log
  prints; an idle `Ctrl-C` exits.
- A session leaves a complete local transcript whatever the transport.
- Pointed at a beacon estate, the shell explains why and exits non-zero rather
  than pretending.
- An `authorized_keys` forced-command line drops an SSH caller into the shell,
  documented and proved by hand against a real `sshd`.
