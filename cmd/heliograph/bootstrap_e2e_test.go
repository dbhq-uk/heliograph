package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dbhq-uk/heliograph/internal/bootstrap"
	"github.com/dbhq-uk/heliograph/station"
)

// THREE WAYS TO PLANT A STATION, AND THEY MUST PRODUCE ONE TREE.
//
//	heliograph bootstrap   needs the Go binary
//	station/bootstrap.sh   needs bash
//	station/bootstrap.ps1  needs neither, which is the whole premise of the
//	                       PowerShell station - the plant may not be the one
//	                       step that assumes a shell the machine does not have
//
// They are the same operation with a different delivery, and this is what keeps
// that sentence true: all three are run against the same checkout and the trees
// must be identical, byte for byte, file for file. If somebody adds a file to a
// payload and only one path picks it up, this is what notices.
func TestBootstrapsAgree(t *testing.T) {
	dir := stationDir(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	ps := findPowerShell()

	for _, flavour := range []struct {
		name string
		fsys fs.FS
		root string
	}{
		{"bash", station.Bash, "bash"},
		{"powershell", station.PowerShell, "powershell"},
	} {
		t.Run(flavour.name, func(t *testing.T) {
			base := t.TempDir()
			fromGo := filepath.Join(base, "go")
			fromSh := filepath.Join(base, "sh")

			if _, err := bootstrap.Install(flavour.fsys, flavour.root, fromGo); err != nil {
				t.Fatal(err)
			}
			sh(t, base, filepath.Join(dir, "station", "bootstrap.sh"), fromSh, "--flavour", flavour.name)
			compareTrees(t, "`heliograph bootstrap`", fromGo, "bootstrap.sh", fromSh)

			if ps == "" {
				// SAID OUT LOUD. A silently unexercised third planter is a
				// planter nobody is checking.
				t.Log("SKIP: no PowerShell here, so bootstrap.ps1 was NOT compared")
				return
			}
			fromPs := filepath.Join(base, "ps")
			run(t, base, ps, "-NoProfile", "-File",
				filepath.Join(dir, "station", "bootstrap.ps1"), fromPs, "-Flavour", flavour.name)
			compareTrees(t, "bootstrap.ps1", fromPs, "bootstrap.sh", fromSh)
		})
	}
}

// The files that must come out runnable, from the path that cannot inherit a
// mode from git. go:embed does not carry file modes, so Install restores the
// execute bit from the shebang.
//
// THERE IS NO POWERSHELL EQUIVALENT and that is not an omission: PowerShell
// resolves a script by extension and never consults the execute bit, so a
// .ps1 planted 0644 runs exactly as one planted 0755. What decides whether it
// runs at all is the execution policy, which start.ps1 reports per scope.
func TestBootstrapPlantsRunnableScripts(t *testing.T) {
	base := t.TempDir()
	if _, err := bootstrap.Install(station.Bash, "bash", base); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"run.sh", "station.sh", "start.sh"} {
		fi, err := os.Stat(filepath.Join(base, f))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode()&0o100 == 0 {
			t.Errorf("%s planted without its execute bit: %v", f, fi.Mode())
		}
	}
}

// `--flavour both` puts two payloads in one repo. Nothing is overwritten, so
// the first one planted supplies the files they share - which is exactly why
// the two ignore files carry an identical shared block, and why this checks
// that the second plant did not quietly replace the first one's .gitignore.
func TestBootstrapBothPlantsTwoPayloads(t *testing.T) {
	base := t.TempDir()
	roots, err := bootstrap.ParseFlavours("both")
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range roots {
		payload := station.Bash
		if root == "powershell" {
			payload = station.PowerShell
		}
		if _, err := bootstrap.Install(payload, root, base); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"run.sh", "station.sh", "run.ps1", "station.ps1", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(base, f)); err != nil {
			t.Errorf("--flavour both did not plant %s: %v", f, err)
		}
	}

	// The shared block is what makes the order not matter for the rules that
	// keep a credential out of the repository.
	body, err := os.ReadFile(filepath.Join(base, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{".station-env", ".station-approved", ".station-approved-ps", ".station-delivery"} {
		if !contains(string(body), "\n"+rule+"\n") {
			t.Errorf("the planted .gitignore does not ignore %s, so a repo with both payloads can commit it", rule)
		}
	}
}

func TestParseFlavours(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
		bad  bool
	}{
		{"", 1, false}, {"bash", 1, false}, {"powershell", 1, false},
		{"both", 2, false}, {"BASH", 0, true}, {"cmd", 0, true},
	} {
		got, err := bootstrap.ParseFlavours(c.in)
		if c.bad {
			if err == nil {
				t.Errorf("--flavour %q was accepted and should not be", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("--flavour %q: %v", c.in, err)
			continue
		}
		if len(got) != c.want {
			t.Errorf("--flavour %q gave %v, wanted %d payload(s)", c.in, got, c.want)
		}
	}
}

func compareTrees(t *testing.T, aName, a, bName, b string) {
	t.Helper()
	got, want := walkTree(t, a), walkTree(t, b)
	for rel, body := range want {
		gb, ok := got[rel]
		if !ok {
			t.Errorf("%s installs %s; %s does not", bName, rel, aName)
			continue
		}
		if gb != body {
			t.Errorf("%s differs between %s and %s", rel, aName, bName)
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			t.Errorf("%s installs %s; %s does not", aName, rel, bName)
		}
	}
}

func walkTree(t *testing.T, root string) map[string]string {
	t.Helper()
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
		m[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func findPowerShell() string {
	if v := os.Getenv("CONF_PS_SHELL"); v != "" {
		if p, err := exec.LookPath(v); err == nil {
			return p
		}
	}
	for _, c := range []string{"pwsh", "powershell", "powershell.exe"} {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(hay); i++ {
			if hay[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
