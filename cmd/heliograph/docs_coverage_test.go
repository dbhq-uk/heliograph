package main

import (
	"os"
	"path/filepath"
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
