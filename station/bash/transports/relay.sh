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
  # BOTH of them. Only the identity was checked, and a station with an
  # unreadable or absent RELAY_PEER started perfectly: tp_describe turned the
  # failed fingerprint into "<unreadable>" and tp_check only ever tested HTTP.
  # Every request then failed verification and every log failed to seal, on a
  # machine nobody can log into, for a reason the preflight had already been
  # told and swallowed.
  [ -r "$RELAY_PEER" ] || {
    echo "station: cannot read $RELAY_PEER, and that file is what a request is verified against." >&2
    echo "         Without it this station can neither accept a request nor seal a log." >&2
    return 1
  }

  # INTERPOLATED INTO A URL, so they are checked before they become one. A `/`
  # in either reaches a different route, a `?` starts a query string, and a `#`
  # truncates the path - all of them silently, and all of them ending as a 404
  # or as somebody else's queue. The control side applies the same rule.
  local _v
  for _v in RELAY_ESTATE RELAY_STATION; do
    case "${!_v}" in
      *[!A-Za-z0-9._-]*|-*)
        echo "station: $_v is '${!_v}', which is not a usable routing key." >&2
        echo "         It becomes part of a URL path, so it may hold only" >&2
        echo "         letters, digits, dot, hyphen and underscore, and may" >&2
        echo "         not begin with a hyphen." >&2
        return 1 ;;
    esac
  done

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

# --- the sequence numbers, across two processes -------------------------------
#
# THE DEFECT THIS EXISTS TO FIX, because it made the relay unusable and looked
# like nothing at all.
#
# The loop and the runner are SEPARATE PROCESSES. The loop reads the state once
# at tp_init and holds it; the runner is a child that loads the transport of its
# own in order to deliver the finished log. So the runner published the log as
# sequence N, and the loop then published `idle` as sequence N as well, from the
# value it had loaded before the step started.
#
# The receiver drops anything at or below what it has already accepted - by
# design, because that is the replay defence and a replayed request is a
# destructive step re-running with its gates already satisfied. So the `idle`
# went into the relay and out of existence.
#
# What that looks like from the control node: the log arrives, and the station
# reports `running` for ever. Nothing errors anywhere. It was recorded as a
# known defect and only became reproducible once a round trip existed to run.
#
# A LOCK, because two processes really are racing and re-reading alone would
# only narrow the window. `mkdir` is the atomic primitive that exists
# everywhere - flock is absent from macOS and from some minimal images, and this
# runs on whatever is there.
#
# THE STALE CHECK ASKS WHETHER THE HOLDER IS ALIVE, NOT HOW OLD THE LOCK IS,
# and that distinction cost a measurement to find.
#
# The first version broke any lock whose directory had not been touched for a
# minute. Under real contention that fires on LIVE locks - the holder is working
# and the directory's mtime does not change while it does - so the breaker
# deleted a lock somebody held, the waiter created its own, and two processes
# were inside at once. Measured: four processes taking fifteen numbers each got
# 33 distinct numbers out of 60. A lock with a heuristic that can fire on a live
# holder is not a lock, and it fails exactly like no lock at all: intermittently,
# silently, under load.
#
# station.sh has always broken `.station.lock` by asking `kill -0` whether the
# recorded pid is still there. Same question, same answer, and it cannot be
# wrong about a process that is running.
_relay_lock() {
  local lock="$RELAY_STATE.lock" waited=0 pid
  # "ALREADY EXISTS" IS THE ONLY mkdir FAILURE WORTH WAITING ON. An unwritable
  # directory, a read-only mount or a full disk makes mkdir fail too, and the
  # loop below then waited for a lock that could never appear - for ever, on a
  # station nobody can log into, with the loop silently no longer publishing.
  # Found by writing the test for the state-write failure below.
  #
  # ASKED ONCE, UP FRONT, rather than inside the loop. Testing it there looked
  # equivalent and was not: mkdir fails, the holder releases, the directory is
  # gone by the time the test runs, and a perfectly ordinary release was read as
  # an unwritable filesystem. Measured - 58 numbers out of 60, with two takers
  # refused for nothing.
  if [ ! -w "$(dirname "$lock")" ]; then
    say "relay: cannot create the sequence lock in $(dirname "$lock") - it is not writable"
    return 1
  fi
  while ! mkdir "$lock" 2>/dev/null; do
    pid="$(cat "$lock/pid" 2>/dev/null || echo)"
    if [ -n "$pid" ] && ! kill -0 "$pid" 2>/dev/null; then
      # The holder is gone. Nothing here is ambiguous: that pid was written by
      # a process that no longer exists.
      #
      # RENAMED, NOT DELETED, and that is the difference between breaking a
      # dead lock and breaking a live one. Two waiters can both read the same
      # dead pid; if both then `rm -rf` the path, the second one deletes the
      # lock the FIRST has already replaced and taken - so breaking a stale
      # lock hands out a duplicate. A rename is atomic and only one of them can
      # win it, and the winner has moved THAT directory rather than whatever is
      # at the path by the time it runs.
      mv "$lock" "$lock.dead.$$" 2>/dev/null && rm -rf "$lock.dead.$$" 2>/dev/null
      continue
    fi
    waited=$((waited + 1))
    if [ "$waited" -gt 200 ]; then
      # Ten seconds. A LIVE holder always has a pid file, and `kill -0` above
      # settles that case immediately - so reaching here with no pid means the
      # holder died in the window between mkdir and writing it, which nothing
      # else can detect. Break only that, and only by rename.
      #
      # A live holder is NEVER broken on a timeout. Waiting for one is correct:
      # it is doing two file operations, and a station that stole a lock from a
      # working process would hand out a duplicate number, which the receiver
      # drops - losing a log or a final status silently.
      if [ -s "$lock/pid" ]; then
        say "relay: still waiting for the sequence lock at $lock (held by pid $(cat "$lock/pid" 2>/dev/null))"
        waited=0
        continue
      fi
      say "relay: breaking a sequence lock that was never claimed, at $lock"
      mv "$lock" "$lock.dead.$$" 2>/dev/null && rm -rf "$lock.dead.$$" 2>/dev/null
      waited=0
      continue
    fi
    sleep 0.05 2>/dev/null || sleep 1
  done
  echo "$$" > "$lock/pid" 2>/dev/null
  return 0
}

_relay_unlock() { rm -rf "$RELAY_STATE.lock" 2>/dev/null || true; }

_relay_read_field() {  # _relay_read_field <seen|out>
  local v
  v="$(sed -n "s/^$1://p" "$RELAY_STATE" 2>/dev/null | head -1)"
  case "$v" in ''|*[!0-9]*) v=0 ;; esac
  printf '%s' "$v"
}

# Write both fields, taking the HIGHER of what is on disk and what this process
# holds.
#
# NEVER A PLAIN OVERWRITE. The runner does not fetch requests, so its RELAY_SEEN
# is whatever the loop had when the step started - and writing that back would
# REGRESS the replay counter, which is the one number whose whole job is never
# to go backwards. Taking the maximum makes a stale writer harmless.
#
# Write-then-rename, because a truncated state file reads as sequence zero, and
# sequence zero accepts every replay the relay has ever seen.
_relay_save_state() {
  local fseen fout
  fseen="$(_relay_read_field seen)"
  fout="$(_relay_read_field out)"
  [ "${RELAY_SEEN:-0}" -gt "$fseen" ] && fseen="$RELAY_SEEN"
  [ "${RELAY_OUT:-0}" -gt "$fout" ] && fout="$RELAY_OUT"
  RELAY_SEEN="$fseen"
  RELAY_OUT="$fout"
  # A temporary NAMED FOR THIS PROCESS. Every writer used `$RELAY_STATE.tmp`,
  # so two of them raced on one path and the loser's `mv` failed with "cannot
  # stat" - visible only as a state file that had not been updated.
  #
  # With the lock working, two writers should never be in here at once. It stays
  # because they still can: _relay_lock breaks a lock that has no pid after ten
  # seconds, which is the right trade for a station nobody can log into, and the
  # price of that trade is exactly this window.
  local tmp="$RELAY_STATE.$$.tmp" rc
  printf 'seen:%s\nout:%s\n' "$fseen" "$fout" > "$tmp" && mv -f "$tmp" "$RELAY_STATE"
  rc=$?
  rm -f "$tmp" 2>/dev/null
  # THE RESULT IS RETURNED, and the `rm` above must not become it. A full disk
  # or a read-only mount made this fail silently, and both callers ignored it -
  # so a request was verified and RUN with its sequence number never persisted,
  # and after the next restart the relay could replay that request and have it
  # accepted again. The replay defence is only a defence if it is written down.
  return "$rc"
}

# Take the next outbound sequence number, and publish it before releasing, so no
# other process can take the same one.
_relay_next_out() {
  local rc
  _relay_lock || return 1
  RELAY_OUT=$(( $(_relay_read_field out) + 1 ))
  _relay_save_state; rc=$?
  _relay_unlock
  # A number that could not be recorded has not been reserved: the next process
  # will hand out the same one, and the receiver will drop everything after the
  # first. Fail rather than send under a number nothing is holding.
  [ "$rc" = "0" ] || { say "relay: could not persist the sequence state at $RELAY_STATE"; return 1; }
  printf '%s' "$RELAY_OUT"
}

_relay_record_seen() {  # _relay_record_seen <seq>
  local rc
  _relay_lock || return 1
  RELAY_SEEN="$1"
  _relay_save_state; rc=$?
  _relay_unlock
  [ "$rc" = "0" ] || { say "relay: could not persist the replay counter at $RELAY_STATE"; return 1; }
  return 0
}

_relay_url() { printf '%s/v1/%s/%s/%s' "$RELAY_BASE" "$RELAY_ESTATE" "$RELAY_STATION" "$1"; }

tp_check() {
  local code
  # THE KEYS FIRST, before any network. Readable is not the same as usable: a
  # truncated peer key, or the wrong file entirely, is readable and fails every
  # verification afterwards. `fingerprint` parses it and is the cheapest thing
  # that does, so a key that will not parse is caught here rather than on the
  # first request that never runs.
  "$RELAY_SEAL" fingerprint --peer "$RELAY_PEER" >/dev/null 2>&1 || {
    say "the control side's public identity at $RELAY_PEER will not parse."
    say "  Every request would fail verification and every log would fail to seal."
    return 1
  }
  code="$(curl -sS -o /dev/null -w '%{http_code}' -m 20 \
            "$RELAY_BASE/health" 2>/dev/null)" || return 1
  [ "$code" = "200" ] || { say "relay health: HTTP $code"; return 1; }

  # Health needs no token, so it proves reachability and nothing else. Prove the
  # credential too, or a wrong one is discovered by a failed log push an hour
  # from now with nobody left to tell.
  #
  # AGAINST A STATION NAME NOTHING USES, and that is not cosmetic. This probed
  # `$(_relay_url c2s)` - this station's own request queue - and the relay
  # DELETES ON COLLECTION. So every `./start.sh` silently ate whatever request
  # was waiting: the operator sent a step, started the station, and the station
  # came up reporting itself idle for ever with the request gone from the relay.
  # Found by running it, not by reading it, because from either side that is
  # indistinguishable from nobody having sent anything.
  #
  # Tokens are scoped to the ESTATE rather than to a station, so a queue nobody
  # reads proves exactly the same thing and costs an empty routing key.
  #
  # A NAME NO STATION CAN HAVE. `${RELAY_STATION}-heliograph-check` was the
  # first choice and it is only a convention: an estate with a station actually
  # called `prod-heliograph-check` would have its pending request eaten every
  # time `prod` started. A station name is validated above to letters, digits,
  # dot, hyphen and underscore, so a leading `~` cannot collide with one - and
  # `~` is unreserved in a URL path, so nothing has to be escaped.
  code="$(curl -sS -o /dev/null -w '%{http_code}' -m 20 \
            -H "Authorization: Bearer ${RELAY_TOKEN}" \
            "${RELAY_BASE}/v1/${RELAY_ESTATE}/~heliograph-preflight/c2s?wait=0" 2>/dev/null)" || return 1
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
  # Under the lock, and taking the maximum, so a concurrent runner publishing a
  # log cannot have its sequence number overwritten by this one.
  _relay_record_seen "$seq" || return 1
  printf '%s\n' "$out" | tail -n +2
}

tp_fetch_request_live() { return 1; }
tp_fetch_self() { return 1; }

_relay_put() {
  local kind="$1" file="$2" tmp rc seq
  # TAKEN UNDER THE LOCK, not incremented from a value this process loaded at
  # start. See _relay_next_out: the loop and the runner are separate processes
  # and both publish, and a collision is silently dropped by the receiver.
  seq="$(_relay_next_out)" || return 1
  [ -n "$seq" ] || return 1
  tmp="$(mktemp)" || return 1
  "$RELAY_SEAL" seal \
    --identity "$RELAY_IDENTITY" --peer "$RELAY_PEER" \
    --estate "$RELAY_ESTATE" --station "$RELAY_STATION" \
    --dir s2c --kind "$kind" --seq "$seq" \
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
  return 0
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
