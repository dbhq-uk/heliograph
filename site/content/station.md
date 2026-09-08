# The station

The far side. A directory of plain bash, planted into a private transport repo,
that watches for a request, runs one step, and sends the log back.

It is the half of heliograph you cannot reach, so it is the half most worth
reading before you run it. Everything here is text you can open in an editor on
the machine it will run on.

```diagram loop
The operator starts it once. Everything after that is the transport.
```

## There is one station, and one launcher

| | what it is |
|---|---|
| **`station/bash/`** | the station. Bash 4+, and what every host runs |
| **`station.ps1`** | a **launcher**, not a port. It finds the bash that Git for Windows installed and hands over to `start.sh`. It re-implements nothing |

**One implementation of the capture, and it must not be forked.** A PowerShell
copy would be a second one, drifting in the least visible way possible: a
buffered port gives every line the same timestamp, which reads like a working
log while destroying the only property the log is for.

A native PowerShell station is
[designed](https://github.com/dbhq-uk/heliograph/blob/main/docs/specs/2026-09-08-powershell-station-and-full-documentation-design.md)
and not built. That design also proposes relaxing the rule above - a second
implementation permitted **only** while it passes [the capture
contract](/conformance) - and that relaxation is a draft, not current policy.

## What it depends on

Bash 4+ and GNU coreutils, plus whatever the transport needs: `git` for the git
transport, `curl` for blob and relay. That is the whole list, and it is the
entire proposition - on a locked-down box, installing anything is its own change
request. **Nothing is ever installed on the far side.**

CI enforces it. No Go, no binary, no interpreter and no package may appear
under `station/`. The one Go file permitted is `station/embed.go`, which never
ships anywhere.

There is exactly one exception, argued for explicitly rather than smuggled in:
the relay transport needs `heliograph-seal`, because its construction is
X25519, HKDF-SHA256, ChaCha20-Poly1305 and Ed25519, and hand-assembling those
in shell across openssl versions is where crypto bugs live and where they are
silent. Every other transport stays pure bash.

## The files

| | |
|---|---|
| `start.sh` | the preflight, then hand over. The one command the operator types |
| `station.sh` | the loop: poll, decide, dispatch, publish |
| `run.sh` | the step runner. Owns the log, the timestamps and the delivery |
| `caprun.sh` | the same capture around an arbitrary command, for ad-hoc use |
| `caplib.sh` | the capture itself, and the only implementation of it |
| `transports/` | `git.sh`, `blob.sh`, `relay.sh` - one file per channel |
| `steps/` | one file per question. `_template.sh` to start from |
| `lib/` | helpers a step can source: probes, ansible, terraform, remote hosts |
| `ops-logs/` | where captured logs land, and are committed from |
| `station/request` | what to run. `station/status` - what happened |

Details of each: [the runner](/runner), [writing a step](/steps).

## The loop, precisely

1. Poll the transport for a request. Default every 5 seconds
2. **The trigger is the `id`, never a new commit.** Documentation and step edits
   land constantly; if any change fired a run, the station would fire on all of
   them. A run is always something somebody asked for on purpose
3. Ask `run.sh --mode` what the step declares itself to be, and apply the gates
4. Publish `running`, then dispatch the step in its own process group
5. Keep polling while it works, so a `cancel` is heard and an hour-long step
   does not make the station deaf for an hour
6. Push a partial log every `PROGRESS_EVERY` seconds, so a long run can be
   watched rather than waited out
7. Deliver the finished log, then publish `idle` - or `undelivered`, if the log
   was captured and the transport would not take it

## The states it publishes

| state | |
|---|---|
| `starting` | the loop is up and its credential works. Nothing asked yet |
| `running` | a step is in flight |
| `idle` | the run finished **and the log was delivered**. Only a confirmed delivery earns it |
| `undelivered` | the run finished and the log did not arrive - refused by the transport, skipped, or the runner never reached delivery. The reason is published with it |
| `refused` | a gate said no, and says which one |
| `cancelled` | signalled mid-run. The partial log is kept |
| `stopped` | the loop ended, by `stop: yes` or by Ctrl-C |

`undelivered` matters more than it looks. Without it, "the log exists and
cannot be shipped" and "the step is still running" are the same silence from
your side, and only one of them is worth waiting on.

## Two properties everything else rests on

**Every captured line carries a UTC timestamp**, applied by a pure-bash read
loop reading straight from the command, *before* any other stage. That ordering
is load-bearing. It used to be applied last, which made the property depend on
`sed -u`; a sed without it gave every line in a block the same time while the
log still read perfectly. After the fact, in an untimed log, a hang and slow
progress are indistinguishable.

**A failed run still ships, and a failed delivery never loses the log.** The
log is delivered whether the step passed or failed, and the real exit code
survives the pipeline via `PIPESTATUS`. Each round trip through an operator is
expensive; none may be wasted by tooling that only reports success.

Both are asserted by [the conformance suite](/conformance), which is the
executable form of the contract rather than a description of it.

## Stopping, cancelling, and surviving a logout

`stop: yes` in the request ends the loop from your side, which matters because
nobody is sitting at that terminal. `cancel: yes` kills the step running right
now; `cancel: <id>` kills it only if that id is the one running, so a stale
cancel cannot reap a later run. The partial log is always kept **on the
station**, and on the git transport it is delivered with the cancellation; on
blob and relay it currently is not, and stays local.

`station.sh` deliberately does **not** trap `HUP`. Its cleanup signals the
running step's process group, so trapping `HUP` would kill an in-flight step
every time a connection dropped. To survive a logout properly, see
[running it as a service](/service).

## Getting one onto the far side

[Planting a station](/bootstrap). The operator's whole job is: clone the repo,
run `./start.sh`, walk away.
