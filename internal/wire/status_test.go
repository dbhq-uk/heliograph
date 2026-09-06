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
