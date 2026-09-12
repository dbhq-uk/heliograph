# Transports

A transport is the channel a request goes out on and a log comes back through.
The loop is identical whichever you pick: same request format, same gates, same
log. That is deliberate, and it is what lets you change transport without
relearning the method.

```diagram transports
One request format, one set of gates, one log. Only the channel changes.
```

## Choosing one

**Measure before reaching past git.** Git is better when git works, and an
image pull succeeding proves nothing: a container platform pulls on its own
side, so a container can start cleanly on a host with no network at all.

**A transport needs both halves.** The control side publishes a request and reads
a log; the station side picks the request up and sends the log back. One half on
its own moves nothing, so the status column below names both.

| transport | reach for it when | control side | station side |
|---|---|---|---|
| **git** | the far side can reach a git host | works | works |
| **relay** | there is no git host, no storage, no share | works | works |
| **file share** | both machines mount the same directory | works | works |
| **object store** | S3-compatible storage is permitted where git is not | works | none yet |
| **bundle** | nothing crosses the gap but a person | works | none yet |
| **Azure Blob** | a VNet-local private endpoint is the only reachable thing | `drop.sh`, in the station payload, not the CLI | works |

**Git, the file share and the relay are the three the CLI drives end to end
today**, each proved by its own round trip in CI against a real station. Azure
Blob also works end to end, through `drop.sh` in the station payload rather
than through the `heliograph` binary - it is the transport the Azure Function
host uses, and it is deployed. The rest are at the stage the table says and no
further; what each still needs is in
[the roadmap](https://github.com/dbhq-uk/heliograph/blob/main/docs/plans/2026-09-08-powershell-and-docs-roadmap.md).
This page describes each one as designed, so that the design can be reviewed -
not as though you could reach for it this afternoon.

### Selecting one on the station

`TRANSPORT` picks the channel, and it is the same command whichever you pick:

```bash
TRANSPORT=relay ./start.sh --check     # will this work here?
TRANSPORT=relay ./start.sh             # check, then run
```

The preflight asks *that* transport rather than assuming git, so a missing
`RELAY_URL` or an unreachable relay is reported by name, in the same table, with
the same remedies. See [the runner](/runner#it-asks-the-transport-rather-than-assuming-git).

A request may **not** override `TRANSPORT`. The channel is the operator's
decision and the far side does not get a vote on it.

## git

The default. A private repository is the transport in both directions: you push
a request, the station pushes back a status and a log.

```bash
heliograph init payments --dir ~/transport/payments
```

### One repository, several stations

The branch is the channel. Both sides read it from whatever is checked out, so
a repository already talks to as many machines as it has station branches.

```bash
heliograph station add db-a        # branch station/db-a, its own checkout, its own estate
heliograph station add db-b
heliograph send net-probe -e db-a  # reaches that machine and no other
```

`station add` creates the branch and pushes it **with an upstream**, checks it
out into a git worktree beside the existing clone, records the estate, and
prints what to send the operator. Those go together: a pushed branch with no
checkout is invisible, and a checkout with no estate cannot be driven.

The worktree is the point on this side. Talking to three stations means three
checkouts, and switching one checkout between branches is how a request reaches
the wrong machine. One clone, one fetch, one credential, a directory each.

**Branches are routing isolation, not security isolation.** A repository
credential usually spans every branch, so a station that is compromised can
read every other station's logs and push a modified `station.sh` to their
branch - and pinning does not cover `station.sh`, so the victim drops its own
gates at its next self-update.

> Put several stations in one repository only where they share a blast radius.
> Across a trust boundary, separate repositories are the isolation mechanism
> rather than untidiness.

Two stations on one branch is a configuration error, not a supported mode: both
answer every request and race on the push. Git refuses to check one branch out
in two worktrees, which catches it locally; nothing catches it across machines,
so the published status names the host that answered.

### The credential

This is where a git station actually fails, so it is worth settling before the
operator walks away rather than an hour into the first run.

**An ssh remote with an agent key needs nothing from heliograph.** Plain
`git push` works, and it is the setup to prefer. The catch is that a forwarded
agent key **dies with the session**, which is exactly when an unattended loop
needs it. A key on disk or a deploy key is what survives a logout.

For an `https://` remote, the station looks for a token in this order and stops
at the first one it finds:

| | |
|---|---|
| `GIT_AUTH_HEADER` | the full header value, used verbatim |
| `GIT_TOKEN` | a token, sent as HTTP Basic |
| `GIT_TOKEN_FILE` | a file whose **first line** is that token |
| `./.git-token` | in the transport repo |
| `~/.git-token` | the one a detached service can read |

`GIT_TOKEN_USER` is the Basic username. It defaults to empty, which is what
Azure DevOps wants. **GitHub needs `x-access-token` and GitLab needs `oauth2`**,
and getting it wrong reports a *missing* username rather than a wrong one,
which sends you looking at the token.

Why a token at all, when the URL can carry one: several hosts reject the
`https://user:token@host` form outright, so the push fails as a bare
authentication error while the token is perfectly valid.

### It is passed through the environment, not the command line

`git -c http.extraHeader=...` puts the value in this process's argv, and
`/proc/<pid>/cmdline` is world-readable - any other user on the box can read
the token straight out of `ps`. The station passes it through
`GIT_CONFIG_COUNT` instead, and `/proc/<pid>/environ` is owner-only.

A reduction in exposure, not a guarantee: root reads either, and a core dump
sees the value whichever route it took. A heliograph station is often a shared
jump host in somebody else's estate, and this token is frequently the only
credential the tool is trusted with.

It needs git 2.31 or newer. Older git ignores `GIT_CONFIG_COUNT` **silently**,
so the station checks the version rather than hoping, and falls back to `-c`.

### Prove it before you need it

```bash
./start.sh --check
```

It reports which credential is in force - by mechanism and length, never by
value - and then **proves the push**, because read access is not write access
and the expensive failure is an hour-long step that captures a perfect log and
cannot deliver it.

The write check dry-runs against `refs/heads/heliograph-write-check`, a ref
that does not exist. That is deliberate: dry-running against the current branch
is refused locally as a non-fast-forward the moment origin holds a commit the
checkout lacks, which is the ordinary state every time, and the credential gets
blamed for it.

Two things it cannot see, said plainly: a local filesystem remote whose
directory is not writable, and a pre-receive hook or branch ruleset. A dry run
sends no pack, so those hooks never execute.

### Scoping it

Read and write on that one repository, and nothing else. A deploy key or a
fine-grained token is the right shape. A read-only deploy key and a token
missing the write scope both look exactly like a failed write check, which is
what the message says.

**Two writers, one branch.** The station pushes far more often than you do: a
status commit on every transition, a progress snapshot every 60 seconds, and the
log itself. So the remote moves while you are writing the next request and a
plain push is rejected. That is not a fault, it is the design working, and the
CLI rebases rather than forcing. A force push here would destroy the evidence
the loop exists to deliver.

## File share

The cheapest transport there is, and it covers a real population: an estate that
will not open an egress path, will not provision a storage account and will not
permit a git host quite often already has a share both machines mount, because
that is how everything else in the estate moves files.

```bash
heliograph init ops --transport share --dir /mnt/ops --scope dns-timeouts
```

**The mount is the credential**, which is also the whole security model. Anyone
who can write to the share can queue a request, so it must be scoped as tightly
as the account the station runs as.

`--scope` is one directory per investigation, so two do not overwrite each
other. Requests are written with write-then-rename: a station polling the
directory can read at any instant, and a partially written request is one with
no `id:` yet, or worse a truncated `env:`, and it would be acted on.

### On the station

```bash
TRANSPORT=share SHARE_DIR=/mnt/ops SHARE_SCOPE=dns-timeouts ./start.sh
```

Three files and one directory, and both sides agree on every path because a
[round trip in CI](/conformance) drives the real CLI against a real station over
a real directory:

```
/mnt/ops/dns-timeouts/request          the control side writes it
/mnt/ops/dns-timeouts/status           the station writes it
/mnt/ops/dns-timeouts/ops-logs/*.txt   the station writes them
```

Everything the station publishes is written with the same write-then-rename,
into the same directory, under a name `mktemp` chose. A rename across
filesystems is not a rename - it degrades to copy-then-unlink - and a share is
by definition a different filesystem from `/tmp`. A half-copied log matters more
than a half-written status: it ends mid-line with no footer, which is the one
shape an operator is trained to read as *still running*.

The temporary's name is unguessable rather than derived from the process id,
and that is worth a sentence. Two stations in two containers commonly have the
same pid; and on a share anyone who can write can plant a file where a
predictable temporary would go.

`./start.sh --check` proves the whole publish and not merely a write: it
creates, renames and removes. An SMB share that permits create but denies
rename or delete is an ordinary way to configure a drop box, and it fails every
publication at the rename while passing any check that only writes a file.

### Two things a share can be wrong about and git cannot

**A typo in `SHARE_SCOPE` is not an error.** The first thing the station
publishes creates that directory, and then both sides run perfectly, for ever,
into two directories that never meet - every symptom pointing at a station that
is asleep. git cannot do this: a branch that does not exist is refused by the
remote. So `./start.sh --check` says out loud when the scope directory is not
there yet, and it does **not** create it: `--check` changes nothing, on a share
as anywhere else.

**A symlink is refused**, for the scope directory and for `ops-logs`. On NFS or
SMB a symlink is resolved by each client separately, so one link can send the
two sides to two different local directories - the same failure again, in the
one shape where both machines look correctly configured.

**The steps have to be there already.** A share carries the request, the status
and the logs and nothing else, so `heliograph send steps/probe.sh` names a step
the station must already have. On git the file travels in the repository; here
it is planted with the payload. This is what `heliograph plant` means when it
refuses a non-git estate with *nothing for the far side to clone*.

It cannot self-update for the same reason, and says so at start rather than
letting you find out when a fix is needed.

## Bundle

The only thing that makes "air-gapped" literally true rather than nearly true.
Everything else still needs some path between the two machines, even if it is a
storage account nobody can route to directly.

```bash
heliograph init air --transport bundle --dir ~/bundles
heliograph send net-probe            # writes request-<id>.hgb
```

Carry the file. Run it. Carry the log back.

It is not a loop and does not pretend to be: a run takes as long as it takes
somebody to walk. The value is that the format, the gates and the log are
identical to every other transport, so the method survives the walk.

## Relay

The only transport that needs no estate infrastructure at all. Both sides dial
**out** over ordinary HTTPS and meet at a server neither of them trusts.

**Not selectable yet.** `heliograph init` knows `git`, `share`, `bundle` and
`objstore`. The relay implementation exists on both sides of the gap and
nothing wires it to a command. Earlier revisions of this page showed an `init`
invocation for it, with two flags that have never existed - the sort of thing
that costs somebody an afternoon before they conclude the tool is broken.

**The relay cannot read your logs, and cannot make a station run anything.** The
second half is the one that matters: a relay able to forge a request would have
code execution inside every estate at once, through a channel the estate
installed deliberately. Everything is sealed and signed before it leaves, and
verified before it is acted on.

Nothing bespoke: X25519, HKDF-SHA256, ChaCha20-Poly1305 and Ed25519. The server
holds no keys, and its source is public precisely so that claim can be checked
rather than trusted.

It is the one transport that needs a binary on the far side. `openssl enc`
refuses AEAD ciphers outright, so a shell implementation would hand-assemble
encrypt-then-MAC and key agreement across openssl version differences, which is
where crypto bugs live and where they are silent. `heliograph-seal` does the
sealing and no networking at all. Every other transport stays pure bash.

Run your own, or use the hosted one:

```bash
docker run -p 8080:8080 \
  -e HELIOGRAPH_RELAY_ESTATES="payments:$CTL:$STN" \
  ghcr.io/dbhq-uk/heliograph-relay:latest
```

## Object store

For an estate where storage is reachable and nothing else is. That sounds
contrived and is common: a locked-down cloud subnet where a firewall appliance
holds the default route and has no policy for the subnet the station sits in
has no outbound anything, while traffic to a **private endpoint** stays inside
the virtual network, never reaches that appliance, and works normally.

```bash
export HELIOGRAPH_S3_ACCESS_KEY=...
export HELIOGRAPH_S3_SECRET_KEY=...

heliograph init payments --transport objstore \
  --dir https://s3.eu-west-2.amazonaws.com \
  --bucket heliograph-transport \
  --scope net-probe
```

The keys come from the environment and are **never written to the estate file**.
That file is on disk, gets copied between machines and ends up in backups; a
secret in it would be a secret in all three.

`--scope` is the lane: one per investigation, so two running at once do not
overwrite each other.

### What works

AWS S3, Cloudflare R2, MinIO, Backblaze B2, DigitalOcean Spaces, Ceph - anything
S3-compatible. Pass `--region auto` (the default) for R2 and MinIO, which have
no regions but reject a request without one.

Azure Blob is **not** S3-compatible. The station reaches it through its own
pigeonhole path instead.

### The layout

```
<prefix>requests/<lane>.txt
<prefix>status/<lane>.txt
<prefix>logs/<name>.txt
```

The same `key: value` documents that cross every other transport, which is why
a log that came back this way reads identically to one that came back over git.

### Scoping the credential

Give it read and write on the prefix and nothing else. It does not need to
create buckets, list other prefixes, or read anything but its own lane.

`heliograph doctor` **writes** a probe object and deletes it, deliberately: a
read-only bucket policy lists and gets perfectly and fails on the first write,
which would be the log, an hour later, with nobody left to tell.

## What a transport must implement

Ten functions on the station side, so the loop never learns which one it is
talking to and the read-only gates live in one place rather than one copy per
transport. Three are lifecycle - `tp_init`, `tp_scope`, `tp_describe` - and
`tp_capabilities` and `tp_revision` report. These seven do the work:

```
tp_fetch_request       emit the request document
tp_fetch_request_live  the same, DURING a run, without touching the working tree
tp_fetch_self          bring a newer station payload, if this transport can
tp_put_status          publish a status document
tp_put_progress        publish a partial-log snapshot
tp_put_log             deliver the FINISHED log
tp_check               can this station WRITE through the transport
```

`tp_check` proves a write, not a read, and every implementation does it the
cheapest way its store allows: git dry-runs a push, Azure Blob puts a
`heliograph-write-check` blob in the lane, the relay makes an authenticated
call. **Read access is not write access**, and a credential that reads
perfectly and cannot write fails on the log, an hour later, with nobody left to
tell.

`tp_put_log` is required of every transport, with no capability flag and no way
to opt out, and it is the newest of the seven for an uncomfortable reason. It
did not exist: the runner ended at a git push, unconditionally, so a station on
any other channel captured a perfect log and delivered nothing. **A transport
that cannot deliver a completed log is not a transport**, so this one is not
optional. See [the capture contract](/conformance), property 9.

`tp_capabilities` reports which of the genuinely optional verbs a transport
offers, and the station says at **start** what it will not be able to do later.

### Two more that only `start.sh` calls

Both optional, and neither is on the capture path:

```
tp_preflight           checks worth more than "can I reach it"
tp_sync                bring the payload up to date before handing over
```

git implements both. `tp_preflight` is where its credential diagnosis, its
`ls-remote` and its `push --dry-run` live, because **read access is not write
access** and only git knows how to prove the difference. A transport that
defines neither still gets the full machine preflight and `tp_check`.

That last part was learned rather than designed. The loop self-updates: a pull
brings a newer `station.sh` and it re-executes into it, which is what lets a fix
take effect without telling the operator to restart. Git gives that away free
and nothing else does. A transport that could not do it, and did not say so,
would be a station nobody could fix without finding a human on the far side.
