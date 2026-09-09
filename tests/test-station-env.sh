#!/usr/bin/env bash
# =============================================================================
#  test-station-env.sh - the rules for .station-env, in the one place they live
# =============================================================================
# A DETACHED PROCESS INHERITS NOTHING from the shell that installed it. For a
# relay, share or blob station that is fatal rather than inconvenient: those
# have no fallback file the way caplib reads ~/.git-token, so .station-env is
# the only way their variables reach a service at all.
#
# THREE THINGS READ THAT FILE and they do not agree with each other. systemd
# reads it with EnvironmentFile; launchd, the setsid fallback and station.ps1
# all SOURCE it in a shell. An ordinary Azure SAS is a value to one and three
# background jobs to the other.
#
# So one script owns the rules - station-env.sh - and both installers call it.
# The first version had the rules TWICE, once in bash and once in PowerShell,
# and an adversarial read found six ways the two classified the same file
# differently: PowerShell regexes are case-insensitive by default, Get-Content
# eats a UTF-8 BOM that bash does not skip when sourcing, an empty file passed
# one and failed the other. A station that installs on Windows and is refused on
# Linux, from one file, is worse than either answer on its own.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
ROOT="$(cd "$HERE/.." && pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
TR="$TMP/payload"
"$ROOT/station/bootstrap.sh" "$TR" >/dev/null 2>&1
mkdir -p "$TR/probe"

env_check() {  # env_check <file-contents> -> RC, OUT
  printf '%s' "$1" > "$TR/.station-env"
  chmod 600 "$TR/.station-env"
  RC=0
  OUT="$( cd "$TR" && ./station-env.sh 2>&1 )" || RC=$?
}

# It asks the TRANSPORT rather than scraping cap_need names, which is the only
# thing that gets blob right: it needs PIGEONHOLE_SAS *or* a managed identity,
# and no cap_need line expresses that.
env_check "TRANSPORT='blob'
PIGEONHOLE_ACCOUNT='a'
PIGEONHOLE_LANE='l'
"
assert_eq "a blob station with neither a SAS nor an identity is refused" "1" "$RC"
assert_contains "and the transport's own words say why" "PIGEONHOLE_SAS" "$OUT"

env_check "TRANSPORT='blob'
PIGEONHOLE_ACCOUNT='a'
PIGEONHOLE_LANE='l'
PIGEONHOLE_SAS='sv=x'
"
assert_eq "and with a SAS it is accepted" "0" "$RC"

env_check "TRANSPORT='relay'
"
assert_eq "a relay env file with only TRANSPORT is refused" "1" "$RC"
assert_contains "and names the first thing missing" "RELAY_URL" "$OUT"

# THE NAME BECOMES A FILENAME THAT GETS SOURCED. This uses caplib's own
# validator, the one start.sh uses, so an install cannot succeed for a station
# start.sh will refuse.
env_check "TRANSPORT='../transports/relay'
"
assert_eq "a transport name that is a path is refused" "1" "$RC"
assert_contains "because the name itself is the problem" "sourced" "$OUT"

env_check "TRANSPORT='realy'
"
assert_eq "a misspelt transport is refused" "1" "$RC"
assert_contains "and this payload's real transports are listed" "relay" "$OUT"

# `FOO=x true` is a TEMPORARY assignment in front of a command: bash runs `true`
# with FOO set for that one command and keeps nothing. Both earlier
# implementations called this file usable.
env_check "TRANSPORT='relay'
RELAY_TOKEN=x true
"
assert_eq "unquoted whitespace in a value is refused" "1" "$RC"
assert_contains "and it says what a shell would actually do with it" \
  "keeps nothing" "$OUT"

env_check "TRANSPORT='blob'
PIGEONHOLE_SAS=?sv=x&ss=b
"
assert_eq "an unquoted ampersand is refused" "1" "$RC"

env_check "export TRANSPORT='relay'
"
assert_eq "'export' is refused: valid shell, invalid to systemd" "1" "$RC"

env_check ""
assert_eq "an empty file is refused rather than read as a git station" "1" "$RC"

# A BOM. bash does NOT skip it when sourcing, so the first assignment becomes an
# unknown command and never happens - while PowerShell's Get-Content eats it
# silently, which is how the two implementations disagreed about a file the
# station could not use.
printf '\xEF\xBB\xBFTRANSPORT=%s\n' "'relay'" > "$TR/.station-env"
RC=0; OUT="$( cd "$TR" && ./station-env.sh 2>&1 )" || RC=$?
assert_eq "a UTF-8 BOM is refused" "1" "$RC"
assert_contains "and it says bash will not skip it" "byte order mark" "$OUT"

# CRLF IS REFUSED, and this assertion started life claiming the opposite.
#
# It was written expecting a CRLF file to be accepted, because the validator
# strips a CR when it READS. Nothing strips it when a service SOURCES the file:
# SHARE_SCOPE became `probe` plus a carriage return and the share transport
# refused it - after installation. The test failing is what found that.
mkdir -p "$TR/probe"
env_check "$(printf "TRANSPORT='share'\r\nSHARE_DIR='%s'\r\nSHARE_SCOPE='probe'\r\n" "$TR")"
assert_eq "a CRLF file is refused, because sourcing does not strip the CR" "1" "$RC"
assert_contains "and it says how to fix the file" "line endings" "$OUT"

# The same file with Unix endings is fine, or the check above is just refusing
# everything.
env_check "$(printf "TRANSPORT='share'\nSHARE_DIR='%s'\nSHARE_SCOPE='probe'\n" "$TR")"
assert_eq "and with Unix line endings it is accepted" "0" "$RC"
assert_contains "naming the transport it read" "transport: share" "$OUT"
rm -f "$TR/.station-env"

t_summary
