package packaging

// The winget job submits three files to a repository we do not control, and
// the files name release assets by filename and by hash. Nothing else ties
// those names to the names packaging/reproduce.sh actually produces: a
// renamed artefact would pass every other test, the release would publish,
// and the manifest would point Windows users at a 404. These tests run the
// generator against a SHA256SUMS shaped like the real one and read what it
// wrote, at head, before there is a tag.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// windowsArtefacts are the Windows executables reproduce.sh builds, derived
// from its target list rather than typed here, so the tests follow the script
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

// generate runs manifests.sh against a SHA256SUMS listing the given files
// with a made-up hash each, and returns the directory it wrote and the
// per-file hashes it was given, upper-cased as winget-pkgs writes them.
func generate(t *testing.T, tag string, names []string) (string, map[string]string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("manifests.sh is bash; the release job runs it on ubuntu")
	}
	dir := t.TempDir()
	sums := filepath.Join(dir, "SHA256SUMS")
	want := map[string]string{}
	var lines []string
	for i, n := range names {
		h := strings.Repeat(string(rune('a'+i)), 64)
		want[n] = strings.ToUpper(h)
		lines = append(lines, h+"  "+n)
	}
	if err := os.WriteFile(sums, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "winget/manifests.sh", tag, sums, filepath.Join(dir, "out"))
	cmd.Env = append(os.Environ(), "RELEASE_DATE=2026-01-02", "GITHUB_REPOSITORY=example/heliograph")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("manifests.sh failed: %v\n%s", err, out)
	}
	return filepath.Join(dir, "out", "manifests", "h", "heliograph-io", "heliograph", strings.TrimPrefix(tag, "v")), want
}

func readManifest(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the generator did not write %s: %v", filepath.Base(path), err)
	}
	return string(b)
}

func TestWingetManifestsNameTheArtefactsReproduceBuilds(t *testing.T) {
	names := windowsArtefacts(t)
	dir, want := generate(t, "v9.9.9", names)
	installer := readManifest(t, filepath.Join(dir, "heliograph-io.heliograph.installer.yaml"))

	for _, n := range names {
		if !strings.Contains(installer, "/releases/download/v9.9.9/"+n) {
			t.Errorf("reproduce.sh builds %s and the installer manifest never names it.\n"+
				"Windows users on that architecture would get no package.", n)
		}
		if !strings.Contains(installer, "InstallerSha256: "+want[n]) {
			t.Errorf("the hash published for %s is not the one SHA256SUMS holds for it", n)
		}
	}
	for _, m := range regexp.MustCompile(`heliograph-windows-[a-z0-9]+\.exe`).FindAllString(installer, -1) {
		if _, ok := want[m]; !ok {
			t.Errorf("the manifest names %s, which reproduce.sh does not build. It would point at a file the release does not contain.", m)
		}
	}
	if !strings.Contains(installer, "InstallerType: portable") || !strings.Contains(installer, "Commands:\n- heliograph\n") {
		t.Error("the installer is not portable with the command heliograph, so the symlink winget puts on PATH would carry the download's name")
	}
	if !strings.Contains(installer, "https://github.com/example/heliograph/releases/download/") {
		t.Error("the InstallerUrl does not follow GITHUB_REPOSITORY, so the URLs would not move with the repository")
	}
}

func TestWingetManifestsAgreeOnVersionAndSchema(t *testing.T) {
	dir, _ := generate(t, "v9.9.9", windowsArtefacts(t))
	for _, f := range []string{
		"heliograph-io.heliograph.yaml",
		"heliograph-io.heliograph.installer.yaml",
		"heliograph-io.heliograph.locale.en-US.yaml",
	} {
		s := readManifest(t, filepath.Join(dir, f))
		if !strings.Contains(s, "PackageIdentifier: heliograph-io.heliograph\n") {
			t.Errorf("%s does not carry the identifier decided in heliograph-cloud#201", f)
		}
		if !strings.Contains(s, "PackageVersion: 9.9.9\n") {
			t.Errorf("%s does not carry the version stripped of its v; winget-pkgs would file it under the wrong directory", f)
		}
		if !strings.Contains(s, "ManifestVersion: 1.12.0\n") {
			t.Errorf("%s does not name the manifest schema; winget-pkgs validation would refuse it", f)
		}
	}
}

func TestWingetRefusesAMissingHash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("manifests.sh is bash; the release job runs it on ubuntu")
	}
	// Every artefact but the first, so one hash is missing.
	names := windowsArtefacts(t)[1:]
	dir := t.TempDir()
	sums := filepath.Join(dir, "SHA256SUMS")
	var lines []string
	for _, n := range names {
		lines = append(lines, strings.Repeat("0", 64)+"  "+n)
	}
	if err := os.WriteFile(sums, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", "winget/manifests.sh", "v9.9.9", sums, filepath.Join(dir, "out")).CombinedOutput()
	if err == nil {
		t.Fatalf("manifests.sh wrote a manifest with an architecture missing from SHA256SUMS.\n"+
			"That manifest would install nothing on that architecture, silently.\n%s", out)
	}
}

func TestWingetLicenceMatchesTheLicenceFile(t *testing.T) {
	// The LICENSE file is the fact; the manifest's License field is a claim
	// about it. This failed on the Apache relicence and caught the reason:
	// it read LICENSE line 1, and Apache's text opens with a BLANK line and
	// then centres its title under 33 spaces. "first line" and "the licence
	// name" are the same thing only under MIT.
	b, err := os.ReadFile("../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	first := firstNonEmptyTrimmed(string(b))
	dir, _ := generate(t, "v9.9.9", windowsArtefacts(t))
	locale := readManifest(t, filepath.Join(dir, "heliograph-io.heliograph.locale.en-US.yaml"))
	m := regexp.MustCompile(`(?m)^License: (.+)$`).FindStringSubmatch(locale)
	if m == nil {
		t.Fatal("the locale manifest has no License line")
	}
	switch {
	case first == "MIT License" && m[1] == "MIT":
	case strings.HasPrefix(first, "Apache License") && m[1] == "Apache-2.0":
	case strings.HasPrefix(first, "Functional Source License") && m[1] == "LicenseRef-FSL-1.1-ALv2":
	default:
		t.Errorf("LICENSE opens with %q and the manifest says License: %s. The package would claim a licence the binary is not under.", first, m[1])
	}
}

// firstNonEmptyTrimmed is what `manifests.sh` does with awk, kept in step with
// it deliberately: if the two disagree about which line names the licence,
// this test passes on one reading and the shipped manifest carries the other.
func firstNonEmptyTrimmed(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

// TestWingetPublisherComesFromNotice holds the other half of the same
// relicence. The publisher was read from LICENSE line 3, which was the
// copyright line under MIT and is "Version 2.0, January 2004" under Apache.
// Nothing would have failed: the manifest would simply have credited the
// Apache Software Foundation's version string as the author.
func TestWingetPublisherComesFromNotice(t *testing.T) {
	b, err := os.ReadFile("../NOTICE")
	if err != nil {
		t.Fatalf("NOTICE is where the copyright lives now, and it is not readable: %v", err)
	}
	var copyright string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Copyright (c) ") {
			copyright = line
			break
		}
	}
	if copyright == "" {
		t.Fatal("NOTICE has no 'Copyright (c) ' line, so manifests.sh has no publisher to name")
	}
	dir, _ := generate(t, "v9.9.9", windowsArtefacts(t))
	locale := readManifest(t, filepath.Join(dir, "heliograph-io.heliograph.locale.en-US.yaml"))
	m := regexp.MustCompile(`(?m)^Copyright: (.+)$`).FindStringSubmatch(locale)
	if m == nil {
		t.Fatal("the locale manifest has no Copyright line")
	}
	if m[1] != copyright {
		t.Errorf("the manifest Copyright is %q and NOTICE says %q", m[1], copyright)
	}
	a := regexp.MustCompile(`(?m)^Author: (.+)$`).FindStringSubmatch(locale)
	if a == nil {
		t.Fatal("the locale manifest has no Author line")
	}
	if !strings.Contains(copyright, a[1]) {
		t.Errorf("the manifest Author is %q and NOTICE says %q. The package would name somebody who does not hold the copyright.", a[1], copyright)
	}
	// And the reader has to be able to get to the notice it came from. Under
	// MIT this pointed at LICENSE, which held the copyright line. Apache's
	// does not.
	if !strings.Contains(locale, "CopyrightUrl") || !regexp.MustCompile(`(?m)^CopyrightUrl: .*/NOTICE$`).MatchString(locale) {
		t.Error("CopyrightUrl does not point at NOTICE, which is the file the copyright is now in")
	}
}

func TestWingetJobRunsTheGenerator(t *testing.T) {
	b, err := os.ReadFile("../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("cannot read release.yml: %v", err)
	}
	m := regexp.MustCompile(`(?ms)^  winget:\n(.*?)(?:^  [a-z]+:\n|\z)`).FindStringSubmatch(string(b))
	if m == nil {
		t.Fatal("release.yml has no winget job, so nothing submits a release to winget")
	}
	job := m[1]
	if !strings.Contains(job, "packaging/winget/manifests.sh") {
		t.Error("the winget job does not run packaging/winget/manifests.sh, so what it submits is not what these tests checked")
	}
	if !strings.Contains(job, "secrets.WINGET_TOKEN") {
		t.Error("the winget job does not read WINGET_TOKEN, so it cannot open a pull request on winget-pkgs")
	}
	if !strings.Contains(job, "-p SHA256SUMS") {
		t.Error("the winget job does not fetch SHA256SUMS from the release, so the hash it publishes is not the signed one")
	}
}
