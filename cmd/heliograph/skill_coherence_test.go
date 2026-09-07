package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This repository makes claims about the far side, and the far side lives in
// another repository on its own release schedule.
//
// The MCP tool descriptions are the sharp end. A model reads them and decides
// what to do, so "an action step needs CONFIRM=yes and a station started with
// --allow-actions" is not documentation, it is an instruction that will be
// followed literally. If the station ever spelled that gate differently, the
// model would send a request that is refused, read the refusal as a fault, and
// retry it forever on a machine nobody here can reach.
//
// Nothing else catches this. Every test in both repositories checks each half
// against its own idea of the contract, which both halves can satisfy while
// disagreeing with each other.
//
// It reads the real skill, checked out beside this repo in CI, and skips when
// it is absent. The CI job asserts the skip did not happen: a silently skipped
// cross-repo check is the same shape of problem as a green suite that checked
// nothing.

// skillDir lives in e2e_test.go. Shared deliberately: both this and the
// end-to-end test resolve the same checkout the same way, and two helpers that
// disagreed about what counts as "the skill repo" would be a bug in the thing
// meant to catch bugs.

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return string(b)
}

// The gate strings this side tells a model to use must be the strings the
// station actually looks for.
func TestGateSpellingsMatchTheStation(t *testing.T) {
	dir := skillDir(t)
	station := read(t, filepath.Join(dir, "skills", "heliograph", "toolkit", "station.sh"))
	runSh := read(t, filepath.Join(dir, "skills", "heliograph", "toolkit", "run.sh"))
	far := station + runSh

	// Everything this side says about the gates, gathered from the places that
	// say it, so adding a claim somewhere new does not escape the check.
	var near strings.Builder
	for _, tl := range tools() {
		near.WriteString(tl.Description)
		near.WriteString("\n")
	}
	for _, f := range []string{"mcp.md", "method.md", "claude-code.md", "cli.md"} {
		p := filepath.Join("..", "..", "site", "content", f)
		if b, err := os.ReadFile(p); err == nil {
			near.Write(b)
		}
	}
	nearText := near.String()

	for _, spell := range []struct{ token, why string }{
		{"--allow-actions", "the flag that lets the station run a step that changes state"},
		{"CONFIRM", "the confirmation the request must carry"},
	} {
		if !strings.Contains(nearText, spell.token) {
			t.Errorf("this side never mentions %q (%s), so a model is not told about it",
				spell.token, spell.why)
			continue
		}
		if !strings.Contains(far, spell.token) {
			t.Errorf("this side tells a model to use %q (%s), but the station does not look for it.\n"+
				"A request built on that instruction would be refused, and the reason would point at "+
				"a spelling nobody can see from here.", spell.token, spell.why)
		}
	}
}

// The files the two sides exchange. A rename on the far side that this side
// does not learn about is a station that goes quiet while looking healthy.
func TestWirePathsMatchTheStation(t *testing.T) {
	dir := skillDir(t)
	station := read(t, filepath.Join(dir, "skills", "heliograph", "toolkit", "station.sh"))

	for _, path := range []string{"station/request", "station/status"} {
		if !strings.Contains(station, path) {
			t.Errorf("the station no longer names %q. This side writes it: see internal/transport.", path)
		}
	}
}

// The station publishes these and this side reports them. A state this side
// does not know about is printed as a bare string a reader has to interpret,
// which is the moment somebody guesses.
func TestStatusStatesAreAllKnownHere(t *testing.T) {
	dir := skillDir(t)
	station := read(t, filepath.Join(dir, "skills", "heliograph", "toolkit", "station.sh"))

	// wire.Status documents these, and heliograph_status names them to a model.
	known := []string{"running", "idle", "cancelled", "refused", "stopped"}
	for _, state := range known {
		if !strings.Contains(station, state) {
			t.Errorf("this side tells a model the state can be %q, but the station never publishes it", state)
		}
	}
}

// The claim that the CLI and a hand-edited request are interchangeable is the
// reason the skill can tell an operator to use either. It rests on the station
// treating the id as the only trigger, which is asserted here rather than
// assumed, because the whole interoperability story falls over without it.
func TestTheIDIsTheTrigger(t *testing.T) {
	dir := skillDir(t)
	station := read(t, filepath.Join(dir, "skills", "heliograph", "toolkit", "station.sh"))
	if !strings.Contains(station, "id") {
		t.Fatal("the station does not read an id, so nothing this side sends would trigger a run")
	}
}
