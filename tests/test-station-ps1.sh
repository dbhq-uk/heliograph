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

# --- the preflight ------------------------------------------------------------
pre -- --check
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

# --check CHANGES NOTHING. It is what gets run where nobody may alter anything
# yet, so the answer to "will this work here" can be had before asking.
rm -rf "$WORK/ops-logs"
pre -- --check
if [ -d "$WORK/ops-logs" ]; then
  t_no "--check created ops-logs, and it must change nothing at all"
else
  t_ok "--check created nothing, so it can run where nothing may be altered"
fi
assert_contains "and it says so" "nothing was changed" "$PRE_OUT"

# A REAL START MAY create it, which is the difference between the two.
pre -- ""
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
pre -- --check
assert_eq "a missing payload file is a blocking problem" "1" "$PRE_RC"
assert_contains "and it names the file" "probe.psm1" "$PRE_OUT"
assert_contains "and says to re-run the bootstrap" "bootstrap" "$PRE_OUT"
mv "$WORK/lib/probe.psm1.hidden" "$WORK/lib/probe.psm1"

pre -- --check
assert_eq "and it passes again once the file is back" "0" "$PRE_RC"

# --- the cancel takes the WHOLE TREE -----------------------------------------
# A grandchild is the case that matters: the step is a child of the capture and
# whatever the step runs is a child of that. Killing one level is the defect.
if [ "$IS_WINDOWS" = "1" ]; then
  cat > "$WORK/tree.ps1" <<'PS'
param([string] $PidFile)
$p = Start-Process -FilePath 'cmd.exe' -ArgumentList '/c','timeout /t 300 /nobreak' -PassThru
[System.IO.File]::WriteAllText($PidFile, "$($p.Id)")
Start-Sleep -Seconds 300
PS
else
  cat > "$WORK/tree.ps1" <<'PS'
param([string] $PidFile)
$p = Start-Process -FilePath '/bin/sleep' -ArgumentList '300' -PassThru
[System.IO.File]::WriteAllText($PidFile, "$($p.Id)")
Start-Sleep -Seconds 300
PS
fi

cat > "$WORK/runtree.ps1" <<'PS'
param([string] $PidFile, [string] $HandleFile)
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here 'lib/cancel.psm1') -Force
$strategy = Enter-CapKillGroup
[System.IO.File]::WriteAllText($HandleFile, "$PID")
Write-Output "strategy=$strategy"
& (Join-Path $here 'tree.ps1') -PidFile $PidFile
PS

cat > "$WORK/stop.ps1" <<'PS'
param([int] $ProcessId)
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here 'lib/cancel.psm1') -Force
if (Stop-CapTree -ProcessId $ProcessId) { exit 0 }
exit 1
PS

GC_FILE="$WORK/grandchild.pid"
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
while { [ ! -s "$GC_FILE" ] || [ ! -s "$H_FILE" ]; } && [ "$waited" -lt 300 ]; do
  waited=$((waited + 1)); sleep 0.1
done

if [ -s "$GC_FILE" ] && [ -s "$H_FILE" ]; then
  t_ok "a two-deep process tree started, and both pids were recorded"
  runner="$(tr -d ' \r\n' < "$H_FILE")"
  grandchild="$(tr -d ' \r\n' < "$GC_FILE")"

  alive() {  # alive <pid> - asked through the module, which is what the station uses
    ( cd "$WORK" && "$PS_BIN" -NoProfile -Command \
        "Import-Module ./lib/cancel.psm1 -Force; if (Test-CapAlive -ProcessId $1) { exit 0 } else { exit 1 }" \
      ) >/dev/null 2>&1
  }

  if alive "$grandchild"; then
    t_ok "the grandchild is running before the cancel, so there is something to prove"
  else
    t_no "the grandchild was not running, so the cancel below proves nothing"
  fi

  ( cd "$WORK" && "$PS_BIN" -NoProfile -File ./stop.ps1 -ProcessId "$runner" ) >/dev/null 2>&1
  stop_rc=$?
  assert_eq "the cancel reports the runner confirmed dead" "0" "$stop_rc"

  sleep 1
  if alive "$grandchild"; then
    t_no "THE GRANDCHILD SURVIVED. A cancel that stops the wrapper and leaves the"
    printf '     step running tells the operator the run was cancelled while it\n'
    printf '     carries on changing the estate.\n'
  else
    t_ok "and the GRANDCHILD is gone too, so the cancel took the whole tree"
  fi

  assert_contains "and the strategy it used is reported, not assumed" \
    "strategy=" "$(cat "$WORK/tree.out" 2>/dev/null)"
else
  t_no "the process tree never started, so the cancel was NOT exercised"
  cat "$WORK/tree.out" 2>/dev/null | sed 's/^/     /' | head -5
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
