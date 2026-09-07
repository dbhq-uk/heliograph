#!/usr/bin/env bash
# =============================================================================
#  build.sh - one .mcpb bundle per platform, from the binaries in dist/
# =============================================================================
#     packaging/mcpb/build.sh <version>
#
#  A bundle is a zip with manifest.json at its root and the server beside it.
#  Built with zip rather than with the mcpb CLI, which is an npm install for a
#  step that is two commands - and would put Node on the runner to package a Go
#  binary.
#
#  ONE PER PLATFORM. A bundle carries a binary, and one carrying all six would
#  be 42MB to deliver 7MB of tool.
#
#  This is a script rather than inline YAML because it embeds a small python
#  program, and a heredoc nested inside a YAML block scalar takes the YAML
#  indentation with it: the terminator stops matching and the step fails with a
#  shell syntax error that says nothing about bundles.
# =============================================================================
set -euo pipefail

ver="${1:?usage: build.sh <version without the v>}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
cd "$root"

[ -d dist ] || { echo "no dist/ - build the binaries first" >&2; exit 1; }

made=0
for target in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64 windows-arm64; do
  src="dist/heliograph-${target}"
  exe=""
  case "$target" in windows-*) src="${src}.exe"; exe=".exe" ;; esac
  [ -f "$src" ] || { echo "missing $src" >&2; exit 1; }

  work="$(mktemp -d)"
  mkdir -p "$work/server"
  cp "$src" "$work/server/heliograph${exe}"

  # The manifest's version must match the tag, or a client installs a bundle
  # reporting a version the binary inside does not have.
  MANIFEST="$here/manifest.json" OUT="$work/manifest.json" VER="$ver" EXE="$exe" \
    python3 -c '
import json, os
m = json.load(open(os.environ["MANIFEST"]))
m["version"] = os.environ["VER"]
exe = os.environ["EXE"]
if exe:
    m["server"]["entry_point"] = "server/heliograph" + exe
    m["server"]["mcp_config"]["command"] = "${__dirname}/server/heliograph" + exe
json.dump(m, open(os.environ["OUT"], "w"), indent=2)
'
  # python's zipfile rather than `zip`: one less thing to be installed, and
  # the executable bit has to be set explicitly either way for the server to be
  # runnable after a client unpacks it.
  WORK="$work" OUT="$root/dist/heliograph-${target}.mcpb" python3 -c '
import os, zipfile
work, out = os.environ["WORK"], os.environ["OUT"]
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
    for base, _, files in os.walk(work):
        for f in files:
            full = os.path.join(base, f)
            rel = os.path.relpath(full, work)
            info = zipfile.ZipInfo(rel)
            mode = os.stat(full).st_mode
            info.external_attr = (mode & 0xFFFF) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            with open(full, "rb") as fh:
                z.writestr(info, fh.read())
'
  rm -rf "$work"
  made=$((made + 1))
done

echo "built $made .mcpb bundles"
