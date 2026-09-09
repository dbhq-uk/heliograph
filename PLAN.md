# PLAN

The living register. Written so the state survives a context compaction, a
handover, or a week away. Update it as work lands rather than at the end.

**Detail lives elsewhere and is not repeated here:**
[`docs/specs/`](docs/specs/) for designs,
[`docs/plans/2026-09-08-powershell-and-docs-roadmap.md`](docs/plans/2026-09-08-powershell-and-docs-roadmap.md)
for the 19-PR breakdown. This file says where we are and what is next.

---

## Where we are

Git, the file share and the relay work end to end, each with its own round trip
in CI. The site documents the far side. There is no PowerShell station.

| | |
|---|---|
| control CLI over git | works, driven end to end in CI against a real station |
| relay | **works end to end**, driven against the deployed relay at `heliograph-relay.dbhq.uk` on 2026-09-09 |
| Azure Blob | works end to end via `drop.sh` and `pigeonhole.sh`, not via the CLI |
| file share | **works end to end**, proved by a CLI round trip in CI |
| bundle, object store | control side only; **no station side at all** |
| bash station | in use; the loop, the gates, the capture |
| PowerShell station | `station.ps1` is a launcher. No native station |
| site | 24 pages, near and far side. **Measured and indexed from 2026-09-09**: GA4 on the dbhq.uk stream behind consent, sitemap with `lastmod` submitted to Search Console |

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
| #35 | this file |
| #36 | **the preflight stops assuming git** - `tp_preflight`, `tp_sync`, and a token that was being printed |
| #37 | **`transports/share.sh`** - the file share gets its far side |
| #38 | **the relay, reachable and usable** - and four defects only a round trip could find |
| #39 | proved over the deployed relay, and CI keeps asking |
| #40 | **containers and services can select a transport** - and twelve defects a review found in it |
| #41 | **`heliograph-seal` in the image** - a relay station runs in a container |
| #42 | the launchd flake carries its own diagnosis |

## Landed 2026-09-09

| PR | |
|---|---|
| - | **analytics and search on the site** - GA4 behind consent, `lastmod`, descriptions, JSON-LD, `og.png`, a 404 page, and the two font preloads that 404ed on every page view. Spec: [`docs/specs/2026-09-09-analytics-and-seo-design.md`](docs/specs/2026-09-09-analytics-and-seo-design.md). Keyword research: [`docs/seo/2026-09-09-keyword-research.md`](docs/seo/2026-09-09-keyword-research.md) |

## Next, in order

1. **The hosts that are still git-only** (the rest of PR 6). Containers,
   Kubernetes and the three service mechanisms carry a transport now. Still to
   do: the five Azure templates, the two pipeline definitions, and
   `service.ps1`
2. **Conformance across every transport in CI** (PR 7). Property 9 only
   exercises git today, so a no-op `tp_put_log` on another transport would pass
3. **Track B: the PowerShell station**, seven PRs, gated on 1. Windows
   PowerShell 5.1, carrying git, share and relay. The conformance driver is the
   deliverable, not the code
4. **The site's content gaps**, from the keyword research: a section on the
   security page for "claude code permissions" and sandboxing (5,400 to 2,900
   worldwide searches a month, soft SERPs), an air-gapped page once the bundle
   has a station side, and a comparison with AWS SSM and Azure Run Command.
   Which query each targets, and what it must not claim:
   [`docs/seo/2026-09-09-keyword-research.md`](docs/seo/2026-09-09-keyword-research.md) section 5

## Operational notes

- **A relay station in a container was driven against the deployed relay on
  2026-09-09.** The image carries `heliograph-seal` built from the same commit,
  and the log came back with a non-zero exit reported honestly. The station name
  used was `in-a-container`
- **The site reports into the dbhq.uk GA4 property** (`544327698`, stream
  `G-3H3NFGSX85`), not a property of its own. One web stream per site
  including subdomains is Google's guidance; separate the docs in reports by
  the **Hostname** dimension. The tag loads only after consent and only on
  `heliograph.dbhq.uk`. Nothing was created in the GA4 account
- **Search Console is the `sc-domain:dbhq.uk` Domain property**, which covers
  every subdomain. `https://heliograph.dbhq.uk/sitemap.xml` was submitted
  through the API on 2026-09-09 using the service account in
  `~/.dbhq-seo/env.sh`. At that moment the home page was "unknown to Google"
  and `/install` was "discovered, not indexed": the site had never been
  crawled. Check again in a week with the URL Inspection API before
  concluding anything from GA
- **The site build refuses a shallow clone.** `lastmod` comes from git, and a
  depth-1 checkout dates every page today. Both workflows that build the site
  set `fetch-depth: 0`; a new one must too
- **The relay estate is `heliograph`**, on `heliograph-relay.dbhq.uk`. Its
  control and station tokens are in 1Password, DBHQ vault, *heliograph relay -
  estate tokens*. **That is the only copy**: Cloudflare secrets are write-only,
  so `wrangler secret put HELIOGRAPH_RELAY_ESTATES` replaces a value nobody can
  read back. The previous value was unrecoverable and was replaced on
  2026-09-09; record any future one before setting it
- The same control token is a repository secret, so CI checks on every push to
  `main` that the deployed relay still answers it. Pull requests skip that step:
  a fork gets no secrets, and a false negative for a contributor is worse than
  the check

## Known defects, recorded and NOT fixed

Stated on the site rather than hidden, so nobody plans around a promise.

- **A cancelled run's partial log does not ship on blob or relay.** The station
  passes it as `tp_put_status`'s third argument, which only git and the share
  honour
- **`test-launchd.sh` has now failed three times**, always on the same
  assertion: *"launchd restarted the loop as pid N after a clean exit"*. The
  test writes `stop: yes`, watches until launchd reports no pid, waits eight
  seconds and asks again - and a pid was there.
  `KeepAlive { SuccessfulExit: false }` should forbid exactly that.
  Two readings, and they need opposite fixes: the loop exited NON-zero, so
  launchd restarted it correctly and the defect is upstream of the assertion; or
  launchd's respawn throttle raced the eight-second window, so the assertion is
  what is wrong. `launchctl print` carries *last exit code*, which separates
  them outright, and it cannot be read after the fact.
  **The test now captures that itself on failure**, along with the station's
  service log and the published status. Nobody had gathered it in three
  occurrences because every one of them was somebody re-running a job. The next
  failure carries its own diagnosis; do not re-run it without reading that

**Fixed on 2026-09-08, and recorded because they were on this list:** the relay
sequence collision between the loop and the runner is closed by a `mkdir` lock
and a max-taking state write. It was reproducible the moment a round trip
existed to run.

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

**A test written for one defect finds another.** Writing the "a state write
that failed must be reported" case made `_relay_lock` spin for ever, because it
waited on any `mkdir` failure and an unwritable directory is one. Moving that
check inside the retry loop then broke a working lock, because a holder
releasing between the failed `mkdir` and the test looks exactly like an
unwritable filesystem - 58 numbers out of 60, two takers refused for nothing.
Ask "can I ever create this" once, up front; ask "does it exist" only in the
loop.

**A stale-lock heuristic that can fire on a live holder is not a lock.** The
relay's sequence lock broke any lock directory untouched for a minute - and a
holder's mtime does not change while it works, so the breaker deleted live
locks and two processes went in at once. Four takers wanting fifteen numbers
each got 33 distinct numbers out of 60. It fails exactly like having no lock:
intermittently, silently, under load. Ask `kill -0` whether the recorded pid is
alive, which is what station.sh has always done.

**A `.dockerignore` is a file nobody re-reads, and it decides what ships.**
`**/secrets/*` looked like prudence and was wrong: `station/bash/secrets/` is
part of the payload, so the image quietly planted one file fewer than every
other way of planting a station, and nothing would have noticed. The image's
payload is now compared file-for-file against `bootstrap.sh`'s.

**Sourcing an env file does not export anything.** `. file` with `KEY=value` in
it sets a SHELL variable, and the next thing the LaunchAgent and the setsid
fallback do is `exec bash start.sh` - a new process, which inherits environment
variables and not shell ones. So the file was read and every value discarded,
and a relay station started as a git one. systemd was unaffected, because
EnvironmentFile exports for you: which is exactly how a defect ends up in two
mechanisms out of three and looks like working code in the one that is tested.
`set -a` around the source.

**One file, two parsers, is a specification.** The same `.station-env` is read
by systemd's EnvironmentFile and sourced by a shell, and an ordinary Azure SAS -
`?sv=...&ss=...&sig=...` - is a value to one and three background jobs to the
other. The file is validated at install time against the intersection of the two
languages rather than hoped about.

**A round trip finds what reading cannot.** The relay had four defects that
every review had walked past, and all four surfaced within an hour of the first
end-to-end run: the preflight proved its token by reading the station's own
request queue, and a relay deletes on collection, so every `./start.sh` silently
ate the waiting request; the loop and the runner collided on sequence numbers so
`idle` was dropped as a replay; `ListLogs` returned an error, leaving the
transport whose whole purpose is retrieving a log with no way to read one; and
`log: <none>` appeared in every status for a step sent by path, on every
transport, because the glob used the path rather than the label. None of them
errored anywhere.

**A reachability check is not a delivery check.** `tp_check` on the blob
transport counted an HTTP 404 as success, on the argument that an absent request
proves the account and the credential. A misspelt container, a wrong account and
a read-only SAS all answer exactly like that, so all three cleared the preflight
and failed on the first upload - an hour later, with nobody left to tell. Every
`tp_check` now proves a WRITE, the cheapest way its store allows.

**Checking one of a pair is checking neither.** The relay verified
`RELAY_IDENTITY` was readable and never `RELAY_PEER`, so a station with no peer
key started, then failed every verification and every seal. `tp_describe` turned
the failed fingerprint into `<unreadable>` and printed it beside an `ok`.

**A command substitution is a subshell, and a transport's `tp_init` sets
variables the rest of the run needs.** `why="$(tp_init 2>&1)"` looked like the
tidy way to fold a failure into the preflight table. It reported the transport
as `ok` and then killed every git check with `BRANCH: unbound variable`, because
`BRANCH=$b` had been set in the subshell and thrown away. Capture stderr through
a file when the function has to run in this shell.

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

macOS is absent locally, so the launchd suite skips. **CI runs it and CI has
caught real defects that skip hid** - do not read a local green as complete.

**Docker may only need starting.** `sudo systemctl start docker` was all it
took, and it turns the container and Kubernetes suites from skipped into 271
assertions that run in about ten minutes - including a whole station run in a
container. They found four defects in one afternoon that CI would have taken
four pushes to surface one at a time. Try it before pushing anything that
touches `station/bash/docker/`.
