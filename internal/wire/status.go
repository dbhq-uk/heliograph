package wire

import (
	"bufio"
	"bytes"
)

// Status is what the station publishes on every transition, so that the far
// side can tell "running for four minutes" from "never woke up".
//
// Without it, the two are indistinguishable until a log appears, and a station
// that refused a request looks exactly like one that is still thinking about
// it. That ambiguity costs a whole round trip through somebody who cannot
// debug the machine, which is the thing this project exists to prevent.
type Status struct {
	State    string // running | idle | cancelled | refused | stopped
	ID       string // the request it is working on
	Step     string
	Host     string
	Branch   string
	UTC      string
	Started  string
	Finished string
	Exit     string
	Log      string // path to the captured log, once there is one
	Progress string // "412 lines", while a step runs
	Last     string // the last real line of the log. Usually the probe in flight
	Reason   string // why, when the state is refused
}

// ParseStatus reads a status document.
//
// Empty input is not an error. A station that has never run publishes nothing,
// and that is a state the caller wants to distinguish rather than a fault:
// State is left empty and the caller decides what to say about it.
func ParseStatus(b []byte) (Status, error) {
	var s Status
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		k, v, ok := splitField(sc.Text())
		if !ok {
			continue
		}
		switch k {
		case "state":
			s.State = v
		case "id":
			s.ID = v
		case "step":
			s.Step = v
		case "host":
			s.Host = v
		case "branch":
			s.Branch = v
		case "utc":
			s.UTC = v
		case "started":
			s.Started = v
		case "finished":
			s.Finished = v
		case "exit":
			s.Exit = v
		case "log":
			s.Log = v
		case "progress":
			s.Progress = v
		case "last":
			s.Last = v
		case "reason":
			s.Reason = v
		}
	}
	return s, sc.Err()
}

// Done reports whether this run has ended, whatever the outcome.
//
// `refused` and `cancelled` are terminal and easy to forget. Treating either
// as still-working would leave a caller waiting for a log that is never
// coming, and it would wait quietly, which is worse.
//
// An unknown state is deliberately NOT terminal. A newer station may publish
// something this build has not heard of, and guessing that an unrecognised
// state means "finished" would abandon a run that is still going.
func (s Status) Done() bool {
	switch s.State {
	case "idle", "cancelled", "refused", "stopped":
		return true
	}
	return false
}

// Refused separates "the station would not do this" from "the step failed".
//
// They need different sentences. A refusal names a flag somebody has to pass;
// a failure names a thing that went wrong on the far side. Reporting the first
// as the second sends the reader hunting for a broken step.
func (s Status) Refused() bool { return s.State == "refused" }

// Running is true while a step is in flight.
func (s Status) Running() bool { return s.State == "running" }
