# PLAN

The living register. Written so the state survives a context compaction, a
handover, or a week away. Update it as work lands rather than at the end.

**Detail lives elsewhere and is not repeated here:**
[`docs/specs/`](docs/specs/) for designs,
[`docs/plans/2026-09-08-powershell-and-docs-roadmap.md`](docs/plans/2026-09-08-powershell-and-docs-roadmap.md)
for the 19-PR breakdown. This file says where we are and what is next.

---

## Where we are

Git is the only transport that works end to end. The site documents the far
side. There is no PowerShell station.

| | |
|---|---|
| control CLI over git | works, driven end to end in CI against a real station |
| relay | station side complete; **no CLI command can select it** |
| Azure Blob | works end to end via `drop.sh` and `pigeonhole.sh`, not via the CLI |
| file share, bundle, object store | control side only; **no station side at all** |
| bash station | in use; the loop, the gates, the capture |
| PowerShell station | `station.ps1` is a launcher. No native station |
| site | 24 pages, near and far side |

## Landed 2026-09-08

| PR | |
|---|---|
| #20 | A8 spec: the PowerShell station, and `tp_put_log` |
| #21 | status claims match the code, plus a drift guard |
| #24 | **`tp_put_log`** - the finished log now ships on relay and blob |
| #25 | the far side published: 14 new site pages, plus coverage guards |
| #26 | the home header was the whole sitemap, three rows deep |
| #27 | mobile drawer, and the host/transport tables |
| #28 | the sidebar stays put when you use it |
| #29 | Codex first-class on the site |
| #30 | the git transport's credential, documented |
| #31 | spec: one repository, many stations |
| #32 | **`heliograph station add`** |
| #33 | the multi-station hardening |
| #34 | documented what shipped, and two more guards |

## Next, in order

1. **Transport-aware preflight** (roadmap A/PR 3). `start.sh`'s `credential()`
   FAILs without a git `origin`, and a non-zero `FAILED` stops before
   `station.sh` runs - so **a relay or blob station cannot be started by the
   command the operator is told to type.** Ask the transport through `tp_check`
   and `tp_describe`, which already exist and which `station.sh` already calls.
   This is what makes the matrix on `/hosts` stop saying "by hand"
2. **`transports/share.sh`** (PR 4). The CLI implements `share` and the station
   cannot read it. Cheapest missing far side, and the PowerShell station needs
   the design anyway
3. **Relay reachable from the CLI** (PR 5). `relay.go` is complete and
   unselectable. Needs estate fields and a key-exchange procedure; the keys are
   the hard part, not the plumbing
4. **Hosts can select a transport** (PR 6). `TRANSPORT` and the `RELAY_*` set
   through the Docker entrypoint, the Kubernetes manifest and the five Azure
   templates. Plant `heliograph-seal` with its checksum populated
5. **Conformance across every transport in CI** (PR 7). Property 9 only
   exercises git today, so a no-op `tp_put_log` on another transport would pass
6. **Track B: the PowerShell station**, seven PRs, gated on 1-4. Windows
   PowerShell 5.1, carrying git, share and relay. The conformance driver is the
   deliverable, not the code

## Known defects, recorded and NOT fixed

Stated on the site rather than hidden, so nobody plans around a promise.

- **A cancelled run's partial log does not ship on blob or relay.** The station
  passes it as `tp_put_status`'s third argument, which only git honours
- **Relay sequence numbers can collide** between the parent loop and the child
  runner: the parent loads `RELAY_OUT` once and does not reload before
  publishing `idle`, so it can emit a number the runner already used, and the
  receiver drops anything at or below what it has accepted - by design, because
  that is the replay defence
- **`test-launchd.sh` is flaky.** Failed once on a branch, passed on re-run,
  clean on main. Watch it; do not act yet

## Lessons this repository has already paid for

Added here when something cost real time. AGENTS.md holds the hard rules; this
holds what was learned proving them.

**A check nobody has watched fail is a check nobody knows works.** Two coverage
guards written on 2026-09-08 were wrong in ways that read as correct:
`GIT_(?:AUTH_HEADER|TOKEN|TOKEN_FILE|TOKEN_USER)` matched `GIT_TOKEN` first and
found two mechanisms out of four; `^\s+echo "([a-z]+):` missed every
conditional field, including the one it was written for. Both reported PASS.
Break every new assertion deliberately and watch it fail before keeping it.

**Documenting a component is how the defects are found.** Writing the far-side
pages turned up three false claims, including one fatal: `station.sh` required a
local `station/request` file, which blob and relay never create, so a relay
station could never run a step at all. Nothing else had noticed.

**Adversarial review finds what self-review does not.** Two codex passes on
2026-09-08 found ten defects, five missed entirely - a quoting bypass of the
env guard, `cap_push` returning 0 on failure, a committed test artefact that
`bootstrap` would have planted into every station.

**Read the skill before changing a default.** Pinning an estate to its branch
looked right and would have broken every existing user, because SKILL.md tells
you to work on `task/<slug>` and then send. `Scope` set means routing matters;
`Branch` alone means follow the checkout.

## Conventions that are easy to lose

- **Specs before code**, in `docs/specs/`, reviewed on their own
- **Every PR states what it cost** - the measurement, the failure it prevents,
  the thing that was tried and did not work
- **British English, plain hyphens, no em dashes**, no trailing full stops on
  headings
- **No backtick may appear inside a Go raw string** in `internal/site/theme.go`.
  This has broken the build twice, both times from a comment
- `station/bash/.station-delivery` appears when the suite runs locally. It is
  gitignored; do not commit it

## Verifying a change

```bash
gofmt -l . && go vet ./... && go test ./...
find skills station tests -name '*.sh' -exec bash -n {} +
shellcheck -S warning $(find . -name '*.sh' -not -path './.git/*')
./tests/run-tests.sh
./tests/conformance/conformance.sh tests/conformance/drivers/bash.sh
./tests/conformance/conformance.sh tests/conformance/drivers/mutant.sh   # must FAIL
go run ./cmd/heliograph-site site/content /tmp/site                      # 24 pages
```

`docker` and macOS are absent locally, so the container, Kubernetes and launchd
suites skip. **CI runs them and CI has caught real defects those skips hid** -
do not read a local green as complete.
