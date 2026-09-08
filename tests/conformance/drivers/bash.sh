#!/usr/bin/env bash
# =============================================================================
#  drivers/bash.sh - the current bash toolkit, under the conformance contract
# =============================================================================
# Sourced by conformance.sh. Implements drv_* and nothing else. All knowledge
# of caplib.sh, run.sh and bootstrap.sh lives here, so the suite itself stays a
# specification rather than a second copy of this implementation.
# =============================================================================

_D_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_D_TOOLKIT="$(cd "$_D_HERE/../../../station/bash" && pwd)"
_D_BOOTSTRAP="$(cd "$_D_HERE/../../../station" && pwd)/bootstrap.sh"

drv_name() { printf 'bash toolkit (caplib.sh, run.sh)'; }

drv_supports() {
  case "$1" in
    capture|gates|cancel|deliver) return 0 ;;
    *) return 1 ;;
  esac
}

# Capture in a subshell so caplib's globals never leak between properties.
drv_capture() {
  local out="$1" script="$2"
  (
    # shellcheck disable=SC1091
    . "$_D_TOOLKIT/caplib.sh"
    cap_header "$out" "conformance"
    cap_run "$out" "$script"
    rc=$?
    cap_footer "$out" "$rc"
    exit "$rc"
  ) >/dev/null 2>&1
}

# Bootstrap a transport repo, WITH somewhere for a delivery to land.
#
# The bare remote is not scenery. Property 9 asks whether a finished log reached
# the far side, and the only honest way to answer that is to read it back from
# the other end rather than from the working tree that wrote it. A checkout with
# no remote would let a delivery that never happened look identical to one that
# did, which is precisely the defect p9 exists to catch.
drv_bootstrap() {
  local dir="$1"
  "$_D_BOOTSTRAP" "$dir" >/dev/null 2>&1 || return 1
  (
    cd "$dir" || exit 1
    git init -q .
    git -c user.email=ci@example.invalid -c user.name=ci add -A
    git -c user.email=ci@example.invalid -c user.name=ci commit -qm init
    git init -q --bare "$dir.remote.git"
    git remote add origin "$dir.remote.git"
    git push -q -u origin HEAD
  ) >/dev/null 2>&1
}

drv_step() {
  local dir="$1" step="$2"
  ( cd "$dir" && PUSH=0 ./run.sh "$step" ) >/dev/null 2>&1
}

# Run a step and let it DELIVER. No PUSH=0 here, deliberately: the delivery is
# the thing under test.
drv_deliver() {
  local dir="$1" step="$2"
  ( cd "$dir" && ./run.sh "$step" ) >/dev/null 2>&1
}

# Read back what the far side actually received.
#
# From the BARE REMOTE, never from the working tree. The tree holds the log
# whether or not it was ever delivered, so reading it there would assert
# nothing at all.
drv_delivered() {
  local dir="$1" remote="$1.remote.git" branch name
  branch="$(git -C "$dir" rev-parse --abbrev-ref HEAD 2>/dev/null)" || return 1
  name="$(git -C "$remote" ls-tree -r --name-only "$branch" 2>/dev/null \
            | grep '^ops-logs/.*\.txt$' | tail -1)"
  [ -n "$name" ] || return 1
  git -C "$remote" show "$branch:$name" 2>/dev/null
}

# Start a capture in its own process group, so a cancel can signal the whole
# group the way station.sh does rather than only the wrapper. Echoes the pid,
# which is also the process group id because setsid made it a leader.
drv_capture_bg() {
  local out="$1" script="$2"
  setsid bash -c '
    # shellcheck disable=SC1091
    . "$1/caplib.sh"
    cap_header "$2" "conformance-cancel"
    cap_run "$2" "$3"
    cap_footer "$2" $?
  ' _ "$_D_TOOLKIT" "$out" "$script" >/dev/null 2>&1 &
  printf '%s' "$!"
}
