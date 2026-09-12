#!/usr/bin/env bash
# =============================================================================
#  test-objstore.sh - the S3 transport, and the signature nothing will explain
# =============================================================================
# THE PROBLEM THIS FILE EXISTS FOR. Every S3-compatible store refuses a wrong
# signature with a 403 that names nothing - deliberately, because saying which
# part disagreed would be an oracle. So from the station, all of these look
# identical:
#
#   the secret is wrong          the region is wrong
#   the clock is wrong           the canonical form is wrong
#   the key has no s3:PutObject  the bucket does not exist
#
# A round trip against a real store would prove the whole chain and is not
# available: there is no account. So the signer is held to
# `tests/fixtures/sigv4-vectors.json`, emitted by the Go implementation, and
# EVERY STAGE is compared separately - canonical request, its hash, the string
# to sign, all four derived keys, the signature. A disagreement then says which
# stage, which is the thing a 403 will never tell anybody.
#
# The transport's own behaviour is driven against a fake curl, the way
# test-transport-contract.sh drives blob and relay.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
ROOT="$(cd "$HERE/.." && pwd)"
TOOLKIT="$ROOT/station/bash"
VECTORS="$ROOT/tests/fixtures/sigv4-vectors.json"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# =============================================================================
#  1. THE SIGNER, AGAINST GO'S OWN BYTES
# =============================================================================
if ! command -v python3 >/dev/null 2>&1; then
  t_skip "no python3 to read the vector file, so the SIGNER was NOT checked"
elif [ ! -f "$VECTORS" ]; then
  t_no "the committed SigV4 vectors are missing from $VECTORS"
elif ! command -v openssl >/dev/null 2>&1; then
  t_no "no openssl, so the station could not sign anything at all"
else
  # The vector file gives a fixed clock. `date -u` is overridden for the whole
  # comparison, because a signature covers the timestamp and a signer checked
  # against "now" is a signer checked against nothing.
  read -r V_KEYID V_SECRET V_REGION V_NOW V_COUNT <<EOF
$(python3 -c '
import json,sys
v=json.load(open(sys.argv[1]))
print(v["accessKey"], v["secretKey"], v["region"], v["now"], len(v["cases"]))' "$VECTORS")
EOF
  assert_eq "the vector file carries cases, or nothing below asserted anything" "yes" \
    "$([ "${V_COUNT:-0}" -ge 4 ] && echo yes || echo no)"

  FAKE="$TMP/bin"; mkdir -p "$FAKE"
  # `date -u +%Y%m%dT%H%M%SZ` is what _obj_sign calls. Pinned to the vector's
  # instant, and ONLY for that format - anything else falls through to the real
  # date, so this cannot silently freeze the station's other clocks.
  cat > "$FAKE/date" <<EOS
#!/usr/bin/env bash
if [ "\$*" = "-u +%Y%m%dT%H%M%SZ" ]; then printf '%s\n' "$V_NOW"; exit 0; fi
exec /usr/bin/env -i PATH=/usr/bin:/bin date "\$@"
EOS
  chmod +x "$FAKE/date"

  pass=0; fail=0; firstfail=""
  for i in $(seq 0 $((V_COUNT - 1))); do
    # Each field on its own line, because a canonical request CONTAINS newlines
    # and reading it field-per-line would split it.
    name="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["cases"][int(sys.argv[2])]["name"])' "$VECTORS" "$i")"
    method="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["cases"][int(sys.argv[2])]["method"])' "$VECTORS" "$i")"
    # The DECODED path and the canonical query, which is what _obj_sign takes -
    # matching Go, where u.Path and u.Query() are both decoded.
    # ONE FIELD PER LINE, not two words on one. A DECODED path can contain a
    # SPACE - the `path-needing-escapes` case exists precisely because it does -
    # and `read -r a b` would split it there, handing the signer half a path and
    # calling the rest a query string. That read as a signer fault.
    dpath="$(python3 -c '
import json,sys,urllib.parse as up
c=json.load(open(sys.argv[1]))["cases"][int(sys.argv[2])]
sys.stdout.write(up.unquote(up.urlsplit(c["url"]).path)+"#")' "$VECTORS" "$i")"
    dpath="${dpath%\#}"
    cquery="$(python3 -c '
import json,sys,urllib.parse as up
c=json.load(open(sys.argv[1]))["cases"][int(sys.argv[2])]
q=up.parse_qs(up.urlsplit(c["url"]).query, keep_blank_values=True)
parts=[]
for k in sorted(q):
    for v in sorted(q[k]):
        parts.append(up.quote(k, safe="~")+"="+up.quote(v, safe="~"))
sys.stdout.write("&".join(parts))' "$VECTORS" "$i")"
    # THE SENTINEL IS NOT DECORATION. `$( )` strips every trailing newline, and
    # the put-log body ENDS in one - so without this the payload hash differed
    # and the failure read as a broken signer rather than a broken harness.
    body="$(python3 -c 'import json,sys;sys.stdout.write(json.load(open(sys.argv[1]))["cases"][int(sys.argv[2])]["body"]+"#")' "$VECTORS" "$i")"
    body="${body%\#}"
    want_auth="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["cases"][int(sys.argv[2])]["authorization"])' "$VECTORS" "$i")"
    want_canon="$(python3 -c 'import json,sys;sys.stdout.write(json.load(open(sys.argv[1]))["cases"][int(sys.argv[2])]["canonicalRequest"])' "$VECTORS" "$i")"
    want_sts="$(python3 -c 'import json,sys;sys.stdout.write(json.load(open(sys.argv[1]))["cases"][int(sys.argv[2])]["stringToSign"])' "$VECTORS" "$i")"

    host="$(python3 -c '
import json,sys,urllib.parse as up
print(up.urlsplit(json.load(open(sys.argv[1]))["cases"][int(sys.argv[2])]["url"]).netloc)' "$VECTORS" "$i")"
    # Content-Type is a SIGNED header when it is sent. The Go client sets it on
    # every write, so a case carrying one is a case the station must match.
    ctype="$(python3 -c '
import json,sys
print(json.load(open(sys.argv[1]))["cases"][int(sys.argv[2])].get("header",{}).get("Content-Type",""))' "$VECTORS" "$i")"

    got="$(
      export PATH="$FAKE:$PATH"
      # shellcheck disable=SC1091
      . "$TOOLKIT/caplib.sh" 2>/dev/null
      # shellcheck disable=SC1091
      . "$TOOLKIT/transports/objstore.sh"
      # Read by _obj_sign, which is called below rather than through tp_init.
      # shellcheck disable=SC2034
      OBJSTORE_SECRET="$V_SECRET" OBJSTORE_KEY_ID="$V_KEYID"
      # shellcheck disable=SC2034
      OBJ_REGION="$V_REGION" OBJ_HOST="$host"
      _obj_sign "$method" "$dpath" "$cquery" \
        "$(printf '%s' "$body" | openssl dgst -sha256 -hex | awk '{print $NF}')" "$ctype"
      printf '%s\n' "---CANON---"; printf '%s\n' "$OBJ_CANONICAL"
      printf '%s\n' "---STS---";   printf '%s\n' "$OBJ_STS"
      printf '%s\n' "---AUTH---";  printf '%s\n' "$OBJ_AUTH"
    )"
    got_canon="$(printf '%s' "$got" | sed -n '/^---CANON---$/,/^---STS---$/p' | sed '1d;$d')"
    got_sts="$(printf '%s' "$got" | sed -n '/^---STS---$/,/^---AUTH---$/p' | sed '1d;$d')"
    got_auth="$(printf '%s' "$got" | sed -n '/^---AUTH---$/,$p' | sed '1d')"

    # STAGE BY STAGE, and the first one that differs is the one reported. The
    # later stages all differ once an early one does, so printing them all
    # buries the answer.
    if [ "$got_canon" != "$want_canon" ]; then
      fail=$((fail + 1)); [ -n "$firstfail" ] || firstfail="$name: canonical request
     want [$want_canon]
     got  [$got_canon]"
    elif [ "$got_sts" != "$want_sts" ]; then
      fail=$((fail + 1)); [ -n "$firstfail" ] || firstfail="$name: string to sign
     want [$want_sts]
     got  [$got_sts]"
    elif [ "$got_auth" != "$want_auth" ]; then
      fail=$((fail + 1)); [ -n "$firstfail" ] || firstfail="$name: Authorization
     want [$want_auth]
     got  [$got_auth]"
    else
      pass=$((pass + 1))
    fi
  done

  if [ "$fail" = "0" ]; then
    t_ok "the bash signer reproduces all $pass of Go's SigV4 vectors, stage by stage"
  else
    t_no "the bash signer disagrees with Go on $fail of $V_COUNT vectors"
    printf '     %s\n' "$firstfail"
  fi

  # THE KEY DERIVATION ON ITS OWN, because it is the stage a hex-versus-string
  # mistake breaks and the only one that fails INTERMITTENTLY: passing a binary
  # key as a string truncates it at the first NUL, which happens in about one
  # derivation in 256.
  want_k4="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["cases"][0]["signingKeyChainHex"][3])' "$VECTORS")"
  got_k4="$(
    # shellcheck disable=SC1091
    . "$TOOLKIT/transports/objstore.sh"
    k="$(printf '%s' "${V_NOW%%T*}" | openssl dgst -sha256 -mac HMAC -macopt "key:AWS4${V_SECRET}" -hex | awk '{print $NF}')"
    k="$(_obj_hmac_hex "$k" "$V_REGION")"
    k="$(_obj_hmac_hex "$k" "s3")"
    _obj_hmac_hex "$k" "aws4_request"
  )"
  assert_eq "the four-stage signing key matches Go's, carried as hex the whole way" \
    "$want_k4" "$got_k4"
fi

# =============================================================================
#  2. THE ESCAPING, WHICH IS THE OTHER HALF OF A SIGNATURE
# =============================================================================
esc() (
  # shellcheck disable=SC1091
  . "$TOOLKIT/transports/objstore.sh"
  _obj_escape "$1"
)
assert_eq "unreserved characters are left alone" "aZ09-_.~" "$(esc 'aZ09-_.~')"
# A SPACE IS %20 AND NEVER +. `+` is what form encoding uses, AWS rejects it,
# and the two differ only on keys with spaces in them.
assert_eq "a space is %20, not +" "a%20b" "$(esc 'a b')"
assert_eq "a plus is escaped, not passed through" "a%2Bb" "$(esc 'a+b')"
assert_eq "a slash is escaped by the segment escaper" "a%2Fb" "$(esc 'a/b')"
assert_eq "and the characters that look safe and are not" "%3D%26%3F%23" "$(esc '=&?#')"

escp() (
  # shellcheck disable=SC1091
  . "$TOOLKIT/transports/objstore.sh"
  _obj_escape_path "$1"
)
assert_eq "a path keeps its separators and escapes the rest" \
  "/bucket/heliograph/logs/a%20b%2Bc.txt" "$(escp '/bucket/heliograph/logs/a b+c.txt')"

# =============================================================================
#  3. WHAT IT REFUSES BEFORE IT EVER SIGNS
# =============================================================================
export TOOLKIT   # the refusal checks below run in a child shell, which needs it
init() (
  # shellcheck disable=SC1091
  . "$TOOLKIT/caplib.sh" 2>/dev/null
  # shellcheck disable=SC1091
  . "$TOOLKIT/transports/objstore.sh"
  tp_init 2>&1
)
BASE_ENV=(OBJSTORE_ENDPOINT=https://s3.example.invalid OBJSTORE_BUCKET=b
          OBJSTORE_LANE=lane1 OBJSTORE_KEY_ID=AKID OBJSTORE_SECRET=sek
          OBJSTORE_REGION=eu-west-2)

OUT="$(env "${BASE_ENV[@]}" OBJSTORE_REGION= bash -c "$(declare -f init); init")"
assert_contains "a missing region is named, because a wrong one is a 403 that says nothing" \
  "OBJSTORE_REGION" "$OUT"

OUT="$(env "${BASE_ENV[@]}" OBJSTORE_LANE='x
state: running' bash -c "$(declare -f init); init")"
assert_contains "a lane holding a newline is refused, or an idle station reports itself busy for ever" \
  "not a usable OBJSTORE_LANE" "$OUT"

OUT="$(env "${BASE_ENV[@]}" OBJSTORE_LANE='../other' bash -c "$(declare -f init); init")"
assert_contains "and so is one that would climb out of its prefix" "not a usable OBJSTORE_LANE" "$OUT"

OUT="$(env "${BASE_ENV[@]}" OBJSTORE_ENDPOINT=http://s3.example.invalid bash -c "$(declare -f init); init")"
assert_contains "a cleartext endpoint is refused, because the body is a captured log" \
  "cross the network in clear" "$OUT"
OUT="$(env "${BASE_ENV[@]}" OBJSTORE_ENDPOINT=http://127.0.0.1:9000 OBJSTORE_ALLOW_HTTP=1 \
        bash -c "$(declare -f init); init")"
assert_eq "  and permitted when the operator says so in as many words" "no" \
  "$(printf '%s' "$OUT" | grep -q 'cross the network in clear' && echo yes || echo no)"

OUT="$(env "${BASE_ENV[@]}" OBJSTORE_ENDPOINT=s3.example.invalid bash -c "$(declare -f init); init")"
assert_contains "an endpoint with no scheme is refused rather than guessed at" "needs a scheme" "$OUT"

DESC="$(env "${BASE_ENV[@]}" OBJSTORE_SECRET=supersecretvalue bash -c "$(declare -f init)
$(declare -f); . '$TOOLKIT/caplib.sh' 2>/dev/null; . '$TOOLKIT/transports/objstore.sh'; tp_init >/dev/null 2>&1; tp_describe")"
assert_eq "tp_describe never prints a character of the secret" "no" \
  "$(printf '%s' "$DESC" | grep -q 'supersecretvalue' && echo yes || echo no)"
assert_contains "  and still says which credential, or it has masked away the answer" "AKID" "$DESC"

# =============================================================================
#  4. THE VERBS, AGAINST A FAKE STORE
# =============================================================================
FAKE2="$TMP/bin2"; mkdir -p "$FAKE2"
CALLS="$TMP/calls"; : > "$CALLS"
cat > "$FAKE2/curl" <<'EOS'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FAKE_CALLS"
# The response body a GET should see, when the caller asked for one.
out=""; prev=""
for a in "$@"; do [ "$prev" = "-o" ] && out="$a"; prev="$a"; done
case " $* " in
  *' -X PUT '*) printf '200' ;;
  *)
    [ -n "$out" ] && [ -n "${FAKE_BODY:-}" ] && printf '%s' "$FAKE_BODY" > "$out"
    [ -n "${FAKE_CODE:-}" ] && { printf '%s' "$FAKE_CODE"; exit 0; }
    printf '200' ;;
esac
EOS
chmod +x "$FAKE2/curl"

drive() (
  export PATH="$FAKE2:$PATH" FAKE_CALLS="$CALLS"
  # FAKE_CODE reaches the fake curl through the environment rather than through
  # `env <var> <fn>`, which cannot work: tp_* are FUNCTIONS, and env only
  # execs binaries. That returned 127 and read as the assertion failing.
  export FAKE_CODE="${FAKE_CODE:-}" FAKE_BODY="${FAKE_BODY:-}"
  export "${BASE_ENV[@]}" OBJSTORE_PREFIX=heliograph
  # shellcheck disable=SC1091
  . "$TOOLKIT/caplib.sh" 2>/dev/null
  # shellcheck disable=SC1091
  . "$TOOLKIT/transports/objstore.sh"
  tp_init >/dev/null 2>&1 || { echo "INIT-FAILED"; exit 1; }
  "$@"
)

printf 'id: r-1\nstep: ./steps/probe.sh\n' > "$TMP/req"
: > "$CALLS"
GOT="$(FAKE_BODY="$(cat "$TMP/req")" drive tp_fetch_request)"
assert_contains "tp_fetch_request returns the queued document" "id: r-1" "$GOT"
assert_contains "  read from requests/<lane>.txt, where the control side writes it" \
  "heliograph/requests/lane1.txt" "$(cat "$CALLS")"

: > "$CALLS"
FAKE_CODE=404 drive tp_fetch_request >/dev/null 2>&1
RC=$?
assert_eq "a 404 is 'nothing queued' and not a failure, or a fresh station never starts" "0" "$RC"

: > "$CALLS"
drive tp_put_status 'state: idle' 'msg' >/dev/null 2>&1
assert_contains "tp_put_status writes status/<lane>.txt" "heliograph/status/lane1.txt" "$(cat "$CALLS")"
assert_contains "  with a PUT" "-X PUT" "$(cat "$CALLS")"

printf 'the whole log\n' > "$TMP/probe-20260912T120000Z.txt"
: > "$CALLS"
drive tp_put_log "$TMP/probe-20260912T120000Z.txt" >/dev/null 2>&1
assert_contains "tp_put_log writes under logs/" "heliograph/logs/probe-20260912T120000Z.txt" "$(cat "$CALLS")"

# THE ONE THE CONTROL SIDE'S LISTING DEPENDS ON. ListLogs skips `.partial.txt`,
# so progress MUST carry that suffix - and a finished log must not. Publishing
# progress under the final name makes every in-flight run look complete to a
# reader, which is the exact failure the suffix exists to prevent.
: > "$CALLS"
drive tp_put_progress 'state: running' 'msg' "$TMP/probe-20260912T120000Z.txt" >/dev/null 2>&1
assert_contains "tp_put_progress writes the snapshot as .partial.txt, which the control side HIDES" \
  "heliograph/logs/probe-20260912T120000Z.partial.txt" "$(cat "$CALLS")"
assert_eq "  and NOT under the final name, which would show a truncated run as complete" "no" \
  "$(grep -- '-X PUT' "$CALLS" | grep -q 'logs/probe-20260912T120000Z.txt' && echo yes || echo no)"

# A cancelled run's partial log, which git, share and bundle all ship.
: > "$CALLS"
drive tp_put_status 'state: cancelled' 'msg' "$TMP/probe-20260912T120000Z.txt" >/dev/null 2>&1
assert_contains "a cancelled run's partial log is shipped under its FINAL name, because the run is over" \
  "heliograph/logs/probe-20260912T120000Z.txt" "$(cat "$CALLS")"

# =============================================================================
#  5. IT DECLARES ONLY WHAT A METERED STORE SHOULD DO
# =============================================================================
CAPS="$(drive tp_capabilities 2>/dev/null)"
assert_contains "it carries a request" "request" "$CAPS"
assert_contains "  a status" "status" "$CAPS"
assert_contains "  and progress" "progress" "$CAPS"
assert_eq "  and NOT a live read, which is a second billed GET per interval" "no" \
  "$(printf '%s' "$CAPS" | grep -qw live && echo yes || echo no)"
assert_eq "  and NOT self-update, because nothing publishes a payload to a bucket" "no" \
  "$(printf '%s' "$CAPS" | grep -qw self && echo yes || echo no)"
for fn in tp_fetch_request_live tp_fetch_self; do
  assert_eq "$fn is defined, so calling it refuses rather than crashing" "yes" \
    "$( ( . "$TOOLKIT/transports/objstore.sh"; declare -F "$fn" >/dev/null && echo yes || echo no ) 2>/dev/null )"
done

t_summary
