# Codex

heliograph ships as a **Codex skill**, and as an MCP server. Install either and
Codex can set up the transport, write the steps, drive the run and read the log
back, on a machine it has no way to reach itself.

That last part is the point. Codex is excellent on a machine it can run
commands on. On a production box behind a bastion, in a client-owned estate, or
on the wrong side of a change-control policy, it cannot run anything at all.

## Install the skill

```bash
git clone https://github.com/dbhq-uk/heliograph
cd heliograph && ./install-codex.sh
```

It installs into `~/.codex/skills/heliograph/`.

**Re-run it after editing `SKILL.md`.** That one file is rewritten at install
time rather than symlinked, because Codex does not substitute
`${CLAUDE_SKILL_DIR}` the way Claude Code does - the installer resolves that
variable to the skill's real path on the way in. `references/` *is* symlinked,
so edits there are live immediately.

That is the only meaningful difference between the two installers, and it is
worth knowing because a stale `SKILL.md` fails silently: the skill loads, and
points at a path that moved.

## Or as MCP tools

```bash
codex mcp add heliograph -- heliograph mcp
```

Every CLI command becomes a typed tool. The gates do not move: a tool call
publishes a request, and the station still decides whether to run it. See
[the MCP server](/mcp) for the tool list.

Use the skill when you want the *method* as well as the tools - it carries the
rules that change what an agent does, which a tool schema cannot. Use MCP alone
when you only want the surface.

## The binary is required either way

The skill drives the `heliograph` CLI and does not reimplement it. If the
binary is missing the skill says so and prints the install command rather than
improvising around it:

```bash
curl -sSL https://github.com/dbhq-uk/heliograph/releases/latest/download/heliograph-linux-amd64 \
  -o /usr/local/bin/heliograph && chmod +x /usr/local/bin/heliograph
```

## What Codex gets that a human relay does not

The same thing Claude Code gets, and for the same reason: the evidence rather
than somebody's summary of it.

- **A log with every line timestamped**, so a hang is a *gap* rather than a
  guess. `heliograph logs --last --gaps` turns reading it from a discipline
  into arithmetic
- **The real exit code**, and the log whether the run passed or failed
- **A refusal it can act on.** A gate says which flag would allow it, published
  within one poll, so the agent is not left waiting on a request that will
  never run

## Working on this repository

`AGENTS.md` at the repository root is the guidance for an agent working *on*
heliograph rather than driving it. It is separate from the skill deliberately:
one is a procedure for an investigation, the other is house rules for a
codebase.

## Start here

[The quick start](/quickstart) is the same for every agent. [The
method](/method) is the part worth reading before a hard investigation - every
rule in it was paid for by a round trip that went wrong first.
