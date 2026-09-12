#!/usr/bin/env bash
# =============================================================================
#  transports/objstore.sh - S3-compatible storage, under the station's contract
# =============================================================================
# Sourced by station.sh. For an estate that permits object storage where it
# permits no git host, no share and no outbound route to a relay.
#
# That combination sounds contrived and is common: a locked-down subnet where a
# firewall appliance holds the default route has no outbound anything, while
# traffic to a storage endpoint inside the virtual network never reaches the
# appliance and works normally. Storage is reachable when nothing else is.
#
#   <PREFIX>requests/<lane>.txt        the control side writes it
#   <PREFIX>status/<lane>.txt          this side writes it
#   <PREFIX>logs/<name>.txt            this side writes them
#   <PREFIX>logs/<name>.partial.txt    progress; the control side HIDES these
#
# THE LAYOUT IS THE CONTROL SIDE'S, and it is not negotiable from here.
# `internal/transport/objstore.go` reads exactly these keys, and its ListLogs
# skips anything ending `.partial.txt` so a truncated in-flight snapshot is
# never read as a finished run. A station that published progress under the
# final name would make the control side show a partial log as complete.
#
# S3-COMPATIBLE, WHICH IS MOST OF THEM: AWS S3, Cloudflare R2, MinIO, Backblaze
# B2, DigitalOcean Spaces, Ceph. Azure Blob is NOT S3-compatible and has its own
# transport in blob.sh.
#
# WHY THIS ONE IS THE LONGEST TRANSPORT HERE
#
# It has to sign. SigV4 is a canonical request, a string to sign, and a
# four-stage HMAC key derivation, and every S3-compatible store rejects a wrong
# signature with a 403 that names NOTHING - deliberately, because saying which
# part disagreed would be an oracle. So "the credential is wrong", "the clock is
# wrong", "the region is wrong" and "the canonical form is wrong" all look
# identical from here.
#
# The signer is therefore held to `tests/fixtures/sigv4-vectors.json`, emitted
# from the Go side, which pins every intermediate stage rather than the final
# header. See tests/test-objstore.sh.
#
# NO AWS CLI, and no python. `openssl` and `curl` are the whole dependency, and
# both are already required by other transports.
# =============================================================================

# No `self`: nothing publishes a payload here, and a station that updated itself
# from a bucket anybody with write access can reach is a worse idea than it
# looks. No `live` either: a mid-run poll is a second billed GET per interval
# against a metered store, and the cancel it would carry can wait one cycle -
# the same trade blob and relay make.
tp_capabilities() { printf 'request status progress\n'; }

tp_init() {
  command -v curl >/dev/null 2>&1 || {
    echo "station: the objstore transport needs curl, which is not on PATH." >&2
    return 1
  }
  # openssl IS the signer. Without it there is no way to authenticate at all,
  # and the failure otherwise arrives as an empty signature and a 403.
  command -v openssl >/dev/null 2>&1 || {
    echo "station: the objstore transport needs openssl to sign requests, and it is not on PATH." >&2
    return 1
  }

  cap_need OBJSTORE_ENDPOINT "the storage endpoint, like https://s3.eu-west-2.amazonaws.com" || return 1
  cap_need OBJSTORE_BUCKET   "the bucket" || return 1
  cap_need OBJSTORE_LANE     "the lane, which is what a run is bound to - one lane per investigation, so two do not overwrite each other" || return 1
  cap_need OBJSTORE_KEY_ID   "the access key id" || return 1
  cap_need OBJSTORE_SECRET   "the secret access key" || return 1
  cap_need OBJSTORE_REGION   "the region the signature is scoped to; a wrong one is a 403 that names nothing" || return 1

  OBJ_ENDPOINT="${OBJSTORE_ENDPOINT%/}"
  OBJ_BUCKET="$OBJSTORE_BUCKET"
  OBJ_LANE="$OBJSTORE_LANE"
  OBJ_REGION="$OBJSTORE_REGION"
  # Optional, and normalised to end in `/` if set at all. The control side does
  # the same, and a prefix that differs by one slash is a station writing into a
  # directory nobody reads.
  OBJ_PREFIX="${OBJSTORE_PREFIX:-}"
  [ -n "$OBJ_PREFIX" ] && OBJ_PREFIX="${OBJ_PREFIX%/}/"

  # A WHITELIST ON THE LANE, and the reason is not path traversal alone.
  #
  # Refusing separators covers the obvious case. It does not cover the one that
  # bites: the lane is published as the status document's `branch:` value, and
  # that document is line-oriented `key: value`. A lane holding a NEWLINE
  # injects a second key and the parser keeps the last - so a lane of
  # "x<newline>state: running" makes an idle station report itself busy for
  # ever. share.sh, bundle.sh and internal/transport refuse exactly this set.
  case "$OBJ_LANE" in
    *[!A-Za-z0-9._-]* | -* | '.' | '..')
      echo "station: '$OBJ_LANE' is not a usable OBJSTORE_LANE. It becomes part of a key AND the status document's branch field, so it may hold only letters, digits, dot, hyphen and underscore, and may not begin with a hyphen." >&2
      return 1 ;;
  esac

  # THE ENDPOINT MUST BE HTTPS, unless the operator says otherwise in as many
  # words. The secret key never crosses the wire, but the request body does -
  # and that body is a captured log.
  case "$OBJ_ENDPOINT" in
    https://*) : ;;
    http://*)
      if [ "${OBJSTORE_ALLOW_HTTP:-0}" != "1" ]; then
        echo "station: OBJSTORE_ENDPOINT is http://, so every captured log would cross the network in clear." >&2
        echo "         Use https, or set OBJSTORE_ALLOW_HTTP=1 if this is a MinIO on a loopback address." >&2
        return 1
      fi
      say "warn: the object store endpoint is http://, permitted by OBJSTORE_ALLOW_HTTP=1" ;;
    *)
      echo "station: OBJSTORE_ENDPOINT is '$OBJ_ENDPOINT', which is not a URL. It needs a scheme: https://s3.eu-west-2.amazonaws.com" >&2
      return 1 ;;
  esac

  OBJ_HOST="${OBJ_ENDPOINT#*://}"
  OBJ_HOST="${OBJ_HOST%%/*}"
  return 0
}

tp_scope() { printf '%s' "$OBJ_LANE"; }
tp_revision() { printf 'object store %s/%s, lane %s' "$OBJ_ENDPOINT" "$OBJ_BUCKET" "$OBJ_LANE"; }

# NEVER A CHARACTER OF THE SECRET. This string is printed by the preflight and
# ends up in a captured log, which is committed. The key id is not secret on its
# own - it is half of a pair, and it is the half that tells an operator WHICH
# credential is in use, which is the question this line exists to answer.
tp_describe() {
  printf 'object store %s/%s, lane %s, credential: access key %s… (%d chars)' \
    "$OBJ_ENDPOINT" "$OBJ_BUCKET" "$OBJ_LANE" \
    "$(printf '%s' "${OBJSTORE_KEY_ID}" | cut -c1-4)" "${#OBJSTORE_SECRET}"
}

# =============================================================================
#  SigV4
# =============================================================================
# Held to tests/fixtures/sigv4-vectors.json, which the Go signer emits. Every
# stage below is a field in that file, so a disagreement says WHICH stage rather
# than "403".

# RFC 3986 unreserved-set encoding: A-Za-z0-9 - _ . ~ and nothing else.
#
# NOT `curl --data-urlencode` AND NOT jq. Those encode a space as `+`, which
# AWS rejects, and leave `+` alone, which changes the signed string. Both are
# silent, and the result is a 403 on one key in a thousand - the one with a
# space in its name.
_obj_escape() {
  local s="$1" out="" i c
  for (( i = 0; i < ${#s}; i++ )); do
    c="${s:i:1}"
    case "$c" in
      [A-Za-z0-9._~-]) out="$out$c" ;;
      # LC_ALL=C so a multi-byte character is escaped byte by byte, which is
      # what AWS expects. Without it a UTF-8 name signs as something else.
      *) out="$out$(LC_ALL=C printf '%%%02X' "'$c")" ;;
    esac
  done
  printf '%s' "$out"
}

# Escape a path, keeping its separators. The control side splits on `/` and
# escapes each segment, so `/` survives and everything else does not.
_obj_escape_path() {
  local p="$1" out="" seg first=1 IFS='/'
  # shellcheck disable=SC2086
  set -- $p
  for seg in "$@"; do
    if [ "$first" = "1" ]; then first=0; out="$(_obj_escape "$seg")"
    else out="$out/$(_obj_escape "$seg")"; fi
  done
  # A trailing slash is a segment too, and losing it changes the key.
  case "$p" in */) out="$out/" ;; esac
  printf '%s' "$out"
}

_obj_sha256_hex() { printf '%s' "$1" | openssl dgst -sha256 -hex 2>/dev/null | awk '{print $NF}'; }
_obj_sha256_file() { openssl dgst -sha256 -hex < "$1" 2>/dev/null | awk '{print $NF}'; }

# HMAC-SHA256 with a HEX key, printing hex. The key is binary at every stage
# after the first, and passing it as a string would truncate it at the first NUL
# - which happens about once in every 256 derivations and produces a signature
# that is wrong only sometimes. `-macopt hexkey:` is the only form that carries
# the bytes intact.
_obj_hmac_hex() {
  printf '%s' "$2" | openssl dgst -sha256 -mac HMAC -macopt "hexkey:$1" -hex 2>/dev/null | awk '{print $NF}'
}

# The content type the Go side sends on every write, and therefore the one this
# side must send too. They are separate clients writing to one bucket, and an
# object whose type depends on which of them wrote it is a difference a reader
# can see. It is also a SIGNED header, so the two must agree byte for byte or
# the vectors do not line up.
OBJ_CONTENT_TYPE='text/plain; charset=utf-8'

# _obj_sign <method> <decoded-path> <canonical-query> <payload-hash> [content-type]
#
# Sets OBJ_AUTH and OBJ_AMZDATE. The caller passes the DECODED path: the
# canonical form is built by escaping it here, exactly as the Go side escapes
# u.Path rather than the raw URL.
_obj_sign() {
  local method="$1" path="$2" query="$3" payload="$4" ctype="${5:-}"
  local amzdate scopedate canon sts scope k hdrs signed

  amzdate="$(date -u +%Y%m%dT%H%M%SZ)"
  scopedate="${amzdate%%T*}"

  # LOWERCASE AND SORTED. `content-type` sorts before `host`, which sorts before
  # the two `x-amz-` ones - and the order is part of what is signed, so getting
  # it from the order somebody happened to write them in is a 403.
  #
  # HOST IS THE ONE PEOPLE MISS. It is never a header on an outgoing request,
  # it still has to be signed, and missing it fails with a 403 naming nothing.
  if [ -n "$ctype" ]; then
    hdrs="$(printf 'content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n' \
      "$ctype" "$OBJ_HOST" "$payload" "$amzdate")"
    signed="content-type;host;x-amz-content-sha256;x-amz-date"
  else
    hdrs="$(printf 'host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n' \
      "$OBJ_HOST" "$payload" "$amzdate")"
    signed="host;x-amz-content-sha256;x-amz-date"
  fi

  # `\n\n` BETWEEN THE HEADERS AND THE SIGNED LIST. The canonical form has a
  # BLANK LINE there - the header block ends with a newline of its own, and the
  # join adds another. `$( )` strips the trailing newline off $hdrs, so it has
  # to be put back here or the blank line disappears and every signature is
  # wrong. Go builds the same shape by joining a block that still ends in \n.
  canon="$(printf '%s\n%s\n%s\n%s\n\n%s\n%s' \
    "$method" "$(_obj_escape_path "$path")" "$query" "$hdrs" "$signed" "$payload")"

  scope="$scopedate/$OBJ_REGION/s3/aws4_request"
  sts="$(printf 'AWS4-HMAC-SHA256\n%s\n%s\n%s' "$amzdate" "$scope" "$(_obj_sha256_hex "$canon")")"

  # The four-stage derivation. The first key is the ASCII string "AWS4"+secret;
  # every stage after it is the previous stage's raw output, carried as hex.
  k="$(printf '%s' "$scopedate" | openssl dgst -sha256 -mac HMAC -macopt "key:AWS4${OBJSTORE_SECRET}" -hex 2>/dev/null | awk '{print $NF}')"
  k="$(_obj_hmac_hex "$k" "$OBJ_REGION")"
  k="$(_obj_hmac_hex "$k" "s3")"
  k="$(_obj_hmac_hex "$k" "aws4_request")"

  OBJ_SIGNATURE="$(_obj_hmac_hex "$k" "$sts")"
  OBJ_AMZDATE="$amzdate"
  OBJ_PAYLOAD="$payload"
  OBJ_AUTH="AWS4-HMAC-SHA256 Credential=${OBJSTORE_KEY_ID}/${scope}, SignedHeaders=${signed}, Signature=${OBJ_SIGNATURE}"
  # Exposed for the vector test, which asserts each stage separately - a
  # disagreement then says WHICH stage rather than "403".
  # shellcheck disable=SC2034
  OBJ_CANONICAL="$canon"
  # shellcheck disable=SC2034
  OBJ_STS="$sts"
  return 0
}

_obj_key() { printf '%s%s' "$OBJ_PREFIX" "$1"; }
_obj_path() { printf '/%s/%s' "$OBJ_BUCKET" "$(_obj_key "$1")"; }

# GET a key to stdout. 0 = got it, 1 = a real failure, 2 = it is not there.
#
# ABSENT IS NOT AN ERROR, AND IS NOT SUCCESS EITHER. A station that has never
# been sent anything has no request, and a 404 there is the ordinary state. A
# 403 is not, and collapsing the two is how a station polls a bucket it cannot
# read for ever while reporting itself idle.
_obj_get() {
  local path code out
  path="$(_obj_path "$1")"
  _obj_sign GET "$path" "" "$(_obj_sha256_hex "")" || return 1
  out="$(mktemp)" || return 1
  code="$(curl -sS -m 60 -o "$out" -w '%{http_code}' \
    -H "Host: $OBJ_HOST" \
    -H "X-Amz-Date: $OBJ_AMZDATE" \
    -H "X-Amz-Content-Sha256: $OBJ_PAYLOAD" \
    -H "Authorization: $OBJ_AUTH" \
    "${OBJ_ENDPOINT}${path}" 2>/dev/null)"
  case "$code" in
    200) cat "$out"; rm -f "$out"; return 0 ;;
    404) rm -f "$out"; return 2 ;;
    403)
      rm -f "$out"
      say "objstore: HTTP 403 reading ${path}. A wrong secret, a wrong region and a"
      say "          credential without read on this bucket all look exactly like this."
      return 1 ;;
    *) rm -f "$out"; say "objstore: HTTP $code reading ${path}"; return 1 ;;
  esac
}

_obj_put_file() {
  local key="$1" file="$2" path code
  path="$(_obj_path "$key")"
  _obj_sign PUT "$path" "" "$(_obj_sha256_file "$file")" "$OBJ_CONTENT_TYPE" || return 1
  code="$(curl -sS -m 120 -o /dev/null -w '%{http_code}' -X PUT \
    -H "Host: $OBJ_HOST" \
    -H "Content-Type: $OBJ_CONTENT_TYPE" \
    -H "X-Amz-Date: $OBJ_AMZDATE" \
    -H "X-Amz-Content-Sha256: $OBJ_PAYLOAD" \
    -H "Authorization: $OBJ_AUTH" \
    --data-binary "@$file" \
    "${OBJ_ENDPOINT}${path}" 2>/dev/null)"
  case "$code" in
    200 | 201) return 0 ;;
    403)
      say "objstore: HTTP 403 writing ${path}. A read-only credential looks exactly"
      say "          like a wrong one here; the store does not say which."
      return 1 ;;
    *) say "objstore: HTTP $code writing ${path}"; return 1 ;;
  esac
}

_obj_put_body() {
  local key="$1" body="$2" tmp rc
  tmp="$(mktemp)" || return 1
  printf '%s' "$body" > "$tmp"
  _obj_put_file "$key" "$tmp"; rc=$?
  rm -f "$tmp"
  return "$rc"
}

# =============================================================================
#  The contract
# =============================================================================

# PROVE THE WRITE, not merely the read.
#
# A GET would count a 404 as success on the argument that an absent request
# proves the bucket and the credential. It does not prove enough: a misspelt
# bucket answers 404, and a read-only key answers 200 to every read it will ever
# be asked for. Both clear a read-only check and then fail on the first status
# upload - an hour later, with nobody left to tell.
#
# NO DELETE afterwards. Deleting needs s3:DeleteObject, which a correctly scoped
# station credential need not have, so a check that deleted would fail on a
# credential that is perfectly good. One small object with a name that says what
# it is, overwritten by the next preflight.
tp_check() {
  local tmp rc
  tmp="$(mktemp)" || return 1
  printf 'heliograph write check\n' > "$tmp"
  _obj_put_file "heliograph-write-check" "$tmp"; rc=$?
  rm -f "$tmp"
  [ "$rc" = "0" ] || {
    say "the objstore transport could not write to ${OBJ_BUCKET}/${OBJ_PREFIX}."
    say "  A read-only key, a misspelt bucket and a wrong region all look like this."
    say "  The station would capture logs it could not ship."
    return 1
  }
  return 0
}

tp_fetch_request() {
  local body rc
  body="$(_obj_get "requests/${OBJ_LANE}.txt")"; rc=$?
  case "$rc" in
    0) printf '%s' "$body"; return 0 ;;
    2) return 0 ;;   # nothing queued, which is the ordinary state
    *) return 1 ;;
  esac
}

# Declared absent from tp_capabilities, so the loop never calls these. Defined
# so that calling one is a refusal rather than "command not found", which reads
# as a broken payload rather than as a transport saying no.
tp_fetch_request_live() { return 1; }
tp_fetch_self() { return 1; }

tp_put_status() {
  local body="$1" _msg="$2" alsofile="${3:-}"
  _obj_put_body "status/${OBJ_LANE}.txt" "$body" || return 1
  # A cancelled run's partial log. It is the last thing the far side will ever
  # see of that run, and losing it loses the only evidence there is. Under its
  # FINAL name, not `.partial.txt`: the run is over, and the control side hides
  # partials from its listing.
  if [ -n "$alsofile" ] && [ -f "$alsofile" ]; then
    _obj_put_file "logs/$(basename -- "$alsofile")" "$alsofile" || return 1
  fi
  return 0
}

# A SNAPSHOT OF A RUN STILL GOING, under `.partial.txt`.
#
# That suffix is load-bearing and belongs to the control side:
# internal/transport/objstore.go's ListLogs SKIPS anything ending in it, so a
# reader following a long run never sees a truncated file listed beside finished
# ones and mistakes it for the whole answer. Publishing progress under the final
# name would make every in-flight run look complete.
tp_put_progress() {
  local body="$1" _msg="$2" logfile="$3"
  _obj_put_body "status/${OBJ_LANE}.txt" "$body" || return 1
  [ -n "$logfile" ] && [ -f "$logfile" ] || return 0
  _obj_put_file "logs/$(basename -- "$logfile" .txt).partial.txt" "$logfile"
}

tp_put_log() {
  local logfile="$1"
  [ -f "$logfile" ] || return 1
  _obj_put_file "logs/$(basename -- "$logfile")" "$logfile"
}

tp_preflight() {
  say "objstore     $(tp_describe)"
  say "objstore     lane ${OBJ_LANE}, keys under ${OBJ_PREFIX}requests|status|logs"
  # THE CLOCK, because a signature covers it. AWS refuses a request more than
  # fifteen minutes out and says "RequestTimeTooSkewed", which is one of the few
  # honest errors in this area - but only if the operator sees it, and a station
  # nobody can log into does not print it anywhere else.
  say "objstore     signing as of $(date -u +%Y-%m-%dT%H:%M:%SZ); a clock more than 15 minutes out is refused by the store"
}
