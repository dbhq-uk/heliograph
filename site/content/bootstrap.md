# Planting a station

Getting the far side onto the machine. Three routes, and the operator's job is
the same whichever you take: clone, run one command, walk away.

**Two payloads.** `--flavour bash` is the default and is what almost everyone
wants. `--flavour powershell` plants the [pure PowerShell
station](/windows#the-powershell-station-for-a-box-with-no-bash) for a Windows
estate with no bash and no permission to install any. A machine gets one of
them: an estate that has bash should run the bash station, because one
implementation of the capture is better than two.

`--flavour both` plants both into one repo, for a transport repo serving two
machines of different kinds. Nothing is overwritten, so the flavour planted
first supplies the files they share - which is why both payloads carry an
identical block of ignore rules, so that what keeps a token out of the
repository does not depend on the order somebody typed two commands in.

## With the CLI

```bash
heliograph bootstrap ~/transport/payments
# or, for a Windows box with no bash:
heliograph bootstrap ~/transport/payments --flavour powershell
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
./heliograph/station/bootstrap.sh ~/transport/payments --flavour powershell
```

Same payload, same layout, same result.

## Without the CLI *and* without bash

The route for a Windows box with neither. `bootstrap.ps1` needs only the
PowerShell that is already on the machine, which is the whole premise of the
PowerShell station - the plant may not be the one step that assumes a shell the
machine does not have.

```powershell
git clone https://github.com/dbhq-uk/heliograph
.\heliograph\station\bootstrap.ps1 C:\ops\payments
```

It defaults to the PowerShell payload, because an operator who had bash would
have used `bootstrap.sh`.

**All three produce the same tree.** A test in this repository runs
`heliograph bootstrap`, `bootstrap.sh` and `bootstrap.ps1` against the same
checkout and compares the results file by file, for both flavours. If somebody
adds a file to a payload and only one planter picks it up, that test is what
notices.

## What gets planted

The bash payload:

```
start.sh station.sh run.sh caprun.sh caplib.sh
transports/   git.sh blob.sh relay.sh share.sh
steps/        _template.sh, and the shipped diagnostics
lib/          probe, remote, ansible, terraform, tfguard
station/      request, status
ops-logs/     empty, and committed to
TASK.md       the question this investigation is answering
.gitignore    shipped as `gitignore`, restored on the way out
.gitattributes  shipped as `gitattributes`, pins the repo to LF
```

The PowerShell payload:

```
start.ps1 station.ps1 run.ps1 caplib.psm1
transports/   git.psm1 share.psm1
lib/          transport, cancel, probe
steps/        _template.ps1, and the shipped env snapshot
station/      request
ops-logs/     empty, and committed to
TASK.md .gitignore .gitattributes    as above
```

The ignore and attributes files ship **without the leading dot** so they govern
the transport repo rather than this one, and all three bootstraps restore the
dot on the way out. That is not a mistake to fix.

**A station's own runtime state is never planted.** `.station.lock`,
`.station-state`, `.station-approved`, `.station-delivery`, `.station-env` and
the rest are written by a running station and belong to one machine. A checkout
that has ever run a station has them sitting in it, invisible to `git status`
because they are gitignored - so every planter prunes them by name, and a test
refuses to let them into the release binary at all. `.station-env` holds a
token.

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
