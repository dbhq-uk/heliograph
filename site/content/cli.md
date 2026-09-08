# CLI reference

Every command the control side has, and the reasoning behind the ones that are
not obvious.

```
heliograph bootstrap <dir>
heliograph init <estate> --dir <path> [--transport git|share|bundle|objstore]
                          [--scope <name>] [--bucket <b>] [--prefix <p>] [--region <r>]
heliograph estates
heliograph plant [--service] [--script]
heliograph send <step> [KEY=VALUE ...] [--note <text>]
heliograph watch [--interval 10s] [--timeout 0]
heliograph status
heliograph logs [--last] [<name>] [--gaps] [--min 10s]
heliograph doctor
heliograph mcp
heliograph version
```

Every command takes `-e` / `--estate`. With exactly one configured, it is
optional. With several it is required: sending a request to the wrong estate
runs a command on the wrong machine, and that is not recoverable by apologising.

## bootstrap

Plants the station payload into a transport repo. The payload is embedded in
the binary at build time, so a release plants exactly the station it was
tested against, and "which station is this estate running" has the same
answer as "which binary planted it".

Nothing is overwritten, ever: an existing file is left alone and reported,
because the second run is usually an upgrade over a repo with a task in
flight. `station/bootstrap.sh` in the repository lays down the same files for
a machine with no CLI, and CI asserts the two produce identical trees.

## init

Remembers a transport by name, so every later command can be typed without a
path. It **attaches before saving**: an estate that names a directory which is
not a usable transport is worse than no estate at all, because it fails later,
from a command that had every reason to expect it to work.

## station add

A second station on the same repository, on its own branch.

```bash
heliograph station add db-a
heliograph send net-probe -e db-a     # reaches that machine and no other
```

It does three things that have to happen together, because doing two is worse
than doing none:

1. creates `station/db-a` and pushes it **with an upstream** - without one the
   station's own first push fails on the far side, where nobody can see it
2. checks it out into a git worktree beside the existing clone, so the control
   side has one checkout per station and never switches between them
3. records the estate, so `-e db-a` routes to that machine

Then it prints what to send the operator, which is the point of the other three.

`--dir` puts the checkout somewhere other than beside the existing one.

**It refuses a branch origin already has**, because that is somebody else's
station and creating it from this HEAD would be about to rewrite their history.
It does not move your checkout either: it uses `git branch`, not `checkout -b`.

### An estate it creates is pinned to its branch

`station add` records the branch as the estate's **scope**, and every later
command verifies the checkout is still on it. Move that checkout and the CLI
refuses, naming both branches, rather than sending your next request to a
different machine.

An estate from `init` is **not** pinned - it records the branch it found and
follows the checkout - because working on `task/<slug>` and then sending is the
ordinary workflow.

Several stations in one repository only makes sense where they share a blast
radius: a repo credential usually spans every branch. See
[transports](/transports#one-repository-several-stations).

## plant

Prints the message to send the operator. Generated rather than typed, because
every retyping of a clone URL is a chance to get it wrong on a machine nobody
can check afterwards.

It carries **no credential**, deliberately. A token pasted into a chat window is
in that history forever, and the operator usually already has one.

It also says what the loop will *not* do: refuses root, runs nothing that
changes state without `--allow-actions`, holds no credentials of its own. That
is not padding. Somebody is being asked to run a stranger's script on a
production machine, and the honest answer to "what does this do" is what gets it
approved.

## send

Publishes a request. The `id` is the trigger and nothing else is: documentation
and step edits land on a branch constantly, and if any change fired a run the
station would run on all of them.

Everything after the step name is environment, passed verbatim:

```bash
heliograph send net-probe HOSTS="sql01 sql02" PORTS=1433
```

Values containing spaces are re-quoted on the way out. The shell that invoked
the CLI has already eaten your quotes, and the station splits that line the way
a shell would, so an unquoted value would set the first word and try to *run*
the rest.

## mcp

Serves every command above as typed tools to any MCP-capable agent, over stdio.

```bash
claude mcp add heliograph -- heliograph mcp
```

It is the same binary, so there is nothing extra to install, and the tools call
the same code the commands do. Full detail on the [MCP page](/mcp).

The gates do not move. A tool call publishes a request; the station still
decides whether to run it.

## Object store estates

`--transport objstore` needs three things the other transports do not: where the
store is, which bucket, and which lane.

```bash
export HELIOGRAPH_S3_ACCESS_KEY=... HELIOGRAPH_S3_SECRET_KEY=...

heliograph init payments --transport objstore \
  --dir https://s3.eu-west-2.amazonaws.com \
  --bucket heliograph-transport \
  --scope net-probe
```

The keys come from the environment and are **never written to the estate file**.
That file is on disk, gets copied between machines and ends up in backups; a
secret in it would be a secret in all three.

`--scope` is the lane: one per investigation, so two running at once do not
overwrite each other. `--region` defaults to `auto`, which is what R2 and MinIO
want. See [transports](/transports).

## logs --gaps

The reason the binary is worth installing.

```
$ heliograph logs --last --gaps
net-probe-20260906T091400Z.txt
412 captured lines

2 interval(s) of 10s or more, longest first.
Each is attributed to the line BEFORE it, which is what was running.

   3m12s  after  09:14:02 | Refreshing state...
     45s  after  09:17:14 | ---------- openssl s_client ----------
```

A log where **every line carries the same timestamp** is reported as an error
rather than "no gaps found". That log is a buffered capture, it reads perfectly,
and calling it clean would be the exact opposite of true.

## doctor

`heliograph check` is the same command under another name.

Answers "will this work from here" and changes nothing. Every line that reports
a problem also says what to do about it: a preflight line that names a fault
without a remedy is a defect, because the person reading it usually cannot ask
anybody.
