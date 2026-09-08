package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The site documented the near side only, for a long time, and nothing said so.
//
// Everything about the far side - the thing the product actually is - lived in
// skills/heliograph/references/, which is tuned for an agent's context budget
// and is published nowhere. A reader deciding whether to permit this in their
// estate could read about the CLI and find nothing about what would run on
// their machine.
//
// That is not a gap anybody notices by looking, because every page that exists
// is fine. It is only visible by asking what is NOT there, which is what these
// tests do.

// siteText is every published page, concatenated. The question here is always
// "is this documented anywhere", never "is it on the right page" - page
// boundaries are an editorial decision and should stay one.
func siteText(t *testing.T) string {
	t.Helper()
	pages, err := filepath.Glob("../../site/content/*.md")
	if err != nil || len(pages) == 0 {
		t.Skipf("the site content is not here: %v", err)
	}
	var b strings.Builder
	for _, p := range pages {
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		b.Write(body)
		b.WriteString("\n")
	}
	return b.String()
}

// Every script an operator could be asked to run must be named on the site.
//
// These are the files a person types the name of. Being told to run
// `./service.sh install` by a colleague, and finding the documentation silent
// on it, is the moment somebody decides the docs are not worth reading.
func TestEveryOperatorFacingScriptIsDocumented(t *testing.T) {
	site := siteText(t)

	dir := "../../station/bash"
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("the station payload is not here: %v", err)
	}

	// Sourced libraries and internal helpers are deliberately exempt: nobody
	// runs caplib.sh, and documenting it as a command would be wrong. It is
	// covered as a library on the runner page instead.
	exempt := map[string]bool{
		"caplib.sh": true,
	}

	checked := 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") || exempt[e.Name()] {
			continue
		}
		checked++
		if !strings.Contains(site, e.Name()) {
			t.Errorf("station/bash/%s is a script an operator can run, and no site page names it", e.Name())
		}
	}
	if checked == 0 {
		t.Fatal("no station scripts were found, so this check asserted nothing")
	}
}

// The components that make heliograph what it is, each of which had no page at
// all before 2026-09-08. Named explicitly rather than derived, because the
// point is coverage of CAPABILITIES, and a capability is not a filename.
func TestTheFarSideIsDocumented(t *testing.T) {
	site := siteText(t)

	for _, want := range []struct{ what, phrase string }{
		{"the mode declaration gate", "heliograph-mode"},
		{"the read-only default", "ALLOW_ACTIONS"},
		{"the root refusal", "ALLOW_ROOT"},
		{"step pinning", "REQUIRE_PIN"},
		{"progress pushes", "PROGRESS_EVERY"},
		{"the undelivered state", "undelivered"},
		{"the finished-log verb", "tp_put_log"},
		{"the conformance suite", "conformance"},
		{"the Docker image", "heliograph-toolkit"},
		{"Kubernetes", "kubectl"},
		{"systemd", "systemd"},
		{"the Windows scheduled task", "scheduled task"},
		{"PowerShell steps", "ps_step"},
		{"redaction", "cap_redact"},
		{"sending a secret to the far side", "secret.sh"},
		{"the pipeline loop guard", "NO_CI"},
		{"the seal binary", "heliograph-seal"},
	} {
		if !strings.Contains(site, want.phrase) {
			t.Errorf("the site never mentions %s (looked for %q)", want.what, want.phrase)
		}
	}
}

// All five Azure templates ship, and all five must be findable. Four were
// deployed for real; the fifth never has been, and saying which is which is
// the part that is worth anything.
func TestEveryAzureTemplateIsDocumented(t *testing.T) {
	site := siteText(t)

	ents, err := os.ReadDir("../../station/bash/azure")
	if err != nil {
		t.Skipf("the Azure templates are not here: %v", err)
	}

	// How a page would name each directory in prose, since a reader searches
	// for the product name rather than the folder.
	naming := map[string]string{
		"aci":              "ACI",
		"webapp":           "Web App for Containers",
		"containerappsjob": "Container Apps Job",
		"vm":               "VM",
		"function":         "Function App",
	}

	checked := 0
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		checked++
		phrase, ok := naming[e.Name()]
		if !ok {
			t.Errorf("station/bash/azure/%s ships and this test has no name for it: "+
				"add one, and make sure a page documents it", e.Name())
			continue
		}
		if !strings.Contains(site, phrase) {
			t.Errorf("the Azure template %q ships and no site page mentions %q", e.Name(), phrase)
		}
	}
	if checked != 5 {
		t.Errorf("expected 5 Azure templates, found %d - the docs claim five", checked)
	}
}

// Every agent this repository ships an installer for must be documented.
//
// install-codex.sh has existed for as long as install.sh, and the site
// mentioned Codex in exactly one line of one page while Claude Code had a page
// of its own. An installer nobody can find is an installer nobody runs, and
// "which agents does this support" is the first question a reader asks.
func TestEveryShippedInstallerIsDocumented(t *testing.T) {
	site := siteText(t)

	// installer file -> the word a reader would search for.
	agents := map[string]string{
		"install.sh":       "Claude Code",
		"install-codex.sh": "Codex",
	}

	ents, err := os.ReadDir("../..")
	if err != nil {
		t.Skipf("the repository root is not here: %v", err)
	}
	found := 0
	for _, e := range ents {
		name := e.Name()
		if !strings.HasPrefix(name, "install") || !strings.HasSuffix(name, ".sh") {
			continue
		}
		found++
		agent, ok := agents[name]
		if !ok {
			t.Errorf("%s ships and this test has no agent name for it: add one, "+
				"and make sure a page documents that agent", name)
			continue
		}
		if !strings.Contains(site, agent) {
			t.Errorf("%s ships and no site page mentions %q", name, agent)
		}
		// And the installer itself has to be findable, or a reader is told the
		// agent is supported without being told how.
		if !strings.Contains(site, name) {
			t.Errorf("the site names %q as supported but never names %s, so there is "+
				"nothing to run", agent, name)
		}
	}
	if found == 0 {
		t.Fatal("no installers were found, so this check asserted nothing")
	}
}

// Every credential mechanism the station honours must be documented.
//
// The git transport's whole credential chain lived in caplib.sh and in a skill
// reference that was never published. The transports page - the page about the
// git transport - did not mention a token at all, and the mechanisms appeared
// only as asides on five other pages: containers, service, pipelines, azure and
// hosts. Somebody setting up their first station read the page named after
// their transport and was told nothing about the thing most likely to stop it
// working.
func TestEveryGitCredentialMechanismIsDocumented(t *testing.T) {
	site := siteText(t)

	dir := stationDir(t)
	caplib := read(t, filepath.Join(dir, "station", "bash", "caplib.sh"))

	// Read the names out of the implementation rather than listing them here,
	// so a mechanism added to the chain and not written up fails this test.
	// LONGEST ALTERNATIVE FIRST, and a word boundary. Written the obvious way
	// round, GIT_TOKEN matches first and GIT_TOKEN_FILE is only ever seen as
	// GIT_TOKEN followed by _FILE - so the check found two mechanisms out of
	// four and would have passed a page documenting half of them.
	mechanisms := regexp.MustCompile(`GIT_(?:AUTH_HEADER|TOKEN_FILE|TOKEN_USER|TOKEN)\b`).
		FindAllString(caplib, -1)
	seen := map[string]bool{}
	for _, m := range mechanisms {
		seen[m] = true
	}
	if len(seen) < 4 {
		t.Fatalf("expected four credential mechanisms in caplib.sh, found %d - "+
			"this check would assert almost nothing", len(seen))
	}
	for m := range seen {
		if !strings.Contains(site, m) {
			t.Errorf("the station honours %s and no site page names it", m)
		}
	}

	// The transports page is where somebody setting up git actually looks.
	tp, err := os.ReadFile("../../site/content/transports.md")
	if err != nil {
		t.Skipf("the transports page is not here: %v", err)
	}
	for _, want := range []string{"GIT_TOKEN", "ssh", "start.sh --check"} {
		if !strings.Contains(string(tp), want) {
			t.Errorf("the transports page never mentions %q, so the git section "+
				"does not tell a reader how the station authenticates", want)
		}
	}
}

// topLevel finds the commands main() actually dispatches on.
//
// Read from the switch rather than from `usage`, because usage is itself a
// piece of documentation and checking documentation against documentation
// proves only that two copies agree.
var topLevel = regexp.MustCompile(`(?m)^\tcase "([a-z]+)"(?:, "([a-z-]+)")?:`)

// Every command the binary dispatches must be on the CLI reference page.
//
// `station` was not. It shipped with its own subcommand, a worktree, an estate
// and printed operator instructions, and the page listing every command did not
// name it - so the only way to discover it was to read main.go or run the
// binary with no arguments.
func TestEveryCommandIsOnTheCLIPage(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Skipf("main.go is not here: %v", err)
	}
	// Only the dispatch switch in main(), which ends at its default arm.
	body := string(src)
	start := strings.Index(body, "switch os.Args[1] {")
	if start < 0 {
		t.Fatal("main() has no dispatch switch, so this check found nothing to assert")
	}
	end := strings.Index(body[start:], "\tdefault:")
	if end < 0 {
		t.Fatal("the dispatch switch has no default arm")
	}
	body = body[start : start+end]

	page, err := os.ReadFile("../../site/content/cli.md")
	if err != nil {
		t.Skipf("the CLI page is not here: %v", err)
	}
	doc := string(page)

	// Not commands: version reporting and the help flags, which no reference
	// page needs to teach.
	exempt := map[string]bool{"version": true, "help": true}

	found := 0
	for _, m := range topLevel.FindAllStringSubmatch(body, -1) {
		if exempt[m[1]] {
			continue
		}
		found++
		// An arm may carry an alias - `case "check", "doctor":`. Documenting
		// one canonical spelling is legitimate, so either satisfies this;
		// documenting NEITHER does not.
		names := []string{m[1]}
		if m[2] != "" {
			names = append(names, m[2])
		}
		ok := false
		for _, n := range names {
			if strings.Contains(doc, "heliograph "+n) {
				ok = true
			}
		}
		if !ok {
			t.Errorf("`heliograph %s` is a command and the CLI page names none of %v, "+
				"so the only way to find it is to read main.go", m[1], names)
		}
	}
	if found == 0 {
		t.Fatal("no commands were found in the dispatch switch, so this check asserted nothing")
	}
}

// Every field the station publishes in its status must be documented, because
// the CLI prints them and a reader meeting one it cannot interpret is the
// moment somebody guesses.
func TestEveryStatusFieldIsDocumented(t *testing.T) {
	site := siteText(t)
	dir := stationDir(t)
	station := read(t, filepath.Join(dir, "station", "bash", "station.sh"))

	// NOT anchored to the start of the line. Anchored, this matched only the
	// fields inside the plain `{ echo ...; }` blocks and silently missed every
	// conditional one - `[ -n "$PAYLOAD" ] && echo "payload:  ..."` among them,
	// which is the field this check was written for. It passed vacuously.
	fields := regexp.MustCompile(`echo "([a-z]+): *\$?`).FindAllStringSubmatch(station, -1)
	seen := map[string]bool{}
	for _, f := range fields {
		seen[f[1]] = true
	}
	if len(seen) < 5 {
		t.Fatalf("found only %d status fields in station.sh, so this asserts almost nothing", len(seen))
	}
	for f := range seen {
		if !strings.Contains(site, f) {
			t.Errorf("the station publishes a %q field and no site page mentions it", f)
		}
	}
}
