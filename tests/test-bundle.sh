#!/usr/bin/env bash
# =============================================================================
#  test-bundle.sh - the transport for a gap nothing crosses but a person
# =============================================================================
# The conformance suite already runs over the bundle and asks whether the
# CAPTURE is honest. This file asks the questions it cannot.
#
# THE ONE THAT MATTERS MOST is where the log lands. Every other transport puts
# logs under `ops-logs/`; the control side's Bundle.ListLogs reads `*.txt` in
# the bundle directory ITSELF. A station writing them one level down produces a
# medium that is carried back perfectly and shows no logs at all - and this is
# the transport least able to recover from that, because the courier has
# already walked. Conformance would not notice: it reads wherever the driver
# tells it to look.
#
# So the round trip here is driven with the REAL CLI on the near side, not with
# a fixture. Two implementations of a layout on opposite sides of a gap that
# nobody can cross is exactly the disagreement this test exists to catch.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
ROOT="$(cd "$HERE/.." && pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

CLI=""
# A skip here loses the round trip, which is the point of the file - so look in
# the default install location too, rather than skipping on a machine that has
# Go but has not put it on a non-interactive PATH.
[ -x /usr/local/go/bin/go ] && PATH="$PATH:/usr/local/go/bin"
if command -v go >/dev/null 2>&1; then
  if ( cd "$ROOT" && go build -o "$TMP/heliograph" ./cmd/heliograph ) >/dev/null 2>&1; then
    CLI="$TMP/heliograph"
  fi
fi

# --- a planted station, and a medium to carry -------------------------------
D="$TMP/station"; MEDIUM="$TMP/stick"
mkdir -p "$MEDIUM"
"$ROOT/station/bootstrap.sh" "$D" >/dev/null 2>&1
cat > "$D/steps/probe.sh" <<'EOS'
#!/usr/bin/env bash
# heliograph-mode: read-only
echo "carried across the gap"
EOS
chmod +x "$D/steps/probe.sh"

station() {  # station <env...> -- <args...>
  local envs=() a
  while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do envs+=("$1"); shift; done
  shift || true
  a=("$@")
  ST_OUT="$( cd "$D" && env TRANSPORT=bundle ALLOW_ROOT=1 "${envs[@]+"${envs[@]}"}" \
               timeout 120 ./station.sh "${a[@]}" 2>&1 )"
  ST_RC=$?
}

# =============================================================================
#  1. THE ROUND TRIP, DRIVEN BY THE REAL CLI ON THE NEAR SIDE
# =============================================================================
if [ -z "$CLI" ]; then
  t_skip "no Go toolchain, so the CLI half of the round trip was NOT driven"
else
  export HELIOGRAPH_HOME="$TMP/estates"
  "$CLI" init air --transport bundle --dir "$MEDIUM" >/dev/null 2>&1
  SENT="$("$CLI" send steps/probe.sh -e air 2>&1)"
  assert_contains "the CLI writes a request bundle" "request-" "$(ls -1 "$MEDIUM")"
  # THE MESSAGE HAS BEEN WRONG TWICE: it named `./station.sh --bundle`, a flag
  # that never existed, and was then corrected to say there was no station side
  # at all. Both cost somebody an afternoon. It has to name what works now.
  assert_contains "  and tells the operator what to actually run" "TRANSPORT=bundle" "$SENT"
  assert_eq "  and no longer says there is no station side" "no" \
    "$(printf '%s' "$SENT" | grep -qi 'no station side' && echo yes || echo no)"

  # --- carry it across ---
  station "BUNDLE_DIR=$MEDIUM" -- --once --interval 1
  assert_eq "the station runs the carried request" "0" "$ST_RC"

  # --- carry it back ---
  assert_eq "the reply is a status on the medium" "yes" \
    "$([ -f "$MEDIUM/status" ] && echo yes || echo no)"
  # FLAT, NOT UNDER ops-logs/. This is the assertion the whole file is for.
  assert_eq "the log is FLAT on the medium, where the control side looks for it" "1" \
    "$(ls -1 "$MEDIUM"/*.txt 2>/dev/null | wc -l | tr -d ' ')"
  assert_eq "  and NOT under ops-logs/, which would travel back invisible" "no" \
    "$([ -d "$MEDIUM/ops-logs" ] && echo yes || echo no)"

  # AND THE CLI CAN ACTUALLY READ IT. Two implementations of a layout that have
  # never been driven against each other are two layouts.
  assert_contains "the CLI reads the status back" "idle" "$("$CLI" status -e air 2>&1)"
  assert_contains "the CLI lists the log" "probe-" "$("$CLI" logs -e air 2>&1)"
  assert_contains "  and reads its contents" "carried across the gap" \
    "$("$CLI" logs --last -e air 2>&1)"
fi

# =============================================================================
#  2. IT REFUSES WHAT WOULD TRAVEL BACK EMPTY
# =============================================================================
# Every refusal here is a case where the station works perfectly and the person
# carries away nothing - which is the failure this transport cannot recover
# from, because the next attempt costs another walk.
station BUNDLE_DIR= -- --once --interval 1
assert_contains "no BUNDLE_DIR is named, not guessed at" "BUNDLE_DIR" "$ST_OUT"

station "BUNDLE_DIR=$TMP/not-mounted" -- --once --interval 1
assert_contains "a directory that is not there is refused" "is not there" "$ST_OUT"
assert_contains "  and says why nothing is created for you" "mount point" "$ST_OUT"
assert_eq "  and nothing was created, so an unmounted stick stays visible as one" "no" \
  "$([ -d "$TMP/not-mounted" ] && echo yes || echo no)"

ln -s "$TMP" "$TMP/linked" 2>/dev/null
if [ -L "$TMP/linked" ]; then
  station "BUNDLE_DIR=$TMP/linked" -- --once --interval 1
  assert_contains "a symlinked medium is refused" "symlink" "$ST_OUT"
  assert_contains "  because the replies must be ON the thing somebody carries" \
    "ON the medium" "$ST_OUT"
else
  t_skip "could not create a symlink here, so the linked-medium refusal was NOT exercised"
fi

# =============================================================================
#  3. IT DECLARES ONLY WHAT A COURIER CHANNEL CAN DO
# =============================================================================
# A capability is a promise the loop holds the transport to: it calls what is
# declared. Claiming `live` would make the loop re-read a file that cannot
# change, once per interval, for ever; claiming `self` would promise an update
# nothing publishes.
CAPS="$( cd "$D" && bash -c '
  . ./caplib.sh 2>/dev/null
  . ./transports/bundle.sh
  tp_capabilities' 2>/dev/null )"
assert_contains "it declares it can carry a request" "request" "$CAPS"
assert_contains "  and a status" "status" "$CAPS"
assert_eq "  and NOT a live read, because a stick does not change while you watch it" "no" \
  "$(printf '%s' "$CAPS" | grep -qw live && echo yes || echo no)"
assert_eq "  and NOT self-update, because nothing publishes a payload to a bundle" "no" \
  "$(printf '%s' "$CAPS" | grep -qw self && echo yes || echo no)"

# THE UNDECLARED VERBS ARE STILL DEFINED, so calling one is a refusal rather
# than "command not found" - which reads as a broken payload rather than as a
# transport saying no.
for fn in tp_fetch_request_live tp_fetch_self; do
  DEFINED="$( cd "$D" && bash -c "
    . ./caplib.sh 2>/dev/null
    . ./transports/bundle.sh
    declare -F $fn >/dev/null && echo yes || echo no" 2>/dev/null )"
  assert_eq "$fn is defined so that calling it refuses rather than crashing" "yes" "$DEFINED"
done

# =============================================================================
#  4. THE PREFLIGHT SAYS THE THING NOBODY WILL WORK OUT
# =============================================================================
# A station on this transport polls a directory that only changes when somebody
# plugs something in. Started against an empty one it sits there behaving
# perfectly for ever, and that is indistinguishable from a station nobody uses.
EMPTY="$TMP/empty"; mkdir -p "$EMPTY"
PRE="$( cd "$D" && env TRANSPORT=bundle "BUNDLE_DIR=$EMPTY" ALLOW_ROOT=1 \
          timeout 60 ./start.sh --check 2>&1 )"
assert_contains "the preflight says no bundle is waiting" "no bundle is waiting" "$PRE"
assert_contains "  and that this is the transport working, not a fault" "as intended" "$PRE"
assert_contains "  and that it is not a round trip" "not a round trip" "$PRE"
assert_contains "  and proves it can write to the medium" "bundle write" "$PRE"

# --check MUST CHANGE NOTHING, which on removable media matters more than
# usual: an operator runs it on a machine where they may alter nothing yet.
LEFT="$(ls -A "$EMPTY" | wc -l | tr -d ' ')"
assert_eq "and --check left the medium exactly as it found it" "0" "$LEFT"

t_summary
