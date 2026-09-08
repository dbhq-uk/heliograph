# Planting a station

Getting the far side onto the machine. Two routes, and the operator's job is
the same either way: clone, run one command, walk away.

## With the CLI

```bash
heliograph bootstrap ~/transport/payments
cd ~/transport/payments
git init && git add -A && git commit -m 'heliograph: transport repo'
# add a PRIVATE remote, push, then:
heliograph init payments --dir ~/transport/payments
heliograph doctor
heliograph plant
```

The binary carries the station payload it was built with, so the station you
plant is the one that version was tested against. Nothing is downloaded.

`plant` prints the message to send the operator. It carries **no credential**,
deliberately - that is a separate conversation, and pasting a token into a chat
message is how tokens end up in chat history.

## Without the CLI

The station is plain bash and stands on its own. This is the route for a person
who cannot install a binary, and it is a procedure for a human rather than for
an agent:

```bash
git clone https://github.com/dbhq-uk/heliograph
./heliograph/station/bootstrap.sh ~/transport/payments
```

Same payload, same layout, same result.

## What gets planted

```
start.sh station.sh run.sh caprun.sh caplib.sh
transports/   git.sh blob.sh relay.sh
steps/        _template.sh, and the shipped diagnostics
lib/          probe, remote, ansible, terraform, tfguard
station/      request, status
ops-logs/     empty, and committed to
TASK.md       the question this investigation is answering
.gitignore    shipped as `gitignore`, restored on the way out
.gitattributes  shipped as `gitattributes`, pins the repo to LF
```

The ignore and attributes files ship **without the leading dot** so they govern
the transport repo rather than this one, and both bootstraps restore the dot on
the way out. That is not a mistake to fix.

## Re-running it is safe

`bootstrap` installs what is missing and leaves what is there alone. It
**reports rather than overwrites** - a file that differs is named, not
replaced. Your steps, your `TASK.md` and your logs are yours.

That matters because upgrading a station in the field is exactly the moment
somebody would lose a week of work to a helpful overwrite.

## The transport repo must be private, and its own repo

Captured logs are committed to it, so everything the operator's commands print
lands in that history permanently and cannot be unpublished.

**Never bootstrap into a repo that holds anything else, and never into a public
one.** See [secrets](/secrets).

## What the operator does

```bash
git clone <the transport repo>
cd <it>
./start.sh --check      # will this work here? changes nothing
./start.sh              # run it, and walk away
```

`--check` is the one to lead with. It changes nothing, it is often runnable
before any permission is granted, and every problem it reports names its own
remedy - because the person reading it usually cannot ask you.

To survive a logout, see [running it as a service](/service). To run it
somewhere other than a person's terminal, see [hosts](/hosts).

## Upgrading a station in the field

Over git, the station **self-updates**: a pull brings a newer `station.sh` and
the loop re-executes into it. That is what lets a fix take effect without
telling the operator to restart.

Other transports cannot do that - there is no working tree to replace - and the
station says so at **start**, while somebody is still listening, rather than
when an update is needed and nobody is there. On those, re-plant.
