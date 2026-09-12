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
#   9               transports/{git,share,relay}.psm1 deliver. The relay needs
#                   no binary on the station side - lib/seal.psm1 does the
#                   construction - but the CONTROL side of this harness reads
#                   the delivery back with heliograph-seal, so p9 skips it when
#                   there is no Go toolchain or no python3 here
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

# The relay's constants, in one place because the stub, the station's env and
# the control-side read all have to agree on them. Same values the bash driver
# uses, so the two runs are comparable.
_P_RELAY_ESTATE=conformance
_P_RELAY_CTL=control-token
_P_RELAY_STN=station-token
_P_RELAY_BASES=""

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
    # DELIVERY, over whichever transport CONF_TRANSPORT names. Only the ones
    # this implementation actually ships, and only where the far side can be
    # stood up: a driver that claimed one it could not read would report a
    # channel as proven that was never dialled.
    deliver)
      case "$CONF_TRANSPORT" in
        git | share) return 0 ;;
        # THE RELAY NEEDS NO BINARY ON THIS SIDE - that is the whole point of
        # lib/seal.psm1 - but the CONTROL side of this harness reads the
        # delivery back with `heliograph-seal`, and the stub is Python. Without
        # either, p9 would be asserted against a far side nobody can read.
        #
        # Said out loud rather than passed. A driver that claimed the relay and
        # returned 0 would report a channel as proven that was never dialled.
        relay)
          command -v go >/dev/null 2>&1 || return 1
          command -v python3 >/dev/null 2>&1 || return 1
          return 0
          ;;
        *) return 1 ;;
      esac
      ;;
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
# The WHOLE payload, because a transport needs lib/ and transports/ and the
# runner needs both. bootstrap.ps1 will do this properly in a later PR; until
# then the driver plants exactly what that will plant, so when bootstrap
# arrives it stops being the thing under test rather than changing what is.
drv_bootstrap() {
  local dir="$1"
  mkdir -p "$dir/steps" "$dir/ops-logs" || return 1
  cp -r "$_P_HERE/../../../station/powershell/." "$dir/" || return 1
  _drv_ps_farside "$dir"
}

# --- the far side, per transport ---------------------------------------------
# The same two the bash driver stands up, and read back the same way: from the
# RECEIVING END, never from the working tree that wrote it. A station with no
# far side would let a delivery that never happened look identical to one that
# did, which is precisely the defect p9 exists to catch.
_drv_ps_farside() {
  local dir="$1"
  case "$CONF_TRANSPORT" in
    git)
      (
        cd "$dir" || exit 1
        git init -q .
        git -c user.email=ci@example.invalid -c user.name=ci add -A
        git -c user.email=ci@example.invalid -c user.name=ci commit -qm init
        git init -q --bare "$dir.remote.git"
        git remote add origin "$dir.remote.git"
        git push -q -u origin HEAD
      ) >/dev/null 2>&1
      ;;
    share)
      # The share root only. NOT the scope under it: Initialize-Tp must create
      # nothing, because `--check` has to run where nobody may alter anything.
      mkdir -p "$dir.share"
      ;;
    relay)
      _drv_ps_relay_farside "$dir"
      ;;
    *) return 0 ;;
  esac
}

# --- relay: the same stub the bash driver uses, and NO binary on this side ----
#
# THE ASYMMETRY IS THE POINT. `heliograph-seal` is built here and used only by
# the CONTROL side - to make the keys and to read the delivery back. The station
# never sees it: `RELAY_SEAL` is deliberately not exported below, and
# transports/relay.psm1 never looks for one.
#
# So this run proves the thing the whole seal port exists for - a station with
# no native binary anywhere on it exchanging sealed messages with a control side
# that has one. If lib/seal.psm1 and internal/seal ever diverge by a byte, the
# read at the end of p9 returns nothing and the property fails.
_drv_ps_relay_farside() {
  local dir="$1" base="$1.relay" port pid waited=0
  mkdir -p "$base" || return 1

  # Built from THIS tree. A conformance run against last release's binary would
  # assert that an old control side still works, which is a different question.
  ( cd "$_P_HERE/../../.." && go build -o "$base/heliograph-seal" ./cmd/heliograph-seal ) \
    >/dev/null 2>&1 || return 1

  # Two identities, because that is the real topology: the control side signs
  # requests and the station verifies them, and the reverse for logs. One shared
  # key would pass while proving nothing about either direction.
  "$base/heliograph-seal" keygen --out "$base/station.key" >/dev/null 2>&1 || return 1
  "$base/heliograph-seal" keygen --out "$base/control.key" >/dev/null 2>&1 || return 1
  "$base/heliograph-seal" public --identity "$base/station.key" > "$base/station.pub" 2>/dev/null || return 1
  "$base/heliograph-seal" public --identity "$base/control.key" > "$base/control.pub" 2>/dev/null || return 1

  # Port 0: the OS picks and the stub prints what it got. A fixed port makes a
  # test that cannot run twice at once, and CI runs these in parallel.
  #
  # TWO TOKENS, because the relay's scopes are asymmetric - a station may
  # collect a request and publish a log, and may not queue a request even for
  # itself. A single token would let a station-side regression that used the
  # wrong credential pass here and be refused by a real relay.
  python3 "$_P_HERE/../relay-stub.py" "$_P_RELAY_CTL" "$_P_RELAY_STN" "$_P_RELAY_ESTATE" 0 \
    > "$base/port" 2>"$base/stub.err" &
  pid=$!
  echo "$pid" > "$base/pid"
  _P_RELAY_BASES="$_P_RELAY_BASES $base"

  # HEALTH, not a number in a file. A port that parses proves the stub printed
  # something, not that it is listening - and a partially written value parses
  # as a different port entirely. The process is checked for still being alive
  # on every turn, or a stub that died on startup is waited out to the timeout.
  while :; do
    port="$(cat "$base/port" 2>/dev/null)"
    case "$port" in
      '' | *[!0-9]*) ;;
      *) curl -sS -o /dev/null -m 2 "http://127.0.0.1:$port/health" 2>/dev/null && break ;;
    esac
    kill -0 "$pid" 2>/dev/null || { echo "relay stub died:" >&2; cat "$base/stub.err" >&2; return 1; }
    waited=$((waited + 1))
    [ "$waited" -gt 100 ] && return 1
    sleep 0.1
  done
  printf 'http://127.0.0.1:%s\n' "$port" > "$base/url"
  return 0
}

_p_relay_url() { cat "$1/url" 2>/dev/null; }

_drv_ps_env() {
  local dir="$1" p
  case "$CONF_TRANSPORT" in
    git)
      p="$(_p_winpath "$dir")"
      export TRANSPORT=git REPO_ROOT="$p"
      ;;
    share)
      p="$(_p_winpath "$dir.share")"
      export TRANSPORT=share SHARE_DIR="$p" SHARE_SCOPE=conformance
      ;;
    relay)
      local base="$dir.relay"
      # NO RELAY_SEAL. The bash driver exports one because relay.sh shells out
      # to it; this station has no binary and must not be handed a path to one,
      # or the run would not be testing what it claims to.
      export TRANSPORT=relay
      RELAY_URL="$(_p_relay_url "$base")"
      RELAY_IDENTITY="$(_p_winpath "$base/station.key")"
      RELAY_PEER="$(_p_winpath "$base/control.pub")"
      RELAY_STATE="$(_p_winpath "$base/state")"
      export RELAY_URL RELAY_IDENTITY RELAY_PEER RELAY_STATE
      export RELAY_ESTATE="$_P_RELAY_ESTATE" RELAY_STATION=station RELAY_TOKEN="$_P_RELAY_STN"
      ;;
    *) return 1 ;;
  esac
}

# Run a step and let it DELIVER. No PUSH=0 here, deliberately: the delivery is
# the thing under test.
drv_deliver() {
  local dir="$1" step="$2"
  ( cd "$dir" && _drv_ps_env "$dir" && ALLOW_ROOT=1 \
      "$_P_SHELL" -NoProfile -File ./run.ps1 "./$step" ) >/dev/null 2>&1
}

# Read back what the far side ACTUALLY RECEIVED.
drv_delivered() {
  local dir="$1"
  case "$CONF_TRANSPORT" in
    git)
      local remote="$dir.remote.git" branch name
      branch="$(git -C "$dir" rev-parse --abbrev-ref HEAD 2>/dev/null)" || return 1
      name="$(git -C "$remote" ls-tree -r --name-only "$branch" 2>/dev/null \
                | grep '^ops-logs/.*\.txt$' | tail -1)"
      [ -n "$name" ] || return 1
      git -C "$remote" show "$branch:$name" 2>/dev/null
      ;;
    share)
      local newest
      newest="$(ls -1 "$dir.share/conformance/ops-logs/"*.txt 2>/dev/null | tail -1)"
      [ -n "$newest" ] || return 1
      cat "$newest"
      ;;
    relay)
      # COLLECTED FROM THE CONTROL SIDE AND UNSEALED, which is the whole point.
      # The station sealed this with lib/seal.psm1 and Go opens it here; a byte
      # of divergence between the two implementations returns nothing and p9
      # fails, which is the only way that divergence is ever visible.
      local base="$dir.relay" tmp out rc
      tmp="$(mktemp)" || return 1
      # THE CONTROL TOKEN. Collecting from s2c is the control side's move, and
      # using the station's here would be a reader that only works against a
      # stub with one token - which is how the asymmetry stops being tested.
      curl -sS -m 10 -H "Authorization: Bearer $_P_RELAY_CTL" \
        "$(_p_relay_url "$base")/v1/$_P_RELAY_ESTATE/station/s2c" \
        > "$tmp" 2>/dev/null || { rm -f "$tmp"; return 1; }
      # The whole array, handed over as it came. `open` sorts by sequence and
      # takes the newest that opens as this KIND - so asking for `log` is what
      # discards the `status` the station published alongside it.
      out="$("$base/heliograph-seal" open \
              --identity "$base/control.key" --peer "$base/station.pub" \
              --estate "$_P_RELAY_ESTATE" --station station \
              --dir s2c --kind log --min-seq 0 --in "$tmp" 2>/dev/null)"
      rc=$?
      rm -f "$tmp"
      [ "$rc" = "0" ] || return 1
      # The accepted sequence is on the first line and the document after it.
      printf '%s\n' "$out" | tail -n +2
      ;;
    *) return 1 ;;
  esac
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

# The stubs are background processes and the suite's own trap only removes the
# work directory, which would leave them running and their ports held.
#
# EVERY ONE, not the last. Several properties bootstrap, so several stubs get
# started, and tearing down only the one p9 used leaks the rest for the life of
# the shell.
drv_teardown() {
  local base pid waited
  for base in $_P_RELAY_BASES; do
    pid="$(cat "$base/pid" 2>/dev/null)" || continue
    [ -n "$pid" ] || continue
    kill "$pid" 2>/dev/null
    # TERM, then wait for it, then insist. A kill that is merely sent proves
    # nothing: the port stays held until the process actually goes, and the
    # next property binding port 0 would be racing a corpse.
    waited=0
    while kill -0 "$pid" 2>/dev/null; do
      waited=$((waited + 1))
      [ "$waited" -gt 50 ] && { kill -9 "$pid" 2>/dev/null; break; }
      sleep 0.1
    done
    wait "$pid" 2>/dev/null
  done
  _P_RELAY_BASES=""
  return 0
}
