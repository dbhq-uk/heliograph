# The runner

`start.sh`, `station.sh`, `run.sh` and `caprun.sh`, and every knob each one
honours. This is the reference for the far side's own scripts. Their PowerShell
twins - `start.ps1`, `station.ps1`, `run.ps1` and `caplib.psm1`, for an estate
with no bash - are at the bottom of this page.

The division of labour is the thing to hold onto: **a runner owns the log, and
a step just prints to stdout.** That is what makes a step runnable on its own,
and it is why there is exactly one implementation of the capture.

## `start.sh` - the one command the operator types

```bash
./start.sh                     # check this machine, then run the station
./start.sh --check             # check only, change nothing, exit
./start.sh --branch task/foo   # check that branch out first (git only)
./start.sh -- --once           # everything after -- goes to station.sh
TRANSPORT=relay ./start.sh     # any transport, same command, same table
```

Three jobs and nothing else: prove the machine can produce a usable capture,
**ask the transport** whether it can carry a log from here before an hour-long
step discovers it cannot, then hand over.

**`--check` changes nothing.** It is what an operator runs to answer "will this
work here", often before they are permitted to alter anything. Every `FAIL` it
prints names a remedy, because the person reading it usually cannot ask you.

It does **not** clone. This file ships inside the transport repo, so by the
time it runs the clone has already happened.

### It asks the transport, rather than assuming git

The machine checks are the same for everyone. The channel checks are the
transport's, and it answers through the same contract the loop uses:

| | |
|---|---|
| `tp_init` | what this transport needs locally. A missing `RELAY_URL` is reported here, in its own words |
| `tp_describe` | the channel and the credential, by mechanism and never by value |
| `tp_preflight` | **optional.** Checks only this transport knows to make |
| `tp_check` | the fallback every transport answers: can it be reached at all |

git implements `tp_preflight`, and that is where its credential diagnosis, its
`ls-remote` and its `push --dry-run` live. Read access is not write access, and
proving the push is worth far more than proving reachability - but it is a git
answer to a git question, so it belongs to git.

`--branch` belongs to git too, and is **refused** on any other transport rather
than ignored. A branch is not what a relay or a blob lane is bound to, and
letting somebody believe they had re-pointed a station is the failure this whole
design exists to prevent.

## `station.sh` - the loop

```bash
./station.sh                    # poll, run, deliver, repeat - READ-ONLY
./station.sh --once             # do one requested run, then exit
./station.sh --interval 15      # seconds between polls (default 5)
./station.sh --allow-actions    # also run steps that declare themselves actions
./station.sh --allow-root       # permit running as root
./station.sh --pin              # approve the current steps, for REQUIRE_PIN=1
```

| variable | default | |
|---|---|---|
| `INTERVAL` | `5` | seconds between polls |
| `ALLOW_ACTIONS` | `0` | run steps that change state |
| `ALLOW_ROOT` | `0` | permit running as root |
| `REQUIRE_PIN` | `0` | refuse any step whose hash the operator has not approved |
| `PROGRESS_EVERY` | `60` | seconds between partial-log pushes; `0` disables |
| `ACTION_ENV` | `APPLY=1 CONFIRM=yes DESTROY=1 FORCE=1 WRITE=1` | env that turns a read-only step into a writing one |
| `TRANSPORT` | `git` | which channel. A request may **not** override this |

### Pinning

`REQUIRE_PIN=1` refuses any step whose file hash the operator has not approved
with `./station.sh --pin`. The hashes live in a **local, gitignored** file:
recorded in the transport repo they could be edited from the far side, which is
the only side a pin exists to distrust, and the approval would then travel with
the change it is supposed to catch.

It is off by default because it makes every new step wait for the operator,
which is the relaying this loop exists to remove. It is here for an estate that
wants "runs only what I approved" and knows what that costs.

It does **not** cover `station.sh` itself, which self-updates on pull. Said
plainly rather than implying a boundary that is not there.

### Self-update

When a pull brings a newer `station.sh`, the loop re-executes itself into it.
Without that, a fix cannot take effect while the station is running and the
operator has to be told to restart, which defeats them starting it once and
walking away.

Not every transport can do it. Git gets it free from a pull; the relay and blob
transports have no working tree to replace. `tp_capabilities` reports which
verbs a transport actually offers, so the station says *"this station cannot
update itself, you will need to re-plant it"* at **start** time, while somebody
is still listening, rather than when an update is needed and nobody is there.

## `run.sh` - the step runner

```bash
./run.sh                    # runs whatever step is currently set
./run.sh <step>             # or name one explicitly
./run.sh /path/to/probe.sh  # or point at a step file directly
./run.sh --list             # what steps exist on this branch
./run.sh --mode <step>      # what that step DECLARES itself to be
./run.sh --file <step>      # which file that declaration came from
```

`--mode` and `--file` exit having touched nothing. `station.sh` asks through
them rather than reading the step table itself, so the mapping from a step name
to a file stays in one place.

| variable | |
|---|---|
| `PUSH=0` | capture only; do not deliver |
| `LOG_DIR` | where the log is written. Default `ops-logs/` |
| `CONFIRM=yes` | required for a step declaring `action` |
| `SUDO=1` | pre-cache sudo, for a step that escalates |
| `REDACT=0` | disable secret masking, when it is hiding something you need |
| `NO_COLOUR=1` | plain banners |

### Exit codes

| | |
|---|---|
| `0` | the step succeeded |
| `2` | unknown step, or a step file that is not executable |
| `3` | the step declared no mode, an unrecognised one, or `action` without `CONFIRM=yes` |
| `4` | a PowerShell step, and no PowerShell on `PATH` |
| `5` | refused: running as root |
| `130` | cancelled mid-run |
| anything else | the step's own exit code, carried through `PIPESTATUS` |

## `caprun.sh` - the same capture, around anything

```bash
./caprun.sh quicklook -- systemctl status nginx
PUSH=0 ./caprun.sh quicklook -- df -h
```

For a one-off where writing a step file is not worth it. Same capture, same
timestamps, same redaction, same delivery.

## The `cap_*` library

Sourced, never executed. A runner calls these; a step never should.

| | |
|---|---|
| `cap_header` | truncate the log and write the provenance block |
| `cap_run` | run a command, timestamped, ANSI-stripped, redacted, teed |
| `cap_footer` | the closing block: finish time, real exit code, `RESULT` |
| `cap_deliver` | ship the finished log over whatever transport is configured |
| `cap_push` | the git implementation of that, used directly when there is no transport |
| `cap_redact` | best-effort secret masking. See [secrets](/secrets) |
| `cap_refuse_root` | the blast-radius gate |
| `cap_git` | git with the auth header attached, through the environment |
| `cap_auth_describe` | which credential is in force, by mechanism, never by value |
| `cap_sudo_precache` | prompt for sudo up front, so the run cannot hang on it |
| `cap_banner`, `cap_section`, `cap_result` | terminal output, not log content |

### Two things not to change

**`cap_run` stamps each line before anything else touches it.** Moving that
stage, batching output, or buffering a command's output in a variable all
destroy the only property these logs exist for.

**`cap_git` passes the auth header through the environment, not argv.**
`git -c k=v` puts the value on the process command line, and `/proc/<pid>/cmdline`
is world-readable: any other user on the box can read the token out of `ps`.
`/proc/<pid>/environ` is owner-only. A reduction in exposure, not a guarantee -
root still reads either.

## The PowerShell twin

For an estate with no bash. `start.ps1`, `station.ps1`, `run.ps1` and
`caplib.psm1` are the twins of the four above: the same request document, the
same published status, the same four gates, the same exit codes. See
[Windows](/windows) for what decides whether that machine can run one at all.

```powershell
.\start.ps1 --check              # will this work here, changing nothing
.\run.ps1 <step>                 # one step, by hand
.\station.ps1                    # poll, run, deliver, repeat - READ-ONLY
.\station.ps1 --once --interval 15 --allow-actions --allow-root --pin
```

Every variable in the `station.sh` table above is honoured, with the same name
and the same default. Two things differ, and both are stated where they are
rather than smoothed over:

**Pinning uses `.station-approved-ps`.** The two payloads approve different
files - `run.sh`/`caplib.sh`/`lib/*.sh` against `run.ps1`/`caplib.psm1`/
`lib/*.psm1` - and each `--pin` truncates before writing. One shared file would
mean each implementation silently unapproved the other's, so in a repo carrying
both payloads every request would be refused by whichever station pinned last.

**A self-update exits rather than re-executing.** PowerShell has no `exec`, and
starting a replacement is worse than useless under the Job Object the cancel
depends on: the moment the old process exits, the kernel terminates the new one.
`run.ps1`, `caplib.psm1` and the steps still come forward with no restart,
because every run is a fresh process that loads them again - only the loop
itself needs one, and it exits **75**, which a scheduled task treats as
*restart me*.

Transports: `git.psm1` and `share.psm1`. There is **no relay transport** for
PowerShell - it needs `heliograph-seal`, which is a Go binary, and a station
that must ship a binary is a different bootstrap question on the estates this
payload exists for.

It is permitted only on one condition, which is the condition the whole
argument turns on: a second implementation of the capture is allowed **only
while it passes [the conformance suite](/conformance)**. It passes all ten
properties, over both transports, on Windows PowerShell 5.1 and PowerShell 7.

## Conventions

Station scripts use `set -uo pipefail`, never `set -e`. A diagnostic wants
every probe's result, not the first failure. This is the opposite of the usual
house rule and it is deliberate.

Steps never prompt. No interactive sudo, no host-key questions, no `read`. A
prompt through the capture pipeline is invisible and the run hangs.
