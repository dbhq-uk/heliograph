package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The documentation and the binary drift the moment nothing checks them. A page
// that names a tool which does not exist teaches an agent to call it, and the
// call fails on the far side of a client that cannot see why.
//
// Reading the shipped page rather than a fixture is the point: this fails when
// somebody adds a tool and forgets the docs, which is the direction the mistake
// actually goes.

var tableRow = regexp.MustCompile("(?m)^\\| `(heliograph_[a-z_]+)` \\|")

func TestDocsListEveryToolAndNoOthers(t *testing.T) {
	b, err := os.ReadFile("../../site/content/mcp.md")
	if err != nil {
		t.Skipf("the site content is not here: %v", err)
	}
	documented := map[string]bool{}
	for _, m := range tableRow.FindAllStringSubmatch(string(b), -1) {
		documented[m[1]] = true
	}
	if len(documented) == 0 {
		t.Fatal("the page has no tool table, so this check would pass on an empty page")
	}

	shipped := map[string]bool{}
	for _, tl := range tools() {
		shipped[tl.Name] = true
		if !documented[tl.Name] {
			t.Errorf("%s ships but is in no table on the mcp page", tl.Name)
		}
	}
	for name := range documented {
		if !shipped[name] {
			t.Errorf("the mcp page names %s, which does not exist", name)
		}
	}
}

// The install line on the page has to be the command that actually works. This
// is the first thing anybody types and the last thing anybody re-reads.
func TestDocsInstallLineMatchesTheSubcommand(t *testing.T) {
	b, err := os.ReadFile("../../site/content/mcp.md")
	if err != nil {
		t.Skipf("the site content is not here: %v", err)
	}
	if !strings.Contains(string(b), "heliograph mcp") {
		t.Error("the page never names the subcommand that starts the server")
	}
	// And the subcommand has to be reachable. A page documenting a command the
	// binary does not have is worse than no page.
	if !strings.Contains(usage, "heliograph mcp") {
		t.Error("`heliograph help` does not mention mcp")
	}
}
