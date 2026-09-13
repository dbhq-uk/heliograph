package packaging

// The winget job in release.yml names two release assets by filename and
// submits those names to a repository we do not control. Nothing else ties
// the names it uses to the names packaging/reproduce.sh actually produces: a
// renamed artefact would pass every other test, the release would publish,
// and the winget submission would point Windows users at a 404. This reads
// both files and refuses the mismatch at head, before there is a tag.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// windowsArtefacts are the Windows executables reproduce.sh builds, derived
// from its target list rather than typed here, so the test follows the script
// if a target is added or dropped.
func windowsArtefacts(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("reproduce.sh")
	if err != nil {
		t.Fatalf("cannot read reproduce.sh: %v", err)
	}
	m := regexp.MustCompile(`(?m)^targets="([^"]+)"`).FindStringSubmatch(string(b))
	if m == nil {
		t.Fatal("reproduce.sh has no targets=\"...\" line, so this test cannot know what is built")
	}
	var out []string
	for _, target := range strings.Fields(m[1]) {
		if strings.HasPrefix(target, "windows/") {
			out = append(out, "heliograph-windows-"+strings.TrimPrefix(target, "windows/")+".exe")
		}
	}
	if len(out) == 0 {
		t.Fatal("reproduce.sh builds no windows target, so there is nothing for winget to install")
	}
	return out
}

// wingetJob is the text of the winget job alone, cut from release.yml at its
// `winget:` key and ending at the next job key, so an artefact name that
// appears elsewhere in the workflow cannot satisfy the check by accident.
func wingetJob(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("cannot read release.yml: %v", err)
	}
	m := regexp.MustCompile(`(?ms)^  winget:\n(.*?)(?:^  [a-z]+:\n|\z)`).FindStringSubmatch(string(b))
	if m == nil {
		t.Fatal("release.yml has no winget job, so nothing submits a release to winget")
	}
	return m[1]
}

func TestWingetSubmitsTheArtefactsReproduceBuilds(t *testing.T) {
	job := wingetJob(t)
	for _, name := range windowsArtefacts(t) {
		if !strings.Contains(job, name) {
			t.Errorf("reproduce.sh builds %s and the winget job never names it.\n"+
				"Windows users on that architecture would get no package, or a manifest pointing at a URL that 404s.", name)
		}
	}
	for _, m := range regexp.MustCompile(`heliograph-windows-[a-z0-9]+\.exe`).FindAllString(job, -1) {
		found := false
		for _, name := range windowsArtefacts(t) {
			if m == name {
				found = true
			}
		}
		if !found {
			t.Errorf("the winget job submits %s, which reproduce.sh does not build. The manifest would name a file the release does not contain.", m)
		}
	}
}

func TestWingetPinsItsToolByHash(t *testing.T) {
	job := wingetJob(t)
	if !regexp.MustCompile(`[0-9a-f]{64}  komac\.tgz`).MatchString(job) {
		t.Error("the winget job does not check komac's download against a SHA-256.\n" +
			"A version pin alone trusts whatever GitHub serves under that tag, and this tool\n" +
			"writes the manifest that decides what a Windows machine downloads and runs.")
	}
	if !strings.Contains(job, "heliograph-io.heliograph") {
		t.Error("the winget job does not name the package identifier heliograph-io.heliograph, so komac would update nothing or the wrong package")
	}
}
