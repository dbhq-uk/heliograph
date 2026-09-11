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
in CI. The site documents the far side. **The PowerShell station is complete**: it
polls, runs, delivers and publishes, and it is planted by all three bootstraps.

| | |
|---|---|
| control CLI over git | works, driven end to end in CI against a real station |
| relay | **works end to end**, driven against the deployed relay at `heliograph-relay.dbhq.uk` on 2026-09-09 |
| Azure Blob | works end to end via `drop.sh` and `pigeonhole.sh`, not via the CLI |
| every host but a pipeline | carries a transport. Azure Blob works outright everywhere; the relay and the file share need a volume the templates do not mount |
| file share | **works end to end**, proved by a CLI round trip in CI |
| bundle, object store | control side only; **no station side at all** |
| bash station | in use; the loop, the gates, the capture |
| PowerShell station | **complete and proven**. Polls, runs, delivers and publishes over git and share, with all four gates. Every conformance property, on 5.1 and on 7. No relay transport (needs `heliograph-seal`, which is Go) |
| site | 26 pages, near and far side. **Measured and indexed from 2026-09-09**: GA4 on the dbhq.uk stream behind consent, sitemap with `lastmod` submitted to Search Console |

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
| #43 | **analytics and search on the site** - GA4 behind consent, `lastmod`, descriptions, JSON-LD, `og.png`, a 404 page, and the two font preloads that 404ed on every page view. Spec: [`docs/specs/2026-09-09-analytics-and-seo-design.md`](docs/specs/2026-09-09-analytics-and-seo-design.md). Keyword research: [`docs/seo/2026-09-09-keyword-research.md`](docs/seo/2026-09-09-keyword-research.md) |
| #44 | **Windows and the pipelines** - the scheduled task carries a transport |
| - | **the breadcrumb says the nav label, not the H1** - `/compared` read "heliograph / heliograph compared with AWS SSM Run Command and Azure Run Command", found by driving the deployed page rather than by a test |
| - | **the rail rebinds after a client-side navigation** - it had stopped marking the current section after one soft nav, on every docs page. Found by driving the deployed site; `swap()` now announces `hg:swap` |
| - | **the DBHQ menu, a `/dbhq` page, and the footer block removed** - #45's "Also from DBHQ" said the same three links on 27 pages, at the point a reader has stopped reading. Also fixed: `\| \| \|`, the headerless-table idiom on ten pages, was skipped as a separator and the first row of data became a `<thead>` |
| - | **the docs affordances, measured against paseo.sh** - a copy button on every code block, Copy/View as markdown above the title, a visible breadcrumb, a rail that marks where you are, `favicon.ico` and `apple-touch-icon.png`, and the GitHub mark on the site's own links to the repository. Spec: [`docs/specs/2026-09-09-site-affordances-design.md`](docs/specs/2026-09-09-site-affordances-design.md) |
| - | **the content the research asked for** - `/air-gapped`, `/compared` (AWS SSM and Azure Run Command), and a permissions section on `/security`. Also: `heliograph send` on a bundle told people to run `./station.sh --bundle`, which has never existed; it now says the honest thing |
| #49 | **the Azure templates carry a transport**, and CI validates them at all - which found that a sensitive value cannot drive `for_each`, so the Container Apps job had never parsed under the pinned terraform. Also: **no station had ever run under launchd**, because a LaunchAgent's PATH holds only macOS's bash 3.2 |
| #50 | **conformance over every transport**, with a stub relay so it needs no Cloudflare account - and a teeth check per transport, because running the suite three times only proves three passes |
| #51 | **the conformance harness stops being Unix** (Track B/PR 8) - p6's privileged account and p8's cancel move into the driver, p8 proves the cancel by watching the log stop growing, and the redaction corpus lands with a test that every rule is load-bearing |
| #52 | **`caplib.psm1`** (Track B/PR 9) - the capture in PowerShell, passing properties 1-4, 7 and 8, skipping the gates and delivery by name. The step fixtures moved into the driver too, which was the last Unix left in the suite |
| #66 | **the PowerShell transports** (Track B/PR 12) - git and share deliver, and property 9 stops skipping. Teeth for both, because a second implementation of delivery is a second thing that can silently stop delivering. Found: `Import-Module` inside a module function imports into THAT module's session and nowhere else, so the transport loaded, initialised, reported success, and every one of its functions was invisible |
| #54 | **the cancel and the preflight** (Track B/PR 11, part) - a Win32 Job Object with `taskkill /T /F` where `Add-Type` is blocked, which is the estate this station is for. Property 8 stops skipping on Windows. `start.ps1` answers the two questions that decide whether a station can run at all: Constrained Language Mode, and a GPO-set execution policy |
| #53 | **`run.ps1`** (Track B/PR 10) - the runner and its three gates, a `probe.psm1` and a shipped `env` step. Found three case-sensitivity divergences from `run.sh`, two of them in a security gate, and added a twin comparison that would have caught all three |

## Landed 2026-09-11

| PR | |
|---|---|
| #71 | **HTTPS enforced, and `llms.txt` announced** (#70) - a `<link rel=alternate>` in every head and a visible footer anchor. It had been reachable only by an agent that already knew the path |
| #72 | **the rest of #70's code half** - `author` splits from `publisher`, so a named person writes the pages and DBHQ publishes them, with a footer byline saying so; `datePublished` from the first commit beside `dateModified`; and Googlebot and Bingbot are kept off the `.md` mirrors, under their own groups so no other agent is. Also corrected: the "roughly 31 times more bytes" claim, quoted in four files and never measured. It is three to sixteen times, about eight on the median page, and a test now measures it on every build |
| - | **the PowerShell station, finished** (Track B, PRs 13-14 merged into one). `station.ps1` - the loop, gate 3, the receive half of both transports, the bootstrap that plants it, and the documentation. Split no further on purpose: every earlier PR was split so that each piece could be *proved*, and once the loop exists the remaining pieces are provable end to end together. See below for what it found |
| #76 | **content gap 5, and two H2s that are questions** (#70) - `/method` answers "run a command on a remote machine" the way that SERP is written: the `ssh`, `Invoke-Command`, PsExec and cloud-agent answers first, then the case where each has been refused. One question-form H2 each on `/claude-code` and `/mcp`, and nowhere else |

**The MCP registry lists heliograph** as of 2026-09-11, at
`io.github.dbhq-uk/heliograph`. Published by hand once; the release workflow
republishes it from now on, so the advertised version cannot drift from the
package. Two things had to be fixed first, and both fail only at publish time:
the description was 106 characters against a limit of 100, and the `$schema`
was a revision the registry now calls deprecated. Publishing as the
organisation needs the Actions OIDC token - a user token carries
`io.github.<user>/*` only, even with public org membership.

The rest of #70 is off-site and stays on that issue: the social previews on
both repositories, the awesome-list entries, and the Glama listing.

### What that change found

Five defects, three of them in code that had already shipped.

- **`./station.sh --allow-root` has never worked.** It set a shell variable
  that was never exported, so it satisfied the LOOP's root gate and reached
  nothing else. `run.sh` is a separate process with a gate of its own, so every
  step was refused with exit 5 while the published status said only *"the
  runner exited before it reached delivery"* - the symptom, and not one word of
  the cause. `ALLOW_ROOT=1 ./station.sh` worked the whole time, because that
  form is already in the environment. **Fixed** (one `export`), and reproduced
  end to end against the real station before and after
- **The release binary embedded 912 MB of Terraform providers.** `go:embed
  all:bash` reads the WORKING TREE, so `station/bash/azure/*/.terraform`, left
  by `terraform init` - which is what `terraform test` runs - went into the
  binary. Measured here: **239 MB, down to 11.3 MB** once cleared. The
  repository never noticed because those paths are gitignored; CI never noticed
  because a runner starts clean. `internal/bootstrap` prunes `.terraform`, and
  its comment names this exact risk, but it prunes at INSTALL time - by then
  the files are already in the binary. **Fixed** with a guard in
  `station/embed_test.go`, which is where the embed is
- **The same build carried `station/bash/.station-delivery`** - a runtime record
  naming a path under `/tmp` on the machine that built it - and `bootstrap.sh`
  planted it into every repo bootstrapped from that checkout. A station's own
  runtime state is now pruned by name in all three bootstraps and refused by the
  embed guard. `.station-env` is in that list and holds a token.

  **Where it came from**: `tests/test-intercom.sh` drove `intercom.py`, which
  finds its toolkit by walking up from its own file - which in a checkout is
  `station/bash`. So it ran the SHIPPED `run.sh` in place, and `run.sh` wrote
  its delivery record there, which is exactly right on a real station and wrong
  in a checkout. The test now runs against a bootstrapped copy via a new
  `HELIOGRAPH_TOOLKIT` override, read from the environment only - it chooses
  which `run.sh` executes, so a request must never be able to set it.
  `tests/run-tests.sh` now FAILS if any test leaves runtime state in a payload
  directory, which is a different thing from its existing leak counter: that one
  is a hint for diagnosing a flaky suite, this is a defect with a blast radius
- **`Stop-CapTree` would have killed the station.** On Unix it signalled
  `-$pid` unconditionally; a child started by `Process.Start` inherits its
  parent's process group, so the negative pid resolves to the STATION'S OWN
  GROUP. It does not fail - it succeeds at the wrong thing, so the fallback
  never fires. Nothing caught it because the only caller until now was the
  conformance driver, which starts its target under `setsid`. It now measures
  `pgid` first and says how far a cancel would reach

- **Progress never published on Windows, and nothing said so.** `Invoke-CapRun`
  holds the log open through `[System.IO.File]::AppendText`, which opens with
  `FileShare.Read`. That is only half the check: a SECOND open must also declare
  a share mode that tolerates the FIRST handle's access, and the first handle is
  a WRITER - so `File.ReadAllLines` and `Copy-Item`, which both open with
  `FileShare.Read`, throw a sharing violation against a log still being written.
  Nothing enforces any of that on Linux, so it worked perfectly there. The
  loop's read is inside a `try/catch` that returns quietly, because losing a
  race with a live writer is not a reason to stop publishing progress - so on
  Windows a long run was a black box, for every step, silently. **Found by the
  conformance run on a real Windows runner**, not by reasoning. caplib gains
  `Read-CapSharedLines` and `Copy-CapSharedFile`

Three of my own assertions proved nothing and were fixed: a metacharacter check
that passed with the guard removed (a *different* guard caught the case); a
"reason names the variable" check asserted once after a loop, so it only ever
saw the last of four spellings; and an `--allow-root` regression check that read
a status the previous sub-test had left, because `VAR=x` arriving through `"$@"`
is taken as the command name rather than as an assignment.

A fourth was timing rather than measuring: the progress check waited 400
iterations of `sleep 0.1`, which on a Windows runner outlasts the 40-second step
it is watching - so it read a DELIVERED log and called it partial. Every wait in
that file is bounded by the clock now, and the condition requires a `progress:`
key AND the step's own output, because each alone is satisfiable by something
that is not progress.

## Next, in order

1. **The bundle's station side.** `/air-gapped` now says plainly that the
   bundle cannot be read by a station, and the CLI says the same. That page is
   the first thing to update when it lands
2. **The PowerShell relay transport.** Deferred deliberately: it needs
   `heliograph-seal`, which is a Go binary, and a station that must ship a
   binary is a different bootstrap question on exactly the estates that will
   not let you install Git for Windows

## Known defects, recorded rather than fixed

- **Delivery pushes to the configured upstream, not to `origin` explicitly.**
  `cap_push` (bash) and `Send-TpLog` (PowerShell) both use a bare `git push`, so
  a branch tracking another remote takes every log somewhere the control side
  never reads and reports success. The status path on both sides names `origin`
  and the branch explicitly and is not affected. **Both implementations share
  this**, so it must be fixed on both together with a test that watches the
  remote rather than the exit code - fixing one side would make the twins
  disagree about where a log goes, which is the one thing they may not do
- **A push the server completed can be reported as failed** if the
  acknowledgement is lost. Shared by both implementations, same argument
- **Progress publishes on the FIRST in-run poll**, not after `PROGRESS_EVERY`
  seconds: `LAST_PROGRESS=0` in bash and `DateTime.MinValue` in PowerShell both
  compare as "long overdue". Every run lasting more than one `INTERVAL` gets an
  extra status and partial-log publication that the documentation does not
  promise. Harmless, arguably useful, and worth knowing before reading a git
  history and wondering where the extra commit came from
- **A PowerShell station killed with SIGTERM leaves `.station.lock`.** Windows
  PowerShell 5.1 cannot catch the signal, so the `finally` never runs. The next
  station reads the pid, finds it dead, and clears it - which is the designed
  recovery and is tested. `station.sh` traps the signal and does remove it

## Operational notes

- **HTTPS is enforced on the Pages site** as of 2026-09-11. Plain HTTP served
  the whole site with no redirect until then, which for a `curl | chmod`
  install page is worse than untidy. The setting is
  `gh api -X PUT repos/dbhq-uk/heliograph/pages -F https_enforced=true`, and
  it survives a deploy. GitHub adds HSTS with it

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
  depth-1 checkout dates every page today. Every job that builds the site OR
  runs `go test ./...` needs `fetch-depth: 0`: the Pages deploy, the site job,
  and the Go job. The Go job was missed first time and failed on PR #43
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
- **Delivery pushes to the branch's configured upstream, not to `origin`.**
  Both implementations do this - `cap_push` and `Send-TpLog` alike. If a branch
  tracks the same-named branch on a DIFFERENT remote, a bare `git push`
  succeeds, delivery is reported as done, and `origin` - the remote the
  preflight named as the far side - receives nothing. The fix is to push
  `HEAD:refs/heads/<branch>` to `origin` explicitly and to rebase from
  `origin/<branch>`, in BOTH implementations together: fixing one would make
  the twins disagree, which is the one thing they may not do. Found by an
  adversarial read on 2026-09-10
- **A push that the server completed can be reported as a failure.** If the
  connection drops after the ref is updated but before the acknowledgement
  reaches git, both push attempts return non-zero while the far side has the
  commit. The station then says DELIVERY FAILED about a log that arrived. This
  errs in the safe direction - it never claims a success it did not have - and
  the fix is to query the remote ref after a failed push and compare. Both
  implementations
- **The two captures disagree about a bare carriage return.** A progress bar
  writing `step 1\rstep 2\rstep 3\n` is ONE line with embedded `^M` to the
  bash capture, because `read` splits on LF alone, and THREE lines to
  caplib.psm1, because .NET's `ReadLine` treats a lone CR as a terminator.
  .NET's is the better answer - an embedded `^M` in a committed log is the same
  defect the trailing-CR strip exists to remove - so the fix belongs on the bash
  side and changes `cap_run`'s read loop. Found on 2026-09-09 by writing
  property 10, which is also what found that bash was DROPPING the final line
  when it had no newline after it. That one is fixed
- **The ACI templates are not twins.** `aci/main.tf` declares an `ip_address`
  block with TCP 65000 and `aci/main.bicep` omits `ipAddress` entirely, so the
  two produce different resources from the same inputs - network policy and
  audit tooling see an exposed private port only under Terraform. Pre-existing,
  found by an adversarial read on 2026-09-09, and NOT fixed here because which
  of the two is right is a deployment question: the Terraform provider refuses a
  VNet-injected group with no ports (that refusal is documented in azure.md),
  and bicep does not. Deciding needs a deployment, not a diff
- **A value an operator types reaches the VM's cloud-init unescaped.**
  `repoUrl`, `gitToken` and `gitTokenUser` are substituted straight into
  double-quoted shell assignments in a script that runs as root at first boot,
  so a quote or a `$` in one is code rather than data. Pre-existing, and the
  same person chose the value and owns the VM, so it is a robustness problem
  rather than an escalation. The transport's environment block was added
  base64-encoded specifically so as not to widen it
**Fixed on 2026-09-09, and recorded because it was on this list: no station has
ever run under launchd.** `test-launchd.sh` had failed four times on *"launchd
restarted the loop as pid N after a clean exit"*, and every occurrence was
re-run clean by somebody, so nobody read it. On the fourth the test captured
`launchctl print` itself, and the answer was one line: `last exit code = 1`,
with `PATH => /usr/bin:/bin:/usr/sbin:/sbin` above it and *"FAIL bash need 4 or
newer, found 3.2.57"* in the station's log.

macOS ships bash 3.2 at `/bin/bash`. A Mac that runs stations has a newer one
from Homebrew and the operator's PATH finds it, so `service.sh install` passes
every check it makes. A LaunchAgent does not inherit that PATH, so it found 3.2,
the preflight refused it, the station exited 1, and `KeepAlive
{ SuccessfulExit: false }` restarted it - correctly. The plist was blameless and
the assertion was right. The station simply never started, and a poll for *"is a
loop running"* kept finding one because a crash loop always has a pid.

`service.sh` now resolves an absolute bash 4-or-newer at install time,
`pick_bash`, writes it into `ProgramArguments`, and prepends its directory to
the agent's PATH; with none on the machine it refuses to install rather than
leaving a crash loop behind. `test-service.sh` asserts all of that on Linux, and
`test-launchd.sh` now checks the plist's bash and the log for *"Not starting the
station"* before believing a pid.

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

**A test can prove a file is absent while claiming to prove a guard works.**
The transport-name whitelist was checked by passing `../../evil` and
`/etc/passwd`, and every case passed with the whitelist DELETED - because no
module exists at those paths, so the refusal came from the filesystem. The
assertion now demands the whitelist's own diagnostic, and adds a traversal that
RESOLVES TO A REAL MODULE: without the whitelist, that one loads.

**`Import-Module` inside a module function imports into THAT module's session
state.** The transport loader loaded the transport, initialised it, and returned
success - and every function the transport exported was invisible to the caller.
It read as "the transport is fine, and `Send-TpLog` does not exist". `-Global`
is the fix, and the reason it was not obvious is that the failure names the
FUNCTION rather than the import.

**A value type read through a property is a COPY.** `$info.BasicLimitInformation.LimitFlags = 0x2000` set the flag on a copy of the nested struct and threw it away, so the Job Object was created without KILL_ON_JOB_CLOSE and guaranteed nothing. Every call succeeded, the mechanism reported itself in force, the tests were green, and `taskkill` was quietly doing all the work. Assign the nested struct back. And the reason it survived: the only assertion looked for `strategy=`, which an empty value satisfies - so nothing ever asked whether the job existed.

**A test that passes with the thing deleted is worse than no test.** An
assertion here claimed `Test-CapAlive` is not fooled by a zombie, and it passed
with the check removed - PowerShell reaps its own children, so the case cannot
be constructed from this suite. It was deleted rather than left looking like
coverage, and the guard it was written for is marked untested in the module.
Removing an assertion is sometimes the honest move.

**A fixed sleep encodes one implementation's startup time.** The cancel
property waited three seconds and then cancelled, which is ample for bash and
not always enough for PowerShell - two interpreter starts and a module import.
On a loaded machine the cancel landed before a single line was captured, and
the property reported *"the partial log does not survive a cancel"* about a run
that had not produced one. It waits for the run to be demonstrably under way
now. A specification may not assume how fast an implementation starts.

**A BOM makes a file look non-executable to Git-Bash.** The exec bit there is
inferred from a shebang at offset 0, and three invisible bytes move it - so
`run.sh` refused a BOM'd step with *"step file is not executable"* and told a
Windows operator to `chmod +x`, which cannot fix it. The BOM check moved ahead
of the executable test: the BOM is the cause and every other symptom points
somewhere unhelpful. Found because the twin comparison disagreed on Windows and
nowhere else, and because the assertion printed the file's first eight bytes
and its mode instead of just a number.

**"It puts it back afterwards" is not "it changes nothing".** `--check` is what
gets run where nobody is permitted to alter anything yet, and it wrote a probe
file and deleted it - with a FIXED NAME, so a file already at that path was
destroyed. The test missed it by deleting the directory first, which skipped the
whole branch that runs when it exists. Snapshot the tree, not one path.

**Git-Bash converts a path at the exec boundary and nowhere else.** An argument
like `-LogPath /tmp/x` arrives at a native program already converted, so
everything looked fine; a path EMBEDDED IN A SCRIPT gets no such treatment, and
PowerShell read `/tmp/x` as `C:\tmp\x`. The step wrote its marker to a
directory the suite never looked in, and property 5 reported *"exit 0, but the
step never ran"* about a step that had run perfectly. `cygpath -w` where the
path goes into a file rather than onto a command line.

**A loop that refuses at the wrong gate has tested nothing.** The check that
`HELIOGRAPH_ASSUME_PRIVILEGED` cannot OPEN the privileged gate ran an action
with no `CONFIRM`, so gate 2 refused every iteration before gate 4 was reached.
Every value "refused", the assertion passed, and a value that opened gate 4
would have gone unnoticed. Found by an adversarial read on 2026-09-10. When a
test asserts that gate N did something, the input has to reach gate N.

**Two implementations of one rule need a test that compares them, not two
tests.** `run.ps1` and `run.sh` were each tested and each passed, and three
things still meant different things to the two of them - `CONFIRM=YES`,
`# heliograph-mode: READ-ONLY`, and the step name `ENV` - because PowerShell
compares case-insensitively everywhere bash does not. Two of those were security
gates: a state-changing step ran on one and was refused by the other, from the
same request. The guard that holds is running the SAME declaration through both
and comparing the exit codes.

**A corpus is only testing the rules it is the ONLY thing catching.** The
redaction corpus was written case by case, each one realistic - a GitLab token
in a clone URL, a Bearer token behind an `Authorization:` header - and every one
of those is caught by a *different, broader* rule. Four rules could be deleted
outright with the corpus still reporting a clean run. Found by deleting them,
one at a time, which is now `test-redaction-corpus.sh` and runs every time. The
mutation itself was wrong twice first: deleting the last `-e` line broke the
line continuation, and `awk -v` ate a trailing backslash - and in both cases a
`cap_redact` that no longer existed leaked nothing, which reads exactly like a
rule the corpus caught. **A mutation test needs a liveness check or its passes
are silence.**

**An exit code is not evidence that anything ran.** The conformance suite's two
gate properties were asserted by exit status alone, so a runner that returned 0
without executing the step satisfied *"a declared step runs"*, and one that ran
the step and THEN refused with 5 satisfied *"nothing runs as root"* - which is
the whole defect wearing the right exit code. Both now write a marker. Three
distinct strings also satisfied *"three distinct timestamps"*; the column is now
checked for being a clock, and for being UTC, by capturing under `TZ` fourteen
hours away.

**A test double more permissive than the real thing is worse than none.** The
relay stub was written with one token; the relay's scopes are asymmetric, so a
station using the control credential would have passed here and been refused in
an estate. There are two doubles of that relay in this repository - this one and
the Go one the CLI tests use - which is two chances to drift, so the rules are
now asserted against the stub behaviourally rather than assumed from having
written it.

**A re-run that goes green is a diagnosis nobody made.** The launchd assertion
failed four times and was re-run clean four times, and the fourth failure - the
first to print `launchctl print` - said in one line that no station had ever
started on a Mac. An intermittent failure is a race between a real bug and a
poll, not an absence of one. Make a test carry its own evidence BEFORE it fails
again, because the evidence that settles it is usually the kind the next run
destroys.

**A process is not a service.** *"the loop is running as pid N"* was true
throughout a crash loop, because launchd kept making new ones. Assert on what
the thing was installed to DO - it got past preflight, it published a status -
never on the existence of a pid.

**A template nobody has validated is a template nobody knows parses.** The very
first CI run of `terraform validate` over `station/bash/azure`, added on
2026-09-09, failed on code that predated it: a SENSITIVE value cannot drive
`for_each`, because a `for_each` key becomes part of a resource address and a
secret may not go there. `var.gitToken` is sensitive, so
`for_each = var.gitToken == "" ? [] : [1]` is sensitive too, and the Container
Apps job had never parsed under the pinned terraform. It had been DEPLOYED -
just with a newer terraform than CI pins, which is why nothing noticed. Unwrap
only what is genuinely not secret: the EMPTINESS of a token, or the NAMES of a
secret map, never the values.

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

**`go:embed` reads the working tree, and .gitignore does not stop it.**
`terraform init` in the Azure templates drops 200MB of provider binaries per
template, and `bootstrap.sh` has pruned `.terraform` since it was written. The Go
planter did not - so a release built on any machine where somebody had run
terraform would have carried those binaries inside the `heliograph` binary,
permanently, in every download. CI never saw it because a CI runner starts
clean. Found by running `terraform test` locally, which is the thing the
templates needed and nothing had ever done.

**One file format, two implementations, is six disagreements.** The
`.station-env` rules were written in bash for `service.sh` and again in
PowerShell for `service.ps1`, and an adversarial read found six ways they
classified the same file differently - PowerShell regexes are case-insensitive
by default, `Get-Content` eats a UTF-8 BOM that bash does not skip when
sourcing, an empty file passed one and failed the other. A station that installs
on Windows and is refused on Linux, from one file, is worse than either answer
alone. `station-env.sh` is now the only implementation and both installers call
it.

**Ask the transport, do not keep a list of what it needs.** Scraping `cap_need`
names out of a transport looked mechanical and was a floor: Azure Blob needs a
SAS *or* a managed identity, which no `cap_need` line expresses, so a file with
the two names it does declare passed the installer and was refused by
`start.sh`. Running the transport's own `tp_init` - local by contract, no
network - gets every case right and stays right when a transport changes.

**Stripping a CR on read is not the same as stripping it on source.** The
validator read `.station-env` with the CR removed and accepted a CRLF file;
nothing strips it when a service SOURCES that file, so `SHARE_SCOPE` became
`probe` plus a carriage return and the transport refused it after installation.
The test asserting CRLF was accepted is what found it.

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
go run ./cmd/heliograph-site site/content /tmp/site                      # 26 pages
```

macOS is absent locally, so the launchd suite skips. **CI runs it and CI has
caught real defects that skip hid** - do not read a local green as complete.

**Docker may only need starting.** `sudo systemctl start docker` was all it
took, and it turns the container and Kubernetes suites from skipped into 271
assertions that run in about ten minutes - including a whole station run in a
container. They found four defects in one afternoon that CI would have taken
four pushes to surface one at a time. Try it before pushing anything that
touches `station/bash/docker/`.
