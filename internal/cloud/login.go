package cloud

import (
	"fmt"
	"strings"
	"time"
)

// The device flow, and why there is nothing to paste.
//
// The thing being removed is a step, not a keystroke. A token pasted into a
// terminal is a token in a shell history, a scrollback buffer and whatever the
// terminal emulator keeps - and the reason it was pasted in the first place is
// that somebody had to be shown it, which means it was on a screen and in a
// clipboard too. The device flow inverts that: the CLI shows a short code that
// opens nothing, the browser carries the authentication, and the credential
// travels back over the wire the CLI already opened.
//
// The contract is `heliograph-io/heliograph-cloud#11`.

// DeviceCode is the service's answer to "somebody at this terminal wants in".
type DeviceCode struct {
	UserCode         string `json:"userCode"`
	DeviceCode       string `json:"deviceCode"`
	VerificationPath string `json:"verificationPath"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
	PollSeconds      int    `json:"pollSeconds"`
	Says             string `json:"says"`
}

// VerificationURL resolves the path against the host we actually asked.
//
// `verificationPath` IS RELATIVE ON PURPOSE: the service does not know what
// hostname it is reached on, and an absolute URL it guessed would send somebody
// to the wrong place. This side does know, because this side chose it.
func (d DeviceCode) VerificationURL(s *Service) string {
	if strings.HasPrefix(d.VerificationPath, "http://") || strings.HasPrefix(d.VerificationPath, "https://") {
		return d.VerificationPath
	}
	return s.Base + "/" + strings.TrimPrefix(d.VerificationPath, "/")
}

// DeviceToken is the answer to a poll: either "keep going" or the credential.
type DeviceToken struct {
	Outcome      string `json:"outcome"`
	ControlToken string `json:"controlToken"`
	AccountID    string `json:"accountId"`
	Label        string `json:"label"`
	PollSeconds  int    `json:"pollSeconds"`
}

// StartDeviceLogin asks for a code. It sends no credential, because the whole
// point is that there is not one yet.
func (s *Service) StartDeviceLogin(client string) (DeviceCode, error) {
	var out DeviceCode
	err := s.postJSON("/device/v1/code", NoAuth(), map[string]string{"client": client}, &out)
	if err != nil {
		return DeviceCode{}, err
	}
	if out.DeviceCode == "" || out.UserCode == "" {
		return DeviceCode{}, fmt.Errorf("%s issued no device code, so there is nothing to enter", s.Base)
	}
	return out, nil
}

// collectDeviceToken makes one poll.
//
// A terminal outcome arrives as a non-2xx, so it reaches here as an error from
// readJSON. Each of the four is re-read into its own sentence, because the next
// move differs: `409` is not `404` specifically because the reader is holding
// something that USED to work.
func (s *Service) collectDeviceToken(deviceCode string) (DeviceToken, error) {
	var out DeviceToken
	err := s.postJSON("/device/v1/token", NoAuth(), map[string]string{"deviceCode": deviceCode}, &out)
	if err == nil {
		return out, nil
	}
	// The outcome is in the body of the refusal too, and the body is what says
	// which of the four this is. readJSON has already turned it into an error,
	// so the outcome is recovered from the text it carries.
	msg := err.Error()
	for outcome, next := range map[string]string{
		"denied":            "somebody declined this sign-in. Ask for a new code if that was not you",
		"expired":           "the code ran out before it was entered. Run `heliograph login` again to ask for a new one",
		"already-collected": "this code has been used once already, so sign in again rather than check for a typo",
		"unknown-code":      "the service has never heard of this code: check what was sent, and that --service names the right host",
	} {
		if strings.Contains(msg, outcome) {
			return DeviceToken{Outcome: outcome},
				fmt.Errorf("sign-in %s: %s", outcome, next)
		}
	}
	return DeviceToken{}, err
}

// AwaitDeviceToken polls at the pace the service asks for, until it answers.
//
// THE SERVICE SETS THE PACE, through `pollSeconds` on every answer rather than
// only on the first. A client that polled faster than it was told is a client
// the service has to defend itself against, and the defence would be a rate
// limit somebody else pays for.
//
// `sleep` is a parameter so that a test can drive this without waiting.
func (s *Service) AwaitDeviceToken(code DeviceCode, sleep func(time.Duration)) (DeviceToken, error) {
	wait := code.PollSeconds
	if wait <= 0 {
		wait = 2
	}
	deadline := code.ExpiresInSeconds
	if deadline <= 0 {
		deadline = 600
	}
	waited := 0
	for {
		tok, err := s.collectDeviceToken(code.DeviceCode)
		if err != nil {
			return tok, err
		}
		if tok.Outcome == "collected" {
			if tok.ControlToken == "" {
				return tok, fmt.Errorf("the service reported the sign-in collected and sent no credential")
			}
			return tok, nil
		}
		if tok.Outcome != "waiting" {
			// An outcome this build has never heard of is reported rather than
			// treated as either answer. Guessing "keep polling" would spin
			// against a service that has finished with us; guessing "done"
			// would report a sign-in that did not happen.
			return tok, fmt.Errorf("the service answered with an outcome this build does not know: %q.\n"+
				"  Neither waiting nor collected, so nothing is assumed about it", tok.Outcome)
		}
		if tok.PollSeconds > 0 {
			wait = tok.PollSeconds
		}
		if waited >= deadline {
			return tok, fmt.Errorf("no answer after %d seconds, and the code expires at %d.\n"+
				"  The browser step may not have been completed. Run `heliograph login` again",
				waited, deadline)
		}
		sleep(time.Duration(wait) * time.Second)
		waited += wait
	}
}

// Login runs the whole flow and stores what it collects.
//
// THE CREDENTIAL REACHES DISK BEFORE THE SUCCESS LINE IS PRINTED, and that
// ordering is the point rather than tidiness: the token is returned exactly
// once and no endpoint will show it again, so a crash between the two would
// lose an account's only credential while telling somebody they had one.
//
// `print` takes a finished line rather than a format string so that a test can
// assert on exactly what a reader sees, including that the credential is not in
// any of it.
func Login(s *Service, client string, print func(string), sleep func(time.Duration)) (DeviceToken, error) {
	code, err := s.StartDeviceLogin(client)
	if err != nil {
		return DeviceToken{}, err
	}

	// The service's own sentence first, then the two things a person acts on.
	if code.Says != "" {
		print(code.Says)
	}
	print("")
	print("  open  " + code.VerificationURL(s))
	print("  enter " + code.UserCode)
	print("")
	print("waiting for that to be completed in a browser...")

	tok, err := s.AwaitDeviceToken(code, sleep)
	if err != nil {
		return tok, err
	}

	creds, err := LoadCredentials()
	if err != nil {
		return tok, err
	}
	creds.SetAccount(s.Base, AccountCredential{
		Token:     tok.ControlToken,
		AccountID: tok.AccountID,
		Label:     tok.Label,
		Obtained:  time.Now().UTC().Format(time.RFC3339),
	})
	if err := creds.Save(); err != nil {
		// Named rather than swallowed. The credential exists and this machine
		// could not keep it, which is a different problem from a failed
		// sign-in and has a different remedy.
		return tok, fmt.Errorf("signed in, and the credential could not be written: %w.\n"+
			"  It is issued once and cannot be read back, so sign in again once that is fixed", err)
	}

	who := tok.Label
	if who == "" {
		who = tok.AccountID
	}
	print("")
	print("signed in to " + s.Base + " as " + who)
	path, _ := CredentialsPath()
	print("  credential stored at " + path + ", mode 600, and never printed")
	return tok, nil
}
