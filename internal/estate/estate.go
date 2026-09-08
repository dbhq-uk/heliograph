// Package estate remembers which transport repo a name refers to, so that
// every other command can be typed without one.
//
// An investigation runs for days and the commands are typed under pressure.
// `heliograph send net-probe` has to be the whole thing; re-stating a path and
// a branch on every invocation is how the wrong repo gets a request.
package estate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Estate is one transport repo, under a name a person chose.
type Estate struct {
	Name      string `json:"name"`
	Transport string `json:"transport"` // git | share | bundle | objstore
	Dir       string `json:"dir"`       // the working clone, or the endpoint for objstore
	Branch    string `json:"branch"`    // recorded for reporting; the checkout decides
	Scope     string `json:"scope"`     // share: one directory, objstore: one lane, per investigation

	// objstore only. Identifiers, all of them: an endpoint, a bucket and a
	// prefix say WHERE, and none of them opens anything.
	//
	// The keys are deliberately absent and there is no field for them. They
	// come from HELIOGRAPH_S3_ACCESS_KEY and HELIOGRAPH_S3_SECRET_KEY at the
	// moment of use. This file is on disk, gets copied between machines and
	// ends up in backups, and a secret in it would be a secret in all three.
	Bucket string `json:"bucket,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	Region string `json:"region,omitempty"`
}

// known transports. A name that is not here is refused at save time rather
// than at send time, when somebody is already waiting on a far side.
var known = map[string]bool{"git": true, "share": true, "bundle": true, "objstore": true}

func configDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "heliograph", "estates"), nil
}

// validName refuses anything that is not a single path element.
//
// The name becomes a filename, so `../../.ssh/authorized_keys` has to be
// refused here rather than trusted to behave. This is a small surface and an
// unpleasant one to get wrong.
// Routing is what this estate is bound to: the git branch, the share
// directory, the object-store lane.
//
// Scope is canonical and Branch is the fallback, in that order, because
// estates written before Scope existed recorded a git branch in Branch alone.
// Reading Scope only would silently un-pin every estate in the field, which is
// the opposite of what pinning is for.
func (e Estate) Routing() string {
	if e.Scope != "" {
		return e.Scope
	}
	return e.Branch
}

// ValidName is validName, exported so a caller can refuse a bad name BEFORE it
// creates a branch and a checkout named after it. Save would catch it, by which
// point there is a pushed branch and a worktree to clean up.
func ValidName(n string) error { return validName(n) }

func validName(n string) error {
	if n == "" {
		return fmt.Errorf("an estate needs a name")
	}
	if n == "." || n == ".." || strings.ContainsAny(n, `/\`) || strings.HasPrefix(n, ".") {
		return fmt.Errorf("%q is not a usable estate name: use letters, digits, dashes", n)
	}
	return nil
}

// Save writes the estate, refusing one that could not be acted on.
func (e Estate) Save() error {
	if err := validName(e.Name); err != nil {
		return err
	}
	if !known[e.Transport] {
		return fmt.Errorf("unknown transport %q: this build knows git, share, bundle and objstore", e.Transport)
	}
	if e.Dir == "" {
		return fmt.Errorf("estate %q has no directory: it would have nothing to write to", e.Name)
	}
	if e.Transport == "objstore" && e.Bucket == "" {
		return fmt.Errorf("estate %q is an object store with no bucket", e.Name)
	}
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	// 0600: this names a working directory and may later name a token file.
	// Not a secret store, but not other people's business either.
	return os.WriteFile(filepath.Join(dir, e.Name+".json"), append(b, '\n'), 0o600)
}

// Load reads an estate by name.
func Load(name string) (Estate, error) {
	if err := validName(name); err != nil {
		return Estate{}, err
	}
	dir, err := configDir()
	if err != nil {
		return Estate{}, err
	}
	b, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			// Name what was not found. "estate not found" sends the reader to
			// check a spelling they cannot see.
			return Estate{}, fmt.Errorf("no estate named %q: run `heliograph init %s --dir <path>` first", name, name)
		}
		return Estate{}, err
	}
	var e Estate
	if err := json.Unmarshal(b, &e); err != nil {
		return Estate{}, fmt.Errorf("estate %q is not readable: %w", name, err)
	}
	return e, nil
}

// List names every estate, sorted.
func List() ([]string, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(names)
	return names, nil
}
