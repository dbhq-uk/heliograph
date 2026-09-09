#!/usr/bin/env bash
# =============================================================================
#  test-redaction-corpus.sh - every redaction rule is load-bearing, or the
#                             corpus is not testing it
# =============================================================================
# `tests/fixtures/redaction-corpus.txt` is the specification both implementations
# of the redactor are measured against. Conformance property 7 runs it through
# the capture path, which proves the redactor is WIRED IN.
#
# It does not prove the corpus is any good, and the first draft was not.
#
# A corpus grows by someone adding a case for a rule they just wrote, and the
# case they reach for is usually the realistic one - a GitLab token in a clone
# URL, a Bearer token behind an `Authorization:` header. Both are caught by a
# DIFFERENT, broader rule. So the vendor rule could be deleted outright and the
# corpus reported a clean run. Three rules were in that state when this was
# written: GitLab, Bearer and Basic, plus the bare-userinfo URL rule and the
# generic `Authorization:` one once those were fixed.
#
# THE ONLY WAY TO KNOW IS TO DELETE THEM. So this deletes each rule in turn,
# from a COPY, and requires the corpus to notice. A rule the corpus does not
# miss is a rule the corpus is not testing, whatever else it appears to cover.
#
# It also runs the corpus straight through cap_redact - both directions, every
# line - which is faster than the conformance run and says which line failed.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=assert.sh disable=SC1091
. "$HERE/assert.sh"
TOOLKIT="$(cd "$HERE/../station/bash" && pwd)"
CORPUS="$HERE/fixtures/redaction-corpus.txt"
CAPLIB="$TOOLKIT/caplib.sh"

TAB="$(printf '\t')"

# The corpus writes a PEM header as %%PEM_BEGIN%% because CI refuses that
# literal anywhere in the repository, with no allow list - see the note in the
# corpus. Assembled rather than written out, or this line would trip the gate.
PEM_BEGIN="-----BEGIN RSA PRIVATE$(printf ' ')KEY-----"
corpus_expand() { printf '%s' "${1//\%\%PEM_BEGIN\%\%/$PEM_BEGIN}"; }

if [ ! -r "$CORPUS" ]; then
  t_no "the redaction corpus is missing from $CORPUS"
  t_summary
  exit 1
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# --- the corpus parses --------------------------------------------------------
# First, because everything below is silently vacuous otherwise: a tab turned
# into spaces by an editor gives zero records and a clean run over nothing.
masks=0 keeps=0
while IFS="$TAB" read -r verdict needle _line; do
  case "$verdict" in
    MASK) [ -n "$needle" ] && masks=$((masks + 1)) ;;
    KEEP) [ -n "$needle" ] && keeps=$((keeps + 1)) ;;
  esac
done < "$CORPUS"

if [ "$masks" -ge 15 ] && [ "$keeps" -ge 5 ]; then
  t_ok "the corpus parses: $masks secrets and $keeps must-keep lines"
else
  t_no "the corpus barely parses: $masks MASK and $keeps KEEP records"
  printf '     Tab-separated, three fields. A tab turned into spaces is enough.\n'
  t_summary
  exit 1
fi

# --- run it through a given caplib --------------------------------------------
# leaks_under <caplib> - prints the MASK needles that survived, one per line.
#
# In a subshell with the caplib sourced fresh each time, because a mutated one
# must not leak into the next iteration - and because `cap_redact` is a
# function, not a program, so there is nothing else to isolate it.
leaks_under() {
  local lib="$1"
  (
    # shellcheck disable=SC1090
    . "$lib" >/dev/null 2>&1
    while IFS="$TAB" read -r verdict needle line; do
      [ "$verdict" = MASK ] || continue
      [ -n "$needle" ] && [ -n "$line" ] || continue
      needle="$(corpus_expand "$needle")"; line="$(corpus_expand "$line")"
      if printf '%s\n' "$line" | cap_redact 2>/dev/null | grep -qF -- "$needle"; then
        printf '%s\n' "$needle"
      fi
    done < "$CORPUS"
  )
}

eaten_under() {
  local lib="$1"
  (
    # shellcheck disable=SC1090
    . "$lib" >/dev/null 2>&1
    while IFS="$TAB" read -r verdict needle line; do
      [ "$verdict" = KEEP ] || continue
      [ -n "$needle" ] && [ -n "$line" ] || continue
      needle="$(corpus_expand "$needle")"; line="$(corpus_expand "$line")"
      if ! printf '%s\n' "$line" | cap_redact 2>/dev/null | grep -qF -- "$needle"; then
        printf '%s\n' "$needle"
      fi
    done < "$CORPUS"
  )
}

# --- the redactor as it stands ------------------------------------------------
leaked="$(leaks_under "$CAPLIB")"
if [ -z "$leaked" ]; then
  t_ok "no secret in the corpus survives cap_redact"
else
  t_no "these corpus secrets survived cap_redact:"
  printf '%s\n' "$leaked" | sed 's/^/     /'
fi

eaten="$(eaten_under "$CAPLIB")"
if [ -z "$eaten" ]; then
  t_ok "and no must-keep line is eaten, so evidence survives"
else
  t_no "cap_redact ate these, which a reader needs:"
  printf '%s\n' "$eaten" | sed 's/^/     /'
fi

# --- EVERY RULE IS LOAD-BEARING ----------------------------------------------
# Each redaction rule is one `-e "s..."` line. Addressed BY LINE NUMBER rather
# than by pattern: a regex of mine that silently matched nothing would remove
# no rule and report that the corpus caught it, which is the same shape of lie
# this whole file exists to find.
rules="$(grep -c '^    -e "s' "$CAPLIB")"
if [ "$rules" -lt 10 ]; then
  t_no "only $rules redaction rules found in caplib.sh - has cap_redact moved?"
  t_summary
  exit 1
fi
t_ok "cap_redact has $rules rules, each checked below"

# NEUTRALISED, NOT DELETED, and this was itself a defect in the first version.
#
# The rules are one long `sed` continued with trailing backslashes, and the
# LAST one has no backslash. Deleting that line left the previous one ending in
# `\` immediately before the closing brace, so cap_redact would not parse -
# and a redactor that does not exist leaks nothing, which read as "the corpus
# caught it". The one rule the mutation could not test was the only one it
# reported as covered.
#
# So the rule is replaced by a no-op expression that matches nothing, keeping
# the line's trailing backslash exactly as it was.
#
# python3 rather than awk or sed, and that too was a defect first. `awk -v`
# processes escape sequences in the value, so a replacement ending in a single
# backslash lost it - which broke the continuation, left a sed invocation
# missing its remaining rules, and made EVERY rule look uncovered. Byte-exact
# rewriting is the whole job here; the tool has to do exactly what it is told.
neutralise() {  # neutralise <line-number> -> writes $WORK/mutant.sh
  python3 - "$CAPLIB" "$1" "$WORK/mutant.sh" <<'PY'
import sys
src, n, dst = sys.argv[1], int(sys.argv[2]), sys.argv[3]
lines = open(src).read().split("\n")
trail = " \\" if lines[n - 1].rstrip().endswith("\\") else ""
lines[n - 1] = '    -e "s/heliograph-no-such-pattern-ever//g"' + trail
open(dst, "w").write("\n".join(lines))
PY
  # And it must still be a shell script. A mutant that will not parse leaks
  # nothing, which reads exactly like a rule the corpus caught.
  bash -n "$WORK/mutant.sh" 2>/dev/null
}

# And the mutant must still WORK, or the result means nothing. Proved against a
# control line every iteration: a secret that this rule has nothing to do with
# must still be masked. If it is not, cap_redact is broken rather than blind.
CONTROL_LINE='aws_access_key_id AKIAIOSFODNN7EXAMPLE'
CONTROL_NEEDLE='AKIAIOSFODNN7EXAMPLE'
alive_under() {
  ( # shellcheck disable=SC1090
    . "$1" >/dev/null 2>&1
    declare -F cap_redact >/dev/null 2>&1 || exit 1
    printf '%s\n' "$CONTROL_LINE" | cap_redact 2>/dev/null | grep -qF -- "$CONTROL_NEEDLE" && exit 1
    exit 0
  )
}

uncovered="" broken=""
while read -r n; do
  line="$(sed -n "${n}p" "$CAPLIB")"
  if ! neutralise "$n"; then
    broken="$broken
$line"
    continue
  fi
  # The control rule's own turn is the one iteration where the control cannot
  # be used, so it is skipped rather than reported as broken.
  case "$line" in
    *AKIA*) ;;
    *)
      if ! alive_under "$WORK/mutant.sh"; then
        broken="$broken
$line"
        continue
      fi
      ;;
  esac
  if [ -z "$(leaks_under "$WORK/mutant.sh")" ]; then
    uncovered="$uncovered
$line"
  fi
done < <(grep -n '^    -e "s' "$CAPLIB" | cut -d: -f1)

if [ -z "$broken" ]; then
  t_ok "every mutant still had a working cap_redact, so each result means something"
else
  t_no "neutralising these left cap_redact broken, so their coverage is UNKNOWN:"
  printf '%s\n' "$broken" | sed 's/^/    /'
fi

if [ -z "$uncovered" ]; then
  t_ok "deleting ANY one rule makes the corpus fail, so every rule is tested"
else
  t_no "these rules can be deleted and the corpus still passes:"
  printf '%s\n' "$uncovered" | sed 's/^/    /'
  printf '     Each is covered only by a broader rule. Add a corpus line that\n'
  printf '     ONLY this rule catches - bare, outside a URL and outside a header.\n'
fi

t_summary
