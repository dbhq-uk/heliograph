#!/usr/bin/env bash
# =============================================================================
#  test-station-ps1.sh - the preflight, and the cancel that has to take a tree
# =============================================================================
# Two things the conformance suite cannot ask about.
#
# THE PREFLIGHT is the first thing anybody runs on a Windows control node, and
# its whole value is in what it says when the answer is no. A preflight that
# refuses without naming the remedy has cost a round trip through somebody who
# cannot debug the machine - so the remedies are asserted, not just the verdict.
#
# THE CANCEL has to reach the STEP, not the wrapper. A step runs terraform,
# which runs a provider, which runs git; killing only what the station started
# leaves all of it running while the operator is told the run was cancelled.
# So a real tree is built here and a grandchild is checked for afterwards.
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
  t_skip "no PowerShell interpreter: the preflight and the cancel were NOT exercised."
  t_summary
  exit 0
fi
t_ok "a PowerShell interpreter is present ($PS_BIN), so the assertions below ran"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cp -r "$PSDIR/." "$WORK/"

IS_WINDOWS=0
case "$(uname -s 2>/dev/null)" in MINGW* | MSYS* | CYGWIN*) IS_WINDOWS=1 ;; esac
winpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s' "$1"; fi
}

pre() {  # pre <env...> -- <args...>   -> PRE_OUT, PRE_RC
  local envs=() a
  while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do envs+=("$1"); shift; done
  shift || true
  a=("$@")
  PRE_OUT="$( cd "$WORK" && env -u ALLOW_ROOT -u HELIOGRAPH_ASSUME_PRIVILEGED \
                "${envs[@]+"${envs[@]}"}" \
                "$PS_BIN" -NoProfile -File ./start.ps1 "${a[@]}" 2>&1 )"
  PRE_RC=$?
}

# --- IS THIS ACCOUNT ALREADY PRIVILEGED? -------------------------------------
# Measured, not assumed, for the same reason test-run-ps1.sh measures it:
# GitHub's Windows runner is an Administrator, so the preflight correctly
# refuses and every assertion about some OTHER line fails for a reason it is
# not testing. The `user` line has its own assertions further down, which is
# where that gate belongs.
BASE_ENV=()
pre -- --check
if printf '%s' "$PRE_OUT" | grep -q 'is an Administrator or SYSTEM'; then
  BASE_ENV=(ALLOW_ROOT=1)
  t_ok "this account is privileged, so the preflight refuses it - ALLOW_ROOT=1 for the rest"
else
  t_ok "this account is not privileged, so the preflight has nothing to refuse"
fi

# --- the preflight ------------------------------------------------------------
pre "${BASE_ENV[@]+"${BASE_ENV[@]}"}" -- --check
assert_eq "the preflight passes on this machine" "0" "$PRE_RC"

# THE TWO THAT DECIDE THIS ON A REAL ESTATE. Neither is guessable from a version
# number, and each is a whole afternoon if it turns up as a syntax error in
# somebody else's file instead.
assert_contains "it reports the language mode, which is what stops a station dead" \
  "language" "$PRE_OUT"
assert_contains "it reports the execution policy PER SCOPE, because a GPO overrides Bypass" \
  "exec policy" "$PRE_OUT"
assert_contains "it says which cancel mechanism this machine actually gets" \
  "cancel" "$PRE_OUT"
assert_contains "it prints a UTC clock to compare against" "clock" "$PRE_OUT"
assert_contains "and it says the transport is missing rather than staying silent" \
  "no transport yet" "$PRE_OUT"

# --check CHANGES NOTHING, AND THE WHOLE TREE IS COMPARED.
#
# The first version of this deleted ops-logs first and then checked it had not
# come back - which skipped the entire branch that runs when the directory
# EXISTS, and that branch wrote a probe file. It proved that --check does not
# create one directory, and nothing else. The implementation as written then
# failed this stronger test, which is the point of writing it this way.
#
# A SENTINEL WITH THE PROBE'S OWN NAME, because the probe had a fixed one: had
# that file already existed, --check would have destroyed it.
snapshot() {
  ( cd "$WORK" && find . -type f -printf '%p %s\n' 2>/dev/null | LC_ALL=C sort
    cd "$WORK" && find . -type f -exec sha256sum {} + 2>/dev/null | LC_ALL=C sort )
}
mkdir -p "$WORK/ops-logs"
printf 'do not touch me\n' > "$WORK/ops-logs/.heliograph-write-check"
before_snap="$(snapshot)"
pre "${BASE_ENV[@]+"${BASE_ENV[@]}"}" -- --check
after_snap="$(snapshot)"
if [ "$before_snap" = "$after_snap" ]; then
  t_ok "--check changed NOTHING in the whole payload, not merely created no directory"
else
  t_no "--check modified the tree:"
  diff <(printf '%s\n' "$before_snap") <(printf '%s\n' "$after_snap") | sed 's/^/     /' | head -10
fi
assert_eq "and a file with the probe's own name is intact" \
  "do not touch me" "$(cat "$WORK/ops-logs/.heliograph-write-check" 2>/dev/null)"
assert_contains "and it says so" "nothing was changed" "$PRE_OUT"

# A REAL START PROVES WRITABILITY, and must not clobber that file either.
pre "${BASE_ENV[@]+"${BASE_ENV[@]}"}" -- ""
assert_contains "a real start proves the directory accepts a write" "writable" "$PRE_OUT"
assert_eq "and its probe did not reuse a name that was already taken" \
  "do not touch me" "$(cat "$WORK/ops-logs/.heliograph-write-check" 2>/dev/null)"
left="$(find "$WORK/ops-logs" -name '.heliograph-write-check.*' 2>/dev/null | wc -l | tr -d ' ')"
assert_eq "and left no probe behind" "0" "$left"
rm -rf "$WORK/ops-logs"

# A REAL START MAY create it, which is the other half of the difference.
pre "${BASE_ENV[@]+"${BASE_ENV[@]}"}" -- ""
if [ -d "$WORK/ops-logs" ]; then
  t_ok "a real start creates ops-logs, so a capture has somewhere to go"
else
  t_no "a real start did not create ops-logs"
fi

# --- the preflight REFUSES, and says what to do -------------------------------
pre HELIOGRAPH_ASSUME_PRIVILEGED=1 -- --check
assert_eq "a privileged account is a blocking problem" "1" "$PRE_RC"
assert_contains "and the refusal names the remedy, not just the verdict" \
  "ALLOW_ROOT=1" "$PRE_OUT"
assert_contains "and says why the account matters" "blast radius" "$PRE_OUT"

pre HELIOGRAPH_ASSUME_PRIVILEGED=1 ALLOW_ROOT=1 -- --check
assert_eq "ALLOW_ROOT=1 turns it into a warning, not a refusal" "0" "$PRE_RC"
assert_contains "and it is still SAID, because it is worth knowing" \
  "whole machine in reach" "$PRE_OUT"

# A MISSING PAYLOAD FILE. The loop would fail on its first request and the
# reason would be somewhere else entirely.
mv "$WORK/lib/probe.psm1" "$WORK/lib/probe.psm1.hidden"
pre "${BASE_ENV[@]+"${BASE_ENV[@]}"}" -- --check
assert_eq "a missing payload file is a blocking problem" "1" "$PRE_RC"
assert_contains "and it names the file" "probe.psm1" "$PRE_OUT"
assert_contains "and says to re-run the bootstrap" "bootstrap" "$PRE_OUT"
mv "$WORK/lib/probe.psm1.hidden" "$WORK/lib/probe.psm1"

pre "${BASE_ENV[@]+"${BASE_ENV[@]}"}" -- --check
assert_eq "and it passes again once the file is back" "0" "$PRE_RC"

# --- the cancel takes the WHOLE TREE -----------------------------------------
# THREE LEVELS, and each records its OWN pid.
#
# The first version built runner -> cmd -> timeout on Windows and asserted on
# cmd's pid, which is the INTERMEDIATE process. cmd could die while timeout
# carried on and the test would still pass - so it checked the one level that
# was never in doubt. A step is a child of the capture and whatever the step
# runs is a child of that, so the deepest is exactly the one that matters:
# terraform surviving a cancel is the defect, not the shell that launched it.
# THE SLEEPER IS POWERSHELL ON BOTH PLATFORMS, which removes a branch and a
# defect with it. The Windows version used `cmd /c timeout /t 300`, and
# `timeout` REFUSES A REDIRECTED STDIN - "ERROR: Input redirection is not
# supported" - so it exited immediately and the deepest process was dead before
# the cancel had anything to prove. The assertion caught that, which is the
# whole reason it checks the tree is up before killing it.

cat > "$WORK/child.ps1" <<'PS'
param([string] $PidFile)
# ITS OWN pid, and its child's. Written by the process they belong to, so
# neither is inferred - the first version recorded an intermediate process and
# asserted on that, which is the one level that was never in doubt.
$shell = [System.Diagnostics.Process]::GetCurrentProcess().MainModule.FileName
$sleeper = Start-Process -FilePath $shell `
    -ArgumentList '-NoProfile', '-Command', 'Start-Sleep -Seconds 300' -PassThru
[System.IO.File]::WriteAllText($PidFile, "$PID`n$($sleeper.Id)")
Start-Sleep -Seconds 300
PS

cat > "$WORK/runtree.ps1" <<'PS'
param([string] $PidFile, [string] $HandleFile)
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here 'lib/cancel.psm1') -Force
$strategy = Enter-CapKillGroup
[System.IO.File]::WriteAllText($HandleFile, "$PID")
Write-Output "strategy=$strategy"
$shell = [System.Diagnostics.Process]::GetCurrentProcess().MainModule.FileName
& $shell -NoProfile -File (Join-Path $here 'child.ps1') -PidFile $PidFile
PS

cat > "$WORK/stop.ps1" <<'PS'
param([int] $ProcessId)
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here 'lib/cancel.psm1') -Force
if (Stop-CapTree -ProcessId $ProcessId) { exit 0 }
exit 1
PS

GC_FILE="$WORK/pids"
H_FILE="$WORK/tree.handle"
rm -f "$GC_FILE" "$H_FILE"

if [ "$IS_WINDOWS" = "1" ]; then
  ( cd "$WORK" && "$PS_BIN" -NoProfile -File ./runtree.ps1 \
      -PidFile "$(winpath "$GC_FILE")" -HandleFile "$(winpath "$H_FILE")" ) >"$WORK/tree.out" 2>&1 &
else
  ( cd "$WORK" && setsid "$PS_BIN" -NoProfile -File ./runtree.ps1 \
      -PidFile "$GC_FILE" -HandleFile "$H_FILE" ) >"$WORK/tree.out" 2>&1 &
fi

waited=0
while { [ ! -s "$GC_FILE" ] || [ ! -s "$H_FILE" ]; } && [ "$waited" -lt 600 ]; do
  waited=$((waited + 1)); sleep 0.1
done

if [ -s "$GC_FILE" ] && [ -s "$H_FILE" ]; then
  runner="$(tr -d ' \r' < "$H_FILE" | head -1)"
  child="$(tr -d ' \r' < "$GC_FILE" | sed -n 1p)"
  sleeper="$(tr -d ' \r' < "$GC_FILE" | sed -n 2p)"
  t_ok "a THREE-level tree started: runner $runner, child $child, sleeper $sleeper"

  alive() {  # alive <pid> - asked through the module, which is what the station uses
    ( cd "$WORK" && "$PS_BIN" -NoProfile -Command \
        "Import-Module ./lib/cancel.psm1 -Force; if (Test-CapAlive -ProcessId $1) { exit 0 } else { exit 1 }" \
      ) >/dev/null 2>&1
  }

  # WHICH MECHANISM, by name. `strategy=` alone matched an empty value, so
  # deleting the Job Object setup left the test green - which is exactly how a
  # dead LimitFlags assignment survived: taskkill was doing all the work and
  # nothing asked whether the job existed.
  strat="$(sed -n 's/^strategy=//p' "$WORK/tree.out" 2>/dev/null | head -1 | tr -d ' \r')"
  if [ "$IS_WINDOWS" = "1" ]; then
    case "$strat" in
      job-object) t_ok "and it established a Job Object, which is the Windows mechanism" ;;
      taskkill)   t_ok "and it fell back to taskkill, having said why: $(sed -n 's/.*Reason: //p' "$WORK/tree.out" | head -1)" ;;
      *)          t_no "the strategy on Windows was [$strat], which is neither job-object nor taskkill" ;;
    esac
  else
    assert_eq "and it established the process-group mechanism" "process-group" "$strat"
  fi

  if alive "$child" && alive "$sleeper"; then
    t_ok "both descendants are running before the cancel, so there is something to prove"
  else
    t_no "the tree was not fully up (child=$(alive "$child" && echo yes || echo no), sleeper=$(alive "$sleeper" && echo yes || echo no)), so the cancel below proves nothing"
  fi

  ( cd "$WORK" && "$PS_BIN" -NoProfile -File ./stop.ps1 -ProcessId "$runner" ) >/dev/null 2>&1
  stop_rc=$?
  assert_eq "the cancel reports the runner confirmed dead" "0" "$stop_rc"

  sleep 2
  if alive "$sleeper"; then
    t_no "THE DEEPEST DESCENDANT SURVIVED. A cancel that stops the wrapper and"
    printf '     leaves the step running tells the operator the run was cancelled\n'
    printf '     while it carries on changing the estate.\n'
  else
    t_ok "and the DEEPEST descendant is gone, two levels down from what was killed"
  fi
  if alive "$child"; then
    t_no "the intermediate process survived the cancel"
  else
    t_ok "and so is the one in between"
  fi
else
  t_no "the process tree never started, so the cancel was NOT exercised"
  sed 's/^/     /' "$WORK/tree.out" 2>/dev/null | head -5
fi

# NO ZOMBIE TEST HERE, and that is a deliberate omission rather than an
# oversight. Test-CapAlive checks `HasExited` as well as existence, because on
# Unix a killed child stays in the process table until its parent reaps it -
# but PowerShell's Start-Process reaps its own children, so this suite cannot
# construct the case. The assertion that was here passed with the check
# REMOVED, which makes it worse than nothing: it would have reported a guard as
# proven while proving something else. The guard stays in the module, marked
# as untested.

t_summary
