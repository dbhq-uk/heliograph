#!/usr/bin/env bash
# =============================================================================
#  test-conformance.sh - run the conformance suite against every driver
# =============================================================================
# The real driver must pass. The mutant driver must FAIL, and the second half
# is not ceremony: nothing else in this repository can tell a suite that checks
# the capture apart from a suite that has quietly stopped checking anything,
# because both report a clean run.
#
# ACROSS EVERY TRANSPORT, because the properties are about the method and the
# method is not supposed to depend on the channel. Properties 1 to 8 look at
# the captured file and pass identically everywhere; property 9 asks whether
# the log ARRIVED, and that is the one that used to be true on git alone.
#
# Then the same inversion again, per transport: a `tp_put_log` that returns 0
# and does nothing must make p9 fail. Without that, running the suite three
# times over would only prove it three times as unable to notice.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"

DRIVER="$HERE/conformance/drivers/bash.sh"

# git, share and bundle need nothing. The relay needs a Go toolchain to build
# heliograph-seal from this tree and python3 for the stub relay - both present
# in CI, and both named here so a machine without them says which.
#
# THE BUNDLE IS IN HERE DESPITE NOT BEING A LOOP. A courier channel cannot be
# polled and cannot carry a cancel, so it declares neither `live` nor `self`
# and the suite skips what depends on them by name. Everything else - the
# stamps, the exit code, the footer, the gates, the redaction, and above all
# that the finished log reaches the far side - is exactly the same question,
# and the answer has to be the same or the method does not survive the walk.
COVERED="git share bundle relay"

# blob is excluded on purpose: its far side is an Azure storage account, and
# there is no honest way to stand one up offline. Excluded WITH A REASON that is
# asserted below, rather than quietly omitted - a transport nobody remembers is
# unchecked is indistinguishable from one nobody checks.
EXCLUDED="blob"

for t in $COVERED; do
  if CONF_TRANSPORT="$t" "$HERE/conformance/conformance.sh" "$DRIVER"; then
    t_ok "the bash toolkit passes the conformance suite over $t"
  else
    t_no "the bash toolkit FAILS the conformance suite over $t"
  fi
done

# The teeth check, per transport. See conformance/deliver-teeth.sh.
for t in $COVERED; do
  "$HERE/conformance/deliver-teeth.sh" "$t" >/dev/null 2>&1
  case $? in
    0) t_ok "p9 over $t notices a tp_put_log that delivers nothing" ;;
    2) t_skip "p9 teeth over $t: this transport cannot deliver from here" ;;
    *) t_no "p9 over $t did NOT notice a tp_put_log that delivers nothing" ;;
  esac
done

# EVERY TRANSPORT IS ACCOUNTED FOR - covered, or excluded for a stated reason.
#
# Asserted rather than skipped. A SKIP line here every run would train people to
# ignore the one signal that means something, and it would still say nothing
# about a FIFTH transport added next month: that one would simply not appear on
# either list, and the suite would go green having never heard of it.
unaccounted=""
for f in "$HERE/../station/bash/transports/"*.sh; do
  [ -e "$f" ] || continue
  name="$(basename "$f" .sh)"
  case " $COVERED $EXCLUDED " in
    *" $name "*) ;;
    *) unaccounted="$unaccounted $name" ;;
  esac
done
if [ -z "$unaccounted" ]; then
  t_ok "every transport is either conformance-tested or excluded with a reason"
else
  t_no "these transports are on neither list:$unaccounted"
  printf '     A transport nobody remembers is untested reads exactly like one\n'
  printf '     that passes. Add it to COVERED, or to EXCLUDED with the reason.\n'
fi

# And the exclusion is on the SITE, where somebody deciding on a transport will
# read it, rather than only in a comment in a test they will never open.
for t in $EXCLUDED; do
  assert_contains "the site says $t is not conformance-tested" \
    "$t" "$(sed -n '/not conformance-tested/,/^$/p' "$HERE/../site/content/conformance.md" 2>/dev/null)"
done

# --- the SECOND implementation ------------------------------------------------
# AGENTS.md permits one only while it passes this suite. caplib.psm1 is the
# capture and run.ps1 the runner, so only property 9 - delivery - has nothing
# to answer it yet, and it SKIPS by name. On Windows p8 skips too: Git-Bash has
# no `setsid` and a Windows cancel needs a Job Object.
#
# The skips are counted rather than tolerated. "2 skipped" is the honest state
# of a half-built implementation; three would mean something stopped being
# checked without anybody deciding to stop checking it.
PS_DRIVER="$HERE/conformance/drivers/powershell.sh"
# OVER EVERY TRANSPORT IT SHIPS, for the same reason the bash driver is: the
# properties are about the method and the method must not depend on the channel.
PS_COVERED="git share"
ps_rc=0
ps_out=""
for t in $PS_COVERED; do
  one="$(CONF_TRANSPORT="$t" "$HERE/conformance/conformance.sh" "$PS_DRIVER" 2>&1)"
  [ $? -eq 0 ] || ps_rc=1
  ps_out="$ps_out
$one"
done
# INDENTED, because CI refuses a line starting with SKIP in this file's output
# and these two are legitimate - they are the ones the assertion below counts.
# Left at the margin they would fail the build for being honest, and the fix
# somebody would reach for is to stop printing them.
printf '%s\n' "$ps_out" | sed 's/^/  /'

if printf '%s' "$ps_out" | grep -q 'no PowerShell here'; then
  t_no "no PowerShell interpreter, so the second implementation was NOT exercised"
elif [ "$ps_rc" = "0" ]; then
  t_ok "the PowerShell station passes every property it claims, over $PS_COVERED"
else
  t_no "the PowerShell station FAILS the conformance suite"
fi

# WHICH properties skipped, not how many.
#
# A count was the first version and it was wrong on Windows, where Git-Bash has
# no `setsid` so the cancel property legitimately skips too - three, not two.
# Loosening the count to "2 or 3" would have accepted a THIRD skip anywhere,
# including a capture property quietly dropping out. So the skippable ones are
# named, and everything else must be answered.
PS_MAY_SKIP='p8'
ps_bad_skip="$(printf '%s' "$ps_out" | grep '^SKIP' | grep -vE "SKIP (${PS_MAY_SKIP}):" || true)"
if [ -z "$ps_bad_skip" ]; then
  t_ok "and it skipped nothing it is not allowed to - only cancel, where it cannot signal a group"
else
  t_no "caplib.psm1 skipped a property that is not on the allowed list:"
  printf '%s\n' "$ps_bad_skip" | sed 's/^/     /'
fi

# And the capture properties are ANSWERED. This is the half a name-based check
# needs: without it, a driver that skipped everything would have no disallowed
# skip either.
for prop in p1 p2 p3 p4 p5 p6 p7 p9 p10; do
  if printf '%s' "$ps_out" | grep -qE "^ok +${prop}:"; then :; else
    t_no "caplib.psm1 did not answer $prop, which is a property it claims"
  fi
done
t_ok "and every property was answered - p1 to p7, p9 and p10"

# THE SAME INVERSION FOR THE SECOND IMPLEMENTATION. Running its suite over two
# transports proves two passes; it says nothing about whether the suite would
# notice either of them silently stopping. A second implementation of delivery
# is a second thing that can stop delivering.
for t in git share; do
  "$HERE/conformance/deliver-teeth.sh" "$t" powershell >/dev/null 2>&1
  case $? in
    0) t_ok "p9 over $t notices a PowerShell Send-TpLog that delivers nothing" ;;
    2) t_skip "p9 teeth over $t (powershell): this transport cannot deliver from here" ;;
    *) t_no "p9 over $t did NOT notice a PowerShell Send-TpLog that delivers nothing" ;;
  esac
done

# --- the two implementations write the SAME log -------------------------------
# The control side parses these logs, and it parses one format. Two capture
# implementations that each pass every property can still disagree about the
# shape of a header, and nothing above would notice: the properties assert what
# a log MEANS, and this asserts what it LOOKS LIKE.
if [ -n "$(command -v pwsh || command -v powershell)" ]; then
  SHAPE="$(mktemp -d)"
  shape_of() {  # the log's structure, with every value that legitimately differs removed
    sed -E \
      -e 's/^[0-9]{2}:[0-9]{2}:[0-9]{2} \| /TIME | /' \
      -e 's/^( started UTC : ).*/\1TIME/' \
      -e 's/^( finished UTC : ).*/\1TIME/' \
      -e 's/^( control node: ).*/\1HOST/' \
      -e 's/^( user        : ).*/\1USER/' \
      -e 's/^( git branch  : ).*/\1BRANCH/' \
      -e 's/^( git commit  : ).*/\1COMMIT/' \
      "$1"
  }
  (
    # shellcheck disable=SC1091
    . "$HERE/conformance/drivers/bash.sh"
    drv_step_file rc42 "$SHAPE/step"
    drv_capture "$SHAPE/bash.log" "$SHAPE/step"
  ) >/dev/null 2>&1
  (
    # shellcheck disable=SC1090,SC1091
    . "$PS_DRIVER"
    drv_step_file rc42 "$SHAPE/step"
    drv_capture "$SHAPE/ps.log" "$SHAPE/step"
  ) >/dev/null 2>&1

  # BOTH LOGS ARE REQUIRED TO EXIST AND TO SAY SOMETHING FIRST.
  #
  # `diff` of two empty streams is equal, and process substitution hides the
  # exit status of the `sed` that produced them - so if neither capture ran,
  # this reported "exactly the same shape" for two logs that were not there.
  # Two failures agreeing is not agreement.
  shape_ready=1
  for f in "$SHAPE/bash.log" "$SHAPE/ps.log"; do
    for want in 'TIME | working' 'exit code    : 42' 'RESULT       : FAILED'; do
      grep -qF "${want#TIME | }" "$f" 2>/dev/null || {
        t_no "the shape fixture produced no usable log at $(basename "$f"): missing [$want]"
        shape_ready=0
        break 2
      }
    done
  done

  if [ "$shape_ready" = "1" ]; then
    if diff -u <(shape_of "$SHAPE/bash.log") <(shape_of "$SHAPE/ps.log") > "$SHAPE/diff" 2>&1; then
      t_ok "both implementations write a log of exactly the same shape"
    else
      t_no "the two implementations write DIFFERENT logs:"
      sed 's/^/     /' "$SHAPE/diff" | head -20
    fi
  fi
  rm -rf "$SHAPE"
fi

# --- the stub enforces the relay's own rules ---------------------------------
# There are two doubles of this relay in the repository - this one, and the Go
# one in cmd/heliograph/e2e_relay_test.go that the CLI tests use. That is
# deliberate: the Go tests can start an httptest server in-process and the bash
# suite cannot. But two doubles is two chances to drift, and a double that is
# more permissive than the thing it stands in for is worse than none: it turns
# a station-side regression into a green run.
#
# So the rules are ASSERTED against the stub, behaviourally, rather than
# assumed from having written it. The token scopes are asymmetric on purpose -
# a station may collect a request and publish a log, and may not queue a
# request even for itself - and getting that backwards in a client is exactly
# the mistake worth catching.
if command -v python3 >/dev/null 2>&1; then
  STUB_DIR="$(mktemp -d)"
  python3 "$HERE/conformance/relay-stub.py" ctl stn est 0 > "$STUB_DIR/port" 2>/dev/null &
  STUB_PID=$!
  for _ in $(seq 1 100); do
    STUB_PORT="$(cat "$STUB_DIR/port" 2>/dev/null)"
    case "$STUB_PORT" in
      '' | *[!0-9]*) ;;
      *) curl -sS -o /dev/null -m 2 "http://127.0.0.1:$STUB_PORT/health" 2>/dev/null && break ;;
    esac
    kill -0 "$STUB_PID" 2>/dev/null || break
    sleep 0.1
  done

  code() {  # code <method> <token> <path>
    curl -sS -o /dev/null -w '%{http_code}' -m 5 -X "$1" \
      ${3:+-H "Authorization: Bearer $2"} \
      ${4:+--data-binary "$4"} \
      "http://127.0.0.1:$STUB_PORT$3" 2>/dev/null
  }
  U=/v1/est/s

  assert_eq "the stub lets the station publish a log" "202" \
    "$(code POST stn "$U/s2c" '{"seq":1,"body":"x"}')"
  assert_eq "and REFUSES the control token publishing one, which is the station's move" \
    "401" "$(code POST ctl "$U/s2c" '{"seq":1,"body":"x"}')"
  assert_eq "the stub lets control queue a request" "202" \
    "$(code POST ctl "$U/c2s" '{"seq":1,"body":"x"}')"
  assert_eq "and REFUSES the station queueing one, even for itself" "401" \
    "$(code POST stn "$U/c2s" '{"seq":1,"body":"x"}')"
  assert_eq "the stub lets control collect a log" "200" "$(code GET ctl "$U/s2c")"
  assert_eq "and REFUSES the station collecting its own s2c" "401" \
    "$(code GET stn "$U/s2c")"
  assert_eq "an unknown estate is 404, not a queue of its own" "404" \
    "$(code GET ctl "/v1/other/s/s2c")"
  assert_eq "a direction that is neither c2s nor s2c is 404" "404" \
    "$(code GET ctl "$U/sideways")"
  # A client regression from GET to DELETE would collect-and-delete against a
  # stub that treated "not POST" as a read, and be refused by the real relay.
  assert_eq "DELETE is 405: only the two methods the API has" "405" \
    "$(code DELETE ctl "$U/s2c")"

  kill "$STUB_PID" 2>/dev/null
  wait "$STUB_PID" 2>/dev/null
  rm -rf "$STUB_DIR"
else
  t_no "python3 is absent, so the relay stub's rules were not checked"
fi

if "$HERE/conformance/mutant-check.sh"; then
  t_ok "the mutant driver fails the suite, so the suite has teeth"
else
  t_no "the mutant driver PASSED the suite - the suite is not checking"
fi

t_summary
