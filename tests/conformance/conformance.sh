#!/usr/bin/env bash
# =============================================================================
#  conformance.sh - the executable form of the capture contract
# =============================================================================
# AGENTS.md constraint 3 says there is one implementation of the capture
# pattern. That is becoming one *specification* with several implementations:
# bash today, PowerShell after A8, and the same properties across every
# transport after A6. This file is that specification.
#
# It asserts properties, never internals. A driver supplies the implementation.
# Nothing here may reference caplib.sh, run.sh or any path inside the toolkit:
# the moment it does, it stops being a specification and becomes a second copy
# of one implementation.
#
# Usage: conformance.sh <driver.sh>
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../assert.sh disable=SC1091
. "$HERE/../assert.sh"

DRIVER="${1:?usage: conformance.sh <driver.sh>}"
[ -f "$DRIVER" ] || { printf 'no such driver: %s\n' "$DRIVER" >&2; exit 2; }
# shellcheck disable=SC1090
. "$DRIVER"

WORK="$(mktemp -d)"
# The driver may have started a far side that is a PROCESS - the relay's is -
# and several properties bootstrap, so several get started. Torn down from the
# trap rather than after the last property, because a suite that fails at p5
# would otherwise leave every one of them running with its port held.
_conf_cleanup() {
  if declare -F drv_teardown >/dev/null 2>&1; then drv_teardown; fi
  rm -rf "$WORK"
}
trap _conf_cleanup EXIT

printf '\n--- conformance: %s ---\n' "$(drv_name)"

# Timestamps are HH:MM:SS. Converted to seconds since midnight so gaps can be
# measured. Midnight rollover is handled by the caller adding 86400 when the
# second value is smaller than the first.
secs_of() {
  local t="$1" h m s
  h="${t%%:*}"; t="${t#*:}"; m="${t%%:*}"; s="${t#*:}"
  # 10# forces base 10, or 08 and 09 are read as invalid octal.
  printf '%s' "$(( 10#$h * 3600 + 10#$m * 60 + 10#$s ))"
}

# The timestamp column of a captured log: the field before the first " | ".
# Header and footer lines carry no such separator and are skipped.
stamps_of() { grep ' | ' "$1" | sed 's/ | .*//'; }

# --- property 1: every captured line carries a DISTINCT UTC timestamp ---------
# This is the busybox failure, and it is the reason the whole suite exists. A
# `sed` without -u buffers, every line of the block is stamped when the buffer
# flushes, and the log reads perfectly while being useless: a hang and slow
# progress become indistinguishable, which is the single property these logs
# exist for.
if drv_supports capture; then
  cat > "$WORK/three-slow.sh" <<'EOS'
#!/usr/bin/env bash
echo first; sleep 1.1; echo second; sleep 1.1; echo third
EOS
  chmod +x "$WORK/three-slow.sh"
  drv_capture "$WORK/p1.log" "$WORK/three-slow.sh" >/dev/null 2>&1

  p1_all="$(stamps_of "$WORK/p1.log" | wc -l | tr -d ' ')"
  p1_uniq="$(stamps_of "$WORK/p1.log" | sort -u | wc -l | tr -d ' ')"

  assert_eq "p1: three captured lines" "3" "$p1_all"
  assert_eq "p1: three DISTINCT timestamps, so the capture is unbuffered" \
    "3" "$p1_uniq"

  # DISTINCT IS NOT ENOUGH, and the property says "timestamp" for a reason.
  # `foo`, `bar` and `baz` are three distinct prefixes and would have passed
  # both assertions above. The column has to be a clock, and it has to be one
  # a reader can compare with their own: a log stamped in local time is read
  # against the wrong hour by everybody who did not run it.
  p1_shape="$(stamps_of "$WORK/p1.log" | grep -cE '^[0-2][0-9]:[0-5][0-9]:[0-5][0-9]$')"
  assert_eq "p1: and all three are HH:MM:SS, not merely three different strings" \
    "3" "$p1_shape"

  # UTC, asked by running the capture in a timezone that is NOT UTC and
  # checking the hour against one taken here. Anything reading local time comes
  # out hours away and fails; a capture that ignores TZ agrees.
  TZ="Pacific/Kiritimati" drv_capture "$WORK/p1tz.log" "$WORK/three-slow.sh" >/dev/null 2>&1
  p1_hour="$(stamps_of "$WORK/p1tz.log" | head -1 | cut -d: -f1)"
  p1_utc_hour="$(date -u +%H)"
  if [ "$p1_hour" = "$p1_utc_hour" ]; then
    t_ok "p1: the stamps are UTC even when TZ says otherwise"
  else
    # An hour boundary crossed between the capture and this line is the only
    # innocent way the two differ, and it is worth one retry rather than a
    # flake somebody re-runs.
    p1_utc_hour="$(date -u +%H)"
    assert_eq "p1: the stamps are UTC even when TZ says otherwise" \
      "$p1_utc_hour" "$p1_hour"
  fi
else
  t_skip "p1: driver does not support capture"
fi

# --- property 2: a real gap in the source appears as a gap in the log --------
# Distinct timestamps are not enough. Stamps could be distinct and still wrong,
# by being applied when a line is read from a buffer rather than when it was
# produced. A hang shows up as a gap in the timestamp column or it does not
# show up at all, so the gap has to be measured, not assumed.
if drv_supports capture; then
  cat > "$WORK/gap.sh" <<'EOS'
#!/usr/bin/env bash
echo before; sleep 3; echo after
EOS
  chmod +x "$WORK/gap.sh"
  drv_capture "$WORK/p2.log" "$WORK/gap.sh" >/dev/null 2>&1

  p2_first="$(stamps_of "$WORK/p2.log" | sed -n 1p)"
  p2_last="$(stamps_of "$WORK/p2.log" | sed -n 2p)"

  if [ -z "$p2_first" ] || [ -z "$p2_last" ]; then
    t_no "p2: expected two stamped lines, got [$p2_first] [$p2_last]"
  else
    p2_a="$(secs_of "$p2_first")"
    p2_b="$(secs_of "$p2_last")"
    # Midnight rollover: the second stamp is smaller only if the day turned.
    [ "$p2_b" -lt "$p2_a" ] && p2_b=$(( p2_b + 86400 ))
    p2_delta=$(( p2_b - p2_a ))

    # The upper bound is 8 rather than 4 because CI runners are slow and a
    # false failure here is worse than a loose bound. A buffered capture
    # yields 0, which the lower bound catches.
    if [ "$p2_delta" -ge 3 ] && [ "$p2_delta" -le 8 ]; then
      t_ok "p2: a 3s gap in the source is a ${p2_delta}s gap in the log"
    else
      t_no "p2: a 3s gap in the source became ${p2_delta}s in the log"
      printf '     wanted 3..8, log was:\n'; sed 's/^/     /' "$WORK/p2.log"
    fi
  fi
else
  t_skip "p2: driver does not support capture"
fi

# --- property 3: the step's real exit code survives the pipeline -------------
# The capture is a pipeline, so $? is the exit of the last stage (tee) and is
# almost always 0. PIPESTATUS[0] is what carries the truth. Getting this wrong
# publishes every failed run as a success, which is the most expensive possible
# defect here: a wasted round trip through someone who cannot debug the machine.
if drv_supports capture; then
  cat > "$WORK/rc42.sh" <<'EOS'
#!/usr/bin/env bash
echo working
exit 42
EOS
  chmod +x "$WORK/rc42.sh"
  drv_capture "$WORK/p3.log" "$WORK/rc42.sh" >/dev/null 2>&1
  assert_eq "p3: exit 42 survives the capture pipeline" "42" "$?"

  cat > "$WORK/rc0.sh" <<'EOS'
#!/usr/bin/env bash
echo working
EOS
  chmod +x "$WORK/rc0.sh"
  drv_capture "$WORK/p3ok.log" "$WORK/rc0.sh" >/dev/null 2>&1
  assert_eq "p3: exit 0 is still 0, so the check is not stuck on failure" \
    "0" "$?"
else
  t_skip "p3: driver does not support capture"
fi

# --- property 4: a failed run still writes a complete log --------------------
# AGENTS.md constraint 2. Each round trip through an operator is expensive and
# none may be wasted by tooling that only reports success. A failed run has to
# read as clearly as a passing one: same footer, real exit code, RESULT FAILED.
if drv_supports capture; then
  drv_capture "$WORK/p4.log" "$WORK/rc42.sh" >/dev/null 2>&1
  p4_body="$(cat "$WORK/p4.log" 2>/dev/null)"

  assert_contains "p4: a failed run still writes the footer" \
    "finished UTC" "$p4_body"
  assert_contains "p4: the footer carries the real exit code" \
    "exit code    : 42" "$p4_body"
  assert_contains "p4: the failure is named, not implied" \
    "RESULT       : FAILED" "$p4_body"
  assert_contains "p4: the output produced before the failure is kept" \
    "working" "$p4_body"

  drv_capture "$WORK/p4ok.log" "$WORK/rc0.sh" >/dev/null 2>&1
  assert_contains "p4: a passing run says OK, so RESULT is not hardcoded" \
    "RESULT       : OK" "$(cat "$WORK/p4ok.log" 2>/dev/null)"
else
  t_skip "p4: driver does not support capture"
fi

# --- properties 5 and 6: the gates fail closed -------------------------------
# A step that declares nothing does not run at all, and nothing runs as root.
# Both are checked here rather than only in test-step-mode.sh and
# test-root-refusal.sh because they must hold for EVERY implementation and
# every transport, not only for the one those tests happen to exercise.
#
# The steps are given to the runner BY PATH rather than registered in its case
# table. The path form exercises the same gate, and patching a case table from
# a test would couple this suite to one implementation's internals, which is
# exactly what the driver split exists to prevent.
if drv_supports gates; then
  P5="$WORK/repo"
  if drv_bootstrap "$P5"; then

    cat > "$P5/steps/undeclared.sh" <<'EOS'
#!/usr/bin/env bash
echo this step declares nothing
EOS
    # A MARKER, because an exit code is not evidence that anything ran. Both
    # gates are asserted by their exit status alone, and a runner that returned
    # 0 without executing the step - or one that executed it and THEN refused
    # with 5 - would satisfy every assertion below while violating the property
    # outright. The marker is what tells those apart.
    cat > "$P5/steps/declared.sh" <<EOS
#!/usr/bin/env bash
# heliograph-mode: read-only
echo this step declares itself and measures nothing
: > "$WORK/p5-declared-ran"
EOS
    chmod +x "$P5/steps/undeclared.sh" "$P5/steps/declared.sh"

    p5_before="$(find "$P5/ops-logs" -name '*.txt' 2>/dev/null | wc -l | tr -d ' ')"
    drv_step "$P5" steps/undeclared.sh
    assert_eq "p5: an undeclared step exits 3" "3" "$?"
    p5_after="$(find "$P5/ops-logs" -name '*.txt' 2>/dev/null | wc -l | tr -d ' ')"
    assert_eq "p5: an undeclared step writes no log at all" \
      "$p5_before" "$p5_after"

    # The control: the same runner, the same path form, a step that DOES
    # declare itself. Without this, p5 would pass just as well if the runner
    # refused everything.
    rm -f "$WORK/p5-declared-ran"
    drv_step "$P5" steps/declared.sh
    assert_eq "p5: a declared step runs, so the gate is not refusing everything" \
      "0" "$?"
    if [ -f "$WORK/p5-declared-ran" ]; then
      t_ok "p5: and it actually EXECUTED, rather than exiting 0 without running"
    else
      t_no "p5: exit 0, but the step never ran - the gate is passing without executing"
    fi

    # THE PRIVILEGED ACCOUNT IS THE DRIVER'S TO SIMULATE.
    #
    # This used to build a fake `id` returning 0 and put it first on PATH, and
    # then call `./run.sh` directly - which is Unix spelled twice over, and a
    # reference to one implementation's runner in a file whose whole job is to
    # avoid that. Windows has no `id` and no root; it has SYSTEM, S-1-5-18, and
    # an Administrator role, and how you pretend to be one is a property of the
    # platform rather than of the capture.
    #
    # So the suite asks the question - "refuse the privileged account with 5" -
    # and the driver answers it however its platform allows. A driver that
    # cannot simulate one says so and p6 skips, loudly, rather than passing.
    rm -f "$WORK/p5-declared-ran"
    if declare -F drv_step_privileged >/dev/null 2>&1; then
      drv_step_privileged "$P5" steps/declared.sh
      assert_eq "p6: the same step refuses with 5 when the account is privileged" \
        "5" "$?"
      # THE REFUSAL HAS TO PRECEDE THE STEP. Exit 5 after running it is the
      # whole defect wearing the right exit code: the destructive thing has
      # already happened with privilege by the time anybody reads the number.
      if [ -f "$WORK/p5-declared-ran" ]; then
        t_no "p6: it exited 5, but the step RAN PRIVILEGED first - the refusal is too late"
      else
        t_ok "p6: and the step did not run at all, so the refusal precedes it"
      fi
    else
      t_skip "p6: this driver cannot simulate a privileged account, so the root gate is UNCHECKED here"
    fi
  else
    t_skip "p5/p6: could not bootstrap a transport repo"
  fi
else
  t_skip "p5/p6: driver does not support gates"
fi

# --- property 7: redaction is wired INTO the capture -------------------------
# test-redact.sh already exercises cap_redact directly and in more depth. This
# asserts something different and weaker on purpose: that redaction is actually
# on the capture path of this implementation. A correct redactor that a second
# implementation forgets to call is exactly the drift this suite exists for,
# and a unit test of the function cannot see it.
#
# Over-masking is asserted too. These logs are the only evidence anybody gets,
# and a redactor that eats ordinary output is its own failure.
CORPUS="$HERE/../fixtures/redaction-corpus.txt"

# The corpus writes a PEM header as %%PEM_BEGIN%% because CI refuses that
# literal anywhere in the repository, with no allow list - see the note in the
# corpus. Expanded here, so the rule is tested against the exact string it
# matches. Assembled rather than written out, or this line would trip the gate.
PEM_BEGIN="-----BEGIN RSA PRIVATE$(printf ' ')KEY-----"
corpus_expand() { printf '%s' "${1//\%\%PEM_BEGIN\%\%/$PEM_BEGIN}"; }

if ! drv_supports capture; then
  t_skip "p7: driver does not support capture"
elif [ ! -r "$CORPUS" ]; then
  t_no "p7: the redaction corpus is missing from $CORPUS"
else
  # THE CORPUS IS EMITTED THROUGH THE IMPLEMENTATION'S OWN CAPTURE PATH, once,
  # and the whole log is then asserted against. Emitting it line by line would
  # be a different and weaker test: several rules are anchored to what else is
  # on the line, and a redactor applied per-line by the harness rather than by
  # the implementation is not the thing under test.
  {
    printf '#!/usr/bin/env bash\n'
    while IFS="$(printf '\t')" read -r verdict needle line; do
      case "$verdict" in
        MASK | KEEP) ;;
        *) continue ;;
      esac
      [ -n "$line" ] || continue
      line="$(corpus_expand "$line")"
      # printf %s with the line as an ARGUMENT, never as the format: a corpus
      # line contains % and \ by design, and putting it in the format string
      # would let the fixture rewrite itself on the way through.
      printf 'printf "%%s\\n" %s\n' "$(printf "'%s'" "$(printf '%s' "$line" | sed "s/'/'\\\\''/g")")"
    done < "$CORPUS"
  } > "$WORK/leaky.sh"
  chmod +x "$WORK/leaky.sh"
  drv_capture "$WORK/p7.log" "$WORK/leaky.sh" >/dev/null 2>&1
  p7_body="$(cat "$WORK/p7.log" 2>/dev/null)"

  p7_leaked=0 p7_eaten=0 p7_masks=0 p7_keeps=0
  while IFS="$(printf '\t')" read -r verdict needle line; do
    [ -n "$needle" ] || continue
    needle="$(corpus_expand "$needle")"
    case "$verdict" in
      MASK)
        p7_masks=$((p7_masks + 1))
        case "$p7_body" in
          *"$needle"*) p7_leaked=$((p7_leaked + 1)); printf '     LEAKED: %s\n' "$line" ;;
        esac
        ;;
      KEEP)
        p7_keeps=$((p7_keeps + 1))
        case "$p7_body" in
          *"$needle"*) ;;
          *) p7_eaten=$((p7_eaten + 1)); printf '     EATEN: %s\n' "$line" ;;
        esac
        ;;
    esac
  done < "$CORPUS"

  # The counts are asserted too. A corpus that failed to parse - a tab turned
  # into spaces by an editor is all it takes - would give zero of each and
  # report two clean passes over nothing at all.
  if [ "$p7_masks" -ge 15 ] && [ "$p7_keeps" -ge 5 ]; then
    t_ok "p7: the corpus parsed - $p7_masks secrets and $p7_keeps must-keep lines"
  else
    t_no "p7: the corpus barely parsed: $p7_masks MASK and $p7_keeps KEEP records"
    printf '     A tab turned into spaces is enough. This assertion is what stops\n'
    printf '     an unparsed corpus reporting a clean run over nothing.\n'
  fi
  assert_eq "p7: no secret in the corpus survives the capture" "0" "$p7_leaked"
  assert_eq "p7: and no ordinary output is eaten, so evidence survives" "0" "$p7_eaten"
fi

# --- property 8: a cancelled run keeps what it captured ----------------------
# A log that stops mid-sentence is still evidence, and usually the evidence you
# wanted: the last line names the probe that was in flight. Discarding a
# partial log on cancel would throw away the only thing an hour-long wrong run
# produced.
#
# The exit-130 half of this property belongs to the station loop, which
# publishes state `cancelled` with exit 130. That needs a transport driver to
# observe a published status and is asserted in A6. This is the half that
# belongs to the capture.
# WHETHER A CANCEL CAN KEEP ANYTHING IS THE DRIVER'S TO ANSWER.
#
# This used to probe `sed -u` here, which is the bash implementation's business
# spelled in the specification. The reason is real and worth keeping - cap_run
# stamps each line before any sed runs, so the TIMESTAMPS are honest either
# way, but redaction is a sed stage and cannot be skipped or reordered, because
# publishing unredacted output to a log that gets committed is not a trade
# available at any price. So where sed buffers, a run killed mid-flight loses
# whatever was in that buffer.
#
# That is a fact about busybox, not about the property. The driver reports it
# through `drv_supports cancel`, and a station that cannot do this skips loudly
# - somebody choosing that platform should know which half they are giving up.
if drv_supports cancel; then
  cat > "$WORK/slow.sh" <<'EOS'
#!/usr/bin/env bash
echo starting the long probe
for i in 1 2 3 4 5 6 7 8 9 10; do echo "probe $i"; sleep 1; done
echo finished
EOS
  chmod +x "$WORK/slow.sh"

  # A PIDFILE, not a captured stdout.
  #
  # `pid="$(drv_capture_bg ...)"` starts the driver in a command substitution,
  # so the background process is a child of a SUBSHELL this one cannot `wait`
  # for. Both kill attempts could fail and the suite would carry on after a
  # fixed sleep, leaving a run that ignored cancellation alive and still
  # writing into a directory about to be deleted. It also cannot work on
  # Windows, where the thing to kill is a job object rather than a pid.
  #
  # So the driver writes an identifier to a file and takes it back to cancel.
  # What is in that file is the driver's business; the suite only hands it over.
  P8_HANDLE="$WORK/p8.handle"
  drv_capture_bg "$WORK/p8.log" "$WORK/slow.sh" "$P8_HANDLE"
  sleep 3

  drv_cancel "$P8_HANDLE"
  p8_reported=$?

  # THE LOG MUST STOP GROWING, and this is asked of the FILE rather than of the
  # driver. A driver that returns 0 without cancelling anything passes every
  # other assertion here: three seconds into a ten-second step the log is
  # partial and has no `finished` line, exactly as a cancelled one would be.
  # The two are told apart by what happens next - a cancelled run's log is
  # frozen and a running one is not - and that is observable without trusting
  # the driver's word for anything.
  p8_size1="$(wc -c < "$WORK/p8.log" 2>/dev/null || echo 0)"
  sleep 3
  p8_size2="$(wc -c < "$WORK/p8.log" 2>/dev/null || echo 0)"
  if [ "$p8_size1" = "$p8_size2" ]; then
    t_ok "p8: the log stopped growing, so the run is genuinely gone"
  else
    t_no "p8: the log grew from $p8_size1 to $p8_size2 bytes AFTER the cancel"
    printf '     The run is still going. Every assertion below would have passed\n'
    printf '     anyway, on a log that is partial only because it is unfinished.\n'
  fi
  if [ "$p8_reported" = "0" ]; then
    t_ok "p8: and the driver reported the cancel delivered"
  else
    t_no "p8: the driver could not confirm the run was cancelled"
  fi

  p8_body="$(cat "$WORK/p8.log" 2>/dev/null)"
  assert_contains "p8: the partial log survives a cancel" \
    "starting the long probe" "$p8_body"
  assert_eq "p8: the run did NOT reach its end, so this was a real cancel" "0" \
    "$(printf '%s\n' "$p8_body" | grep -c '| finished$')"

  p8_lines="$(printf '%s\n' "$p8_body" | grep -c ' | ')"
  if [ "$p8_lines" -ge 2 ]; then
    t_ok "p8: the partial log holds the ${p8_lines} lines captured before the cancel"
  else
    t_no "p8: expected at least 2 captured lines, got $p8_lines"
  fi
else
  t_skip "p8: this driver cannot cancel and keep a partial log (timestamps are unaffected)"
fi

# --- property 9: the FINISHED log reaches the far side -----------------------
# The property whose absence let a real defect ship, which is the best argument
# for it existing.
#
# Properties 1 to 4 all assert things about the log FILE. Every one of them
# passed on every transport, because the file is written correctly to local disk
# every single time. None of them asks the only question that matters to the
# person the log is for: did it arrive.
#
# It had not, on two transports out of three. `run.sh` finished with `cap_push`,
# which is git unconditionally, so a station on the relay or the blob transport
# captured a perfect log and delivered nothing - no footer, no exit code, no
# RESULT line, and with PROGRESS_EVERY=0 not a single byte. AGENTS.md constraint
# 2 says a failed run still ships and a failed push never loses a log; the first
# half was quietly untrue wherever git was not the channel.
#
# So this asserts DELIVERY, from the receiving end, and it deliberately does not
# care how: the driver hands back whatever the far side would have. A driver
# that cannot observe the far side skips, and says so, rather than passing.
#
# The footer is what is looked for, not merely the log's existence. A partial
# log delivered by a progress snapshot would satisfy "something arrived" while
# still leaving the reader unable to tell a finished run from a hung one, which
# is the same failure wearing a different hat.
if drv_supports deliver; then
  P9="$WORK/deliver"
  if drv_bootstrap "$P9"; then
    cat > "$P9/steps/ships.sh" <<'EOS'
#!/usr/bin/env bash
# heliograph-mode: read-only
echo the evidence
exit 7
EOS
    chmod +x "$P9/steps/ships.sh"

    drv_deliver "$P9" steps/ships.sh
    p9_body="$(drv_delivered "$P9")"

    assert_contains "p9: the delivered log carries the output" \
      "the evidence" "$p9_body"
    assert_contains "p9: the delivered log carries the footer, so it is COMPLETE" \
      "finished UTC" "$p9_body"
    assert_contains "p9: the delivered log carries the real exit code" \
      "exit code    : 7" "$p9_body"
    assert_contains "p9: a failed run is delivered too, and says so" \
      "RESULT       : FAILED" "$p9_body"
  else
    t_skip "p9: could not bootstrap a transport repo"
  fi
else
  t_skip "p9: driver cannot observe the far side, so delivery is unchecked here"
fi

t_summary
