// Package station carries the far-side station payloads inside the control
// binary.
//
// This is the one Go file permitted under station/, and it never ships to the
// far side: it exists so that `heliograph bootstrap` can plant a station
// without needing a checkout of this repository, and so that a release binary
// plants exactly the station it was built against - "which station is this
// estate running" then has the same answer as "which binary planted it".
//
// Everything under bash/ and powershell/ is the far side: bash 4+, git and GNU
// coreutils for one; Windows PowerShell 5.1 and nothing else for the other.
// No packages, no credentials of its own. CI enforces that no other Go appears
// under station/.
//
// TWO PAYLOADS, NOT ONE WITH A SWITCH. They are separate directories because
// they are separate implementations that must pass the same conformance suite,
// and a machine gets one or the other: an estate with bash runs the bash
// station, because one implementation is better than two wherever there is a
// choice. The PowerShell payload is for the estate that has no bash at all.
package station

import "embed"

// Bash is the bash station payload, rooted at "bash". The all: prefix is
// load-bearing: without it go:embed silently drops dotfiles and underscore
// files, which here would mean no ops-logs/.gitkeep, no Terraform lock files
// and no steps/_template.sh - a payload that looks complete and is not.
//
//go:embed all:bash
var Bash embed.FS

// PowerShell is the pure-PowerShell station payload, rooted at "powershell".
//
// The same all: prefix, and the same reason with different files: without it
// this payload loses ops-logs/.gitkeep and steps/_template.ps1, and a station
// planted from it has nowhere to write a log and no example to copy.
//
//go:embed all:powershell
var PowerShell embed.FS
