package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"bash/run.sh":             {Data: []byte("#!/usr/bin/env bash\necho run\n")},
		"bash/caplib.sh":          {Data: []byte("# sourced, no shebang\n")},
		"bash/gitignore":          {Data: []byte("*.secret\n")},
		"bash/gitattributes":      {Data: []byte("* text eol=lf\n")},
		"bash/steps/_template.sh": {Data: []byte("#!/usr/bin/env bash\n")},
		"bash/ops-logs/.gitkeep":  {Data: []byte("")},
	}
}

func TestInstallLaysDownEverythingWithTheDotsRestored(t *testing.T) {
	target := t.TempDir()
	r, err := Install(testFS(), "bash", target)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Installed) != 6 || len(r.LeftAlone) != 0 {
		t.Fatalf("installed=%v leftAlone=%v", r.Installed, r.LeftAlone)
	}
	for _, want := range []string{".gitignore", ".gitattributes", "run.sh", "steps/_template.sh", "ops-logs/.gitkeep"} {
		if _, err := os.Stat(filepath.Join(target, want)); err != nil {
			t.Errorf("missing %s: %v", want, err)
		}
	}
	// The undotted originals must not be left behind: a transport repo with
	// both a gitignore and a .gitignore invites editing the wrong one.
	for _, bad := range []string{"gitignore", "gitattributes"} {
		if _, err := os.Stat(filepath.Join(target, bad)); err == nil {
			t.Errorf("undotted %s left behind", bad)
		}
	}
}

func TestTheExecuteBitFollowsTheShebang(t *testing.T) {
	target := t.TempDir()
	if _, err := Install(testFS(), "bash", target); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(target, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o100 == 0 {
		t.Errorf("run.sh has a shebang and no execute bit: %v", fi.Mode())
	}
	fi, err = os.Stat(filepath.Join(target, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o100 != 0 {
		t.Errorf(".gitignore has no shebang and is executable: %v", fi.Mode())
	}
}

// The property the shell bootstrap's whole walk exists for: the second run is
// an upgrade over a repo with a task in flight, and it must not clobber it.
func TestAnExistingFileIsLeftAloneAndReported(t *testing.T) {
	target := t.TempDir()
	if _, err := Install(testFS(), "bash", target); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(target, "run.sh")
	if err := os.WriteFile(edited, []byte("#!/usr/bin/env bash\necho EDITED\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := Install(testFS(), "bash", target)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Installed) != 0 {
		t.Errorf("second run installed %v over an existing tree", r.Installed)
	}
	if len(r.LeftAlone) != 6 {
		t.Errorf("second run did not report every file left alone: %v", r.LeftAlone)
	}
	b, _ := os.ReadFile(edited)
	if string(b) != "#!/usr/bin/env bash\necho EDITED\n" {
		t.Error("an existing file was overwritten")
	}
}
