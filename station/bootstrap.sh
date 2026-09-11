#!/usr/bin/env bash
# =============================================================================
#  bootstrap.sh  -  drop the heliograph toolkit into a transport repo
# =============================================================================
#     bootstrap.sh <target-dir> [--flavour bash|powershell|both]
#
#  Copies start.sh, run.sh, station.sh, caprun.sh, caplib.sh, secret.sh, lib/,
#  steps/, station/, ops-logs/, secrets/ and TASK.md into <target-dir>, and installs
#  the toolkit's gitignore as <target-dir>/.gitignore.
#
#  --flavour CHOOSES THE PAYLOAD, and bash is the default. A machine gets one:
#  an estate with bash runs the bash station, because one implementation is
#  better than two wherever there is a choice, and the PowerShell payload is for
#  the estate that has none at all. `both` is for a repo serving two machines of
#  different kinds, not a recommendation - and because nothing here is
#  overwritten, the flavour planted FIRST supplies the files they share.
#
#  THE TARGET SHOULD BE ITS OWN PRIVATE REPO, not this one and not a repo that
#  holds anything else. Captured logs are committed and pushed - that is how a
#  run escapes a machine nobody can reach - so whatever the operator's commands
#  print ends up in that repo's history permanently. A transport repo is
#  cheap to create and cheap to delete; a shared one is neither.
#
#  Nothing here is overwritten silently: an existing file is left alone and
#  reported, so re-running this to pick up a newer toolkit is safe.
# =============================================================================
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET=""
FLAVOUR="bash"

while [ $# -gt 0 ]; do
  case "$1" in
    --flavour)
      [ $# -ge 2 ] || { echo "--flavour needs a value: bash, powershell or both" >&2; exit 2; }
      FLAVOUR="$2"; shift ;;
    --flavour=*) FLAVOUR="${1#--flavour=}" ;;
    -h|--help) sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) echo "unknown option: $1" >&2; exit 2 ;;
    *)
      [ -z "$TARGET" ] || { echo "give one target directory, not two" >&2; exit 2; }
      TARGET="$1" ;;
  esac
  shift
done

case "$FLAVOUR" in
  bash)       ROOTS="bash" ;;
  powershell) ROOTS="powershell" ;;
  # The order matters and is the same one `heliograph bootstrap` uses: nothing
  # is overwritten, so whichever is first supplies the files the two share.
  both)       ROOTS="bash powershell" ;;
  *) echo "unknown --flavour '$FLAVOUR': use bash, powershell or both" >&2; exit 2 ;;
esac

if [ -z "$TARGET" ]; then
  echo "usage: $0 <target-dir> [--flavour bash|powershell|both]" >&2
  echo "  the target should be a fresh, PRIVATE repo - captured logs are committed to it" >&2
  exit 2
fi

mkdir -p "$TARGET" || exit 1
TARGET="$(cd "$TARGET" && pwd)"

for r in $ROOTS; do
  [ -d "$HERE/$r" ] || { echo "this checkout carries no '$r' payload at $HERE/$r" >&2; exit 2; }
  if [ "$TARGET" = "$(cd "$HERE/$r" && pwd)" ]; then
    echo "refusing to bootstrap the toolkit over itself" >&2
    exit 2
  fi
done

copied=0
skipped=0

# Walk the toolkit rather than `cp -R` the directory: this is what lets an
# existing file be reported instead of clobbered, and it is the case that
# matters - the second run of this script is usually an upgrade over a repo
# that already has a task in flight.
for r in $ROOTS; do
  SRC="$(cd "$HERE/$r" && pwd)"
  # Named only when there is more than one, so the ordinary single-flavour
  # output is byte for byte what it has always been - `heliograph bootstrap`
  # mirrors it line for line and a test compares the two.
  prefix=""
  [ "$ROOTS" = "bash powershell" ] && prefix="$r: "
  while IFS= read -r rel; do
    dest="$TARGET/$rel"
    # The toolkit ships its ignore and attributes files without a leading dot so
    # that they apply to the transport repo and not to the skill repo carrying
    # them. Restore the dot here.
    #
    # .gitattributes matters most on a repo that will be cloned on Windows: it
    # pins the working tree to LF whatever core.autocrlf says, and without it a
    # sourced caplib.sh loses `set -uo pipefail` without stopping. See the header
    # of toolkit/gitattributes.
    [ "$rel" = "gitignore" ] && dest="$TARGET/.gitignore"
    [ "$rel" = "gitattributes" ] && dest="$TARGET/.gitattributes"
    if [ -e "$dest" ]; then
      echo "  exists, left alone : $prefix${dest#$TARGET/}"
      skipped=$((skipped + 1))
      continue
    fi
    mkdir -p "$(dirname "$dest")"
    cp -p "$SRC/$rel" "$dest"
    echo "  installed          : $prefix${dest#$TARGET/}"
    copied=$((copied + 1))
  # THE SECOND `-o` GROUP IS A STATION'S OWN RUNTIME STATE, and it is not
  # tidiness. Every one of those files is written by a running station, is local
  # to one machine, and is in the payload's .gitignore because it is nobody
  # else's - so planting one hands a brand-new transport repo the last
  # machine's lock pid, its approved step hashes, or, worst, .station-env,
  # which holds a token.
  #
  # It is `find` that needs telling, because find does not read .gitignore: a
  # checkout that has ever run a station or its tests has these sitting there,
  # invisible to `git status`. Measured on exactly such a checkout, where
  # .station-delivery was being planted into every repo bootstrapped from it.
  # internal/bootstrap prunes the same list, and station/embed_test.go refuses
  # to let them into the binary in the first place.
  done < <(cd "$SRC" && find . \
    \( -name '.terraform' -o -name '.git' -o -name 'node_modules' \) -prune -o \
    \( -name '.station.lock' -o -name '.station-state' \
       -o -name '.station-approved' -o -name '.station-approved-ps' \
       -o -name '.station-delivery' -o -name '.station-env' \
       -o -name '.station-relay-state' -o -name '.agent-service.pid' \
       -o -name '.station-service.log' \) -prune -o \
    -type f -print | sed 's|^\./||' | sort)
done

# The prune is not tidiness. `terraform init` drops a provider binary into
# .terraform/ next to each template, and azurerm alone is over 300MB. Without
# this, a bootstrapped transport repo went from about 200KB to 913MB, and that
# repo gets cloned on a locked-down control node, sometimes over a link that is
# the reason this tool exists. Measured, not guessed.

echo
echo "heliograph: $copied file(s) installed, $skipped left alone, in $TARGET"

if [ ! -d "$TARGET/.git" ]; then
  echo
  echo "$TARGET is not a git repository yet, and git is the transport. Next:"
  echo "  cd $TARGET && git init && git add -A && git commit -m 'heliograph: transport repo'"
  echo "  then add a PRIVATE remote and push."
fi

echo
case "$FLAVOUR" in
  powershell) echo "Then, on the control node: git clone <remote> && .\\run.ps1 env" ;;
  both)       echo "Then, on the control node: git clone <remote> && ./run.sh env   (or .\\run.ps1 env)" ;;
  *)          echo "Then, on the control node: git clone <remote> && ./run.sh env" ;;
esac
