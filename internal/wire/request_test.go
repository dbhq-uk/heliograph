package wire

import (
	"strings"
	"testing"
	"time"
)

// The format is not ours to choose. station.sh reads it with
//
//	sed -n "s/^id:[[:space:]]*//p"
//
// so a round trip through our own parser proves nothing on its own: the bytes
// have to be the shape that sed reads. These assertions are about the bytes.
func TestMarshalIsWhatTheStationParses(t *testing.T) {
	r := Request{
		Version: 1,
		ID:      "20260906T101500Z-net",
		Step:    "net-probe",
		Env:     `HOSTS="a b" PORTS=1433`,
	}
	got := string(r.Marshal())
	for _, want := range []string{
		"version: 1\n",
		"id: 20260906T101500Z-net\n",
		"step: net-probe\n",
		"env: HOSTS=\"a b\" PORTS=1433\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("marshal is missing %q\ngot:\n%s", want, got)
		}
	}
}

// station.sh reads `cancel:` and `stop:` on every poll. Omitting the keys
// entirely is fine for sed, but the file is also read by an operator who
// cannot ask us what else they could have set, so the full shape is always
// present.
func TestEmptyFieldsArePresentButBlank(t *testing.T) {
	got := string(Request{Version: 1, ID: "x"}.Marshal())
	for _, want := range []string{"cancel:\n", "stop:\n", "note:\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("marshal is missing %q\ngot:\n%s", want, got)
		}
	}
}

// The id is the trigger, so two requests a second apart must differ, and a
// directory of them should sort into the order they were sent.
func TestNewIDIsSortableAndNamesTheStep(t *testing.T) {
	ts := time.Date(2026, 9, 6, 10, 15, 0, 0, time.UTC)
	if got, want := NewID("net-probe", ts), "20260906T101500Z-net-probe"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// A step given by path is ordinary - run.sh accepts one - but a slash in an id
// reads as a path and invites someone to treat it as one.
func TestNewIDFlattensAStepGivenByPath(t *testing.T) {
	ts := time.Date(2026, 9, 6, 10, 15, 0, 0, time.UTC)
	got := NewID("steps/net-probe.sh", ts)
	if strings.Contains(got, "/") {
		t.Errorf("id contains a slash: %q", got)
	}
	if !strings.Contains(got, "net-probe") {
		t.Errorf("id no longer names the step: %q", got)
	}
}

// A station in the field may write keys this build has never heard of, and an
// operator may hand-edit one out. Neither is an error: refusing would make a
// newer station unreadable by an older CLI, on the one machine nobody can
// reach to upgrade.
func TestParseToleratesExtraAndMissingKeys(t *testing.T) {
	r, err := ParseRequest([]byte("id: abc\nstep: env\nfuture: 42\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "abc" || r.Step != "env" {
		t.Errorf("got %+v", r)
	}
}

// `env:` carries shell text with colons in it - a URL, a time, a path list -
// and splitting on every colon would silently truncate it. The station splits
// on the FIRST colon only, and so must we.
func TestParseKeepsColonsInsideAValue(t *testing.T) {
	r, err := ParseRequest([]byte("env: URL=https://h:8443/x TZ=10:30\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "URL=https://h:8443/x TZ=10:30"; r.Env != want {
		t.Errorf("got %q want %q", r.Env, want)
	}
}

func TestRoundTrip(t *testing.T) {
	in := Request{Version: 1, ID: "i", Step: "s", Env: "E=1", Note: "why"}
	out, err := ParseRequest(in.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Errorf("got %+v want %+v", out, in)
	}
}

// A newline in a value would forge a second key. The id and step reach a shell
// on a machine nobody can reach, so this fails loudly here rather than there.
func TestMarshalRefusesAValueContainingANewline(t *testing.T) {
	for _, r := range []Request{
		{ID: "a\nstop: yes"},
		{Step: "a\ncancel: yes"},
		{Env: "A=1\nstop: yes"},
	} {
		if err := r.Validate(); err == nil {
			t.Errorf("accepted a value with a newline: %+v", r)
		}
	}
}
