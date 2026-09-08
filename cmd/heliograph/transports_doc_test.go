package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A transport needs BOTH halves. The control side publishes a request and reads
// a log; the station side picks the request up and sends the log back. One half
// on its own moves nothing.
//
// The documentation drifted in exactly that gap. `site/content/transports.md`
// carried
//
//	heliograph init payments --transport relay --url ... --estate ...
//
// naming a transport `init` has never accepted and two flags that have never
// existed, while the status tables in the README and on the page called relay,
// file share, bundle and object store "works". internal/transport implements
// four of those; `station/bash/transports/` holds git, blob and relay, so no
// combination of the two was true of more than git.
//
// The cost of that drift is not a wrong sentence. Somebody plants a station
// against a transport that cannot carry a log, on a machine they cannot log
// into, and finds out by waiting.
//
// These tests read the shipped page and the shipped payload, because the
// mistake goes in that direction: a transport gets written, the page gets
// enthusiastic, and nothing disagrees.

var transportFlag = regexp.MustCompile(`--transport[ =]+([a-z]+)`)

// Nothing in the documentation may show `--transport X` unless `init` accepts X.
// This is the assertion that would have caught the relay example on the day it
// was written.
func TestDocsNeverShowATransportInitCannotSelect(t *testing.T) {
	known := map[string]bool{}
	for _, k := range initTransports {
		known[k] = true
	}
	if len(known) == 0 {
		t.Fatal("initTransports is empty, so this check could not fail")
	}

	pages, err := filepath.Glob("../../site/content/*.md")
	if err != nil || len(pages) == 0 {
		t.Skipf("the site content is not here: %v", err)
	}

	// A placeholder is not a claim. `--transport git|share|...` in a usage
	// block is describing the flag, not invoking it.
	checked := 0
	for _, p := range pages {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, m := range transportFlag.FindAllStringSubmatch(string(b), -1) {
			checked++
			if !known[m[1]] {
				t.Errorf("%s shows `--transport %s`, which `heliograph init` does not accept. "+
					"It knows %s", filepath.Base(p), m[1], strings.Join(initTransports, ", "))
			}
		}
	}
	if checked == 0 {
		t.Error("no --transport example was found on any page, so this check asserted nothing")
	}
}

// Every transport the station payload ships must be named on the transports
// page, and the page must not invent one the payload does not have. The station
// side is the half a reader cannot inspect from here, so it is the half most
// worth pinning.
func TestEveryStationTransportIsOnThePage(t *testing.T) {
	b, err := os.ReadFile("../../site/content/transports.md")
	if err != nil {
		t.Skipf("the site content is not here: %v", err)
	}
	page := string(b)

	shipped, err := filepath.Glob("../../station/bash/transports/*.sh")
	if err != nil || len(shipped) == 0 {
		t.Skipf("the station payload is not here: %v", err)
	}

	// The page is prose, so it names transports the way a reader would: "Azure
	// Blob" rather than "blob". Match on the word a human would write.
	spelling := map[string]string{
		"git":   "git",
		"blob":  "Azure Blob",
		"relay": "relay",
		"share": "file share",
	}
	for _, f := range shipped {
		name := strings.TrimSuffix(filepath.Base(f), ".sh")
		want, ok := spelling[name]
		if !ok {
			t.Errorf("the station ships transports/%s.sh and this test has no spelling for it: "+
				"add one, and make sure the page documents it", name)
			continue
		}
		if !strings.Contains(page, want) {
			t.Errorf("the station ships transports/%s.sh and the transports page never mentions %q",
				name, want)
		}
	}
}

// The status table has to name both sides. A single "works" column is what let
// a control-side-only transport read as finished.
func TestTheTransportsPageStatesBothSides(t *testing.T) {
	b, err := os.ReadFile("../../site/content/transports.md")
	if err != nil {
		t.Skipf("the site content is not here: %v", err)
	}
	page := string(b)
	for _, want := range []string{"control side", "station side"} {
		if !strings.Contains(page, want) {
			t.Errorf("the transports page never says %q, so a reader cannot tell "+
				"which half of a transport exists", want)
		}
	}
}
