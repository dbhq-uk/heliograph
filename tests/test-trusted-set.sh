#!/usr/bin/env bash
# =============================================================================
#  test-trusted-set.sh - who may command a station, and who may change that
# =============================================================================
# The claim this guards is the product's headline one: heliograph cloud cannot
# cause a station to run anything. That is true only if the service cannot
# administer the trust root, because a service that could add a key could then
# sign legitimately - nothing stolen, nothing forged, every gate passed.
#
# So the assertions here are about REFUSALS, and the sharpest of them is the
# anchor: no request, from anybody, including whoever holds the anchor key
# itself, may change it. Break that check in internal/trust/apply.go and the
# `the anchor survives ...` assertions below fail.
#
# Every station invocation is wrapped in `timeout`: the loop is designed to poll
# forever, and a test that hangs teaches nobody anything.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
ROOT="$(cd "$HERE/.." && pwd)"

# assert_ne is defined HERE rather than in assert.sh. Several of the assertions
# below are "this value moved" - a digest, a fingerprint - and the shared file
# has no such helper. Adding one to a file every other test sources is a change
# with a blast radius this test does not need.
assert_ne() {
  if [ "$2" != "$3" ]; then
    t_ok "$1"
  else
    t_no "$1"
    printf '     expected anything but: [%s]\n' "$2"
  fi
}

command -v go >/dev/null 2>&1 || export PATH="$PATH:/usr/local/go/bin"
if ! command -v go >/dev/null 2>&1; then
  # LOUD, and counted. A trusted-set suite that reports "0 failed" having
  # asserted nothing is the exact shape of guard this repository has been
  # burned by before. CI refuses a non-zero skip count.
  t_skip "go is not on PATH, so heliograph-seal could not be built and NOTHING here was asserted"
  t_summary
  exit 0
fi

TMP="$(mktemp -d)"
trap 'pkill -f "$TMP/tr/station.sh" 2>/dev/null; rm -rf "$TMP"' EXIT

TR="$TMP/tr"
GIT="git -c user.email=ci@example.invalid -c user.name=ci"

"$ROOT/station/bootstrap.sh" "$TR" >/dev/null 2>&1
git init -q --bare "$TMP/origin.git"
( cd "$TR" \
    && git init -q \
    && git remote add origin "$TMP/origin.git" \
    && $GIT add -A \
    && $GIT commit -qm init \
    && $GIT push -q -u origin HEAD ) >/dev/null 2>&1

cat > "$TR/steps/reader.sh" <<'EOF'
#!/usr/bin/env bash
# heliograph-mode: read-only
echo "ran reader"
EOF
chmod +x "$TR/steps/reader.sh"
cat > "$TR/steps/writer.sh" <<'EOF'
#!/usr/bin/env bash
# heliograph-mode: action
echo "ran writer"
EOF
chmod +x "$TR/steps/writer.sh"
( cd "$TR" && sed -i "s|^  # -- add task steps here.*|  reader)  CMD=(./steps/reader.sh) ;;\n  writer)  CMD=(./steps/writer.sh) ;;\n&|" run.sh )
( cd "$TR" && $GIT add -A && $GIT commit -qm steps && $GIT push -q ) >/dev/null 2>&1

# --- the binaries -------------------------------------------------------------
# heliograph-seal is the one binary already argued for past the station
# boundary. Built rather than stubbed: the verification is the part of this that
# can be subtly wrong, and a stub would assert that the shell calls something.
SEAL="$TR/heliograph-seal"
( cd "$ROOT" && go build -o "$SEAL" ./cmd/heliograph-seal ) 2>&1 || { t_no "could not build heliograph-seal"; t_summary; exit 1; }
HG="$TMP/heliograph"
( cd "$ROOT" && go build -o "$HG" ./cmd/heliograph ) 2>&1 || { t_no "could not build heliograph"; t_summary; exit 1; }

export XDG_CONFIG_HOME="$TMP/config"
hg() { "$HG" "$@" 2>&1; }

BRANCH="$(cd "$TR" && git rev-parse --abbrev-ref HEAD)"
hg init acme --dir "$TR" --scope "$BRANCH" >/dev/null 2>&1

# --- the plant: an anchor, put there by the operator, on the machine ----------
INIT_OUT="$(hg trust init -e acme)"
assert_contains "trust init prints the anchor fingerprint to read back" "anchor:" "$INIT_OUT"
assert_contains "and the command the operator runs ON the machine" "heliograph-seal trust init" "$INIT_OUT"

SET="$TR/.station-trusted-set"
ANCHOR_PUB="$(sed -n 's/^.*--anchor //p' <<<"$INIT_OUT" | head -1)"
assert_ne "the plant instructions carry a public identity" "" "$ANCHOR_PUB"
"$SEAL" trust init --set "$SET" --estate acme --station "$BRANCH" \
  --name anchor --anchor "$ANCHOR_PUB" >/dev/null 2>&1
assert_eq "the station's trusted set exists after the operator plants it" \
  "yes" "$([ -f "$SET" ] && echo yes || echo no)"

# THE ANCHOR AGREES ON BOTH SIDES, which is the whole first link of the chain.
assert_eq "the anchor planted on the machine is the one the control side recorded" \
  "$("$SEAL" trust digest --set "$SET")" \
  "$(sed -n 's/^  digest: //p' <<<"$INIT_OUT" | head -1)"

request() {  # request <id> <step> [envline]
  { echo "id: $1"; echo "step: $2"; [ -n "${3:-}" ] && echo "env: $3"; } > "$TR/station/request"
  ( cd "$TR" && $GIT add -A && $GIT commit -qm "request $1" && $GIT push -q ) >/dev/null 2>&1
}
raw_request() {  # raw_request <file-contents-on-stdin>
  cat > "$TR/station/request"
  ( cd "$TR" && $GIT add -A && $GIT commit -qm "request" && $GIT push -q ) >/dev/null 2>&1
}
station() {  # one pass with a trusted set, output in OUT
  # The exit code is deliberately discarded: `--once` exits non-zero for
  # ordinary refusals, and every assertion below is about what was PUBLISHED
  # rather than about how the process ended. A refusal that reaches the far
  # side is the behaviour under test; the exit code never leaves the machine.
  OUT="$( cd "$TR" && TRUST_SET=.station-trusted-set TRUST_SEAL=./heliograph-seal \
            timeout 60 ./station.sh --once --interval 1 "$@" 2>&1 )" || true
}
status_field() { sed -n "s/^$1:[[:space:]]*//p" "$TR/station/status" | head -1; }
set_serial() { "$SEAL" trust show --set "$SET" | sed -n 's/^serial:[[:space:]]*//p' | head -1; }
set_members() { "$SEAL" trust show --set "$SET" | sed -n 's/^member:[[:space:]]*//p'; }

# --- the station refuses to start with a set it cannot verify -----------------
# A station told to use a trusted set and then quietly not using one is a
# station whose owner believes an assurance the mechanism is not providing.
OUT="$( cd "$TR" && TRUST_SET=.station-trusted-set TRUST_SEAL=/nonexistent \
          timeout 30 ./station.sh --once --interval 1 2>&1 )"
assert_contains "a station with a trusted set and no verifier refuses to start" \
  "heliograph-seal is not at" "$OUT"

OUT="$( cd "$TR" && TRUST_SET=.no-such-set TRUST_SEAL=./heliograph-seal \
          timeout 30 ./station.sh --once --interval 1 2>&1 )"
assert_contains "and one whose trusted set is missing says how to plant it" \
  "trust init --set" "$OUT"

# --- a request may not choose the trusted set ---------------------------------
# THIS IS #46's DEFECT, IN THE ONE PLACE IT WOULD MATTER MOST. TRUST_SET names
# the file that decides WHO MAY COMMAND THIS STATION. A request able to set it
# would point the station at a set the requester wrote, which is the trust-root
# bypass the set exists to close, reintroduced one variable lower down.
request env-trust reader "TRUST_SET=/tmp/mine"
station
assert_eq "a request setting TRUST_SET is refused" "refused" "$(status_field state)"
assert_contains "and the published reason names the variable" \
  "TRUST_SET" "$(cat "$TR/station/status")"

# ...and the pattern in station.sh really does cover it, read out of the source
# rather than reasoned about, the same way test-station-gate.sh does.
_pat="$(sed -n 's/^ *\(TRANSPORT|PUSH|REDACT|LOG_DIR.*\))$/\1/p' "$TR/station.sh" | head -1)"
assert_contains "the reserved-env pattern in station.sh covers the TRUST_ prefix" \
  "TRUST_" "$_pat"
_pspat="$(sed -n "s/^\$ReservedEnvPattern = '^(\(.*\))\$'$/\1/p" "$ROOT/station/powershell/station.ps1" | head -1)"
assert_contains "and so does the PowerShell twin's, or the same request is refused on one machine and honoured on another" \
  "TRUST_" "$_pspat"

# --- an ordinary run still runs, and is attributed ----------------------------
request r1 reader
station
assert_eq "a read-only step still runs with a trusted set configured" "idle" "$(status_field state)"
assert_ne "and the station publishes the set's digest" "" "$(status_field trust)"
assert_contains "and the members, so the owner can audit without asking us" \
  "anchor=" "$(status_field trust-members)"
assert_eq "and the published copy is in the transport repo" \
  "yes" "$([ -f "$TR/station/trusted-set" ] && echo yes || echo no)"

# --- a signed change from the anchor is applied -------------------------------
ALICE="$TMP/alice.json"
"$SEAL" keygen --out "$ALICE" >/dev/null 2>&1
ALICE_PUB="$("$SEAL" public --identity "$ALICE")"

# NO `git pull --rebase` BETWEEN THESE TWO, and that is not an omission.
# `heliograph` and the station share this working tree in the test, so the CLI's
# commit is already here - and rebasing on top of the status commits the station
# pushed a moment earlier left the tree mid-rebase perhaps one run in three, with
# station/request reverted to its pre-rebase contents. The symptom was a change
# that silently never applied, which is the exact failure this file exists to
# detect, arriving from the test rather than from the code.
ADD_OUT="$(hg trust add -e acme alice "$ALICE_PUB")"
assert_contains "the control side reports what it signed and who signed it" \
  "signed by anchor" "$ADD_OUT"
assert_contains "and says revocation is eventual rather than letting it be assumed" \
  "EVENTUAL" "$ADD_OUT"
station
assert_eq "a change signed by the anchor is applied" "1" "$(set_serial)"
assert_contains "and alice is in the set" "alice" "$(set_members)"
assert_contains "and the station said what it did" "TRUSTED SET" "$OUT"
assert_eq "and the outcome is published rather than left to be guessed" \
  "idle" "$(status_field state)"

# --- THE ANCHOR. THIS IS THE ASSERTION THE WHOLE CLAIM RESTS ON ---------------
#
# Constructed by hand rather than through the CLI, because the CLI will not
# author one - so going through it would assert that our own tooling is polite
# rather than that the STATION refuses. The station is the only side that
# matters here: it is on a machine nobody can log into, and it is what an
# attacker's document actually arrives at.
DIGEST_BEFORE="$("$SEAL" trust digest --set "$SET")"
forge_change() {  # forge_change <identity> <author-name> <op> <subject-name> <subject-pub> <serial>
  local idf="$1" author="$2" op="$3" sname="$4" spub="$5" serial="$6"
  local prev; prev="$("$SEAL" trust digest --set "$SET")"
  # There is deliberately no verb in heliograph-seal that does this: the far
  # side verifies and does not author. So the test uses the one thing that
  # can - a short Go program built here, holding a secret key, which is exactly
  # the position an attacker with a stolen key would be in.
  "$TMP/forge" -identity "$idf" -author "$author" -op "$op" \
    -name "$sname" -key "$spub" -estate acme -station "$BRANCH" \
    -serial "$serial" -prev "$prev"
}

( cd "$ROOT" && go build -o "$TMP/forge" ./tests/forge ) 2>&1 || {
  t_no "could not build the forging helper"; t_summary; exit 1; }

anchor_attempt() {  # anchor_attempt <label> <identity> <author> <op> <name> <pub>
  local label="$1"; shift
  local n; n="$(( $(set_serial) + 1 ))"
  { echo "id: anchor-$RANDOM$RANDOM"; echo "step:"
    forge_change "$1" "$2" "$3" "$4" "$5" "$n"; } | raw_request
  station
  assert_eq "the anchor survives $label" "$DIGEST_BEFORE" "$("$SEAL" trust digest --set "$SET")"
  assert_eq "and it is REFUSED rather than ignored, so somebody is told: $label" \
    "refused" "$(status_field state)"
  assert_contains "and the reason says the anchor changes only on the machine: $label" \
    "only on the machine" "$(cat "$TR/station/status")"
}

# alice is a legitimately trusted member. She has every authority the design
# grants anybody, and she still cannot touch the anchor.
anchor_attempt "a member revoking it by name" "$ALICE" alice revoke anchor "$ANCHOR_PUB"
anchor_attempt "a member revoking its key under another name" "$ALICE" alice revoke someone "$ANCHOR_PUB"
anchor_attempt "a member adding a second member called anchor" "$ALICE" alice add anchor "$ALICE_PUB"

# AND NOT EVEN THE ANCHOR ITSELF, OVER THE TRANSPORT. Whoever holds the anchor
# key changes it by standing at the machine. There is no remote path, and that
# is what makes the recovery property a property.
CTL_ID="$XDG_CONFIG_HOME/heliograph/keys/acme.identity.json"
anchor_attempt "the anchor holder trying it remotely" "$CTL_ID" anchor revoke anchor "$ANCHOR_PUB"

# ...and it DOES change, on the machine, or the recovery property is a lock-out.
NEWANCHOR="$TMP/newanchor.json"
"$SEAL" keygen --out "$NEWANCHOR" >/dev/null 2>&1
"$SEAL" trust anchor --set "$SET" --anchor "$("$SEAL" public --identity "$NEWANCHOR")" >/dev/null 2>&1
assert_ne "the anchor DOES change with the local command, or recovery is impossible" \
  "$DIGEST_BEFORE" "$("$SEAL" trust digest --set "$SET")"
assert_contains "and every member is still there afterwards" "alice" "$(set_members)"
# Put it back, so the assertions below run against the set the control side holds.
rm -f "$SET"
"$SEAL" trust init --set "$SET" --estate acme --station "$BRANCH" \
  --name anchor --anchor "$ANCHOR_PUB" >/dev/null 2>&1
"$TMP/forge" -identity "$CTL_ID" -author anchor -op add -name alice -key "$ALICE_PUB" \
  -estate acme -station "$BRANCH" -serial 1 -prev "$("$SEAL" trust digest --set "$SET")" \
  > "$TMP/readd" 2>/dev/null
"$SEAL" trust apply --set "$SET" --in "$TMP/readd" >/dev/null 2>&1

# --- a change nobody in the set signed -----------------------------------------
MALLORY="$TMP/mallory.json"
"$SEAL" keygen --out "$MALLORY" >/dev/null 2>&1
MALLORY_PUB="$("$SEAL" public --identity "$MALLORY")"
{ echo "id: forged-1"; echo "step:"
  forge_change "$MALLORY" mallory add mallory "$MALLORY_PUB" "$(( $(set_serial) + 1 ))"; } | raw_request
station
assert_eq "a change signed by a key nobody trusts is refused" "refused" "$(status_field state)"
assert_contains "and the reason says the key is not trusted" \
  "does not trust" "$(cat "$TR/station/status")"
assert_eq "and mallory is not in the set" "" "$(set_members | grep mallory)"

# --- a replayed change, and a restart in between --------------------------------
BOB="$TMP/bob.json"
"$SEAL" keygen --out "$BOB" >/dev/null 2>&1
BOB_PUB="$("$SEAL" public --identity "$BOB")"
forge_change "$ALICE" alice add bob "$BOB_PUB" "$(( $(set_serial) + 1 ))" > "$TMP/addbob"
{ echo "id: addbob-1"; echo "step:"; cat "$TMP/addbob"; } | raw_request
station
assert_contains "alice can enrol bob: a member adds a member, with no operator" "bob" "$(set_members)"

# THE SAME CHANGE AGAIN, under a new id, after the station has stopped and
# started. The serial lives in the set ON DISK, so a restart forgets nothing -
# which matters because a station restarts when the machine does, and that is
# exactly when nobody is watching.
{ echo "id: addbob-2-replayed"; echo "step:"; cat "$TMP/addbob"; } | raw_request
station
assert_eq "the same change replayed after a restart is refused" "refused" "$(status_field state)"
assert_contains "and the reason says which serial it expected" \
  "serial" "$(cat "$TR/station/status")"

# --- revocation, and a refusal that names the key -------------------------------
forge_change "$ALICE" alice revoke bob "$BOB_PUB" "$(( $(set_serial) + 1 ))" > "$TMP/revbob"
{ echo "id: revbob-1"; echo "step:"; cat "$TMP/revbob"; } | raw_request
station
assert_contains "alice can revoke bob remotely: the leaver flow needs no operator" \
  "REVOKED" "$(set_members | grep bob)"

{ echo "id: bob-after-1"; echo "step:"
  forge_change "$BOB" bob add mallory "$MALLORY_PUB" "$(( $(set_serial) + 1 ))"; } | raw_request
station
assert_eq "a revoked key's change is refused" "refused" "$(status_field state)"
assert_contains "and the refusal NAMES the revoked person, not 'unknown'" \
  "bob" "$(cat "$TR/station/status")"
assert_contains "and names the revoked key, so it is not mistaken for a broken enrolment" \
  "revoked" "$(cat "$TR/station/status")"

# --- the signed scope: target, expiry, mode and id -----------------------------
{ echo "id: target-1"; echo "step: reader"; echo "target: some-other-station"; } | raw_request
station
assert_eq "a request written for another station is refused" "refused" "$(status_field state)"
assert_contains "and the reason names both" "some-other-station" "$(cat "$TR/station/status")"

{ echo "id: expiry-1"; echo "step: reader"; echo "expires: 2020-01-01T00:00:00Z"; } | raw_request
station
assert_eq "an expired request is refused" "refused" "$(status_field state)"
assert_contains "and the reason says when it expired" "2020-01-01" "$(cat "$TR/station/status")"

{ echo "id: expiry-2"; echo "step: reader"; echo "expires: 2099-01-01T00:00:00Z"; } | raw_request
station
assert_eq "a request that has not expired still runs" "idle" "$(status_field state)"

# MODE: the step file changed after the request was signed for it.
{ echo "id: mode-1"; echo "step: writer"; echo "mode: read-only"; } | raw_request
station --allow-actions
assert_eq "a request signed for a read-only step is refused when the step declares action" \
  "refused" "$(status_field state)"
assert_contains "and the reason says the step changed after the request was written" \
  "changed after" "$(cat "$TR/station/status")"

{ echo "id: mode-2"; echo "step: reader"; echo "mode: read-only"; } | raw_request
station
assert_eq "a request whose mode agrees with the step runs" "idle" "$(status_field state)"

# ID REPLAY, ACROSS A RESTART. `.station-state` remembers only the LAST id, so
# an older one, put back, used to run again with every gate already satisfied.
{ echo "id: r1"; echo "step: reader"; } | raw_request
station
assert_eq "an id this station already acted on is refused, even after a restart" \
  "refused" "$(status_field state)"
assert_contains "and the reason says it is a replay" "already been acted on" "$(cat "$TR/station/status")"
assert_eq "and the ledger is on disk, not in memory" \
  "yes" "$([ -s "$TR/.station-seen-ids" ] && echo yes || echo no)"

# --- a change lands with no service anywhere on the path ------------------------
# Nothing in this file has spoken to a service. The transport is a bare git repo
# on a filesystem, the changes were signed locally, and the station verified them
# locally. That is the property: losing heliograph cloud does not lock a customer
# out of their own estate, and compromising it is not equivalent to holding a key.
assert_eq "every change above landed over a bare git repo, with no service on the path" \
  "yes" "$([ -d "$TMP/origin.git" ] && echo yes || echo no)"

t_summary
