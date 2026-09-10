#!/usr/bin/env bash
# =============================================================================
#  test-transports-ps1.sh - what the PowerShell transports refuse, and never say
# =============================================================================
# The conformance suite asks whether a log ARRIVES. This asks the things that
# have no property because they are not about the capture at all:
#
#   what a transport REFUSES        a name that becomes a path, a scope that
#                                   becomes a document field
#   what it never SAYS              a credential, in the one string both the
#                                   preflight and the startup banner print
#   what it does when it FAILS      reports a failure. A delivery that returns
#                                   success having shipped nothing is the exact
#                                   defect property 9 exists for, and p9 cannot
#                                   see it from the far side of a channel that
#                                   is working.
#
# Every one of these is a rule the bash transports already carry. Two
# implementations of one rule need a test that compares them, and where a
# behaviour cannot be compared directly it is at least asserted twice.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
PSDIR="$(cd "$HERE/../station/powershell" && pwd)"

PS_CANDIDATES=()
[ -n "${CONF_PS_SHELL:-}" ] && PS_CANDIDATES+=("$CONF_PS_SHELL")
PS_CANDIDATES+=(pwsh powershell powershell.exe)
PS_BIN=""
for c in "${PS_CANDIDATES[@]}"; do
  if command -v "$c" >/dev/null 2>&1; then PS_BIN="$c"; break; fi
done
if [ -z "$PS_BIN" ]; then
  t_skip "no PowerShell interpreter: the transports were NOT exercised."
  t_summary
  exit 0
fi
t_ok "a PowerShell interpreter is present ($PS_BIN), so the assertions below ran"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
winpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s' "$1"; fi
}

# tp <ps-body> <env...>  -> TP_OUT, TP_RC. The body runs with lib/transport
# imported, which is how run.ps1 and start.ps1 reach a transport.
tp() {
  local body="$1"; shift
  TP_OUT="$( cd "$WORK" && env -u TRANSPORT -u SHARE_DIR -u SHARE_SCOPE \
               -u GIT_TOKEN -u GIT_TOKEN_FILE -u GIT_AUTH_HEADER -u GIT_TOKEN_USER \
               "$@" "$PS_BIN" -NoProfile -Command "
      Import-Module '$(winpath "$PSDIR/caplib.psm1")' -Force
      Import-Module '$(winpath "$PSDIR/lib/transport.psm1")' -Force
      $body" 2>&1 )"
  # shellcheck disable=SC2034  # read by callers that care about the code
  TP_RC=$?
}

mkdir -p "$WORK/mnt"
SHARE_OK=(TRANSPORT=share "SHARE_DIR=$(winpath "$WORK/mnt")" SHARE_SCOPE=inv)

# --- the transport NAME becomes a path that gets imported --------------------
# It can come from a .station-env, which is a file the far side may have
# written. A name that escapes the directory runs somebody else's code with the
# station's credentials.
for bad in '../../evil' '/etc/passwd' 'Share' 'sh are' 'x;y' ''; do
  tp "if (Import-Tp -Name '$bad') { 'ACCEPTED' } else { 'refused' }"
  case "$TP_OUT" in
    *ACCEPTED*) t_no "the transport name [$bad] was ACCEPTED, and it becomes a path that gets imported" ;;
    *) t_ok "the transport name [$bad] is refused" ;;
  esac
done

tp "if (Import-Tp -Name 'share') { 'loaded' }" "${SHARE_OK[@]}"
assert_contains "and an ordinary name still loads, so the guard is not refusing everything" \
  "loaded" "$TP_OUT"

# --- the SCOPE becomes a document field, not just a directory ----------------
# THE REASON IS NOT PATH TRAVERSAL ALONE. The scope is published as the status
# document's `branch:` value, and that document is line-oriented `key: value`.
# A scope holding a NEWLINE injects a second key, and the parser keeps the last
# - so a scope of "x\nstate: running" makes an idle station report itself busy
# for ever. transports/share.sh and internal/transport/share.go refuse exactly
# this set.
newline_scope="$(printf 'x\nstate: running')"
for bad in '../..' '.' '..' '-flag' 'a/b' 'a b' "$newline_scope"; do
  tp "if (Import-Tp) { 'ACCEPTED' } else { 'refused' }" \
    TRANSPORT=share "SHARE_DIR=$(winpath "$WORK/mnt")" "SHARE_SCOPE=$bad"
  case "$TP_OUT" in
    *ACCEPTED*) t_no "SHARE_SCOPE [$bad] was ACCEPTED: it becomes a directory AND the status document's branch field" ;;
    *) t_ok "SHARE_SCOPE [$(printf '%s' "$bad" | tr '\n' '~')] is refused" ;;
  esac
done

# --- a missing variable is NAMED, with what it is for ------------------------
# The operator is on the far side of a gap. "share failed" costs a round trip;
# "SHARE_SCOPE is not set, and it is what a run is bound to" does not.
tp "if (Import-Tp) { 'loaded' }" TRANSPORT=share "SHARE_DIR=$(winpath "$WORK/mnt")"
assert_contains "a missing SHARE_SCOPE is named" "SHARE_SCOPE" "$TP_OUT"
assert_contains "and says what it is FOR, not just that it is missing" \
  "what a run is bound to" "$TP_OUT"

tp "if (Import-Tp) { 'loaded' }" TRANSPORT=share SHARE_SCOPE=inv
assert_contains "a missing SHARE_DIR is named too" "SHARE_DIR" "$TP_OUT"

# --- a share that is not there is a refusal, not a directory that gets made ---
tp "if (Import-Tp) { 'ACCEPTED' } else { 'refused' }" \
  TRANSPORT=share "SHARE_DIR=$(winpath "$WORK/nosuchmount")" SHARE_SCOPE=inv
assert_contains "a SHARE_DIR that is not mounted is refused rather than created" \
  "refused" "$TP_OUT"
if [ -d "$WORK/nosuchmount" ]; then
  t_no "and it CREATED the missing share, which hides an unmounted volume"
else
  t_ok "and nothing was created, so an unmounted volume stays visible as one"
fi

# --- Initialize-Tp CREATES NOTHING, because --check calls it -----------------
before="$(find "$WORK/mnt" | LC_ALL=C sort)"
tp "if (Import-Tp) { 'loaded' }" "${SHARE_OK[@]}"
after="$(find "$WORK/mnt" | LC_ALL=C sort)"
if [ "$before" = "$after" ]; then
  t_ok "Initialize-Tp created nothing, so --check can call it where nothing may change"
else
  t_no "Initialize-Tp created something, and --check calls it:"
  diff <(printf '%s\n' "$before") <(printf '%s\n' "$after") | sed 's/^/     /' | head -5
fi

# --- a FAILED delivery is reported as a failure ------------------------------
# THE DEFECT PROPERTY 9 EXISTS FOR, and p9 cannot see it: from the far side of a
# working channel, a delivery that succeeded and one that lied are the same
# picture. The only way to ask is to break the channel.
printf 'a log\n' > "$WORK/mnt/fake.txt"
# THE CONTROL, and it needs a real needle: `assert_contains ""` matches
# everything, so the first version of this line passed on any output at all -
# including a failure. Without a control, "delivery reports a failure" is
# satisfied by a transport that can never deliver anything.
tp "if (Import-Tp) { if (Send-TpLog -LogPath '$(winpath "$WORK/mnt/fake.txt")') { 'CLAIMED-OK' } else { 'reported-failure' } }" \
  TRANSPORT=share "SHARE_DIR=$(winpath "$WORK/mnt")" SHARE_SCOPE=inv
assert_contains "an ordinary delivery SUCCEEDS, so the checks below are not stuck on failure" \
  "CLAIMED-OK" "$TP_OUT"

tp "if (Import-Tp) { if (Send-TpLog -LogPath '$(winpath "$WORK/nosuchlog.txt")') { 'CLAIMED-OK' } else { 'reported-failure' } }" \
  "${SHARE_OK[@]}"
assert_contains "delivering a log that does not exist REPORTS A FAILURE, rather than claiming success" \
  "reported-failure" "$TP_OUT"

# A destination that cannot be written. `ops-logs` as a FILE rather than a
# directory is the shape that catches a rename-into-a-directory bug.
mkdir -p "$WORK/mnt/blocked"
printf 'not a directory\n' > "$WORK/mnt/blocked/ops-logs"
tp "if (Import-Tp) { if (Send-TpLog -LogPath '$(winpath "$WORK/mnt/fake.txt")') { 'CLAIMED-OK' } else { 'reported-failure' } }" \
  TRANSPORT=share "SHARE_DIR=$(winpath "$WORK/mnt")" SHARE_SCOPE=blocked
assert_contains "a destination that cannot be written REPORTS A FAILURE" \
  "reported-failure" "$TP_OUT"

# --- the description NEVER carries a credential ------------------------------
# Both callers print it: the preflight table, and the startup banner, which goes
# into a log that gets committed and pushed. A token in a remote URL is the
# commonest way one ends up in a repository for ever.
GITREPO="$WORK/gitrepo"
mkdir -p "$GITREPO" && git init -q "$GITREPO"
( cd "$GITREPO" && git config user.email t@e.invalid && git config user.name t \
  && printf x > f && git add -A && git commit -qm init && git branch -M main ) >/dev/null 2>&1

# The two shapes that matter. In the SECOND, the secret is the USERNAME - it is
# a documented git form, and masking only the password prints the token in full.
for url in \
  'https://ci-user:SUPERSECRETTOKENVALUE@git.invalid/x.git' \
  'https://ghp_SECRETTOKENVALUE1234@github.invalid/x.git'; do
  ( cd "$GITREPO" && git remote remove origin >/dev/null 2>&1; git remote add origin "$url" )
  tp "if (Import-Tp) { Get-TpDescribe }" TRANSPORT=git "REPO_ROOT=$(winpath "$GITREPO")"
  case "$TP_OUT" in
    *SUPERSECRETTOKENVALUE* | *ghp_SECRETTOKENVALUE1234*)
      t_no "Get-TpDescribe LEAKED the credential from [$url]"
      printf '     it said: %s\n' "$TP_OUT" ;;
    *)
      t_ok "Get-TpDescribe masks the credential in [${url%%:*}...@...]" ;;
  esac
  assert_contains "and still says which host, or it has masked away the answer" \
    "invalid" "$TP_OUT"
done

# --- the credential never reaches argv ---------------------------------------
# `git -c http.extraHeader=...` puts the token in the process table, where every
# other user on the box can read it out of `ps`. GIT_CONFIG_* is how the bash
# side avoids that, and this asserts the PowerShell side did not take the
# shortcut.
# CODE ONLY, not the comment that explains why this is avoided. The first
# version grepped the whole file and matched its own explanation - the same
# shape as the no-truncation gate firing on a comment about truncation.
assert_eq "the git transport passes no credential on a git command line" "0" \
  "$(grep -v '^[[:space:]]*#' "$PSDIR/transports/git.psm1" \
     | grep -c -- '-c *http\.extraHeader')"
assert_contains "it uses GIT_CONFIG_* instead" \
  "GIT_CONFIG_KEY_" "$(cat "$PSDIR/transports/git.psm1")"
# APPENDED at the next free index rather than slot 0: an operator may already
# export their own GIT_CONFIG_*, and slot 0 would truncate theirs off the list.
assert_contains "and appends at the next free index rather than overwriting slot 0" \
  'GIT_CONFIG_COUNT' "$(cat "$PSDIR/transports/git.psm1")"

t_summary
