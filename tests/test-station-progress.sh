#!/usr/bin/env bash
# =============================================================================
#  test-station-progress.sh - a long step is watchable WHILE it runs
# =============================================================================
# station.sh pushes a snapshot of the partial log every PROGRESS_EVERY seconds
# so that a human watching a forty-minute step can tell it is still moving.
# Without it a long run is a black box: nothing reaches the far side until the
# step finishes, and "running" and "wedged" look identical from the only side
# that can see anything. It is the same argument AGENTS.md makes about
# timestamps - after the fact, in an untimed log, a hang and slow progress are
# indistinguishable.
#
# THIS COUNTS THE SNAPSHOTS, AND THAT IS THE WHOLE POINT OF THE FILE.
#
# The progress path published NOTHING for every step sent by path, which is the
# documented way to send one. `ls -t ops-logs/"${STEP}"-*.txt` globbed on the
# raw step, so a step sent as `./steps/slow.sh` looked for
# `ops-logs/./steps/slow.sh-*.txt` and matched nothing; publish_progress took
# the empty logfile and returned on its first line. Nothing errored, nothing
# was said, and the run completed normally - the defect's only symptom was an
# absence, which is why it survived.
#
# So an assertion of the shape "a progress snapshot arrived" is not enough on
# its own, and neither is one that reads the status document at the end: the
# loop publishes `running` before the step starts and `idle` after it finishes,
# and both are status documents that are not progress. The number is the
# evidence. A step running for STEP_SECONDS at PROGRESS_EVERY=1 must put
# several snapshots on the far side, and the broken code put zero.
#
# WHY A BAND AND NOT AN EXACT EQUALITY. The loop's cycle is `sleep INTERVAL`
# plus a git fetch and a git push, so the count is bounded rather than fixed:
# one publication per iteration at most, and each iteration takes at least
# INTERVAL. The floor is five times what "at least one" would have asked for,
# and the ceiling catches the opposite failure - a publisher that ignores
# PROGRESS_EVERY and commits on every poll.
#
# THE FAR SIDE, AND ONLY THE FAR SIDE. Every count below is taken from the bare
# origin, not from the station's working tree and not from its terminal output.
# A station that publishes nothing and a station that died are the same thing
# from where the operator is standing, and the far side is the only place that
# difference shows.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
ROOT="$(cd "$HERE/.." && pwd)"

TMP="$(mktemp -d)"
trap 'pkill -f "$TMP/tr/station.sh" 2>/dev/null; rm -rf "$TMP"' EXIT

TR="$TMP/tr"
ORIGIN="$TMP/origin.git"
GIT="git -c user.email=ci@example.invalid -c user.name=ci"

# LONG ENOUGH TO PRODUCE SEVERAL SNAPSHOTS, AND NO LONGER. Twelve seconds at
# PROGRESS_EVERY=1 leaves room for the loop to publish about ten times while
# keeping the whole file under half a minute.
STEP_SECONDS=12
PROGRESS_FLOOR=5

"$ROOT/station/bootstrap.sh" "$TR" >/dev/null 2>&1
git init -q --bare "$ORIGIN"
( cd "$TR" \
    && git init -q \
    && git remote add origin "$ORIGIN" \
    && $GIT add -A \
    && $GIT commit -qm init \
    && $GIT push -q -u origin HEAD ) >/dev/null 2>&1

# A step that prints on a clock, so each snapshot sees more of the log than the
# one before it. A step that printed everything at once would give every
# snapshot the same line count, and "the count rose" is the part that proves
# these are snapshots of a RUNNING step rather than repeats of one reading.
cat > "$TR/steps/slow.sh" <<EOF
#!/usr/bin/env bash
# heliograph-mode: read-only
echo "the first line"
for i in \$(seq 1 $STEP_SECONDS); do
  echo "probe \$i"
  sleep 1
done
EOF
chmod +x "$TR/steps/slow.sh"
( cd "$TR" && $GIT add -A && $GIT commit -qm "the slow step" && $GIT push -q ) >/dev/null 2>&1

# BY PATH, which is the documented way to send a step - `heliograph send
# steps/probe.sh` - and the only way that reproduces this. A step named in
# run.sh's table has a bare name, its label and its name are the same string,
# and the broken glob matched for exactly that case. That is how the defect
# survived a suite that already drove this loop.
{ echo "id: pr1"; echo "step: ./steps/slow.sh"; } > "$TR/station/request"
( cd "$TR" && $GIT add -A && $GIT commit -qm "request pr1" && $GIT push -q ) >/dev/null 2>&1

STATION_OUT="$(
  cd "$TR" && env PROGRESS_EVERY=1 \
    timeout 120 ./station.sh --once --interval 1 2>&1
)"
STATION_RC=$?

# --- what reached the far side ------------------------------------------------
# tp_put_progress is the only thing that writes a commit whose subject begins
# "station: progress". `running`, `idle`, `refused` and `cancelled` all go
# through tp_put_status and say so, so this counts the progress path and
# nothing else.
progress_subjects() {
  git -C "$ORIGIN" log --all --format='%s' 2>/dev/null | grep '^station: progress'
}
progress_count() { progress_subjects | grep -c . ; }

COUNT="$(progress_count)"

assert_eq "the station ran the step and exited cleanly" "0" "$STATION_RC"
assert_contains "  and it was the step sent by path that ran" "./steps/slow.sh" "$STATION_OUT"

# THE COUNT. Against the unfixed glob this is 0.
if [ "$COUNT" -ge "$PROGRESS_FLOOR" ]; then
  t_ok "a ${STEP_SECONDS}s step at PROGRESS_EVERY=1 published $COUNT progress snapshots (floor $PROGRESS_FLOOR)"
else
  t_no "a ${STEP_SECONDS}s step at PROGRESS_EVERY=1 published $COUNT progress snapshots, wanted at least $PROGRESS_FLOOR"
  printf '     the far side holds these status commits:\n'
  git -C "$ORIGIN" log --all --format='       %h %s' 2>/dev/null | head -20
fi

# The opposite failure: a publisher that ignored PROGRESS_EVERY and committed on
# every poll would bury the history it exists to make readable. At most one
# publication per iteration, and an iteration sleeps INTERVAL, so the step's own
# duration in seconds is the ceiling.
if [ "$COUNT" -le "$((STEP_SECONDS + 1))" ]; then
  t_ok "  and no more than one per second, so PROGRESS_EVERY is honoured"
else
  t_no "  and no more than one per second, so PROGRESS_EVERY is honoured"
  printf '     expected: <= %s\n     actual:   [%s]\n' "$((STEP_SECONDS + 1))" "$COUNT"
fi

# --- the snapshots are of a RUNNING step, not one reading published N times ---
# `progress: N lines` is in every snapshot's commit subject. If the counts never
# rise, whatever is being published is not following anything.
#
# OLDEST FIRST. `git log` is newest first, so reading these in git's own order
# says the count FELL across the run and fails against perfectly good code,
# which is what the first version of this check did.
LINE_COUNTS="$(progress_subjects | sed -n 's/.*) \([0-9][0-9]*\) lines.*/\1/p' | tac)"
FIRST_LINES="$(printf '%s\n' "$LINE_COUNTS" | head -1)"
LAST_LINES="$(printf '%s\n' "$LINE_COUNTS" | tail -1)"
if [ -n "$FIRST_LINES" ] && [ -n "$LAST_LINES" ] && [ "$LAST_LINES" -gt "$FIRST_LINES" ]; then
  t_ok "the line count rose across the run ($FIRST_LINES -> $LAST_LINES), so these follow a running step"
else
  t_no "the line count rose across the run, so these follow a running step"
  printf '     counts seen, oldest first: [%s]\n' "$(printf '%s' "$LINE_COUNTS" | tr '\n' ' ')"
fi

# --- the snapshot names the log by the label run.sh actually gave it ----------
# The number alone would be satisfied by a snapshot naming a log nobody can
# fetch. On git the control side can list the directory and survive that; on a
# relay it cannot, because a relay is a queue and the name in the status is the
# only name the log will ever have.
#
# THE FIRST SNAPSHOT, and the choice is not cosmetic. The LAST one is published
# after run.sh has already committed the finished log, so it carries the status
# document alone - there is no remaining change to the log to stage - and
# reading that one as "the partial log did not travel" is a false negative. It
# was the first version of this check, and it failed against the fixed code.
# The first snapshot is the one that puts a log on the far side while the step
# is still running, which is the property worth asserting.
FIRST_PROGRESS_COMMIT="$(git -C "$ORIGIN" log --all --format='%H %s' 2>/dev/null |
  grep '^[0-9a-f]* station: progress' | tail -1 | cut -d' ' -f1)"
if [ -n "$FIRST_PROGRESS_COMMIT" ]; then
  PROGRESS_DOC="$(git -C "$ORIGIN" show "$FIRST_PROGRESS_COMMIT:station/status" 2>/dev/null)"
  assert_contains "the snapshot names the log by run.sh's label, not by the step's path" \
    "log:      ops-logs/slow-" "$PROGRESS_DOC"
  assert_contains "  and the state it publishes is running" "state:    running" "$PROGRESS_DOC"
  # THE PARTIAL LOG ITSELF, not just a count. A line saying "12 lines" with no
  # log beside it is a number nobody can act on, and tp_put_progress commits the
  # file alongside the document precisely so it can be read.
  PARTIAL="$(git -C "$ORIGIN" show "$FIRST_PROGRESS_COMMIT" --name-only --format='' 2>/dev/null)"
  assert_contains "  and the partial log travels with it, so the run can be followed" \
    "ops-logs/slow-" "$PARTIAL"
  # AND IT ARRIVED BEFORE DELIVERY DID. Without this, everything above could be
  # satisfied by a finished run: tp_put_log also puts a log on the far side, and
  # a check that cannot tell the two apart proves nothing about watching a step
  # in flight. This is the ordering that makes the snapshot a live signal.
  DELIVERY_COMMIT="$(git -C "$ORIGIN" log --all --format='%H %s' 2>/dev/null |
    grep '^[0-9a-f]* step: ' | head -1 | cut -d' ' -f1)"
  if [ -n "$DELIVERY_COMMIT" ] &&
     git -C "$ORIGIN" merge-base --is-ancestor "$FIRST_PROGRESS_COMMIT" "$DELIVERY_COMMIT" 2>/dev/null; then
    t_ok "  and it reached the far side BEFORE the finished log was delivered"
  else
    t_no "  and it reached the far side BEFORE the finished log was delivered"
  fi
else
  t_no "no progress commit reached the far side, so nothing above could be read"
fi

# --- the log is NOT written by the progress path -----------------------------
# AGENTS.md constraint 3: the runner owns the log, and the partial push
# publishes a snapshot and never writes to the file. A progress path that
# touched the log would corrupt the evidence it exists to publish.
DELIVERED_LOG="$(ls -t "$TR"/ops-logs/slow-*.txt 2>/dev/null | head -1)"
if [ -n "$DELIVERED_LOG" ]; then
  assert_contains "the finished log is intact and carries its footer" \
    "finished UTC" "$(cat "$DELIVERED_LOG")"
  assert_eq "  and the station wrote nothing of its own into it" "0" \
    "$(grep -c 'station: progress' "$DELIVERED_LOG")"
else
  t_no "the step produced no log at all, so the assertions about it did not run"
fi

# --- the label is derived ONCE, not at each call site -------------------------
# This is the defect's actual shape: the correct derivation existed, twenty
# lines below the call site that was not using it. A third caller copying it by
# hand would get it wrong the same way and nothing would say so. So the
# derivation is a function, and every caller calls it.
#
# COMMENTS ARE EXCLUDED DELIBERATELY. station.sh quotes the broken glob in the
# comment that records what it cost, and that history must not have to be
# deleted to keep this assertion honest. Matching the comment was the first
# version of this check and it failed against the FIXED code, which is the
# clearest way to be told an assertion is measuring the wrong thing.
STATION_SH="$ROOT/station/bash/station.sh"
station_code() { grep -v '^[[:space:]]*#' "$STATION_SH"; }

assert_eq "station.sh derives the step's log label in exactly one place" "1" \
  "$(grep -c '^step_log()' "$STATION_SH")"
assert_eq "  and both call sites call it rather than deriving their own" "2" \
  "$(station_code | grep -c 'step_log "\$STEP"')"
assert_eq "  so no ops-logs glob is built from the raw step any more" "0" \
  "$(station_code | grep -c 'ops-logs/"\${STEP}"')"
# The other half of the same mistake: a hand-rolled basename anywhere outside
# step_log is the pattern being copied again.
assert_eq "  and no call site derives a label by hand" "0" \
  "$(station_code | grep -c 'basename "\$STEP"')"

# The twins must agree. station.ps1 has had this right since it was written -
# Find-StepLog, whose own comment is "THE LABEL, NOT THE STEP NAME" - and two
# stations answering the same request differently is the one thing they may not
# do. The bash side now has the same single helper.
STATION_PS1="$ROOT/station/powershell/station.ps1"
if [ -f "$STATION_PS1" ]; then
  assert_eq "the PowerShell twin derives it in one helper too" "1" \
    "$(grep -c '^function Find-StepLog' "$STATION_PS1")"
  assert_eq "  called by both of its call sites" "2" \
    "$(grep -c 'Find-StepLog -Step' "$STATION_PS1")"
fi

t_summary
