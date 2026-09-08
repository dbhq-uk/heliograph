#!/usr/bin/env bash
# =============================================================================
#  transports/relay.sh - the relay, under the station's contract
# =============================================================================
# Sourced by station.sh. For an estate that will not give you a git host, a
# storage account, a share or an inbound route: both sides dial OUT over
# ordinary HTTPS and meet at a server neither of them trusts.
#
# THE RELAY IS NOT TRUSTED, IN EITHER DIRECTION.
#
# Everything is sealed and signed by the far side before it is sent, and
# verified here before it is acted on. A relay that could merely read logs would
# be a privacy problem; a relay that could FORGE a request would have code
# execution inside every estate at once, through a channel the estate installed
# deliberately. So authenticity is checked first and confidentiality second.
#
# WHY THIS ONE NEEDS A BINARY, WHEN NO OTHER TRANSPORT DOES
#
# The construction is X25519, HKDF-SHA256, ChaCha20-Poly1305 and Ed25519.
# `openssl enc` refuses AEAD ciphers outright, so a shell implementation would
# have to assemble encrypt-then-MAC and key agreement by hand across openssl
# 1.1.1 and 3.x behaviour differences. That is where crypto bugs live, and a
# crypto bug here is silent.
#
# So `heliograph-seal` does the sealing and nothing else. It does NOT do the
# networking: curl stays here, in the shell, where its behaviour can be read and
# debugged. git, blob, share and bundle remain pure bash and always will.
#
# The binary is checksum-verified before it is executed, and the station refuses
# to start if it is missing or wrong. There is no plaintext fallback and no
# opportunistic mode: a station configured for the relay either speaks sealed or
# does not speak.
# =============================================================================

# No `self`: a payload update arrives as a sealed message, which is the same
# --allow-payload capability as anywhere else and is off by default. No `live`
# either - a mid-run poll costs a request per interval against a metered edge,
# and the cancel it would deliver can wait for the next cycle.
tp_capabilities() { printf 'request status progress\n'; }

RELAY_SEAL="${RELAY_SEAL:-$REPO_ROOT/heliograph-seal}"

tp_init() {
  command -v curl >/dev/null 2>&1 || {
    echo "station: the relay transport needs curl, which is not on PATH." >&2
    return 1
  }
  # cap_need rather than `${VAR:?}`, which exits the SHELL rather than failing
  # this function. See cap_need in caplib.sh for what that cost.
  cap_need RELAY_URL     "the base URL of the relay both sides dial out to" || return 1
  cap_need RELAY_ESTATE  "which estate this station belongs to" || return 1
  cap_need RELAY_STATION "the station name, which is what a run is bound to" || return 1
  cap_need RELAY_TOKEN   "the station-scoped token; without it every request is refused and it reads like a fault at the far end" || return 1

  if [ ! -x "$RELAY_SEAL" ]; then
    echo "station: heliograph-seal is not at $RELAY_SEAL." >&2
    echo "         The relay transport cannot verify a request without it, and" >&2
    echo "         running unverified is not an option this transport offers." >&2
    return 1
  fi
  # A checksum, checked before the binary is ever executed. It runs on a machine
  # nobody can reach, so "we shipped the right one" has to be establishable here
  # rather than assumed.
  if [ -n "${RELAY_SEAL_SHA256:-}" ]; then
    local got
    got="$(sha256sum "$RELAY_SEAL" 2>/dev/null | cut -d' ' -f1)"
    if [ "$got" != "$RELAY_SEAL_SHA256" ]; then
      echo "station: heliograph-seal does not match RELAY_SEAL_SHA256." >&2
      echo "         expected $RELAY_SEAL_SHA256" >&2
      echo "         got      ${got:-<unreadable>}" >&2
      return 1
    fi
  else
    say "warn: RELAY_SEAL_SHA256 is not set, so heliograph-seal is unverified."
    say "      Set it to the checksum published with the release."
  fi

  # Identity and peer. Without both there is nothing to verify against, and a
  # station that cannot verify must not start rather than start and accept.
  cap_need RELAY_IDENTITY "this station's key file" || return 1
  cap_need RELAY_PEER     "the control side's public identity, which is what a request is verified against" || return 1
  [ -r "$RELAY_IDENTITY" ] || { echo "station: cannot read $RELAY_IDENTITY" >&2; return 1; }

  RELAY_BASE="${RELAY_URL%/}"
  RELAY_STATE="${RELAY_STATE:-$REPO_ROOT/.station-relay-state}"
  # Sequence numbers are the whole replay defence, so they are read once here
  # and written after every accepted message.
  RELAY_SEEN="$(sed -n 's/^seen://p' "$RELAY_STATE" 2>/dev/null | head -1)"
  RELAY_OUT="$(sed -n 's/^out://p' "$RELAY_STATE" 2>/dev/null | head -1)"
  RELAY_SEEN="${RELAY_SEEN:-0}"
  RELAY_OUT="${RELAY_OUT:-0}"
  return 0
}

tp_scope() { printf '%s' "$RELAY_STATION"; }
tp_revision() { printf 'relay %s, estate %s' "$RELAY_BASE" "$RELAY_ESTATE"; }

tp_describe() {
  printf 'relay %s, estate %s, station %s, peer %s' \
    "$RELAY_BASE" "$RELAY_ESTATE" "$RELAY_STATION" \
    "$("$RELAY_SEAL" fingerprint --peer "$RELAY_PEER" 2>/dev/null || echo '<unreadable>')"
}

_relay_save_state() {
  # Write-then-rename. A truncated state file reads as sequence zero, and
  # sequence zero accepts every replay the relay has ever seen.
  printf 'seen:%s\nout:%s\n' "$RELAY_SEEN" "$RELAY_OUT" > "$RELAY_STATE.tmp" &&
    mv -f "$RELAY_STATE.tmp" "$RELAY_STATE"
}

_relay_url() { printf '%s/v1/%s/%s/%s' "$RELAY_BASE" "$RELAY_ESTATE" "$RELAY_STATION" "$1"; }

tp_check() {
  local code
  code="$(curl -sS -o /dev/null -w '%{http_code}' -m 20 \
            "$RELAY_BASE/health" 2>/dev/null)" || return 1
  [ "$code" = "200" ] || { say "relay health: HTTP $code"; return 1; }
  # Health needs no token, so it proves reachability and nothing else. Prove the
  # credential too, or a wrong one is discovered by a failed log push an hour
  # from now with nobody left to tell.
  code="$(curl -sS -o /dev/null -w '%{http_code}' -m 20 \
            -H "Authorization: Bearer ${RELAY_TOKEN}" \
            "$(_relay_url c2s)?wait=0" 2>/dev/null)" || return 1
  case "$code" in
    200) return 0 ;;
    401|403) say "the relay is reachable but refused this token for estate $RELAY_ESTATE"; return 1 ;;
    *) say "relay: HTTP $code"; return 1 ;;
  esac
}

# Collect requests, verify them, and emit the newest acceptable one.
#
# A message that does not verify is dropped rather than reported as an error:
# the relay is entitled to hand us anything, and a station that stopped on
# rubbish would be one a hostile relay could halt at will.
tp_fetch_request() {
  local body tmp
  body="$(curl -sS -m 40 -H "Authorization: Bearer ${RELAY_TOKEN}" \
            "$(_relay_url c2s)?wait=0" 2>/dev/null)" || return 1
  [ -n "$body" ] || return 0

  tmp="$(mktemp)" || return 1
  printf '%s' "$body" > "$tmp"

  local out
  out="$("$RELAY_SEAL" open \
          --identity "$RELAY_IDENTITY" --peer "$RELAY_PEER" \
          --estate "$RELAY_ESTATE" --station "$RELAY_STATION" \
          --dir c2s --kind request --min-seq "$RELAY_SEEN" \
          --in "$tmp" 2>/dev/null)"
  local rc=$?
  rm -f "$tmp"
  [ "$rc" = "0" ] || { printf ''; return 0; }

  # heliograph-seal prints the accepted sequence number on its first line and
  # the document after it, so the shell never has to parse the envelope.
  local seq
  seq="$(printf '%s\n' "$out" | head -1)"
  case "$seq" in
    ''|*[!0-9]*) return 0 ;;
  esac
  RELAY_SEEN="$seq"
  _relay_save_state
  printf '%s\n' "$out" | tail -n +2
}

tp_fetch_request_live() { return 1; }
tp_fetch_self() { return 1; }

_relay_put() {
  local kind="$1" file="$2" tmp rc
  RELAY_OUT=$((RELAY_OUT + 1))
  tmp="$(mktemp)" || return 1
  "$RELAY_SEAL" seal \
    --identity "$RELAY_IDENTITY" --peer "$RELAY_PEER" \
    --estate "$RELAY_ESTATE" --station "$RELAY_STATION" \
    --dir s2c --kind "$kind" --seq "$RELAY_OUT" \
    --in "$file" --out "$tmp" 2>/dev/null
  rc=$?
  if [ "$rc" != "0" ]; then rm -f "$tmp"; return 1; fi

  local code
  code="$(curl -sS -o /dev/null -w '%{http_code}' -m 60 -X POST \
            -H "Authorization: Bearer ${RELAY_TOKEN}" \
            -H "Content-Type: application/json" \
            --data-binary "@$tmp" "$(_relay_url s2c)" 2>/dev/null)"
  rm -f "$tmp"
  [ "$code" = "202" ] || { say "relay put ($kind): HTTP $code"; return 1; }
  _relay_save_state
}

tp_put_status() {
  local body="$1" tmp rc
  tmp="$(mktemp)" || return 1
  printf '%s' "$body" > "$tmp"
  _relay_put status "$tmp"; rc=$?
  rm -f "$tmp"
  return "$rc"
}

tp_put_progress() {
  local body="$1" _msg="$2" logfile="$3" tmp rc
  tmp="$(mktemp)" || return 1
  printf '%s' "$body" > "$tmp"
  _relay_put status "$tmp"; rc=$?
  rm -f "$tmp"
  [ "$rc" = "0" ] || return "$rc"
  _relay_put progress "$logfile"
}

# Deliver the finished log.
#
# A DISTINCT KIND from `progress`, and not a tidiness point. The kind travels
# inside the sealed envelope as a signed field, so the control side can tell a
# completed log from a mid-run snapshot without trusting the relay to label it.
# Sending the final log as `progress` would leave the reader unable to know it
# had the whole thing, which is the one question a log with a footer exists to
# answer.
#
# The message is ignored: it is a git commit subject, and there is no history
# here to carry it.
tp_put_log() {
  local logfile="$1"
  [ -f "$logfile" ] || return 1
  _relay_put log "$logfile"
}
