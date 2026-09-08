#!/usr/bin/env bash
# =============================================================================
#  test-share.sh - the share transport, against a real directory
# =============================================================================
# This one CAN be tested for real, and that makes it the odd transport out.
# blob and relay are exercised against a fake curl, asserting the request they
# would send; a share is a directory, so every assertion here is against files
# that actually exist on disk.
#
# WHAT IT IS REALLY CHECKING is that both halves agree on the layout.
# internal/transport/share.go decides where the documents live, and the station
# is on the other side of a gap nobody can reach. A disagreement about a path is
# two sides that each work perfectly and never meet, and no amount of care on
# one side detects it - so the paths are asserted literally here, spelled the
# way share.go spells them.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
ROOT="$(cd "$HERE/.." && pwd)"
TOOLKIT="$ROOT/station/bash"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Run one command with the transport loaded, in a subshell, the way station.sh
# would have it: caplib first, because tp_init calls cap_need and without it the
# check that reports a missing variable is itself "command not found".
share() {  # share <command...>
  (
    export SHARE_DIR="$MOUNT" SHARE_SCOPE="${SCOPE:-dns-timeouts}"
    # Read by caplib and by share.sh at source time. Unused by this script
    # itself, which is what SC2034 is about, and that is the point of it.
    # shellcheck disable=SC2034
    REPO_ROOT="$TOOLKIT"
    # shellcheck disable=SC1091
    . "$TOOLKIT/caplib.sh"
    # shellcheck disable=SC1091
    . "$TOOLKIT/transports/share.sh"
    tp_init >/dev/null 2>&1 || exit 1
    "$@"
  )
}

MOUNT="$TMP/mnt"; mkdir -p "$MOUNT"

# --- tp_init CREATES NOTHING -------------------------------------------------
# `./start.sh --check` promises to change nothing, and an operator runs it on a
# node where they are not yet permitted to alter anything. tp_init made the
# scope and ops-logs in the first version of this transport, so every share
# station broke that promise silently.
share tp_init
assert_eq "tp_init creates no scope directory: --check must change nothing" "0" \
  "$(find "$MOUNT" -mindepth 1 | grep -c .)"

# --- the layout, spelled the way share.go spells it --------------------------
# The publish path creates what it needs, when it needs it.
share tp_put_status "state: starting
" "first" ""
assert_eq "the first publish creates the scope directory" "1" \
  "$([ -d "$MOUNT/dns-timeouts" ] && echo 1 || echo 0)"
printf 'seed\n' > "$TMP/seed-20260908T110000Z.txt"
share tp_put_log "$TMP/seed-20260908T110000Z.txt" "seed"
assert_eq "and ops-logs, which is where the control side lists logs" "1" \
  "$([ -d "$MOUNT/dns-timeouts/ops-logs" ] && echo 1 || echo 0)"

assert_eq "the scope is what a run is bound to" "dns-timeouts" "$(share tp_scope)"
assert_contains "describe names the mount as the credential, because it is" \
  "the mount itself" "$(share tp_describe)"

# --- the request the control side wrote --------------------------------------
printf 'id: r1\nstep: probe.sh\n' > "$MOUNT/dns-timeouts/request"
assert_contains "tp_fetch_request reads the file share.go writes" \
  "id: r1" "$(share tp_fetch_request)"
assert_contains "and the live read is the same read, because there is no tree to disturb" \
  "id: r1" "$(share tp_fetch_request_live)"

# An absent request is the ordinary state of a station that has just started.
# Reporting it as a fetch failure makes a quiet scope look like a flapping link
# for ever - which is what the loop would report, on a machine nobody can reach.
rm -f "$MOUNT/dns-timeouts/request"
share tp_fetch_request >/dev/null 2>&1
assert_eq "an absent request is not a fetch failure" "0" "$?"
assert_eq "and it emits nothing" "" "$(share tp_fetch_request)"

# --- status ------------------------------------------------------------------
share tp_put_status "state: idle
branch: dns-timeouts
" "a message" ""
assert_eq "tp_put_status writes where share.go reads" "1" \
  "$([ -f "$MOUNT/dns-timeouts/status" ] && echo 1 || echo 0)"
assert_contains "and it is the body it was given" "state: idle" \
  "$(cat "$MOUNT/dns-timeouts/status")"

assert_eq "and it leaves no temporary behind" "0" \
  "$(find "$MOUNT/dns-timeouts" -maxdepth 1 -name '*.tmp' | grep -c .)"

# --- the finished log --------------------------------------------------------
printf 'the whole log\nRESULT: pass\n' > "$TMP/probe-20260908T120000Z.txt"
share tp_put_log "$TMP/probe-20260908T120000Z.txt" "log: probe"
assert_eq "tp_put_log puts the log under ops-logs, where ListLogs looks" "1" \
  "$([ -f "$MOUNT/dns-timeouts/ops-logs/probe-20260908T120000Z.txt" ] && echo 1 || echo 0)"
assert_contains "and it is the log, not the status body" "RESULT: pass" \
  "$(cat "$MOUNT/dns-timeouts/ops-logs/probe-20260908T120000Z.txt")"

# .txt is not decoration: share.go's ListLogs filters on that suffix, so a log
# published under any other name is a log the control side cannot see at all.
assert_eq "the name is preserved exactly, because ListLogs filters on .txt" "1" \
  "$(find "$MOUNT/dns-timeouts/ops-logs" -name 'probe-*.txt' | grep -c .)"
assert_eq "and no temporary is ever left where ListLogs would find it" "0" \
  "$(find "$MOUNT/dns-timeouts" -name '.heliograph-tmp.*' | grep -c .)"

# A missing file must not be reported as a delivery. cap_deliver publishes
# `undelivered` on a 1 and `idle` on a 0, and the difference is whether anybody
# goes looking for a log that is not there.
share tp_put_log "$TMP/does-not-exist.txt" "log: nope" >/dev/null 2>&1
assert_eq "tp_put_log refuses a log that is not there rather than reporting success" \
  "1" "$?"

# --- progress ----------------------------------------------------------------
# Under its FINAL name, deliberately: the control side lists one directory and
# reads by name, so a reader following a long step sees the log grow and then be
# completed in place, rather than finding a stale snapshot beside the real thing.
printf 'partial\n' > "$TMP/slow-20260908T130000Z.txt"
share tp_put_progress "state: running
" "progress" "$TMP/slow-20260908T130000Z.txt"
assert_eq "progress publishes the partial log under the name it will finish under" "1" \
  "$([ -f "$MOUNT/dns-timeouts/ops-logs/slow-20260908T130000Z.txt" ] && echo 1 || echo 0)"
assert_contains "and updates the status alongside it" "state: running" \
  "$(cat "$MOUNT/dns-timeouts/status")"

# The cancelled-run case: tp_put_status's third argument is the partial log of a
# run that was killed. On git it is committed so the cancellation is not
# stranded behind a dirty tree; here it is simply the only evidence that run
# will ever produce, and dropping it loses it.
printf 'got this far\n' > "$TMP/killed-20260908T140000Z.txt"
share tp_put_status "state: cancelled
" "cancelled" "$TMP/killed-20260908T140000Z.txt"
assert_eq "a cancelled run's partial log ships with the status" "1" \
  "$([ -f "$MOUNT/dns-timeouts/ops-logs/killed-20260908T140000Z.txt" ] && echo 1 || echo 0)"

# --- tp_check proves a WRITE -------------------------------------------------
assert_eq "tp_check passes on a writable share" "0" "$(share tp_check >/dev/null 2>&1; echo $?)"
assert_eq "and leaves no probe behind, because a share is a directory people browse" "0" \
  "$(find "$MOUNT/dns-timeouts" -name '.heliograph-write-check' | grep -c .)"

# A read-only mount and a stale NFS handle both stat perfectly and fail on the
# first write, which would be the log, an hour later, with nobody left to tell.
#
# tp_check IS THE AUTHORITY, not tp_init. tp_init deliberately writes nothing -
# `--check` promises to change nothing - so it cannot know, and `[ -w ]` lies
# on NFS and on anything with an ACL. The one honest answer is to try.
RO="$TMP/ro"; mkdir -p "$RO/locked"
chmod a-w "$RO/locked"
if [ -w "$RO/locked" ]; then
  t_skip "cannot make a directory unwritable here (running as root?), so the read-only case is unchecked"
else
  rc=0
  ( export SHARE_DIR="$RO" SHARE_SCOPE=locked
    # Read by caplib and by share.sh at source time. Unused by this script
    # itself, which is what SC2034 is about, and that is the point of it.
    # shellcheck disable=SC2034
    REPO_ROOT="$TOOLKIT"
    say() { :; }
    # shellcheck disable=SC1091
    . "$TOOLKIT/caplib.sh"
    # shellcheck disable=SC1091
    . "$TOOLKIT/transports/share.sh"
    tp_init && tp_check ) >/dev/null 2>&1 || rc=$?
  assert_eq "tp_check refuses a share this account cannot write to" "1" "$rc"

  # And it must not be reported as a delivery either. cap_deliver publishes
  # `undelivered` on a 1 and `idle` on a 0, and the difference is whether
  # anybody goes looking for a log that never arrived.
  rc=0
  ( export SHARE_DIR="$RO" SHARE_SCOPE=locked
    # shellcheck disable=SC2034
    REPO_ROOT="$TOOLKIT"
    say() { :; }
    # shellcheck disable=SC1091
    . "$TOOLKIT/caplib.sh"
    # shellcheck disable=SC1091
    . "$TOOLKIT/transports/share.sh"
    tp_init && tp_put_log "$TMP/probe-20260908T120000Z.txt" "msg" ) >/dev/null 2>&1 || rc=$?
  assert_eq "and a log that could not be written is not reported as delivered" "1" "$rc"
fi
chmod u+w "$RO/locked" 2>/dev/null

# --- a destination that is a directory ---------------------------------------
# `mv tmp status` where `status` is a directory succeeds, and puts the file
# INSIDE it. The station would report a delivery the control side cannot see.
mkdir -p "$MOUNT/dirstatus/status"
rc=0
( export SHARE_DIR="$MOUNT" SHARE_SCOPE=dirstatus
  # shellcheck disable=SC2034
  REPO_ROOT="$TOOLKIT"
  say() { :; }
  # shellcheck disable=SC1091
  . "$TOOLKIT/caplib.sh"
  # shellcheck disable=SC1091
  . "$TOOLKIT/transports/share.sh"
  tp_init && tp_put_status "state: idle
" "msg" "" ) >/dev/null 2>&1 || rc=$?
assert_eq "publishing onto a directory fails rather than putting the file inside it" "1" "$rc"
assert_eq "and nothing was left inside that directory" "0" \
  "$(find "$MOUNT/dirstatus/status" -mindepth 1 | grep -c .)"

# --- a request that is there and cannot be read ------------------------------
# ABSENT AND UNREADABLE ARE DIFFERENT. Collapsing them made the station poll a
# share it could no longer read, for ever, reporting itself idle.
mkdir -p "$MOUNT/unreadable"
printf 'id: r9\n' > "$MOUNT/unreadable/request"
chmod a-r "$MOUNT/unreadable/request"
if [ -r "$MOUNT/unreadable/request" ]; then
  t_skip "cannot make a file unreadable here (running as root?)"
else
  rc=0
  ( export SHARE_DIR="$MOUNT" SHARE_SCOPE=unreadable
    # shellcheck disable=SC2034
    REPO_ROOT="$TOOLKIT"
    say() { :; }
    # shellcheck disable=SC1091
    . "$TOOLKIT/caplib.sh"
    # shellcheck disable=SC1091
    . "$TOOLKIT/transports/share.sh"
    tp_init && tp_fetch_request ) >/dev/null 2>&1 || rc=$?
  assert_eq "a request that exists and cannot be read is a fetch FAILURE, not an empty queue" \
    "1" "$rc"
fi
chmod u+r "$MOUNT/unreadable/request" 2>/dev/null

# --- the scope is one directory name -----------------------------------------
# It becomes a directory under a path this station did not choose. `../..` would
# put every status and every log outside the share entirely, somewhere the
# control side will never look. share.go refuses the same shapes on the near
# side; this is the far side's own copy, because a station may not assume the
# thing that configured it was the CLI.
# A NEWLINE is the one that matters and the one a separator check misses. The
# scope is published as the status document's `branch:` value, and that document
# is line-oriented `key: value` - so a scope holding a newline injects a second
# key, the parser keeps the last, and an idle station reports itself busy for
# ever. internal/transport/share.go refuses the same set on the near side.
for bad in ".." "." "a/b" 'a\b' "" "$(printf 'x\nstate: running')" "-rf" 'a b' 'a*'; do
  rc=0
  ( export SHARE_DIR="$MOUNT" SHARE_SCOPE="$bad"
    # Read by caplib and by share.sh at source time. Unused by this script
    # itself, which is what SC2034 is about, and that is the point of it.
    # shellcheck disable=SC2034
    REPO_ROOT="$TOOLKIT"
    # shellcheck disable=SC1091
    . "$TOOLKIT/caplib.sh"
    # shellcheck disable=SC1091
    . "$TOOLKIT/transports/share.sh"
    tp_init ) >/dev/null 2>&1 || rc=$?
  assert_eq "SHARE_SCOPE='$bad' is refused: it becomes one directory name" "1" "$rc"
done
assert_eq "and nothing escaped the share while proving it" "0" \
  "$(find "$TMP" -maxdepth 1 -name 'status' -o -maxdepth 1 -name 'ops-logs' | grep -c .)"

# --- a share that is not mounted ---------------------------------------------
# This transport creates the SCOPE, never the share. A missing mount that got
# silently created is a station publishing into local disk that the control side
# will never see, which is the failure mode a share has and the others do not.
rc=0
( export SHARE_DIR="$TMP/never-mounted" SHARE_SCOPE=x
  # shellcheck disable=SC2034
  REPO_ROOT="$TOOLKIT"
  # shellcheck disable=SC1091
  . "$TOOLKIT/caplib.sh"
  # shellcheck disable=SC1091
  . "$TOOLKIT/transports/share.sh"
  tp_init ) >/dev/null 2>&1 || rc=$?
assert_eq "a share directory that is not there is refused" "1" "$rc"
assert_eq "and is NOT created, because creating it hides an unmounted share" "0" \
  "$([ -d "$TMP/never-mounted" ] && echo 1 || echo 0)"

# --- capabilities ------------------------------------------------------------
CAPS="$(share tp_capabilities)"
assert_contains "it offers a live read, which git has to keep separate and this does not" \
  "live" "$CAPS"
assert_eq "and does NOT claim self-update: nothing publishes a payload to a share" "" \
  "$(printf '%s' "$CAPS" | grep -ow self)"
assert_eq "tp_fetch_self is still DEFINED, so calling it is a refusal and not 'command not found'" \
  "1" "$(share tp_fetch_self >/dev/null 2>&1; echo $?)"

# =============================================================================
#  Through start.sh, which is how an operator meets it
# =============================================================================
make_payload() {
  "$ROOT/station/bootstrap.sh" "$1" >/dev/null 2>&1
  cat > "$1/station.sh" <<'EOF'
#!/usr/bin/env bash
echo "STUB AGENT args=$*"
EOF
  chmod +x "$1/station.sh"
}

make_payload "$TMP/station"
MOUNT2="$TMP/mnt2"; mkdir -p "$MOUNT2"
RC=0
OUT="$( cd "$TMP/station" && TRANSPORT=share SHARE_DIR="$MOUNT2" SHARE_SCOPE=probe \
        ./start.sh --check 2>&1 )" || RC=$?
assert_eq "a share station passes the preflight, in a directory that is not a git repository" \
  "0" "$RC"
assert_contains "the transport is named" "ok    transport     share" "$OUT"
assert_contains "and the write is proved rather than assumed" "share write" "$OUT"
assert_eq "and no git row appears" "0" \
  "$(printf '%s\n' "$OUT" | grep -cE '^(ok|warn|FAIL) +(git|branch|remote|token)\b')"

# THE SILENT FAILURE THIS TRANSPORT HAS AND THE OTHERS DO NOT. A typo in
# SHARE_SCOPE is not an error - the publish path just creates that directory -
# and then both sides run perfectly for ever, publishing into two directories
# that never meet. git cannot do this: a branch that does not exist is refused
# by the remote.
assert_contains "a scope that does not exist yet is called out, because a typo in it is silent" \
  "does not exist yet" "$OUT"

# `--check` PROMISES TO CHANGE NOTHING. It is what an operator runs on a node
# where they are not yet permitted to alter anything, and the git path has
# asserted this since the beginning. The first version of this transport made
# the scope directory in tp_init, so every share station broke that promise,
# and the test asserted the mutation rather than catching it.
assert_eq "--check created nothing at all, which is what it promises" "0" \
  "$(find "$MOUNT2" -mindepth 1 | grep -c .)"

# Run it FOR REAL, and only then is the scope there.
cat > "$TMP/station/steps/probe.sh" <<'EOF'
#!/usr/bin/env bash
# heliograph-mode: read-only
echo probe
EOF
RC=0
OUT="$( cd "$TMP/station" && TRANSPORT=share SHARE_DIR="$MOUNT2" SHARE_SCOPE=probe \
        ./station.sh 2>&1 )" || RC=$?
mkdir -p "$MOUNT2/probe"
RC=0
OUT="$( cd "$TMP/station" && TRANSPORT=share SHARE_DIR="$MOUNT2" SHARE_SCOPE=probe \
        ./start.sh --check 2>&1 )" || RC=$?
assert_contains "and once the scope is there, the preflight says so instead" \
  "is already there" "$OUT"

# A scope that is a SYMLINK is refused. On NFS or SMB a symlink is resolved by
# each client separately, so the two sides follow it to different directories
# and never meet - this transport's signature failure, and the hardest to see.
ln -s "$TMP" "$MOUNT2/linked"
RC=0
OUT="$( cd "$TMP/station" && TRANSPORT=share SHARE_DIR="$MOUNT2" SHARE_SCOPE=linked \
        ./start.sh --check 2>&1 )" || RC=$?
assert_eq "a scope that is a symlink blocks" "1" "$RC"
assert_contains "and the message says why a symlink on a share is different" \
  "resolved by each client" "$OUT"

RC=0
OUT="$( cd "$TMP/station" && TRANSPORT=share SHARE_DIR="$TMP/not-mounted" SHARE_SCOPE=probe \
        ./start.sh --check 2>&1 )" || RC=$?
assert_eq "an unmounted share blocks" "1" "$RC"
assert_contains "and the preflight says to mount it rather than blaming something else" \
  "Mount it first" "$OUT"

# --branch is git's word for a scope, and start.sh refuses it on any other
# transport. The share names its own variable, because an operator who reached
# for --branch wants to know what to reach for instead.
RC=0
OUT="$( cd "$TMP/station" && TRANSPORT=share SHARE_DIR="$MOUNT2" SHARE_SCOPE=probe \
        ./start.sh --check --branch station/db-a 2>&1 )" || RC=$?
assert_eq "--branch is refused on a share" "1" "$RC"
assert_contains "and the message says whose flag it is" "the git transport's" "$OUT"

# --- world-writable is an access-control fact, and this transport has no other -
WW="$TMP/worldwritable"; mkdir -p "$WW"
chmod 777 "$WW"
RC=0
OUT="$( cd "$TMP/station" && TRANSPORT=share SHARE_DIR="$WW" SHARE_SCOPE=probe \
        ./start.sh --check 2>&1 )" || RC=$?
assert_eq "a world-writable share does not block: it is a fact, not a fault" "0" "$RC"
assert_contains "but it is reported, because the mount is the only credential there is" \
  "world-writable" "$OUT"

chmod 755 "$WW"
RC=0
OUT="$( cd "$TMP/station" && TRANSPORT=share SHARE_DIR="$WW" SHARE_SCOPE=probe \
        ./start.sh --check 2>&1 )" || RC=$?
assert_eq "and a share nobody else can write to is not warned about" "" \
  "$(printf '%s\n' "$OUT" | grep -o 'world-writable')"

t_summary
