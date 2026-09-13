package cloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Refusal is the service saying no, with a machine token and a sentence.
//
// A REFUSAL IS AN OBJECT, NOT A STATUS CODE. Branching on the code alone
// collapses nine different situations into three numbers: 402 is both an
// exhausted allowance and a spend cap, and 503 is both a full queue and our own
// authoriser being down. Those need different sentences and different next
// moves, and one of them is not the operator's problem at all.
//
// The contract is `heliograph-io/heliograph-cloud#10`.
type Refusal struct {
	Cause             string `json:"cause"`
	Dimension         string `json:"dimension,omitempty"`
	Says              string `json:"says,omitempty"`
	Next              string `json:"next,omitempty"`
	Line              string `json:"line,omitempty"`
	Reference         string `json:"reference,omitempty"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`

	Status int `json:"-"` // the HTTP status it arrived with, for a reader who wants it
}

// knownCauses is the closed set as of heliograph-cloud#10.
//
// It exists so `doctor` can say "this build knows this word" and NOT so an
// unrecognised one can be discarded. See Known.
var knownCauses = map[string]bool{
	"credential-invalid":        true,
	"credential-unscoped":       true,
	"allowance-exhausted":       true,
	"spend-cap-reached":         true,
	"rate-limited":              true,
	"not-admitted":              true,
	"delivery-ceiling-exceeded": true,
	"queue-full":                true,
	"authoriser-unavailable":    true,
}

// Known says whether this build has heard of the cause.
//
// AN UNKNOWN CAUSE IS STILL PRINTED. The same rule as an unrecognised run
// state: a reader can look up a word they have not seen, and cannot recover one
// we replaced with "error". This is only ever used to decide whether to add a
// sentence of our own, never to decide whether to show the service's.
func (r *Refusal) Known() bool { return knownCauses[r.Cause] }

// Credential says whether the operator's credential is the thing to look at.
//
// `authoriser-unavailable` is deliberately NOT in here. It is our outage, and a
// doctor that reads it as an authentication problem sends somebody to rotate a
// token during our incident - which is the exact failure the whole taxonomy
// exists to prevent.
func (r *Refusal) Credential() bool {
	return r.Cause == "credential-invalid" || r.Cause == "credential-unscoped"
}

// Exhaustion says whether something ran out.
//
// `rate-limited` is NOT exhaustion. The client has lost nothing and the answer
// is to wait, not to tell somebody their account is out of quota.
func (r *Refusal) Exhaustion() bool {
	return r.Cause == "allowance-exhausted" || r.Cause == "spend-cap-reached"
}

// Ours says whether the thing that went wrong is on our side.
func (r *Refusal) Ours() bool {
	return r.Cause == "authoriser-unavailable" || r.Cause == "queue-full"
}

// Error prints the service's own sentence, and adds nothing to it.
//
// `line` is composed by the service precisely so every client says the same
// thing, and because the sentences differ by dimension: "no new command was
// admitted" is true of `messages` and false of `stations`. Composing one here
// from the cause would get that wrong in a way nobody would notice until a
// customer read it.
func (r *Refusal) Error() string {
	var b strings.Builder
	b.WriteString("refused (" + r.Cause + ")")
	switch {
	case r.Line != "":
		b.WriteString(": " + r.Line)
	case r.Says != "" && r.Next != "":
		b.WriteString(": " + r.Says + ". " + r.Next)
	case r.Says != "":
		b.WriteString(": " + r.Says)
	}
	if r.Reference != "" {
		b.WriteString(" [" + r.Reference + "]")
	}
	return b.String()
}

// AsRefusal recovers the refusal from an error, when there is one.
func AsRefusal(err error) (*Refusal, bool) {
	var r *Refusal
	if errors.As(err, &r) {
		return r, true
	}
	return nil, false
}

// refusalFrom reads whatever the service said about why it said no.
//
// Three shapes reach here and all three have to survive: a `refusal` object
// from the control plane, a `{"problem": "..."}` from the ingest surface, and
// an unparseable body from anything in front of either. The last one is the
// case that matters most, because a proxy returning HTML is exactly when a
// client that printed only a status code leaves somebody with nothing.
func refusalFrom(status int, raw []byte) error {
	var envelope struct {
		Refusal *Refusal `json:"refusal"`
		Problem string   `json:"problem"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		if envelope.Refusal != nil && envelope.Refusal.Cause != "" {
			envelope.Refusal.Status = status
			return envelope.Refusal
		}
		if envelope.Problem != "" {
			return fmt.Errorf("the service refused this (%d): %s", status, envelope.Problem)
		}
	}
	body := strings.TrimSpace(string(raw))
	if len(body) > 400 {
		body = body[:400] + "..."
	}
	if body == "" {
		return fmt.Errorf("the service answered %d and said nothing about why", status)
	}
	return fmt.Errorf("the service answered %d: %s", status, body)
}
