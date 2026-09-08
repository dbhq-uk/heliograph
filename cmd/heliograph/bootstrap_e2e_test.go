package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dbhq-uk/heliograph/internal/bootstrap"
	"github.com/dbhq-uk/heliograph/station"
)

// `heliograph bootstrap` and station/bootstrap.sh are the same operation with
// a different delivery, and this is what keeps that sentence true: both are
// run against the same checkout and the trees they produce must be identical,
// byte for byte, file for file. If someone adds a file to station/bash and
// only one path picks it up, this is what notices.
func TestBootstrapMatchesTheShellScript(t *testing.T) {
	dir := stationDir(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	base := t.TempDir()
	fromGo := filepath.Join(base, "go")
	fromSh := filepath.Join(base, "sh")

	if _, err := bootstrap.Install(station.Bash, "bash", fromGo); err != nil {
		t.Fatal(err)
	}
	sh(t, base, filepath.Join(dir, "station", "bootstrap.sh"), fromSh)

	walk := func(root string) map[string]string {
		m := map[string]string{}
		err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			m[rel] = string(b)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}

	got, want := walk(fromGo), walk(fromSh)
	for rel, body := range want {
		gb, ok := got[rel]
		if !ok {
			t.Errorf("bootstrap.sh installs %s; `heliograph bootstrap` does not", rel)
			continue
		}
		if gb != body {
			t.Errorf("%s differs between the two bootstraps", rel)
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			t.Errorf("`heliograph bootstrap` installs %s; bootstrap.sh does not", rel)
		}
	}

	// The one file that must come out runnable, from the path that cannot
	// inherit a mode from git.
	fi, err := os.Stat(filepath.Join(fromGo, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o100 == 0 {
		t.Errorf("run.sh planted without its execute bit: %v", fi.Mode())
	}
}
