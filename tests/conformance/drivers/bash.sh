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

drv_name() { printf 'bash toolkit (caplib.sh, run.sh) over %s' "$CONF_TRANSPORT"; }

drv_supports() {
  case "$1" in
    capture|gates|cancel) return 0 ;;
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
# group the way station.sh does rather than only the wrapper. Echoes the pid,
# which is also the process group id because setsid made it a leader.
drv_capture_bg() {
  local out="$1" script="$2"
  setsid bash -c '
    # shellcheck disable=SC1091
    . "$1/caplib.sh"
    cap_header "$2" "conformance-cancel"
    cap_run "$2" "$3"
    cap_footer "$2" $?
  ' _ "$_D_TOOLKIT" "$out" "$script" >/dev/null 2>&1 &
  printf '%s' "$!"
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
  local dir="$1" base="$1.relay" port
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
  python3 "$_D_STUB" conformance-token 0 > "$base/port" 2>"$base/stub.err" &
  echo $! > "$base/pid"
  local waited=0
  while :; do
    port="$(cat "$base/port" 2>/dev/null)"
    case "$port" in
      '' | *[!0-9]*) ;;
      *) break ;;
    esac
    waited=$((waited + 1))
    [ "$waited" -gt 100 ] && return 1
    sleep 0.1
  done

  cat > "$base/env" <<EOF
export TRANSPORT=relay
export RELAY_URL=http://127.0.0.1:$port
export RELAY_ESTATE=conformance
export RELAY_STATION=station
export RELAY_TOKEN=conformance-token
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
  curl -sS -m 10 -H "Authorization: Bearer conformance-token" \
    "$(sed -n 's/^export RELAY_URL=//p' "$base/env")/v1/conformance/station/s2c" \
    > "$tmp" 2>/dev/null || { rm -f "$tmp"; return 1; }

  # The WHOLE array, handed over as it came. `open` sorts by sequence, applies
  # the replay floor, and takes the newest that opens as this kind - so asking
  # for `log` is what discards the `status` the station published alongside it.
  # A shell that picked a message itself would be a second implementation of
  # the replay defence, in the language least suited to it.
  out="$("$base/heliograph-seal" open \
          --identity "$base/control.key" --peer "$base/station.pub" \
          --estate conformance --station station \
          --dir s2c --kind log --min-seq 0 --in "$tmp" 2>/dev/null)"
  rc=$?
  rm -f "$tmp"
  [ "$rc" = "0" ] || return 1
  # heliograph-seal prints the accepted sequence on the first line and the
  # document after it, so the shell never parses the envelope.
  printf '%s\n' "$out" | tail -n +2
}

# The stub is a background process and the suite's own trap only removes the
# work directory, which would leave it running and the port held.
drv_teardown() {
  local dir="$1" pid
  [ "$CONF_TRANSPORT" = relay ] || return 0
  pid="$(cat "$dir.relay/pid" 2>/dev/null)" || return 0
  [ -n "$pid" ] && kill "$pid" 2>/dev/null
  return 0
}
