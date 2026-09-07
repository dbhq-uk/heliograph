# Claude Code

heliograph ships as a **Claude Code skill and plugin**. Install it, and Claude
can set up the transport, write the steps, drive the run and read the log back,
on a machine it has no way to reach itself.

That last part is the point. Claude Code is excellent on a machine it can run
commands on. On a production box behind a bastion, in a client-owned estate, or
on the wrong side of a change-control policy, it cannot run anything at all.

## Install

```
/plugin marketplace add dbhq-uk/marketplace
/plugin install heliograph@dbhq
```

Or for any other agent - Cursor, Copilot, Windsurf, Gemini, Cline:

```bash
npx skills add dbhq-uk/heliograph-skill
```

## Using it

Ask in plain words. The skill knows the rest.

```
heliograph: set up a transport repo for the payments cluster
heliograph a step that proves whether node A can reach node B on 5985
read the log that just came back and tell me what it measured
```

What you decide is which question to ask next. Claude writes the step,
publishes it, waits, and reads the captured log.

## Why not just give the agent SSH

Because in the estates this is for, there is no SSH to give. The machine is
behind a bastion you are not on, or in a client's estate where only their staff
may log in, or the access is blocked by policy rather than capability:
regulated, restricted, change-controlled.

heliograph does not get around any of that, and it is worth being plain that it
does not try. It does not tunnel, proxy or hold a connection open, and there is
nothing in it to punch through a firewall with. **Every command runs on the far
side because somebody with legitimate access chose to run it.** What changes is
that they run one command, once, and then stop being your terminal.

## What the agent gets that a human relay does not

**A log it can trust.** Every line carries a UTC timestamp, ANSI is stripped,
obvious secrets are masked, and the log ships whether the run passed or failed.
No screenshots of half a terminal, no "it said something about DNS".

**A gap it can find.** A hang shows up as a gap in the timestamp column, and
`heliograph logs --gaps` does that arithmetic. An agent reading a log does not
have to notice; it can measure.

**Gates it cannot talk its way past.** A step declares itself read-only or an
action in its own file, and one that declares neither does not run. A
state-changing step needs `CONFIRM=yes` and a station started with
`--allow-actions`. The agent can ask; the station decides.

## The skills

The skill is one component. The full set:

| | |
|---|---|
| the skill | the method, the gates, and how to write a step |
| the CLI | `heliograph send`, `watch`, `logs --gaps`, `plant` |
| the station | plain bash on the far side, nothing to install |
| the relay | when there is no git host, no storage and no share |
