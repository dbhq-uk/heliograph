# Air-gapped

Running heliograph on a machine with no network path at all, or nearly none.
What works today, what is designed and not finished, and how to tell which kind
of air gap you actually have.

## Three things people mean by air-gapped

Most "air-gapped" estates are not. They are cut off from the internet, and from
you, but not from everything. Which of these you have decides the transport:

| the far side can reach | transport | status |
|---|---|---|
| an internal git host: GitLab, Gitea, Bitbucket Server, Azure DevOps Server | [git](/transports#git) | works, driven end to end in CI |
| a directory both machines mount | [file share](/transports#file-share) | works, driven end to end in CI |
| S3-compatible storage inside the estate | [object store](/transports#object-store) | control side only |
| nothing. A person walks between the two machines | [bundle](/transports#bundle) | control side only |

The first two are the common case, and they are the whole answer for it. An
internal git host is enough: the station is planted from the private transport
repo, the operator starts the loop, and nothing on the far side ever needs a
route out. The only requirement is that *you* can reach the same host from the
near side, which is what a bastion-only estate usually permits.

## A true air gap, today

When nothing crosses but a person, heliograph still works. It stops being a loop
and becomes a procedure, and the value is that the format, the gates and the log
are identical to every other transport, so the [method](/method) survives the
walk.

**Plant the station by hand.** The station is plain bash and stands on its own.
Clone this repository on a machine that can, carry the clone across, and run the
bootstrap from it:

```bash
git clone https://github.com/dbhq-uk/heliograph          # near side
# carry the directory across
./heliograph/station/bootstrap.sh ~/transport/payments    # far side
```

That is the same route as [planting without the CLI](/bootstrap#without-the-cli):
same payload, same layout.

**Run each step by hand, with delivery off.** `PUSH=0` captures and does not
deliver, and the log lands under `ops-logs/`:

```bash
cd ~/transport/payments
PUSH=0 ./run.sh net-probe
ls ops-logs/
```

Every line still carries a UTC timestamp, the exit code still survives, and the
gates still apply: a step that declares `action` still needs `CONFIRM=yes`.
Carry `ops-logs/` back. It reads like every other captured log, and a hang is
still a gap in the timestamp column.

**What you give up.** The loop. Nobody is polling, so the operator runs each
step themselves and `heliograph watch` has nothing to watch. A round trip costs
a walk, which is the one thing this tool cannot make cheaper.

## The bundle transport: designed for this, not finished

`heliograph init --transport bundle` exists, and `heliograph send` writes a
request file for somebody to carry:

```bash
heliograph init air --transport bundle --dir ~/bundles
heliograph send net-probe            # writes ~/bundles/request-<id>.hgb
```

**The station cannot read it yet.** The bundle has a control side and no station
side, which is what the [transports table](/transports) says. Until that lands,
the procedure above is the honest air-gapped path, and a request file is a note
to yourself about what to run. Do not plan around the bundle until this page
says otherwise.

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
