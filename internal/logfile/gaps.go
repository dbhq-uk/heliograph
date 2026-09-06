// Package logfile reads a captured log and finds the gaps in its timestamp
// column.
//
// "Scan the timestamp column for gaps before reading the content" is the
// single most valuable instruction in the heliograph method, and it has always
// been a discipline somebody has to remember. It is arithmetic. This does it.
//
// The reason it matters: after the fact, in an untimed log, a hang and slow
// progress are indistinguishable. With a clock on every line, a gap in the
// column tells you exactly which operation stalled and for how long.
package logfile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Gap is an interval between two captured lines, attributed to the operation
// that was running during it.
type Gap struct {
	Duration time.Duration
	At       string // the timestamp the gap started from
	After    string // the line before the gap: what was running
	Next     string // the line that ended it
	Line     int    // 1-based line number of After, within the stamped lines
}

// stamped splits a captured line into its timestamp and content.
//
// The format is `HH:MM:SS | text`, written by cap_run. A line without that
// separator is a header, a footer or a divider, and carries no time.
func stamped(line string) (hhmmss, text string, ok bool) {
	i := strings.Index(line, " | ")
	if i != 8 {
		return "", "", false
	}
	ts := line[:8]
	if ts[2] != ':' || ts[5] != ':' {
		return "", "", false
	}
	return ts, line[i+3:], true
}

func secondsOf(hhmmss string) (int, bool) {
	var h, m, s int
	if _, err := fmt.Sscanf(hhmmss, "%02d:%02d:%02d", &h, &m, &s); err != nil {
		return 0, false
	}
	if h > 23 || m > 59 || s > 59 {
		return 0, false
	}
	return h*3600 + m*60 + s, true
}

type entry struct {
	secs int
	ts   string
	text string
}

func read(r io.Reader) ([]entry, error) {
	var out []entry
	sc := bufio.NewScanner(r)
	// Captured lines can be long: a terraform plan, a base64 blob, a stack
	// trace. The default 64KB limit would stop the scan partway and report a
	// clean result for a truncated read, which is the worst of both.
	sc.Buffer(make([]byte, 0, 256*1024), 8*1024*1024)
	for sc.Scan() {
		ts, text, ok := stamped(sc.Text())
		if !ok {
			continue
		}
		secs, ok := secondsOf(ts)
		if !ok {
			continue
		}
		out = append(out, entry{secs: secs, ts: ts, text: text})
	}
	return out, sc.Err()
}

// CountStamped reports how many lines carry a timestamp.
func CountStamped(r io.Reader) (int, error) {
	e, err := read(r)
	return len(e), err
}

// Gaps returns every interval at or above min, longest first.
//
// A gap is attributed to the line BEFORE it. The stamp on a line is when that
// line was produced, so a long interval means the operation named on the
// preceding line is what took the time. Attributing it to the line after names
// the thing that finally finished, which is the wrong end and sends the reader
// to the wrong place.
func Gaps(r io.Reader, min time.Duration) ([]Gap, error) {
	es, err := read(r)
	if err != nil {
		return nil, err
	}
	if len(es) < 2 {
		// One stamped line has no intervals, and none at all is ordinary for a
		// step that failed before printing anything. Neither is an error.
		return nil, nil
	}

	// A capture where every line carries the same stamp is broken, and it is
	// the specific way a buffered `sed` fails: the log reads perfectly. Saying
	// "no gaps found" about it would be worse than saying nothing, because it
	// looks like a clean run and it is the exact opposite.
	same := true
	for _, e := range es[1:] {
		if e.secs != es[0].secs {
			same = false
			break
		}
	}
	if same {
		return nil, errors.New(
			"every line in this log carries the same timestamp, so the capture was buffered: " +
				"a hang and slow progress are indistinguishable in it. On the far side, check that " +
				"`sed -u` is available (busybox sed has no -u) and re-run the step")
	}

	var gaps []Gap
	for i := 1; i < len(es); i++ {
		d := es[i].secs - es[i-1].secs
		// A run crossing midnight is ordinary: these loops go for days. Without
		// this, 23:59:59 -> 00:00:01 reads as minus twenty-four hours and every
		// gap after it is wrong too.
		if d < 0 {
			d += 24 * 3600
		}
		dur := time.Duration(d) * time.Second
		if dur < min {
			continue
		}
		gaps = append(gaps, Gap{
			Duration: dur,
			At:       es[i-1].ts,
			After:    es[i-1].text,
			Next:     es[i].text,
			Line:     i,
		})
	}
	sort.SliceStable(gaps, func(a, b int) bool {
		return gaps[a].Duration > gaps[b].Duration
	})
	return gaps, nil
}

// Format renders one gap for a terminal.
func (g Gap) Format() string {
	text := g.After
	if len(text) > 96 {
		text = text[:96] + "..."
	}
	return fmt.Sprintf("%8s  after  %s | %s", short(g.Duration), g.At, text)
}

func short(d time.Duration) string {
	s := int(d.Seconds())
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm%02ds", s/60, s%60)
	default:
		return fmt.Sprintf("%dh%02dm", s/3600, (s%3600)/60)
	}
}
