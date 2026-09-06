// Package wire is the request and status documents that cross the gap, and
// nothing else. It has no knowledge of how they travel.
//
// The format is `key: value`, one per line, because a transport repo is read
// by an operator on a machine nobody here can reach. When a loop is not doing
// what somebody expected, the first useful act is `cat station/request`, and
// that has to be legible without tooling, a schema, or us.
//
// JSON would be tidier to parse and worse at the only job that matters.
package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Version is the protocol this build speaks. It is written into every request
// so that a station can refuse a document it does not understand rather than
// guess at it. Stations live in the field and the CLI will update faster than
// they do.
const Version = 1

// Request is what the control side publishes and the station acts on.
//
// The trigger is ID and only ID. Documentation, step edits and payload changes
// land on a task branch constantly; if any change fired a run, the station
// would run on all of them.
type Request struct {
	Version int
	ID      string // the trigger. A change here, and only here, starts a run
	Step    string // a registered name, or a path. Blank means the default
	Env     string // extra environment for the run, verbatim shell text
	Cancel  string // "yes" kills whatever is running; an id kills only that run
	Stop    string // "yes" ends the loop cleanly after the current run
	Note    string // free text for the next human. The station ignores it
}

// order fixes the sequence keys are written in. Stable output means a diff
// between two requests shows what changed rather than everything moving.
var order = []string{"version", "id", "step", "env", "cancel", "stop", "note"}

func (r Request) field(k string) string {
	switch k {
	case "version":
		if r.Version == 0 {
			return ""
		}
		return strconv.Itoa(r.Version)
	case "id":
		return r.ID
	case "step":
		return r.Step
	case "env":
		return r.Env
	case "cancel":
		return r.Cancel
	case "stop":
		return r.Stop
	case "note":
		return r.Note
	}
	return ""
}

// Validate refuses anything that would forge a second key.
//
// A newline inside a value ends the line the station is reading and starts one
// it was never sent. `id: x\nstop: yes` would stop a loop on a machine nobody
// can reach, and it would look like the loop had simply died. Refused here,
// where there is somebody to tell.
func (r Request) Validate() error {
	for _, k := range order {
		v := r.field(k)
		if strings.ContainsAny(v, "\n\r") {
			return fmt.Errorf("wire: %s contains a newline, which would forge a second key: %q", k, v)
		}
	}
	if r.ID == "" {
		return fmt.Errorf("wire: a request needs an id, which is the only thing that triggers a run")
	}
	return nil
}

// Marshal renders the document.
//
// Every key is written even when its value is empty. sed does not need them,
// but the operator reading the file does: an absent `cancel:` gives no hint
// that cancelling is a thing they could do.
func (r Request) Marshal() []byte {
	var b bytes.Buffer
	for _, k := range order {
		if v := r.field(k); v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		} else {
			fmt.Fprintf(&b, "%s:\n", k)
		}
	}
	return b.Bytes()
}

// ParseRequest reads the document back.
//
// Unknown keys are ignored and missing keys are zero. Neither is an error: a
// station newer than this CLI will write keys we have never heard of, and
// refusing to read it would make the newer side unreadable by the older one,
// on exactly the machine nobody can reach to upgrade.
func ParseRequest(b []byte) (Request, error) {
	var r Request
	s := bufio.NewScanner(bytes.NewReader(b))
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for s.Scan() {
		k, v, ok := splitField(s.Text())
		if !ok {
			continue
		}
		switch k {
		case "version":
			r.Version, _ = strconv.Atoi(v)
		case "id":
			r.ID = v
		case "step":
			r.Step = v
		case "env":
			r.Env = v
		case "cancel":
			r.Cancel = v
		case "stop":
			r.Stop = v
		case "note":
			r.Note = v
		}
	}
	return r, s.Err()
}

// splitField splits on the FIRST colon only.
//
// Values carry colons routinely - a URL, a time, a PATH - and splitting on
// every one truncates them silently. The station splits on the first, so this
// does too. Disagreeing about that would be a bug nobody could see from either
// side of the gap.
func splitField(line string) (key, value string, ok bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
}

// NewID builds a trigger that sorts by time and says what it asked for.
//
// A bare timestamp would be enough to trigger a run and useless in a status
// line three days later. The step name is what makes `git log` readable.
func NewID(step string, t time.Time) string {
	stamp := t.UTC().Format("20060102T150405Z")
	name := strings.TrimSuffix(step, ".sh")
	name = strings.TrimSuffix(name, ".ps1")
	// A step given by path is ordinary, but a slash in an id reads as a path
	// and invites somebody to treat it as one.
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return stamp
	}
	return stamp + "-" + name
}
