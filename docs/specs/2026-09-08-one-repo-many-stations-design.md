# One repository, many stations

**Date:** 2026-09-08
**Status:** draft, for review

A transport repo can already talk to several stations, one per branch. Nothing
designed that, nothing documents it, and nothing stops you getting it wrong.
This makes it explicit, makes it safe, and states the one condition under which
it is not the right answer.

## It already works, and that is the problem

Neither side is configured with a branch. Both derive it from whatever happens
to be checked out.

| | |
|---|---|
| station | `transports/git.sh` `tp_init` reads `git rev-parse --abbrev-ref HEAD` into `BRANCH`, then reads `origin/$BRANCH:station/request` |
| control | `transport.NewGit(dir)` reads the checkout's branch and reads `origin/<branch>:station/status` |

`estate.Estate.Branch` is even commented *"recorded for reporting; the checkout
decides"*, and that is exactly what happens: `open()` calls `NewGit(e.Dir)`,
which re-derives the branch and **ignores the stored value**.

So the branch is the channel today. It is a property of the checkout rather
than a decision anybody recorded, which is fine for one station and three
distinct hazards for several.

### Hazard 1: the checkout is the only routing control

There is no `--branch` on any CLI command and no per-estate pin. A stray
`git checkout` in the estate directory silently retargets which machine the
next `heliograph send` reaches. The stored `Branch` would have caught it and is
not consulted.

Sending a request to the wrong estate runs a command on the wrong machine, and
that is not recoverable by apologising. The CLI already refuses to guess
between several estates for this reason. It should refuse to guess here too.

### Hazard 2: nothing prevents two stations on one branch

`.station.lock` is per checkout. It protects one machine from starting itself
twice and does nothing at all across machines. `station.sh`'s own comment says
two stations "would double-run every request and race on push", which is
precisely what happens, silently, with no lock in play.

### Hazard 3: the station pushes implicitly, and the control side does not

The control side is explicit: `push --quiet origin HEAD:<branch>`
(`internal/transport/git.go`). The station is not: `cap_git push --quiet`, and
on failure `-u origin HEAD` (`transports/git.sh`).

A bare push depends on upstream configuration. Once the branch means *which
machine this is*, that is too implicit to leave alone.

## The rule that decides the shape

**Branches are routing isolation, not security isolation.**

A repository credential usually grants read and write across every branch. So
a compromised station can read every other station's logs, and can push a
modified `station.sh` to another station's branch. Pinning explicitly does not
cover `station.sh`, so the victim removes its own gates at its next
self-update.

That gives the condition, and it is a hard one:

> A repository may hold several stations only if all of them share one
> confidentiality and compromise blast radius. Across a trust boundary,
> separate repositories are not administrative untidiness. They are the
> isolation mechanism.

## The shape: one branch per station

`station/<name>`, one branch per machine, one checkout per branch.

Considered and rejected:

**A directory per station on one branch.** Every status and log push makes
every other station's checkout stale, and progress publishing is deliberately
push-only and never rebases, so it degrades worst. It spends the strongest
property the git transport has - that two stations on two branches never
conflict - to save creating a branch.

**Worktrees as the channel.** A worktree is not the channel; the branch is, and
conflating the two would make the model depend on how somebody arranged their
disk.

**Corrected 2026-09-08.** The first draft filed worktrees as far-side storage
and that had it backwards. **The control side is where they are needed.**
Talking to three stations means three checkouts, because both sides read the
branch from the checkout - and switching one checkout between branches is
exactly the accident hazard 1 describes. A worktree gives one clone, one fetch
and one credential with a directory per station, and git's refusal to check one
branch out in two worktrees is a useful refusal rather than a limitation: it is
two stations on one branch, caught locally.

**A `station:` field in the request.** Routing belongs to the branch. A station
old enough to ignore the field would run the request anyway, so the field would
look like protection while providing none. That is worse than not having it.

## What changes

Small, and mostly hardening.

### Control side

- **`Estate.Scope` becomes the canonical routing field** for every transport,
  with git's scope being its branch. Records with an empty `Scope` fall back to
  `Branch`, so estate files in the field keep working
- **`heliograph station add <name>`** creates a station and is the answer to a
  question the first draft left open. Nothing in `internal/transport` ran
  `checkout`, `switch` or `branch`, and `bootstrap` runs no git at all - so
  `init --branch` would have *recorded* a branch somebody else had to create by
  hand. `station add` does the three things that must happen together, because
  doing two is worse than doing none: create the branch and push it **with an
  upstream**, check it out into its own worktree, and record the estate. Then it
  prints what to send the operator
- **`heliograph init <name> --branch station/db-a`** records the binding for a
  branch that already exists
- **`open()` verifies the checkout is still on the configured branch** and
  fails closed if it is not, naming both. This is the fix for hazard 1
- **`heliograph estates`** prints the repository and the scope, so which
  machine a name reaches is visible without inspecting a checkout

`heliograph send` gains no `--station` flag. An estate already *is* one
destination, and the resolver already refuses to guess when several are
configured.

### Station side

- **`transports/git.sh` pushes explicitly**, naming `origin` and the expected
  branch, in `tp_put_status`, `tp_put_progress` and `tp_put_log`
- **`start.sh --branch` verifies the result.** It already checks the branch out;
  it should confirm the resulting branch is exactly what was asked for, and
  `--check --branch x` should validate the remote branch rather than only
  inspecting the current one
- **status gains a payload digest**, because `HEAD` advances on every status and
  log commit and therefore says nothing about which payload is running. Branches
  carry independent copies of the station and self-update pulls only its own,
  so drift is real and should be visible rather than discovered

The request document does not change. `station/request`, `station/status` and
`ops-logs/` stay fixed: the branch already supplies the namespace.

### What is deliberately not built

- **Any cross-machine lock.** A distributed lease protocol with lock commits and
  timestamps is a fragile distributed system built to avoid creating a branch.
  Two stations on one branch is a configuration error, and the honest response
  is to document it and make the status say which host answered - which it
  already does
- **Automatic merges from `main` on the far side.** Autonomous code changes on
  the station, from a second ref, with non-atomic payload and channel state
- **`station/*` enforced globally.** Deployments in the field use `task/*` and
  arbitrary names and cannot be reached to be upgraded. It is a convention for
  new repositories

## Compatibility

Nothing in the field breaks. Existing branches remain their own scope, the
fixed paths do not move, the request document is unchanged, the pre-rename
`agent/request` shim stays branch-local, and old estate files load through the
`Branch` fallback. No new station-side argument is required to keep working.

## What this does not solve

It does not make one repository safe for stations in different trust domains,
and no amount of branch discipline will. The blast-radius rule above is the
answer to that, and it is a rule rather than a feature.

It also does not make two stations on one branch impossible. It makes it
documented, and it makes the status say which host answered.
