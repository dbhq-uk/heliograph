#!/usr/bin/env bash
# =============================================================================
#  transports/share.sh - a directory both sides can see
# =============================================================================
# Sourced by station.sh. The cheapest transport there is, and it covers a real
# population: an estate that will not open an egress path, will not provision a
# storage account and will not permit a git host quite often already has a share
# both machines mount, because that is how everything else in the estate moves
# files.
#
# THE CONTROL SIDE HAS EXISTED SINCE A3 AND THIS DID NOT. `heliograph init
# --transport share` wrote requests into a directory nothing on the far side
# could read, so the pairing could not work at all - and the transports page
# said "works" in a single status column, which is what let that stand. Both
# halves are now here, and the page names both.
#
# THE LAYOUT IS THE CONTROL SIDE'S, not this file's invention. internal/
# transport/share.go decides it and this reads and writes exactly those paths:
#
#   <SHARE_DIR>/<SHARE_SCOPE>/request          the control side writes it
#   <SHARE_DIR>/<SHARE_SCOPE>/status           this side writes it
#   <SHARE_DIR>/<SHARE_SCOPE>/ops-logs/*.txt   this side writes them
#
# THE MOUNT IS THE CREDENTIAL, and that is the whole security model. There is no
# token, no signature and no verification: anyone who can write to the share can
# queue a step, and this station will run it under the same gates as any other
# request. So the share must be scoped as tightly as the account the station
# runs as, and the preflight says so out loud rather than leaving it implied.
#
# WHAT THIS TRANSPORT CANNOT DO
#
# It cannot self-update. `heliograph plant` refuses every non-git transport with
# "nothing for the far side to clone", so nothing publishes a payload to a
# share, and a `self` capability would advertise a verb with no source behind
# it. Declaring one you cannot honour is worse than not having it: the loop
# calls what is declared. A station on this transport is changed by re-planting.
# =============================================================================

# `live` IS offered, and git's reason for keeping it separate does not apply.
#
# On git a live read has to avoid pulling, because the step is appending to its
# log through an open descriptor and a rebase would rewrite the file underneath
# it. There is no working tree here and no rebase: reading the request is a file
# read either way, so the live verb is the ordinary verb and costs nothing. That
# makes a cancel reach a running step within one poll, which on a share is the
# cheapest win available.
tp_capabilities() { printf 'request status progress live\n'; }

tp_init() {
  # CLEARED FIRST, not merely assigned at the end. Everything below can return
  # early, and tp_preflight is contracted to run after a FAILED tp_init - so a
  # SHARE_BASE inherited from the environment would be read by the preflight as
  # though tp_init had resolved it, and probed. It is this function's variable
  # and nobody else's.
  SHARE_BASE=""
  SHARE_BASE_EXISTS=no

  # cap_need rather than `${VAR:?}`, which exits the SHELL rather than failing
  # this function. See cap_need in caplib.sh for what that cost.
  cap_need SHARE_DIR   "the directory both sides mount" || return 1
  cap_need SHARE_SCOPE "the scope, which is what a run is bound to - one directory per investigation" || return 1

  # ONE PATH COMPONENT, AND A NARROW ONE.
  #
  # It becomes a directory name under a path this station did not choose, so
  # `SHARE_SCOPE=../..` would put every status and every log outside the share
  # entirely - somewhere the control side will never look and nobody will think
  # to clean up.
  #
  # But refusing separators is not enough, because the scope is not only a path.
  # It is published as the status document's `branch:` value, and that document
  # is line-oriented `key: value`. A scope containing a NEWLINE injects a second
  # key, and the control side's parser keeps the last one - so
  # `SHARE_SCOPE=$'x\nstate: running'` makes an idle station report itself busy
  # for ever. A whitelist rather than a blacklist: letters, digits, dot, hyphen
  # and underscore is every scope anybody would write.
  case "$SHARE_SCOPE" in
    ''|.|..|-*|*[!A-Za-z0-9._-]*)
      echo "station: '$SHARE_SCOPE' is not a usable SHARE_SCOPE." >&2
      echo "         It becomes one directory name AND the status document's" >&2
      echo "         branch field, so it may hold only letters, digits, dot," >&2
      echo "         hyphen and underscore, and may not begin with a hyphen." >&2
      return 1 ;;
  esac

  [ -d "$SHARE_DIR" ] || {
    echo "station: the share at $SHARE_DIR is not there." >&2
    echo "         Mount it first - this transport creates nothing above the" >&2
    echo "         scope directory, deliberately: creating a missing mount" >&2
    echo "         point turns 'the share is not mounted' into a station that" >&2
    echo "         publishes into local disk nobody else can see." >&2
    return 1
  }

  SHARE_BASE="$SHARE_DIR/$SHARE_SCOPE"

  # A SYMLINK IS REFUSED, and the reason is not only the obvious one.
  #
  # The obvious one: anybody who can write to the share can point the scope at
  # a station-local directory and take the logs somewhere the control side
  # never looks.
  #
  # The one that will actually happen: on NFS or SMB a symlink is resolved by
  # each client separately, so `dns-timeouts -> /var/tmp/dns-timeouts` resolves
  # to two DIFFERENT local directories on the two machines. Both sides then work
  # perfectly and never meet, which is this transport's signature failure and
  # the hardest one to see.
  if [ -L "$SHARE_BASE" ] || [ -L "$SHARE_BASE/ops-logs" ]; then
    echo "station: $SHARE_BASE is a symlink, or contains one at ops-logs." >&2
    echo "         Refused: a symlink on a share is resolved by each client" >&2
    echo "         separately, so the two sides can follow it to different" >&2
    echo "         directories and never meet. Use a real directory." >&2
    return 1
  fi

  # Recorded, NOT created. `./start.sh --check` promises to change nothing, and
  # an operator runs it on a node where they are not yet permitted to alter
  # anything - so a tp_init that made directories would break that promise for
  # every share station, and would do it silently. The publish path creates what
  # it needs, at the moment it needs it.
  #
  # Whether the scope was already there is worth saying out loud, because a
  # scope directory that does not exist yet is the share's version of being on
  # the wrong branch.
  [ -d "$SHARE_BASE" ] && SHARE_BASE_EXISTS=yes

  # Litter from a station that was killed between writing a temporary and
  # renaming it. Unpredictable names mean they cannot collide, only accumulate,
  # and this is a directory people browse. An hour is far longer than any
  # publish takes and far shorter than anybody would keep one.
  if [ "$SHARE_BASE_EXISTS" = yes ]; then
    find "$SHARE_BASE" -maxdepth 2 -name "$SHARE_TMP_PREFIX*" -mmin +60 \
      -exec rm -f {} + 2>/dev/null || true
  fi
  return 0
}

tp_scope() { printf '%s' "$SHARE_SCOPE"; }

tp_revision() { printf 'share %s, scope %s' "$SHARE_DIR" "$SHARE_SCOPE"; }

tp_describe() {
  printf 'file share %s, scope %s, credential: the mount itself' \
    "$SHARE_DIR" "$SHARE_SCOPE"
}

# --- writing without ever being read half-written ----------------------------
# Write-then-rename, into the SAME directory, for every file this side
# publishes.
#
# The control side reads these at an instant it chooses, and a partially written
# status is a status with no `state:` yet. Worse, a half-copied log looks
# exactly like a step that hung: it ends mid-line with no footer, which is the
# one shape an operator is trained to read as "still running". Rename within a
# directory is atomic on every filesystem worth running this on.
#
# THE TEMPORARY MUST SHARE THE DIRECTORY, not live in /tmp. A rename across
# filesystems is not a rename - it degrades to copy-then-unlink, which is
# exactly the non-atomic write this exists to avoid, and a share is by
# definition a different filesystem from /tmp.
#
# AND ITS NAME MUST BE UNPREDICTABLE. It was `$dst.$$.tmp`, and a PID is neither
# unique nor unguessable:
#
#   - two stations in two containers commonly have the SAME pid, so both open
#     one temporary, one renames it while the other is still writing, and the
#     destination is visible while it is still changing. That is precisely the
#     torn read this function exists to prevent, reintroduced by its own
#     temporary
#   - anybody who can write to the share can plant `status.<pid>.tmp` as a
#     symlink to a station-local file, and the next publish truncates whatever
#     it points at. Being able to queue a gated step is the documented
#     capability of a share writer; truncating arbitrary station files is not
#
# `mktemp` creates with O_EXCL and mode 600, which answers both: the name cannot
# be guessed and an existing symlink cannot be followed. The mode is then opened
# up to 0644 because the control side may well be a different user on a share,
# and a log nobody can read is a log nobody has.
SHARE_TMP_PREFIX=".heliograph-tmp."

_share_tmp() {  # _share_tmp <destination> - a safe temporary beside it
  local dst="$1" dir
  dir="$(dirname -- "$dst")"
  mkdir -p "$dir" 2>/dev/null || return 1
  mktemp "$dir/$SHARE_TMP_PREFIX$(basename -- "$dst").XXXXXX" 2>/dev/null
}

# `mv` INTO AN EXISTING DIRECTORY SUCCEEDS, and puts the file inside it. So
# `mv tmp status` where `status` is somehow a directory returns 0 having
# published nothing, and the station reports a delivery the control side cannot
# see. `mv -T` says what is meant and is GNU-only, so the case is refused
# explicitly instead.
_share_rename() {  # _share_rename <tmp> <destination>
  local tmp="$1" dst="$2"
  if [ -d "$dst" ]; then
    say "$dst is a directory, so publishing there would put the file INSIDE it."
    rm -f -- "$tmp"
    return 1
  fi
  chmod 0644 -- "$tmp" 2>/dev/null
  mv -f -- "$tmp" "$dst" 2>/dev/null || { rm -f -- "$tmp"; return 1; }
  return 0
}

_share_publish() {  # _share_publish <source-file> <destination>
  local src="$1" dst="$2" tmp
  tmp="$(_share_tmp "$dst")" || return 1
  [ -n "$tmp" ] || return 1
  cp -- "$src" "$tmp" 2>/dev/null || { rm -f -- "$tmp"; return 1; }
  _share_rename "$tmp" "$dst"
}

_share_publish_body() {  # _share_publish_body <text> <destination>
  local body="$1" dst="$2" tmp
  tmp="$(_share_tmp "$dst")" || return 1
  [ -n "$tmp" ] || return 1
  printf '%s' "$body" > "$tmp" 2>/dev/null || { rm -f -- "$tmp"; return 1; }
  _share_rename "$tmp" "$dst"
}

# PROVE THE WRITE, and prove the WHOLE write.
#
# A share mounted read-only, and one whose server has gone away leaving a stale
# handle, both stat perfectly and fail on the first write - which would be the
# log, an hour later, with nobody left to tell.
#
# CREATE, RENAME, THEN REMOVE, because publishing needs all three and an ACL can
# grant them separately. An SMB share that permits create and write but denies
# rename or delete - which is an ordinary way to configure a drop box - passes a
# check that only writes a file, and then fails on every single publication at
# the `mv`. So this does exactly what _share_publish_body does, and then undoes
# it.
#
# IT CREATES NO DIRECTORY. `./start.sh --check` promises to change nothing, so
# this probes the deepest directory that already exists and says which one that
# was, rather than making the scope in order to test it.
_share_probe() {  # _share_probe <directory> - create, rename, remove, or fail
  local dir="$1" tmp dst
  dst="$dir/.heliograph-write-check"
  [ -d "$dst" ] && { say "$dst exists and is a directory"; return 1; }
  tmp="$(mktemp "$dir/$SHARE_TMP_PREFIX""check.XXXXXX" 2>/dev/null)" || return 1
  [ -n "$tmp" ] || return 1
  printf 'heliograph write check\n' > "$tmp" 2>/dev/null || { rm -f -- "$tmp"; return 1; }
  mv -f -- "$tmp" "$dst" 2>/dev/null || { rm -f -- "$tmp"; return 1; }
  rm -f -- "$dst" 2>/dev/null || {
    say "wrote and renamed a probe in $dir but could not remove it: $dst"
    say "  Delete is what a station needs to tidy its own temporaries."
    return 1
  }
  return 0
}

tp_check() {
  local dir="$SHARE_DIR" what="the share root"
  if [ "${SHARE_BASE_EXISTS:-no}" = yes ]; then
    dir="$SHARE_BASE"; what="the scope directory"
    # ops-logs is where every log goes, and it can have permissions of its own:
    # a writable scope containing a mode-0555 ops-logs passes a scope-only check
    # and then rejects every log there has ever been. Probed only when it is
    # already there, because creating it would change the machine.
    if [ -d "$SHARE_BASE/ops-logs" ] && ! _share_probe "$SHARE_BASE/ops-logs"; then
      say "cannot publish into $SHARE_BASE/ops-logs, which is where every log goes."
      return 1
    fi
  fi
  _share_probe "$dir" || {
    say "cannot publish into $dir ($what)."
    say "  A read-only mount, a stale NFS or SMB handle, and an ACL that allows"
    say "  create but denies rename or delete all look exactly like this."
    say "  The station would capture logs it could not deliver."
    return 1
  }
  return 0
}

# What only a share can be wrong about, beyond "can I write to it".
#
# The seams are start.sh's and documented in transports/git.sh: `report`, and
# TP_WANT_SCOPE for a scope named on the command line. It must be safe to call
# after a FAILED tp_init, so everything here is guarded.
tp_preflight() {
  report ok share "${SHARE_DIR:-<unset>}, scope ${SHARE_SCOPE:-<unset>}"

  # --branch is git's word for this, and start.sh has already refused it on any
  # other transport. Said here as well, because an operator who reached for it
  # wants to know which variable to reach for instead.
  [ -n "${TP_WANT_SCOPE:-}" ] &&
    report warn scope "the scope of a share station is SHARE_SCOPE, set before this script runs. It is currently '${SHARE_SCOPE:-<unset>}'"

  # EVERY variable below is tp_init's, and tp_init may have failed - start.sh
  # calls this afterwards on purpose, so that one trip names every blocker. So
  # nothing here may assume tp_init got as far as assigning anything, and
  # SHARE_BASE in particular is cleared at the top of tp_init rather than merely
  # set at the bottom: inherited from the environment it would send this
  # function probing a directory nobody configured.
  [ -n "${SHARE_DIR:-}" ] && [ -n "${SHARE_BASE:-}" ] || return 0

  # THE SILENT FAILURE THIS TRANSPORT HAS AND THE OTHERS DO NOT.
  #
  # A typo in SHARE_SCOPE is not an error: the publish path simply creates that
  # directory the first time it writes. Both sides then run perfectly, for ever,
  # publishing into two directories that never meet - and every symptom points
  # at the station being asleep. git cannot do this: a branch that does not
  # exist is refused by the remote.
  if [ "${SHARE_BASE_EXISTS:-no}" = "yes" ]; then
    report ok scope "$SHARE_BASE is already there, so something has used this scope before"
  else
    report warn scope "$SHARE_BASE does not exist yet and will be created by the first thing this station publishes. If the control side is using a different scope, both sides will run perfectly and never meet. Check it against 'heliograph estates' on the control node"
  fi

  # The mount IS the credential, so who else can write to it is the whole
  # access-control question, and it is one a station can actually answer about
  # itself. `ls -ld` rather than `stat`: stat's format flag is spelled
  # differently on GNU and BSD, and this has to run on whatever is there.
  local mode
  mode="$(ls -ld -- "$SHARE_DIR" 2>/dev/null | cut -c1-10)"
  case "$mode" in
    # `drwxrwxrwx` - the type, then three triplets. The OTHER-write bit is the
    # NINTH character, so the pattern is eight of anything, a `w`, then one
    # more: `drwxrwxrwt` matches, `drwxr-xr-x` does not.
    ????????w?)
      report warn share "$SHARE_DIR is world-writable ($mode). The mount is the only credential this transport has, so anyone on this machine can queue a step for this station, and it will run under the same gates as anything else. Scope the share as tightly as the account the station runs as" ;;
    ??????????)
      report ok share "permissions $mode" ;;
    *)
      # Not a failure. The write probe below is what settles whether the share
      # is usable; this line only ever adds context.
      report warn share "cannot read the permissions of $SHARE_DIR, so who else may queue a step for this station is unknown. The mount is this transport's only credential" ;;
  esac

  if tp_check; then
    report ok "share write" "a file created, renamed and removed again - which is exactly what publishing does"
  else
    report FAIL "share write" "cannot publish into the share. A read-only mount, a stale NFS or SMB handle, and an ACL that allows create but denies rename or delete all look exactly like this, and the station would capture logs it could not deliver. Check the mount, and that $(whoami 2>/dev/null || echo 'this account') may write, rename and delete there"
  fi
}

# Bring the payload up to date: there is nothing to bring. tp_sync is not
# defined, which is how a transport says so - see start.sh.

# The ordinary poll.
#
# ABSENT AND UNREADABLE ARE DIFFERENT, and collapsing them is how a station goes
# permanently deaf without anybody being told. An absent request is the ordinary
# state of a station that has just started, and reporting it as a fetch failure
# would make a quiet scope look like a flapping link for ever. A request that is
# THERE and cannot be read - a permissions change, a stale NFS handle, an I/O
# error, or `request` having somehow become a directory - is a real failure, and
# the loop counts those and says so.
#
# `cat || printf ''` was the first version and it reported every one of those as
# an empty queue: the station would poll a share it could no longer read, for
# ever, reporting itself idle.
tp_fetch_request() {
  local req="$SHARE_BASE/request"
  [ -e "$req" ] || { printf ''; return 0; }
  cat -- "$req" 2>/dev/null || {
    say "the request at $req is there and cannot be read."
    return 1
  }
  return 0
}

# The same read. There is no working tree here, so nothing to disturb.
tp_fetch_request_live() { tp_fetch_request; }

# Cannot self-update: nothing publishes a payload to a share. Declared in
# tp_capabilities so the loop never calls it, and defined so that calling it
# would be a refusal rather than "command not found".
tp_fetch_self() { return 1; }

tp_put_status() {
  local body="$1" _msg="$2" alsofile="${3:-}"
  _share_publish_body "$body" "$SHARE_BASE/status" || return 1
  # The third argument is a partial log from a cancelled run. git commits it
  # alongside the status so the cancellation is not stranded behind a dirty
  # tree; here it is simply the last thing the far side will see of that run,
  # and losing it is losing the only evidence there is.
  if [ -n "$alsofile" ] && [ -f "$alsofile" ]; then
    _share_publish "$alsofile" "$SHARE_BASE/ops-logs/$(basename -- "$alsofile")" || return 1
  fi
  return 0
}

# The partial log goes under its FINAL name, deliberately.
#
# The blob transport puts progress under a fixed `log` blob and the finished log
# under `logs/<name>`, because there the two are different objects. Here the
# control side lists one directory and reads by name, so publishing progress
# under the same name means a reader following a long step sees it grow and then
# be completed in place - rather than finding a stale snapshot beside the real
# thing and having to know which is which.
tp_put_progress() {
  local body="$1" _msg="$2" logfile="$3"
  _share_publish_body "$body" "$SHARE_BASE/status" || return 1
  [ -f "$logfile" ] || return 0
  _share_publish "$logfile" "$SHARE_BASE/ops-logs/$(basename -- "$logfile")"
}

# Deliver the finished log. The message is ignored: it is a git commit subject,
# and there is no history here to carry it.
tp_put_log() {
  local logfile="$1"
  [ -f "$logfile" ] || return 1
  _share_publish "$logfile" "$SHARE_BASE/ops-logs/$(basename -- "$logfile")"
}
