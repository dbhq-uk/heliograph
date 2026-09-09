#!/usr/bin/env bash
# =============================================================================
#  drivers/bash.sh - the current bash toolkit, under the conformance contract
# =============================================================================
# Sourced by conformance.sh. Implements drv_* and nothing else. All knowledge
# of caplib.sh, run.sh and bootstrap.sh lives here, so the suite itself stays a
# specification rather than a second copy of this implementation.
#
# ONE DRIVER, SEVERAL TRANSPORTS, chosen by CONF_TRANSPORT. Properties 1 to 8
# are about the captured file and do not touch a transport at all; property 9
# is about DELIVERY and is the only one that differs. So the transport-specific
# part is three functions - stand up a far side, configure the station, read
# back what arrived - and everything else is shared.
#
# That split is what the property was missing. p9 exercised git alone, so a
# transport whose `tp_put_log` did nothing whatsoever would have passed the
# whole suite: the log is written correctly to local disk on every transport,
# and properties 1 to 8 only ever look at local disk.
# =============================================================================

_D_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_D_TOOLKIT="$(cd "$_D_HERE/../../../station/bash" && pwd)"
_D_ROOT="$(cd "$_D_HERE/../../.." && pwd)"
_D_BOOTSTRAP="$_D_ROOT/station/bootstrap.sh"
_D_STUB="$_D_HERE/../relay-stub.py"

CONF_TRANSPORT="${CONF_TRANSPORT:-git}"

# The relay's constants, in one place because the stub, the station's env and
# the control-side read all have to agree on them.
_D_RELAY_ESTATE=conformance
_D_RELAY_CTL=control-token
_D_RELAY_STN=station-token
_D_RELAY_BASES=""

drv_name() { printf 'bash toolkit (caplib.sh, run.sh) over %s' "$CONF_TRANSPORT"; }

drv_supports() {
  case "$1" in
    capture|gates) return 0 ;;
    # A CANCEL CAN ONLY KEEP A PARTIAL LOG WHERE sed HAS -u.
    #
    # cap_run stamps each line before any sed runs, so the timestamps are
    # honest either way. Redaction is a sed stage and cannot be skipped or
    # reordered - publishing unredacted output to a log that gets committed is
    # not a trade available at any price - so where sed buffers, a run killed
    # mid-flight loses whatever was in that buffer. That is busybox, and it is
    # this implementation's fact to report rather than the specification's to
    # assume.
    cancel) printf 'x\n' | sed -u 's/x/y/' >/dev/null 2>&1 ;;
    deliver) _drv_deliverable ;;
    *) return 1 ;;
  esac
}

# Whether THIS transport can be delivered over from here.
#
# A transport that cannot is skipped by name and reason rather than silently
# passing, because "p9 was not run" and "p9 passed" are the two things this
# property exists to keep apart.
_drv_deliverable() {
  case "$CONF_TRANSPORT" in
    git|share) return 0 ;;
    relay)
      # heliograph-seal is Go, and the seal is not optional on this transport:
      # a station configured for the relay either speaks sealed or does not
      # speak. So no Go toolchain means no relay conformance, said out loud.
      command -v go >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1
      ;;
    *) return 1 ;;
  esac
}

# --- the step fixtures --------------------------------------------------------
# THE SUITE DOES NOT WRITE STEPS ANY MORE, and this was the last piece of Unix
# left in it. Every fixture was a heredoc beginning `#!/usr/bin/env bash`, so a
# PowerShell driver could not have run a single property: the specification was
# describing behaviour in one implementation's language.
#
# The suite names a step by KIND and the driver writes it. Nine kinds, each
# doing exactly what its name says, and a driver that cannot express one of
# them has no business claiming to implement the capture.
#
#   three-slow  three lines, a second apart
#   gap         two lines, three seconds apart
#   rc42        one line, then exit 42
#   rc0         one line, exit 0
#   slow        ten lines a second apart, so a cancel lands mid-run
#   leaky       written by the suite from the redaction corpus, not here
#   undeclared  declares no mode - the gate must refuse it
#   declared    declares read-only, and TOUCHES A MARKER given as $3
#   ships       declares read-only, prints evidence, exits 7
#   messy       ANSI, a CRLF line, a line on stderr, and a final line with no
#               newline after it
drv_step_name() { printf 'steps/%s.sh' "$1"; }

# drv_step_echo <path-without-extension> <file-of-lines> - a step that prints
# each line of the file verbatim and nothing else.
#
# `printf %s` with the line as an ARGUMENT, never as the format: the redaction
# corpus contains % and \ by design, and putting it in a format string would
# let the fixture rewrite itself on the way through.
drv_step_echo() {
  local path="$1.sh" lines="$2" l
  {
    printf '#!/usr/bin/env bash\n'
    while IFS= read -r l; do
      printf 'printf "%%s\\n" %s\n' "$(printf "'%s'" "$(printf '%s' "$l" | sed "s/'/'\\\\''/g")")"
    done < "$lines"
  } > "$path"
  chmod +x "$path"
}

drv_step_file() {  # drv_step_file <kind> <path-without-extension> [marker]
  local kind="$1" path="$2.sh" marker="${3:-}"
  case "$kind" in
    three-slow)
      printf '#!/usr/bin/env bash\necho first; sleep 1.1; echo second; sleep 1.1; echo third\n' > "$path" ;;
    gap)
      printf '#!/usr/bin/env bash\necho before; sleep 3; echo after\n' > "$path" ;;
    rc42)
      printf '#!/usr/bin/env bash\necho working\nexit 42\n' > "$path" ;;
    rc0)
      printf '#!/usr/bin/env bash\necho working\n' > "$path" ;;
    slow)
      printf '#!/usr/bin/env bash\necho starting the long probe\nfor i in 1 2 3 4 5 6 7 8 9 10; do echo "probe $i"; sleep 1; done\necho finished\n' > "$path" ;;
    undeclared)
      printf '#!/usr/bin/env bash\necho this step declares nothing\n' > "$path" ;;
    declared)
      {
        printf '#!/usr/bin/env bash\n'
        printf '# heliograph-mode: read-only\n'
        printf 'echo this step declares itself and measures nothing\n'
        [ -n "$marker" ] && printf ': > %s\n' "$(printf "'%s'" "$marker")"
      } > "$path" ;;
    ships)
      printf '#!/usr/bin/env bash\n# heliograph-mode: read-only\necho the evidence\nexit 7\n' > "$path" ;;
    messy)
      # RAW printf, never echo: the point is the exact bytes. The last line has
      # no newline after it deliberately.
      {
        printf '#!/usr/bin/env bash\n'
        printf "printf '\\\\033[1;31mred line\\\\033[0m\\\\n'\n"
        printf "printf 'crlf line\\\\r\\\\n'\n"
        printf "printf 'to stderr\\\\n' >&2\n"
        printf "printf 'no trailing newline'\n"
      } > "$path" ;;
    *) return 1 ;;
  esac
  chmod +x "$path"
}

# Capture in a subshell so caplib's globals never leak between properties.
drv_capture() {
  local out="$1" script="$2.sh"
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

# Bootstrap a payload, WITH somewhere for a delivery to land.
#
# The far side is not scenery. Property 9 asks whether a finished log reached
# it, and the only honest way to answer that is to read it back from the other
# end rather than from the working tree that wrote it. A station with no far
# side would let a delivery that never happened look identical to one that did,
# which is precisely the defect p9 exists to catch.
drv_bootstrap() {
  local dir="$1"
  "$_D_BOOTSTRAP" "$dir" >/dev/null 2>&1 || return 1
  _drv_farside "$dir"
}

drv_step() {
  local dir="$1" step="$2"
  ( cd "$dir" && PUSH=0 ./run.sh "$step" ) >/dev/null 2>&1
}

# Run a step and let it DELIVER. No PUSH=0 here, deliberately: the delivery is
# the thing under test. The transport is passed in the environment, which is
# where run.sh reads it from, so nothing here has to write a .station-env.
drv_deliver() {
  local dir="$1" step="$2"
  ( cd "$dir" && _drv_env "$dir" && ./run.sh "$step" ) >/dev/null 2>&1
}

drv_delivered() { _drv_read "$1"; }

# Start a capture in its own process group, so a cancel can signal the whole
# group the way station.sh does rather than only the wrapper.
#
# THE PID GOES TO A FILE, not to stdout. Printing it meant the caller ran this
# in a command substitution, which put the background process under a subshell
# nobody could `wait` for - so a cancel that failed was indistinguishable from
# one that worked, and the suite carried on after a fixed sleep. The handle
# file also gives a Windows driver somewhere to put a job object.
#
# The process group id is what is written, negated at kill time by drv_cancel.
# setsid makes the child a group leader, so its pid IS the group.
drv_capture_bg() {
  local out="$1" script="$2.sh" handle="$3"
  setsid bash -c '
    # shellcheck disable=SC1091
    . "$1/caplib.sh"
    cap_header "$2" "conformance-cancel"
    cap_run "$2" "$3"
    cap_footer "$2" $?
  ' _ "$_D_TOOLKIT" "$out" "$script" >/dev/null 2>&1 &
  printf '%s' "$!" > "$handle"
}

# Cancel it, and PROVE it is gone.
#
# A kill that is merely sent proves nothing: a capture that ignored the signal
# would keep writing into a directory the suite is about to delete, and every
# assertion after it would be racing. So this waits, escalates, and returns
# non-zero if the run is still there - which is a finding, not a tidy-up.
drv_cancel() {
  local handle="$1" pid waited=0
  pid="$(cat "$handle" 2>/dev/null)" || return 1
  [ -n "$pid" ] || return 1

  # The GROUP first, the way station.sh cancels: the step is a child of the
  # capture, and signalling only the wrapper leaves the step running.
  kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null

  while kill -0 "$pid" 2>/dev/null; do
    waited=$((waited + 1))
    if [ "$waited" -gt 50 ]; then
      kill -KILL -- "-$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null
      sleep 0.5
      kill -0 "$pid" 2>/dev/null && return 1
      break
    fi
    sleep 0.1
  done
  wait "$pid" 2>/dev/null
  return 0
}

# Run a declared step as the platform's PRIVILEGED account, without being one.
#
# A fake `id` answering 0 to `id -u`, deferring to the real one otherwise.
# caplib's cap_refuse_root calls `id -u` rather than reading $EUID precisely so
# this is possible - that seam is documented, and a PowerShell station will
# need one of its own for S-1-5-18.
drv_step_privileged() {
  local dir="$1" step="$2" fakebin real_id
  fakebin="$(mktemp -d)" || return 1
  real_id="$(command -v id)"
  cat > "$fakebin/id" <<EOF
#!/usr/bin/env bash
[ "\$*" = "-u" ] && { echo 0; exit 0; }
exec "$real_id" "\$@"
EOF
  chmod +x "$fakebin/id"
  ( cd "$dir" && PATH="$fakebin:$PATH" PUSH=0 ./run.sh "$step" ) >/dev/null 2>&1
  local rc=$?
  rm -rf "$fakebin"
  return "$rc"
}

# --- the transport-specific three --------------------------------------------
# _drv_farside <dir>  - stand up whatever receives a delivery
# _drv_env <dir>      - export what the station needs to reach it
# _drv_read <dir>     - the body of the newest log the far side actually holds

_drv_farside() {
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
      # The share root only. NOT the scope directory under it: `tp_init` must
      # create nothing, because `--check` has to be able to run on a node where
      # nobody is permitted to alter anything yet. If this created the scope,
      # a transport that had quietly stopped creating it would still pass.
      mkdir -p "$dir.share"
      ;;
    relay)
      _drv_relay_farside "$dir"
      ;;
    *) return 1 ;;
  esac
}

_drv_env() {
  local dir="$1"
  case "$CONF_TRANSPORT" in
    git) export TRANSPORT=git ;;
    share)
      export TRANSPORT=share SHARE_DIR="$dir.share" SHARE_SCOPE=conformance
      ;;
    relay)
      # shellcheck disable=SC1090,SC1091
      . "$dir.relay/env" || return 1
      ;;
    *) return 1 ;;
  esac
}

_drv_read() {
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
      # From the SHARE, never from the working tree, for the same reason the
      # git case reads the bare remote.
      local newest
      newest="$(ls -1 "$dir.share/conformance/ops-logs/"*.txt 2>/dev/null | tail -1)"
      [ -n "$newest" ] || return 1
      cat "$newest"
      ;;
    relay) _drv_relay_read "$dir" ;;
    *) return 1 ;;
  esac
}

# --- relay: a stub server, two keypairs, and an unsealing read ----------------
# The relay is the only transport whose far side is a program. It gets one
# here, in memory, so this suite needs no Cloudflare account and no network -
# see relay-stub.py for what that stub does and, more importantly, does not do.
_drv_relay_farside() {
  local dir="$1" base="$1.relay" port pid waited=0
  mkdir -p "$base" || return 1

  # Built from THIS tree, not downloaded. The seal format and the station side
  # that speaks it are changed together, and a conformance run against last
  # release's binary would assert that an old station still works.
  ( cd "$_D_ROOT" && go build -o "$base/heliograph-seal" ./cmd/heliograph-seal ) \
    >/dev/null 2>&1 || return 1

  # Two identities, because that is the real topology: the control side signs
  # requests and the station verifies them, and vice versa for logs. One shared
  # key would pass while proving nothing about either direction.
  "$base/heliograph-seal" keygen --out "$base/station.key" >/dev/null 2>&1 || return 1
  "$base/heliograph-seal" keygen --out "$base/control.key" >/dev/null 2>&1 || return 1
  "$base/heliograph-seal" public --identity "$base/station.key" > "$base/station.pub" 2>/dev/null || return 1
  "$base/heliograph-seal" public --identity "$base/control.key" > "$base/control.pub" 2>/dev/null || return 1

  # Port 0: the OS picks, the stub prints what it got. A fixed port makes a
  # test that cannot run twice at once, and CI runs these in parallel.
  #
  # TWO TOKENS. The relay's scopes are asymmetric - a station may collect a
  # request and publish a log, and may not queue a request even for itself - so
  # the station gets the station token and the reader below gets the control
  # one. A single token would let a station-side regression that used the wrong
  # credential pass here and be refused by a real relay.
  python3 "$_D_STUB" "$_D_RELAY_CTL" "$_D_RELAY_STN" "$_D_RELAY_ESTATE" 0 \
    > "$base/port" 2>"$base/stub.err" &
  pid=$!
  echo "$pid" > "$base/pid"
  # Registered NOW, before the readiness wait. Every property that bootstraps
  # starts one of these, and a bootstrap that fails half way through would
  # otherwise leave a server holding a port for the rest of the run.
  _D_RELAY_BASES="$_D_RELAY_BASES $base"

  # HEALTH, not a number in a file. A port that parses proves the stub printed
  # something, not that it is listening - and a partially written value parses
  # as a different port entirely. So the port is read, then dialled, and the
  # process is checked for still being alive on every turn: a stub that died
  # on startup would otherwise be waited for until the timeout.
  while :; do
    port="$(cat "$base/port" 2>/dev/null)"
    case "$port" in
      '' | *[!0-9]*) ;;
      *)
        if curl -sS -o /dev/null -m 2 "http://127.0.0.1:$port/health" 2>/dev/null; then
          break
        fi
        ;;
    esac
    kill -0 "$pid" 2>/dev/null || { echo "relay stub died:" >&2; cat "$base/stub.err" >&2; return 1; }
    waited=$((waited + 1))
    [ "$waited" -gt 100 ] && return 1
    sleep 0.1
  done

  cat > "$base/env" <<EOF
export TRANSPORT=relay
export RELAY_URL=http://127.0.0.1:$port
export RELAY_ESTATE=$_D_RELAY_ESTATE
export RELAY_STATION=station
export RELAY_TOKEN=$_D_RELAY_STN
export RELAY_SEAL=$base/heliograph-seal
export RELAY_IDENTITY=$base/station.key
export RELAY_PEER=$base/control.pub
export RELAY_STATE=$base/state
EOF
  return 0
}

# Collect from the CONTROL side and unseal, which is the whole point.
#
# A GET drains the queue, so this reads what the control side would have read -
# and it opens the envelope with the control identity against the station's
# public key. A log that was sealed for somebody else, or signed by nobody,
# fails here rather than being counted as delivered.
_drv_relay_read() {
  local dir="$1" base="$1.relay" tmp out rc
  tmp="$(mktemp)" || return 1
  # THE CONTROL TOKEN, because collecting from s2c is the control side's move.
  # Using the station's here would be a reader that only works against a stub
  # with one token, which is how the asymmetry stops being tested.
  curl -sS -m 10 -H "Authorization: Bearer $_D_RELAY_CTL" \
    "$(_drv_relay_url "$base")/v1/$_D_RELAY_ESTATE/station/s2c" \
    > "$tmp" 2>/dev/null || { rm -f "$tmp"; return 1; }

  # The WHOLE array, handed over as it came. `open` sorts by sequence, applies
  # the replay floor, and takes the newest that opens as this kind - so asking
  # for `log` is what discards the `status` the station published alongside it.
  # A shell that picked a message itself would be a second implementation of
  # the replay defence, in the language least suited to it.
  out="$("$base/heliograph-seal" open \
          --identity "$base/control.key" --peer "$base/station.pub" \
          --estate "$_D_RELAY_ESTATE" --station station \
          --dir s2c --kind log --min-seq 0 --in "$tmp" 2>/dev/null)"
  rc=$?
  rm -f "$tmp"
  [ "$rc" = "0" ] || return 1
  # heliograph-seal prints the accepted sequence on the first line and the
  # document after it, so the shell never parses the envelope.
  printf '%s\n' "$out" | tail -n +2
}

# The base URL of a station's stub, for anything that has to dial it directly.
_drv_relay_url() { sed -n 's/^export RELAY_URL=//p' "$1/env"; }

# The stubs are background processes and the suite's own trap only removes the
# work directory, which would leave them running and their ports held.
#
# EVERY ONE, not the last. Several properties bootstrap, so several stubs get
# started, and tearing down only the one p9 used leaked the rest for the life
# of the shell. Takes no argument for that reason: the driver knows what it
# started and the suite does not have to.
drv_teardown() {
  local base pid waited
  for base in $_D_RELAY_BASES; do
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
    # Reaped, so it does not sit as a zombie for the rest of the run.
    wait "$pid" 2>/dev/null
  done
  _D_RELAY_BASES=""
  return 0
}
