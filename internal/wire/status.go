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
	Payload  string // digest of the station payload that is running
	Progress string // "412 lines", while a step runs
	Last     string // the last real line of the log. Usually the probe in flight
	Reason   string // why, when the state is refused
	Actions  string // allowed | refused; empty when the station does not publish it
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
		case "payload":
			s.Payload = v
		case "progress":
			s.Progress = v
		case "last":
			s.Last = v
		case "reason":
			s.Reason = v
		case "actions":
			s.Actions = v
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
// `undelivered` is terminal for the same reason and is the sharpest case of
// it: the run finished, the log is complete, and the transport would not take
// it. Waiting is precisely the wrong response, because nothing further is
// going to arrive.
//
// An unknown state is deliberately NOT terminal. A newer station may publish
// something this build has not heard of, and guessing that an unrecognised
// state means "finished" would abandon a run that is still going.
func (s Status) Done() bool {
	switch s.State {
	case "idle", "cancelled", "refused", "stopped", "undelivered":
		return true
	}
	return false
}

// Undelivered separates "the log could not be shipped" from "the step failed"
// and from "nothing has happened yet".
//
// All three look the same to somebody watching for a log that does not come,
// and the reader's next move differs completely: a failed step is read in the
// log, a silent station is chased, and an undelivered log is fetched by the
// one person who can reach the machine. Saying which it is costs a sentence
// and saves a round trip through them.
func (s Status) Undelivered() bool { return s.State == "undelivered" }

// Refused separates "the station would not do this" from "the step failed".
//
// They need different sentences. A refusal names a flag somebody has to pass;
// a failure names a thing that went wrong on the far side. Reporting the first
// as the second sends the reader hunting for a broken step.
func (s Status) Refused() bool { return s.State == "refused" }

// Running is true while a step is in flight.
func (s Status) Running() bool { return s.State == "running" }

// --- the action mode ---------------------------------------------------------
//
// `actions:` is the one field here that is not about the run. Whether a station
// will run a step that changes state is fixed when the process starts, by
// `--allow-actions`, and it holds for the station's whole lifetime. Nothing on
// the request carries it, so before this field the only way to answer "can this
// station make changes" was to infer it from history, and history is wrong in
// both directions: a station restarted without the flag still has action logs,
// and one started with it may never have been asked.
//
// THREE ANSWERS, NOT TWO, and the third is the point. A boolean has to put
// silence somewhere, and both places are a lie told to somebody deciding
// whether an estate is safe to point at. Every station in the field today
// publishes nothing, and there is no version to ask.
//
// So ActionsAllowed and ActionsRefused are BOTH false for a station that said
// nothing, and both false for a value this build does not recognise.
// ActionsReported says which of those two silences it is. Callers render the
// raw Actions string when they want to show an unrecognised value verbatim.

// ActionsAllowed is true only when the station said so in as many words.
func (s Status) ActionsAllowed() bool { return s.Actions == "allowed" }

// ActionsRefused is true only when the station said so in as many words.
//
// NOT `!ActionsAllowed()`. Written that way it reported every station that has
// never heard of the field as read-only, which is the exact misrepresentation
// the field was added to prevent.
func (s Status) ActionsRefused() bool { return s.Actions == "refused" }

// ActionsReported says whether the station answered the question at all, in a
// value this build understands.
//
// A newer station may publish a mode this build has not heard of, just as it
// may publish an unknown state. There is no safe guess: "allowed" overstates
// it and "refused" understates it. Saying "not answered" sends the reader to
// the station, which is where the answer is.
func (s Status) ActionsReported() bool { return s.ActionsAllowed() || s.ActionsRefused() }

// Alive is true when the station has published something that means "I am
// here", whether or not a step is in flight.
//
// `starting` is published once, before any request, by a station proving its
// credential and its network reach. It is the difference between "nobody has
// ever started this" and "it is up and has been asked nothing", and those need
// different next moves: the first is chased with the operator, the second is
// answered by sending a step.
func (s Status) Alive() bool { return s.State == "running" || s.State == "starting" }
