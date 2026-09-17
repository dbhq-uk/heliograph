#!/usr/bin/env bash
# =============================================================================
#  manifests.sh - write the winget manifests for one release, from SHA256SUMS
# =============================================================================
#     packaging/winget/manifests.sh v0.4.2 dist/SHA256SUMS out
#
#  Writes three files under
#     out/manifests/h/heliograph-io/heliograph/0.4.0/
#  which is the path microsoft/winget-pkgs expects, and the release workflow
#  pushes them to a fork and opens the pull request.
#
#  WHY A SCRIPT AND NOT KOMAC. komac was the first design. `komac update`
#  refuses a package that does not exist in winget-pkgs yet ("does not exist
#  in microsoft/winget-pkgs", tested 2026-09-13), and `komac new` stops at a
#  "Custom switch:" prompt that no flag suppresses, so neither could make the
#  first submission from a workflow. A winget package for a bare executable is
#  three short YAML files, and everything in them is already in this
#  repository or in the release: the two Windows executables and their hashes
#  are in SHA256SUMS, the description and URLs in packaging/npm/package.json,
#  the licence and its holder in LICENSE.
#
#  THE HASH COMES FROM SHA256SUMS, not from downloading the file again. That
#  is the file the release job signs, so the number winget checks before
#  running the executable is the number a stranger can reproduce with
#  packaging/reproduce.sh. Nothing is hashed here.
#
#  PORTABLE, which is winget's word for a bare executable with a symlink on
#  PATH. `Commands: [heliograph]` makes that symlink heliograph.exe rather than
#  heliograph-windows-amd64.exe: winget-cli takes Commands[0] as the alias
#  when PortableCommandAlias is unset (Workflows/PortableFlow.cpp, the
#  portable install path). No installer is built, no zip, no MSI.
#
#  The identifier is heliograph-io.heliograph, decided in
#  heliograph-cloud#201: a winget identifier is permanent, and this is the
#  name the org move already settled.
# =============================================================================
set -euo pipefail

tag="${1:?usage: manifests.sh <tag, e.g. v0.4.2> <SHA256SUMS> <outdir>}"
sums="${2:?usage: manifests.sh <tag> <SHA256SUMS> <outdir>}"
out="${3:?usage: manifests.sh <tag> <SHA256SUMS> <outdir>}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"

id="heliograph-io.heliograph"
ver="${tag#v}"
schema="1.12.0"
repo="${GITHUB_REPOSITORY:-heliograph-io/heliograph}"
date="${RELEASE_DATE:-$(date -u +%F)}"
base="https://github.com/${repo}/releases/download/${tag}"

# A hash from SHA256SUMS, upper-cased because that is how winget-pkgs writes
# them. Refused rather than defaulted when the file is not listed: a manifest
# with no hash for an architecture is a manifest that installs nothing there.
hash_for() {
  local name="$1" h
  h="$(awk -v n="$name" '$2 == n { print toupper($1) }' "$sums")"
  if [ -z "$h" ]; then
    echo "manifests.sh: $name is not in $sums, so there is no hash to publish for it" >&2
    exit 2
  fi
  printf '%s' "$h"
}
amd64="$(hash_for heliograph-windows-amd64.exe)"
arm64="$(hash_for heliograph-windows-arm64.exe)"

# The words, read from where they already live rather than typed here again.
pkg="$root/packaging/npm/package.json"
description="$(jq -r .description "$pkg")"
homepage="$(jq -r .homepage "$pkg")"
bugs="$(jq -r .bugs "$pkg")"
# THE COPYRIGHT COMES FROM `NOTICE`, NOT FROM `LICENSE`, and the Apache
# relicence is why. Under MIT the copyright line was `LICENSE:3` and this read
# it from there. Apache's text has no copyright line at all - the whole point
# of a NOTICE file is that the licence text stays pristine and the attribution
# lives beside it - so `sed -n 3p LICENSE` returned
# "Version 2.0, January 2004" and every winget manifest would have credited it
# as the author.
copyright="$(grep -m1 '^Copyright (c) ' "$root/NOTICE" || true)"
if [ -z "$copyright" ]; then
  echo "manifests.sh: NOTICE has no 'Copyright (c) ' line, and a manifest must name a publisher" >&2
  exit 2
fi
author="${copyright#Copyright (c) [0-9][0-9][0-9][0-9] }"

# THE LICENCE NAME IS MATCHED AFTER STRIPPING LEADING WHITESPACE. Apache's text
# opens with a blank line and centres its title with 33 spaces, so both
# `sed -n 1p` and an unanchored prefix match fail on it. Read the first
# non-empty line and trim it.
first="$(awk 'NF { $1=$1; print; exit }' "$root/LICENSE")"
case "$first" in
  "MIT License")           license="MIT" ;;
  "Apache License"*)       license="Apache-2.0" ;;
  "Functional Source License"*) license="LicenseRef-FSL-1.1-ALv2" ;;
  *) echo "manifests.sh: LICENSE opens with '$first', which is not a licence this script knows, so it will not guess an SPDX id" >&2; exit 2 ;;
esac

dir="$out/manifests/h/heliograph-io/heliograph/$ver"
mkdir -p "$dir"

cat > "$dir/$id.yaml" <<YAML
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.version.${schema}.schema.json

PackageIdentifier: $id
PackageVersion: $ver
DefaultLocale: en-US
ManifestType: version
ManifestVersion: $schema
YAML

cat > "$dir/$id.installer.yaml" <<YAML
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.installer.${schema}.schema.json

PackageIdentifier: $id
PackageVersion: $ver
InstallerType: portable
Commands:
- heliograph
ReleaseDate: $date
Installers:
- Architecture: x64
  InstallerUrl: $base/heliograph-windows-amd64.exe
  InstallerSha256: $amd64
- Architecture: arm64
  InstallerUrl: $base/heliograph-windows-arm64.exe
  InstallerSha256: $arm64
ManifestType: installer
ManifestVersion: $schema
YAML

cat > "$dir/$id.locale.en-US.yaml" <<YAML
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.defaultLocale.${schema}.schema.json

PackageIdentifier: $id
PackageVersion: $ver
PackageLocale: en-US
Publisher: heliograph
PublisherUrl: $homepage
PublisherSupportUrl: $bugs
Author: $author
PackageName: heliograph
PackageUrl: $homepage
License: $license
LicenseUrl: https://github.com/${repo}/blob/${tag}/LICENSE
Copyright: $copyright
CopyrightUrl: https://github.com/${repo}/blob/${tag}/NOTICE
ShortDescription: Run commands on a machine you cannot SSH into.
Description: $description
Moniker: heliograph
Tags:
- air-gapped
- cli
- devops
- mcp
- mcp-server
- remote-execution
- ssh-alternative
ReleaseNotesUrl: https://github.com/${repo}/releases/tag/${tag}
ManifestType: defaultLocale
ManifestVersion: $schema
YAML

echo "wrote $dir"
ls -1 "$dir"
