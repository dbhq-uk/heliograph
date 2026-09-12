# Windows

Two different questions get confused here, so this page separates them.

1. **Can a Windows machine host the station?** Yes, today, if it has Git for
   Windows.
2. **Can a step be written in PowerShell?** Yes, today, on any host.

3. **Can the station itself be pure PowerShell, with no bash at all?** Yes.
   It polls, runs, delivers and publishes, and it passes the same conformance
   suite the bash station does. It is at the bottom of this page.

## Hosting the loop on Windows

```powershell
.\station.ps1                 # preflight, then run the station
.\station.ps1 --check         # preflight only, change nothing
.\station.ps1 -- --once       # everything after -- goes to station.sh
```

**`station.ps1` is a launcher, not a port.** It finds the bash that Git for
Windows installed and hands over to `start.sh`. It re-implements nothing.

That is deliberate. The capture pattern has exactly one implementation, and a
PowerShell copy of it would be a second one that drifts - in the least visible
way possible, because a buffered port gives every line the same timestamp,
which reads like a working log while destroying the only property the log is
for.

Git for Windows ships bash 4.4+ with GNU coreutils. Git is the transport, so a
Windows host without git cannot participate anyway: the dependency is already
paid for.

### Finding bash, and the one that must not be found

Order matters: an explicit `HELIOGRAPH_BASH` override first, then the registry
(authoritative for where Git for Windows actually landed), then the usual
install paths, then `PATH`.

**`PATH` is last on purpose.** WSL puts a `bash.exe` in `System32` that is not
Git bash - it launches a Linux distribution, or fails with an install prompt if
none exists. A lookup finding that first would fail with a confusing WSL
message that says nothing about heliograph, so it is explicitly ignored.

```powershell
$env:HELIOGRAPH_BASH = 'D:\tools\Git\bin\bash.exe'   # if git is somewhere unusual
```

### Surviving a logout

[`service.ps1`](/service) registers a scheduled task. It does **not** look for
bash itself - it starts `station.ps1`, so the registry lookup stays in one
place.

## Steps written in PowerShell

A step is an argv array, so the runner does not care what language it is in. A
Windows question wants `Get-WinEvent`, not a bash reimplementation of it.

```bash
winev)  ps_step ./steps/win-events.ps1 ;;
```

`ps_step` exists so the four things that make PowerShell output unreadable in a
captured log are fixed **once**, rather than in every step by every author who
remembers. Every number below was measured on Windows Server 2022 with
PowerShell 5.1.20348 and on Linux with pwsh 7.6.5.

**Line endings.** `powershell.exe -File` emits CRLF - measured CR=8, LF=8 for
an eight-line script, whether redirected to a file or through a pipe. A stray
CR on every line is invisible in a terminal, wrong in the file, and quietly
breaks any later `grep` anchored with `$`. Rewriting each line with an explicit
LF drops CR to 0.

**Hyperlinks, not colour.** The capture already strips ANSI `ESC[` sequences,
which covers everything pwsh 7 emits for `Format-Table`: 8 ESC bytes in, 0 out.
The gap is OSC 8 hyperlinks - `ESC]8;;<url>` - which that pattern does not
match. Measured 4 ESC bytes surviving for a step calling
`$PSStyle.FormatHyperlink`, and 0 with `NO_COLOR` set. `NO_COLOR` is what earns
its place here.

**Encoding.** Forcing UTF-8 is about the OEM codepage mangling non-ASCII, not
about UTF-16: the redirected stream measured NUL=0, so the widely repeated
"PowerShell redirects as UTF-16" does not apply to this path.

**Exit codes, and this one would have shipped a wrong answer.** Running the
step in-process as `& './step.ps1'` leaves `$LASTEXITCODE` holding whatever the
last *native* command inside it returned. The shipped Windows snapshot ends by
probing `git config --get core.autocrlf`, which exits 1 when the key is unset -
so a perfectly good snapshot reported failure. Invoking the step as a child
with `-File` fixes it: measured 0 for a clean run, 3 for a genuine `exit 3`,
and 0 for a run whose internal native command exited 7.

`--mode` is answered before any interpreter is looked for, so a step's
declaration is readable on a machine with no PowerShell on it at all.

## Line endings, and what they really break

The station ships a `gitattributes` file that pins the transport repo to LF.

**The point is not to protect Windows from itself.** Git for Windows' bash
strips CR and runs a CRLF checkout perfectly well - measured on Server 2022
with Git 2.55. The point is that CRLF which gets **committed** breaks every
Linux clone afterwards, and breaks it silently when a file is *sourced*: on
Linux, `caplib.sh` reports `set: pipefail: invalid option name` and then
**carries on** with neither `-u` nor `pipefail` applied.

So `start.sh` probes the bash it is actually running under rather than
assuming, and reports one of three things: LF throughout, a CR-tolerant bash
with no `.gitattributes` (a warning - it runs here and would break a Linux
clone), or a CR-intolerant bash with CRLF files (a failure, with the fix).

If you get it wrong: `git config --global core.autocrlf false` and clone again.
The checkout is disposable and anything already pushed is safe.

## The PowerShell station, for a box with no bash

Everything above assumes Git for Windows, and where you have it that is the
right answer: one implementation of the capture is better than two.

`station/powershell/` is for the estate that has no bash and will not be given
any. Run its preflight first - it changes nothing, and it answers the two
questions that decide whether a station can work on that machine at all:

```powershell
.\start.ps1 --check
```

**Constrained Language Mode** stops the capture dead. Under it, .NET method
calls and type literals are refused, which is most of `caplib.psm1`, and the
failure otherwise reads as a syntax error in somebody else's file rather than
as a policy decision. The preflight names it, and names AppLocker/WDAC and
`__PSLockDownPolicy` as what sets it.

**A GPO-set execution policy overrides `-ExecutionPolicy Bypass`.** A station
that launches fine by hand then refuses to launch from a scheduled task. The
preflight reads the policy *per scope*, because the effective value alone does
not say who set it and therefore does not say whether you can change it.

It also reports which **cancel** this machine gets: a Job Object where
`Add-Type` is permitted, and `taskkill /T /F` where it is not. The second walks
the child tree at the moment it runs, so a process started immediately after
can survive - worth knowing before you rely on cancelling a long step.

### Planting it

Three ways, and they produce the same tree. A test in this repository runs all
three against the same checkout and compares the results file by file.

```powershell
# from a machine with the CLI
heliograph bootstrap C:\ops\my-task --flavour powershell

# from a checkout, with PowerShell and nothing else - which is the point
.\station\bootstrap.ps1 C:\ops\my-task

# from a checkout, with bash
./station/bootstrap.sh /c/ops/my-task --flavour powershell
```

`--flavour both` puts both payloads in one repo, for a transport repo serving
two machines of different kinds. It is not a recommendation: a machine gets one
station, and an estate that has bash should run the bash one.

### The loop

```powershell
.\station.ps1                    # poll, run, deliver, repeat - READ-ONLY
.\station.ps1 --once             # do one requested run, then exit
.\station.ps1 --interval 15      # seconds between polls (default 5)
.\station.ps1 --allow-actions    # also run steps that declare themselves actions
.\station.ps1 --allow-root       # permit a privileged account
.\station.ps1 --pin              # approve the current steps, for REQUIRE_PIN=1
```

It is the twin of `station.sh`: the same request document, the same published
status, the same four gates, the same exit codes. A control side reads one
document and cannot tell which implementation wrote it - and a test asserts
exactly that, by running both against the same request and comparing the keys
they publish.

**`run.ps1` carries gates 1, 2 and 4.** A step declares `# heliograph-mode:
read-only` or `action` or does not run; an action needs `CONFIRM=yes`; nothing
runs as Administrator or SYSTEM unless `ALLOW_ROOT=1`.

**`station.ps1` carries gate 3**, which cannot live in the runner: the station
must have been *started* with `--allow-actions`, and a runner invoked by hand
has no station behind it to ask. The refusal is **published** within one poll,
with the flag that would allow it - which is what makes a read-only default
affordable instead of a wasted day.

`ACTION_ENV` catches what a declaration cannot see. A step that plans is
read-only until `env: APPLY=1` makes it apply, so that env line is gated as an
action too.

### What the request may not say

The `env:` line becomes variables for the run, and the names that decide where
a log goes are refused: `TRANSPORT`, `PUSH`, `REDACT`, `LOG_DIR`, the gate
variables, and **every transport's own configuration** - `RELAY_*`, `SHARE_*`,
`OBJSTORE_*` and the rest. Each of them turns a working station into one that
looks fine and delivers nothing, or delivers to somebody else, or publishes an
unredacted log that cannot be unpublished. They are settled when the station is
started, not per request.

Reserved by **prefix rather than by name**, the same rule `station.sh` applies,
and `tests/test-station-gate.sh` compares the two patterns so the twins cannot
drift apart on it.

**This one check is case-INSENSITIVE, and it is the only one here that is.**
Every other gate compares case-sensitively, because `read-only` and `READ-ONLY`
are different declarations. Environment variable names on Windows are not:
`transport=relay` and `TRANSPORT=relay` are the same entry in the child's
environment, and the second overwrites the first. A case-sensitive guard would
refuse the uppercase spelling, allow the lowercase one, and Windows would
honour it. The bash twin needs no equivalent - there `transport=relay` sets a
genuinely different variable that nothing reads.

The check is on the **parsed** name, not the text of the line: `FOO=1
"TRANSPORT=relay"` walks straight past a check on the raw string and still
reaches the step as a plain assignment.

Nothing in the line is evaluated. Shell metacharacters are refused by name, so
the two implementations refuse the same set of requests - `station.sh` passes
that line to an `eval` and must, and this one agrees with it rather than being
quietly more permissive.

### Watching, and stopping

While a step runs the station publishes the partial log every
`PROGRESS_EVERY` seconds (default 60, `0` disables), with a line count and the
last real line. Without it a long step is a black box: *running for forty
minutes* and *wedged* look identical from the only side that can see anything.

`cancel: yes` in the request kills the step running right now; `cancel: <id>`
kills it only if that is the id running, so a stale cancel cannot reap a later
run. A cancelled run publishes `cancelled` and **still delivers the partial
log** - which is usually the evidence you wanted. `stop: yes` ends the station
cleanly.

### What it delivers, and how you know

`transports/git.psm1` and `transports/share.psm1` carry both halves of the
contract: fetching a request and publishing a status and progress, as well as
shipping the finished log.

`idle` means **the log arrived**. Anything else is published as `undelivered`
with the reason - including the case where the runner exited before it ever
reached delivery, which is the one an earlier version of the bash loop reported
as a clean run with a log nobody would ever receive.

The credential is handled the way the bash transport handles it: through
`GIT_CONFIG_*` rather than `git -c`, so a token never reaches the process table
where `ps` shows it to every other user on the box. `Get-TpDescribe` masks both
halves of a URL's userinfo, because `https://<token>:x-oauth-basic@host` is a
documented git form in which **the secret is the username**.

**Surviving a logout** is `.\service.ps1 install`, which this payload ships - a
scheduled task, with the transport's variables carried into a restricted file
because a task inherits none of them. See [service](/service).

### The relay works here, and needs nothing installed

git, the file share and **the relay**. That last one is the reason this payload
is worth having on a locked-down box at all: the bash station's relay shells out
to `heliograph-seal`, a native Go binary, and an estate that refuses bash is not
going to permit that either.

So the seal is built in **managed C#**, shipped as source under `lib/seal/` and
compiled by `Add-Type` when the transport loads. X25519, Ed25519 and Poly1305
come from a vendored Chaos.NaCl - djb's ref10 from SUPERCOP, MIT - and ChaCha20,
the RFC 8439 framing and HKDF-SHA256 are written beside them. Nothing to
install, nothing to checksum, and still plain text you can read first.

It is held to the Go implementation rather than trusted to match it.
`internal/seal` emits golden vectors from fixed keys; this side must reproduce
them byte for byte at every stage and open what Go sealed, and CI runs the whole
conformance suite over the relay on both editions.

Two things follow from `Add-Type`, and both are worth knowing before you plan
around it:

- **Constrained Language Mode rules the relay out**, along with everything
  else. `Add-Type` is refused under CLM - but so is most of the capture, and
  `start.ps1` refuses to start at all under it. An estate in CLM has no
  station, relay or otherwise
- **The first load compiles about 330 KB of C#**, once per process. That is a
  few seconds at station start and nothing thereafter

**A self-update needs a restart.** `run.ps1`, `caplib.psm1` and the steps come
forward with no restart at all, because every run is a fresh process that loads
them again. `station.ps1` itself cannot be replaced while it is running -
PowerShell has no `exec`, and starting a replacement is worse than useless
under the Job Object the cancel depends on, because the moment the old process
exits the kernel terminates the new one. So the loop exits **75** and says so,
which a scheduled task or a systemd unit treats as *restart me*. Started by
hand, it prints the command to type.

**A `kill` leaves the lock file.** Windows PowerShell 5.1 cannot catch a
SIGTERM, so the `finally` that releases `.station.lock` never runs. The next
station reads the pid, finds it dead, says `clearing a stale lock` and starts.
`station.sh` traps the signal and does remove it.
