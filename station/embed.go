// Package station carries the bash station payload inside the control binary.
//
// This is the one Go file permitted under station/, and it never ships to the
// far side: it exists so that `heliograph bootstrap` can plant the station
// without needing a checkout of this repository, and so that a release binary
// plants exactly the station it was built against - "which station is this
// estate running" then has the same answer as "which binary planted it".
//
// Everything under bash/ (and, when it exists, powershell/) is the far side:
// bash 4+, git and GNU coreutils, no packages, no credentials of its own. CI
// enforces that no other Go appears under station/.
package station

import "embed"

// Bash is the bash station payload, rooted at "bash". The all: prefix is
// load-bearing: without it go:embed silently drops dotfiles and underscore
// files, which here would mean no ops-logs/.gitkeep, no Terraform lock files
// and no steps/_template.sh - a payload that looks complete and is not.
//
//go:embed all:bash
var Bash embed.FS
