#!/usr/bin/env bash
# =============================================================================
#  test-root-refusal.sh - the account is the blast radius, so it is checked
# =============================================================================
# This toolkit carries no credentials: no cloud auth, no API keys, nothing but
# the git remote. So the only honest answer to "what could a step do to this
# estate" is "whatever the account running it could do" - and that answer is
# only worth anything if the account is not root.
#
# Refused rather than warned about, because a warning in a captured log is read
# after the run, by which time the run has happened.
#
# The refusal is exercised with a fake `id` on PATH rather than by running the
# suite as root. Same code path, no privileged CI.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
ROOT="$(cd "$HERE/.." && pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

TR="$TMP/tr"
GIT="git -c user.email=ci@example.invalid -c user.name=ci"
"$ROOT/station/bootstrap.sh" "$TR" >/dev/null 2>&1
( cd "$TR" && git init -q && $GIT add -A && $GIT commit -qm init ) >/dev/null 2>&1

# A fake `id` that answers 0 to `id -u` and defers to the real one otherwise, so
# nothing else in the run starts behaving oddly.
mkdir -p "$TMP/bin"
real_id="$(command -v id)"
cat > "$TMP/bin/id" <<EOF
#!/bin/sh
[ "\$1" = "-u" ] && { echo 0; exit 0; }
exec "$real_id" "\$@"
EOF
chmod +x "$TMP/bin/id"
as_root() { ( cd "$TR" && PATH="$TMP/bin:$PATH" PUSH=0 "$@" 2>&1 ); }

# --- run.sh -------------------------------------------------------------------
RC=0; OUT="$(as_root ./run.sh env)" || RC=$?
assert_eq "run.sh refuses to run as root" "5" "$RC"
assert_contains "and says why, in terms of blast radius" "blast radius" "$OUT"
assert_contains "and says what to do instead" "unprivileged" "$OUT"
assert_eq "and captured nothing on the way out" "0" \
  "$(ls "$TR"/ops-logs/env-*.txt 2>/dev/null | wc -l)"

RC=0; OUT="$(as_root env ALLOW_ROOT=1 ./run.sh env)" || RC=$?
assert_eq "ALLOW_ROOT=1 is the documented way out, for an image with no other user" "0" "$RC"
assert_eq "and it really ran" "1" "$(ls "$TR"/ops-logs/env-*.txt 2>/dev/null | wc -l)"

# An unknown step must still say it is unknown, whoever is asking: the root
# check sits AFTER validation so a typo does not come back as a lecture about
# privilege.
RC=0; OUT="$(as_root ./run.sh nosuchstep)" || RC=$?
assert_eq "an unknown step is still reported as unknown, even as root" "2" "$RC"

# --- caprun.sh ----------------------------------------------------------------
RC=0; OUT="$(as_root ./caprun.sh label -- echo hello)" || RC=$?
assert_eq "caprun.sh refuses too - it is the same capture, without the step table" "5" "$RC"

# --- station.sh -----------------------------------------------------------------
# The worst place to discover this is an unattended loop, so the agent checks at
# startup rather than per request: a loop that would refuse everything should
# say so before the operator walks away.
RC=0; OUT="$(as_root ./station.sh --once --interval 1)" || RC=$?
assert_eq "the agent refuses at startup" "5" "$RC"
assert_contains "and names the override rather than leaving it to be guessed" "ALLOW_ROOT=1" "$OUT"
assert_eq "and leaves no lock behind, so the next run is not blocked by a corpse" "0" \
  "$( [ -e "$TR/.station.lock" ] && echo 1 || echo 0 )"

# --- AND THE OVERRIDE HAS TO REACH THE RUNNER --------------------------------
# `--allow-root` set a plain shell variable for as long as the flag has existed,
# so it satisfied the check immediately above and reached nothing else. run.sh
# is a SEPARATE PROCESS with a root gate of its own, reading ALLOW_ROOT from its
# environment - so the loop started perfectly and then refused every single step
# with exit 5, publishing `undelivered` with "the runner exited before it
# reached delivery". That names the symptom and not one word of the cause.
#
# `ALLOW_ROOT=1 ./station.sh` worked the whole time, because that form is
# already in the environment. So the variable worked and the flag documented
# beside it did not - the worse of the two to get wrong, because the flag is
# what SAFETY tells the appliance and minimal-image case to use, and that case
# is precisely the one with no other account to fall back to.
#
# ASSERTING THAT THE STATION STARTS IS NOT ENOUGH, and was the shape of the
# hole: it started. What catches it is a LOG, because the flag has only done its
# job once the step actually ran.
# `env` RATHER THAN AN ASSIGNMENT PREFIX, and this is not style.
#
# Bash recognises `VAR=x cmd` at PARSE time, before expansion - so a `VAR=x`
# arriving through "$@" is taken as the COMMAND NAME, not as an assignment. The
# first version of this helper did exactly that: the station never ran at all,
# and the assertion below read a status file the PREVIOUS sub-test had left
# saying `idle`. It passed while proving nothing, and only showed itself when
# the fix was reverted and the check still passed.
#
# ALLOW_ROOT IS UNSET FIRST, ALWAYS, and a caller that wants it re-adds it.
# These checks are about WHICH MECHANISM delivers that variable, so inheriting
# one from whoever ran this file would answer the question before it was asked.
# A later `VAR=value` overrides the `-u`, which is what `env` does.
as_root_share() {  # as_root_share [VAR=value ...] <command...>
  ( cd "$TR" && env -u ALLOW_ROOT PATH="$TMP/bin:$PATH" TRANSPORT=share \
      SHARE_DIR="$TMP/share" SHARE_SCOPE=scope "$@" 2>&1 )
}
# CLEARED BEFORE EACH RUN, for the same reason. A status left by the previous
# sub-test reads exactly like a successful one from this test.
fresh_scope() {  # fresh_scope <id>
  rm -rf "$TMP/share/scope"
  mkdir -p "$TMP/share/scope"
  printf 'id: %s\nstep: steps/env-snapshot.sh\n' "$1" > "$TMP/share/scope/request"
}

fresh_scope allow-root-1
RC=0; OUT="$(as_root_share ./station.sh --once --interval 1 --allow-root)" || RC=$?
STATE="$(sed -n 's/^state:[[:space:]]*//p' "$TMP/share/scope/status" 2>/dev/null | head -1)"
assert_eq "--allow-root lets the loop run a step, not merely start" "idle" "$STATE"
assert_eq "  and the log reached the far side, which is the only proof the flag crossed into run.sh" "1" \
  "$(ls -1 "$TMP/share/scope/ops-logs/"*.txt 2>/dev/null | wc -l | tr -d ' ')"

# The environment form has to keep working too. It always did - which is what
# made the flag's failure so hard to see - and a "fix" that moved the flag while
# breaking the variable would be a straight trade.
fresh_scope allow-root-2
RC=0; OUT="$(as_root_share ALLOW_ROOT=1 ./station.sh --once --interval 1)" || RC=$?
STATE="$(sed -n 's/^state:[[:space:]]*//p' "$TMP/share/scope/status" 2>/dev/null | head -1)"
assert_eq "ALLOW_ROOT=1 in the environment does the same" "idle" "$STATE"

# AND THE GATE STILL REFUSES when neither is given. Without this, a station.sh
# that simply stopped checking would pass both assertions above.
fresh_scope allow-root-3
RC=0; OUT="$(as_root_share ./station.sh --once --interval 1)" || RC=$?
assert_eq "and with neither, the loop still refuses at startup" "5" "$RC"
assert_eq "  having published nothing and run nothing" "0" \
  "$(ls -1 "$TMP/share/scope/ops-logs/"*.txt 2>/dev/null | wc -l | tr -d ' ')"

t_summary
