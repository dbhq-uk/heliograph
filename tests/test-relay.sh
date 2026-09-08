#!/usr/bin/env bash
# =============================================================================
#  test-relay.sh - the relay's sequence numbers, under contention
# =============================================================================
# THE DEFECT THIS FILE EXISTS FOR was recorded as known and unfixed for a day,
# because nothing could run it. The loop and the runner are separate processes
# and both publish: the runner sends the finished log, the loop sends `idle`.
# Both took a sequence number from a value loaded when they started, so both
# used the same one - and the receiver drops anything at or below what it has
# accepted, because that is the replay defence and a replayed request is a
# destructive step re-running with its gates already satisfied.
#
# So the log arrived and the station reported `running` for ever. Nothing
# errored, on either side of a gap nobody can cross.
#
# Contention is the whole subject here, so it is created rather than described:
# processes take numbers at the same time and every number must be distinct.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
TOOLKIT="$(cd "$HERE/../station/bash" && pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# One process taking N numbers, writing each to its own file. Sourced fresh, the
# way the runner sources the transport in a process of its own.
taker() {  # taker <id> <count>
  (
    say() { :; }
    # shellcheck disable=SC2034
    REPO_ROOT="$TOOLKIT"
    RELAY_STATE="$TMP/state"
    # shellcheck disable=SC1091
    . "$TOOLKIT/caplib.sh"
    # shellcheck disable=SC1091
    . "$TOOLKIT/transports/relay.sh"
    RELAY_SEEN=0
    RELAY_OUT=0
    local i
    for ((i = 0; i < $2; i++)); do
      _relay_next_out >> "$TMP/taken.$1"
      printf '\n' >> "$TMP/taken.$1"
    done
  )
}

# --- numbers are never handed out twice --------------------------------------
: > "$TMP/state"
for id in a b c d; do taker "$id" 15 & done
wait

got="$(cat "$TMP"/taken.* 2>/dev/null | grep -c .)"
uniq="$(cat "$TMP"/taken.* 2>/dev/null | sort -u | grep -c .)"
assert_eq "every taker got the numbers it asked for" "60" "$got"
assert_eq "and no two processes were given the same one" "$got" "$uniq"
assert_eq "the sequence has no gaps either, so nothing was lost under the lock" \
  "1 60" "$(cat "$TMP"/taken.* | sort -n | sed -n '1p;$p' | tr '\n' ' ' | sed 's/ $//')"

# --- the lock is released, not leaked ----------------------------------------
assert_eq "the lock directory is gone afterwards" "0" \
  "$([ -d "$TMP/state.lock" ] && echo 1 || echo 0)"

# --- a dead holder is broken at once -----------------------------------------
# A station is on a machine nobody can log into. A holder that was killed
# between mkdir and rmdir would otherwise stop it publishing anything, ever, and
# the only symptom would be silence.
#
# THE QUESTION IS WHETHER THE HOLDER IS ALIVE, not how old the lock is. An
# earlier version broke any lock untouched for a minute, which fires on a LIVE
# holder - the mtime does not change while it works - so the breaker deleted
# somebody's lock and two processes went in at once. Measured: 33 distinct
# numbers out of 60. A lock with a heuristic that can fire on a live holder is
# not a lock, and it fails exactly like no lock at all.
: > "$TMP/state"
rm -rf "$TMP/state.lock"
mkdir -p "$TMP/state.lock"
# A pid nothing is using. 4194304 is above the default pid_max on Linux, so it
# cannot be a live process, and `kill -0` says so.
echo 4194304 > "$TMP/state.lock/pid"
before="$(date +%s)"
rm -f "$TMP/taken.dead"
taker dead 1 >/dev/null 2>&1
after="$(date +%s)"
assert_eq "a lock whose holder is gone is broken" "1" \
  "$([ -s "$TMP/taken.dead" ] && echo 1 || echo 0)"
assert_eq "and broken AT ONCE, not after the timeout" "1" \
  "$([ "$((after - before))" -lt 5 ] && echo 1 || echo 0)"

# A LIVE holder is waited for, which is the whole point.
#
# HELD PAST THE TIMEOUT, deliberately. An earlier version broke ANY lock after
# ten seconds, so a two-second holder proved nothing about the case that
# matters: a station whose runner is mid-publish while the loop wants a number.
# Stealing there hands out a duplicate, and the receiver drops the second - so
# a log or a final status disappears with nothing reported anywhere.
rm -rf "$TMP/state.lock"
mkdir -p "$TMP/state.lock"
echo "$$" > "$TMP/state.lock/pid"
rm -f "$TMP/taken.live"
( sleep 13; rm -rf "$TMP/state.lock" ) &
releaser=$!
before="$(date +%s)"
taker live 1 >/dev/null 2>&1
after="$(date +%s)"
wait "$releaser"
assert_eq "a live holder is still not broken after the ten-second timeout" "1" \
  "$([ "$((after - before))" -ge 12 ] && echo 1 || echo 0)"
assert_eq "and the lock is taken the moment that holder releases" "1" \
  "$([ -s "$TMP/taken.live" ] && echo 1 || echo 0)"

# A lock directory with NO pid is the one genuinely ambiguous case: a holder
# that died in the window between mkdir and writing its pid looks exactly like
# one that is about to write it. Waited on, then broken, because a station that
# stopped publishing for ever is worse than one duplicate sequence number - and
# a duplicate is dropped by the receiver rather than acted on.
rm -rf "$TMP/state.lock"
mkdir -p "$TMP/state.lock"
rm -f "$TMP/taken.nopid"
taker nopid 1 >/dev/null 2>&1
assert_eq "a lock with no pid is eventually broken rather than waited on for ever" "1" \
  "$([ -s "$TMP/taken.nopid" ] && echo 1 || echo 0)"

# --- the replay counter never goes backwards ---------------------------------
# The runner does not fetch requests, so its RELAY_SEEN is whatever the loop
# held when the step started. Writing that back would REGRESS the one number
# whose entire job is never to go backwards - and a regressed counter accepts
# every replay the relay has ever seen.
printf 'seen:99\nout:5\n' > "$TMP/state"
(
  say() { :; }
  # shellcheck disable=SC2034
  REPO_ROOT="$TOOLKIT"
  # Read by relay.sh, which is sourced below. Unused by this script itself,
  # which is what SC2034 is about, and that is the point of them.
  # shellcheck disable=SC2034
  RELAY_STATE="$TMP/state"
  # shellcheck disable=SC1091
  . "$TOOLKIT/caplib.sh"
  # shellcheck disable=SC1091
  . "$TOOLKIT/transports/relay.sh"
  # shellcheck disable=SC2034
  RELAY_SEEN=3      # stale, as a runner's would be
  # shellcheck disable=SC2034
  RELAY_OUT=0
  _relay_next_out >/dev/null
) >/dev/null 2>&1
assert_eq "a stale writer cannot wind the replay counter back" "99" \
  "$(sed -n 's/^seen://p' "$TMP/state" | head -1)"
assert_eq "and the outbound counter still advanced" "6" \
  "$(sed -n 's/^out://p' "$TMP/state" | head -1)"

# --- the state file is never left half-written -------------------------------
# A truncated state file reads as sequence zero, and sequence zero accepts every
# replay the relay has ever seen.
assert_eq "no temporary is left beside the state file" "0" \
  "$(find "$TMP" -name 'state.*.tmp' -o -name 'state.tmp' | grep -c .)"

# --- a state write that FAILS is reported, not swallowed ---------------------
# A full disk or a read-only mount made this silent, and both callers ignored
# the result - so a request was verified and RUN with its sequence number never
# written down. After the next restart the relay could replay that request and
# have it accepted again. The replay defence is only a defence if it persists.
UNWRITABLE="$TMP/nowrite"
mkdir -p "$UNWRITABLE"
printf 'seen:0\nout:0\n' > "$UNWRITABLE/state"
chmod a-w "$UNWRITABLE"
if [ -w "$UNWRITABLE" ]; then
  t_skip "cannot make a directory unwritable here (running as root?)"
else
  rc=0
  (
    say() { :; }
    # shellcheck disable=SC2034
    REPO_ROOT="$TOOLKIT"
    # shellcheck disable=SC2034
    RELAY_STATE="$UNWRITABLE/state"
    # shellcheck disable=SC1091
    . "$TOOLKIT/caplib.sh"
    # shellcheck disable=SC1091
    . "$TOOLKIT/transports/relay.sh"
    # shellcheck disable=SC2034
    RELAY_SEEN=0
    # shellcheck disable=SC2034
    RELAY_OUT=0
    _relay_next_out
  ) >/dev/null 2>&1 || rc=$?
  assert_eq "a sequence number that could not be persisted is a FAILURE, not a number" "1" "$rc"

  rc=0
  (
    say() { :; }
    # shellcheck disable=SC2034
    REPO_ROOT="$TOOLKIT"
    # shellcheck disable=SC2034
    RELAY_STATE="$UNWRITABLE/state"
    # shellcheck disable=SC1091
    . "$TOOLKIT/caplib.sh"
    # shellcheck disable=SC1091
    . "$TOOLKIT/transports/relay.sh"
    # shellcheck disable=SC2034
    RELAY_SEEN=0
    # shellcheck disable=SC2034
    RELAY_OUT=0
    _relay_record_seen 7
  ) >/dev/null 2>&1 || rc=$?
  assert_eq "and a replay counter that could not be persisted is one too" "1" "$rc"
fi
chmod u+w "$UNWRITABLE" 2>/dev/null

t_summary
