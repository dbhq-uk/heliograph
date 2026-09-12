#!/usr/bin/env bash
# =============================================================================
#  test-service-ps1.sh - the PowerShell payload's service installer
# =============================================================================
# `--flavour powershell` planted no way to survive a logout at all. The bash
# payload's service.ps1 registers the LAUNCHER and refuses to install without a
# start.sh beside it, so on this payload it refuses every time.
#
# THE TASK REGISTRATION IS NOT THE HARD PART, and it is not what most of this
# file tests. A scheduled task starts with a FRESH ENVIRONMENT and inherits
# nothing from the shell that registered it - so the transport's variables and
# any credential simply are not there. The loop then starts, polls happily,
# captures a perfect log and cannot deliver it, and the far side waits for hours
# with nothing reporting a fault.
#
# So the assertions here are mostly about `.station-env-ps`: that the installer
# writes it, that the loop READS it, that the environment still wins over it,
# and that an uninstall takes it away because it may hold a token.
#
# WHAT CANNOT BE TESTED HERE. Register-ScheduledTask is Windows-only, so the
# install/status/uninstall verbs are exercised in CI on the Windows runner and
# SKIP loudly here. The env carriage is portable, and it is the half that fails
# silently, so it is tested everywhere.
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
  t_skip "no PowerShell interpreter: the service installer was NOT exercised."
  t_summary
  exit 0
fi
t_ok "a PowerShell interpreter is present ($PS_BIN), so the assertions below ran"

WORK="$(mktemp -d)"
cleanup() {
  [ -n "${LOOP_PID:-}" ] && { kill -TERM "$LOOP_PID" 2>/dev/null; wait "$LOOP_PID" 2>/dev/null; }
  rm -rf "$WORK"
}
trap cleanup EXIT

winpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s' "$1"; fi
}
IS_WINDOWS=0
case "$(uname -s 2>/dev/null)" in MINGW* | MSYS* | CYGWIN*) IS_WINDOWS=1 ;; esac

D="$WORK/payload"; S="$WORK/far"
mkdir -p "$D/steps" "$D/ops-logs" "$S/scope"
cp -r "$PSDIR/." "$D/"
printf '# heliograph-mode: read-only\nWrite-Output "the evidence"\n' > "$D/steps/probe.ps1"

svc() {  # svc <env...> -- <args...>
  local envs=() a
  while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do envs+=("$1"); shift; done
  shift || true
  a=("$@")
  SVC_OUT="$( cd "$D" && env "${envs[@]+"${envs[@]}"}" \
                timeout 120 "$PS_BIN" -NoProfile -File ./service.ps1 "${a[@]}" 2>&1 )"
  SVC_RC=$?
}

ENVFILE="$D/.station-env-ps"

# =============================================================================
#  1. It exists at all, and says what it is for
# =============================================================================
assert_eq "the PowerShell payload ships a service installer" "yes" \
  "$([ -f "$PSDIR/service.ps1" ] && echo yes || echo no)"

svc -- ''
assert_eq "with no verb it prints usage and exits 2" "2" "$SVC_RC"
assert_contains "  and names the trap it exists for" "inherits none of them" "$SVC_OUT"

# =============================================================================
#  2. IT REFUSES TO INSTALL A LOOP THAT COULD NOT DELIVER
# =============================================================================
# The credential is where an unattended loop fails, and it fails silently. The
# point of checking at install time is that somebody is still watching.
svc TRANSPORT=share -- install
assert_contains "install refuses when the share transport has no directory" \
  "SHARE_DIR is not set" "$SVC_OUT"
assert_eq "  and wrote no config file" "no" \
  "$([ -e "$ENVFILE" ] && echo yes || echo no)"

# THE REFUSAL IS OVERRIDABLE, because an operator who knows better must not be
# stuck. It is a refusal rather than a warning so that the default is safe.
# Not run on Linux: -Force proceeds to Register-ScheduledTask, which is Windows.
if [ "$IS_WINDOWS" = "0" ]; then
  t_skip "Register-ScheduledTask is Windows-only, so -Force and the install verbs were NOT exercised here (CI does it)"
fi

# =============================================================================
#  3. THE CONFIGURATION IS CARRIED, and this is the half that matters
# =============================================================================
# Write-EnvFile is what a detached task depends on. It is exercised directly,
# because on Linux the install verb cannot get that far.
cat > "$WORK/carry.ps1" <<PSEOF
\$RepoRoot = '$(winpath "$D")'
. { \$args } > \$null   # no-op, keeps the parser honest about what follows
# Pull the two functions out of service.ps1 without registering anything: the
# task API is Windows-only and the carriage is not.
\$src = [System.IO.File]::ReadAllText((Join-Path \$RepoRoot 'service.ps1'))
foreach (\$fn in 'Write-EnvFile', 'Protect-EnvFile') {
    \$m = [regex]::Match(\$src, "(?s)function \$fn \{.*?\n\}\n")
    if (-not \$m.Success) { Write-Output "MISSING \$fn"; exit 1 }
    Invoke-Expression \$m.Value
}
\$CarriedVars = @('TRANSPORT','SHARE_DIR','SHARE_SCOPE','GIT_TOKEN','ALLOW_ACTIONS')
\$EnvFile = Join-Path \$RepoRoot '.station-env-ps'
\$carried = Write-EnvFile
Write-Output ("carried: " + (\$carried -join ','))
PSEOF
CARRY_OUT="$( cd "$D" && env TRANSPORT=share "SHARE_DIR=$(winpath "$S")" SHARE_SCOPE=scope \
    'GIT_TOKEN=tok en"with:awkward\chars' ALLOW_ACTIONS=1 \
    timeout 60 "$PS_BIN" -NoProfile -File "$(winpath "$WORK/carry.ps1")" 2>&1 )"
assert_contains "the installer records the transport's variables" "carried: TRANSPORT" "$CARRY_OUT"
assert_eq "  and the file is there" "yes" "$([ -e "$ENVFILE" ] && echo yes || echo no)"

# NOT EXECUTABLE CODE. This file sits in a directory the far side can write to
# on some transports, so a `.ps1` the loop ran at every start would hand it code
# execution. It is KEY=value and is read, never evaluated.
assert_eq "  and it is KEY=value, not PowerShell" "no" \
  "$(grep -qE '^\s*(\$|function |Invoke-|&\s)' "$ENVFILE" 2>/dev/null && echo yes || echo no)"

# A TOKEN SURVIVES VERBATIM. No quoting scheme, so nothing to get wrong: the
# value is everything after the first `=`. A credential that differs by one byte
# fails authentication with a message that says nothing about quoting.
assert_contains "  and an awkward token is written verbatim" \
  'GIT_TOKEN=tok en"with:awkward\chars' "$(cat "$ENVFILE")"

# =============================================================================
#  4. AND THE LOOP ACTUALLY READS IT
# =============================================================================
# The half that would be easy to leave out. An installer that writes a file
# nothing consumes is worse than no installer: it reports success.
#
# Proved by starting the loop with NO transport variables in its environment at
# all - exactly what a scheduled task gets - and watching a log reach the far
# side.
printf 'id: svc-1\nstep: ./steps/probe.ps1\n' > "$S/scope/request"
cat > "$ENVFILE" <<EOF
# written by the test
TRANSPORT=share
SHARE_DIR=$(winpath "$S")
SHARE_SCOPE=scope
ALLOW_ROOT=1
EOF
# The loop's own chatter is not the evidence: a LOG ON THE FAR SIDE is.
[ -n "$( cd "$D" && env -u TRANSPORT -u SHARE_DIR -u SHARE_SCOPE -u ALLOW_ROOT \
    timeout 120 "$PS_BIN" -NoProfile -File ./station.ps1 --once --interval 1 2>&1 )" ] || true
STATE="$(sed -n 's/^state:[[:space:]]*//p' "$S/scope/status" 2>/dev/null | head -1)"
assert_eq "the loop reads its configuration from the file, with nothing in the environment" \
  "idle" "$STATE"
assert_eq "  and a log reached the far side, which is the only proof it worked" "1" \
  "$(ls -1 "$S/scope/ops-logs/"*.txt 2>/dev/null | wc -l | tr -d ' ')"

# THE ENVIRONMENT WINS. An operator debugging by hand must be able to override
# what the service was installed with, without finding and editing a dotfile.
rm -rf "$S/scope"; mkdir -p "$S/scope"
S2="$WORK/far2"; mkdir -p "$S2/scope"
printf 'id: svc-2\nstep: ./steps/probe.ps1\n' > "$S2/scope/request"
( cd "$D" && env TRANSPORT=share "SHARE_DIR=$(winpath "$S2")" SHARE_SCOPE=scope ALLOW_ROOT=1 \
    timeout 120 "$PS_BIN" -NoProfile -File ./station.ps1 --once --interval 1 ) >/dev/null 2>&1
assert_eq "the environment overrides the file, so a hand run goes where you said" "idle" \
  "$(sed -n 's/^state:[[:space:]]*//p' "$S2/scope/status" 2>/dev/null | head -1)"
assert_eq "  and the file's own far side was left alone" "0" \
  "$(ls -1 "$S/scope/ops-logs/"*.txt 2>/dev/null | wc -l | tr -d ' ')"

# =============================================================================
#  5. AND SO DOES THE PREFLIGHT, WHICH IS WHAT THE TASK ACTUALLY RUNS
# =============================================================================
# THE DEFECT THIS EXISTS FOR, found by CI on a real Windows runner and not by
# section 4. The reader lived in station.ps1 alone, and the task registers
# `start.ps1`, which preflights FIRST and hands over only if it passes. So a
# task installed with `TRANSPORT=share ALLOW_ROOT=1` in its config file was
# refused twice over - for being an Administrator, and for running the `git`
# transport - and never reached the loop that would have read either. The
# install reported success. The table named neither the config file nor the
# task, so every line of it pointed at the machine.
#
# Section 4 passed throughout: the loop did read the file. Testing the half you
# wrote rather than the half the operator runs is what let this ship.
rm -rf "$S/scope"; mkdir -p "$S/scope"
cat > "$ENVFILE" <<EOF
TRANSPORT=share
SHARE_DIR=$(winpath "$S")
SHARE_SCOPE=scope
ALLOW_ROOT=1
EOF

# A TWIN COMPARISON. The same preflight, once with the variables in the
# environment and once with them only in the file, must reach the same verdict
# on the transport - because the loop behind it is going to.
preflight() {  # preflight <env...>  -> PRE_OUT / PRE_RC
  PRE_OUT="$( cd "$D" && env -u TRANSPORT -u SHARE_DIR -u SHARE_SCOPE -u ALLOW_ROOT \
      "$@" timeout 120 "$PS_BIN" -NoProfile -File ./start.ps1 --check 2>&1 )"
  PRE_RC=$?
}
tp_line() { printf '%s' "$1" | grep -E '^(ok|warn|FAIL)  (transport|reach)' | tr -s ' '; }

preflight
FROM_FILE="$(tp_line "$PRE_OUT")"
FILE_RC=$PRE_RC
preflight TRANSPORT=share "SHARE_DIR=$(winpath "$S")" SHARE_SCOPE=scope ALLOW_ROOT=1
FROM_ENV="$(tp_line "$PRE_OUT")"

assert_eq "the preflight reaches the same verdict from the file as from the environment" \
  "$FROM_ENV" "$FROM_FILE"
assert_eq "  and that verdict is not a refusal, or the two agree only in failing" "no" \
  "$(printf '%s' "$FROM_FILE" | grep -q FAIL && echo yes || echo no)"
assert_eq "  so a task with only a config file preflights clean" "0" "$FILE_RC"

# THE TABLE SAYS WHERE THE SETTINGS CAME FROM. An operator staring at a refusal
# has no other way to tell "the task is misconfigured" from "my shell is", and
# that distinction is invisible in a line that only names the variable.
preflight
assert_contains "the table names the config file it read" "config" "$PRE_OUT"
assert_contains "  and the variables it took from it" "TRANSPORT" \
  "$(printf '%s' "$PRE_OUT" | grep '^ok    config')"

# AND IT NO LONGER SAYS THE PAYLOAD HAS NO INSTALLER. That warning was true
# until this branch and is now the opposite of true, printed by the very script
# the installer registers.
assert_eq "the preflight stops claiming this payload ships no service installer" "no" \
  "$(printf '%s' "$PRE_OUT" | grep -q 'ships no service installer' && echo yes || echo no)"
assert_contains "  and points at the installer instead" "service.ps1 install" "$PRE_OUT"

# --check STILL CHANGES NOTHING, including not consuming the request.
assert_eq "and --check started no station" "0" \
  "$(ls -1 "$S/scope/ops-logs/"*.txt 2>/dev/null | wc -l | tr -d ' ')"

# =============================================================================
#  6. A NEWLINE IS REFUSED RATHER THAN STRIPPED
# =============================================================================
# The same injection the share transport refuses in a scope name: a newline in
# a value forges a second variable. Refused, because silently changing a
# credential is worse than not writing it.
cat > "$WORK/inject.ps1" <<PSEOF
\$RepoRoot = '$(winpath "$D")'
\$src = [System.IO.File]::ReadAllText((Join-Path \$RepoRoot 'service.ps1'))
\$m = [regex]::Match(\$src, "(?s)function Write-EnvFile \{.*?\n\}\n")
Invoke-Expression \$m.Value
\$p = [regex]::Match(\$src, "(?s)function Protect-EnvFile \{.*?\n\}\n")
Invoke-Expression \$p.Value
\$CarriedVars = @('SHARE_SCOPE')
\$EnvFile = Join-Path \$RepoRoot '.station-env-inject'
try { [void](Write-EnvFile); Write-Output 'ACCEPTED' } catch { Write-Output "REFUSED: \$(\$_.Exception.Message)" }
PSEOF
INJ="$( cd "$D" && env "SHARE_SCOPE=$(printf 'x\nGIT_TOKEN=stolen')" \
    timeout 60 "$PS_BIN" -NoProfile -File "$(winpath "$WORK/inject.ps1")" 2>&1 | tr -d '\r' | tail -1 )"
assert_contains "a newline in a value is refused, not stripped" "REFUSED" "$INJ"
assert_eq "  and no file was left behind carrying the forged variable" "no" \
  "$([ -e "$D/.station-env-inject" ] && echo yes || echo no)"

# =============================================================================
#  7. The whole payload is planted, including this file
# =============================================================================
# A service installer that is not planted is a service installer nobody has.
PLANT="$WORK/planted"
"$HERE/../station/bootstrap.sh" "$PLANT" --flavour powershell >/dev/null 2>&1
assert_eq "bootstrap plants service.ps1 with the payload" "yes" \
  "$([ -f "$PLANT/service.ps1" ] && echo yes || echo no)"
# AND NOT THE CONFIG FILE. It holds a token; planting one would hand a new
# transport repo the last machine's credential.
assert_eq "  and never the config file, which may hold a credential" "no" \
  "$([ -e "$PLANT/.station-env-ps" ] && echo yes || echo no)"
# The module that reads it. Planted separately from service.ps1 because it is
# what makes the file worth writing, and the preflight refuses without it.
assert_eq "  and the module that reads the config, or the installer writes to nothing" "yes" \
  "$([ -f "$PLANT/lib/stationenv.psm1" ] && echo yes || echo no)"

t_summary
