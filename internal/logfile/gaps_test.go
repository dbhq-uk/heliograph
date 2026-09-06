package logfile

import (
	"strings"
	"testing"
	"time"
)

// A gap belongs to the line BEFORE it. That is the whole point: the stamp on a
// line is when that line was produced, so a long interval means the operation
// named on the preceding line is what took the time. Attributing a gap to the
// line after it names the thing that finally finished, which is exactly the
// wrong end.
func TestGapIsAttributedToTheLineBeforeIt(t *testing.T) {
	log := `09:14:00 | ---------- terraform plan ----------
09:14:02 | Refreshing state...
09:17:14 | Plan: 3 to add
09:17:15 | done
`
	gaps, err := Gaps(strings.NewReader(log), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 {
		t.Fatalf("got %d gaps, want 1: %+v", len(gaps), gaps)
	}
	g := gaps[0]
	if g.After != "Refreshing state..." {
		t.Errorf("gap attributed to %q, want the line before it", g.After)
	}
	if g.Duration != 3*time.Minute+12*time.Second {
		t.Errorf("duration %v", g.Duration)
	}
	if g.Line != 2 {
		t.Errorf("line %d, want 2", g.Line)
	}
}

func TestThresholdExcludesOrdinaryIntervals(t *testing.T) {
	log := `10:00:00 | a
10:00:01 | b
10:00:02 | c
`
	gaps, err := Gaps(strings.NewReader(log), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Errorf("got %+v, want none", gaps)
	}
}

func TestGapsAreOrderedLongestFirst(t *testing.T) {
	log := `10:00:00 | a
10:00:20 | b
10:02:00 | c
10:02:30 | d
`
	gaps, err := Gaps(strings.NewReader(log), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 3 {
		t.Fatalf("got %d: %+v", len(gaps), gaps)
	}
	if gaps[0].Duration < gaps[1].Duration || gaps[1].Duration < gaps[2].Duration {
		t.Errorf("not longest first: %+v", gaps)
	}
}

// A run that crosses midnight is ordinary: these loops run for days. Without
// handling it, 23:59:59 -> 00:00:01 reads as a gap of minus twenty-four hours,
// and every real gap after it is wrong too.
func TestMidnightIsNotATwentyFourHourGap(t *testing.T) {
	log := `23:59:58 | before midnight
23:59:59 | still going
00:00:01 | after midnight
`
	gaps, err := Gaps(strings.NewReader(log), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Errorf("midnight was read as a gap: %+v", gaps)
	}
}

// Header and footer lines carry no timestamp column. They are not gaps and
// they must not break the parse.
func TestHeaderAndFooterAreIgnored(t *testing.T) {
	log := `============================================================
 net-probe
 started UTC : 20260906T091400Z
============================================================

09:14:00 | first
09:16:00 | second

============================================================
 finished UTC : 20260906T091600Z
 exit code    : 0
============================================================
`
	gaps, err := Gaps(strings.NewReader(log), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 || gaps[0].After != "first" {
		t.Errorf("got %+v", gaps)
	}
}

// A log with one stamped line has no intervals at all, and a log with none is
// not an error either: some steps produce nothing before failing.
func TestTooFewLinesIsNotAnError(t *testing.T) {
	for _, in := range []string{"", "09:14:00 | only\n", "no timestamps here\n"} {
		if _, err := Gaps(strings.NewReader(in), time.Second); err != nil {
			t.Errorf("%q: %v", in, err)
		}
	}
}

// The single most useful property of these logs is that every line carries a
// distinct stamp. A log where they are all identical is a broken capture, and
// saying "no gaps found" about it would be actively misleading: it looks like
// a clean run.
func TestAllStampsIdenticalIsReportedAsABrokenCapture(t *testing.T) {
	log := `09:14:00 | a
09:14:00 | b
09:14:00 | c
09:14:00 | d
`
	_, err := Gaps(strings.NewReader(log), time.Second)
	if err == nil {
		t.Fatal("a capture where every line carries the same stamp must be reported")
	}
	if !strings.Contains(err.Error(), "same") {
		t.Errorf("the error should say what is wrong with it: %v", err)
	}
}

func TestParseCountsStampedLines(t *testing.T) {
	log := "header\n09:14:00 | a\n09:14:01 | b\nfooter\n"
	n, err := CountStamped(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("got %d want 2", n)
	}
}
