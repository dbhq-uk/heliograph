#!/usr/bin/env bash
# =============================================================================
#  start.sh - one command that gets the loop running on this machine
# =============================================================================
#     ./start.sh                     # check this machine, then run the station
#     ./start.sh --check             # check only, change nothing, exit
#     ./start.sh --branch task/foo   # check that branch out first (git only)
#     ./start.sh -- --once           # everything after -- goes to station.sh
#
#  TRANSPORT selects the channel, exactly as it does for station.sh, and git is
#  still the default.
#
#  Three jobs, and nothing else:
#
#    1. Prove this machine can produce a usable capture AT ALL. The toolkit
#       depends on `sed -u` and `base64 -w0`, and neither spelling is
#       universal: `sed -u` is absent from busybox, `base64 -w0` is absent
#       from BSD/macOS base64. A busybox sed does not fail loudly: it produces
#       a log where every line carries the same timestamp, which is worse than
#       no timestamp because it looks like one.
#    2. ASK THE TRANSPORT whether it can carry a log from here, before an
#       hour-long step discovers that it cannot.
#    3. Hand over to station.sh.
#
#  JOB 2 USED TO BE "prove git can push". Read access is not write access and a
#  token that works against a host's REST API says nothing about the git path,
#  so that check was worth having - and it is unchanged, in transports/git.sh
#  where it belongs. What was wrong was running it unconditionally: one FAIL
#  here stops before station.sh runs, so a relay or blob station could not be
#  started by the one command every host and every page tells the operator to
#  type. The transport is asked through tp_check, and through tp_preflight
#  where it has more to say.
#
#  IT DOES NOT CLONE. This file ships inside the transport repo, so by the time
#  it runs the clone has already happened. Whoever cloned owns that step.
#
#  IT INSTALLS NOTHING, and that separation is the point: this has to run on a
#  node where installing is forbidden.
#
#  `--check` CHANGES NOTHING. It is what an operator runs to answer "will this
#  work here", often before they are permitted to alter anything.
# =============================================================================
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_ROOT" || exit 1
# shellcheck source=caplib.sh disable=SC1091
. "$REPO_ROOT/caplib.sh"

CHECK_ONLY=0
WANT_BRANCH=""
AGENT_ARGS=()

while [ $# -gt 0 ]; do
  case "$1" in
    --check)   CHECK_ONLY=1 ;;
    --branch)
      WANT_BRANCH="${2:-}"
      if [ -z "$WANT_BRANCH" ]; then
        echo "--branch requires a branch name" >&2
        exit 2
      fi
      shift ;;
    --)
      shift
      AGENT_ARGS=("$@")
      break ;;
    -h|--help) sed -n '2,41p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
  shift
done

FAILED=0
# report <ok|warn|FAIL> <label> <detail...>
# A warn is a fact worth knowing that does not stop a run. A FAIL always names
# what to do about it: this output is often the only thing the person on the far
# side has to work from.
report() {
  local status="$1" label="$2"; shift 2
  [ "$status" = "FAIL" ] && FAILED=$((FAILED + 1))
  printf '%-4s  %-12s  %s\n' "$status" "$label" "$*"
}

preflight() {
  if [ "${BASH_VERSINFO[0]:-0}" -ge 4 ]; then
    report ok bash "$BASH_VERSION"
  else
    report FAIL bash "need 4 or newer, found ${BASH_VERSION:-unknown}. Install bash 4 or newer and put it first on PATH"
  fi

  # git is NOT checked here any more. It is the default transport and not a
  # requirement of the machine: a relay or blob station needs curl, and telling
  # its operator to install git would be a lie in the one table they read. The
  # git transport asks for it in its own tp_init, which is where a requirement
  # belonging to one channel belongs.

  # Line endings. A transport repo cloned on Windows arrives with CRLF, because
  # Git for Windows sets core.autocrlf=true at install time.
  #
  # WHETHER THAT MATTERS DEPENDS ENTIRELY ON WHICH BASH THIS IS, so probe it
  # rather than assume. Measured both ways: on Linux, bash treats the CR as part
  # of the token, and the sourced case is the nasty one - caplib.sh reports
  # "set: pipefail: invalid option name" and CARRIES ON with neither -u nor
  # pipefail applied. Git for Windows' bash strips CR transparently and behaves
  # identically to LF, pipefail and set -u included. Reporting a blanket failure
  # would therefore refuse to start on a Windows control node that works
  # perfectly, which is worse than not checking at all.
  local probe tolerant=no crlf
  probe="$(mktemp 2>/dev/null || printf '/tmp/hg-crlf-probe.%s' "$$")"
  printf 'hg_crlf_probe=1\r\n' > "$probe"
  # shellcheck disable=SC1090
  if ( . "$probe" 2>/dev/null; [ "${hg_crlf_probe:-}" = "1" ] ); then tolerant=yes; fi
  rm -f "$probe"

  crlf="$(grep -lr $'\r' --include='*.sh' --exclude-dir=.git . 2>/dev/null |
            sed 's|^\./||' | sort | tr '\n' ' ')"

  if [ "$tolerant" = yes ]; then
    if [ -n "$crlf" ] && [ ! -e .gitattributes ]; then
      report warn "line endings" "this bash strips CR so the loop runs fine here, but the checkout holds CRLF and there is no .gitattributes pinning it. A Linux control node or the container cloning this repo would fail on those files. Re-run bootstrap.sh to install .gitattributes"
    else
      report ok "line endings" "this bash tolerates CR, so a CRLF checkout runs here"
    fi
  elif [ -z "$crlf" ]; then
    report ok "line endings" "LF throughout, so every script can be sourced and run"
  else
    report FAIL "line endings" "CRLF in: ${crlf% }. This bash does not tolerate CR: an executed script dies with 'bash\\r: No such file or directory' and a sourced one loses 'set -uo pipefail' WITHOUT stopping. Fix with 'git config --global core.autocrlf false' and clone again - the checkout is disposable and anything already pushed is safe"
  fi

  # THE load-bearing check, and the reason this script exists.
  if [ "$(printf 'x\n' | sed -u 's/x/y/' 2>/dev/null)" = "y" ]; then
    report ok "sed -u" "the capture streams, and a cancelled run keeps its partial log"
  else
    # This used to be a FAIL, and it was right at the time: the timestamp was
    # applied AFTER sed, so a buffered sed gave every line in a block the same
    # time and a hang became invisible while the log still read perfectly.
    #
    # cap_run now stamps each line before any sed runs, so the timestamps are
    # honest whatever sed does. What is left is smaller and worth saying
    # plainly rather than refusing over: redaction is still a sed stage and
    # cannot be skipped, so a run killed mid-flight loses whatever sed was
    # holding.
    report warn "sed -u" "this sed has no -u (busybox does not). Timestamps are unaffected - they are applied before sed - but a CANCELLED run will lose its partial log, and terminal output arrives in blocks. Install GNU sed and put it first on PATH to get both back"
  fi

  # base64 -w0 is how caplib builds the HTTPS auth header. BSD base64 wraps.
  if printf 'x' | base64 | tr -d '\n' >/dev/null 2>&1; then
    report ok "base64" "an HTTPS auth header can be built"
  else
    # caplib no longer asks for -w0: `base64 | tr -d '\n'` is the same thing
    # and is universal, so this only fails where there is no base64 at all.
    report FAIL "base64" "no usable base64 on PATH, so cap_git cannot build an auth header for an HTTPS remote"
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    report ok sha256sum "station.sh can detect its own updates"
  else
    report warn sha256sum "absent, so station.sh cannot detect its own updates: a pushed fix to station.sh will not take effect until someone restarts it by hand"
  fi

  if date -u +%Y-%m-%dT%H:%M:%SZ >/dev/null 2>&1; then
    report ok "date -u" "$(date -u +%Y-%m-%dT%H:%M:%SZ)  <- compare this with a clock you trust"
  else
    report FAIL "date -u" "cannot produce a UTC stamp, and every line of every log needs one. Install GNU coreutils and put date first on PATH"
  fi

  if command -v setsid >/dev/null 2>&1; then
    report ok setsid "cancel can signal the step's whole process group"
  else
    report warn setsid "absent, so station.sh falls back to 'set -m' job control"
  fi

  # Asked here as well as in the runners, because this file exists to answer
  # "will this work on this machine" before anyone commits to it. Finding out at
  # handover that the loop refuses to start is the same wasted trip the preflight
  # is for.
  if [ "$(id -u 2>/dev/null || echo 1000)" != "0" ]; then
    report ok user "$(whoami 2>/dev/null || echo unknown) - not root, so the blast radius is this account"
  elif [ "${ALLOW_ROOT:-0}" = "1" ]; then
    report warn user "root, permitted by ALLOW_ROOT=1. Every step will run with the whole machine in reach"
  else
    report FAIL user "root, and the runners refuse that: this toolkit has no credentials of its own, so the account it runs as is the whole blast radius. Run as an unprivileged user, or set ALLOW_ROOT=1 if this image has no other"
  fi

  # The branch checks used to be here. They are git's, they are unchanged, and
  # they now run from transports/git.sh's tp_preflight - which is also the only
  # place that can honour --branch against the remote, because on any other
  # transport there is no such thing as a branch.

  if [ -d ops-logs ] && [ -w ops-logs ]; then
    report ok ops-logs "writable"
  else
    report FAIL ops-logs "missing or not writable, so a capture would have nowhere to go. Run 'mkdir -p ops-logs', or fix its permissions"
  fi
}

# =============================================================================
#  The transport - ASKED, not assumed
# =============================================================================
# This file used to hold three git commands: `git remote get-url`, an
# `ls-remote` and a `push --dry-run`. They were good checks and they are still
# run, unchanged, from transports/git.sh. What was wrong was running them for
# every station, because one FAIL here stops before station.sh ever starts - so
# `./start.sh` could not start a relay or blob station at all, and the answer
# on /hosts was "by hand", meaning set eight variables and skip the only
# preflight there is on a machine nobody can log into.
#
# THE CONTRACT THIS USES, in order:
#
#   cap_transport_file   the name is a filename, so caplib validates it. One
#                        copy of that check, shared with the delivery path
#   tp_init              what this transport needs locally, with a remedy
#   tp_describe          the channel and the credential, by mechanism
#   tp_preflight         OPTIONAL. Checks only this transport knows to make
#   tp_check             the fallback: can it be reached at all
#
# tp_init IS CAPTURED RATHER THAN LET LOOSE. It reports to stderr, and its text
# is the remedy - "detached HEAD", "RELAY_URL is not set". Printed raw it lands
# above the table out of order; folded into a FAIL line it reads as one of the
# checks, which is what it is.
#
# CAPTURED THROUGH A FILE, NOT `$(tp_init 2>&1)`. A command substitution is a
# SUBSHELL, so git's `BRANCH=$b` - and every other variable a tp_init resolves
# for the rest of the run - is set there and lost on return. Measured, not
# reasoned about: the first version did exactly that and every git check below
# died on "BRANCH: unbound variable" while still reporting the transport as ok.
transport() {
  local f rc why errf
  CAP_TRANSPORT="${TRANSPORT:-git}"

  f="$(cap_transport_file "$CAP_TRANSPORT" 2>/dev/null)"; rc=$?
  if [ "$rc" = "2" ]; then
    report FAIL transport "'$CAP_TRANSPORT' is not a usable transport name. It becomes a filename that gets sourced, so only lowercase letters, digits and hyphens are accepted. Set TRANSPORT to one of: $(transport_names)"
    return 0
  fi
  if [ "$rc" != "0" ]; then
    report FAIL transport "no transport named '$CAP_TRANSPORT' in $REPO_ROOT/transports/. This payload ships: $(transport_names). Set TRANSPORT to one of those, or re-run bootstrap.sh if the directory is missing entirely"
    return 0
  fi
  # shellcheck disable=SC1090
  . "$f"

  errf="$(mktemp 2>/dev/null || printf '/tmp/hg-tp-init.%s' "$$")"
  tp_init 2>"$errf"; rc=$?
  # Its own words, with the "station: " prefix it writes for the loop's output
  # stripped and the lines joined, because this goes in a one-line table cell.
  why="$(sed -e 's/^ *station: */ /' -e 's/^ *//' "$errf" 2>/dev/null | tr '\n' ' ')"
  why="${why%"${why##*[![:space:]]}"}"
  rm -f "$errf"
  if [ "$rc" != "0" ]; then
    report FAIL transport "the '$CAP_TRANSPORT' transport will not initialise here.${why:+ $why}"
    return 0
  fi
  # A tp_init that succeeded but still had something to say has said something
  # worth reading. Swallowing it is how a warning about an unverified binary
  # disappears on the machine that most needs it.
  [ -n "$why" ] && report warn transport "$why"

  report ok transport "$CAP_TRANSPORT - $(tp_describe)"

  # A transport with more to say says it. Everything else gets the one question
  # every transport can answer.
  if declare -F tp_preflight >/dev/null 2>&1; then
    TP_WANT_SCOPE="$WANT_BRANCH" tp_preflight
    return 0
  fi
  if tp_check; then
    report ok "$CAP_TRANSPORT reach" "the channel answered and the credential was accepted"
  else
    # tp_check writes its own diagnosis to stderr as it goes, which is why this
    # line does not try to guess one. It names the variables instead, because on
    # every non-git transport a misconfiguration is a variable.
    report FAIL "$CAP_TRANSPORT reach" "the '$CAP_TRANSPORT' transport could not be reached, or refused this credential - see the line(s) above. The station would capture logs it could not deliver. Check the transport's variables: 'grep cap_need transports/$CAP_TRANSPORT.sh' lists every one it requires"
  fi
}

# What this payload actually ships, for a message that would otherwise say
# "pick a valid one" and leave the operator guessing which those are.
transport_names() {
  local n out=""
  for n in "$REPO_ROOT"/transports/*.sh; do
    [ -f "$n" ] || continue
    n="$(basename "$n" .sh)"
    out="${out:+$out, }$n"
  done
  printf '%s' "${out:-none - transports/ is missing}"
}


echo "heliograph preflight on $(hostname -f 2>/dev/null || hostname)"
echo
preflight
transport

# --branch is git's, and only git's. Every other transport is pointed at its
# scope by a variable - PIGEONHOLE_LANE, RELAY_STATION - which is set before
# this script runs and which this script has no business rewriting.
#
# REFUSED RATHER THAN IGNORED. Silently accepting it would let somebody believe
# they had pointed a relay station at a different scope, and being wrong about
# which machine a station answers for is the failure the whole design exists to
# prevent. Checked here, after the transport is known, so the message can name
# the right variable rather than a general rule.
if [ -n "$WANT_BRANCH" ] && [ "${CAP_TRANSPORT:-git}" != "git" ]; then
  report FAIL --branch "--branch is the git transport's, and TRANSPORT is '$CAP_TRANSPORT'. A branch is not what this channel is bound to. Point it with the transport's own variable instead: 'grep cap_need transports/$CAP_TRANSPORT.sh' lists them, and tp_scope in that file names the one that decides the scope"
fi
echo

if [ "$FAILED" -gt 0 ]; then
  echo "preflight: $FAILED blocking problem(s) above. Not starting the station."
  exit 1
fi

if [ "$CHECK_ONLY" = "1" ]; then
  echo "preflight: clear. --check was given, so stopping here without changing anything."
  exit 0
fi

# --- get on the right branch, up to date -------------------------------------
# Deliberately after the checks and after --check has already exited: this is
# the first thing here that touches the working tree.
#
# It never resolves a conflict, never forces and never discards the operator's
# work, for the same reason station.sh does not: their local state may be the
# evidence, and destroying it to make a poll succeed is never the right trade.
if [ -n "$WANT_BRANCH" ]; then
  cap_git fetch --quiet origin >/dev/null 2>&1
  if git checkout --quiet "$WANT_BRANCH" 2>/dev/null; then
    # VERIFIED, not assumed. `git checkout` succeeds in shapes that do not
    # leave you on the branch you named - a detached checkout of a tag or a
    # commit that shares the name being the obvious one - and the branch is now
    # which MACHINE this station answers for. Landing on the wrong one quietly
    # is the failure this whole design exists to prevent.
    got="$(git rev-parse --abbrev-ref HEAD 2>/dev/null)"
    if [ "$got" != "$WANT_BRANCH" ]; then
      report FAIL checkout "asked for '$WANT_BRANCH' and landed on '$got'. Not starting: the branch decides which requests this station answers"
      echo
      echo "preflight: 1 blocking problem above. Not starting the station."
      exit 1
    fi
    report ok checkout "$WANT_BRANCH"
  else
    report FAIL checkout "cannot check out '$WANT_BRANCH'. It may not exist here yet, or the working tree may be dirty"
    echo
    echo "preflight: 1 blocking problem above. Not starting the station."
    exit 1
  fi
fi

# Bring the payload up to date, where the transport can do that at all.
#
# OPTIONAL, and asked rather than assumed: this was `cap_git pull --rebase`
# unconditionally, which on a relay station is a git command in a directory that
# need not be a repository. A transport with nothing to sync simply does not
# define tp_sync, and the loop's first poll is where it gets current anyway.
if declare -F tp_sync >/dev/null 2>&1; then
  tp_sync
fi

echo
echo "preflight: clear. Handing over to station.sh."
echo
# exec, not a child: the operator's Ctrl-C has to reach the station so its
# cleanup trap runs and a mid-run step gets signalled rather than orphaned.
exec ./station.sh ${AGENT_ARGS[@]+"${AGENT_ARGS[@]}"}
