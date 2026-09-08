# The PowerShell station, the transports that only half exist, and documenting all of it

**Date:** 2026-09-08
**Status:** draft, for review

Three pieces of work that turned out to be one. The repo claims five transports
and two station implementations. It has one and a half of each, and the
documentation site describes the near side only. This spec makes the claims
true, builds the second station, and publishes what the far side actually is.

## Why these are one piece of work and not three

The PowerShell station exists for a Windows Server estate with no Git for
Windows and no permission to install it. That estate has no git, so the
PowerShell station needs a transport that is not git, and the two non-git
transports it would use are the two that do not fully work. Building the
station first would mean building it against a contract with a hole in it.

So the order is forced: fix the transport contract, then port the capture onto
it, then document the result.

## Part A - make the transport claims true

### A0. The finished log does not ship on relay or blob, and this is a regression

The one that matters, the reason this part comes first, and worse than an
omission: the behaviour existed and the transport refactor lost it.

`docs/specs/2026-09-06-heliograph-next-design.md:57` specifies a
`transport_put_log` verb. It was never implemented: `tp_put_log` is defined
nowhere in the repo. `run.sh:296` ends at `cap_push`, which is unconditional
git, and neither `run.sh` nor `caplib.sh` contains a single `tp_*` call.

The `alsofile` mechanism does not cover it either. `station.sh` passes a log to
`publish_status` as its fifth argument on the **cancelled** path only
(`station.sh:651`); the `idle` path at `station.sh:655` passes four arguments.
And `transports/git.sh`'s `tp_put_status` accepts that third parameter and
commits the file, while `transports/relay.sh:190` and `transports/blob.sh:163`
take only the body and drop it.

So on relay and blob the only log content crossing the gap is the
`tp_put_progress` snapshot taken every `PROGRESS_EVERY` seconds *during* the
run. The completed log never arrives - no footer, no `exit code`, no `RESULT` -
and a cancelled run's partial log does not arrive either. With
`PROGRESS_EVERY=0` nothing arrives at all.

**The regression.** `pigeonhole.sh:531-553`, the standalone blob loop this
refactor replaced, does it correctly and completely: it runs the step with
`PUSH=0` because there is no repo to commit to, finds the log, `drop_put`s it,
and publishes `idle` with the blob name and line count - or publishes a distinct
`undelivered` state when the upload fails, so that the far side can tell "the
log exists and could not be shipped" from "the step hung". The A4 port onto the
transport interface deleted roughly 500 duplicated lines and this behaviour with
them.

`station.sh` also runs `./run.sh` without `PUSH=0`, so on a relay or blob
station `cap_push` still attempts a git push. In a bootstrapped transport repo
that is a git clone, it may push the log to a git remote that is not the
transport anyone is reading. Where there is no remote, it prints `PUSH FAILED -
run 'git push' manually` to a terminal nobody is watching.

This is AGENTS.md constraint 2 broken on two of three transports, and it is
invisible from the control side because the log is written correctly to local
disk every single time. Only somebody waiting on the far side would notice, and
they cannot tell it from a station that died.

**The fix.** Add `tp_put_log` to the contract and to all three transports.
`run.sh` publishes the finished log through it rather than through `cap_push`;
`cap_push` becomes the git implementation of it, so the git path keeps its
careful rebase-and-retry behaviour unchanged and is not re-litigated. Restore
the `undelivered` state, and check that the CLI and
`skill_coherence_test.go`'s `TestStatusStatesAreAllKnownHere` know it.

`caplib.sh` gains no transport knowledge. `run.sh` sources the selected
transport the way `station.sh` already does, and calls `tp_put_log`. The runner
still owns the log; what changes is only which function carries it out.

### A1. A new conformance property, because the contract had a hole

Property 9: **the finished log reaches the far side.** After a run completes,
the transport holds a log containing the footer and the real exit code.

This is the property that would have caught A0 four transports ago. It is added
as a property rather than as a unit test for exactly the reason the suite
exists: it has to hold for every implementation and every transport, and a test
of one implementation cannot see it.

### A2. Relay is unreachable from the control CLI

`internal/transport/relay.go` is a complete implementation. Nothing can select
it: `cmdInit` accepts `git|share|bundle|objstore`, `open()` has no relay case,
`estate.Estate` has no fields for the relay's URL, station, identity, peer or
token, and no command performs the key exchange. Meanwhile
`site/content/transports.md` documents

```bash
heliograph init payments --transport relay --url https://... --estate payments
```

which cannot work, and `README.md`'s status table says relay works.

**The fix.** Estate fields for the relay's identifiers, keeping the existing
rule that secrets never enter the estate file - the token comes from the
environment as the S3 keys already do. `init --transport relay`, an `open()`
case, and a key-exchange command so an operator can generate an identity and
trust a peer without hand-editing files. `heliograph-seal` already has
`keygen` and `fingerprint`; this wires them into a procedure rather than
inventing cryptography.

### A3. No shipped host can run a non-git station

`station.sh:145` is `TRANSPORT="${TRANSPORT:-git}"`, and nothing else in the
repo ever sets `TRANSPORT`: not `docker/entrypoint.sh`, not
`kubernetes/heliograph.yaml`, not any of the five Azure templates, not the
pipelines. Nothing plants `heliograph-seal`, which `transports/relay.sh`
requires at `$REPO_ROOT/heliograph-seal`.

So selecting relay today means hand-setting `TRANSPORT` plus seven `RELAY_*`
variables and planting a binary yourself, on a machine you cannot reach.

**The fix.** `TRANSPORT` and each transport's variables plumbed through the
Docker entrypoint, the Kubernetes manifest and all five Azure templates.
`heliograph-seal` planted by `bootstrap` with `RELAY_SEAL_SHA256` set, so the
checksum pin the station already enforces is actually populated rather than
warned about.

### A4. `start.sh` hard-fails a station that is not a git clone

`credential()` reports `FAIL` when `git remote get-url origin` is empty, and a
non-zero `FAILED` stops the station before it reaches `station.sh`. A
relay-only or blob-only station therefore cannot be started through the one
command the operator is told to run.

**The fix.** The preflight becomes transport-aware: it asks the selected
transport what it needs and reports against that, rather than assuming git.
`tp_check` and `tp_describe` already exist for this and are already called by
`station.sh`; `start.sh` should use the same two rather than a second, git-only
copy of the question.

### A5. Share, bundle and object store have no far side

`station/bash/transports/` is `git`, `blob`, `relay`. The CLI implements
`share`, `bundle` and `objstore`, and `site/content/transports.md` marks all
three as working. They work on the control side only: a share station cannot
start, because `station.sh` would look for `transports/share.sh` and not find
it.

**The fix.** `transports/share.sh`, which is the cheapest of the three to write
and the one the PowerShell station needs anyway. `bundle` is honestly not a
loop and should be documented as an export/import procedure rather than as a
station transport. `objstore` gets a far side or gets its status corrected;
that decision is deferred to Part A's implementation plan rather than guessed
at here.

## Part B - the pure PowerShell station

Workstream A8 in the master design. That document already rewrote AGENTS.md
constraint 7 from "never a port" to "a second implementation is permitted only
while it passes the conformance suite". This is that implementation.

### The floor is Windows PowerShell 5.1

5.1 ships in-box on Windows Server 2016 and later and needs no install. That is
the entire point: "we support Windows, provided you first install Git for
Windows" is a weak claim on precisely the estates this targets, and replacing
it with "provided you first install PowerShell 7" would be the same claim in a
different hat.

The costs are real and accepted: no ternary operator, no `$PSStyle`, no
`ForEach-Object -Parallel`, and .NET Framework 4.8 cryptography only.

### Layout

```
station/powershell/
  start.ps1              preflight, then hand over
  station.ps1            the loop
  run.ps1                the step runner
  caplib.psm1            the capture, and only the capture
  transports/git.ps1
  transports/share.ps1
  transports/relay.ps1
  steps/_template.ps1
  steps/env-snapshot.ps1
  steps/net-probe.ps1
```

`station/bash/station.ps1` stays exactly where it is and keeps its current job.
It is the launcher for a Windows box that *does* have Git for Windows, and one
implementation of the capture is still better than two where there is a choice.
The two are told apart by which payload was planted, not by a flag inside one
of them.

### The capture, and the one thing that must not break

Constraint 1 says every captured line carries a UTC timestamp applied when the
line is produced. In PowerShell the analogue of bash's `while IFS= read -r` is a
streaming `ForEach-Object`: the pipeline passes objects one at a time, so the
stamp is taken as each line arrives from the native command.

```powershell
& $exe @rest 2>&1 |
  ForEach-Object { '{0} | {1}' -f [DateTime]::UtcNow.ToString('HH:mm:ss'), (Get-Text $_) } |
  ForEach-Object { $_ -replace $AnsiPattern, '' } |
  ForEach-Object { Protect-Secret $_ } |
  Write-Capture -Path $OutFile
```

**What must never appear on that path**, and this is the PowerShell spelling of
the busybox failure the whole conformance suite exists for: `$( )`,
`Out-String` without `-Stream`, `Select-Object`, `Sort-Object`, `Group-Object`
and `-Wait` all buffer the entire stream. Any one of them gives every line in a
block the same timestamp, and the log still reads perfectly. Property 1 and
property 2 are what catch it, which is why the suite is the gate on this work
rather than a formality after it.

Five further traps, each the PowerShell equivalent of something the bash side
already paid for:

- **stderr becomes objects.** `2>&1` on a native command yields `ErrorRecord`s,
  not strings, and `$ErrorActionPreference = 'Stop'` turns them into
  terminating errors. `service.ps1` records being bitten by exactly this on a
  real Windows runner. The capture runs with `Continue` and converts
  explicitly.
- **`$LASTEXITCODE` is clobbered by the next native call.** It is read
  immediately after the pipeline, and it is only set by native commands, so a
  step that is pure PowerShell needs its own `exit` handling. `ps_step` in
  `run.sh` already documents this one: a good snapshot reported failure because
  a probe's internal `git config` exited 1.
- **Encoding.** `[Console]::OutputEncoding` must be UTF-8 or the OEM codepage
  mangles non-ASCII. The log is written through a `StreamWriter` with
  `UTF8Encoding($false)` rather than `Out-File -Encoding utf8`, because 5.1
  writes a BOM and these logs are read on Linux.
- **Trailing CR.** Stripped in the capture, once, for the same reason
  `cap_run` strips it: invisible in a terminal, wrong in the file, and it
  breaks any later `grep` anchored with `$`.
- **TLS.** 5.1's `Invoke-WebRequest` honours
  `[Net.ServicePointManager]::SecurityProtocol`, which on older builds defaults
  below TLS 1.2. The relay transport forces it, or presents as an
  inexplicable connection failure on a machine nobody can log into.

### The gates, in Windows terms

The four gates keep their names and their exit codes, so that one set of
documentation is true of both implementations.

- **Mode declaration.** Unchanged: `# heliograph-mode: read-only` or `action`
  in the first 30 lines of the step's own file. Exit 3 when absent.
- **`CONFIRM=yes`** for an action. Unchanged.
- **`ALLOW_ACTIONS`**, default 0, with the refusal published within one poll.
  Unchanged.
- **The privileged-account refusal**, exit 5. The Windows analogue of root is
  running elevated or as `SYSTEM`: `WindowsPrincipal.IsInRole(Administrator)`,
  plus an explicit check for `S-1-5-18`. `ALLOW_ROOT=1` keeps its name rather
  than gaining a Windows synonym, because a second spelling is a second thing
  to document and to get wrong.

### Cancelling, without process groups

Windows has no process group to signal. A `taskkill /T /F` is in-box and works,
but races: a grandchild spawned between the enumeration and the kill survives.

The design uses a **Win32 Job Object** with
`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, which is the actual Windows equivalent of
what `setsid` buys the bash station, and cannot leak a grandchild. It needs
`Add-Type`, so `taskkill /T /F` remains a real fallback rather than decoration:
a hardened estate may run PowerShell in Constrained Language Mode, where
`Add-Type` is blocked.

### The preflight answers the two questions that actually decide this

`start.ps1` reports on the usual things, and on two that are specific to a
locked-down Windows Server and will otherwise present as an incomprehensible
failure mid-run:

- **Language mode.** Under Constrained Language Mode most .NET method calls are
  blocked and the capture cannot work. Detected and reported as blocking, with
  the remedy, because a preflight line that names a problem without a remedy is
  a defect - the person reading it usually cannot ask us.
- **Execution policy.** `-ExecutionPolicy Bypass` is a per-process setting and
  is normally permitted, but a GPO-set policy overrides it. Detected and
  reported, with what to ask for.

### Transports: all three

- **`git.ps1`** where `git.exe` happens to be present. Same capabilities as
  `git.sh`, including `self` and `live`.
- **`share.ps1`** for a UNC path or mounted directory. No credential, no
  binary, no egress - the mount is the credential, which is also its whole
  security model and the documentation says so plainly. Its bash twin from A5
  is the same design in the other language.
- **`relay.ps1`** over HTTPS. The sealing is delegated to
  `heliograph-seal.exe`, unchanged, because .NET Framework 4.8 has no X25519,
  no Ed25519 and no ChaCha20-Poly1305, and hand-rolling any of them is exactly
  what the relay spec forbids and for exactly the reason it gives. The checksum
  pin is enforced as `relay.sh` enforces it.

Azure Blob is deliberately not in this set. It is reachable, but nothing asks
for it on a Windows estate that the other three do not already cover, and a
fourth transport is a fourth place for the gates to drift.

### The conformance driver is the deliverable, not the code

`tests/conformance/drivers/powershell.sh`. The harness stays bash and shells
out to `powershell.exe` or `pwsh`, so there is one suite and one set of
properties rather than two that agree by inspection.

Two existing properties are currently spelled in Unix and need generalising -
generalising, not weakening:

- **p6** currently fakes `id` on `PATH`. `cap_refuse_root` calls `id -u` rather
  than reading `$EUID` specifically so that a test could do that, which is a
  documented seam. PowerShell gets the equivalent seam, and the property
  becomes "the platform's privileged account is refused with exit 5", with the
  driver supplying the simulation.
- **p8** needs a `drv_capture_bg` that returns something killable as a tree on
  Windows.

**Redaction parity gets its own fixture.** One corpus at
`tests/fixtures/redaction-corpus.txt` with its expected output, and both
`cap_redact` and `Protect-Secret` asserted against it. The bash rules are
POSIX; the PowerShell ones are .NET regex. They will not be identical text and
must be identical in effect, and the only honest way to hold that is to make
both implementations mask the same corpus. A correct redactor that the second
implementation gets subtly wrong is the exact drift the suite exists for, and
it is the one where being wrong is not recoverable: these logs are committed.

### Bootstrap plants what you asked for

`heliograph bootstrap --flavour bash|powershell|both`, defaulting to bash.
`station/embed.go` and `internal/bootstrap` carry both payloads.
`station/bootstrap.sh` gains the same flag, and a new `station/bootstrap.ps1`
provides the no-CLI path for a Windows box with no bash - which is the premise
of the whole exercise, so bootstrapping it with a bash script would be absurd.

## Part C - documentation

The site documents the near side only. Everything about the far side - the
thing the product actually is - lives in `skills/heliograph/references/`, which
is tuned for an agent's context budget and is published nowhere.

This is workstream F3 in the master design, which already decided the
direction: one canonical source on the site, and the skill keeps only what
changes agent behaviour and links out.

### New pages

| page | covers |
|---|---|
| `station` | what runs on the far side, the loop, the lifecycle, the state files |
| `runner` | `start.sh`, `station.sh`, `run.sh`, `caprun.sh`, every `cap_*` and every knob |
| `steps` | writing one, the mode declaration, the templates, the shipped steps, `lib/` |
| `powershell` | the PowerShell station: floor, gates, traps, what it does not do |
| `windows` | both Windows paths - the launcher and the native station - and line endings |
| `hosts` | the host contract, and which hosts are proven versus recipes |
| `azure` | all five templates, and the deployment findings that were paid for live |
| `containers` | the Docker image and the Kubernetes manifest |
| `service` | systemd, launchd, `setsid`, and the Windows scheduled task |
| `relay` | setup an operator can follow: keys, tokens, the seven variables, self-hosting |
| `secrets` | `secret.sh`, `cap_redact`, and what a safety net is not |
| `security` | the gates, the blast-radius argument, and what this refuses to do |
| `conformance` | the capture contract, as the specification it is |
| `pipelines` | the GitHub Actions and Azure Pipelines templates |
| `bootstrap` | planting a station, with and without the CLI |

The site generator already refuses a page that is in no navigation, so nav
wiring is enforced rather than remembered.

### Corrections that are part of this work

- `site/content/transports.md` and `README.md` stop claiming a capability
  before Part A delivers it. Each transport's status reflects both sides of the
  gap, because a transport that works on one side is not a transport.
- `skills/heliograph/references/azure.md:3` says each of the five templates
  ships as bicep and as Terraform. `azure/function/` ships Terraform only.
- `tests/test-doc-coherence.sh` extends to cover the facts that will now be
  stated in two places, so the slimmed skill and the site cannot drift.

## Verification

Nothing here is complete until all of this passes:

```bash
gofmt -l . && go vet ./... && go test ./...
bash -n install.sh install-codex.sh
find skills station tests -name '*.sh' -exec bash -n {} +
shellcheck -S warning $(find . -name '*.sh' -not -path './.git/*')
./tests/run-tests.sh
./tests/conformance/conformance.sh tests/conformance/drivers/bash.sh
./tests/conformance/conformance.sh tests/conformance/drivers/powershell.sh
./tests/conformance/conformance.sh tests/conformance/drivers/mutant.sh   # must FAIL
```

plus, in CI, the conformance suite against both implementations across every
transport, on Linux, macOS and Windows, and the station purity gate still
refusing Go under `station/`.

And the honest limit, restated because it does not change: nothing here
exercises a capture against a real remote machine. The behaviour that matters
is what a log looks like after a round trip through someone else's terminal,
and no test asserts that.

## Order

Part A first, because Part B needs a transport contract with no hole in it.
Part C last, because documenting a claim before it is true is how the current
drift happened.

Within Part A, A0 leads: a log that does not arrive is a worse defect than a
transport that cannot be selected, and it is the one nobody on this side of the
gap can see.
