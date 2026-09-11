package site

import (
	"strings"
	"testing"
)

// A pairing naming a transport that does not exist is a cell that silently
// does nothing: the picker never lights it, the table never prints it, and
// nothing anywhere says a word. This is the only thing that would notice.
func TestEveryPairingNamesATransportThatExists(t *testing.T) {
	known := map[string]bool{}
	for _, tr := range Transports {
		known[tr.ID] = true
	}
	for _, s := range Stations {
		for id := range s.Transports {
			if !known[id] {
				t.Errorf("station %q pairs with transport %q, which does not exist", s.ID, id)
			}
		}
	}
	for _, c := range Controllers {
		for id := range c.Transports {
			if !known[id] {
				t.Errorf("controller %q pairs with transport %q, which does not exist", c.ID, id)
			}
		}
	}
}

// Two rows with the same ID is two rows the picker cannot tell apart: the
// filter would light both and dim neither.
func TestIDsAreUnique(t *testing.T) {
	for _, group := range []struct {
		name string
		ids  []string
	}{
		{"transport", transportIDs()},
		{"station", stationIDs()},
		{"controller", controllerIDs()},
	} {
		seen := map[string]bool{}
		for _, id := range group.ids {
			if seen[id] {
				t.Errorf("duplicate %s id %q", group.name, id)
			}
			seen[id] = true
		}
	}
}

func transportIDs() []string {
	out := make([]string, 0, len(Transports))
	for _, t := range Transports {
		out = append(out, t.ID)
	}
	return out
}

func stationIDs() []string {
	out := make([]string, 0, len(Stations))
	for _, s := range Stations {
		out = append(out, s.ID)
	}
	return out
}

func controllerIDs() []string {
	out := make([]string, 0, len(Controllers))
	for _, c := range Controllers {
		out = append(out, c.ID)
	}
	return out
}

// A transport with no station side cannot work however the picker lights up,
// so the table has to say so in the column for that side. This asserts the
// claim the page makes about itself.
func TestATransportMissingAHalfSaysWhichHalf(t *testing.T) {
	html := Matrix()
	for _, tr := range Transports {
		if tr.Station != Missing {
			continue
		}
		if !strings.Contains(html, "no station side") {
			t.Fatalf("transport %q has no station side and the matrix never says so", tr.ID)
		}
	}
}

// The whole matrix has to be in the HTML before a line of script runs. Most
// of this site's readers are agents, and a picker that assembles itself on
// click is an empty page to every one of them.
func TestTheMatrixIsCompleteWithoutJavaScript(t *testing.T) {
	html := Matrix()
	for _, tr := range Transports {
		if !strings.Contains(html, esc(tr.Name)) {
			t.Errorf("transport %q is not in the rendered matrix", tr.Name)
		}
	}
	for _, s := range Stations {
		if !strings.Contains(html, esc(s.Name)) {
			t.Errorf("station %q is not in the rendered matrix", s.Name)
		}
	}
	for _, c := range Controllers {
		if !strings.Contains(html, esc(c.Name)) {
			t.Errorf("controller %q is not in the rendered matrix", c.Name)
		}
	}
	// Nothing may start dimmed, because dimming is what a selection means.
	if strings.Contains(html, "data-off") {
		t.Error("the matrix ships with something already dimmed, so a reader with no script sees a filtered view")
	}
}

// Both shapes have to appear, or the page's central claim is decoration. The
// intercom is the only member of its kind and would be the one lost.
func TestBothTransportShapesArePresent(t *testing.T) {
	var pig, ic int
	for _, tr := range Transports {
		switch tr.Kind {
		case Pigeonhole:
			pig++
		case Intercom:
			ic++
		default:
			t.Errorf("transport %q has no shape", tr.ID)
		}
	}
	if pig == 0 || ic == 0 {
		t.Fatalf("want both shapes represented, got %d pigeonhole and %d intercom", pig, ic)
	}
}

// The picker filters both ways, so the link data has to be symmetric. If a
// station lists a transport but that transport does not list the station,
// clicking the station lights the transport and clicking the transport dims
// the station - the same pair, two different answers, depending which column
// the reader started in. That is the defect a user would actually hit.
func TestThePickerFiltersTheSameBothWays(t *testing.T) {
	links := map[string]map[string]bool{}
	for _, group := range [][]cell{transportCells(), stationCells(), controllerCells()} {
		for _, c := range group {
			links[c.id] = map[string]bool{}
			for _, l := range c.links {
				links[c.id][l] = true
			}
		}
	}
	for id, out := range links {
		for target := range out {
			back, known := links[target]
			if !known {
				t.Errorf("%s links to %s, which is not in the picker at all", id, target)
				continue
			}
			if !back[id] {
				t.Errorf("%s lights %s, but %s does not light %s back", id, target, target, id)
			}
		}
	}
}

// A transport with no station side must light no station, or the picker
// promises a combination that cannot run. The object store and the bundle are
// the two that would.
func TestATransportWithNoStationSideLightsNoStation(t *testing.T) {
	byID := map[string]Transport{}
	for _, tr := range Transports {
		byID[tr.ID] = tr
	}
	for _, c := range transportCells() {
		tr := byID[strings.TrimPrefix(c.id, "t:")]
		if tr.Station != Missing {
			continue
		}
		for _, l := range c.links {
			if strings.HasPrefix(l, "s:") {
				t.Errorf("%q has no station side but the picker lights station %q", tr.Name, strings.TrimPrefix(l, "s:"))
			}
		}
	}
}
