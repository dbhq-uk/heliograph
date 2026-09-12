package wire

import (
	"strings"
	"testing"
)

// Taken verbatim from what station.sh's publish_progress writes. If this
// parses, the CLI and a real station agree; if it does not, they do not, and
// no amount of round-tripping our own output would have told us.
func TestParseStatusFromARealRunningStation(t *testing.T) {
	in := []byte(`state:    running
id:       20260906T101500Z-net
step:     net-probe
host:     box01.example
branch:   task/dns
utc:      2026-09-06T10:15:02Z
started:  2026-09-06T10:15:00Z
progress: 412 lines
log:      ops-logs/net-probe-20260906T101500Z.txt
last:     11:31:29 | ---------- openssl s_client -connect hostb:443 ----------
`)
	s, err := ParseStatus(in)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != "running" {
		t.Errorf("state: got %q", s.State)
	}
	if s.Step != "net-probe" {
		t.Errorf("step: got %q", s.Step)
	}
	if s.Log != "ops-logs/net-probe-20260906T101500Z.txt" {
		t.Errorf("log: got %q", s.Log)
	}
	// `last:` is the field that says where a long run has got to, and it is
	// full of colons: a timestamp, a host and port, a divider. Splitting on
	// every colon would cut it at "11" and throw away the probe name, which is
	// the only part anybody reads it for.
	if !strings.HasSuffix(s.Last, "openssl s_client -connect hostb:443 ----------") {
		t.Errorf("last was truncated: %q", s.Last)
	}
}

func TestParseStatusFromAFinishedRun(t *testing.T) {
	in := []byte(`state:    idle
id:       20260906T101500Z-net
step:     net-probe
host:     box01.example
branch:   task/dns
utc:      2026-09-06T10:22:41Z
started:  2026-09-06T10:15:00Z
finished: 2026-09-06T10:22:40Z
exit:     0
log:      ops-logs/net-probe-20260906T101500Z.txt
`)
	s, err := ParseStatus(in)
	if err != nil {
		t.Fatal(err)
	}
	if s.Exit != "0" || s.Finished == "" {
		t.Errorf("got %+v", s)
	}
	if !s.Done() {
		t.Error("idle must be terminal")
	}
}

// Which states end a run decides when `watch` stops. Getting `refused` wrong
// would leave the CLI waiting forever for a log that is never coming, which is
// the exact round trip this tooling exists to save.
func TestDoneCoversEveryStateTheStationPublishes(t *testing.T) {
	for _, tc := range []struct {
		state string
		done  bool
	}{
		{"running", false},
		{"idle", true},
		{"cancelled", true},
		{"refused", true},
		{"stopped", true},
		{"", false}, // never published; treat an unknown state as still working
	} {
		if got := (Status{State: tc.state}).Done(); got != tc.done {
			t.Errorf("%q: got %v want %v", tc.state, got, tc.done)
		}
	}
}

// A refusal is not a failure, and reporting it as one sends the reader looking
// for a broken step instead of a missing flag.
func TestRefusedIsDistinguishableFromFailed(t *testing.T) {
	if !(Status{State: "refused"}).Refused() {
		t.Error("refused not recognised")
	}
	if (Status{State: "idle", Exit: "1"}).Refused() {
		t.Error("a failed run reported as a refusal")
	}
}

// The station's action mode is a property of how the process was started, not
// of the request, so it reaches this side only if the station publishes it.
// Taken verbatim from what station.sh's publish_status writes.
func TestTheActionModeParsesBothValuesAStationPublishes(t *testing.T) {
	for _, tc := range []struct {
		actions          string
		allowed, refused bool
	}{
		{"allowed", true, false},
		{"refused", false, true},
	} {
		in := []byte(`state:    idle
id:       20260912T101500Z-net
step:     net-probe
host:     box01.example
branch:   task/dns
utc:      2026-09-12T10:22:41Z
actions:  ` + tc.actions + `
exit:     0
`)
		s, err := ParseStatus(in)
		if err != nil {
			t.Fatal(err)
		}
		if s.Actions != tc.actions {
			t.Errorf("actions: got %q want %q", s.Actions, tc.actions)
		}
		if s.ActionsAllowed() != tc.allowed {
			t.Errorf("%q: ActionsAllowed got %v want %v", tc.actions, s.ActionsAllowed(), tc.allowed)
		}
		if s.ActionsRefused() != tc.refused {
			t.Errorf("%q: ActionsRefused got %v want %v", tc.actions, s.ActionsRefused(), tc.refused)
		}
		if !s.ActionsReported() {
			t.Errorf("%q: a station that published its mode is reported as silent", tc.actions)
		}
	}
}

// THE CASE THIS FIELD EXISTS FOR, and the one a boolean gets wrong.
//
// Every station in the field today publishes no `actions:` line, and there is
// no version to ask. Silence is not read-only: the station may well allow
// actions and simply predate the field. A reader who is told "read-only" by a
// station that is not is being told the opposite of the truth in the exact
// place somebody decides whether an estate is safe to point at.
//
// So the absent case has to be distinguishable from both published values, in
// the same way `undelivered` is kept distinct from a failed step at
// status.go:102-117. Two booleans, both false, and Reported false to say why.
func TestAStatusWithNoActionModeIsNotReadOnly(t *testing.T) {
	in := []byte(`state:    idle
id:       20260912T101500Z-net
step:     net-probe
host:     box01.example
branch:   task/dns
utc:      2026-09-12T10:22:41Z
exit:     0
`)
	s, err := ParseStatus(in)
	if err != nil {
		t.Fatal(err)
	}
	if s.Actions != "" {
		t.Errorf("a field nobody published came out as %q", s.Actions)
	}
	if s.ActionsAllowed() {
		t.Error("a station that said nothing is reported as allowing actions")
	}
	if s.ActionsRefused() {
		t.Error("a station that said nothing is reported as read-only, which it may not be")
	}
	if s.ActionsReported() {
		t.Error("silence is reported as an answer")
	}
}

// A newer station may publish a value this build has not heard of, exactly as
// an unknown state may arrive. Guessing is the failure mode here too, and the
// safe guess does not exist: "allowed" overstates it and "refused" understates
// it. Neither predicate fires, the raw value is kept so the CLI can show it,
// and the caller is told the answer is not one it understands.
func TestAnUnrecognisedActionModeIsNotReadOnly(t *testing.T) {
	s, err := ParseStatus([]byte("state:    idle\nactions:  supervised\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Actions != "supervised" {
		t.Errorf("the raw value was lost: got %q", s.Actions)
	}
	if s.ActionsAllowed() {
		t.Error("an unrecognised mode is reported as allowing actions")
	}
	if s.ActionsRefused() {
		t.Error("an unrecognised mode is reported as read-only, which it may not be")
	}
	if s.ActionsReported() {
		t.Error("an unrecognised mode is reported as an answer this build understands")
	}
}

// The mode is a property of the station's whole lifetime, so it is published
// on every transition and while a step runs. A field that arrives only at the
// end is one the fleet view cannot show for a station mid-run.
func TestTheActionModeSurvivesAProgressStatus(t *testing.T) {
	in := []byte(`state:    running
id:       20260912T101500Z-net
step:     net-probe
host:     box01.example
branch:   task/dns
utc:      2026-09-12T10:15:02Z
started:  2026-09-12T10:15:00Z
actions:  refused
progress: 412 lines
log:      ops-logs/net-probe-20260912T101500Z.txt
`)
	s, err := ParseStatus(in)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ActionsRefused() {
		t.Errorf("a running station's mode was lost: %+v", s)
	}
	if !s.Running() {
		t.Error("adding the mode disturbed Running")
	}
}

// Alive is what tells "nobody started this" from "it is up and idle", and the
// action mode is published alongside it. A field added to the same document
// must not change which states count as alive.
func TestTheActionModeDoesNotDisturbAlive(t *testing.T) {
	for _, tc := range []struct {
		state string
		alive bool
	}{
		{"running", true},
		{"starting", true},
		{"idle", false},
		{"refused", false},
	} {
		s := Status{State: tc.state, Actions: "allowed"}
		if got := s.Alive(); got != tc.alive {
			t.Errorf("%q with a mode published: Alive got %v want %v", tc.state, got, tc.alive)
		}
	}
}

func TestParseStatusOfEmptyInputIsNotAnError(t *testing.T) {
	// A station that has never run publishes nothing. That is a state, not a
	// fault, and the caller distinguishes it by State being empty.
	s, err := ParseStatus(nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != "" {
		t.Errorf("got %+v", s)
	}
}
