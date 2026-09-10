#!/usr/bin/env bash
# =============================================================================
#  drivers/powershell.sh - caplib.psm1, under the conformance contract
# =============================================================================
# The second implementation. AGENTS.md permits one ONLY while it passes this
# suite, which is the whole reason the suite exists in its current shape.
#
# WHAT THIS DRIVER CAN AND CANNOT ANSWER TODAY, and it says so by skipping
# rather than by passing:
#
#   1-4, 7, 8, 10   caplib.psm1 exists. These are its properties.
#   5, 6            run.ps1 carries gates 1, 2 and 4, the same three run.sh
#                   carries, with the same exit codes
#   9               delivery lives in the transports, which do not exist yet
#
# A driver that claimed `gates` and returned 0 would report the root gate as
# proven on a station that has no gate at all, which is the most expensive
# possible thing to be wrong about here.
#
# It is a BASH driver for a PowerShell implementation, and that is not a
# contradiction: the suite is the specification and it is written once. What
# the driver does is shell out. The alternative - a PowerShell copy of the
# suite - is precisely the second copy of the specification that the driver
# split exists to prevent.
# =============================================================================

_P_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_P_CAPTURE="$_P_HERE/powershell-capture.ps1"
_P_STOP="$_P_HERE/powershell-stop.ps1"

# Windows is asked about once. `uname` under Git-Bash answers MINGW64_NT-*, and
# MSYS_NT-* under an MSYS shell.
_P_WINDOWS=0
case "$(uname -s 2>/dev/null)" in MINGW* | MSYS* | CYGWIN*) _P_WINDOWS=1 ;; esac

# CONF_PS_SHELL OVERRIDES, and that is not a convenience.
#
# Windows PowerShell 5.1 is the floor this implementation targets, and the
# driver preferring `pwsh` meant the floor was never once exercised - on a
# Windows host with both editions installed it tested 7 and ignored 5.1
# entirely. That is exactly how the first version shipped using
# ProcessStartInfo.ArgumentList, which does not exist on .NET Framework and
# would have thrown on every 5.1 station.
#
# So Windows CI runs this suite twice, once per edition, by setting
# CONF_PS_SHELL. The default order is only what to do when nobody said.
_P_SHELL="${CONF_PS_SHELL:-}"
if [ -n "$_P_SHELL" ]; then
  command -v "$_P_SHELL" >/dev/null 2>&1 || _P_SHELL=""
else
  for _c in pwsh powershell powershell.exe; do
    if command -v "$_c" >/dev/null 2>&1; then _P_SHELL="$_c"; break; fi
  done
fi

drv_name() {
  if [ -n "$_P_SHELL" ]; then
    printf 'caplib.psm1 (%s)' "$("$_P_SHELL" -NoProfile -Command '$PSVersionTable.PSVersion.ToString()' 2>/dev/null | tr -d '\r')"
  else
    printf 'caplib.psm1 (no PowerShell here)'
  fi
}

drv_supports() {
  [ -n "$_P_SHELL" ] || return 1
  case "$1" in
    capture) return 0 ;;
    # A cancel loses nothing here - caplib.psm1 redacts in-process, line by
    # line, inside the read loop, so there is no buffer to lose the way busybox
    # `sed` does. What it needs is a way to SIGNAL the whole tree.
    #
    # On Unix that is `setsid` and a negative pid; on Windows it is a Job
    # Object with KILL_ON_JOB_CLOSE, falling back to `taskkill /T /F` where
    # Add-Type is blocked. Both live in station/powershell/lib/cancel.psm1, so
    # this answers yes on either platform - and `setsid` is only asked about
    # where it is the mechanism.
    cancel)
      if [ "$_P_WINDOWS" = "1" ]; then return 0; fi
      command -v setsid >/dev/null 2>&1
      ;;
    gates) return 0 ;;
    # NOT YET, and said out loud. The transports are PR 12.
    deliver) return 1 ;;
    *) return 1 ;;
  esac
}

# A PATH POWERSHELL WILL UNDERSTAND.
#
# Git-Bash converts Unix-looking paths to Windows ones AT THE EXEC BOUNDARY, so
# an argument like `-LogPath /tmp/x` arrives native and everything works. A path
# EMBEDDED IN A SCRIPT gets no such conversion: PowerShell reads `/tmp/x` as
# `C:\tmp\x`, writes the file there, and the suite looks in Git-Bash's /tmp and
# finds nothing.
#
# That is exactly how p5 failed on Windows and nowhere else - the step exited 0
# and its marker was written to another directory, so "the gate is passing
# without executing" was reported about a step that had executed perfectly.
_p_winpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s' "$1"; fi
}

drv_step_name() { printf 'steps/%s.ps1' "$1"; }

# --- the fixtures, in PowerShell ---------------------------------------------
# Deliberately written in the plainest PowerShell that does the job. A fixture
# that used a clever construct would be testing that construct.
#
# `Start-Sleep -Milliseconds` rather than -Seconds: the three-slow step needs
# 1.1s, and -Seconds takes an int.
drv_step_file() {  # drv_step_file <kind> <path-without-extension> [marker]
  local kind="$1" path="$2.ps1" marker="${3:-}"
  case "$kind" in
    three-slow)
      cat > "$path" <<'PS'
Write-Output 'first'
Start-Sleep -Milliseconds 1100
Write-Output 'second'
Start-Sleep -Milliseconds 1100
Write-Output 'third'
PS
      ;;
    gap)
      cat > "$path" <<'PS'
Write-Output 'before'
Start-Sleep -Seconds 3
Write-Output 'after'
PS
      ;;
    rc42)
      cat > "$path" <<'PS'
Write-Output 'working'
exit 42
PS
      ;;
    rc0)
      cat > "$path" <<'PS'
Write-Output 'working'
PS
      ;;
    slow)
      cat > "$path" <<'PS'
Write-Output 'starting the long probe'
foreach ($i in 1..10) {
    Write-Output "probe $i"
    Start-Sleep -Seconds 1
}
Write-Output 'finished'
PS
      ;;
    undeclared)
      cat > "$path" <<'PS'
Write-Output 'this step declares nothing'
PS
      ;;
    declared)
      {
        printf '# heliograph-mode: read-only\n'
        printf "Write-Output 'this step declares itself and measures nothing'\n"
        [ -n "$marker" ] && printf "New-Item -ItemType File -Force -Path '%s' | Out-Null\n" \
          "$(_p_winpath "$marker")"
      } > "$path"
      ;;
    ships)
      cat > "$path" <<'PS'
# heliograph-mode: read-only
Write-Output 'the evidence'
exit 7
PS
      ;;
    messy)
      # [Console]::Out.Write and ::Error.Write rather than Write-Output and
      # Write-Error: the point is the exact bytes on the exact stream. A
      # Write-Error would arrive as a formatted ErrorRecord with its own
      # decoration, which is a different thing to capture.
      cat > "$path" <<'PS'
$esc = [char]27
[Console]::Out.Write("$esc[1;31mred line$esc[0m`n")
[Console]::Out.Write("crlf line`r`n")
[Console]::Error.Write("to stderr`n")
[Console]::Out.Write('no trailing newline')
PS
      ;;
    *) return 1 ;;
  esac
}

# A step that prints each line of a file verbatim.
#
# THE LINES ARE READ AT RUN TIME rather than embedded in the script, and that
# is not laziness. The redaction corpus contains quotes, dollars, backticks and
# backslashes - every one of which is a PowerShell metacharacter - and any
# quoting scheme that embedded them would eventually get one wrong and silently
# change the line the redactor is measured against.
drv_step_echo() {
  local path="$1.ps1" lines="$2"
  cp "$lines" "$path.lines"
  cat > "$path" <<'PS'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$name = [System.IO.Path]::GetFileName($MyInvocation.MyCommand.Path)
foreach ($l in [System.IO.File]::ReadAllLines((Join-Path $here ($name + '.lines')))) {
    Write-Output $l
}
PS
}

# A payload with the runner in it. bootstrap.ps1 will do this properly in a
# later PR; until then the driver plants the two files run.ps1 needs, which is
# exactly what it will plant - so when bootstrap arrives, this stops being the
# thing under test rather than changing what is tested.
drv_bootstrap() {
  local dir="$1"
  mkdir -p "$dir/steps" "$dir/ops-logs" || return 1
  cp "$_P_HERE/../../../station/powershell/run.ps1" \
     "$_P_HERE/../../../station/powershell/caplib.psm1" "$dir/" || return 1
  return 0
}

# ALLOW_ROOT=1, and it is not a hole in the test.
#
# p5 asks whether a DECLARED step runs - it is testing the declaration gate,
# and it needs one step that gets through. On a machine where the account is
# already privileged EVERY step refuses with 5, which is the privileged gate
# doing exactly its job, and p5 then fails for a reason that has nothing to do
# with the property it is asserting.
#
# GitHub's Windows runner is an Administrator, so this is not hypothetical; it
# is also true of anyone running the suite in a root container. p6 is what
# tests the privileged gate, and it does NOT set this - so the gate is still
# proved to refuse, by the property written for it.
drv_step() {
  local dir="$1" step="$2"
  ( cd "$dir" && PUSH=0 ALLOW_ROOT=1 \
      "$_P_SHELL" -NoProfile -File ./run.ps1 "./$step" ) >/dev/null 2>&1
}

# WITHOUT BEING ADMINISTRATOR, and without a way to become one.
#
# The bash side puts a fake `id` on PATH, which works because cap_refuse_root
# asks an external program. There is no external program here: the check is
# WindowsPrincipal.IsInRole plus an explicit S-1-5-18, and neither can be
# shadowed by a PATH entry.
#
# So caplib.psm1 has a seam, and it is ONE-DIRECTIONAL:
# HELIOGRAPH_ASSUME_PRIVILEGED=1 can only make the gate REFUSE. There is no
# value of it that permits a run, which is what stops a test hook being a
# backdoor with a test's name on it.
drv_step_privileged() {
  local dir="$1" step="$2"
  ( cd "$dir" && PUSH=0 HELIOGRAPH_ASSUME_PRIVILEGED=1 \
      "$_P_SHELL" -NoProfile -File ./run.ps1 "./$step" ) >/dev/null 2>&1
}

drv_capture() {
  local out="$1" step="$2.ps1"
  "$_P_SHELL" -NoProfile -File "$_P_CAPTURE" -LogPath "$out" -Step "$step" >/dev/null 2>&1
}

# Start a capture in its own process group so the cancel can signal the whole
# tree. On Windows the driver will need a Job Object - that is PR 11's problem,
# and this suite runs the PowerShell implementation on Linux and on Windows
# both, so the difference will be visible rather than assumed.
# The capture writes its OWN pid to the handle, from inside PowerShell, after
# putting itself in a kill group. That is the pid a canceller needs: on Windows
# the bash pid here is Git-Bash's idea of the process and taskkill wants the
# Windows one, and the two are not the same number.
drv_capture_bg() {
  local out="$1" step="$2.ps1" handle="$3"
  rm -f "$handle"
  if [ "$_P_WINDOWS" = "1" ]; then
    "$_P_SHELL" -NoProfile -File "$_P_CAPTURE" -LogPath "$out" -Step "$step" \
      -HandleFile "$(_p_winpath "$handle")" >/dev/null 2>&1 &
  else
    # setsid, so the group exists for Stop-CapTree to signal.
    setsid "$_P_SHELL" -NoProfile -File "$_P_CAPTURE" -LogPath "$out" -Step "$step" \
      -HandleFile "$handle" >/dev/null 2>&1 &
  fi
  # Wait for the handle rather than assuming it is there: PowerShell takes a
  # moment to start, and a canceller reading an empty file would aim at nothing.
  local waited=0
  while [ ! -s "$handle" ]; do
    waited=$((waited + 1))
    [ "$waited" -gt 300 ] && break
    sleep 0.1
  done
}

# Cancelled by the implementation's own mechanism, not by bash. That is the
# point: p8 asks whether THIS station can cancel a run, and answering it with a
# `kill` the station itself would never use would prove nothing about Windows.
drv_cancel() {
  local handle="$1"
  [ -s "$handle" ] || return 1
  "$_P_SHELL" -NoProfile -File "$_P_STOP" \
    -HandleFile "$(_p_winpath "$handle")" >/dev/null 2>&1
}

# Not implemented, and absent rather than stubbed. The suite checks with
# `declare -F` and skips by name, which is the honest report: p5, p6 and p9
# are UNCHECKED against this implementation, not passing.
