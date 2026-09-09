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

# git and share need nothing. The relay needs a Go toolchain to build
# heliograph-seal from this tree and python3 for the stub relay - both present
# in CI, and both named here so a machine without them says which.
COVERED="git share relay"

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
