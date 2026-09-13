package cloud

import (
	"fmt"
	"strings"
)

// Provisioning: an estate, a scoped credential pair, and the line to send the
// operator.
//
// The step being removed is the one `heliograph-io/heliograph-cloud#11` names:
// write a Docker command, generate two tokens by hand, keep them somewhere you
// can read back, put something in front that terminates TLS, and do all of it
// before you have seen the product work once. None of that is in this path. The
// customer terminates no TLS and runs no container, because both ends dial out
// over ordinary HTTPS to something somebody else operates.

// ProvisionedEstate is what the control plane hands back, once.
type ProvisionedEstate struct {
	Estate                string `json:"estate"`
	ControlToken          string `json:"controlToken"`
	EnrolmentKey          string `json:"enrolmentKey"`
	EnrolmentKeyExpiresAt string `json:"enrolmentKeyExpiresAt"`
	PlantingLine          string `json:"plantingLine"`
	Says                  string `json:"says"`
	Next                  string `json:"next"`

	// RelayURL is the base URL of the relay this estate is routed on.
	//
	// IT IS NOT IN THE CONTRACT AS POSTED, and this field is here so that it
	// can be without a client change. The relay and the broker are two products
	// on the split design, so they may not share a hostname - and the estate
	// file records a relay base URL, not a control-plane one. Until the service
	// sends it, RelayBase falls back to the service's own URL and the CLI says
	// which it used, rather than recording a guess silently.
	RelayURL string `json:"relayUrl,omitempty"`
}

// RelayBase is the URL to record for this estate's transport.
func (p ProvisionedEstate) RelayBase(s *Service) (base string, assumed bool) {
	if p.RelayURL != "" {
		return strings.TrimRight(p.RelayURL, "/"), false
	}
	return s.Base, true
}

// ProvisionEstate creates an estate under an ACCOUNT credential.
//
// The estate credential it returns is scoped to that estate and cannot
// provision another. That asymmetry is the whole reason there are two
// credentials, and it is preserved here by never storing the two in one place
// and never passing one where the other belongs.
func (s *Service) ProvisionEstate(account Auth, name string) (ProvisionedEstate, error) {
	var out ProvisionedEstate
	if err := s.postJSON("/provision/v1/estate", account, map[string]string{"name": name}, &out); err != nil {
		return ProvisionedEstate{}, err
	}
	if out.Estate == "" || out.ControlToken == "" {
		return ProvisionedEstate{}, fmt.Errorf("the service reported estate %q provisioned and returned no credential for it", name)
	}
	// THE TWO MAY NOT BE EQUAL. Equal tokens collapse the only scope separation
	// there is - a station credential sits on a machine nobody can reach, and
	// if it were also the control credential it could queue a request. The
	// broker refuses to issue such a pair; this refuses to use one, because a
	// guard on one side of a wire is a guard that stops existing the day
	// somebody points the CLI at a different implementation.
	if out.EnrolmentKey != "" && out.EnrolmentKey == out.ControlToken {
		return ProvisionedEstate{}, fmt.Errorf(
			"the service issued the same value as both the control credential and the enrolment key for estate %q.\n"+
				"  Those have different scopes on purpose and one of them lives on a machine nobody can reach.\n"+
				"  Nothing has been recorded here", name)
	}
	return out, nil
}

// Rotation is the answer to rotating an estate's control credential.
type Rotation struct {
	ControlToken  string `json:"controlToken"`
	Says          string `json:"says"`
	StationTokens string `json:"stationTokens"`
}

// RotateControlToken replaces an estate's control credential.
//
// The old one stops working the moment the new one is issued, and then answers
// `revoked` rather than `unknown` - so somebody who rotated and lost the new
// value gets told what happened rather than told their estate does not exist.
func (s *Service) RotateControlToken(estateAuth Auth, estateID string) (Rotation, error) {
	var out Rotation
	path := "/provision/v1/estate/" + estateID + "/rotate"
	if err := s.postJSON(path, estateAuth, nil, &out); err != nil {
		return Rotation{}, err
	}
	if out.ControlToken == "" {
		return Rotation{}, fmt.Errorf("the service reported estate %s rotated and returned no new credential", estateID)
	}
	return out, nil
}

// RotationWarning is what the CLI prints BEFORE it rotates anything.
//
// IT IS HONEST ABOUT RE-ENROLMENT, and it is printed first rather than
// afterwards. `relay.md` carries the scar: "Keep the estates value somewhere
// you can read it back. Cloudflare secrets are write-only, so a hosted relay
// whose token nobody recorded can only be re-issued - which means re-enrolling
// every station on it."
//
// The asymmetry is the reason. A control credential lives on the machine of the
// person doing the rotating, so replacing it costs them a command. A station
// credential lives on a machine nobody here can reach, in front of somebody who
// may not be at work today, so replacing one is not a rotation at all: it is a
// re-enrolment, scheduled with another person. Offering the two side by side as
// if they were the same action is how a station gets stranded.
func RotationWarning() string {
	return strings.Join([]string{
		"This rotates the CONTROL credential for this estate, and nothing else.",
		"",
		"  The old control credential stops working the moment the new one is issued.",
		"  It is held here, so replacing it costs this machine one command.",
		"",
		"  It does NOT rotate any station credential. A station credential sits on a",
		"  machine that cannot be reached from here, in front of an operator who has",
		"  their own schedule, so replacing one is a re-enrolment rather than a",
		"  rotation: somebody has to run the planting line there again.",
	}, "\n")
}

// Timing is the signup-to-first-run measurement, read back from the service's
// own record.
type Timing struct {
	SignedUpAt    string `json:"signedUpAt"`
	ProvisionedAt string `json:"provisionedAt"`
	FirstEnrolAt  string `json:"firstEnrolAt"`
	FirstRunAt    string `json:"firstRunAt"`

	// A POINTER, because null means "no run has arrived" and 0 would mean "it
	// took no time". Rendering the first as the second reports a first run that
	// has not happened, and this is a product claim rather than a dashboard
	// number.
	SecondsToFirstRun *int `json:"secondsToFirstRun"`
}

// Line says what the measurement is, or says plainly that there is not one yet.
func (t Timing) Line() string {
	if t.SecondsToFirstRun == nil {
		return "no run has arrived yet, so there is no time to first run. " +
			"That is an absence of a measurement, not a measurement of zero"
	}
	return fmt.Sprintf("%d seconds from signup to the first run reaching the service", *t.SecondsToFirstRun)
}

// Timing reads the estate's own record of how long this took.
func (s *Service) Timing(estateAuth Auth, estateID string) (Timing, error) {
	var out Timing
	path := "/provision/v1/estate/" + estateID + "/timing"
	if err := s.getJSON(path, estateAuth, &out); err != nil {
		return Timing{}, err
	}
	return out, nil
}
