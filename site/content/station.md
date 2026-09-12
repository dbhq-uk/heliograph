# The station

The far side. A directory of plain text, planted into a private transport repo,
that watches for a request, runs one step, and sends the log back.

It is the half of heliograph you cannot reach, so it is the half most worth
reading before you run it. Everything here is text you can open in an editor on
the machine it will run on.

```diagram loop
The operator starts it once. Everything after that is the transport.
```

## Two stations, and a launcher

| | what it is |
|---|---|
| **`station/bash/`** | the station. Bash 4+, and what almost every host runs |
| **`station/powershell/`** | the twin, for a Windows estate with **no bash and no permission to install any**. Plant it with `--flavour powershell` |
| **`station/bash/station.ps1`** | a **launcher**, not a station. It finds the bash that Git for Windows installed and hands over to `start.sh`. Do not confuse it with `station/powershell/station.ps1`, which is the loop itself |

**Prefer the bash station wherever it will run.** One implementation of the
capture is better than two, and a Windows box with Git for Windows should use
the launcher rather than the twin.

### A second implementation is permitted only while it passes the contract

That is the rule, and it replaced a blanket prohibition. The argument against a
port was never about PowerShell - it was about **untested drift**, and a
buffered port is the worst kind: it gives every line the same timestamp, which
reads like a working log while destroying the only property the log is for.

So the rule is now a gate rather than a ban. `station/powershell/` passes all
ten properties of [the capture contract](/conformance), over both transports it
ships, on Windows PowerShell 5.1 and on 7, in CI, on every change. If it ever
stops passing, it stops shipping.

The twin carries the same request document, the same published status, the same
four gates and the same exit codes. A control side reads one document and
cannot tell which of them wrote it, and a test asserts exactly that.

What it does **not** have is recorded on [Windows](/windows): no relay
transport, and a self-update that needs a restart.

## What it depends on

The bash station needs bash 4+ and GNU coreutils. The PowerShell station needs
Windows PowerShell 5.1, which is in-box on Server 2016 and later. Then whatever
the transport needs: `git` for the git transport, `curl` for blob and relay,
nothing at all for a file share.

That is the whole list, and it is the entire proposition - on a locked-down box,
installing anything is its own change request. **Nothing is ever installed on
the far side.**

CI enforces it. No Go, no binary and no package may appear under `station/`.
The only Go permitted is `station/embed.go`, which lets the CLI carry the
payload, and its test - neither ships anywhere.

There is exactly one exception, argued for explicitly rather than smuggled in:
the relay transport needs `heliograph-seal`, because its construction is
X25519, HKDF-SHA256, ChaCha20-Poly1305 and Ed25519, and hand-assembling those
in shell across openssl versions is where crypto bugs live and where they are
silent. Every other transport is pure text.

The PowerShell station has **no relay either**, but not for that reason - and
the reason it was given for a while turned out to be wrong. All four primitives
are available in about 200 KB of managed C#, verified against the standards'
own vectors, so no native binary is needed there at all. It is simply not
built. [The design](https://github.com/dbhq-uk/heliograph/blob/main/docs/specs/2026-09-11-powershell-relay-design.md)
says what it would take.

## The files

| | bash | PowerShell |
|---|---|---|
| the preflight, and the one command the operator types | `start.sh` | `start.ps1` |
| the loop: poll, decide, dispatch, publish | `station.sh` | `station.ps1` |
| the step runner. Owns the log, the timestamps and the delivery | `run.sh` | `run.ps1` |
| the capture itself | `caplib.sh` | `caplib.psm1` |
| one file per channel | `transports/git.sh`, `share.sh`, `bundle.sh`, `blob.sh`, `relay.sh` | `transports/git.psm1`, `share.psm1` |
| one file per question, and a template to start from | `steps/` | `steps/` |
| helpers a step can use | `lib/` | `lib/` |
| where captured logs land | `ops-logs/` | `ops-logs/` |
| what to run, and what happened | `station/request`, `station/status` | the same |

Both payloads carry a `service.ps1` for Windows, and they are different files:
the bash one registers the launcher, the PowerShell one registers `start.ps1`
and carries the transport's variables into a restricted file, because a
scheduled task inherits none of them. The bash payload also has `caprun.sh` -
the same capture around an arbitrary command - and `secret.sh`; the PowerShell
payload has neither yet.

Details of each: [the runner](/runner), [writing a step](/steps),
[Windows](/windows).

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

Every status also carries `host:` and `payload:`. The payload is a digest of
`station.sh`, `run.sh` and `caplib.sh` - what a step's behaviour actually rests
on. `HEAD` cannot answer "which payload is running", because every status
commit and every log advances it, so two stations on identical payloads report
different revisions within a minute. Branches carry independent copies and
self-update pulls only its own, so drift between stations is real, and worth
seeing rather than discovering when a step behaves differently on one machine.

The steps themselves are deliberately **not** in the digest: they are supposed
to differ per branch, and including them would make it change for the ordinary
reason and stop meaning anything.

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
