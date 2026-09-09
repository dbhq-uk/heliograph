#!/usr/bin/env bash
# =============================================================================
#  test-service-transport.sh - a service can run a station that is not git
# =============================================================================
# A DETACHED PROCESS INHERITS NOTHING, and that fact is the whole subject.
#
# service.sh has warned about it for GIT_TOKEN since it was written: type it in
# a shell, install the service, and the loop starts perfectly, polls happily,
# and cannot push a single log. For a relay, share or blob station it was worse
# and unhandled - those have no fallback file the way caplib reads ~/.git-token,
# so there was no way to give a detached station its configuration at all, and
# `install` refused outright because it asked git for a remote the station was
# never going to use.
#
# THE ENVIRONMENT IS ASSERTED BY RUNNING IT, not by reading the command that
# would run. The first version of this file extracted functions with sed and
# asserted on strings, and it was green while the launchd and setsid mechanisms
# both DISCARDED every variable in the file - `. file` sets shell variables, and
# the very next thing either does is exec a new process, which inherits
# environment variables and not shell ones. A test that reads a command it never
# runs cannot see that. So the command is built by the real function and then
# executed against a start.sh that prints what it was given.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
ROOT="$(cd "$HERE/.." && pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

payload() {  # payload <dir> - a bootstrapped station with a start.sh that talks
  "$ROOT/station/bootstrap.sh" "$1" >/dev/null 2>&1
  cat > "$1/start.sh" <<'EOF'
#!/usr/bin/env bash
# Stands in for the real preflight. It reports what the ENVIRONMENT holds,
# which is the only thing these assertions are about.
echo "TRANSPORT=[${TRANSPORT:-}]"
echo "RELAY_URL=[${RELAY_URL:-}]"
echo "PIGEONHOLE_SAS=[${PIGEONHOLE_SAS:-}]"
echo "ARGS=[$*]"
EOF
  chmod +x "$1/start.sh"
}

# Run service.sh's OWN env-file wrapper, exactly as launchd and the setsid
# fallback build it, and see what start.sh actually received.
#
# THE EXTRACTION IS BY EXACT FUNCTION, and a one-liner is taken as one LINE.
# `/^sh_quote()/,/^}/` looks right and is not: sh_quote is a single line, so the
# range runs on to the NEXT function's closing brace and prints those lines
# twice - which nests a function inside itself, defines nothing when called, and
# made every assertion here fail for a reason that had nothing to do with the
# code under test.
through_wrapper() {  # through_wrapper <repo> [start args...]
  local repo="$1"; shift
  OUT="$( cd "$repo" && bash -c '
      REPO_ROOT="$PWD"
      STATION_ENV="$REPO_ROOT/.station-env"
      # shellcheck disable=SC1091
      source <(sed -n "/^sh_quote()/p;/^env_wrapped_prefix()/,/^}/p" ./service.sh)
      cmd="$(env_wrapped_prefix)exec bash $(sh_quote "$REPO_ROOT/start.sh")"
      for a in "$@"; do cmd="$cmd $(sh_quote "$a")"; done
      bash -c "$cmd"
    ' _ "$@" 2>&1 )"
}

# --- THE ONE THAT MATTERS: the variables actually arrive ---------------------
payload "$TMP/relay"
cat > "$TMP/relay/.station-env" <<'EOF'
# a comment, and a blank line follow
TRANSPORT='relay'
RELAY_URL='https://relay.invalid'
EOF
chmod 600 "$TMP/relay/.station-env"
through_wrapper "$TMP/relay"
assert_contains "TRANSPORT reaches start.sh, which is a NEW PROCESS and inherits only the environment" \
  "TRANSPORT=[relay]" "$OUT"
assert_contains "and so does the transport's own variable" \
  "RELAY_URL=[https://relay.invalid]" "$OUT"

# An Azure SAS is full of ampersands, and this file is SOURCED by two of the
# three mechanisms. Unquoted, the shell backgrounds a job at the first '&' and
# the rest of the credential is gone.
payload "$TMP/sas"
printf "TRANSPORT='blob'\nPIGEONHOLE_SAS='?sv=2021&ss=b&sig=abc%%3D'\n" > "$TMP/sas/.station-env"
chmod 600 "$TMP/sas/.station-env"
through_wrapper "$TMP/sas"
assert_contains "a quoted value survives the shell whole, ampersands and all" \
  'PIGEONHOLE_SAS=[?sv=2021&ss=b&sig=abc%3D]' "$OUT"

# Arguments still reach start.sh, quoted, alongside the environment.
through_wrapper "$TMP/relay" --once "--interval" "15"
assert_contains "start.sh arguments survive the wrapper" "ARGS=[--once --interval 15]" "$OUT"

# A repo path with a space in it, because the wrapper is a `bash -c` string.
mkdir -p "$TMP/a dir"
payload "$TMP/a dir/repo"
printf "TRANSPORT='relay'\n" > "$TMP/a dir/repo/.station-env"
through_wrapper "$TMP/a dir/repo"
assert_contains "a path with a space in it does not split the wrapper" "TRANSPORT=[relay]" "$OUT"

# --- the unit reads the env file ---------------------------------------------
write_unit_for() {  # write_unit_for <repo> -> prints the unit
  ( cd "$1" && bash -c '
      REPO_ROOT="$PWD"
      UNIT_DIR="'"$TMP"'/units"
      UNIT_PATH="$UNIT_DIR/x.service"
      STATION_ENV="$REPO_ROOT/.station-env"
      START_ARGS=()
      # shellcheck disable=SC1091
      source <(sed -n "/^write_unit()/,/^}/p" ./service.sh)
      write_unit
      cat "$UNIT_PATH"
    ' )
}
UNIT="$(write_unit_for "$TMP/relay")"
assert_contains "the unit reads an EnvironmentFile, or a detached relay station has no configuration at all" \
  "EnvironmentFile=" "$UNIT"
assert_contains "and it names the file beside the payload" ".station-env" "$UNIT"
# The leading dash is not decoration: systemd REFUSES to start a unit whose
# EnvironmentFile is missing unless it is marked optional, and a git station
# keeping its token in ~/.git-token has no such file.
assert_contains "marked optional, so a git station with no such file still starts" \
  "EnvironmentFile=-" "$UNIT"
assert_eq "and the env file's contents are NOT inlined into the unit, where systemctl cat would print them" \
  "" "$(printf '%s' "$UNIT" | grep -o 'relay.invalid')"

# The LaunchAgent must not carry them either: a plist is world-readable.
plist_for() {
  ( cd "$1" && bash -c '
      REPO_ROOT="$PWD"
      STATION_ENV="$REPO_ROOT/.station-env"
      LOG_FILE="$REPO_ROOT/.log"
      START_ARGS=()
      # shellcheck disable=SC1091
      source <(sed -n "/^sh_quote()/p;/^env_wrapped_prefix()/,/^}/p;/^xml_escape()/,/^}/p" ./service.sh)
      cmd="$(env_wrapped_prefix)exec bash $(sh_quote "$REPO_ROOT/start.sh")"
      printf "%s" "$(xml_escape "$cmd")"
    ' )
}
PLIST_CMD="$(plist_for "$TMP/relay")"
assert_eq "the LaunchAgent command carries no credential: a plist is world-readable" \
  "" "$(printf '%s' "$PLIST_CMD" | grep -o 'relay.invalid')"
assert_contains "it sources the file instead" ".station-env" "$PLIST_CMD"
assert_contains "with set -a, or the exec below it would inherit nothing" "set -a" "$PLIST_CMD"

# --- the credential check follows the TRANSPORT ------------------------------
check() {  # check <repo> - runs credential_check, prints its output, sets RC
  RC=0
  OUT="$( cd "$1" && bash -c '
      REPO_ROOT="$PWD"
      say()  { printf "%s\n" "$*"; }
      warn() { printf "%s\n" "$*"; }
      STATION_ENV="$REPO_ROOT/.station-env"
      # shellcheck disable=SC1091
      source <(sed -n "/^env_file_lines()/,/^}/p;/^validate_env_file()/,/^}/p;/^env_file_value()/,/^}/p;/^station_transport()/,/^}/p;/^transport_needs()/,/^}/p;/^transport_env_check()/,/^}/p;/^credential_check()/,/^}/p" ./service.sh)
      credential_check
    ' 2>&1 )" || RC=$?
}

payload "$TMP/nogit"
check "$TMP/nogit"
assert_eq "a git station with no remote is still refused, which is right" "1" "$RC"
assert_contains "and told so in git's terms" "origin remote" "$OUT"

# --- FAIL CLOSED: every variable the transport asks for ----------------------
# Checking only that the file exists was fail-open in the worst way: a file
# holding TRANSPORT=relay and nothing else installed cleanly and produced a
# service that could not start, restarting on a timer, unwatched.
payload "$TMP/thin"
printf "TRANSPORT='relay'\n" > "$TMP/thin/.station-env"
chmod 600 "$TMP/thin/.station-env"
check "$TMP/thin"
assert_eq "a relay env file with only TRANSPORT in it is refused" "1" "$RC"
for want in RELAY_URL RELAY_TOKEN RELAY_IDENTITY RELAY_PEER; do
  assert_contains "and it names $want, which transports/relay.sh asks for with cap_need" "$want" "$OUT"
done

# The same file, complete. The names come out of the transport itself, so this
# also proves transport_needs is reading the right thing.
payload "$TMP/full"
{
  printf "TRANSPORT='relay'\n"
  for v in $(sed -n 's/^[[:space:]]*cap_need[[:space:]]\{1,\}\([A-Z_][A-Z0-9_]*\).*/\1/p' \
               "$TMP/full/transports/relay.sh" | sort -u); do
    printf "%s='x'\n" "$v"
  done
} > "$TMP/full/.station-env"
chmod 600 "$TMP/full/.station-env"
check "$TMP/full"
assert_eq "a complete relay env file installs" "0" "$RC"
assert_contains "and says which transport it read" "relay" "$OUT"
assert_eq "and never mentions a git remote it was never going to use" "" \
  "$(printf '%s' "$OUT" | grep -o 'origin remote')"

# --- a transport that does not exist -----------------------------------------
# A typo installs a service that cannot start and retries for ever.
payload "$TMP/typo"
printf "TRANSPORT='realy'\n" > "$TMP/typo/.station-env"
chmod 600 "$TMP/typo/.station-env"
check "$TMP/typo"
assert_eq "a misspelt transport is refused before anything is installed" "1" "$RC"
assert_contains "and it lists what this payload actually ships" "relay" "$OUT"

# --- the file must be readable by BOTH parsers -------------------------------
# systemd reads it with EnvironmentFile; launchd and setsid source it in a
# shell. A line that means different things to the two is refused, quoted back.
payload "$TMP/badsas"
printf "TRANSPORT='blob'\nPIGEONHOLE_SAS=?sv=2021&ss=b&sig=abc\n" > "$TMP/badsas/.station-env"
chmod 600 "$TMP/badsas/.station-env"
check "$TMP/badsas"
assert_eq "an unquoted value with an ampersand in it is refused" "1" "$RC"
assert_contains "and the message shows how to write it" "PIGEONHOLE_SAS='...'" "$OUT"

payload "$TMP/exported"
printf "export TRANSPORT='relay'\n" > "$TMP/exported/.station-env"
chmod 600 "$TMP/exported/.station-env"
check "$TMP/exported"
assert_eq "'export' is refused: valid shell, and invalid to systemd" "1" "$RC"
assert_contains "and it says which of the two would reject it" "EnvironmentFile" "$OUT"

payload "$TMP/spaces"
printf "TRANSPORT = relay\n" > "$TMP/spaces/.station-env"
chmod 600 "$TMP/spaces/.station-env"
check "$TMP/spaces"
assert_eq "spaces around the '=' are refused: valid to neither parser" "1" "$RC"

# A file edited on Windows. Without the CR strip, TRANSPORT becomes 'relay\r',
# which names no transport and produces a service that cannot start.
payload "$TMP/crlf"
printf "TRANSPORT='relay'\r\nRELAY_URL='https://r.invalid'\r\n" > "$TMP/crlf/.station-env"
chmod 600 "$TMP/crlf/.station-env"
check "$TMP/crlf"
assert_eq "a CRLF file does not turn the transport name into one nothing ships" "" \
  "$(printf '%s' "$OUT" | grep -o 'no /.*transports/relay..sh')"

# --- the env file is read from the FILE, not from this shell -----------------
# The service sees the file and not this shell, so the file is what the check
# has to reason about.
payload "$TMP/onlyshell"
RC=0
OUT="$( cd "$TMP/onlyshell" && TRANSPORT=relay bash -c '
    REPO_ROOT="$PWD"
    say()  { printf "%s\n" "$*"; }
    warn() { printf "%s\n" "$*"; }
    STATION_ENV="$REPO_ROOT/.station-env"
    # shellcheck disable=SC1091
    source <(sed -n "/^env_file_lines()/,/^}/p;/^validate_env_file()/,/^}/p;/^env_file_value()/,/^}/p;/^station_transport()/,/^}/p;/^transport_needs()/,/^}/p;/^transport_env_check()/,/^}/p;/^credential_check()/,/^}/p" ./service.sh)
    credential_check
  ' 2>&1 )" || RC=$?
assert_eq "TRANSPORT exported in this shell alone still blocks: the service would not see it" "1" "$RC"
assert_contains "and it says where the variables have to be" ".station-env" "$OUT"

# --- a world-readable env file is reported -----------------------------------
cp "$TMP/full/.station-env" "$TMP/full/.station-env.bak"
chmod 644 "$TMP/full/.station-env"
check "$TMP/full"
assert_eq "a world-readable env file does not block the install" "0" "$RC"
assert_contains "but it is reported, because it holds a token" "chmod 600" "$OUT"
chmod 600 "$TMP/full/.station-env"
check "$TMP/full"
assert_eq "and a 600 file is not complained about" "" "$(printf '%s' "$OUT" | grep -o 'chmod 600')"

t_summary
