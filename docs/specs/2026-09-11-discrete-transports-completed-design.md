# Discrete transports, completed

**Date:** 2026-09-11
**Status:** draft, for review
**Part of:** [the signalling toolkit](../plans/2026-09-11-signalling-toolkit-roadmap.md) (S2)

Two of the three shapes are discrete - they carry a request and hand back a
log, one exchange at a time - and both already exist in part. The **beacon**
over git is driven end to end. The **beacon** over the relay works but the CLI
cannot select it. The **flare** has a complete far side and a bash control side
(`flare.sh`, formerly `intercom.sh`) but no Go transport at all. This spec
finishes the discrete half: it makes the relay selectable and gives the flare a
control-side `Transport`, so `send`, `watch` and `logs` work over all three
discrete carriers, and so S3's shell has something to sit on besides git.

It writes no live channel. The beam is S4.

## Where the interface stands

`internal/transport/transport.go` is the whole contract a control side has:
`FetchStatus`, `PutRequest`, `ListLogs`, `ReadLog`, `Check`, `Describe`. git,
relay, share and objstore implement it; `internal/estate` decides which one a
name resolves to. The interface does not change in this spec - the work is to
bring two carriers up to it.

## Part 1: the relay, selectable

The relay `Transport` is written and proven against the deployed relay
(`PLAN.md`), but no estate configuration routes to it. This part is wiring, not
new transport code.

- **`internal/estate`** learns the relay: an estate can be created and resolved
  as a relay estate, carrying the base URL, the estate and station ids, the
  token and the two key identities the relay transport already needs.
- **`heliograph init <name> --transport relay ...`** (or the project's existing
  init surface) writes that configuration, the same way a git estate is written
  today.
- **`heliograph doctor`** runs the relay's `Check` for a relay estate, so "will
  this work from here" is answered before a run.
- No new credential handling: the relay transport already owns its token and
  identities. This part only lets a name reach it.

## Part 2: the flare transport

A new `Transport` implementation, `internal/transport/flare.go`, speaking the
two-endpoint HTTPS contract the Function already serves and `flare.sh`
documents: `POST /api/run` and `GET /api/task/{taskId}`.

### The one impedance, and how it is resolved

A beacon request only *names* a step, because the far side already holds the
code. A flare **ships the code** - there is no git on the far side to have
planted it. But `PutRequest` takes a `wire.Request`, which carries `Step` (a
name or a path) and `Env`, and no script body.

Resolution, without touching the interface: **the flare transport treats
`Request.Step` as a path and reads the script itself.** It has filesystem
access; it opens the file, and ships its contents. A `Step` that is a bare
registered name with no file behind it is refused with a message that says
what flare needs - a path, because it carries the script rather than naming a
planted one. This is exactly how `flare.sh` already behaves (`flare.sh run
steps/net-probe.sh`), and it is what lets S3's shell hand the flare an ephemeral
step file with no new plumbing.

### The method mapping

| interface method | over flare |
|---|---|
| `PutRequest(req)` | read the file at `req.Step`, `POST /api/run` with `{name, script, env: req.Env, wait}`; remember the returned `taskId` against `req.ID` so the later reads can find it |
| `FetchStatus()` | the flare has no standing status document like a beacon's `station/status`; report the state of the run just submitted (its task), mapped to `wire.Status`, plus endpoint liveness |
| `ReadLog(name)` | `name` is a `taskId`; page `GET /api/task/{taskId}?offset=` to the end, the way `flare.sh` walks it, never truncating |
| `ListLogs()` | newest task ids first (see the list route below) |
| `Check()` | a cheap reachable-and-authorised probe that runs no step - a list with limit zero, or a dedicated health route - so the key and the IP allowlist are proved before a capture depends on them |
| `Describe()` | "function key (NN chars)", never the value |

`name`, `env` and `wait` are validated on the control side to the same rules
the far side enforces (`NAME_RE`, `ENV_KEY_RE`, the wait clamp), so a bad
request is a local error rather than a round trip - the pattern `flare.sh`
already follows.

### The `ListLogs` gap

The Function exposes `GET /api/task/{id}` but no way to list tasks, so history
cannot be enumerated over a flare. Two parts, and only the first is required:

1. **Required (control side).** `ListLogs` returns the task ids this process
   has issued, newest first, held in memory. Enough for `watch` and for S3's
   shell, which only ever reads the task it just submitted.
2. **In scope, small (far side).** Add `GET /api/tasks?limit=N` to the Function,
   listing recent tasks newest first from the blobs already under `tasks/`.
   `ListLogs` uses it when present and falls back to session memory when not.
   This is the sole far-side change in S2, and it is a few lines against the
   store that already lists blobs.

### Estate configuration

A flare estate carries the base URL and the function key, the way `flare.sh`
reads `FLARE_URL` and `FLARE_KEY` (renamed from `INTERCOM_*` per S1, old names
honoured). `heliograph init <name> --transport flare --url ... ` writes it; the
key is read from the environment or a credential file, never a flag, so it does
not land in shell history - the same care `flare.sh` takes sending it as a
header rather than `?code=`.

## What this unblocks

- `send`, `watch`, `logs` and `doctor` work over the relay and the flare, not
  only git.
- S3's `heliograph shell` has a low-latency, script-with-request carrier (the
  flare) to sit on, which is where an interactive shell is actually usable.
- The flare stops being a bash-only side path and becomes a first-class
  transport, so the MCP server and every CLI verb reach it for free.

## Non-goals

- No live channel, no beam, no held-open connection. S4.
- No shell. S3.
- No queue-mode work on the Function; inline stays the default, as the flare
  design already argues.

## Done when

- A relay estate can be created and driven end to end in CI, the way a git
  estate is, reusing the existing relay round-trip harness.
- A flare estate runs a step from a local path and reads its log back, proved
  against the reference Function (or the existing flare test double).
- `PutRequest` refuses a bare step name over a flare, with the message that
  says to give a path.
- `ListLogs` over a flare returns this session's tasks, and the full history
  when `/api/tasks` is present.
- Old `INTERCOM_*` names still resolve a flare estate, with the S1 warning.
