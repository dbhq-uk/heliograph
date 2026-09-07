# The method

The tooling exists to serve this, not the other way round. Every rule here was
paid for by an investigation that went wrong first.

## The hard rules

**1. Never truncate.** No `head`, no `tail -20`, no `2>/dev/null` on the thing
being diagnosed. The line you cut is the one you needed.

**2. Measure, do not infer.** Say what a log showed, not what it implies.

**3. Keep a control.** A probe with nothing to compare against is an anecdote. A
passing probe beside a failing one is what tells you what the failure means.

**4. Change one thing between runs.** Two changes and a different result tells
you nothing.

**5. Read-only until earned.** A step changes state only when you can say
precisely what it will do and why, and then it carries the `CONFIRM=yes` gate.

**6. Never ask the operator to hand-edit anything.** Deliver a change as a
payload the step copies into place. An unlogged manual edit is exactly the
divergence these logs exist to rule out.

## Baseline before theorising

```bash
heliograph send env
```

`env` is the right first step of any investigation, whatever it turns out to be
about: OS, tools, sudo, proxy, DNS, cloud auth, and which commit of the repo is
actually checked out. A divergence between the commit you pushed and the one
they ran explains a surprising share of "but I fixed that".

## Reading a log

**Header block first.** Branch, commit, host, user.

**Then scan the timestamp column for gaps, before reading any content.** In an
untimed log a hang and slow progress are indistinguishable. `--gaps` does this
arithmetic for you.

**Read the whole log, including the parts that worked.** That is where the
control is.

**Record what was measured separately from what you concluded.** Measurements
stay true; conclusions get revised.

```diagram gap
A hang and slow progress look identical in an untimed log. With a UTC stamp on every line, the hang is arithmetic.
```

## Safety, and what it actually rests on

The loop is **read-only unless the operator started it otherwise**. A step
declares itself in its own file, one that declares neither does not run at all,
a state-changing step needs `CONFIRM=yes`, and the station refuses actions
outright unless started with `--allow-actions`.

**The account is the blast radius.** This toolkit holds no credentials of its
own, so "what could this do" is answered entirely by the account it runs as.
Every runner refuses to run as root.

A declaration is a statement by a step's author, checked at the boundary. It
does not stop an author declaring `read-only` and then writing `rm -rf`, and
nothing in a shell runner can. It makes the classification explicit and
machine-checked rather than guessed from a filename.

## Secrets

Logs are committed and pushed, so anything a command prints is in that history
permanently.

Redaction masks the obvious shapes on the way out. **It is a safety net, not a
guarantee.** Do not run things that print secrets, and keep the transport
private.

**Name secrets, never read them.** Listing secret *names* settles "does this
exist here". The value is never the question.
