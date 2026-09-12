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

Or from a clone, which is what you want if you intend to edit the skill:

```bash
git clone https://github.com/dbhq-uk/heliograph
cd heliograph && ./install.sh
```

That one symlinks the whole skill directory into `~/.claude/skills/`, so every
edit is live with no re-run. Claude Code substitutes `${CLAUDE_SKILL_DIR}` to
the skill's own directory, which is what makes a pure symlink install possible;
[Codex does not, so its installer differs](/codex).

For any other agent - Cursor, Copilot, Windsurf, Gemini, Cline:

```bash
npx skills add dbhq-uk/heliograph
```

**The skill drives the `heliograph` binary**, so [install that too](/install).
If it is missing, the skill stops and prints the install command rather than
improvising around it: the read-only gates live in the CLI and the station,
and one driver is what keeps them in one place.

For typed tools rather than a taught CLI, add the [MCP server](/mcp) instead:

```bash
claude mcp add heliograph -- heliograph mcp
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

## Can Claude Code run commands without SSH?

Yes, and it is the only way this runs them: no SSH is involved anywhere in the
loop. In the estates this is for, there is no SSH to give. The machine is
behind a bastion you are not on, or in a client's estate where only their staff
may log in, or the access is blocked by policy rather than capability:
restricted, change-controlled, somebody else's sign-off.

heliograph does not get around any of that, and it is worth being plain that it
does not try. It does not tunnel, proxy or hold a connection open, and there is
nothing in it to punch through a firewall with. **Every command runs on the far
side because somebody with legitimate access chose to run it.** What changes is
that they run one command, once, and then stop being your terminal.

This is also not Claude Code's Remote Control, which drives a session on your
own machine from your phone. That is for a machine you can already reach.
heliograph is for one that neither you nor the agent can log into.

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
`--allow-actions`. The agent can ask; the station decides. None of this is
Claude Code's own permission system, and skipping that changes nothing here:
[where the gates sit](/security#claude-code-permissions-sandboxes-and-where-the-gates-sit).

## The pieces

The skill is one component. The full set, all in
[one repository](https://github.com/dbhq-uk/heliograph):

| | |
|---|---|
| the skill | the method, the workflow, and how to write a step. Drives the CLI |
| the CLI | `heliograph bootstrap`, `send`, `watch`, `logs --gaps`, `plant` - and the gates |
| the MCP server | `heliograph mcp`, the same CLI as typed tools |
| the station | plain bash on the far side, nothing to install, planted by the CLI |
| the relay | when there is no git host, no storage and no share |
