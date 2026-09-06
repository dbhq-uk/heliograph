#!/usr/bin/env bash
# =============================================================================
#  transports/git.sh - the git transport, under the station's transport contract
# =============================================================================
# Sourced by station.sh. Implements tp_* and nothing else. Every git command
# the loop issues lives in this file, so the loop itself no longer knows what
# it is talking to.
#
# THE CONTRACT
#
#   tp_capabilities            which optional verbs this transport offers
#   tp_check                   can this station reach the transport at all
#   tp_describe                the credential in force, by mechanism, never value
#   tp_fetch_request           emit the request document, or fail
#   tp_fetch_request_live      the same, DURING a run, without touching the tree
#   tp_fetch_self              bring a newer payload, if this transport can
#   tp_put_status              publish a status document, plus an optional file
#   tp_put_progress            publish a partial-log snapshot
#
# WHY tp_fetch_request_live IS SEPARATE
#
# During a run the step is appending to its log through an open descriptor.
# Reading the request by pulling would rewrite the working tree underneath it
# and the appends would carry on at a stale offset, corrupting the evidence the
# run exists to produce. So the live read goes to the remote ref and never
# touches the tree. Two verbs rather than a flag, because the difference is not
# a detail: getting it wrong destroys a run while only trying to observe it.
#
# WHY tp_fetch_self EXISTS AT ALL
#
# The loop re-executes itself when a newer station.sh arrives, which is what
# lets a fix take effect without telling the operator to restart - the whole
# point of them starting it once and walking away. git gives that away free,
# because a pull replaces the file. Nothing else does, so it is a declared
# capability rather than an assumption.
# =============================================================================

# What this transport can do. A loop reads this at START, so it can say what it
# will not be able to do later, while somebody is still listening.
tp_capabilities() { printf 'request status progress self live\n'; }

tp_check() {
  git rev-parse --git-dir >/dev/null 2>&1 || {
    echo "not a git repository" >&2
    return 1
  }
  cap_git ls-remote --exit-code origin "refs/heads/$BRANCH" >/dev/null 2>&1
}

tp_describe() {
  local url
  url="$(git remote get-url origin 2>/dev/null)"
  printf 'git %s on %s' "${url:-<no origin>}" "$BRANCH"
}

# The ordinary poll: bring the branch up to date and read the request from the
# working tree. Returns 1 on a fetch failure, which the loop treats as a blip.
tp_fetch_request() {
  cap_git fetch --quiet origin "$BRANCH" 2>/dev/null || return 1
  cat "$REQUEST" 2>/dev/null
}

# The live read, used while a step is running. Never pulls, never rebases,
# never touches the working tree - see the header.
tp_fetch_request_live() {
  cap_git fetch --quiet origin "$BRANCH" 2>/dev/null || return 1
  git show "origin/$BRANCH:$REQUEST" 2>/dev/null
}

# Bring a newer payload into the working tree.
#
# Exits 0 when something changed, 1 when nothing did, 2 when it could not be
# done. The loop uses that to decide whether to re-execute, and a 2 is not
# fatal: a station that cannot update itself is still a working station.
tp_fetch_self() {
  local before after
  before="$(git rev-parse HEAD 2>/dev/null)"
  after="$(git rev-parse "origin/$BRANCH" 2>/dev/null)"
  [ "$before" = "$after" ] && return 1

  if cap_git pull --rebase --quiet 2>/dev/null; then
    return 0
  fi
  # Bare git: abort touches no network, so cap_git would put the auth header
  # into this process's argv for nothing.
  git rebase --abort >/dev/null 2>&1
  return 2
}

# Publish a status document, and optionally a file alongside it.
#
# The extra file is not decoration. A killed step leaves its log modified in the
# working tree, and progress pushes have made that file TRACKED, so unless it is
# committed here every later `pull --rebase` refuses on a dirty tree and the
# station wedges with the cancellation never reaching the far side. Found by
# cancelling a run that had been publishing progress.
tp_put_status() {
  local body="$1" msg="$2" alsofile="${3:-}"
  mkdir -p "$(dirname "$STATUS")"
  [ -n "$alsofile" ] && [ -f "$alsofile" ] || alsofile=""
  printf '%s' "$body" > "$STATUS"

  git add -f "$STATUS" ${alsofile:+"$alsofile"} >/dev/null 2>&1
  git diff --cached --quiet -- "$STATUS" ${alsofile:+"$alsofile"} >/dev/null 2>&1 && return 0
  git -c user.name="${GIT_AUTHOR_NAME:-station}" \
      -c user.email="${GIT_AUTHOR_EMAIL:-station@$(hostname)}" \
      commit -q -m "$msg" -- "$STATUS" ${alsofile:+"$alsofile"} 2>/dev/null
  cap_git pull --rebase --quiet >/dev/null 2>&1
  cap_git push --quiet >/dev/null 2>&1 || cap_git push --quiet -u origin HEAD >/dev/null 2>&1 || return 1
  return 0
}

# Publish a snapshot of a running step's log.
#
# PUSHES BUT NEVER PULLS OR REBASES, deliberately. The step is appending to that
# log through an open descriptor; a rebase would rewrite the file underneath it
# and the appends would continue at a stale offset. A rejected push is simply
# retried next cycle, and run.sh's own cap_push reconciles properly at the end.
tp_put_progress() {
  local body="$1" msg="$2" logfile="$3"
  mkdir -p "$(dirname "$STATUS")"
  printf '%s' "$body" > "$STATUS"

  git add -f "$STATUS" "$logfile" >/dev/null 2>&1
  git diff --cached --quiet -- "$STATUS" "$logfile" >/dev/null 2>&1 && return 0
  git -c user.name="${GIT_AUTHOR_NAME:-station}" \
      -c user.email="${GIT_AUTHOR_EMAIL:-station@$(hostname)}" \
      commit -q -m "$msg" -- "$STATUS" "$logfile" 2>/dev/null
  cap_git push --quiet >/dev/null 2>&1 || return 1
  return 0
}
