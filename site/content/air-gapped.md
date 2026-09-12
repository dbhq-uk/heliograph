# Air-gapped

Running heliograph on a machine with no network path at all, or nearly none.
What works today, and how to tell which kind of air gap you actually have.

## Three things people mean by air-gapped

Most "air-gapped" estates are not. They are cut off from the internet, and from
you, but not from everything. Which of these you have decides the transport:

| the far side can reach | transport | status |
|---|---|---|
| an internal git host: GitLab, Gitea, Bitbucket Server, Azure DevOps Server | [git](/transports#git) | works, driven end to end in CI |
| a directory both machines mount | [file share](/transports#file-share) | works, driven end to end in CI |
| S3-compatible storage inside the estate | [object store](/transports#object-store) | control side only |
| nothing. A person walks between the two machines | [bundle](/transports#bundle) | **works, both halves**, and passes the same conformance suite as every other transport |

The first two are the common case, and they are the whole answer for it. An
internal git host is enough: the station is planted from the private transport
repo, the operator starts the loop, and nothing on the far side ever needs a
route out. The only requirement is that *you* can reach the same host from the
near side, which is what a bastion-only estate usually permits.

## A true air gap, today

When nothing crosses but a person, the **bundle** transport is the answer. It is
the only one that needs no path between the two machines at all - not a git
host, not a share, not a storage account nobody can route to. A person carries a
file.

It stops being a loop and becomes a procedure. The value is that the request
format, the four gates and the captured log are identical to every other
transport, so the [method](/method) survives the walk - and the station does not
have to be told it is reading from a stick.

### Plant the station by hand

The station is plain text and stands on its own. Clone this repository on a
machine that can, carry the clone across, and run the bootstrap from it. On a
Windows box with no bash, `station/bootstrap.ps1` does the same job with only
the PowerShell already installed:

```bash
git clone https://github.com/dbhq-uk/heliograph          # near side
# carry the directory across
./heliograph/station/bootstrap.sh ~/transport/payments    # far side
```

Same route as [planting without the CLI](/bootstrap#without-the-cli): same
payload, same layout.

### Then the round trip, one walk each way

On the near side:

```bash
heliograph init air --transport bundle --dir /media/stick
heliograph send net-probe            # writes /media/stick/request-<id>.hgb
```

Carry the medium across. On the far side:

```bash
cd ~/transport/payments
TRANSPORT=bundle BUNDLE_DIR=/media/stick ./start.sh -- --once
```

Carry it back. On the near side:

```bash
heliograph status                    # reads the status the station wrote
heliograph logs --last               # reads the log it carried back
```

`--once` because there is nothing to poll for: the medium will not change while
the station watches it. Without it the station will sit there for ever behaving
perfectly, which is indistinguishable from a station nobody is using - and the
preflight says so before you walk away.

### What is on the medium

```
request-<id>.hgb   the control side writes it
status             the station writes it
<step>-<UTC>.txt   the log, beside the status
```

Nothing else, and no credential of any kind: **the medium is the channel and
there is nothing to authenticate to**. That is worth being plain about, because
it cuts both ways - anyone who can write to that stick can queue a step, and it
will run under the same four gates as anything else.

### What you give up, and it is only two things

**The loop.** A round trip costs a walk, which is the one thing this tool cannot
make cheaper.

**The cancel.** The bundle declares neither `live` nor `self`, honestly, because
there is nothing to re-read: a cancel would have to be carried in by hand, by
which time the step it was meant for has finished. A station on this transport
cannot be stopped from the near side, and cannot update itself - it is changed
by re-planting, which is another walk.

Everything else is the same. Every line still carries a UTC timestamp, the exit
code still survives, the footer still says whether it passed, and a hang is
still a gap in the timestamp column.

### Or without the CLI at all

`PUSH=0 ./run.sh <step>` captures into `ops-logs/` and delivers nothing, and you
carry that directory back. It works, and it is what to reach for when the near
side has no `heliograph` binary either. The bundle is better when it is
available: `heliograph logs --gaps` and `heliograph watch` understand what comes
back, and a plain directory of files is something you have to read yourself.

## Kubernetes and containers behind an air gap

The [station image](/containers) is published on GHCR. In an air-gapped cluster,
mirror it to the registry the cluster can pull from and set the image reference
in the manifest. The entrypoint clones the transport repo and hands over to
`start.sh`, so the cluster needs a route to the git host that holds it, which
in this setting is the internal one from the table above, not the internet.

## What "no network" does not change

The four [gates](/security) live in the loop, not in the transport, so a station
planted by hand refuses exactly what a station over git refuses: undeclared
steps, actions without `CONFIRM=yes`, running as root. And
[redaction](/secrets) still runs before a log is written, which matters more
here, because a log carried on removable media passes through more hands.
