<div align="center">

# heliograph

**Remote, captured, auditable execution on a machine you cannot log into**

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-in%20design-orange)]()

A free, open-source tool by [DBHQ](https://dbhq.uk)

</div>

---

## Status

**Early, and working. The CLI drives a real station over four transports.**

| | |
|---|---|
| control CLI over git | works, tested end to end against a stock station |
| `--gaps` | works |
| file share, bundle | work |
| MCP server (`heliograph mcp`) | works |
| relay | works: [dbhq-uk/heliograph-relay](https://github.com/dbhq-uk/heliograph-relay), deployed to the Cloudflare edge |
| documentation site | [heliograph.dbhq.uk](https://heliograph.dbhq.uk) |
| object store | designed, not built |

The far side is [**dbhq-uk/heliograph-skill**](https://github.com/dbhq-uk/heliograph-skill),
which is complete and in use. This CLI drives it **unmodified** - if you already
run heliograph, this works against what you have today.

## Install

```bash
# Linux and macOS, from a release
curl -sSL https://github.com/dbhq-uk/heliograph/releases/latest/download/heliograph-linux-amd64 \
  -o /usr/local/bin/heliograph && chmod +x /usr/local/bin/heliograph

# or from source
go install github.com/dbhq-uk/heliograph/cmd/heliograph@latest
```

A single static binary, no runtime. Checksums are published with each release.

## Use

```bash
heliograph init payments --dir ~/transport/payments   # git, the default
heliograph init ops --transport share --dir /mnt/ops --scope dns
heliograph init air --transport bundle --dir ~/bundles
heliograph plant                                      # what to send the operator
heliograph send net-probe HOSTS="sql01 sql02"         # publish a request
heliograph watch                                      # follow it
heliograph logs --last                                # read the whole log
heliograph logs --last --gaps                         # where it stalled
heliograph doctor                                     # will this work from here
heliograph mcp                                        # serve all of the above as tools
```

For an agent, `heliograph mcp` is the same CLI as typed MCP tools - one command
to configure, nothing extra to install:

```bash
claude mcp add heliograph -- heliograph mcp
```

The gates do not move. A tool call publishes a request; the station still
decides whether to run it. An MCP client asks, it does not get to answer.

`--gaps` is the one worth knowing about. *"Scan the timestamp column for gaps
before reading the content"* is the most valuable instruction in the method, and
it has always been a discipline somebody has to remember. It is arithmetic:

```
$ heliograph logs --last --gaps
demo-20260906T183628Z.txt
5 captured lines

1 interval(s) of 10s or more, longest first.
Each is attributed to the line BEFORE it, which is what was running.

   3m12s  after  09:14:02 | Refreshing state...
```

The gap belongs to the line **before** it: the stamp on a line is when that line
was produced, so a long interval means the operation named on the preceding line
is what took the time.

## What this is

Someone can reach the machine. You cannot, and you are the one who knows what to
ask it. heliograph runs that gap as a loop rather than a relay: you push a step,
it runs on the far side, and the whole run comes back as a log with every line
timestamped in UTC, whether it passed or failed.

Three components:

| | |
|---|---|
| **control** | your machine: the `heliograph` CLI, the skill family, you |
| **transport** | the channel: git, relay, object store, file share, bundle |
| **station** | the far side: the box, and the loop running on it |

## Why this is a separate repository

[`heliograph-skill`](https://github.com/dbhq-uk/heliograph-skill) is the skill
and its bash toolkit. Plain bash, no interpreter, no packages, nothing to
install on the far side. That single constraint is what lets it run on a
locked-down box where installing anything is its own change request, and it is
the entire proposition.

This product adds Go, a release pipeline, a web build and a cryptographic
dependency. None of those belong in a repository whose promise is *"plain bash,
you can read it before you run it"* - not because they would break it by
argument, but because they would erode it by proximity. The next person reading
that repo would have to work out which half they were looking at.

So the boundary between the two repositories is the gap itself:

| | |
|---|---|
| `heliograph-skill` | the far side. Bash and PowerShell, zero dependencies, the method |
| `heliograph` | the near side. Go CLI, relay server, transports, site |

**No Go will ever be added to `heliograph-skill`.** If something here needs a
change over there, it goes as a PR on its own merits, and it is bash.

## What is being built

**A control CLI.** A single static Go binary. `init`, `plant`, `send`, `watch`,
`logs`, `doctor`. `logs --gaps` computes inter-line deltas and prints the
outliers, which turns "scan the timestamp column for gaps" from a discipline
you have to remember into a feature.

**A relay.** The flagship. Both sides dial out over ordinary HTTPS, so an
estate needs no git host, no storage account and no VNet. Hosted, and
self-hostable from the same binary.

**The relay cannot read your logs, and cannot make a station run anything.**
That second half is the one that matters: a relay able to forge a request would
be code execution inside every estate at once, which is a far worse position
than any disclosure. Content is end-to-end encrypted with keys the relay never
holds, and every message is signed. Nothing bespoke -
[age](https://age-encryption.org/v1) primitives plus Ed25519. The full account,
including what DBHQ can and cannot honestly claim, is in
[`docs/specs/2026-09-06-relay-encryption-design.md`](docs/specs/2026-09-06-relay-encryption-design.md).

**More transports.** Generic object store (S3-compatible), a mounted file share,
and a signed bundle for a true air gap - all behind one interface, so the read-only
gates live in one place and cannot drift per transport.

**A documentation site**, at `heliograph.dbhq.uk`.

## What it will not do

Give you access you do not have. It does not tunnel, proxy or hold a connection
open to a host you control, and there is nothing here to punch through a
firewall with. A raw TCP transport was considered and **dropped** for exactly
that reason: a persistent reverse connection is a C2 channel by any blue team's
definition, and that sentence is a large part of why this class of tool is
permitted in regulated estates.

Every command runs on the far side because someone with legitimate access chose
to run it.

## Layout

```
cmd/heliograph/         the control CLI, and `heliograph mcp`
cmd/heliograph-seal/    key generation for the relay transport
cmd/heliograph-site/    the static site generator
internal/transport/     git | relay | share | bundle
internal/wire/          the request and status documents that cross the gap
internal/seal/          sign-then-encrypt, for the relay
internal/logfile/       gap analysis
internal/mcp/           JSON-RPC over stdio, no dependencies
internal/estate/        which transport repo a name refers to
internal/plant/         what to send the operator
internal/site/          markdown subset, CSS, hero
site/content/           the documentation, one source, three renderings
infra/                  terraform: DNS, Pages, R2 state
docs/specs/             the designs, written before the code
```

The relay server is its own repository,
[dbhq-uk/heliograph-relay](https://github.com/dbhq-uk/heliograph-relay), because
it holds no keys and must be publicly, obviously incapable of reading anything
it carries.

## Licence

[MIT](LICENSE) (c) 2026 DBHQ Consulting Ltd
