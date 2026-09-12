#!/usr/bin/env bash
# =============================================================================
#  test-seal-ps1.sh - the seal in PowerShell, and the compiler it has to survive
# =============================================================================
# Two questions, and the second is the one nothing else in this repository can
# ask on Linux.
#
#   1. DOES IT AGREE WITH GO? tests/seal-vectors.ps1 runs the RFCs' own vectors
#      for each primitive and then the Go side's own bytes for the whole
#      construction. Neither is a round trip, deliberately.
#
#   2. WILL IT COMPILE WHERE IT HAS TO RUN? Windows PowerShell 5.1's `Add-Type`
#      uses the .NET Framework CodeDOM compiler, which is C# 5 and has no
#      Span<T>. Nothing on a Linux runner would notice a modern construct
#      creeping into the C# - pwsh 7 compiles it with Roslyn against .NET 10
#      and every vector still passes - and the failure would land on the exact
#      estate this payload exists for, at Add-Type, as a compiler error in
#      somebody else's file.
#
#      So the whole tree is built as `net48` with `LangVersion 5`. That is the
#      same contract, measured here rather than assumed, and it is why
#      NaCl.Core could not be used: Span<T> in every file.
#
# The real 5.1 is exercised by CI on a Windows runner. This is what makes the
# same question answerable before the push.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
ROOT="$(cd "$HERE/.." && pwd)"
SEAL="$ROOT/station/powershell/lib/seal"

# CONF_PS_SHELL OVERRIDES, for the reason the conformance driver honours it:
# Windows PowerShell 5.1 is the floor, and a runner with both editions installed
# would otherwise only ever test 7 - which is the edition where none of this
# file's constraints apply.
PS_BIN="${CONF_PS_SHELL:-}"
if [ -n "$PS_BIN" ]; then
  command -v "$PS_BIN" >/dev/null 2>&1 || PS_BIN=""
else
  for c in pwsh powershell powershell.exe; do
    if command -v "$c" >/dev/null 2>&1; then PS_BIN="$c"; break; fi
  done
fi

# A PATH POWERSHELL WILL UNDERSTAND.
#
# Git-Bash converts Unix-looking paths at the EXEC BOUNDARY, so `-File /tmp/x`
# arrives native. A path EMBEDDED IN A SCRIPT - which is what `-Command` takes -
# gets no such conversion: PowerShell reads `/home/runner/...` as
# `C:\home\runner\...`, Import-Module fails, and the assertion below sees an
# empty string rather than an error.
#
# That is exactly how the capability checks in section 4 failed on Windows and
# nowhere else, reporting "the transport declares no capabilities" about a
# transport that was never loaded. The conformance driver's header warns about
# this trap; this file walked into it anyway.
winpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s' "$1"; fi
}

# =============================================================================
#  1. WHAT IS VENDORED, AND WHAT IS NOT
# =============================================================================
assert_eq "the vendored crypto is planted with the payload" "yes" \
  "$([ -f "$SEAL/vendor/Chaos.NaCl/Ed25519.cs" ] && echo yes || echo no)"
assert_eq "  with its licence" "yes" \
  "$([ -f "$SEAL/vendor/Chaos.NaCl/LICENSE.md" ] && echo yes || echo no)"
assert_eq "  and a note saying where it came from and what was changed" "yes" \
  "$([ -f "$SEAL/vendor/Chaos.NaCl/ORIGIN.md" ] && echo yes || echo no)"

# SOURCE, NOT AN ASSEMBLY. The proposition is plain text you can read before
# you run it, and station/embed_test.go refuses a binary in the payload for
# exactly this reason. A .dll here would pass the size cap and fail that.
BIN="$(find "$SEAL" -type f \( -name '*.dll' -o -name '*.exe' -o -name '*.so' \) 2>/dev/null | head -5)"
assert_eq "nothing under lib/seal is a compiled binary" "" "$BIN"

# THE TRAP, PINNED BY ABSENCE rather than by not calling it. `KeyExchange`
# returns crypto_box_beforenm and not the raw RFC 7748 secret, so it is deleted
# from the vendored source: an unused wrong function is one autocomplete away
# from being the used one, and no test catches a call nobody has written yet.
assert_eq "Ed25519.KeyExchange is deleted from the vendored source" "" \
  "$(grep -rn 'public static.*KeyExchange' "$SEAL" 2>/dev/null || true)"
assert_eq "  and the deletion is explained where it was" "yes" \
  "$(grep -q 'KeyExchange REMOVED WHEN THIS WAS VENDORED' "$SEAL/vendor/Chaos.NaCl/Ed25519.cs" && echo yes || echo no)"
# AND THE OTHER WRAPPER IS NOT IN THE TREE AT ALL. MontgomeryCurve25519 is the
# class whose KeyExchange has the same defect, so it is simply not vendored.
assert_eq "  and MontgomeryCurve25519 is not vendored either" "no" \
  "$([ -f "$SEAL/vendor/Chaos.NaCl/MontgomeryCurve25519.cs" ] && echo yes || echo no)"

# Nothing in the payload's PowerShell reaches for it. Comment lines are
# excluded, because the files that EXPLAIN the trap name it - and a check that
# banned the word would fail on the documentation warning people off it, which
# is the wrong direction to push.
assert_eq "  and no PowerShell in the payload names it outside a comment" "" \
  "$(grep -rn 'MontgomeryCurve25519' --include='*.ps1' --include='*.psm1' \
       "$ROOT/station/powershell" 2>/dev/null | grep -vE ':[[:space:]]*#' || true)"

# NO FRAMEWORK CONDITIONAL MAY SURVIVE IN THE VENDORED TREE.
#
# `Add-Type` compiles with NO preprocessor symbols defined, so every `#if
# NETSTANDARD`, `#if NET6_0_OR_GREATER` and friend takes its `#else` branch on a
# real station - and upstream's `#else` branches are written for the NEWEST
# runtime, not the oldest. That is how `GeneratePrivateKeySeed` shipped: its
# else-branch calls a .NET 6 overload, and Windows PowerShell 5.1 refused the
# whole module at load.
#
# The build below cannot catch this on its own, because a build has to choose
# some set of symbols and any choice that is not "none" compiles something other
# than what Add-Type does. So the conditionals are banned outright and the
# vendored helper is deleted rather than guarded.
#
# DEBUG is allowed: it is off by default in both, so both take the same branch.
COND="$(grep -rn '^[[:space:]]*#\(if\|elif\)' "$SEAL" 2>/dev/null | grep -v '#if DEBUG' || true)"
assert_eq "no framework conditional survives in the seal, because Add-Type defines no symbols" \
  "" "$COND"

# =============================================================================
#  2. IT COMPILES AS C# 5 AGAINST net48
# =============================================================================
# The single most valuable check in this file, and the one a Linux runner can
# do that a reading cannot.
if ! command -v dotnet >/dev/null 2>&1; then
  t_skip "no dotnet SDK, so C# 5 / net48 compatibility was NOT checked here (CI does it on a real 5.1)"
else
  BUILD="$(mktemp -d)"
  mkdir -p "$BUILD/src"
  cp -r "$SEAL/." "$BUILD/src/"
  # NO DefineConstants, AND THAT IS THE WHOLE POINT OF THIS BUILD.
  #
  # An earlier version defined NET462, because upstream guarded one helper
  # behind it. `Add-Type` defines NOTHING - so that build compiled a different
  # branch from the one Windows PowerShell 5.1 compiles, this check passed, and
  # the real 5.1 refused the file with "the best overloaded method match for
  # RandomNumberGenerator.GetBytes(byte[]) has some invalid arguments". A build
  # check that does not use the real compiler's flags is checking something
  # else. The helper is deleted from the vendored source instead.
  cat > "$BUILD/p.csproj" <<'EOF'
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net48</TargetFramework>
    <LangVersion>5</LangVersion>
    <EnableDefaultCompileItems>false</EnableDefaultCompileItems>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
  </PropertyGroup>
  <ItemGroup><Compile Include="src/**/*.cs" /></ItemGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.NETFramework.ReferenceAssemblies" Version="1.0.3" PrivateAssets="All" />
  </ItemGroup>
</Project>
EOF
  BUILD_OUT="$( cd "$BUILD" && timeout 600 dotnet build p.csproj -v q --nologo 2>&1 )"
  BUILD_RC=$?
  if [ "$BUILD_RC" = "0" ]; then
    t_ok "the whole seal compiles as C# 5 against net48, which is what Add-Type uses on Windows PowerShell 5.1"
  else
    t_no "the seal will NOT compile as C# 5 against net48, so it cannot load on a 5.1 station"
    printf '%s\n' "$BUILD_OUT" | grep -E 'error|warning' | head -10 | sed 's/^/     /'
  fi
  # WARNINGS ARE ERRORS ABOVE, and that is not fussiness: Add-Type itself
  # refuses to load source that warns, which is how the vendored Poly1305's
  # [Obsolete] attribute stopped the whole module loading the first time.
  rm -rf "$BUILD"
fi

# =============================================================================
#  3. THE VECTORS
# =============================================================================
if [ -z "$PS_BIN" ]; then
  t_skip "no PowerShell interpreter, so the seal's own vectors were NOT run"
else
  assert_eq "the Go side's golden vectors are committed" "yes" \
    "$([ -f "$ROOT/tests/fixtures/seal-vectors.json" ] && echo yes || echo no)"

  VEC_OUT="$( timeout 600 "$PS_BIN" -NoProfile -File "$HERE/seal-vectors.ps1" 2>&1 )"
  VEC_RC=$?

  if [ "$VEC_RC" = "0" ]; then
    printf '%s\n' "$VEC_OUT" | grep -E '^(FAIL|     )' | head -20 | sed 's/^/     /'
    t_ok "$(printf '%s' "$VEC_OUT" | sed -n 's/^seal-vectors.ps1: //p')"
  else
    # THE WHOLE TAIL, NOT THE LINES THAT LOOK LIKE ASSERTIONS.
    #
    # This printed only lines starting with `FAIL` or five spaces, which is the
    # shape a failed CHECK has. A script that dies before its first Check - a
    # missing type, a parse error, a module that will not import - produces a
    # PowerShell exception, matches neither, and was shown as nothing at all.
    #
    # That happened twice: once when Add-Type refused the vendored source, and
    # again when a type literal for a .NET 5 class killed the script on 5.1.
    # Both times CI reported seven failed assertions and not one word of why,
    # on the only platform that cannot be reproduced here.
    t_no "the PowerShell seal disagrees with the RFCs or with Go, or did not run at all (exit $VEC_RC)"
    printf '%s\n' "$VEC_OUT" | tail -30 | sed 's/^/     /'
  fi

  # A COUNT, so a vector file that silently stopped being read cannot pass. The
  # whole risk here is a check that reports success without measuring anything,
  # and "0 passed, 0 failed" is what that looks like.
  VEC_N="$(printf '%s' "$VEC_OUT" | grep -c '^ok   ')"
  assert_eq "and enough of them ran to mean something" "yes" \
    "$([ "${VEC_N:-0}" -ge 60 ] && echo yes || echo no)"

  # BOTH DIRECTIONS ARE ASSERTED BY NAME. Opening Go's bytes and producing Go's
  # bytes are different failures - a signing bug passes the first - and a run
  # that quietly stopped doing one of them would still report dozens of passes.
  assert_contains "it opens what Go sealed" "opens Go's own bytes" "$VEC_OUT"
  assert_contains "  and produces what Go would have sealed, byte for byte" \
    "matches Go byte for byte" "$VEC_OUT"
  assert_contains "  and refuses a tampered ciphertext" "refuses: tampered-ciphertext" "$VEC_OUT"
  assert_contains "  and refuses a replayed sequence number" "refuses: wrong-sequence-number" "$VEC_OUT"
  assert_contains "  and refuses a message forwarded to a third party" \
    "refuses: wrong-recipient-fingerprint" "$VEC_OUT"
fi

# =============================================================================
#  4. THE TRANSPORT DECLARES WHAT A METERED EDGE CAN DO
# =============================================================================
RELAY="$ROOT/station/powershell/transports/relay.psm1"
assert_eq "the PowerShell payload ships a relay transport" "yes" \
  "$([ -f "$RELAY" ] && echo yes || echo no)"

if [ -n "$PS_BIN" ]; then
  # winpath, because these are EMBEDDED IN A SCRIPT rather than passed as
  # arguments. See the note on winpath above.
  CAPS="$( timeout 120 "$PS_BIN" -NoProfile -Command "
    Import-Module '$(winpath "$ROOT/station/powershell/lib/transport.psm1")' -Force
    Import-Module '$(winpath "$RELAY")' -Force
    Get-TpCapabilities" 2>&1 | tr -d '\r' )"
  # NAMED WHEN IT IS EMPTY. An empty string satisfies none of the assertions
  # below and they each report "wanted [request], in: []", which describes the
  # transport rather than the import that failed.
  if [ -z "$CAPS" ]; then
    t_no "the relay transport would not load, so its capabilities could not be read"
  fi
  assert_contains "it carries a request" "request" "$CAPS"
  assert_contains "  a status" "status" "$CAPS"
  assert_contains "  and progress" "progress" "$CAPS"
  # NOT live: a mid-run poll costs a request per interval against a metered
  # edge. NOT self: a payload update is a sealed message and its own capability.
  assert_eq "  and NOT a live read, which would cost a request per interval" "no" \
    "$(printf '%s' "$CAPS" | grep -qw live && echo yes || echo no)"
  assert_eq "  and NOT self-update" "no" \
    "$(printf '%s' "$CAPS" | grep -qw self && echo yes || echo no)"

  # THE TWO DECLARATIONS MUST AGREE. bash and PowerShell stations are talked to
  # by the same control side, and a capability that differs between them is a
  # station that behaves differently from the same request.
  BASH_CAPS="$( bash -c "
    . '$ROOT/station/bash/caplib.sh' 2>/dev/null
    . '$ROOT/station/bash/transports/relay.sh'
    tp_capabilities" 2>/dev/null | tr -d '\r' )"
  assert_eq "the twins declare the same capabilities, or one channel behaves differently from the same request" \
    "$(printf '%s' "$BASH_CAPS" | tr ' ' '\n' | sort | tr '\n' ' ')" \
    "$(printf '%s' "$CAPS" | tr ' ' '\n' | sort | tr '\n' ' ')"
fi

# THE STATION SIDE NEEDS NO BINARY, which is the entire reason this exists. If
# a path to heliograph-seal ever appears here, the payload has quietly gone back
# to requiring an install on an estate that refuses one.
assert_eq "the PowerShell relay never reaches for heliograph-seal" "" \
  "$(grep -n 'heliograph-seal\|RELAY_SEAL' "$RELAY" 2>/dev/null | grep -v '^\s*[0-9]*:#' | grep -v '# ' || true)"

# =============================================================================
#  5. IT CAN DELIVER MORE THAN ONCE
# =============================================================================
# THE DEFECT THIS EXISTS FOR, and conformance structurally cannot find it.
#
# p9 bootstraps a station, delivers ONE log and reads it back. So the sequence
# counter is only ever written once, from nothing to one - and the second write
# was broken for a completely different reason from the first: PowerShell
# marshals `$null` to an EMPTY STRING for a `string` parameter, so
# File.Replace's backup argument was rejected, every write after the first
# failed, and `Get-RelayNextOut` handed out sequence 1 for ever. The relay's
# receiver drops anything at or below what it has accepted, so every delivery
# after the first vanished silently. 36 passed, 0 failed.
#
# A station's whole job is to deliver repeatedly. One delivery is not the
# property.
if [ -z "$PS_BIN" ] || ! command -v go >/dev/null 2>&1 || ! command -v python3 >/dev/null 2>&1; then
  t_skip "no PowerShell, Go or python3, so REPEATED delivery over the relay was NOT exercised"
else
  W="$(mktemp -d)"
  # THE RUNS' OUTPUT IS KEPT, and this is a correction rather than a nicety.
  #
  # The first version sent all three to /dev/null. When this failed on Windows
  # and nowhere else, CI could report only "expected [3 1,2,3], actual [0 ]" -
  # which says a delivery did not happen and nothing whatever about why, on the
  # one platform that cannot be reproduced locally. A test that discards the
  # evidence of its own failure costs a whole round trip through CI to learn
  # what one line would have said.
  (
    export CONF_TRANSPORT=relay
    # shellcheck disable=SC1091
    . "$HERE/conformance/drivers/powershell.sh"
    trap 'drv_teardown 2>/dev/null' EXIT
    if ! drv_bootstrap "$W/s" > "$W/bootstrap.log" 2>&1; then
      echo "BOOTSTRAP-FAILED"
      exit 1
    fi
    drv_step_file ships "$W/s/steps/ships"
    for i in 1 2 3; do
      ( cd "$W/s" && _drv_ps_env "$W/s" && ALLOW_ROOT=1 \
          timeout 120 "$_P_SHELL" -NoProfile -File ./run.ps1 ./steps/ships.ps1 ) \
        > "$W/run$i.log" 2>&1
    done
    # READ FROM THE RELAY, not from the state file. The counter advancing and
    # three messages arriving are different claims, and only the second is the
    # one an operator cares about.
    curl -sS -m 10 -H "Authorization: Bearer $_P_RELAY_CTL" \
      "$(_p_relay_url "$W/s.relay")/v1/$_P_RELAY_ESTATE/station/s2c" 2>/dev/null \
      | python3 -c 'import sys,json
try: d=json.load(sys.stdin)
except Exception: print("0 -"); raise SystemExit
print(len(d), ",".join(str(m["seq"]) for m in sorted(d, key=lambda x: x["seq"])))'
  ) > "$W/out" 2>"$W/err"
  GOT="$(cat "$W/out" 2>/dev/null)"
  if [ "$GOT" = "3 1,2,3" ]; then
    t_ok "three runs put three messages on the relay, with three distinct sequence numbers"
  else
    t_no "three runs did not put three messages on the relay: got [$GOT], wanted [3 1,2,3]"
    for f in bootstrap.log run1.log run2.log run3.log err; do
      [ -s "$W/$f" ] || continue
      printf '     --- %s\n' "$f"
      tail -12 "$W/$f" | sed 's/^/     /'
    done
  fi
  rm -rf "$W"
fi

t_summary
