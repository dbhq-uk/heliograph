# PowerShell station, transport contract and full documentation - PR roadmap

**Goal:** Make every transport claim true end to end, build a pure PowerShell
station that proves itself against the conformance suite, and publish what the
far side actually is.

**Spec:** [`docs/specs/2026-09-08-powershell-station-and-full-documentation-design.md`](../specs/2026-09-08-powershell-station-and-full-documentation-design.md)

19 PRs across three tracks. Track A must land before track B, because the
PowerShell station's premise is a box with no git, and the non-git transports
are the ones with the hole in them. Track C is last, because documenting a
claim before it is true is how the current drift happened.

## What has landed

Updated as work merges, so this file stays the register rather than a snapshot.

| | | |
|---|---|---|
| PR 1 | status claims match the code, plus a drift guard | **merged** (#21) |
| PR 2 | `tp_put_log`, so the finished log ships on every transport | **merged** (#24) |
| Track C | the far side published: 14 pages, 8 -> 22, plus coverage guards | **merged** (#25) |

Track C landed early, out of the order below, and the reason is worth keeping:
writing the pages is what found the defects. Documenting a component forces
somebody to state what it does, and three claims turned out to be false the
moment they were written down - including one that meant a relay or blob station
could never run a step at all. The pages describe only what is true today, and
say plainly what is not yet.

Two adversarial review passes ran against this work (codex, high effort,
read-only). Between them they found ten defects, five of which I had missed
entirely, including the fatal one above and a request-side guard that quoting
walked straight through. Both passes are recorded on #22 and #25.

Still open, in the order below: PRs 3 to 7 of track A, and the whole of track B.

## Global constraints

Every PR in this roadmap obeys these. They are not restated per task.

- **The station gains no dependency.** No Go, no binary, no interpreter and no
  package under `station/`, with the two existing exceptions: `station/embed.go`,
  which never ships, and `heliograph-seal`, argued for in the relay spec
- **PowerShell floor is Windows PowerShell 5.1**, in-box on Server 2016+. No
  ternary, no `$PSStyle`, no `-Parallel`, .NET Framework 4.8 only
- **Every gate keeps its name and its exit code** across both implementations:
  mode declaration, `CONFIRM=yes`, `ALLOW_ACTIONS`, `ALLOW_ROOT`, exits 2/3/5/130
- Station scripts use `set -uo pipefail`, never `set -e`
- House style: British English, plain hyphens, **no em dashes**, no trailing
  full stops on headings. Commit messages say what changed and what it cost
- Green before every commit: `gofmt -l .`, `go vet ./...`, `go test ./...`,
  `shellcheck -S warning`, `./tests/run-tests.sh`, and the conformance suite
  against every driver including the mutant, which must still **fail**

---

# Track A - make the transport claims true

## PR 1 - stop claiming what does not work

**Why first:** somebody could plant a relay station today and never receive a
log. Docs-only, no code, mergeable immediately.

**Files:** `README.md`, `site/content/transports.md`,
`skills/heliograph/references/azure.md`

- Status table entries reflect **both sides of the gap**. A transport that works
  on the control side only is not a transport
- Remove the `heliograph init --transport relay --url ... --estate ...` example
  from `transports.md`; it names flags that do not exist
- `azure.md:3` says each of the five templates ships as bicep and Terraform.
  `azure/function/` ships Terraform only

**Test:** a new coherence assertion that every transport named as working in
`transports.md` has both a `internal/transport/*.go` implementation reachable
from `cmdInit` and a `station/bash/transports/*.sh`. This is the check whose
absence let the claim drift.

## PR 2 - `tp_put_log`, so the finished log ships on every transport

**Why:** the regression described in the spec at A0. `pigeonhole.sh:531-553`
ships the final log correctly; the transport-interface port that replaced it
does not, so on relay and blob the completed log never crosses the gap.

**Files:** `station/bash/transports/{git,blob,relay}.sh`, `station/bash/run.sh`,
`station/bash/station.sh`, `tests/test-transport-contract.sh`,
`tests/conformance/conformance.sh`

- `tp_put_log <logfile> <message>` added to the contract; `tp_capabilities`
  unchanged, because every transport must implement this one
- `git.sh`'s implementation is today's `cap_push`, moved, so its rebase and
  retry behaviour is not re-litigated
- `blob.sh` and `relay.sh` implement it as a put of the log under its own name
- `run.sh` sources the selected transport and calls `tp_put_log`. It runs with
  `PUSH=0` where the transport is not git, so `cap_push` stops attempting a git
  push on a station whose transport is not git
- Restore the `undelivered` status state that `pigeonhole.sh` publishes and
  `station.sh` does not, so the far side can tell "the log exists and could not
  be shipped" from "the step hung"
- **Conformance property 9:** after a run completes, the transport holds a log
  containing the footer and the real exit code

**Test:** property 9 against every driver. `test-transport-contract.sh` adds
`tp_put_log` to `REQUIRED`. Check `undelivered` against
`skill_coherence_test.go`'s `TestStatusStatesAreAllKnownHere`.

## PR 3 - the preflight stops assuming git

**Why:** `start.sh`'s `credential()` reports `FAIL` when there is no `origin`
remote, and a non-zero `FAILED` stops the station before `station.sh` runs. A
relay-only or blob-only station cannot be started through the one command the
operator is told to type.

**Files:** `station/bash/start.sh`, `tests/test-start.sh`

- Ask the selected transport through `tp_check` and `tp_describe`, which
  `station.sh` already calls, rather than keeping a second git-only copy of the
  question
- The git checks stay, and stay exactly as thorough, for the git transport
- Every `FAIL` still names a remedy

## PR 4 - `transports/share.sh`

**Why:** the CLI implements `share` and nothing on the far side can read it.
Cheapest of the three missing far sides, and the PowerShell station needs the
design.

**Files:** `station/bash/transports/share.sh`, `tests/test-share.sh`

Capabilities `request status progress log self live`: a directory can carry a
new payload, and a live read is a file read. Its security model is the mount,
and the documentation says so plainly rather than implying more.

## PR 5 - relay reachable from the control CLI

**Files:** `internal/estate/estate.go`, `cmd/heliograph/main.go`, tests

- Estate fields for the relay's URL, estate, station, identity path and peer
  path. **The token is not a field** - it comes from the environment, as the S3
  keys already do, for the reason `estate.go:29-32` gives
- `init --transport relay`, an `open()` case, and a key-exchange command that
  wires up `heliograph-seal keygen` and `fingerprint` into a procedure an
  operator can follow rather than a set of files to hand-edit

## PR 6 - a host can select a transport

**Why:** `station.sh:145` defaults `TRANSPORT` to git and nothing else in the
repo ever sets it. Selecting relay today means hand-setting eight variables and
planting a binary on a machine you cannot reach.

**Files:** `station/bash/docker/entrypoint.sh`,
`station/bash/kubernetes/heliograph.yaml`, `station/bash/azure/*/`,
`station/bash/pipelines/*`, `internal/bootstrap/`

- `TRANSPORT` and each transport's variables plumbed through every host
- `heliograph-seal` planted by `bootstrap`, with `RELAY_SEAL_SHA256` actually
  populated, so the pin `relay.sh:62` already enforces stops being a warning
- **Decide and record:** `azure/function/` runs the old `pigeonhole.sh` path
  (`PIGEONHOLE_RESUME`, `PIGEONHOLE_ONCE`), not the transport interface. Either
  port it or document why it stays. It is the only shipped non-git host, which
  is precisely why nobody noticed PR 2's defect

## PR 7 - conformance across every transport, in CI

Workstream A6. This is what stops all of the above regressing again.

**Files:** `tests/conformance/drivers/`, `.github/workflows/station.yml`

---

# Track B - the pure PowerShell station

Gated on PR 2, 3 and 4. Workstream A8, under the constraint-7 rewrite the
master design already made: a second implementation of the capture is permitted
**only while it passes the conformance suite**.

## PR 8 - the driver and the harness, before any implementation

**Why this order:** properties 6 and 8 are currently spelled in Unix. Generalise
them while there is no PowerShell implementation to bend them towards.

**Files:** `tests/conformance/conformance.sh`,
`tests/conformance/drivers/powershell.sh`, `tests/fixtures/redaction-corpus.txt`

- **p6** becomes "the platform's privileged account is refused with exit 5",
  with the driver supplying the simulation. `cap_refuse_root` calls `id -u`
  rather than reading `$EUID` specifically so a test can fake it; PowerShell
  gets the same documented seam
- **p8** gains a `drv_capture_bg` that returns something killable as a tree on
  Windows
- **The redaction corpus** is created here and **bash is asserted against it
  first**, which is what proves the corpus is right before a second
  implementation is measured by it

## PR 9 - `caplib.psm1`

The capture, and nothing else. Passes properties 1-4, 7 and 9.

**What must never appear on the capture path**, this being the PowerShell
spelling of the busybox failure the whole suite exists for: `$( )`,
`Out-String` without `-Stream`, `Select-Object`, `Sort-Object`, `Group-Object`,
`-Wait`. Each buffers the stream, gives every line in a block the same
timestamp, and leaves a log that reads perfectly.

Five further traps, each the PowerShell equivalent of something the bash side
already paid for: `ErrorRecord` objects from `2>&1` under
`$ErrorActionPreference`; `$LASTEXITCODE` clobbered by the next native call and
unset by pure-PowerShell steps; `[Console]::OutputEncoding` and the OEM
codepage; the trailing CR; and 5.1's `SecurityProtocol` defaulting below TLS 1.2.

## PR 10 - `run.ps1`

The runner and all four gates. Passes properties 5 and 6. The privileged-account
check is `WindowsPrincipal.IsInRole(Administrator)` plus an explicit `S-1-5-18`,
and keeps the name `ALLOW_ROOT` rather than gaining a Windows synonym.

## PR 11 - `station.ps1` and `start.ps1`

The loop, and the preflight. Cancel uses a Win32 **Job Object** with
`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` - the real equivalent of what `setsid` buys
the bash station - with `taskkill /T /F` as a genuine fallback, because a
hardened estate may block `Add-Type`.

The preflight answers the two questions that actually decide this on the target
estate, each with a remedy: **Constrained Language Mode**, under which the
capture cannot work, and a **GPO-set execution policy**, which overrides
`-ExecutionPolicy Bypass`.

`station/bash/station.ps1` is untouched. It remains the launcher for a Windows
box that does have Git for Windows, and one implementation of the capture is
still better than two where there is a choice.

## PR 12 - `transports/{git,share,relay}.ps1`

All three, per the decision recorded in the spec. Azure Blob is deliberately not
in the set: nothing asks for it on a Windows estate that these three do not
cover, and a fourth transport is a fourth place for the gates to drift.

## PR 13 - bootstrap plants it

`heliograph bootstrap --flavour bash|powershell|both`, default bash.
`station/embed.go` and `internal/bootstrap` carry both payloads. A new
`station/bootstrap.ps1` gives the no-CLI path, because bootstrapping a
no-bash box with a bash script would be absurd.

## PR 14 - Windows CI

Conformance against both drivers on a real Windows runner, plus Linux and macOS.

---

# Track C - documentation

## PR 15 - the station pages
`station`, `runner`, `steps`, `bootstrap`

## PR 16 - the host pages
`hosts`, `containers`, `service`, `azure`, `pipelines`. All five Azure templates
documented, including the deployment findings that were paid for live.

## PR 17 - transports and security
`relay` (setup an operator can follow: keys, tokens, the variables,
self-hosting), `secrets`, `security`, `conformance`

## PR 18 - Windows and PowerShell
`windows`, `powershell`

## PR 19 - slim the skill, extend the coherence gate

`skills/heliograph/references/` keeps what changes agent behaviour and links out
for reference material. `tests/test-doc-coherence.sh` covers every fact now
stated in two places, so the split cannot drift.

---

## The honest limit

Restated because it does not change, and PR 14 will make it tempting to forget:
nothing here exercises a capture against a real remote machine. The behaviour
that matters is what a log looks like after a round trip through someone else's
terminal, and no test in this roadmap asserts that.
