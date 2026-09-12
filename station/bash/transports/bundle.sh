#!/usr/bin/env bash
# =============================================================================
#  transports/bundle.sh - the gap nothing crosses but a person
# =============================================================================
# Sourced by station.sh. A request is exported to a file, carried by hand, and
# the reply is carried back the same way.
#
# THIS IS THE ONLY TRANSPORT THAT MAKES "AIR-GAPPED" LITERALLY TRUE. Every other
# one here still needs some path between the two machines, even a storage
# account nobody can route to directly. This needs none: a USB stick, a burned
# disc, a file handed over at a desk.
#
# THE CONTROL SIDE HAS EXISTED SINCE A3 AND THIS DID NOT, which is the same hole
# the file share had: `heliograph init --transport bundle` wrote requests
# nothing on the far side could read, and the transports page said so in a
# column while /air-gapped had to admit that the one transport named for the
# case did not work.
#
# THE LAYOUT IS THE CONTROL SIDE'S, not this file's invention.
# internal/transport/share.go's Bundle decides it, and this reads and writes
# exactly those paths:
#
#   <BUNDLE_DIR>/request-<id>.hgb   the control side writes it, a person carries it
#   <BUNDLE_DIR>/status             this side writes it, a person carries it back
#   <BUNDLE_DIR>/<name>.txt         the logs, FLAT, beside the status
#
# THE LOGS ARE NOT IN ops-logs/, AND THAT IS THE ONE THING TO GET RIGHT HERE.
# Every other transport puts them in a subdirectory; the control side's
# Bundle.ListLogs reads `*.txt` in the bundle directory ITSELF, so a station
# writing them one level down produces a bundle that carries back perfectly and
# shows no logs at all. That is the failure this transport is least able to
# recover from - the courier has already walked.
#
# IT IS NOT A LOOP AND DOES NOT PRETEND TO BE. A run takes as long as it takes
# somebody to walk. The value is that the request format, the four gates and the
# captured log are identical to every other transport: the method survives the
# walk, and the station does not have to be told it is on a stick.
# =============================================================================

# NO `live`, AND NO `self`.
#
# `live` is a mid-run re-read so that a cancel reaches a running step. There is
# nothing to re-read: the request arrived on a stick and the stick is not going
# to change while the step runs. Declaring it would make the loop poll a file
# that cannot move, once per interval, for ever.
#
# `self` is a payload update, and nothing publishes a payload to a bundle -
# `heliograph plant` refuses every non-git transport with "nothing for the far
# side to clone". A station on this transport is changed by re-planting, which
# on an air-gapped machine means another walk.
#
# Declaring a verb this cannot honour is worse than not having it: the loop
# calls what is declared.
tp_capabilities() { printf 'request status progress\n'; }

BUNDLE_TMP_PREFIX=".heliograph-tmp."

tp_init() {
  # CLEARED FIRST, for the reason share.sh clears its own: tp_preflight is
  # contracted to run after a FAILED tp_init, so a value inherited from the
  # environment would be read as though this function had resolved it.
  BUNDLE_BASE=""

  cap_need BUNDLE_DIR "the directory the bundles are carried in - a mount point for the stick, usually" || return 1

  [ -d "$BUNDLE_DIR" ] || {
    echo "station: the bundle directory $BUNDLE_DIR is not there." >&2
    echo "         Mount the stick, or point BUNDLE_DIR at where it is mounted" >&2
    echo "         on THIS machine. Nothing is created above it, deliberately:" >&2
    echo "         creating a missing mount point turns 'the stick is not in'" >&2
    echo "         into a station writing replies onto local disk that nobody" >&2
    echo "         will ever carry anywhere." >&2
    return 1
  }

  # A SYMLINK IS REFUSED, for the reason the share refuses one and one more.
  #
  # The share's reason: anybody who can write here can point it at a
  # station-local directory and take the replies somewhere nobody looks.
  #
  # The one particular to this transport: the whole point is that a person
  # picks the medium up and walks away with it. A symlink into local disk
  # produces a station that works perfectly, a stick that is carried back
  # empty, and no error anywhere.
  if [ -L "$BUNDLE_DIR" ]; then
    echo "station: $BUNDLE_DIR is a symlink." >&2
    echo "         Refused: the replies have to be ON the medium somebody" >&2
    echo "         carries, and a link points somewhere that will not travel." >&2
    return 1
  fi

  BUNDLE_BASE="$BUNDLE_DIR"

  # Litter from a station killed between writing a temporary and renaming it.
  # An hour is far longer than any publish takes and far shorter than anybody
  # would keep one - and this is a directory a human browses, on a stick.
  find "$BUNDLE_BASE" -maxdepth 1 -name "$BUNDLE_TMP_PREFIX*" -mmin +60 \
    -exec rm -f {} + 2>/dev/null || true
  return 0
}

# THE SCOPE IS THE DIRECTORY, because a bundle has no branch, no lane and no
# estate id. The control side's Bundle.Branch() answers the literal string
# "bundle", and the published status has to match what it expects to read back.
tp_scope() { printf 'bundle'; }

tp_revision() { printf 'bundle in %s' "$BUNDLE_DIR"; }

tp_describe() {
  printf 'bundle in %s, carried by hand - no credential, and no network at all' "$BUNDLE_DIR"
}

# --- writing without ever being read half-written ----------------------------
# Write-then-rename, into the same directory, for the same reason the share
# does it: a half-copied log ends mid-line with no footer, which is the one
# shape an operator is trained to read as "still running".
#
# It matters MORE here. On a share a torn read is a moment; on a stick, the
# thing somebody unplugs mid-write is what they carry back, and there is no
# second chance until the next walk.
_bundle_tmp() {  # _bundle_tmp <destination>
  local dst="$1" dir
  dir="$(dirname -- "$dst")"
  mkdir -p "$dir" 2>/dev/null || return 1
  mktemp "$dir/$BUNDLE_TMP_PREFIX$(basename -- "$dst").XXXXXX" 2>/dev/null
}

_bundle_rename() {  # _bundle_rename <tmp> <destination>
  local tmp="$1" dst="$2"
  if [ -d "$dst" ]; then
    say "$dst is a directory, so publishing there would put the file INSIDE it."
    rm -f -- "$tmp"
    return 1
  fi
  # 0644 because the control side may well be a different user, and a reply
  # nobody can read is a reply nobody has. mktemp creates 600.
  chmod 0644 -- "$tmp" 2>/dev/null
  mv -f -- "$tmp" "$dst" 2>/dev/null || { rm -f -- "$tmp"; return 1; }
  return 0
}

_bundle_publish_body() {  # _bundle_publish_body <text> <destination>
  local body="$1" dst="$2" tmp
  tmp="$(_bundle_tmp "$dst")" || return 1
  [ -n "$tmp" ] || return 1
  printf '%s' "$body" > "$tmp" 2>/dev/null || { rm -f -- "$tmp"; return 1; }
  _bundle_rename "$tmp" "$dst"
}

_bundle_publish_file() {  # _bundle_publish_file <source> <destination>
  local src="$1" dst="$2" tmp
  tmp="$(_bundle_tmp "$dst")" || return 1
  [ -n "$tmp" ] || return 1
  cp -- "$src" "$tmp" 2>/dev/null || { rm -f -- "$tmp"; return 1; }
  _bundle_rename "$tmp" "$dst"
}

# PROVE THE WRITE, AND PROVE ALL THREE PARTS OF IT.
#
# A stick mounted read-only is the ordinary failure here, and it stats
# perfectly. So does one that is full, and one whose filesystem the kernel has
# remounted read-only after an error - which is what a failing stick does.
#
# Create, rename, then remove, because publishing needs all three: a medium
# that permits create and write but denies rename passes a check that only
# writes a file, and then fails on every single publication.
tp_check() {
  local tmp dst
  dst="$BUNDLE_BASE/.heliograph-write-check"
  [ -d "$dst" ] && { say "$dst exists and is a directory"; return 1; }
  tmp="$(mktemp "$BUNDLE_BASE/$BUNDLE_TMP_PREFIX""check.XXXXXX" 2>/dev/null)" || return 1
  [ -n "$tmp" ] || return 1
  printf 'heliograph write check\n' > "$tmp" 2>/dev/null || { rm -f -- "$tmp"; return 1; }
  mv -f -- "$tmp" "$dst" 2>/dev/null || { rm -f -- "$tmp"; return 1; }
  rm -f -- "$dst" 2>/dev/null || {
    say "wrote and renamed a probe in $BUNDLE_BASE but could not remove it."
    say "  Delete is what a station needs to tidy its own temporaries."
    return 1
  }
  return 0
}

# What only a bundle can be wrong about.
#
# Safe to call after a FAILED tp_init, which is start.sh's contract, so every
# variable here is guarded.
tp_preflight() {
  report ok bundle "${BUNDLE_DIR:-<unset>}"

  [ -n "${TP_WANT_SCOPE:-}" ] &&
    report warn scope "a bundle has no branch and no lane to select. --branch is git's word, and there is nothing here for it to mean"

  [ -n "${BUNDLE_BASE:-}" ] || return 0

  # THE THING AN OPERATOR MOST NEEDS TOLD, and it is not a fault.
  #
  # A station on this transport polls a directory that only changes when
  # somebody walks over and plugs something in. Started with an empty
  # directory, it will sit there for ever behaving perfectly - and "the station
  # is not doing anything" is exactly what that looks like from both sides.
  #
  # Said at START, while somebody is still watching, rather than discovered.
  local waiting
  # shellcheck disable=SC2012
  waiting="$(ls -1 "$BUNDLE_BASE"/request-*.hgb 2>/dev/null | wc -l | tr -d ' ')"
  if [ "${waiting:-0}" -gt 0 ]; then
    report ok request "$waiting bundle(s) waiting to be run"
  else
    report warn request "no bundle is waiting. This station will poll until somebody carries one in - which is the transport working as intended, and is indistinguishable from a station nobody is using"
  fi

  report warn loop "this is not a round trip. The reply is written here and goes nowhere until a person carries the medium back, so 'cancel' cannot reach a running step and the station cannot update itself"

  if tp_check; then
    report ok "bundle write" "a file created, renamed and removed again - which is exactly what publishing does"
  else
    report FAIL "bundle write" "cannot write into $BUNDLE_DIR. A read-only mount, a full or failing stick, and a filesystem the kernel has remounted read-only all look exactly like this - and the station would capture logs that never leave the machine"
  fi
}

# Nothing to bring down: tp_sync is not defined, which is how a transport says
# so - see start.sh.

# --- the poll ------------------------------------------------------------------
# THE NEWEST REQUEST BUNDLE, by name.
#
# The control side names them `request-<id>.hgb`, so a stack of them on a stick
# sorts into the order they were made and is self-describing when a human looks
# at it. Newest wins, because a courier carrying three bundles is carrying three
# attempts at the same question and the last is the one meant.
#
# ABSENT AND UNREADABLE ARE DIFFERENT, and collapsing them is how a station goes
# permanently deaf without anybody being told. An empty directory is the
# ORDINARY state of this transport - it is what waiting for a courier looks
# like - so reporting it as a fetch failure would make every bundle station
# look like a flapping link for ever. A file that is THERE and cannot be read -
# a stick pulled mid-read, a dying filesystem - is a real failure and is
# counted.
tp_fetch_request() {
  local newest
  # shellcheck disable=SC2012
  newest="$(ls -1 "$BUNDLE_BASE"/request-*.hgb 2>/dev/null | LC_ALL=C sort | tail -1)"
  [ -n "$newest" ] || { printf ''; return 0; }
  cat -- "$newest" 2>/dev/null || {
    say "the bundle at $newest is there and cannot be read."
    say "  A stick pulled out mid-read looks exactly like this."
    return 1
  }
  return 0
}

# NOT DECLARED IN tp_capabilities, so the loop never calls it. Defined so that
# calling it is a refusal rather than "command not found", which would read as a
# broken payload rather than as a transport saying no.
#
# There is genuinely nothing to re-read: the medium is not going to change while
# the step runs, and a cancel would have to be carried in by hand - by which
# time the step it was meant for has long finished.
tp_fetch_request_live() { return 1; }

tp_fetch_self() { return 1; }

tp_put_status() {
  local body="$1" _msg="$2" alsofile="${3:-}"
  _bundle_publish_body "$body" "$BUNDLE_BASE/status" || return 1
  # A partial log from a cancelled run. On git it is committed alongside the
  # status so the cancellation is not stranded behind a dirty tree; here it is
  # simply the last thing that will ever be carried back from that run, and
  # losing it is losing the only evidence there is.
  if [ -n "$alsofile" ] && [ -f "$alsofile" ]; then
    _bundle_publish_file "$alsofile" "$BUNDLE_BASE/$(basename -- "$alsofile")" || return 1
  fi
  return 0
}

# Progress IS published, even though nobody is watching in real time.
#
# It looks pointless on a courier channel and is not. If the step is still
# running when somebody comes to collect the medium, the partial log is what
# they carry back; without this, they carry back a status saying `running` and
# no evidence at all, and the next walk is wasted.
tp_put_progress() {
  local body="$1" _msg="$2" logfile="$3"
  _bundle_publish_body "$body" "$BUNDLE_BASE/status" || return 1
  [ -f "$logfile" ] || return 0
  _bundle_publish_file "$logfile" "$BUNDLE_BASE/$(basename -- "$logfile")"
}

# THE LOG GOES FLAT IN THE BUNDLE DIRECTORY, not in ops-logs/.
#
# This is the one place this transport differs from the share, and it is not a
# choice: internal/transport/share.go's Bundle.ListLogs reads `*.txt` in the
# directory itself. A log written one level down travels back perfectly and is
# invisible to `heliograph logs`.
tp_put_log() {
  local logfile="$1"
  [ -f "$logfile" ] || return 1
  _bundle_publish_file "$logfile" "$BUNDLE_BASE/$(basename -- "$logfile")"
}
