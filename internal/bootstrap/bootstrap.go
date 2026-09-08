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
		if !d.IsDir() {
			files = append(files, p)
		}
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
