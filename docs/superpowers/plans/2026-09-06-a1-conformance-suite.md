# A1 - conformance suite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an executable specification of the heliograph capture contract, parameterised by implementation, that passes against the current bash toolkit and demonstrably fails against a deliberately broken one.

**Architecture:** A driver contract (`drv_*` functions) separates *what* is asserted from *which implementation* is under test. `tests/conformance/conformance.sh` holds the eight properties and nothing implementation-specific. `drivers/bash.sh` wires it to today's `caplib.sh` and `run.sh`. `drivers/mutant.sh` is a deliberately broken implementation whose only job is to fail, proving the suite has teeth. Later work adds `drivers/powershell.sh` (A8) and per-transport drivers (A6) with no change to the properties.

**Tech Stack:** Plain bash 4+, the repo's existing `tests/assert.sh`. No framework, no packages.

## Global Constraints

- Bash 4+, git, GNU coreutils only. No packages, no interpreter beyond bash, no credentials
- Toolkit and test scripts use `set -uo pipefail`, never `set -e`
- House style: British English, plain hyphens, **no em dashes**, no trailing full stops on headings
- Every new test file is `tests/test-*.sh` or lives under `tests/conformance/`, and is picked up by `tests/run-tests.sh`
- A skip must go through `t_skip` so it is uppercase and counted. CI refuses a non-zero skip count
- **No behaviour change to anything under `skills/heliograph/toolkit/`.** A1 adds tests only. If a property fails against the real driver, that is a finding to report, not a licence to edit the toolkit
- Validation before every commit: `bash -n` on changed scripts, then `shellcheck -S warning`, then `./tests/run-tests.sh`

---

### Task 1: The driver contract, the harness, and the first property

**Files:**
- Create: `tests/conformance/conformance.sh`
- Create: `tests/conformance/drivers/bash.sh`
- Create: `tests/test-conformance.sh`

**Interfaces:**
- Consumes: `tests/assert.sh` (`t_ok`, `t_no`, `t_skip`, `assert_eq`, `assert_contains`, `t_summary`)
- Produces: the driver contract every later task and A8 depends on:
  - `drv_name` - echoes a human name for this implementation
  - `drv_supports <cap>` - exit 0 if supported. Capabilities: `capture`, `gates`, `cancel`
  - `drv_capture <out> <script>` - capture `<script>` to log file `<out>`, return the script's real exit code
  - `drv_bootstrap <dir>` - create a working transport repo at `<dir>`
  - `drv_step <dir> <step>` - run registered step `<step>` in repo `<dir>`, return the runner's exit code

- [ ] **Step 1: Write the harness and the first property**

Create `tests/conformance/conformance.sh`:

```bash
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
trap 'rm -rf "$WORK"' EXIT

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
else
  t_skip "p1: driver does not support capture"
fi

t_summary
```

Create `tests/conformance/drivers/bash.sh`:

```bash
#!/usr/bin/env bash
# =============================================================================
#  drivers/bash.sh - the current bash toolkit, under the conformance contract
# =============================================================================
# Sourced by conformance.sh. Implements drv_* and nothing else. All knowledge
# of caplib.sh, run.sh and bootstrap.sh lives here, so the suite itself stays a
# specification rather than a second copy of this implementation.
# =============================================================================

_D_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_D_TOOLKIT="$(cd "$_D_HERE/../../../skills/heliograph/toolkit" && pwd)"
_D_BOOTSTRAP="$(cd "$_D_HERE/../../../skills/heliograph/scripts" && pwd)/bootstrap.sh"

drv_name() { printf 'bash toolkit (caplib.sh, run.sh)'; }

drv_supports() {
  case "$1" in
    capture|gates|cancel) return 0 ;;
    *) return 1 ;;
  esac
}

# Capture in a subshell so caplib's globals never leak between properties.
drv_capture() {
  local out="$1" script="$2"
  (
    # shellcheck disable=SC1091
    . "$_D_TOOLKIT/caplib.sh"
    cap_header "$out" "conformance"
    cap_run "$out" "$script"
    rc=$?
    cap_footer "$out" "$rc"
    exit "$rc"
  ) >/dev/null 2>&1
}

drv_bootstrap() {
  local dir="$1"
  "$_D_BOOTSTRAP" "$dir" >/dev/null 2>&1 || return 1
  (
    cd "$dir" || exit 1
    git init -q .
    git -c user.email=ci@example.invalid -c user.name=ci add -A
    git -c user.email=ci@example.invalid -c user.name=ci commit -qm init
  ) >/dev/null 2>&1
}

drv_step() {
  local dir="$1" step="$2"
  ( cd "$dir" && PUSH=0 ./run.sh "$step" ) >/dev/null 2>&1
}
```

Create `tests/test-conformance.sh`:

```bash
#!/usr/bin/env bash
# =============================================================================
#  test-conformance.sh - run the conformance suite against every driver
# =============================================================================
# The real driver must pass. The mutant driver must FAIL, and that assertion is
# the point: a conformance suite that cannot fail is decoration. The mutant is
# added in Task 8; until then only the real driver is run.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"

if "$HERE/conformance/conformance.sh" "$HERE/conformance/drivers/bash.sh"; then
  t_ok "the bash toolkit passes the conformance suite"
else
  t_no "the bash toolkit FAILS the conformance suite"
fi

t_summary
```

- [ ] **Step 2: Make them executable and run**

```bash
cd /home/devops/dbhq-heliograph
chmod +x tests/conformance/conformance.sh tests/test-conformance.sh
./tests/test-conformance.sh
```

Expected: `p1: three captured lines` and `p1: three DISTINCT timestamps` both `ok`, summary `2 passed, 0 failed, 0 skipped`, and the wrapper reports `ok the bash toolkit passes`.

- [ ] **Step 3: Prove the property has teeth**

Temporarily break the capture to confirm p1 can fail. Edit `tests/conformance/drivers/bash.sh`, replace the `cap_run` line in `drv_capture` with a buffered stand-in:

```bash
    # TEMPORARY - proving p1 fails when the capture buffers
    ts="$(date -u +%H:%M:%S)"
    "$script" 2>&1 | while IFS= read -r l; do printf '%s | %s\n' "$ts" "$l"; done >> "$out"
    rc=0
```

Run `./tests/test-conformance.sh`. Expected: `FAIL p1: three DISTINCT timestamps`, actual `1`.

**Then revert that edit** with `git checkout tests/conformance/drivers/bash.sh` if committed, or by restoring the original `cap_run` block. Re-run and confirm it passes again.

- [ ] **Step 4: Validate and commit**

```bash
cd /home/devops/dbhq-heliograph
bash -n tests/conformance/conformance.sh tests/conformance/drivers/bash.sh tests/test-conformance.sh
shellcheck -S warning tests/conformance/conformance.sh tests/conformance/drivers/bash.sh tests/test-conformance.sh
./tests/run-tests.sh
git add tests/conformance tests/test-conformance.sh
git commit -m "conformance: the contract, a driver for today's bash, and the property that started it

The busybox failure is the reason this exists: a capture that buffers
stamps every line of a block with the same time, which reads like a
working log while destroying the one thing the logs are for."
```

---

### Task 2: Property 2 - a gap in the source is a gap in the log

**Files:**
- Modify: `tests/conformance/conformance.sh` (append before `t_summary`)

**Interfaces:**
- Consumes: `drv_capture`, `drv_supports`, `secs_of`, `stamps_of` from Task 1
- Produces: nothing new

- [ ] **Step 1: Write the failing test**

Insert into `tests/conformance/conformance.sh` immediately before the `t_summary` line:

```bash
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
```

The upper bound is 8 rather than 4 because CI runners are slow and a false failure in this suite is worse than a loose bound. A buffered capture yields 0, which the bound catches.

- [ ] **Step 2: Run it**

```bash
cd /home/devops/dbhq-heliograph && ./tests/test-conformance.sh
```

Expected: `ok p2: a 3s gap in the source is a 3s gap in the log`.

- [ ] **Step 3: Validate and commit**

```bash
bash -n tests/conformance/conformance.sh
shellcheck -S warning tests/conformance/conformance.sh
./tests/run-tests.sh
git add tests/conformance/conformance.sh
git commit -m "conformance: a hang must read as a gap, so measure the gap"
```

---

### Task 3: Property 3 - the real exit code survives the pipeline

**Files:**
- Modify: `tests/conformance/conformance.sh` (append before `t_summary`)

**Interfaces:**
- Consumes: `drv_capture`, `drv_supports` from Task 1

- [ ] **Step 1: Write the failing test**

Insert immediately before the `t_summary` line:

```bash
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
```

- [ ] **Step 2: Run it**

```bash
cd /home/devops/dbhq-heliograph && ./tests/test-conformance.sh
```

Expected: both p3 assertions `ok`.

- [ ] **Step 3: Validate and commit**

```bash
bash -n tests/conformance/conformance.sh
shellcheck -S warning tests/conformance/conformance.sh
./tests/run-tests.sh
git add tests/conformance/conformance.sh
git commit -m "conformance: a failed run reported as a success is the expensive defect"
```

---

### Task 4: Property 4 - a failed run still writes its footer

**Files:**
- Modify: `tests/conformance/conformance.sh` (append before `t_summary`)

**Interfaces:**
- Consumes: `drv_capture`, `drv_supports` from Task 1

- [ ] **Step 1: Write the failing test**

Insert immediately before the `t_summary` line:

```bash
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
```

- [ ] **Step 2: Run it**

```bash
cd /home/devops/dbhq-heliograph && ./tests/test-conformance.sh
```

Expected: all five p4 assertions `ok`.

- [ ] **Step 3: Validate and commit**

```bash
bash -n tests/conformance/conformance.sh
shellcheck -S warning tests/conformance/conformance.sh
./tests/run-tests.sh
git add tests/conformance/conformance.sh
git commit -m "conformance: a failed run must read as clearly as a passing one"
```

---

### Task 5: Properties 5 and 6 - the gates fail closed

**Files:**
- Modify: `tests/conformance/conformance.sh` (append before `t_summary`)

**Interfaces:**
- Consumes: `drv_supports`, `drv_bootstrap`, `drv_step` from Task 1

- [ ] **Step 1: Write the failing test**

Insert immediately before the `t_summary` line:

```bash
# --- properties 5 and 6: the gates fail closed -------------------------------
# A step that declares nothing does not run at all, and nothing runs as root.
# Both are checked here rather than only in test-step-mode.sh and
# test-root-refusal.sh because they must hold for EVERY implementation and
# every transport, not only for the one those tests happen to exercise.
if drv_supports gates; then
  P5="$WORK/repo"
  if drv_bootstrap "$P5"; then

    # A step declaring no heliograph-mode at all.
    cat > "$P5/steps/undeclared.sh" <<'EOS'
#!/usr/bin/env bash
echo this step declares nothing
EOS
    chmod +x "$P5/steps/undeclared.sh"
    # Register it, so the refusal is about the declaration and not the step
    # table. sed inserts a case arm before the first existing one.
    sed -i 's|^\( *\)env)|\1undeclared) STEP_CMD=(./steps/undeclared.sh) ;;\n\1env)|' \
      "$P5/run.sh" 2>/dev/null

    before="$(find "$P5/ops-logs" -name '*.txt' 2>/dev/null | wc -l | tr -d ' ')"
    drv_step "$P5" undeclared
    assert_eq "p5: an undeclared step exits 3" "3" "$?"
    after="$(find "$P5/ops-logs" -name '*.txt' 2>/dev/null | wc -l | tr -d ' ')"
    assert_eq "p5: an undeclared step writes no log at all" "$before" "$after"

    # A fake `id` answering 0 to `id -u`, deferring to the real one otherwise.
    # caplib's cap_refuse_root calls `id -u` rather than reading $EUID
    # specifically so this is possible without being root.
    mkdir -p "$WORK/fakebin"
    real_id="$(command -v id)"
    cat > "$WORK/fakebin/id" <<EOF
#!/usr/bin/env bash
[ "\$*" = "-u" ] && { echo 0; exit 0; }
exec "$real_id" "\$@"
EOF
    chmod +x "$WORK/fakebin/id"

    ( cd "$P5" && PATH="$WORK/fakebin:$PATH" PUSH=0 ./run.sh env ) >/dev/null 2>&1
    assert_eq "p6: running as root exits 5" "5" "$?"
  else
    t_skip "p5/p6: could not bootstrap a transport repo"
  fi
else
  t_skip "p5/p6: driver does not support gates"
fi
```

- [ ] **Step 2: Run it**

```bash
cd /home/devops/dbhq-heliograph && ./tests/test-conformance.sh
```

Expected: three assertions `ok`. If `p5` reports exit 2 rather than 3, the `sed` registration did not match `run.sh`'s current case table. Inspect with `grep -n 'env)' tests-tmp-repo/run.sh` on a manual bootstrap and adjust the pattern; do **not** edit `run.sh` in the toolkit.

- [ ] **Step 3: Validate and commit**

```bash
bash -n tests/conformance/conformance.sh
shellcheck -S warning tests/conformance/conformance.sh
./tests/run-tests.sh
git add tests/conformance/conformance.sh
git commit -m "conformance: undeclared refuses, root refuses, for every implementation"
```

---

### Task 6: Property 7 - redaction masks each documented shape

**Files:**
- Modify: `tests/conformance/conformance.sh` (append before `t_summary`)

**Interfaces:**
- Consumes: `drv_capture`, `drv_supports` from Task 1

- [ ] **Step 1: Write the failing test**

Insert immediately before the `t_summary` line:

```bash
# --- property 7: redaction masks each documented shape -----------------------
# test-redact.sh already exercises cap_redact directly and in more depth. This
# asserts something different and weaker on purpose: that redaction is actually
# WIRED INTO the capture path of this implementation. A correct cap_redact that
# a second implementation forgets to call is exactly the drift this suite is
# for, and a unit test of the function cannot see it.
#
# Over-masking is asserted too. These logs are the only evidence anybody gets.
if drv_supports capture; then
  cat > "$WORK/leaky.sh" <<'EOS'
#!/usr/bin/env bash
echo "password=hunter2correct"
echo "Authorization: Bearer eyJleUJTRUNSRVQi"
echo "cloning https://ci-user:glpat-LEAKEDTOKENVALUE@git.invalid/x.git"
echo "listening on port 8443 and exit code 0"
EOS
  chmod +x "$WORK/leaky.sh"
  drv_capture "$WORK/p7.log" "$WORK/leaky.sh" >/dev/null 2>&1
  p7_body="$(cat "$WORK/p7.log" 2>/dev/null)"

  assert_eq "p7: a password= value does not survive the capture" "" \
    "$(printf '%s' "$p7_body" | grep -c 'hunter2correct')"
  assert_eq "p7: a Bearer token does not survive the capture" "" \
    "$(printf '%s' "$p7_body" | grep -c 'eyJleUJTRUNSRVQi')"
  assert_eq "p7: a credential in a URL does not survive the capture" "" \
    "$(printf '%s' "$p7_body" | grep -c 'glpat-LEAKEDTOKENVALUE')"
  assert_contains "p7: ordinary output is NOT masked, so evidence survives" \
    "listening on port 8443 and exit code 0" "$p7_body"
else
  t_skip "p7: driver does not support capture"
fi
```

Note the assertions compare `grep -c` output to the empty string rather than to `0`: `grep -c` in a command substitution that matches nothing exits non-zero and prints `0`, but the surrounding `$( )` keeps the `0`. Use `assert_eq ... "0"` if the first run reports `expected: [] actual: [0]`, and record which it was in the commit message.

- [ ] **Step 2: Run it and fix the grep comparison**

```bash
cd /home/devops/dbhq-heliograph && ./tests/test-conformance.sh
```

If the three masking assertions report `expected: [] actual: [0]`, change each expected value from `""` to `"0"` and re-run. Expected after that: all four p7 assertions `ok`.

- [ ] **Step 3: Validate and commit**

```bash
bash -n tests/conformance/conformance.sh
shellcheck -S warning tests/conformance/conformance.sh
./tests/run-tests.sh
git add tests/conformance/conformance.sh
git commit -m "conformance: redaction wired into the capture, not merely present"
```

---

### Task 7: Property 8 - a cancelled run keeps its partial log

**Files:**
- Modify: `tests/conformance/conformance.sh` (append before `t_summary`)
- Modify: `tests/conformance/drivers/bash.sh` (add `drv_capture_bg`)

**Interfaces:**
- Consumes: `drv_supports` from Task 1
- Produces: `drv_capture_bg <out> <script>` - start a capture in the background, echo its pid

- [ ] **Step 1: Add the background capture to the driver**

Append to `tests/conformance/drivers/bash.sh`:

```bash
# Start a capture in its own process group, so a cancel can signal the group
# the way station.sh does rather than only the wrapper.
drv_capture_bg() {
  local out="$1" script="$2"
  setsid bash -c '
    . "$1/caplib.sh"
    cap_header "$2" "conformance-cancel"
    cap_run "$2" "$3"
    cap_footer "$2" $?
  ' _ "$_D_TOOLKIT" "$out" "$script" >/dev/null 2>&1 &
  printf '%s' "$!"
}
```

- [ ] **Step 2: Write the failing test**

Insert into `tests/conformance/conformance.sh` immediately before the `t_summary` line:

```bash
# --- property 8: a cancelled run keeps what it captured ----------------------
# A log that stops mid-sentence is still evidence, and usually the evidence you
# wanted: the last line names the probe that was in flight. Discarding a
# partial log on cancel would throw away the only thing an hour-long wrong run
# produced.
if drv_supports cancel; then
  cat > "$WORK/slow.sh" <<'EOS'
#!/usr/bin/env bash
echo starting the long probe
for i in 1 2 3 4 5 6 7 8 9 10; do echo "probe $i"; sleep 1; done
echo finished
EOS
  chmod +x "$WORK/slow.sh"

  p8_pid="$(drv_capture_bg "$WORK/p8.log" "$WORK/slow.sh")"
  sleep 3
  kill -TERM -- "-$p8_pid" 2>/dev/null || kill -TERM "$p8_pid" 2>/dev/null
  sleep 2

  p8_body="$(cat "$WORK/p8.log" 2>/dev/null)"
  assert_contains "p8: the partial log survives a cancel" \
    "starting the long probe" "$p8_body"
  assert_eq "p8: the run did NOT reach its end, so this was a real cancel" "0" \
    "$(printf '%s' "$p8_body" | grep -c 'finished$')"

  p8_lines="$(printf '%s' "$p8_body" | grep -c ' | ')"
  if [ "$p8_lines" -ge 2 ]; then
    t_ok "p8: the partial log holds the ${p8_lines} lines captured before the cancel"
  else
    t_no "p8: expected at least 2 captured lines, got $p8_lines"
  fi
else
  t_skip "p8: driver does not support cancel"
fi
```

The exit-130 half of property 8 belongs to `station.sh`, which publishes `cancelled` with exit 130. That is asserted in A6 once a transport driver exists to observe a published status; this task asserts the half that belongs to the capture.

- [ ] **Step 3: Run it**

```bash
cd /home/devops/dbhq-heliograph && ./tests/test-conformance.sh
```

Expected: three p8 assertions `ok`.

- [ ] **Step 4: Validate and commit**

```bash
bash -n tests/conformance/conformance.sh tests/conformance/drivers/bash.sh
shellcheck -S warning tests/conformance/conformance.sh tests/conformance/drivers/bash.sh
./tests/run-tests.sh
git add tests/conformance
git commit -m "conformance: a log that stops mid-sentence is still the evidence you wanted"
```

---

### Task 8: The mutant driver, which proves the suite can fail

**Files:**
- Create: `tests/conformance/drivers/mutant.sh`
- Modify: `tests/test-conformance.sh`
- Modify: `AGENTS.md`

**Interfaces:**
- Consumes: the driver contract from Task 1
- Produces: nothing later depends on

- [ ] **Step 1: Write the mutant**

Create `tests/conformance/drivers/mutant.sh`:

```bash
#!/usr/bin/env bash
# =============================================================================
#  drivers/mutant.sh - a deliberately broken capture, which MUST fail
# =============================================================================
# A conformance suite that cannot fail is decoration. This driver breaks the
# three properties that matter most, in the exact ways a real port breaks them:
#
#   p1/p2  buffered: one timestamp taken up front and applied to every line,
#          which is what a `sed` without -u does. The log reads perfectly.
#   p3     the pipeline's exit code is reported instead of the step's, which is
#          what happens whenever PIPESTATUS is forgotten.
#   p4     no footer, so a failed run ends mid-air.
#
# test-conformance.sh asserts this driver FAILS. If it ever passes, the suite
# has stopped checking and the next real port will sail through it.
# =============================================================================

_M_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_M_TOOLKIT="$(cd "$_M_HERE/../../../skills/heliograph/toolkit" && pwd)"

drv_name() { printf 'MUTANT - deliberately broken, must fail'; }

drv_supports() {
  case "$1" in
    capture) return 0 ;;
    *) return 1 ;;
  esac
}

drv_capture() {
  local out="$1" script="$2" ts
  ts="$(date -u +%H:%M:%S)"
  {
    echo "============================================================"
    echo " mutant"
    echo "============================================================"
    echo
  } > "$out"
  # One stamp for every line, and $? from the pipeline rather than the step.
  "$script" 2>&1 | while IFS= read -r l; do printf '%s | %s\n' "$ts" "$l"; done >> "$out"
  return $?
}

drv_bootstrap() { return 1; }
drv_step() { return 1; }
drv_capture_bg() { return 1; }
```

- [ ] **Step 2: Assert the mutant fails**

Replace the body of `tests/test-conformance.sh` between the `. assert.sh` line and `t_summary` with:

```bash
if "$HERE/conformance/conformance.sh" "$HERE/conformance/drivers/bash.sh"; then
  t_ok "the bash toolkit passes the conformance suite"
else
  t_no "the bash toolkit FAILS the conformance suite"
fi

# The suite must be able to fail. Without this, a suite that silently stopped
# asserting anything would report a clean run forever, and the next real
# implementation would be waved through by a test that checks nothing.
if "$HERE/conformance/mutant-check.sh" >/dev/null 2>&1; then
  t_ok "the mutant driver fails the suite, so the suite has teeth"
else
  t_no "the mutant driver PASSED the suite - the suite is not checking"
fi
```

Create `tests/conformance/mutant-check.sh`:

```bash
#!/usr/bin/env bash
# Exits 0 when the mutant FAILS the suite, which is the wanted outcome.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if "$HERE/conformance.sh" "$HERE/drivers/mutant.sh" >/dev/null 2>&1; then
  exit 1
else
  exit 0
fi
```

- [ ] **Step 3: Run and confirm both halves**

```bash
cd /home/devops/dbhq-heliograph
chmod +x tests/conformance/mutant-check.sh
./tests/test-conformance.sh
```

Expected: `ok the bash toolkit passes the conformance suite` and `ok the mutant driver fails the suite, so the suite has teeth`.

Confirm by eye that the mutant fails for the right reasons:

```bash
./tests/conformance/conformance.sh tests/conformance/drivers/mutant.sh
```

Expected: `FAIL p1: three DISTINCT timestamps`, `FAIL p2`, `FAIL p3`, `FAIL p4` footer assertions, and `SKIP` for p5, p6, p8.

- [ ] **Step 4: Record the suite in AGENTS.md**

In `AGENTS.md`, under **Validating a change**, after the `./tests/run-tests.sh` line in the code block, add this paragraph immediately below that block:

```markdown
`tests/conformance/` is the executable form of the capture contract, run
against a driver per implementation. `drivers/mutant.sh` is deliberately
broken and the suite asserts that it **fails**: a conformance suite that
cannot fail is decoration, and the next port would sail through it. When you
add an implementation of the capture, add a driver rather than a new set of
tests, and do not weaken a property to make a driver pass.
```

- [ ] **Step 5: Validate and commit**

```bash
cd /home/devops/dbhq-heliograph
bash -n tests/conformance/*.sh tests/conformance/drivers/*.sh tests/test-conformance.sh
shellcheck -S warning tests/conformance/*.sh tests/conformance/drivers/*.sh tests/test-conformance.sh
grep -n '—' AGENTS.md tests/conformance/*.sh tests/conformance/drivers/*.sh || echo "no em dashes"
./tests/run-tests.sh
git add tests AGENTS.md
git commit -m "conformance: a suite that cannot fail is decoration, so prove it fails

The mutant breaks the capture the three ways a real port breaks it: one
timestamp for the whole block, the pipeline's exit code instead of the
step's, and no footer. The suite asserts it fails."
```

---

## Self-Review

**Spec coverage.** The parent spec lists eight properties. Task 1 covers 1, Task 2 covers 2, Task 3 covers 3, Task 4 covers 4, Task 5 covers 5 and 6, Task 6 covers 7, Task 7 covers the capture half of 8. The exit-130 half of property 8 is explicitly deferred to A6 in Task 7, with the reason stated: it is `station.sh` behaviour and needs a transport driver to observe a published status.

**No behaviour change.** No task modifies anything under `skills/heliograph/toolkit/`. Task 5 notes explicitly that a mismatch in the step table is fixed in the test, not in `run.sh`.

**Type consistency.** `drv_name`, `drv_supports`, `drv_capture`, `drv_bootstrap`, `drv_step` are defined in Task 1 and used with those exact names in Tasks 5, 6 and 7. `drv_capture_bg` is added in Task 7 and stubbed in the mutant in Task 8. `secs_of` and `stamps_of` are defined in Task 1 and used in Task 2.

**Known rough edge, deliberately left in.** Task 5's `sed` registration of a step into `run.sh`'s case table depends on that table's current shape, and Task 6's `grep -c` comparison may need `""` or `"0"`. Both are called out in their steps with what to check and what to change. They are left as verify-and-adjust rather than guessed at, because guessing wrong would be a silent pass.
