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
# Usage: deliver-teeth.sh <transport>   exits 0 when p9 correctly FAILED
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
T="${1:?usage: deliver-teeth.sh <transport>}"

export CONF_TRANSPORT="$T"
# shellcheck disable=SC1091
. "$HERE/drivers/bash.sh"

drv_supports deliver || {
  printf 'deliver-teeth: %s cannot deliver here, so there is nothing to blunt\n' "$T"
  exit 2
}

WORK="$(mktemp -d)"
cleanup() {
  if declare -F drv_teardown >/dev/null 2>&1; then drv_teardown "$WORK/d"; fi
  rm -rf "$WORK"
}
trap cleanup EXIT

drv_bootstrap "$WORK/d" || { echo "deliver-teeth: bootstrap failed for $T"; exit 3; }

TP="$WORK/d/transports/$T.sh"
[ -f "$TP" ] || { echo "deliver-teeth: no planted transport at $TP"; exit 3; }

# Appended, not edited in place. A later definition of a shell function wins, so
# this needs no knowledge of how the original is written - and a sed that
# silently matched nothing would leave the real one in place and make this whole
# check report a pass it never earned.
cat >> "$TP" <<'EOS'

# --- deliver-teeth.sh: the mutation ------------------------------------------
tp_put_log() { return 0; }
EOS

cat > "$WORK/d/steps/ships.sh" <<'EOS'
#!/usr/bin/env bash
# heliograph-mode: read-only
echo the evidence
exit 7
EOS
chmod +x "$WORK/d/steps/ships.sh"

drv_deliver "$WORK/d" steps/ships.sh
body="$(drv_delivered "$WORK/d")"

case "$body" in
  *"finished UTC"*)
    printf 'deliver-teeth: %s DELIVERED a log with a neutered tp_put_log.\n' "$T"
    printf '  p9 is reading something other than the far side, so running the\n'
    printf '  suite over this transport proves nothing.\n'
    exit 1
    ;;
esac
printf 'deliver-teeth: %s - a neutered tp_put_log delivers nothing, as it must\n' "$T"
exit 0
