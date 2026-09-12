<div align="center">

<img src="site/assets/logo.svg" alt="heliograph, by DBHQ" width="120">

# heliograph

**Remote, captured, auditable execution on a machine you cannot log into**

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docs](https://img.shields.io/badge/docs-heliograph.dbhq.uk-4E7FB3)](https://heliograph.dbhq.uk)

A free, open-source tool by [DBHQ](https://dbhq.uk)

</div>

---

Someone can reach the machine. You cannot, and you are the one who knows what
to ask it. heliograph runs that gap as a loop rather than a relay: you publish
a step, it runs on the far side, and the whole run comes back as a log with
every line timestamped in UTC, whether it passed or failed.

```
you        heliograph send net-probe ────────────────▶ transport
station    picks it up within seconds, runs it
           pushes status, then the log ──────────────▶ transport
you        heliograph logs --last --gaps ◀────────────
```

## The three ways across

heliograph carries a request to a machine you cannot log into, and brings the
log back. There are three ways across the gap, and they differ in one thing:
**what stays held, and for how long.**

**Beacon.** You cannot reach the machine and it cannot reach you - but you can
both reach one agreed place. You leave the request there and walk away. Later
the machine passes by, picks it up, runs it, and leaves the log for you to
collect. Nobody is ever connected; a *message* waits in the middle. It is the
safest of the three, because the code being run is already on the far side and
can be read before anything happens - and the slowest, because you wait for the
next visit.

**Flare.** You can reach the machine's door directly. You knock, hand over the
request, wait on the step while it runs, and take the log away in the same
visit. Nothing waits in the middle and no line stays open. Faster, because
there is no pickup to wait for. The trade: you bring the code with you, so the
machine trusts *the door* rather than vetting the code in advance.

**Beam.** You and the machine bring up a connection and hold it open. Either
side can speak at any moment and the other hears it at once, until you hang up.
A real session, not a message or a knock - and the most exposed, because while
the line is open anything can travel down it. You turn it on deliberately and
close it when you are done. It is designed, and not yet a transport you can
pick.

In one line: a beacon holds a *message*, a flare is a *single exchange*, a beam
holds the *connection itself*.

## Does this sound familiar

- You have **no SSH access to production**, and you are not going to be given any.
- The environment is **air-gapped**, or behind a bastion, a jump host or a VPN you are not on.
- It is a **client-owned or customer-managed estate**. Only their staff can log in.
- Access is blocked by **policy, not capability**: regulated, restricted, change-controlled.
- You are on the fourth round of **"can you run this and paste the output"**, and what came back was a screenshot of half a terminal.
- You are an **AI coding agent** driving an investigation, and you need the evidence rather than somebody's summary of it.

If you can just SSH in, you do not need this.

## Three roles, one boundary

The boundary is the gap, and the layout states it once:

| | |
|---|---|
| **control** | your machine: the `heliograph` CLI, `heliograph mcp`, and the skill that drives them. Go, and whatever the near side can afford |
| **transport** | the channel: git, relay, file share, bundle, object store - all behind one interface, so the read-only gates live in one place and cannot drift per transport |
| **station** | the far side: [`station/bash/`](station/), planted into a private transport repo. Bash 4+, git and GNU coreutils. No packages, no credentials, no tunnel |

**Nothing is ever installed on the far side.** The station is plain bash you
can read before you run, and no Go will ever appear under `station/bash/` -
CI enforces it. That constraint is the entire proposition on a locked-down
box where installing anything is its own change request.

## Install

```bash
# Linux and macOS, from a release
curl -sSL https://github.com/dbhq-uk/heliograph/releases/latest/download/heliograph-linux-amd64 \
  -o /usr/local/bin/heliograph && chmod +x /usr/local/bin/heliograph

# or from source
go install github.com/dbhq-uk/heliograph/cmd/heliograph@latest
```

A single static binary, no runtime. Checksums are published with each
release, and the binary carries the station payload it was built with.

**The agent skill** - the same loop, driven from Claude Code, Codex, Cursor
and friends:

```
/plugin marketplace add dbhq-uk/marketplace
/plugin install heliograph@dbhq         # Claude Code
./install-codex.sh                      # Codex, from a clone
./install.sh                            # Claude Code, from a clone
npx skills add dbhq-uk/heliograph       # any agent, via skills.sh
```

## Use

```bash
heliograph bootstrap ~/transport/payments             # plant the station payload
heliograph init payments --dir ~/transport/payments   # git, the default
heliograph plant                                      # what to send the operator
heliograph send net-probe HOSTS="sql01 sql02"         # publish a request
heliograph watch                                      # follow it
heliograph logs --last                                # read the whole log
heliograph logs --last --gaps                         # where it stalled
heliograph doctor                                     # will this work from here
heliograph mcp                                        # serve all of the above as tools
```

The operator's whole job is what `plant` prints: clone the transport repo,
run `./start.sh`, walk away. The loop is **read-only unless the operator said
otherwise**: every step declares itself (`# heliograph-mode: read-only` or
`action`), one that declares neither does not run, and the station refuses an
action unless it was started with `--allow-actions`. It will not run as root
either.

For an agent, `heliograph mcp` is the same CLI as typed MCP tools:

```bash
claude mcp add heliograph -- heliograph mcp
```

The gates do not move. A tool call publishes a request; the station still
decides whether to run it.

`--gaps` is the one worth knowing about. *"Scan the timestamp column for gaps
before reading the content"* is the most valuable instruction in the method,
and it is arithmetic:

```
$ heliograph logs --last --gaps
demo-20260906T183628Z.txt
5 captured lines

1 interval(s) of 10s or more, longest first.
Each is attributed to the line BEFORE it, which is what was running.

   3m12s  after  09:14:02 | Refreshing state...
```

The gap belongs to the line **before** it: the stamp on a line is when that
line was produced, so a long interval means the operation named on the
preceding line is what took the time. A log where every line carries the same
timestamp is reported as an **error**, not as "no gaps".

## Status

| | |
|---|---|
| control CLI over git | works, tested end to end against a stock station |
| `heliograph bootstrap` | works: the binary plants the station it was built with |
| `--gaps` | works |
| MCP server (`heliograph mcp`) | works |
| bash station | in use over git: the loop, the gates, the capture, Azure hosts, Kubernetes, the Windows launcher |
| relay | **half a transport.** The station side is written and complete - it fetches requests, publishes status and delivers the finished log - and the [relay server](https://github.com/dbhq-uk/heliograph-relay) is deployed. No CLI command can select it |
| file share, bundle, object store | **control side only.** The CLI implements all three; the station has no transport for any of them |
| Azure Blob | works end to end, through `drop.sh` in the station payload rather than the CLI. It is what the Azure Function host uses |
| PowerShell station | planned: [A8](docs/specs/2026-09-08-powershell-station-and-full-documentation-design.md) |
| documentation site | [heliograph.dbhq.uk](https://heliograph.dbhq.uk): the CLI, the transports, and the far side - the station, the runner, steps, hosts, Azure, Windows, containers, services, secrets, security and the capture contract |

A transport that works on one side of the gap is not a transport, so this
table names both sides. Git is the one the CLI drives end to end; what the
others still need, and in what order, is
[the roadmap](docs/plans/2026-09-08-powershell-and-docs-roadmap.md).

## The relay

Both sides dial out over ordinary HTTPS, so an estate needs no git host, no
storage account and no VNet. Hosted, and self-hostable from the same binary.

**Not yet usable end to end.** The station side is complete and the server is
deployed; no CLI command can select it, so the near side is the missing half.

**The relay cannot read your logs, and cannot make a station run anything.**
That second half is the one that matters: a relay able to forge a request
would be code execution inside every estate at once. Content is end-to-end
encrypted with keys the relay never holds, and every message is signed.
Nothing bespoke - [age](https://age-encryption.org/v1) primitives plus
Ed25519. The full account, including what DBHQ can and cannot honestly claim,
is in
[`docs/specs/2026-09-06-relay-encryption-design.md`](docs/specs/2026-09-06-relay-encryption-design.md).
The relay server is its own repository,
[dbhq-uk/heliograph-relay](https://github.com/dbhq-uk/heliograph-relay),
because it holds no keys and must be publicly, obviously incapable of reading
anything it carries.

## What it will not do

Give you access you do not have. It does not tunnel, proxy or hold a
connection open to a host you control, and there is nothing here to punch
through a firewall with. A raw TCP transport was considered and **dropped**
for exactly that reason: a persistent reverse connection is a C2 channel by
any blue team's definition, and that sentence is a large part of why this
class of tool is permitted in regulated estates.

Every command runs on the far side because someone with legitimate access
chose to run it.

## Layout

```
cmd/heliograph/         the control CLI, and `heliograph mcp`
cmd/heliograph-seal/    key generation for the relay transport
cmd/heliograph-site/    the static site generator
internal/transport/     git | relay | share | bundle | objstore
internal/bootstrap/     `heliograph bootstrap`: plants the embedded station
internal/wire/          the request and status documents that cross the gap
internal/seal/          sign-then-encrypt, for the relay
internal/logfile/       gap analysis
internal/mcp/           JSON-RPC over stdio, no dependencies
internal/estate/        which transport a name refers to
internal/plant/         what to send the operator
station/bash/           the bash station: everything that runs on the far side
station/bootstrap.sh    the no-CLI bootstrap: clone this repo, run it by hand
skills/heliograph/      the agent skill: drives the CLI, and nothing else
tests/                  the station's own suite, conformance contract included
site/content/           the documentation, one source, three renderings
infra/                  terraform: DNS, Pages, R2 state
docs/specs/             the designs, written before the code
```

The two halves used to be separate repositories, split along Go-versus-bash
rather than along the gap, and every reader had to work out which half they
were looking at. `dbhq-uk/heliograph-skill` was merged in on 2026-09-08 with
its full history; the reasoning is in
[`docs/specs/2026-09-08-station-and-skill-merge.md`](docs/specs/2026-09-08-station-and-skill-merge.md).

## Development

[`PLAN.md`](PLAN.md) is where the work stands: what has landed, what is next,
and which defects are known and unfixed.
[`CONTRIBUTING.md`](CONTRIBUTING.md) covers working on it and
[`AGENTS.md`](AGENTS.md) is for an AI agent doing so. The skill is
[`skills/heliograph/SKILL.md`](skills/heliograph/SKILL.md);
[`docs/dev-setup.md`](docs/dev-setup.md) sets it up from source with live
edits.

## Licence

[MIT](LICENSE) (c) 2026 DBHQ Consulting Ltd
