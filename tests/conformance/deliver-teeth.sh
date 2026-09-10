#!/usr/bin/env bash
# =============================================================================
#  deliver-teeth.sh - property 9 must FAIL when a transport delivers nothing
# =============================================================================
# Running the suite over three transports proves three transports pass. It does
# not prove the suite would notice if one of them stopped delivering, and that
# is the whole reason property 9 exists: `run.sh` used to finish with `cap_push`
# unconditionally, so a station on the relay or the blob captured a perfect log
# and shipped nothing at all, and every other property still passed.
#
# So each transport's `tp_put_log` is replaced with `return 0` - a delivery that
# claims success and does nothing, which is exactly the shape of the original
# defect - and p9 must fail. If it passes, p9 is checking the local file again
# and the coverage across transports is decoration.
#
# THE PAYLOAD IS MUTATED, NOT THE TOOLKIT. `bootstrap.sh` plants a copy, and
# that copy is what the station runs, so editing it changes this one station and
# cannot leak into the repository or another test running in parallel.
#
# Usage: deliver-teeth.sh <transport> [driver]   exits 0 when p9 correctly FAILED
#
# The driver defaults to bash. It matters because the MUTATION differs: the bash
# payload has `tp_put_log` in a sourced .sh, the PowerShell payload has
# `Send-TpLog` in a .psm1. Everything else about this check is the same, and it
# has to cover both - a second implementation of delivery is a second thing that
# can silently stop delivering.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
T="${1:?usage: deliver-teeth.sh <transport> [driver]}"
DRV="${2:-bash}"

export CONF_TRANSPORT="$T"
# shellcheck disable=SC1090,SC1091
. "$HERE/drivers/$DRV.sh"

drv_supports deliver || {
  printf 'deliver-teeth: %s cannot deliver here, so there is nothing to blunt\n' "$T"
  exit 2
}

WORK="$(mktemp -d)"
cleanup() {
  if declare -F drv_teardown >/dev/null 2>&1; then drv_teardown; fi
  rm -rf "$WORK"
}
trap cleanup EXIT

drv_bootstrap "$WORK/d" || { echo "deliver-teeth: bootstrap failed for $T"; exit 3; }

# APPENDED, NOT EDITED IN PLACE, in either language. A later definition wins in
# both bash and PowerShell, so this needs no knowledge of how the original is
# written - and a sed that silently matched nothing would leave the real one in
# place and make this whole check report a pass it never earned.
case "$DRV" in
  powershell)
    TP="$WORK/d/transports/$T.psm1"
    MARKER='function Send-TpLog { return $true }'
    ;;
  *)
    TP="$WORK/d/transports/$T.sh"
    MARKER='tp_put_log() { return 0; }'
    ;;
esac
[ -f "$TP" ] || { echo "deliver-teeth: no planted transport at $TP"; exit 3; }

{
  printf '\n# --- deliver-teeth.sh: the mutation ---\n'
  printf '%s\n' "$MARKER"
} >> "$TP"

# THE MUTATION IS CONFIRMED PRESENT. A `cat >>` into a read-only file, or into a
# path that turned out not to be the one the station loads, fails silently and
# would leave this whole check asserting that a WORKING transport delivered
# nothing - which it would then blame on the transport.
grep -qxF "$MARKER" "$TP" || {
  echo "deliver-teeth: the mutation did not reach $TP"
  exit 3
}

# A MARKER, because "nothing arrived" has two causes and only one of them is
# the one under test. If the step never ran at all - a bootstrap that planted a
# broken payload, a runner that refused the step, a gate that fired - the far
# side is empty for a reason that has nothing to do with tp_put_log, and this
# check would report a pass it did not earn. So the step proves it ran.
MARKER="$WORK/the-step-ran"
cat > "$WORK/d/steps/ships.sh" <<EOS
#!/usr/bin/env bash
# heliograph-mode: read-only
echo the evidence
: > "$MARKER"
exit 7
EOS
chmod +x "$WORK/d/steps/ships.sh"

drv_deliver "$WORK/d" steps/ships.sh
[ -f "$MARKER" ] || {
  printf 'deliver-teeth: %s - the step never ran, so an empty far side proves nothing.\n' "$T"
  exit 3
}

body="$(drv_delivered "$WORK/d")"

case "$body" in
  *"finished UTC"*)
    printf 'deliver-teeth: %s/%s DELIVERED a log with delivery neutered.\n' "$DRV" "$T"
    printf '  p9 is reading something other than the far side, so running the\n'
    printf '  suite over this transport proves nothing.\n'
    exit 1
    ;;
esac
printf 'deliver-teeth: %s/%s - the step ran, and a neutered delivery delivered nothing\n' "$DRV" "$T"
exit 0
