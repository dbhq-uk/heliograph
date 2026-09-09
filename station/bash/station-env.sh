#!/usr/bin/env bash
# =============================================================================
#  station-env.sh - is this station's .station-env usable, before it is installed
# =============================================================================
#     ./station-env.sh              # check, report, exit 0 or 1
#     ./station-env.sh --transport  # print the transport it names, and nothing else
#
#  A DETACHED PROCESS INHERITS NOTHING from the shell that installed it. That
#  has always been true of GIT_TOKEN - service.sh has warned about it since it
#  was written - and for a relay, share or blob station it is worse: those have
#  no fallback file the way caplib reads ~/.git-token, so this file is the only
#  way their variables can reach a service at all.
#
#  ONE IMPLEMENTATION, AND THIS IS IT. service.sh and service.ps1 both call this
#  rather than each carrying its own copy of the rules, and the reason is
#  measured rather than tidy: the first version DID have two, and an adversarial
#  read found six ways they classified the same file differently - PowerShell's
#  regexes are case-insensitive by default, so `transport=relay` passed on
#  Windows and set nothing in bash; Get-Content eats a UTF-8 BOM that bash does
#  not; an empty file passed one and failed the other. A station that installs
#  on Windows and is refused on Linux, from the same file, is worse than either
#  answer on its own.
#
#  IT IS BASH BECAUSE THE FILE IS BASH. systemd reads it with EnvironmentFile;
#  launchd, the setsid fallback and station.ps1 all SOURCE it. The authority on
#  what a shell will do with a line is a shell.
#
#  WHAT IT DOES NOT DO: reach the network. `./start.sh --check` asks the
#  transport whether the values WORK, which needs the far side. This asks the
#  one thing start.sh cannot - whether a detached process will be handed them at
#  all - and then asks the transport's own tp_init, which is local by contract.
# =============================================================================
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATION_ENV="${STATION_ENV:-$REPO_ROOT/.station-env}"
MODE="${1:-}"

say()  { printf '%s\n' "$*"; }
warn() { printf '%s\n' "$*" >&2; }

# The file's lines, comments and blanks dropped.
#
# A BOM IS NOT WHITESPACE. A file saved by Notepad as "UTF-8" begins with EF BB
# BF, and bash does not skip it when sourcing: the first line becomes an unknown
# command and the assignment on it never happens. PowerShell's Get-Content
# silently swallows it, which is exactly how the two implementations disagreed
# about a file the station could not use.
env_lines() {
  [ -r "$STATION_ENV" ] || return 0
  sed -e '1s/^\xEF\xBB\xBF//' -e 's/\r$//' -e 's/^[[:space:]]*//' \
      -e '/^#/d' -e '/^$/d' "$STATION_ENV" 2>/dev/null
}

has_crlf() {
  [ -r "$STATION_ENV" ] || return 1
  grep -q $'\r$' "$STATION_ENV" 2>/dev/null
}

has_bom() {
  [ -r "$STATION_ENV" ] || return 1
  [ "$(head -c 3 "$STATION_ENV" 2>/dev/null | od -An -tx1 | tr -d ' \n')" = "efbbbf" ]
}

env_value() {  # env_value <KEY>
  local v
  v="$(env_lines | sed -n "s/^$1=//p" | tail -1)"
  case "$v" in
    \'*\') v="${v#\'}"; v="${v%\'}" ;;
    \"*\") v="${v#\"}"; v="${v%\"}" ;;
  esac
  printf '%s' "$v"
}

# ONE FILE, THREE PARSERS. systemd's EnvironmentFile does no expansion; a shell
# does. An ordinary Azure SAS - ?sv=...&ss=...&sig=... - is a value to systemd
# and three background jobs to a shell. So the file is held to the intersection:
# KEY=value, the value either single-quoted or free of anything a shell acts on.
validate() {
  local line key value bad=0 n=0
  # CRLF IS REFUSED, not stripped, and the difference is the whole point.
  #
  # This script can strip a CR when it READS the file. Nothing strips it when a
  # service SOURCES it: `SHARE_SCOPE='probe'\r` sets the variable to `probe` and
  # a carriage return, and share.sh then refuses it as an unusable scope - after
  # installation, on a machine nobody is watching.
  #
  # Found by this file's own test, which asserted a CRLF file was ACCEPTED and
  # then watched the transport refuse it. Stripping on read was papering over
  # the one thing that matters. start.sh's preflight has taken the same line on
  # CRLF since it was written, and for the same reason.
  if has_crlf; then
    warn "$STATION_ENV has Windows line endings."
    warn "  This file is SOURCED by a shell, and nothing strips the CR when it is:"
    warn "  TRANSPORT='relay' becomes 'relay' followed by a carriage return, which"
    warn "  names no transport that ships. Save it with Unix line endings, or:"
    warn "      sed -i 's/\\r\$//' $STATION_ENV"
    bad=1
  fi
  if has_bom; then
    warn "$STATION_ENV begins with a UTF-8 byte order mark."
    warn "  bash does not skip it when sourcing, so the first assignment becomes"
    warn "  an unknown command and never happens. Save it as UTF-8 without a BOM."
    bad=1
  fi
  while IFS= read -r line; do
    n=$((n + 1))
    case "$line" in *=*) ;; *)
      warn "$STATION_ENV line $n is not a KEY=value assignment: $line"
      bad=1; continue ;;
    esac
    key="${line%%=*}"
    value="${line#*=}"
    # `export FOO=x` is valid shell and invalid to systemd. `FOO = x` is valid
    # to neither, and a parser that merely looked for the name accepted it and
    # then did not set it.
    case "$key" in
      [A-Za-z_]*) ;;
      *) warn "$STATION_ENV line $n has no usable variable name: $line"; bad=1; continue ;;
    esac
    case "$key" in
      *[!A-Za-z0-9_]*)
        warn "$STATION_ENV line $n: '$key' is not a variable name. No 'export',"
        warn "  and no spaces around the '=' - systemd's EnvironmentFile takes neither."
        bad=1; continue ;;
    esac
    case "$value" in
      \'*\')
        case "${value#\'}" in
          *\'*\'*) warn "$STATION_ENV line $n: nested single quote in $key"; bad=1 ;;
        esac ;;
      *[\ \	]*)
        # `FOO=x true` is a TEMPORARY assignment in front of a command: bash
        # runs `true` with FOO set for that one command and keeps nothing.
        # Neither implementation caught it, and both called the file usable.
        warn "$STATION_ENV line $n: $key has unquoted whitespace in its value."
        warn "  A shell reads 'FOO=x y' as running 'y' with FOO set for that one"
        warn "  command, and keeps nothing. Wrap it:  $key='...'"
        bad=1 ;;
      *[\$\`\&\;\|\<\>\(\)\"\\]*|*\'*)
        warn "$STATION_ENV line $n: $key holds a character a shell would act on."
        warn "  launchd, the setsid fallback and station.ps1 all SOURCE this file,"
        warn "  so an unquoted '&' backgrounds a job and loses the rest. Wrap it:"
        warn "      $key='...'"
        bad=1 ;;
    esac
  done < <(env_lines)
  if [ "$n" = "0" ]; then
    warn "$STATION_ENV is empty, so it configures nothing."
    return 1
  fi
  return "$bad"
}

transport_of() {
  local t
  t="$(env_value TRANSPORT)"
  printf '%s' "${t:-git}"
}

if [ "$MODE" = "--transport" ]; then
  [ -r "$STATION_ENV" ] && validate >/dev/null 2>&1
  transport_of
  exit 0
fi

# --- the checks ---------------------------------------------------------------
if [ -e "$STATION_ENV" ] && ! validate; then
  warn "  Fix those lines and run this again. Nothing has been installed."
  exit 1
fi

TRANSPORT_NAME="$(transport_of)"

if [ "$TRANSPORT_NAME" = "git" ]; then
  # The git credential chain is caplib's, and service.sh reports it. Nothing to
  # add here: this file exists for the transports that have no such chain.
  say "transport: git"
  exit 0
fi

if [ ! -r "$STATION_ENV" ]; then
  warn "TRANSPORT is '$TRANSPORT_NAME', and a detached service inherits nothing"
  warn "  from the shell that installed it. Its variables have to be somewhere"
  warn "  the service can read: $STATION_ENV, one KEY='value' per line, mode 600."
  exit 1
fi

# THE NAME BECOMES A FILENAME THAT GETS SOURCED, so caplib validates it. Without
# this, `TRANSPORT=../transports/relay` resolved to the shipped file and passed,
# while start.sh - which uses the same validator - refuses it outright. An
# install that cannot possibly start is worse than a refusal.
# shellcheck source=caplib.sh disable=SC1091
. "$REPO_ROOT/caplib.sh" 2>/dev/null || {
  warn "cannot read $REPO_ROOT/caplib.sh, so the transport name cannot be validated"
  exit 1
}
TP_FILE="$(cap_transport_file "$TRANSPORT_NAME" 2>&1)" || {
  warn "TRANSPORT is '$TRANSPORT_NAME', which this payload cannot use."
  warn "$TP_FILE"
  shipped=""
  for f in "$REPO_ROOT"/transports/*.sh; do
    [ -f "$f" ] || continue
    f="$(basename "$f" .sh)"
    shipped="${shipped:+$shipped, }$f"
  done
  warn "  This payload ships: ${shipped:-none}"
  warn "  A typo here installs a service that cannot start and retries for ever."
  exit 1
}

# ASK THE TRANSPORT, in a subshell, with the file loaded exactly as a service
# would load it.
#
# tp_init is the transport's OWN local validation - "validate what this
# transport needs locally, and fail with a remedy rather than a symptom" - and
# it is the only thing that gets this right. Scraping `cap_need` names was the
# first attempt and it was a floor rather than an answer: the blob transport
# needs PIGEONHOLE_SAS *or* a managed identity, which no cap_need line
# expresses, so a file with the two names it does declare passed here and was
# refused by start.sh.
#
# It touches no network by contract. Everything that does is tp_check's.
out="$(
  set -a
  # shellcheck disable=SC1090
  . "$STATION_ENV"
  set +a
  # shellcheck disable=SC1090
  . "$TP_FILE"
  tp_init 2>&1
)" || {
  warn "TRANSPORT is '$TRANSPORT_NAME' and it will not initialise from $STATION_ENV:"
  printf '%s\n' "$out" | sed 's/^ *station: */  /; s/^/  /' >&2
  warn ""
  warn "  A detached service sees only that file. It would start, fail its own"
  warn "  preflight, and be restarted on a timer for ever."
  exit 1
}

# A file anybody can read is a token anybody can read. `-rw-------` and stricter.
# Ten characters: the type, then owner, group and other, so a file only its
# owner can touch has six dashes from position five.
mode="$(ls -ld -- "$STATION_ENV" 2>/dev/null | cut -c1-10)"
case "$mode" in
  ????------) : ;;
  "")         warn "cannot read the permissions of $STATION_ENV" ;;
  *)
    warn "$STATION_ENV is readable or writable beyond its owner ($mode)."
    warn "  It holds this station's transport credential. chmod 600 it." ;;
esac

say "transport: $TRANSPORT_NAME, configured in $STATION_ENV"
exit 0
