#!/usr/bin/env bash
# =============================================================================
#  test-powershell.sh - a Windows control node, and steps written in PowerShell
# =============================================================================
# Two things ship together here and they fail in different places:
#
#   1. LINE ENDINGS. .gitattributes pins the transport repo to LF. The point is
#      NOT to protect Windows from itself - Git for Windows' bash strips CR and
#      runs a CRLF checkout perfectly well, measured on Server 2022 with Git
#      2.55. The point is that CRLF which gets COMMITTED breaks every Linux
#      clone afterwards, and breaks it silently when the file is sourced.
#
#   2. ps_step. A .ps1 step goes through the same capture as a .sh one, so the
#      log has to come out with the same properties: every line stamped, no
#      stray CR, no ANSI escapes, and the step's OWN exit code.
#
# --- what is asserted against what -------------------------------------------
# Every value below is read back from a real run: a real bootstrap into a real
# git repo, a real capture into a real log file. Nothing here asserts against a
# fixture this file already knows the answer to. That "echoes its own input
# back" shape is the recurring defect on this project - fourteen assertions
# across earlier PRs turned out unable to fail - and the defence is to make git
# and the capture do the comparing.
#
# --- the skip ----------------------------------------------------------------
# PowerShell is not guaranteed on the machine running this suite, and on a
# Linux dev box it usually is not there. The line-ending assertions need only
# git and run unconditionally; the ps_step ones need pwsh. When it is absent
# this file says so LOUDLY and names what went unchecked, rather than quietly
# reporting a clean run. A scrollback that reads as though everything was
# checked, when half of it was not, is the same defect as a silently skipped
# checksum.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
REPO="$(cd "$HERE/.." && pwd)"
BOOTSTRAP="$REPO/station/bootstrap.sh"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# --- a real transport repo, made the way an operator would make one ----------
TR="$WORK/transport"
mkdir -p "$TR"
git -C "$TR" init -q .
git -C "$TR" config user.email test@example.invalid
git -C "$TR" config user.name test
bash "$BOOTSTRAP" "$TR" >/dev/null 2>&1

# =============================================================================
#  1. LINE ENDINGS
# =============================================================================

# bootstrap has to restore the dot, exactly as it does for gitignore. Shipping
# the file as `gitattributes` is deliberate so it governs the transport repo
# rather than the skill repo carrying it.
if [ -f "$TR/.gitattributes" ]; then
  t_ok ".gitattributes is installed with its dot restored"
else
  t_no ".gitattributes missing from a bootstrapped repo (found: $(ls -a "$TR" | tr '\n' ' '))"
fi

if [ -f "$TR/gitattributes" ]; then
  t_no "the undotted gitattributes was copied through as well, so the transport repo has both"
else
  t_ok "the undotted gitattributes is not left behind in the transport repo"
fi

# THE functional test, and the only one that proves the file does anything.
# Commit a CRLF file with autocrlf=true, then ask git what it actually STORED.
# If .gitattributes is working, the blob is LF whatever the local setting says,
# which is what stops a Windows contributor breaking every Linux clone.
git -C "$TR" add -A >/dev/null 2>&1
git -C "$TR" commit -qm "toolkit" >/dev/null 2>&1
git -C "$TR" config core.autocrlf true
printf '#!/usr/bin/env bash\r\nset -uo pipefail\r\necho hi\r\n' > "$TR/steps/crlf-probe.sh"
git -C "$TR" add steps/crlf-probe.sh >/dev/null 2>&1
git -C "$TR" commit -qm "a file written with CRLF" >/dev/null 2>&1
stored_cr="$(git -C "$TR" cat-file blob HEAD:steps/crlf-probe.sh | tr -cd '\r' | wc -c | tr -d ' ')"
if [ "$stored_cr" = "0" ]; then
  t_ok "a CRLF file committed with autocrlf=true is stored as LF, so Linux clones are safe"
else
  t_no "git stored $stored_cr CR bytes in the blob, so .gitattributes is not pinning line endings"
fi

# The working tree is pinned as well as the blob: eol=lf means checkout hands
# back LF even with autocrlf=true asking for the opposite. Restoring the file
# here also leaves the checkout clean for the preflight assertions below, which
# would otherwise find this deliberately broken file and fail on it.
rm -f "$TR/steps/crlf-probe.sh"
git -C "$TR" checkout -q -- steps/crlf-probe.sh
wt_cr="$(tr -cd '\r' < "$TR/steps/crlf-probe.sh" | wc -c | tr -d ' ')"
if [ "$wt_cr" = "0" ]; then
  t_ok "checkout hands that file back as LF, so eol=lf beats a local autocrlf=true"
else
  t_no "checkout produced $wt_cr CR bytes despite eol=lf, so the working tree is not pinned"
fi

# And the negative: without the attributes file, the same commit keeps its CRLF.
# This is what makes the assertion above capable of failing.
BARE="$WORK/noattrs"
mkdir -p "$BARE"
git -C "$BARE" init -q .
git -C "$BARE" config user.email test@example.invalid
git -C "$BARE" config user.name test
git -C "$BARE" config core.autocrlf false
printf 'echo hi\r\n' > "$BARE/f.sh"
git -C "$BARE" add f.sh >/dev/null 2>&1
git -C "$BARE" commit -qm x >/dev/null 2>&1
bare_cr="$(git -C "$BARE" cat-file blob HEAD:f.sh | tr -cd '\r' | wc -c | tr -d ' ')"
if [ "$bare_cr" != "0" ]; then
  t_ok "without .gitattributes the same commit keeps its CRLF, so the check above can fail"
else
  t_no "a repo with no .gitattributes also normalised to LF, so the test above proves nothing"
fi

# --- start.sh's probe, which must give opposite answers on different bashes --
# It asks THIS bash whether it tolerates CR rather than assuming. On Linux the
# answer is no; under Git for Windows it is yes, and reporting a blanket
# failure there would refuse to start a control node that works.
probe="$WORK/crprobe"
printf 'hg_crlf_probe=1\r\n' > "$probe"
# shellcheck disable=SC1090
if ( . "$probe" 2>/dev/null; [ "${hg_crlf_probe:-}" = "1" ] ); then
  tolerant=yes
else
  tolerant=no
fi
out="$(cd "$TR" && ./start.sh --check 2>&1 | grep -i 'line endings')"
if [ "$tolerant" = yes ]; then
  assert_contains "on a CR-tolerant bash the preflight says so instead of failing" "tolerates CR" "$out"
else
  assert_contains "on a CR-intolerant bash a clean checkout passes" "LF throughout" "$out"
fi

# Mutation, run for real: break one SOURCED file and require the preflight to
# notice. On a tolerant bash it must NOT fail, which is the whole point of the
# probe, so the expectation flips with the platform.
cp "$TR/lib/probe.sh" "$WORK/probe.sh.bak"
sed -i 's/$/\r/' "$TR/lib/probe.sh"
mut="$(cd "$TR" && ./start.sh --check 2>&1 | grep -i 'line endings')"
cp "$WORK/probe.sh.bak" "$TR/lib/probe.sh"
if [ "$tolerant" = yes ]; then
  if printf '%s' "$mut" | grep -q '^FAIL'; then
    t_no "a CR-tolerant bash was told its working checkout is broken, which would block a good Windows node"
  else
    t_ok "a CR-tolerant bash is not blocked by a CRLF checkout it can actually run"
  fi
else
  if printf '%s' "$mut" | grep -q 'lib/probe.sh'; then
    t_ok "a CRLF sourced file is caught and named on a bash that cannot run it"
  else
    t_no "a CRLF lib/probe.sh went unreported on a bash that cannot source it: got [$mut]"
  fi
fi

# =============================================================================
#  2. ps_step
# =============================================================================
PS_BIN=""
for c in pwsh powershell.exe powershell; do
  command -v "$c" >/dev/null 2>&1 && { PS_BIN="$c"; break; }
done

if [ -z "$PS_BIN" ]; then
  echo
  echo "SKIPPING every PowerShell assertion in this file: no pwsh, powershell.exe"
  echo "or powershell on PATH. NOT CHECKED, and none of it is covered elsewhere:"
  echo "  - a .ps1 step reaches the log with every line timestamped"
  echo "  - no stray CR and no ANSI escape survives into the committed log"
  echo "  - the step's OWN exit code is reported, not one leaked from a native"
  echo "    command inside it"
  echo "  - the capture stays unbuffered, so a hang is still visible"
  echo "  - station.ps1 and the shipped .ps1 files parse"
  echo "Install PowerShell 7 (https://aka.ms/powershell) to run them."
  echo
  t_summary
  exit 0
fi

t_ok "a PowerShell interpreter is present ($PS_BIN), so the assertions below ran"

# Every .ps1 the toolkit ships has to parse. Nothing else in CI reads them, so
# without this a syntax error would ship and only fail on a Windows box.
parse_out="$("$PS_BIN" -NoProfile -Command '
  $bad = 0
  Get-ChildItem -Path $args[0] -Filter *.ps1 -Recurse -File | ForEach-Object {
    $errors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile($_.FullName, [ref]$null, [ref]$errors)
    if ($errors -and $errors.Count -gt 0) { $bad++; Write-Output "BAD $($_.Name): $($errors[0].Message)" }
  }
  Write-Output "count=$((Get-ChildItem -Path $args[0] -Filter *.ps1 -Recurse -File).Count) bad=$bad"
' "$TR" 2>&1)"
assert_contains "every .ps1 shipped into a transport repo parses" "bad=0" "$parse_out"
if printf '%s' "$parse_out" | grep -q 'count=0'; then
  t_no "no .ps1 files were found to parse, so the assertion above checked nothing"
else
  t_ok "the parse check actually found .ps1 files to read"
fi

# AND EVERY .psm1 AND .ps1 IN THE REPOSITORY, which the check above does not
# reach: it reads a bootstrapped transport repo, and station/powershell/ is not
# planted into one yet. So caplib.psm1 - the whole second implementation of the
# capture - could have shipped with a syntax error and only failed on a Windows
# box, which is precisely the failure this file exists to prevent.
repo_parse="$("$PS_BIN" -NoProfile -Command '
  $bad = 0
  $files = @(Get-ChildItem -Path $args[0] -Include *.ps1,*.psm1 -Recurse -File |
             Where-Object { $_.FullName -notmatch "[\\/]\.git[\\/]" })
  # Broken deliberately once, to watch it fail: a filter that matched nothing
  # reported bad=0 over an empty list, which reads identically to a clean run.
  foreach ($f in $files) {
    $errors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile($f.FullName, [ref]$null, [ref]$errors)
    if ($errors -and $errors.Count -gt 0) { $bad++; Write-Output "BAD $($f.Name): $($errors[0].Message)" }
  }
  Write-Output "count=$($files.Count) bad=$bad"
' "$HERE/.." 2>&1)"
assert_contains "every .ps1 and .psm1 in the repository parses" "bad=0" "$repo_parse"
# The count, asserted, because -Include silently matches nothing when -Path has
# no wildcard and -Recurse is absent - a shape that reports a clean sweep over
# an empty list.
repo_count="$(printf '%s' "$repo_parse" | sed -n 's/.*count=\([0-9]*\).*/\1/p')"
if [ -n "$repo_count" ] && [ "$repo_count" -ge 5 ]; then
  t_ok "the repository sweep found $repo_count PowerShell files to read"
else
  t_no "the repository sweep found ${repo_count:-no} PowerShell files, so it checked nothing"
fi

# --- a real capture through the real runner ---------------------------------
cat > "$TR/steps/t-clean.ps1" <<'EOF'
# heliograph-mode: read-only
Write-Output "plain line"
[PSCustomObject]@{ Name = 'alpha'; Value = 1 } | Format-Table -AutoSize
EOF
# A step whose last NATIVE command fails while the step itself succeeds. This
# is the shape that made win-snapshot report failure: `git config --get` on an
# unset key exits 1, and an in-process invocation leaked that as the step's
# exit code.
cat > "$TR/steps/t-leak.ps1" <<'EOF'
# heliograph-mode: read-only
Write-Output "the step itself succeeded"
git config --get heliograph.no.such.key 2>$null | Out-Null
EOF
cat > "$TR/steps/t-fail.ps1" <<'EOF'
# heliograph-mode: read-only
Write-Output "this one really did fail"
exit 3
EOF
cat > "$TR/steps/t-slow.ps1" <<'EOF'
# heliograph-mode: read-only
Write-Output "first"
Start-Sleep -Seconds 2
Write-Output "second"
EOF
sed -i 's|^  win)  ps_step ./steps/win-snapshot.ps1 ;;|  win)   ps_step ./steps/win-snapshot.ps1 ;;\
  tclean) ps_step ./steps/t-clean.ps1 ;;\
  tleak)  ps_step ./steps/t-leak.ps1 ;;\
  tfail)  ps_step ./steps/t-fail.ps1 ;;\
  tslow)  ps_step ./steps/t-slow.ps1 ;;|' "$TR/run.sh"

( cd "$TR" && PUSH=0 ./run.sh tclean >/dev/null 2>&1 )
LOG="$(ls -t "$TR"/ops-logs/tclean-*.txt 2>/dev/null | head -1)"
if [ -z "$LOG" ]; then
  t_no "a PowerShell step produced no log at all"
else
  t_ok "a PowerShell step produced a log through the ordinary runner"

  body_lines="$(grep -cE '^[0-9]{2}:[0-9]{2}:[0-9]{2} \| ' "$LOG")"
  if [ "$body_lines" -gt 0 ]; then
    t_ok "the PowerShell step's output is timestamped in the log ($body_lines lines)"
  else
    t_no "no timestamped lines in a PowerShell step's log"
  fi

  # The step's own text has to be there. Without this, an empty capture would
  # satisfy every "no CR, no ESC" assertion below.
  assert_contains "the step's actual output reached the log" "plain line" "$(cat "$LOG")"
  assert_contains "formatted object output reached the log too" "alpha" "$(cat "$LOG")"

  cr="$(tr -cd '\r' < "$LOG" | wc -c | tr -d ' ')"
  assert_eq "no stray CR survives into the committed log" "0" "$cr"

  esc="$(tr -cd '\033' < "$LOG" | wc -c | tr -d ' ')"
  assert_eq "no ANSI escape survives into the committed log" "0" "$esc"
fi

# --- the escape that cap_run does NOT already handle -------------------------
# The assertion above cannot fail on its own, and saying so is the point.
# cap_run strips ANSI with 's/\x1b\[[0-9;]*[mGKHF]//g', which removes everything
# Format-Table emits, so a colour-only step would satisfy it whether ps_step set
# NO_COLOR or not. OSC 8 hyperlinks are the real gap: pwsh writes them as
# ESC]8;;<url>, that sed only matches ESC[ sequences, and they reach the log.
# This step emits one, so the assertion below is capable of failing.
cat > "$TR/steps/t-link.ps1" <<'EOF'
# heliograph-mode: read-only
Write-Output "plain line"
if ($PSStyle) { $PSStyle.FormatHyperlink("docs", "https://example.invalid") }
EOF
sed -i 's|^  tslow)  ps_step ./steps/t-slow.ps1 ;;|  tslow)  ps_step ./steps/t-slow.ps1 ;;\
  tlink)  ps_step ./steps/t-link.ps1 ;;|' "$TR/run.sh"

if "$PS_BIN" -NoProfile -Command 'exit $(if ($PSStyle) { 0 } else { 1 })' 2>/dev/null; then
  ( cd "$TR" && PUSH=0 ./run.sh tlink >/dev/null 2>&1 )
  LLOG="$(ls -t "$TR"/ops-logs/tlink-*.txt 2>/dev/null | head -1)"
  if [ -n "$LLOG" ]; then
    link_esc="$(tr -cd '\033' < "$LLOG" | wc -c | tr -d ' ')"
    assert_eq "an OSC 8 hyperlink does not reach the log either, which cap_run alone would not prevent" "0" "$link_esc"

    # Prove that assertion is load-bearing: without NO_COLOR the same step
    # leaves the escape in the log. If this comes back 0 too, the check above
    # is decoration and should be deleted rather than trusted.
    cp "$TR/run.sh" "$TR/run.sh.bak"
    sed -i 's|CMD=(env NO_COLOR=1 TERM=dumb "\$sh"|CMD=(env "$sh"|' "$TR/run.sh"
    rm -f "$TR"/ops-logs/tlink-*.txt
    ( cd "$TR" && PUSH=0 ./run.sh tlink >/dev/null 2>&1 )
    MLOG="$(ls -t "$TR"/ops-logs/tlink-*.txt 2>/dev/null | head -1)"
    mut_esc="$(tr -cd '\033' < "$MLOG" 2>/dev/null | wc -c | tr -d ' ')"
    mv "$TR/run.sh.bak" "$TR/run.sh"
    if [ "${mut_esc:-0}" -gt 0 ]; then
      t_ok "removing NO_COLOR puts the escape back ($mut_esc bytes), so the assertion above can fail"
    else
      t_no "removing NO_COLOR changed nothing, so the hyperlink assertion proves nothing and NO_COLOR is doing no work"
    fi
  else
    t_no "the hyperlink step produced no log"
  fi
else
  t_skip "no \$PSStyle on this edition, so OSC 8 hyperlinks cannot be produced to test against"
fi

# --- exit codes, the bug this file exists to keep fixed ----------------------
( cd "$TR" && PUSH=0 ./run.sh tleak >/dev/null 2>&1 ); leak_rc=$?
assert_eq "a step whose internal native command fails still reports success" "0" "$leak_rc"

( cd "$TR" && PUSH=0 ./run.sh tfail >/dev/null 2>&1 ); fail_rc=$?
assert_eq "a step that genuinely exits 3 reports 3" "3" "$fail_rc"

FLOG="$(ls -t "$TR"/ops-logs/tfail-*.txt 2>/dev/null | head -1)"
if [ -n "$FLOG" ]; then
  assert_contains "a failing PowerShell step still ships a log carrying its own output" "this one really did fail" "$(cat "$FLOG")"
  assert_contains "and that log records the real exit code" "exit code    : 3" "$(cat "$FLOG")"
else
  t_no "a failing PowerShell step produced no log, and a failed run must still ship"
fi

# --- unbuffered, which is the constraint the whole toolkit exists for --------
# PowerShell pipelines buffer by default. If Out-String -Stream collected the
# step's output and released it at the end, every line would carry one
# timestamp: a log that looks fine and hides exactly the hang it was captured
# to find.
( cd "$TR" && PUSH=0 ./run.sh tslow >/dev/null 2>&1 )
SLOG="$(ls -t "$TR"/ops-logs/tslow-*.txt 2>/dev/null | head -1)"
if [ -n "$SLOG" ]; then
  stamps="$(grep -E 'first|second' "$SLOG" | grep -oE '^[0-9]{2}:[0-9]{2}:[0-9]{2}' | sort -u | wc -l | tr -d ' ')"
  if [ "$stamps" -ge 2 ]; then
    t_ok "two seconds apart, the lines carry different stamps: the capture stays unbuffered"
  else
    t_no "both lines share one timestamp, so PowerShell output is being buffered and a hang would be invisible"
  fi
else
  t_no "the slow step produced no log"
fi

# EVERY FUNCTION MUST ALSO PARSE ON ITS OWN, which is not the same check.
#
# A single quote inside a PowerShell single-quoted string is written by
# DOUBLING it; a backslash does not escape it. That produced a function whose
# body was unbalanced, and the whole-file parse still reported it - at a line
# number the eye slides over, in a file nothing else on Linux ever executes.
# Asking per function names the one that is broken.
#
# -File, NOT -Command. `pwsh -Command '<script>' arg` does not bind arg into
# $args, so the first version scanned the CURRENT directory instead of the
# bootstrapped payload and would have passed with the payload empty.
PS_PARSE="$TR/.parsecheck.ps1"
cat > "$PS_PARSE" <<'PSPARSE'
$bad = 0; $n = 0
Get-ChildItem -Path $args[0] -Filter *.ps1 -Recurse -File | ForEach-Object {
  $ast = [System.Management.Automation.Language.Parser]::ParseFile($_.FullName, [ref]$null, [ref]$null)
  foreach ($d in $ast.FindAll({ param($x) $x -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true)) {
    $n++
    $e = $null
    [void][System.Management.Automation.Language.Parser]::ParseInput($d.Extent.Text, [ref]$null, [ref]$e)
    if ($e -and $e.Count -gt 0) { $bad++; Write-Output "BAD $($_.Name)/$($d.Name): $($e[0].Message)" }
  }
}
Write-Output "functions=$n bad=$bad"
PSPARSE
funcs_out="$("$PS_BIN" -NoProfile -File "$PS_PARSE" "$TR" 2>&1)"
assert_contains "and every function in them parses on its own" "bad=0" "$funcs_out"
if printf '%s' "$funcs_out" | grep -q 'functions=0'; then
  t_no "no functions were found, so the assertion above checked nothing"
else
  t_ok "there were functions to check"
fi

# =============================================================================
#  A Windows station that is not git
# =============================================================================
# A SCHEDULED TASK INHERITS NOTHING from the shell that registered it, and on
# Windows that bites hardest: there is no systemd EnvironmentFile and no
# ~/.git-token equivalent for a relay. So .station-env is the ONLY way a
# detached Windows station can be given a RELAY_TOKEN.
#
# ONE IMPLEMENTATION OF THE RULES, and it is bash, because the file is bash.
# service.ps1 used to carry its own in PowerShell, and an adversarial read found
# six ways the two classified the same file differently. The rules themselves
# have their own file - tests/test-station-env.sh - so what is left here is the
# WINDOWS half: that service.ps1 delegates, and that station.ps1 exports.

# --- service.ps1 asks that script, rather than carrying its own copy ---------
assert_eq "service.ps1 no longer parses the env file itself" "0" \
  "$(grep -c 'Test-StationEnvFormat\|Get-TransportNeeds' "$TR/service.ps1")"
assert_contains "it calls station-env.sh" "station-env.sh" "$(cat "$TR/service.ps1")"
assert_contains "through the bash station.ps1 already knows how to find" \
  "Find-GitBash" "$(cat "$TR/service.ps1")"
assert_eq "and that discovery is defined once, not twice" "1" \
  "$(grep -c 'function Find-GitBash' "$TR/lib/Find-GitBash.ps1")"
assert_eq "station.ps1 dot-sources it rather than defining its own" "0" \
  "$(grep -c 'function Find-GitBash' "$TR/station.ps1")"

# --- station.ps1 itself, run end to end --------------------------------------
#
# station.ps1 must EXPORT what it sources, or start.sh - a new process - sees
# none of it. That omission made the launchd and setsid paths read the file and
# discard every value, and Windows has no EnvironmentFile to fall back on.
#
# RUN, NOT READ, AND THE REAL SCRIPT. The first version grepped for "set -a" and
# passed on the COMMENT explaining why "set -a" is there. The second extracted
# the prefix and ran that alone, which stays green if the prefix is removed from
# the command or moved after the exec. station.ps1 honours $env:HELIOGRAPH_BASH,
# so on Linux it can be pointed at the bash that is already here and driven.
# HELIOGRAPH_BASH IS SET ONLY OFF WINDOWS, and that is the point rather than a
# convenience. On Windows, finding Git bash IS what station.ps1 is for, so
# overriding it there would skip the thing under test - and the first version
# did exactly that and failed on the runner: under Git bash `command -v bash`
# gives /usr/bin/bash, which converts to C:/Program Files/Git/usr/bin/bash, and
# the real file is bash.exe. Find-GitBash knows that; a test guessing at it does
# not.
PS_BASH_ENV=()
case "$(uname -s 2>/dev/null)" in
  MINGW*|MSYS*|CYGWIN*) ;;
  *) PS_BASH_ENV=("HELIOGRAPH_BASH=$(command -v bash)") ;;
esac

cat > "$TR/start.sh" <<'STARTSH'
#!/usr/bin/env bash
echo "TRANSPORT=[${TRANSPORT:-}]"
echo "RELAY_URL=[${RELAY_URL:-}]"
echo "ARGS=[$*]"
exit 7
STARTSH
chmod +x "$TR/start.sh"
printf "TRANSPORT='relay'\nRELAY_URL='https://r.invalid'\n" > "$TR/.station-env"

RC=0
OUT="$(env ${PS_BASH_ENV[@]+"${PS_BASH_ENV[@]}"} "$PS_BIN" -NoProfile -File "$TR/station.ps1" --once 2>&1)" || RC=$?
assert_contains "station.ps1 hands the env file's values to start.sh, a NEW process" \
  "TRANSPORT=[relay]" "$OUT"
assert_contains "including the transport's own variables" \
  "RELAY_URL=[https://r.invalid]" "$OUT"
assert_contains "and the arguments still arrive alongside them" "ARGS=[--once]" "$OUT"
# exec, so the station's exit code is the one Windows sees. A scheduled task
# that always reported success would hide every failed start.
assert_eq "and start.sh's own exit code comes back through station.ps1" "7" "$RC"

rm -f "$TR/.station-env"
RC=0
OUT="$(env ${PS_BASH_ENV[@]+"${PS_BASH_ENV[@]}"} "$PS_BIN" -NoProfile -File "$TR/station.ps1" --once 2>&1)" || RC=$?
assert_contains "with no env file at all it still hands over" "ARGS=[--once]" "$OUT"
assert_contains "with nothing set, which is right" "TRANSPORT=[]" "$OUT"

t_summary
