# Merging heliograph-skill into this repository

Date: 2026-09-08. Status: being implemented in the same PR that carries this
spec.

## The problem

Two repositories both present themselves as "heliograph", and the boundary
between them does not hold. This repo's own README says the skill is on the
near side ("control: the CLI, the skill family, you") and, twenty lines later,
that `heliograph-skill` is the far side. The skill repo in fact holds both
halves: `SKILL.md` runs in an agent on the control node, while `toolkit/` is
the payload that runs on the far side. The stated boundary was the gap; the
real boundary was Go versus bash. Every reader had to work out which half they
were looking at, and the two repos' own tests could only check each half
against its own idea of the shared contract.

## The premise

Decided by Dan, 2026-09-08: **skills exist to drive the CLI, and pure bash and
PowerShell station implementations exist for the far side.** The bash-purity
constraint was always load-bearing on the far side only - the control node was
never the machine where installing something is a change request.

## The shape

One repository, `dbhq-uk/heliograph`. The boundary is the gap, stated once by
the layout:

```
cmd/heliograph/         the control CLI, and `heliograph mcp`
internal/...            control-side packages, as now
station/bash/           the bash station: what `bootstrap` copies into a
                        transport repo and what runs on the far side
                        (was heliograph-skill's skills/heliograph/toolkit/)
station/bootstrap.sh    the no-CLI path: copies station/bash into a transport
                        repo by hand (was skills/heliograph/scripts/bootstrap.sh)
station/embed.go        go:embed of station/bash, for `heliograph bootstrap`
station/powershell/     a pure PowerShell station. Not built yet - its own
                        later piece of work, and it does not gate this merge
skills/heliograph/      the agent skill: SKILL.md and references/. Drives the
                        CLI and nothing else
tests/                  the station's bash test suite, moved across unchanged
                        but repointed at station/bash
site/ docs/ infra/      as now
```

Everything under `station/bash/` (and later `station/powershell/`) runs on the
far side and depends on nothing: bash 4+, git, GNU coreutils, no packages, no
credentials of its own. Everything else is control-side and may depend on
whatever it argues for. `station/embed.go` is the one Go file under `station/`
and it never ships to the far side - it exists so the release binary carries
the station payload it was built against.

CI enforces the boundary: the only `.go` file permitted under `station/` is
`station/embed.go`.

## What is new in the CLI

`heliograph bootstrap <dir>` writes the embedded station payload into a
transport repo, with exactly `bootstrap.sh`'s semantics: nothing overwritten
silently, an existing file left alone and reported, `gitignore` and
`gitattributes` restored to their dotted names on the way out. This is the
keystone of "skills drive the CLI": without it the skill still needs a
checkout of this repo to set an investigation up, which is the two-path mess
the merge exists to end. It also pins versions honestly - a binary plants the
station it was built with, so "which station is this estate running" has the
same answer as "which binary planted it".

`station/bootstrap.sh` remains the path for someone with no CLI and no agent:
clone this repo, run it by hand. The station being the station.

## What changes in the skill

`skills/heliograph/SKILL.md` is rewritten as a CLI driver. It requires the
`heliograph` binary; when the binary is missing it prints the exact install
command and stops. The gates - read-only default, `CONFIRM=yes`,
`--allow-actions` - live in the CLI and the station only, so there is one
driver and the security-relevant enforcement cannot drift between two.

Where a transport exists only in the station today (the Azure Blob pigeonhole,
the Function App intercom), the skill drives the station's own control-side
script in the transport repo - `drop.sh`, `intercom.sh` - because the CLI has
no equivalent yet. That is coverage of a gap, not a second driver of the same
path, and each graduates into the CLI as a transport on its own merits.

`references/` stays with the skill unchanged: the method and the station
reference ship with the payload's documentation, as before.

## What happens to heliograph-skill

- Histories merged with `--allow-unrelated-histories`, so blame and the commit
  trail survive.
- The marketplace entry (`dbhq-uk/marketplace`) repoints at this repo. The
  plugin manifest (`.claude-plugin/plugin.json`) and `skills/` stay at the
  repo root, so `/plugin install heliograph@dbhq` and
  `npx skills add dbhq-uk/heliograph` both resolve without special-casing.
- `dbhq-uk/heliograph-skill` gets a pointer README and is archived. Existing
  installs break once, with a README that names the exact new install command.
- The container image stays `ghcr.io/dbhq-uk/heliograph-toolkit`, now built
  from `station/bash/docker/Dockerfile`. Estates that recorded an image tag in
  a change record keep a name that still resolves.

## What this does not change

- **heliograph-relay stays its own repository.** Its boundary is load-bearing
  in a way the skill's was not: the repo must be publicly, obviously incapable
  of reading what it carries.
- Every constraint in AGENTS.md: timestamps, failed runs still ship, one
  capture implementation, gates fail closed, never truncate, the account is
  the blast radius. The merge moves files; it does not renegotiate any of
  them.
- The station's own CI - conformance suite, mutant check, Windows, macOS,
  Kubernetes, container - moves across intact as `station.yml`, repointed at
  the new paths.
- The cross-repo e2e and coherence tests stop being cross-repo: they resolve
  the station in this repo by default (`HELIOGRAPH_SKILL_DIR` still overrides,
  for testing against an external checkout), and the CI steps that checked out
  a second repository go away.

## Sequencing

1. Merge histories; move `toolkit/` to `station/bash/`, `bootstrap.sh` to
   `station/`, the skill's tests to `tests/`; resolve README/AGENTS/workflow
   conflicts; repoint every path.
2. `station/embed.go` and `heliograph bootstrap`, with tests.
3. Rewrite `SKILL.md` as a CLI driver.
4. One README, control/transport/station, single H1.
5. Site, marketplace, org profile, dbhq.uk updated; heliograph-skill archived
   with a pointer.
6. PowerShell station: later, its own spec.
