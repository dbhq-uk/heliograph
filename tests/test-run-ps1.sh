#!/usr/bin/env bash
# =============================================================================
#  test-run-ps1.sh - run.ps1's gates, and the exit codes they promise
# =============================================================================
# The conformance suite asks whether the gates FAIL CLOSED, which is the
# property. This asks the rest: the exact exit codes, the refusals that are not
# a gate at all, and the two questions a security gate has to answer that no
# property covers -
#
#   does a refusal write a log?          A refusal that leaves a log looks like
#                                        a run that happened.
#   can the test seam open the gate?     HELIOGRAPH_ASSUME_PRIVILEGED exists so
#                                        a test can pretend to be Administrator.
#                                        If any value of it PERMITTED a run it
#                                        would be a backdoor with a test's name
#                                        on it, so that is asserted directly.
#
# EXIT CODES ARE THE CONTRACT and they are run.sh's, so they are asserted as
# numbers rather than as "non-zero". A gate that refuses with the wrong code is
# a gate the loop will misreport.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
PSDIR="$(cd "$HERE/../station/powershell" && pwd)"

PS_BIN=""
for c in ${CONF_PS_SHELL:-} pwsh powershell powershell.exe; do
  [ -n "$c" ] || continue
  if command -v "$c" >/dev/null 2>&1; then PS_BIN="$c"; break; fi
done
if [ -z "$PS_BIN" ]; then
  t_skip "no PowerShell interpreter: run.ps1's gates were NOT exercised."
  # --- THE SHIPPED STEP ACTUALLY RUNS ------------------------------------------
# The default step must work, or `.\run.ps1` with no arguments - the one command
# an operator is asked to remember - fails on a fresh payload.
#
# AND IT MUST REPORT ZERO FAILED PROBES on the platform it is run on. That is
# the assertion, not "it exited 0": three defects hid behind an exit code here.
# `$LASTEXITCODE` is set only by a native call, so probes that ran nothing
# native inherited the previous probe's code and reported failures they had
# nothing to do with. `WindowsIdentity::GetCurrent()` threw everywhere but
# Windows. And `exit (Write-ProbeSummary)` captured the tally into the
# parentheses instead of printing it, so the log ended with no summary at all -
# which reads exactly like a step that was cut off.
SHIPPED="$WORK/shipped"
mkdir -p "$SHIPPED"
cp -r "$PSDIR/." "$SHIPPED/"
step_out="$( cd "$SHIPPED" && PUSH=0 "$PS_BIN" -NoProfile -File ./run.ps1 env 2>&1 )"
step_rc=$?
assert_eq "the shipped default step runs, exit 0" "0" "$step_rc"
assert_contains "and prints its summary, which is the last thing a reader looks for" \
  "probes, " "$step_out"
tally="$(printf '%s' "$step_out" | sed -n 's/.*| \([0-9]*\) probes, \([0-9]*\) failed.*/\1 \2/p' | tail -1)"
case "$tally" in
  *" 0") t_ok "and every required probe passed here: $tally" ;;
  "")    t_no "the step printed no tally, so nothing above was measured" ;;
  *)     t_no "the shipped step has failing probes on this platform: $tally"
         printf '%s\n' "$step_out" | grep -A 2 'FAILED:' | sed 's/^/     /' | head -12 ;;
esac
assert_contains "and it asks the question that decides whether a station can run here" \
  "LanguageMode" "$step_out"

# --- THE TWINS AGREE, case by case -------------------------------------------
# Every divergence above was found by hand, one spelling at a time, and two of
# them were in a security gate: `CONFIRM=YES` ran a state-changing step here and
# was refused by run.sh, and `# heliograph-mode: READ-ONLY` did the same. Both
# because PowerShell compares case-insensitively and bash does not.
#
# Finding those by hand does not scale and does not stay found. So the same
# DECLARATION goes through both runners and the exit codes must match. The body
# of each step differs - one is bash, one is PowerShell - but the header is the
# thing under test, and it is byte-identical.
BASHREPO="$WORK/bashrepo"
if "$HERE/../station/bootstrap.sh" "$BASHREPO" >/dev/null 2>&1; then

  # case <name> <header> <env...> - runs it both ways, compares the codes
  twin() {
    local name="$1" header="$2"; shift 2
    local envs=("$@") brc prc

    printf '#!/usr/bin/env bash\n%s\necho ran\n' "$header" > "$BASHREPO/steps/twin.sh"
    chmod +x "$BASHREPO/steps/twin.sh"
    printf '%s\nWrite-Output "ran"\n' "$header" > "$WORK/steps/twin.ps1"

    ( cd "$BASHREPO" && env "${envs[@]+"${envs[@]}"}" PUSH=0 \
        ./run.sh ./steps/twin.sh ) >/dev/null 2>&1
    brc=$?
    ( cd "$WORK" && env "${envs[@]+"${envs[@]}"}" PUSH=0 \
        "$PS_BIN" -NoProfile -File ./run.ps1 ./steps/twin.ps1 ) >/dev/null 2>&1
    prc=$?

    if [ "$brc" = "$prc" ]; then
      t_ok "twins agree on $name: both exit $brc"
    else
      t_no "twins DISAGREE on $name: run.sh exits $brc, run.ps1 exits $prc"
      printf '     The same declaration means two different things to two readers.\n'
    fi
  }

  twin "a read-only step"              "# heliograph-mode: read-only"
  twin "no declaration at all"         "# nothing declared here"
  twin "an unrecognised mode"          "# heliograph-mode: banana"
  twin "a mode in capitals"            "# heliograph-mode: READ-ONLY"
  twin "a header in capitals"          "# HELIOGRAPH-MODE: read-only"
  twin "a mode with odd spacing"       "#heliograph-mode:read-only"
  twin "an action with no CONFIRM"     "# heliograph-mode: action"
  twin "an action with CONFIRM=yes"    "# heliograph-mode: action" CONFIRM=yes
  twin "an action with CONFIRM=YES"    "# heliograph-mode: action" CONFIRM=YES
  twin "an action with CONFIRM=1"      "# heliograph-mode: action" CONFIRM=1
  twin "an action with CONFIRM=true"   "# heliograph-mode: action" CONFIRM=true
else
  t_no "could not bootstrap a bash payload, so the twins were NOT compared"
fi

t_summary
  exit 0
fi
t_ok "a PowerShell interpreter is present ($PS_BIN), so the assertions below ran"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/steps"
cp "$PSDIR/run.ps1" "$PSDIR/caplib.psm1" "$WORK/"

printf '# heliograph-mode: read-only\nWrite-Output "ran ok"\n'      > "$WORK/steps/ok.ps1"
printf 'Write-Output "no declaration"\n'                            > "$WORK/steps/undeclared.ps1"
printf '# heliograph-mode: action\nWrite-Output "changed state"\n'  > "$WORK/steps/act.ps1"
printf '# heliograph-mode: banana\nWrite-Output "x"\n'              > "$WORK/steps/bad.ps1"
# A declaration BELOW the 30-line window, which must not count. The window is
# the whole point of the gate being cheap: it reads a header, not a program.
{ for i in $(seq 1 35); do printf '# filler %s\n' "$i"; done
  printf '# heliograph-mode: read-only\nWrite-Output "too late"\n'; } > "$WORK/steps/late.ps1"

# run <env...> -- <args...>  -> sets RC, OUT, ERR
run() {
  local envs=() a
  while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do envs+=("$1"); shift; done
  shift || true
  a=("$@")
  OUT="$( cd "$WORK" && env "${envs[@]+"${envs[@]}"}" PUSH=0 \
            "$PS_BIN" -NoProfile -File ./run.ps1 "${a[@]}" 2>"$WORK/err" )"
  RC=$?
  ERR="$(cat "$WORK/err")"
}

logs() { find "$WORK/ops-logs" -name '*.txt' 2>/dev/null | wc -l | tr -d ' '; }

# --- gate 1: a step declares itself, or it does not run ----------------------
run -- ./steps/ok.ps1
assert_eq "a declared read-only step runs, exit 0" "0" "$RC"
assert_contains "and its output reaches the log" "ran ok" "$OUT"

before="$(logs)"
run -- ./steps/undeclared.ps1
assert_eq "an undeclared step refuses with exit 3" "3" "$RC"
assert_contains "and says what to add, rather than just refusing" \
  "heliograph-mode: read-only" "$ERR"
assert_eq "and writes NO log: a refusal that leaves one looks like a run" \
  "$before" "$(logs)"

run -- ./steps/bad.ps1
assert_eq "an unrecognised mode refuses with exit 3, not a guess" "3" "$RC"
assert_contains "and names the two values it will accept" \
  "'read-only' and 'action'" "$ERR"

run -- ./steps/late.ps1
assert_eq "a declaration below the 30-line window does not count" "3" "$RC"

# CASE-SENSITIVE, all the way through, because PowerShell's comparisons are not
# and bash's are. Each of these ran on PowerShell and refused on bash before it
# was fixed - the same file meaning two different things to two readers, which
# is the one thing a twin may not do.
printf '# heliograph-mode: READ-ONLY\nWrite-Output "shouted"\n'  > "$WORK/steps/shout.ps1"
printf '# HELIOGRAPH-MODE: read-only\nWrite-Output "shouted"\n'  > "$WORK/steps/shouthdr.ps1"
run -- ./steps/shout.ps1
assert_eq "a mode spelled READ-ONLY is not read-only" "3" "$RC"
run -- ./steps/shouthdr.ps1
assert_eq "a header spelled HELIOGRAPH-MODE is not a declaration" "3" "$RC"
run -- ENV
assert_eq "a step named ENV is not the step named env" "2" "$RC"

# --- gate 2: an action needs CONFIRM=yes -------------------------------------
before="$(logs)"
run -- ./steps/act.ps1
assert_eq "an action without CONFIRM refuses with exit 3" "3" "$RC"
assert_contains "and says exactly how to re-run it" "CONFIRM" "$ERR"
assert_eq "and writes no log either" "$before" "$(logs)"

run CONFIRM=yes -- ./steps/act.ps1
assert_eq "the same action runs with CONFIRM=yes" "0" "$RC"
assert_contains "and actually executed" "changed state" "$OUT"

# CONFIRM is `yes` and nothing else. `1`, `true` and `YES` are the three things
# somebody types instead, and each would be a state change nobody confirmed.
for v in 1 true YES y; do
  run "CONFIRM=$v" -- ./steps/act.ps1
  assert_eq "CONFIRM=$v is not CONFIRM=yes, so the action still refuses" "3" "$RC"
done

# --- gate 4: nothing runs as the privileged account --------------------------
before="$(logs)"
run HELIOGRAPH_ASSUME_PRIVILEGED=1 -- ./steps/ok.ps1
assert_eq "a privileged account refuses with exit 5" "5" "$RC"
assert_contains "and says the account is the blast radius" "blast radius" "$ERR"
assert_eq "and writes no log, so the refusal precedes the capture" \
  "$before" "$(logs)"

run HELIOGRAPH_ASSUME_PRIVILEGED=1 ALLOW_ROOT=1 -- ./steps/ok.ps1
assert_eq "ALLOW_ROOT=1 permits it, and keeps the bash spelling" "0" "$RC"

# THE SEAM IS ONE-DIRECTIONAL, and this is the assertion that makes it safe to
# have at all. HELIOGRAPH_ASSUME_PRIVILEGED exists so a test can pretend to be
# Administrator without being one. If any value of it PERMITTED a run it would
# be a backdoor: an attacker who can set an environment variable could turn the
# gate off. So every value that is not `1` must leave the gate exactly as it
# was, and `1` must only ever refuse.
opened=""
for v in 0 no false '' yes 2 1; do
  run "HELIOGRAPH_ASSUME_PRIVILEGED=$v" -- ./steps/act.ps1
  # An action with no CONFIRM must refuse whatever this variable says. If it
  # ever exits 0, the variable has opened a gate rather than closed one.
  [ "$RC" = "0" ] && opened="$opened [$v]"
done
if [ -z "$opened" ]; then
  t_ok "no value of HELIOGRAPH_ASSUME_PRIVILEGED opens any gate"
else
  t_no "HELIOGRAPH_ASSUME_PRIVILEGED opened a gate for:$opened"
  printf '     A test seam that can permit a run is a backdoor with a test name.\n'
fi

# --- not a gate: an unknown step is exit 2 -----------------------------------
# Kept apart from the gates deliberately. "I do not know what you asked for" and
# "I know and I refuse" are different answers and the loop reports them
# differently.
run -- nosuchstep
assert_eq "an unknown step is exit 2, not a gate refusal" "2" "$RC"
assert_contains "and points at --list" "--list" "$ERR"

printf 'env = steps/missing.ps1\n' > /dev/null   # the table entry exists; the file does not
rm -f "$WORK/steps/env-snapshot.ps1"
run -- env
assert_eq "a registered step whose file is missing is exit 2" "2" "$RC"
assert_contains "and names the file it looked for" "env-snapshot.ps1" "$ERR"

# --- --mode and --file touch nothing -----------------------------------------
before="$(logs)"
run -- --mode ./steps/act.ps1
assert_eq "--mode exits 0" "0" "$RC"
assert_contains "--mode answers what the step declares" "action" "$OUT"

run -- --mode ./steps/undeclared.ps1
assert_contains "--mode says 'undeclared' rather than nothing at all" "undeclared" "$OUT"

run -- --file ./steps/ok.ps1
assert_eq "--file exits 0" "0" "$RC"
assert_contains "--file answers which file the declaration came from" "ok.ps1" "$OUT"

assert_eq "and neither wrote a log, because station.ps1 asks about every request" \
  "$before" "$(logs)"

# --- the exit code of the step itself survives -------------------------------
printf '# heliograph-mode: read-only\nWrite-Output "failing"\nexit 7\n' > "$WORK/steps/fail.ps1"
run -- ./steps/fail.ps1
assert_eq "a failed step's own exit code reaches the caller" "7" "$RC"
last="$(find "$WORK/ops-logs" -name 'fail-*.txt' 2>/dev/null | tail -1)"
if [ -n "$last" ]; then
  assert_contains "and the log says FAILED, not OK" "RESULT       : FAILED" "$(cat "$last")"
else
  t_no "a failed step wrote no log at all"
fi

# --- THE SHIPPED STEP ACTUALLY RUNS ------------------------------------------
# The default step must work, or `.\run.ps1` with no arguments - the one command
# an operator is asked to remember - fails on a fresh payload.
#
# AND IT MUST REPORT ZERO FAILED PROBES on the platform it is run on. That is
# the assertion, not "it exited 0": three defects hid behind an exit code here.
# `$LASTEXITCODE` is set only by a native call, so probes that ran nothing
# native inherited the previous probe's code and reported failures they had
# nothing to do with. `WindowsIdentity::GetCurrent()` threw everywhere but
# Windows. And `exit (Write-ProbeSummary)` captured the tally into the
# parentheses instead of printing it, so the log ended with no summary at all -
# which reads exactly like a step that was cut off.
SHIPPED="$WORK/shipped"
mkdir -p "$SHIPPED"
cp -r "$PSDIR/." "$SHIPPED/"
step_out="$( cd "$SHIPPED" && PUSH=0 "$PS_BIN" -NoProfile -File ./run.ps1 env 2>&1 )"
step_rc=$?
assert_eq "the shipped default step runs, exit 0" "0" "$step_rc"
assert_contains "and prints its summary, which is the last thing a reader looks for" \
  "probes, " "$step_out"
tally="$(printf '%s' "$step_out" | sed -n 's/.*| \([0-9]*\) probes, \([0-9]*\) failed.*/\1 \2/p' | tail -1)"
case "$tally" in
  *" 0") t_ok "and every required probe passed here: $tally" ;;
  "")    t_no "the step printed no tally, so nothing above was measured" ;;
  *)     t_no "the shipped step has failing probes on this platform: $tally"
         printf '%s\n' "$step_out" | grep -A 2 'FAILED:' | sed 's/^/     /' | head -12 ;;
esac
assert_contains "and it asks the question that decides whether a station can run here" \
  "LanguageMode" "$step_out"

# --- THE TWINS AGREE, case by case -------------------------------------------
# Every divergence above was found by hand, one spelling at a time, and two of
# them were in a security gate: `CONFIRM=YES` ran a state-changing step here and
# was refused by run.sh, and `# heliograph-mode: READ-ONLY` did the same. Both
# because PowerShell compares case-insensitively and bash does not.
#
# Finding those by hand does not scale and does not stay found. So the same
# DECLARATION goes through both runners and the exit codes must match. The body
# of each step differs - one is bash, one is PowerShell - but the header is the
# thing under test, and it is byte-identical.
BASHREPO="$WORK/bashrepo"
if "$HERE/../station/bootstrap.sh" "$BASHREPO" >/dev/null 2>&1; then

  # case <name> <header> <env...> - runs it both ways, compares the codes
  twin() {
    local name="$1" header="$2"; shift 2
    local envs=("$@") brc prc

    printf '#!/usr/bin/env bash\n%s\necho ran\n' "$header" > "$BASHREPO/steps/twin.sh"
    chmod +x "$BASHREPO/steps/twin.sh"
    printf '%s\nWrite-Output "ran"\n' "$header" > "$WORK/steps/twin.ps1"

    ( cd "$BASHREPO" && env "${envs[@]+"${envs[@]}"}" PUSH=0 \
        ./run.sh ./steps/twin.sh ) >/dev/null 2>&1
    brc=$?
    ( cd "$WORK" && env "${envs[@]+"${envs[@]}"}" PUSH=0 \
        "$PS_BIN" -NoProfile -File ./run.ps1 ./steps/twin.ps1 ) >/dev/null 2>&1
    prc=$?

    if [ "$brc" = "$prc" ]; then
      t_ok "twins agree on $name: both exit $brc"
    else
      t_no "twins DISAGREE on $name: run.sh exits $brc, run.ps1 exits $prc"
      printf '     The same declaration means two different things to two readers.\n'
    fi
  }

  twin "a read-only step"              "# heliograph-mode: read-only"
  twin "no declaration at all"         "# nothing declared here"
  twin "an unrecognised mode"          "# heliograph-mode: banana"
  twin "a mode in capitals"            "# heliograph-mode: READ-ONLY"
  twin "a header in capitals"          "# HELIOGRAPH-MODE: read-only"
  twin "a mode with odd spacing"       "#heliograph-mode:read-only"
  twin "an action with no CONFIRM"     "# heliograph-mode: action"
  twin "an action with CONFIRM=yes"    "# heliograph-mode: action" CONFIRM=yes
  twin "an action with CONFIRM=YES"    "# heliograph-mode: action" CONFIRM=YES
  twin "an action with CONFIRM=1"      "# heliograph-mode: action" CONFIRM=1
  twin "an action with CONFIRM=true"   "# heliograph-mode: action" CONFIRM=true
else
  t_no "could not bootstrap a bash payload, so the twins were NOT compared"
fi

t_summary
