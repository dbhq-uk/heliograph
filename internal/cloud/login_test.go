package cloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// deviceStub is the device-code half of the control plane, as posted on
// heliograph-cloud#11.
type deviceStub struct {
	mu sync.Mutex

	waits     int    // how many `waiting` answers before the token is handed over
	polls     int    // how many times /device/v1/token was called
	terminal  string // when set, the terminal outcome to answer with instead
	sawAuth   string // the Authorization header on /device/v1/code
	sawClient string
}

func (d *deviceStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch r.URL.Path {
	case "/device/v1/code":
		d.sawAuth = r.Header.Get("Authorization")
		var body struct {
			Client string `json:"client"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		d.sawClient = body.Client
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"userCode": "Q7RQ-RPC1", "deviceCode": "DC-0000",
			"verificationPath": "/console/device",
			"expiresInSeconds": 600, "pollSeconds": 2,
			"says": "open /console/device and enter Q7RQ-RPC1",
		})
	case "/device/v1/token":
		d.polls++
		if d.terminal != "" {
			status := map[string]int{
				"denied": 403, "expired": 410, "already-collected": 409, "unknown-code": 404,
			}[d.terminal]
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"outcome": d.terminal})
			return
		}
		if d.polls <= d.waits {
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{"outcome": "waiting", "pollSeconds": 2})
			return
		}
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"outcome": "collected", "controlToken": "ctl-secret",
			"accountId": "acc_1", "label": "dan's laptop",
		})
	default:
		http.Error(w, `{"problem":"not found"}`, 404)
	}
}

func deviceService(t *testing.T, d *deviceStub) *Service {
	t.Helper()
	srv := httptest.NewServer(d)
	t.Cleanup(srv.Close)
	s, err := NewService(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// NO CREDENTIAL IS SENT TO ASK FOR ONE, by design. The whole point of the
// device flow is that there is nothing to paste, so there is nothing to hold
// before it runs.
func TestAskingForADeviceCodeSendsNoCredential(t *testing.T) {
	d := &deviceStub{}
	s := deviceService(t, d)
	got, err := s.StartDeviceLogin("heliograph 1.2 on a test")
	if err != nil {
		t.Fatal(err)
	}
	if d.sawAuth != "" {
		t.Errorf("a credential was sent to acquire one: %q", d.sawAuth)
	}
	if d.sawClient != "heliograph 1.2 on a test" {
		t.Errorf("client %q, want the label this build sends", d.sawClient)
	}
	if got.UserCode != "Q7RQ-RPC1" || got.DeviceCode != "DC-0000" {
		t.Errorf("device code %+v", got)
	}
	if got.Says == "" {
		t.Error("the service's own sentence was dropped, and it is what the reader is shown")
	}
}

// `verificationPath` is relative on purpose: the service does not know what
// hostname it is reached on, and an absolute URL it guessed would send somebody
// to the wrong place. Resolving it is this side's job, because this side is the
// one that knows which host it asked.
func TestTheVerificationPathIsResolvedAgainstTheHostWeAsked(t *testing.T) {
	d := &deviceStub{}
	s := deviceService(t, d)
	got, err := s.StartDeviceLogin("t")
	if err != nil {
		t.Fatal(err)
	}
	want := s.Base + "/console/device"
	if got.VerificationURL(s) != want {
		t.Errorf("verification URL %q, want %q", got.VerificationURL(s), want)
	}
}

func TestLoginPollsUntilTheTokenIsCollected(t *testing.T) {
	d := &deviceStub{waits: 3}
	s := deviceService(t, d)
	code, err := s.StartDeviceLogin("t")
	if err != nil {
		t.Fatal(err)
	}
	var slept []time.Duration
	tok, err := s.AwaitDeviceToken(code, func(d time.Duration) { slept = append(slept, d) })
	if err != nil {
		t.Fatal(err)
	}
	if tok.ControlToken != "ctl-secret" || tok.AccountID != "acc_1" {
		t.Errorf("token %+v", tok)
	}
	if len(slept) != 3 {
		t.Errorf("waited %d times for 3 `waiting` answers: %v", len(slept), slept)
	}
	// THE SERVICE SETS THE PACE. A client polling faster than it was told is a
	// client the service has to defend itself against.
	for _, d := range slept {
		if d != 2*time.Second {
			t.Errorf("polled after %v, want the pollSeconds the service asked for", d)
		}
	}
}

// Four terminal outcomes, four different next moves. `409` is not `404`
// specifically because the reader is holding something that used to work, and
// the next move is to sign in again rather than to check a typo.
func TestEachTerminalOutcomeSaysWhatToDoNext(t *testing.T) {
	for _, c := range []struct{ outcome, want string }{
		{"denied", "declined"},
		{"expired", "ask for a new one"},
		{"already-collected", "sign in again"},
		{"unknown-code", "check what was sent"},
	} {
		d := &deviceStub{terminal: c.outcome}
		s := deviceService(t, d)
		code, err := s.StartDeviceLogin("t")
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.AwaitDeviceToken(code, func(time.Duration) {})
		if err == nil {
			t.Fatalf("%s was read as success", c.outcome)
		}
		if !strings.Contains(err.Error(), c.outcome) {
			t.Errorf("%s: the outcome was not named: %v", c.outcome, err)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: the error does not say what to do next (%q): %v", c.outcome, c.want, err)
		}
		if d.polls != 1 {
			t.Errorf("%s: polled %d times, want one - a terminal answer is terminal", c.outcome, d.polls)
		}
	}
}

// The token is returned exactly once and no endpoint will show it again, so it
// is on disk before the line that says the sign-in worked. A crash between the
// two otherwise loses an account's only credential while telling somebody they
// have one.
func TestTheControlTokenIsOnDiskBeforeTheSuccessLineIsPrinted(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	d := &deviceStub{waits: 1}
	s := deviceService(t, d)

	var printed []string
	storedWhenAnnounced := false
	tok, err := Login(s, "t", func(line string) {
		printed = append(printed, line)
		if strings.Contains(line, "signed in") {
			if c, cerr := LoadCredentials(); cerr == nil {
				_, storedWhenAnnounced = c.Account(s.Base)
			}
		}
	}, func(time.Duration) {})
	if err != nil {
		t.Fatal(err)
	}
	if tok.ControlToken != "ctl-secret" {
		t.Fatalf("token %+v", tok)
	}
	if !storedWhenAnnounced {
		t.Error("the sign-in was announced before the token reached disk, and there is no second chance to collect it")
	}

	creds, err := LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	acct, ok := creds.Account(s.Base)
	if !ok {
		t.Fatal("the control token was not stored")
	}
	if acct.Token != "ctl-secret" || acct.AccountID != "acc_1" {
		t.Errorf("stored %+v", acct)
	}
	// AND IT IS NEVER PRINTED. It never touches the clipboard, a terminal
	// scrollback, or a CI log.
	for _, line := range printed {
		if strings.Contains(line, "ctl-secret") {
			t.Errorf("the control token was printed: %q", line)
		}
	}
	// The user code IS printed. It is what somebody types into a browser, and
	// it is not a credential: it opens nothing on its own.
	joined := strings.Join(printed, "\n")
	if !strings.Contains(joined, "Q7RQ-RPC1") {
		t.Errorf("the user code was never shown, so there is nothing to type: %q", joined)
	}
}
