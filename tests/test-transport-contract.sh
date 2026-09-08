#!/usr/bin/env bash
# =============================================================================
#  test-transport-contract.sh - every transport implements the same verbs
# =============================================================================
# A3 moved every command that crosses the gap behind tp_*, so the loop has one
# copy of the gates rather than one per transport. That only holds while every
# transport actually implements the contract.
#
# A missing verb is not a syntax error in bash. Calling one is "command not
# found", which in a loop running unattended surfaces as the station going quiet
# on a machine nobody can reach. So it is checked here, where somebody is
# listening, rather than there.
#
# This asserts SHAPE, not behaviour. Behaviour against a real store needs that
# store, and test-pigeonhole.sh already covers the blob primitives.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
TOOLKIT="$(cd "$HERE/../station/bash" && pwd)"

# Required of every transport, no exceptions.
REQUIRED="tp_capabilities tp_init tp_scope tp_revision tp_describe tp_check
          tp_fetch_request tp_put_status tp_put_progress"

# Optional, but a transport that ADVERTISES one must define it. Advertising a
# verb you have not written is worse than not having it: the loop calls it.
OPTIONAL="self:tp_fetch_self live:tp_fetch_request_live"

# Source a transport in a subshell with the loop's globals stubbed, then run one
# command in it.
#
# The stubs matter: a transport that reads a loop global at source time has to
# fail here rather than at start on the far side. They are unused by this script
# itself, which is what SC2034 is about, and that is the point of them.
probe() {  # probe <transport.sh> <command...>
  local tp="$1"; shift
  (
    say() { :; }
    # shellcheck disable=SC2034
    BRANCH=""
    # shellcheck disable=SC2034
    STATUS=""
    # shellcheck disable=SC2034
    REQUEST=""
    # shellcheck disable=SC2034
    REPO_ROOT="$TOOLKIT"
    # shellcheck disable=SC1090
    . "$tp" 2>/dev/null || exit 1
    "$@" 2>/dev/null
  )
}

has_fn() { probe "$1" declare -F "$2" >/dev/null 2>&1 && echo yes || echo no; }

found=0
for tp in "$TOOLKIT"/transports/*.sh; do
  [ -f "$tp" ] || continue
  found=$((found + 1))
  name="$(basename "$tp" .sh)"

  caps="$(probe "$tp" tp_capabilities)"
  assert_eq "$name: sources cleanly and reports capabilities" \
    "0" "$([ -n "$caps" ] && echo 0 || echo 1)"

  for fn in $REQUIRED; do
    assert_eq "$name: defines $fn" "yes" "$(has_fn "$tp" "$fn")"
  done

  for pair in $OPTIONAL; do
    cap="${pair%%:*}"; fn="${pair##*:}"
    case " $caps " in
      *" $cap "*)
        assert_eq "$name: advertises '$cap', so it must define $fn" \
          "yes" "$(has_fn "$tp" "$fn")" ;;
      *)
        t_ok "$name: does not advertise '$cap', so $fn is not required" ;;
    esac
  done
done

# A green run against an empty directory would assert nothing at all, and would
# look exactly like a green run against every transport.
assert_eq "there were transports to check" "0" \
  "$([ "$found" -gt 0 ] && echo 0 || echo 1)"

# The blob transport must NOT claim self-update. There is no repository to pull
# and no working tree to replace, and a station that claimed it could update
# itself and then silently could not is the failure the capability mechanism
# exists to prevent.
case " $(probe "$TOOLKIT/transports/blob.sh" tp_capabilities) " in
  *" self "*) t_no "blob claims self-update, which it cannot do" ;;
  *) t_ok "blob does not claim self-update, which it cannot do" ;;
esac

t_summary
