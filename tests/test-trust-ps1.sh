#!/usr/bin/env bash
# =============================================================================
#  test-trust-ps1.sh - the PowerShell trusted set, against the Go side's bytes
# =============================================================================
# The station has two implementations and a control side cannot tell which one
# answered. If they disagreed about who may command a machine, the same signed
# change would be applied on one station and refused on another - and an estate
# owner's audit would be right about half their estate with no way to know which
# half.
#
# So this runs tests/trust-vectors.ps1, which reads the golden vectors written
# by internal/trust/vectors_test.go and checks that the PowerShell side produces
# the same canonical bytes, the same digests, the same signing input and THE
# SAME REFUSALS - the whole sentence, not a substring, because two stations
# giving different reasons for the same decision is a support call nobody can
# close.
#
# It caught exactly that on its first run: the anchor refusal here named
# `heliograph-seal trust anchor`, a binary the PowerShell station does not have
# and never will, because its verification is managed C#.
#
# WHAT THIS IS NOT. It does not drive a station. tests/test-station-ps1.sh and
# tests/test-station-loop-ps1.sh do that. This is the format comparison, and it
# is separate because a format that differs is a different failure from a loop
# that misbehaves, and they want different first moves.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
ROOT="$(cd "$HERE/.." && pwd)"

PS=""
for c in pwsh powershell; do
  command -v "$c" >/dev/null 2>&1 && { PS="$c"; break; }
done
if [ -z "$PS" ]; then
  # LOUD AND COUNTED. A cross-implementation check that reports "0 failed"
  # having compared nothing is the exact shape of guard this repository has been
  # burned by, and CI refuses a non-zero skip count.
  t_skip "no pwsh or powershell on PATH, so the two implementations were NOT compared"
  t_summary
  exit 0
fi

VEC="$ROOT/tests/fixtures/trust-vectors.json"
assert_eq "the golden vectors are committed, or there is nothing to compare against" \
  "yes" "$([ -f "$VEC" ] && echo yes || echo no)"
[ -f "$VEC" ] || { t_summary; exit 1; }

# THE VECTORS MUST STILL DESCRIBE THE GO SIDE. Otherwise this compares
# PowerShell against a fixture that has drifted from the implementation it was
# supposed to pin, and both halves pass while neither is right.
if command -v go >/dev/null 2>&1 || [ -x /usr/local/go/bin/go ]; then
  export PATH="$PATH:/usr/local/go/bin"
  if ( cd "$ROOT" && go test ./internal/trust/ -run TestTrustGoldenVectors -count=1 >/dev/null 2>&1 ); then
    t_ok "the committed vectors still describe the Go implementation"
  else
    t_no "the committed vectors no longer describe the Go implementation, so this comparison is against a stale fixture"
    printf '     regenerate deliberately: go test ./internal/trust/ -run TestTrustGoldenVectors -update\n'
  fi
else
  t_skip "go is not on PATH, so the vectors were not checked against the Go side first"
fi

# `< /dev/null`, because a PowerShell script that hits a mandatory parameter it
# cannot bind PROMPTS, and a prompt in CI is a job that hangs until the runner
# times it out rather than a test that fails.
OUT="$(cd "$ROOT" && timeout 600 "$PS" -NoProfile -NonInteractive \
        -File tests/trust-vectors.ps1 < /dev/null 2>&1)"
RC=$?

printf '%s\n' "$OUT" | sed 's/^/    /'

assert_eq "the PowerShell trusted set agrees with the Go side on every vector" "0" "$RC"

# The counts are read out of the script's own summary rather than inferred from
# the exit code: a script that died before asserting anything also exits
# non-zero, and "0 passed, 0 failed" must not read as agreement.
SUM="$(printf '%s\n' "$OUT" | sed -n 's/^trust-vectors.ps1: //p' | tail -1)"
assert_ne_zero() {
  if [ "${2:-0}" -gt 0 ]; then t_ok "$1"; else t_no "$1"; printf '     summary was: [%s]\n' "$SUM"; fi
}
assert_ne_zero "and it actually compared something rather than exiting early" \
  "$(printf '%s' "$SUM" | sed -n 's/^\([0-9]\{1,\}\) passed.*/\1/p')"

t_summary
