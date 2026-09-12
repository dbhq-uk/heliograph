// Package bootstrap plants the embedded station payload into a transport repo.
//
// It is station/bootstrap.sh with the checkout requirement removed: the same
// files, the same semantics, from the copy of the station embedded in the
// binary at build time. The semantics that matter are inherited deliberately:
//
//   - Nothing is overwritten, ever. An existing file is left alone and
//     reported, because the second run is usually an upgrade over a repo that
//     already has a task in flight.
//   - The station ships its ignore and attributes files without a leading dot
//     so they govern the transport repo rather than the repo carrying them.
//     The dot is restored here, at the root only.
//
// One divergence from the shell script: go:embed does not carry file modes,
// so the execute bit is restored from the shebang. Every file that begins
// with "#!" is written 0755, everything else 0644. That marks a couple of
// sourced-only libraries executable, which is harmless; the failure the rule
// prevents - run.sh landing without its execute bit on a machine nobody can
// reach - is not.
package bootstrap

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Report says what a plant did, in the same vocabulary bootstrap.sh uses.
type Report struct {
	Installed []string
	LeftAlone []string
}

// Flavours are the payloads a station can be planted from, in the order they
// are planted when more than one is asked for.
//
// A MACHINE GETS ONE OF THEM. An estate with bash runs the bash station,
// because one implementation is better than two wherever there is a choice;
// the PowerShell payload is for the estate that has none. "both" exists for the
// repo that serves two machines of different kinds, not as a recommendation.
//
// WHEN BOTH ARE PLANTED THE FIRST ONE WINS the files they share - the ignore
// file, the attributes file, TASK.md, station/request. Install never
// overwrites, so the second plant reports them as left alone rather than
// silently replacing a file the first one put there. That is why the ignore
// files carry an identical shared block: the rules that keep a credential out
// of the repository must not depend on the order somebody typed two commands
// in.
var Flavours = []string{"bash", "powershell"}

// StationRuntimeState names the files a running station writes for itself.
// Never planted, never embedded - see the prune in Install and the guard in
// station/embed_test.go, which enforce the same list from opposite ends.
var StationRuntimeState = []string{
	".station.lock",
	".station-state",
	".station-approved",
	".station-approved-ps",
	".station-delivery",
	".station-env",
	".station-env-ps",
	".station-relay-state",
	".agent-service.pid",
	".station-service.log",
}

func isStationRuntimeState(name string) bool {
	for _, s := range StationRuntimeState {
		if name == s {
			return true
		}
	}
	return false
}

// ValidFlavour reports whether name is a payload this binary carries.
func ValidFlavour(name string) bool {
	for _, f := range Flavours {
		if f == name {
			return true
		}
	}
	return false
}

// ParseFlavours turns the --flavour value into the payload roots to plant.
func ParseFlavours(spec string) ([]string, error) {
	switch spec {
	case "", "bash":
		return []string{"bash"}, nil
	case "powershell":
		return []string{"powershell"}, nil
	case "both":
		return append([]string(nil), Flavours...), nil
	}
	return nil, fmt.Errorf("unknown --flavour %q: use bash, powershell or both", spec)
}

// Install writes every file under root in fsys into target. It creates target
// if it is missing and refuses nothing else: the caller owns the "is this a
// fresh private repo" conversation, exactly as with bootstrap.sh.
func Install(fsys fs.FS, root, target string) (Report, error) {
	var r Report
	if target == "" {
		return r, fmt.Errorf("no target directory")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return r, err
	}

	var files []string
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// THE SAME PRUNE bootstrap.sh MAKES, and for the same measured reason.
		//
		// `terraform init` drops a provider binary next to each Azure template,
		// and azurerm alone is over 200MB - so a bootstrapped repo went from
		// about 200KB to 913MB before that prune existed, and that repo gets
		// cloned on a locked-down control node, sometimes over the link that is
		// the reason this tool exists.
		//
		// HERE IT IS WORSE THAN A LARGE REPO. `go:embed all:bash` reads the
		// WORKING TREE at build time, so a release built on a machine where
		// anybody had run `terraform init` would carry those binaries inside
		// the `heliograph` binary itself - permanently, in every download.
		// .gitignore keeps them out of the repository and does nothing about
		// the embed, and CI never saw it because a CI runner starts clean.
		//
		// Found by running `terraform test` locally, which is exactly the thing
		// the templates needed and nothing had ever done.
		if d.IsDir() {
			switch d.Name() {
			case ".terraform", ".git", "node_modules":
				return fs.SkipDir
			}
			return nil
		}
		// A STATION'S OWN RUNTIME STATE IS NOT PART OF THE PAYLOAD.
		//
		// Every one of these is written by a running station, is local to one
		// machine, and is in the payload's .gitignore because it is nobody
		// else's. Planting one means a brand-new transport repo arrives holding
		// the last machine's lock pid, its approved hashes, or - worst -
		// .station-env, which holds a token.
		//
		// Found in the same pass as the embed guard: a checkout that had run
		// the test suite carried station/bash/.station-delivery, and
		// bootstrap.sh's `find` planted it. go:embed carried it too; that half
		// is caught in station/embed_test.go, and this is the half that matters
		// when somebody runs bootstrap.sh straight out of a working checkout.
		if isStationRuntimeState(d.Name()) {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return r, err
	}
	sort.Strings(files)

	for _, p := range files {
		rel := strings.TrimPrefix(p, root+"/")
		dest := rel
		// The dot restored at the root only: azure/function/.funcignore and
		// friends already carry theirs.
		switch rel {
		case "gitignore":
			dest = ".gitignore"
		case "gitattributes":
			dest = ".gitattributes"
		}
		full := filepath.Join(target, filepath.FromSlash(dest))
		if _, err := os.Lstat(full); err == nil {
			r.LeftAlone = append(r.LeftAlone, dest)
			continue
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return r, err
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return r, err
		}
		mode := os.FileMode(0o644)
		if bytes.HasPrefix(b, []byte("#!")) {
			mode = 0o755
		}
		if err := os.WriteFile(full, b, mode); err != nil {
			return r, err
		}
		r.Installed = append(r.Installed, dest)
	}
	return r, nil
}
