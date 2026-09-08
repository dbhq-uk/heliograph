#!/usr/bin/env bash
# =============================================================================
#  test-station-compat.sh - a transport repo bootstrapped before the rename
# =============================================================================
# The loop was the "agent" until A2 renamed it, and its paths went with it. A
# transport repo is a SEPARATE repo on a machine nobody here can reach, so it
# does not get upgraded when this one does. An operator who bootstrapped last
# month still has `agent/request` in their checkout.
#
# Without the shim that presents as the loop going deaf: polling happily,
# answering nothing, with the request they just pushed apparently ignored. That
# is expensive to diagnose from the far side and it is exactly the failure this
# toolkit exists to prevent, so it is asserted rather than assumed.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
BOOTSTRAP="$HERE/../skills/heliograph/scripts/bootstrap.sh"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
GIT="git -c user.email=ci@example.invalid -c user.name=ci"

# --- a repo as it looked BEFORE the rename ------------------------------------
# The loop fetches before it reads a request, so a repo with no origin never
# gets as far as running anything. Each fixture gets its own bare remote.
mkrepo() {  # mkrepo <dir> <origin.git>
  "$BOOTSTRAP" "$1" >/dev/null 2>&1
  git init -q --bare "$2"
  ( cd "$1" \
      && git init -q \
      && git remote add origin "$2" \
      && $GIT add -A \
      && $GIT commit -qm init \
      && $GIT push -q -u origin HEAD ) >/dev/null 2>&1
}

OLD="$TMP/old"
mkrepo "$OLD" "$TMP/old-origin.git"
mv "$OLD/station" "$OLD/agent"
printf 'id: none\nstep:\nenv:\n' > "$OLD/agent/request"

out="$( cd "$OLD" && PUSH=0 timeout 8 ./station.sh --interval 2 2>&1 )"

assert_contains "the station starts against a pre-rename repo" \
  "station up on" "$out"
assert_contains "it says which paths it fell back to" \
  "reading agent/request" "$out"
assert_contains "and tells the reader how to move off the fallback" \
  "Re-run bootstrap.sh" "$out"

# The notice must be said ONCE. Repeating it every poll would bury the log the
# operator is reading, and this loop runs for days.
n="$(printf '%s\n' "$out" | grep -c 'reading agent/request')"
assert_eq "the compat notice is said once, not once per poll" "1" "$n"

# --- a repo bootstrapped AFTER the rename -------------------------------------
# The control. Without it the assertions above would pass just as well if the
# shim fired unconditionally, which would be its own bug: a current repo told
# it is running in compatibility mode is a false alarm on every start.
NEW="$TMP/new"
mkrepo "$NEW" "$TMP/new-origin.git"
printf 'id: none\nstep:\nenv:\n' > "$NEW/station/request"

out2="$( cd "$NEW" && PUSH=0 timeout 8 ./station.sh --interval 2 2>&1 )"

assert_contains "a current repo starts too" "station up on" "$out2"
assert_eq "a current repo says nothing about compatibility" "0" \
  "$(printf '%s\n' "$out2" | grep -c 'reading agent/request')"

# --- what the shim must NOT do ------------------------------------------------
# Falling back is for reading what is already there. It must never CREATE the
# old layout, or a repo bootstrapped today would quietly acquire the paths this
# rename exists to retire.
assert_eq "the shim does not create agent/ in a current repo" "0" \
  "$( [ -e "$NEW/agent" ] && echo 1 || echo 0 )"

# --- and it must not create it while actually WORKING -------------------------
# The assertion above was not enough on its own, and this is the test that
# proves it. `publish_status` and `publish_progress` each ran `mkdir -p agent`
# with the directory spelled out as a literal, so the moment a run published
# anything a current repo grew a stray `agent/` beside its `station/`. The
# checks above never fired it because they only start the loop and stop it;
# nothing was ever requested, so nothing was ever published.
#
# The fix derives the directory from the path being written, so the two cannot
# drift again. This asserts the behaviour rather than the spelling.
cat > "$NEW/steps/quick.sh" <<'EOS'
#!/usr/bin/env bash
# heliograph-mode: read-only
echo a quick measurement
EOS
chmod +x "$NEW/steps/quick.sh"
( cd "$NEW" && $GIT add -A && $GIT commit -qm step && $GIT push -q origin HEAD ) >/dev/null 2>&1
printf 'id: run-1\nstep: steps/quick.sh\nenv:\n' > "$NEW/station/request"

( cd "$NEW" && PUSH=0 timeout 25 ./station.sh --once --interval 2 ) >/dev/null 2>&1

assert_eq "a published status does not create a stray agent/ directory" "0" \
  "$( [ -e "$NEW/agent" ] && echo 1 || echo 0 )"
assert_eq "the status lands in station/, where the reader is looking" "1" \
  "$( [ -e "$NEW/station/status" ] && echo 1 || echo 0 )"

t_summary
