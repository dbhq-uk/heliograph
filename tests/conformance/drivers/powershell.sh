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
#   1-4, 7, 8   caplib.psm1 exists. These are its properties.
#   5, 6        the gates live in run.ps1, which does not exist yet
#   9           delivery lives in the transports, which do not exist yet
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
    # On Unix that is `setsid` and a negative pid. Git-Bash on Windows has
    # neither, and a Windows station will use a Job Object with
    # JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE - which is PR 11. Until then this says
    # so and p8 SKIPS on Windows, rather than starting nothing and letting the
    # property report on a log that does not exist.
    cancel) command -v setsid >/dev/null 2>&1 ;;
    # NOT YET, and said out loud. run.ps1 and the transports are PR 10 and 12.
    gates | deliver) return 1 ;;
    *) return 1 ;;
  esac
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
        [ -n "$marker" ] && printf "New-Item -ItemType File -Force -Path '%s' | Out-Null\n" "$marker"
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

drv_capture() {
  local out="$1" step="$2.ps1"
  "$_P_SHELL" -NoProfile -File "$_P_CAPTURE" -LogPath "$out" -Step "$step" >/dev/null 2>&1
}

# Start a capture in its own process group so the cancel can signal the whole
# tree. On Windows the driver will need a Job Object - that is PR 11's problem,
# and this suite runs the PowerShell implementation on Linux and on Windows
# both, so the difference will be visible rather than assumed.
drv_capture_bg() {
  local out="$1" step="$2.ps1" handle="$3"
  setsid "$_P_SHELL" -NoProfile -File "$_P_CAPTURE" -LogPath "$out" -Step "$step" \
    >/dev/null 2>&1 &
  printf '%s' "$!" > "$handle"
}

drv_cancel() {
  local handle="$1" pid waited=0
  pid="$(cat "$handle" 2>/dev/null)" || return 1
  [ -n "$pid" ] || return 1
  kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null
  while kill -0 "$pid" 2>/dev/null; do
    waited=$((waited + 1))
    if [ "$waited" -gt 50 ]; then
      kill -KILL -- "-$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null
      sleep 0.5
      kill -0 "$pid" 2>/dev/null && return 1
      break
    fi
    sleep 0.1
  done
  wait "$pid" 2>/dev/null
  return 0
}

# Not implemented, and absent rather than stubbed. The suite checks with
# `declare -F` and skips by name, which is the honest report: p5, p6 and p9
# are UNCHECKED against this implementation, not passing.
