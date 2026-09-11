# heliograph next - master design

**Date:** 2026-09-06
**Status:** provisionally approved, decomposition and roadmap only

heliograph becomes a product: a control-side CLI, a portable far-side payload, six transports behind one interface, a hosted relay, a documentation site, and a family of skills. This document records the decisions and the order. Each workstream gets its own spec before it is built.

## What changes, in one sentence

Today heliograph is a skill that ships a bash toolkit. It becomes an application that gives you remote, captured, auditable execution on a machine you cannot log into, with the skill as the layer that makes it callable by an AI agent.

## The three components

| | |
|---|---|
| **control** | your machine: the `heliograph` CLI, the skill family, you |
| **transport** | the channel: git, relay, object store, file share, bundle |
| **station** | the far side: the box, and the loop running on it |

`step`, `runner` and `operator` keep their current meanings. **`control node` retires.** It was borrowed from Ansible and it points the wrong way once there is a control plane: the machine it named is a station.

`agent` retires as a heliograph term. It is unusable in a product whose primary audience drives AI coding agents, and the confusion would need explaining in every conversation for the life of the project.

### The rename, concretely

```
agent.sh        -> station.sh
agent.ps1       -> station.ps1
agent/request   -> station/request
agent/status    -> station/status
.agent.lock     -> .station.lock
.agent-state    -> .station-state
.agent-approved -> .station-approved
```

Stations already deployed in the field cannot be reached to be upgraded. A compat shim reads the old paths when the new ones are absent, and says so once in the log. It is removed at the first major version, not before.

## The transport interface

This is the crux of the programme and the reason the order below is what it is.

### The problem being fixed

`agent.sh` is 594 lines and `pigeonhole.sh` is 576. They are near-duplicates: the same `field`, `publish_status`, `cleanup`, `is_action_step` and progress logic, differing only in how a request is fetched and a log is put. `intercom.sh` is a third shape again.

Writing the relay, the object store, the file share and the bundle the same way means roughly 2,500 more lines of duplicated loop and four more places for the read-only gates to drift. The interface has to exist **before** the fourth transport is written.

### The contract

A station runs one loop. A transport implements five verbs and two lifecycle calls, and nothing else:

```
transport_fetch_request     -> emit the request document, or nothing
transport_fetch_payload     -> fetch step files (gated; see below)
transport_put_status        <- publish a status document
transport_put_progress      <- publish a partial-log snapshot
transport_put_log           <- publish the finished log

transport_check             -> can this station reach the transport at all
transport_describe          -> which credential is in force, by mechanism, never by value
```

Each transport is one file under `station/transports/<name>.sh`. The loop never learns which one it is talking to.

**All four gates live in the loop.** The mode declaration, `CONFIRM`, `ALLOW_ACTIONS` and the non-root refusal are read and enforced in one place, so they cannot drift per transport. This is the whole point.

### What it fixes in intercom

`intercom.sh` ships the script it wants run, so `heliograph-mode` degrades from a control into a claim the caller makes about its own file. The current design says so honestly rather than dressing it up, which was right, but the hole is still there.

Under the interface, shipping a step becomes `transport_fetch_payload`: a named capability that **every** transport gains, rather than a property of one.

This is a capability gain, not a loss. Today only intercom can ship a script, which is why it is the only transport with the fast write-a-probe, run-it, read-it loop. Afterwards a git or relay station can do the same. What changes is that the capability is declared and gated rather than implied by the transport you happened to pick, so `heliograph-mode` stops silently degrading from a control into a claim.

`--allow-payload` defaults **off**, consistent with `ALLOW_ACTIONS` and every other gate here failing closed. A station started without it runs only steps already present on the far side.

**This is still a breaking change for anyone using intercom today**, because the flag must now be passed. It needs a deprecation note and a version bump, not a changelog line.

### The verb list is incomplete, and self-update is why

Found by reading `station.sh` to plan the extraction rather than by designing it, which is the only reason it was found at all.

The loop **self-updates**. When a pull brings a newer `station.sh`, it re-executes itself into it. That is not a nicety: without it a fix to the station cannot take effect while the station is running, and the operator has to be told to restart, which defeats the whole point of them starting it once and walking away.

It is also **git-specific and does not generalise**. Under the relay there is no repository to pull and no working tree to replace. Under the bundle transport there is no live channel at all. A five-verb interface that quietly drops self-update would ship a relay station that can never be fixed without finding a human on the far side, which is the exact failure mode the product exists to remove.

So the contract gains a sixth verb and an honest admission that not every transport can implement it:

```
transport_fetch_self      -> fetch a newer copy of the station payload, or nothing
```

- **git** implements it as it does today: a pull brings the new file, and the loop re-executes
- **object store and file share** can implement it: the payload is a blob like any other
- **relay** implements it as a signed payload message, which is the same `--allow-payload` capability with the station itself as the payload, and therefore off by default
- **bundle** cannot, and says so: an air-gapped station is updated by carrying a new bundle

`transport_capabilities` reports which verbs a transport actually offers, so the loop can say "this station cannot self-update, you will need to re-plant it to change it" at **start** time rather than discovering it when an update is needed and nobody is there.

The general lesson, worth stating because it will recur: an interface derived from one implementation will be missing whatever that implementation happened to get for free. Git gave self-update away free, so the design never noticed it was a feature.

### Wire protocol v1

The `key: value` request and status documents stay. An operator being able to read `station/request` and understand it without tooling is a real property, and losing it to JSON would buy nothing. They become a versioned protocol, because stations live in the field and the CLI will update faster than they do:

```
version: 1
id:      20260906T101500Z-net-probe
step:    net-probe
env:     HOSTS="sql01 sql02" PORTS=1433
cancel:
stop:
note:
```

A station reading a `version:` it does not know refuses the request and publishes why, rather than guessing.

## The transports

| transport | status | notes |
|---|---|---|
| git | have | remains the default. Nothing about it changes |
| relay | new, flagship | hosted and self-hostable, one implementation |
| object store | generalise | S3-compatible plus the existing Azure Blob path |
| file share | new | any mounted path: SMB, NFS, anything |
| bundle | new | signed tarball, export and import, for a true air gap |
| intercom (HTTP inbound) | keep | narrow case: you can reach the station |

**TCP is dropped.** A persistent reverse connection from the far side to a listener you control is a C2 channel by any blue team's definition, and the README's claim that heliograph does not tunnel, proxy or hold a connection open is a large part of why the tool is permitted in regulated estates. The relay delivers the low-latency loop over ordinary HTTPS without spending that claim.

**Message brokers are parked, not rejected.** AMQP 1.0 and MQTT have real value where an estate already runs one, but no pure-bash client exists, so the station would gain its first dependency. Revisit once the interface has proven itself.

**DNS tunnelling, email and chat transports are declined.** DNS is a covert channel and would get the product banned from the estates it targets. Email genuinely works in mail-only air gaps but carries a large operational burden for a small population. Captured logs in a chat system is a compliance problem, not a feature.

## The relay

A Go server. Both sides dial out over HTTPS. This is the only transport that requires no estate infrastructure at all.

**Long-poll, not WebSocket.** Paseo uses WebSocket because its daemon runs where the user controls the network. A heliograph station runs behind a corporate proxy that may strip the upgrade header, and a transport that fails on those estates fails on exactly the estates this is for.

### Token scopes

Two, because the station token sits on a machine you do not trust and cannot reach.

| token | may |
|---|---|
| station | read requests, write status and logs, for one estate only |
| control | write requests, read logs, for one estate only |

### End-to-end encryption is not optional

A hosted relay would otherwise see everything every command in the estate prints. That is an unacceptable trust ask for regulated customers, who are the customers.

Content is encrypted with a key held only on control and station. The relay stores and forwards ciphertext and can demonstrate it never held the key. This extends `secret.sh`'s existing model from a single value to the whole channel.

**The encryption design is the largest unknown in this programme and gets its own spec** (C1). It decides whether a hosted relay is a product or a liability, and it must not be guessed at here.

### Self-hosting

The same binary, `heliograph-relay serve`, in one container. This is what the "HTTP broker" option meant: one protocol, two deployments, no second implementation.

## The conformance suite

One artefact answering two problems: a second station implementation, and transport drift.

Run against **both** station implementations, across **every** transport, in CI:

1. every captured line carries a **distinct** UTC timestamp
2. a real three-second gap in the source appears as a three-second gap in the log
3. the real exit code survives the pipeline
4. a failed run still writes the footer and still ships
5. an undeclared step refuses, exit 3, having written nothing
6. running as root refuses, exit 5
7. redaction masks each documented shape
8. a cancel mid-run keeps the partial log and publishes exit 130

### Two AGENTS.md constraints change

**Constraint 3** becomes: one *specification* of the capture pattern, whose executable form is the conformance suite.

**Constraint 7** becomes: a second implementation of the capture is permitted **only** while it passes the conformance suite. An unproven port is forbidden.

Constraint 7 currently forbids a PowerShell port outright, and its reasoning is sound: a buffered port gives every line the same timestamp, which reads like a working log while destroying the property the logs exist for. That is an argument about *untested* drift. The suite makes it testable, so the constraint is rewritten rather than weakened or deleted.

## Portability

### The GNU dependency is two spellings

`base64 -w0` becomes `base64 | tr -d '\n'`.

`sed -u` is not needed at all. A pure-bash `while IFS= read -r line` loop is line-buffered by definition, requires no external tool, and removes the dependency entirely. On bash 4.2 and later, `printf '%(%H:%M:%SZ)T'` also removes the per-line `date` subprocess.

Floor is **bash 3.2**, because stock macOS ships 3.2.57 and telling an operator to `brew install coreutils gnu-sed` on a managed Mac is precisely the change request this tool exists to avoid.

Alpine and busybox come free. `start.sh` currently refuses both, correctly, because a busybox `sed` silently produces a log where every line carries the same timestamp. Once the capture uses no `sed`, the refusal can be lifted rather than documented around.

### Windows

`station.ps1` becomes a real loop rather than a launcher, for Windows Server estates with no Git for Windows and no permission to install it. "We support Windows, provided you first install Git for Windows" is a weak claim on exactly the locked-down estates this targets.

The launcher path is kept for machines that do have Git for Windows, because one implementation of the capture is still better than two where there is a choice.

### macOS

`service.sh` gains a launchd path. It currently has systemd and a `setsid` fallback, and no macOS answer at all.

## The control CLI

Go. Single static binary, no runtime to install, cross-compiles to every platform, and it keeps the door open to shipping the station as the same binary later if that ever becomes the right call.

```
heliograph init <estate> --transport relay|git|s3|share|bundle|intercom
heliograph plant --via ssh|azure-run-command|aws-ssm|manual|bundle
heliograph send <step> [k=v...]
heliograph watch
heliograph logs [--gaps] [--since]
heliograph doctor
heliograph station status|stop|cancel
heliograph step new <name>
heliograph bundle export|import
```

### `--gaps` earns its place

"Scan the timestamp column for gaps before reading the content" is currently a discipline asked of a human or an agent, and it is the single most valuable thing these logs support. The CLI computes inter-line deltas and prints the outliers. A discipline becomes a feature.

### `plant` closes a real gap

Planting a station currently requires a human to clone a repo. `--via azure-run-command` and `--via aws-ssm` execute inside a VM with no inbound path at all, through the cloud control plane.

This is the correct home for cloud run-command, and the only one. Azure caps returned output at 4,096 bytes on the action-oriented flavour and AWS routes real output to an S3 bucket. Truncated output is forbidden by this repo's hard rules, so run-command is a useless transport and an excellent installer: 4KB is ample for "did the install succeed".

## The host matrix

**Publish the host contract, then prove a small core.** A station needs very little:

1. a process that can run bash or PowerShell
2. reach to one transport, outbound only
3. a restart policy
4. a non-root account
5. somewhere to write a file it then hands off

No VNet, no storage account, no inbound port, no persistent disk. Git or the relay is the persistence; if the compute dies you run the step again.

Building the relay makes this matrix **smaller**. Several of the five Azure templates exist to solve networking problems - VNet injection, private endpoints, no-egress subnets - that a station on the relay does not have.

| | |
|---|---|
| proven set | systemd, launchd, Windows scheduled task, Docker, Kubernetes, ACI, ECS Fargate, Cloud Run |
| recipes | everything else, documented against the contract, not owned as templates |

The repo currently states honestly that two Azure templates are "written and validates, never deployed". That honesty is an asset. Shipping twenty untested templates would spend it.

## Documentation

### One source, many renderings, with one genuine exception

The 2026 industry consensus is a single canonical documentation source rendered several ways, not separate content for humans and agents. Heliograph follows it, with one exception that is real rather than convenient.

| artefact | reader | source |
|---|---|---|
| docs | humans, and agents answering questions *about* heliograph | one source in `site/`, rendered as HTML, `.md` mirrors, `llms.txt`, MCP |
| skill | an agent *conducting an investigation* | separate, hand-tuned for context budget |
| AGENTS.md | an agent working *on* this repo | separate, already exists, already correct |

`references/method.md` says "never truncate", "keep a control", "change one thing between runs". That is not a description of the product; it is a procedure that changes what the agent does, and it has no reader on a documentation site. It is the same category as the tool and schema definitions that every survey identifies as the one thing genuinely authored twice.

### What the site ships

Astro Starlight at `heliograph.dbhq.uk`, on the shape of paseo.sh: marketing home plus `/docs`.

- `.md` mirror of every page at the same URL plus `.md`. The same page costs roughly 31 times more bytes as HTML than as markdown, so chrome is a token tax on every agent that reads the site. (Remeasured on 2026-09-11 over all 27 built pages: three to sixteen times, about eight on the median page. The saving is real, the figure in this line was never measured, and a test in `cmd/heliograph-site/main_test.go` now holds the claim to the build)
- `llms.txt` and `llms-full.txt` at the origin root, organised by section rather than as one flat list
- guidance blockquote at the **top** of each markdown page, because coding agents truncate long pages to preserve context and anything at the bottom is not read
- a docs MCP server, cheap because the CLI is already Go

The SEO case for `llms.txt` is weak and the docs should not pretend otherwise: one log study found 408 requests to `llms.txt` out of more than 500 million AI bot visits in ninety days. It is shipped because IDE agents fetch it and it costs half a day, not because it will win citations.

### The skill stops restating the docs

`references/runner.md` currently duplicates flag tables that will live on the site. **That duplication is the drift risk**, not the split between audiences. The skill keeps what changes agent behaviour - the method, the gates, the hard rules - and links to the docs for reference material.

The coherence gate survives with a narrower job: assert that facts the skill *does* state (exit codes, `ALLOW_ACTIONS=0`, `PROGRESS_EVERY=60`, file names, image tags) agree with the docs. A much smaller check, and it stops being a workaround for duplication that was chosen rather than inherited.

## Repository

**Two repositories, not one.** The monorepo-with-a-rename in the first draft of this design was wrong, and the reason is worth stating because it is the same reason the whole product exists.

`dbhq-uk/heliograph-skill` is the skill and its bash toolkit. Plain bash, no interpreter, no packages, nothing to install on the far side. That single constraint is what lets it run on a locked-down box where installing anything is a change request, and it is the entire proposition.

Fold this product into that repository and every decision here becomes a negotiation with that constraint. Adding Go, a release pipeline, a web build and a cryptographic dependency to a repo whose promise is "plain bash, you can read it before you run it" does not weaken the promise by argument, it weakens it by proximity: the next person reading it has to work out which half they are looking at.

So they are separate, and the boundary is the gap itself:

| | |
|---|---|
| `dbhq-uk/heliograph-skill` | the far side. Bash and PowerShell, zero dependencies, the method |
| `dbhq-uk/heliograph` | the near side. Go CLI, relay server, transports, site |

```
heliograph/
  cmd/heliograph/         the control CLI                     (Go)
  cmd/heliograph-relay/   the relay server                    (Go)
  internal/transport/     git | relay | objstore | share | bundle
  station/                station-side transport adapters
  station/conformance/    the suite every adapter must pass
  hosts/                  the host contract, and the proven templates
  skills/                 the skill family
  site/                   the documentation site
```

### What crosses between them

The **wire protocol** and the **conformance suite**, and nothing else. Both are specifications rather than code, both are versioned, and both are duplicated deliberately: a copy that must not drift is honest about the coupling, where a shared library would hide it behind an import and let a version skew reach a machine nobody can reach.

The skill repo keeps its own conformance suite for its own capture. This repo keeps one for the transports. They assert the same properties because they are the same contract.

### What must never happen

No Go in `heliograph-skill`. No station-side dependency introduced here that the skill repo is then expected to carry. If a change to this product requires a change over there, it goes as a PR to that repo on its own merits, and it is bash.

## Domain

`heliograph.dbhq.uk`. `heliograph.sh` was available and is the stronger standalone brand, but the subdomain is free, inherits dbhq.uk's authority, and correctly reads as a DBHQ project. Registering `heliograph.sh` later as the primary, with the subdomain redirecting, remains open.

```
heliograph.dbhq.uk             marketing home
heliograph.dbhq.uk/docs        docs
heliograph.dbhq.uk/llms.txt    root of its own origin
relay.heliograph.dbhq.uk       the hosted relay endpoint
```

`heliograph.com` is registered and significant, and the product name is an ordinary English word, so plan to rank for "heliograph agent", "heliograph skill" and "heliograph remote debugging" rather than for the word itself.

## The roadmap

The obvious plan is eight sequential phases, and it is wrong: it puts seven PRs of refactoring in front of anything a user can see. **The CLI does not need the transport refactor**, because it lives on the other side of the gap and can drive today's git station unchanged. So the work runs as parallel tracks with one convergence point.

### Already done, in `heliograph-skill`

Three of these landed before the two-repo split was decided, and they stay where they are because all three are bash-only improvements to that toolkit on their own terms.

| | | |
|---|---|---|
| A1 | conformance suite, 21 properties, plus a mutant driver that must fail | merged |
| A2 | `agent` to `station`, 51 files, with a compat shim for repos in the field | merged |
| F1 | attempted as a rename to `heliograph`; **reverted** in favour of this split | reverted |

A1 is the one that matters most to the work below: it is the safety net that lets the transport refactor prove it changed nothing.

### The first four here, in this order

1. **B1** Go CLI skeleton: `init`, `send`, `logs` against the existing git transport. It drives an unmodified `heliograph-skill` station, which is what makes it independent of everything below
2. **A3** transport contract and wire protocol v1, git as the only implementation
3. **B2** `watch`, `--gaps`, `doctor`
4. **C1** relay protocol and encryption, specced and reviewed before any code

### The tracks

**A - payload.** Gated by A1, always.

| | | |
|---|---|---|
| A1 | conformance suite against the current code | no behaviour change |
| A2 | vocabulary migration, `agent` to `station`, compat shim | mechanical |
| A3 | transport contract and wire protocol v1, git as sole implementation | gates C and D |
| A4 | port the blob transport onto the interface | deletes roughly 500 duplicated lines |
| A5 | port intercom, add the `--allow-payload` gate | **breaking change** |
| A6 | run the conformance suite across every transport in CI | |
| A7 | remove the GNU dependency; stock macOS, Alpine, busybox | lifts `start.sh`'s refusal |
| A8 | `station.ps1` as a native loop, passing conformance | gates E for Windows |
| A9 | launchd path in `service.sh` | |

**B - control CLI.** Gated by A1 only; runs parallel to all of A.

| | |
|---|---|
| B1 | Go CLI skeleton, monorepo layout, `init`/`send`/`logs` on the existing git transport |
| B2 | `watch`, `--gaps`, `doctor` |
| B3 | `plant --via ssh|manual` |
| B4 | `plant --via azure-run-command|aws-ssm` |

**C - relay.** Gated by A3 and B1.

| | |
|---|---|
| C1 | relay protocol and end-to-end encryption design, specced and reviewed before any code |
| C2 | relay server, self-hostable, single container |
| C3 | relay transport in the station and the CLI |
| C4 | hosted relay at `relay.heliograph.dbhq.uk`: deployment, tokens, terms |

**D - transports.** Gated by A3.

| | |
|---|---|
| D1 | generic object store, S3-compatible plus the existing Azure path |
| D2 | file share |
| D3 | bundle: signed export and import |

**E - hosts.** Gated by A2, and by A8 for Windows.

| | |
|---|---|
| E1 | the host contract; unproven templates marked as recipes or retired |
| E2 | proven core: Docker, Kubernetes, systemd, launchd, Windows scheduled task |
| E3 | one proven host per cloud: ACI, ECS Fargate, Cloud Run |

**F - product.** F1 early, the rest late.

| | |
|---|---|
| F1 | monorepo restructure, repo renamed, redirects, marketplace update |
| F2 | site scaffold, marketing home, `llms.txt`, `.md` mirrors |
| F3 | move reference material from the skill to the site; slim the skill |
| F4 | split the skill into the family |
| F5 | ship to every AI agent platform, plus the docs MCP server |
| F6 | CI coherence gate: facts stated in the skill agree with the docs |

29 PRs. Track B starts fourth in wall-clock order and never blocks on track A, which is what removes the dead runway.

### Milestones

Counts are PRs merged in wall-clock order across all tracks, not track labels.

| milestone | after | what is new |
|---|---|---|
| CLI | ~7 merged | drive an existing estate from a real CLI; `--gaps` makes gap detection a feature |
| Portable | ~12 merged | stock macOS with no Homebrew, Alpine, busybox, Windows Server with no Git for Windows |
| Relay | ~18 merged | zero-infrastructure heliograph: no git host, no storage account, no VNet |
| Product | ~26 merged | site, docs, skill family, every AI platform, docs MCP server |

## What is deliberately not in this programme

- **A desktop or web UI.** The CLI's command surface has to stabilise first, and a UI built on a moving surface is rework. Revisit after the relay ships
- **A near-side daemon.** Paseo needs one because it manages long-lived agent processes. heliograph's control side is request-and-read, and a daemon would be a dependency with no job
- **Message broker transports.** Parked; see above
- **Any Paseo integration.** Paseo is a model for the site and product shape, not a dependency. The two are adjacent rather than competing: Paseo orchestrates agents on a machine you control, heliograph runs work on a machine you cannot reach

## Open questions, to be settled in their own specs

1. **Relay encryption** (C1). Key exchange, rotation, what the relay may see in metadata, and what happens when a station's key is lost. The largest unknown here
2. **The compat shim's lifetime.** Removed at the first major version, but that version is not scheduled
3. **Whether the station ever becomes a Go binary.** Deliberately left open. The Go CLI makes it cheap if it becomes right, and "it is just bash, you can read it before you run it" is a real part of why regulated estates permit this. Not a decision for now
