package cloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAServiceURLMustBeHTTPSOrLoopback(t *testing.T) {
	if _, err := NewService("http://archive.example.com"); err == nil {
		t.Error("a plain http service was accepted, and a credential travels in a header on every request")
	}
	if _, err := NewService("https://archive.example.com"); err != nil {
		t.Errorf("https was refused: %v", err)
	}
	// Loopback, and only loopback. A hostname somebody controls could resolve
	// to loopback today and elsewhere tomorrow, so only the literals pass.
	for _, u := range []string{"http://127.0.0.1:8787", "http://localhost:8787", "http://[::1]:8787"} {
		if _, err := NewService(u); err != nil {
			t.Errorf("%s was refused, so nothing can be driven against a local Worker: %v", u, err)
		}
	}
}

// THREE SCHEMES AND NO SHARED ONE. `Control` for the control plane, `Station`
// for a station's own endpoints, `Bearer` for the read-only API. Presenting one
// where another belongs is refused at the parser rather than by a lookup that
// misses, and that is what keeps two tokens with two scopes from flattening
// into one. A client that reused a variable would be the flattening.
func TestEachSchemeIsSentUnderItsOwnName(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	s, err := NewService(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		auth Auth
		want string
	}{
		{Control("ctl"), "Control ctl"},
		{Station("stn"), "Station stn"},
		{ReadOnly("ro"), "Bearer ro"},
		{NoAuth(), ""},
	} {
		got = ""
		var out map[string]any
		if err := s.getJSON("/anything", c.auth, &out); err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("Authorization %q, want %q", got, c.want)
		}
	}
}

// A refusal is an object with a stable machine token, not a status code.
// `doctor` branches on `cause` and prints `line`, so both have to survive the
// trip intact.
func TestARefusalIsReadAsARefusalAndNotAsAStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(402)
		_ = json.NewEncoder(w).Encode(map[string]any{"refusal": map[string]any{
			"cause": "allowance-exhausted", "dimension": "messages",
			"says": "the message allowance for this period is used up, so no new command was admitted",
			"next": "results already accepted still deliver; raise the allowance to admit more work",
			"line": "the message allowance for this period is used up, so no new command was admitted. " +
				"dimension: messages. results already accepted still deliver; raise the allowance to admit more work",
			"reference": "heliograph-cloud#72",
		}})
	}))
	defer srv.Close()
	s, err := NewService(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	err = s.getJSON("/anything", Control("t"), &out)
	if err == nil {
		t.Fatal("a refusal was read as success")
	}
	r, ok := AsRefusal(err)
	if !ok {
		t.Fatalf("a refusal object was flattened into a plain error: %v", err)
	}
	if r.Cause != "allowance-exhausted" || r.Dimension != "messages" {
		t.Errorf("cause %q dimension %q", r.Cause, r.Dimension)
	}
	// PRINTED VERBATIM. `allowance-exhausted` carries a dimension and the
	// sentences differ by it, so composing our own from the cause would say
	// "no new command was admitted" about a station limit, where it is false.
	if !strings.Contains(err.Error(), "raise the allowance to admit more work") {
		t.Errorf("the service's own sentence was not printed: %v", err)
	}
}

// A cause this build has never heard of is printed, not collapsed into
// "error". The reader can look up a word they have not seen; they cannot
// recover one we replaced. Same rule as an unrecognised run state.
func TestAnUnknownCauseIsPrintedRatherThanCollapsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(409)
		_ = json.NewEncoder(w).Encode(map[string]any{"refusal": map[string]any{
			"cause": "estate-suspended-pending-review",
			"line":  "this estate is suspended pending review. somebody will be in touch",
		}})
	}))
	defer srv.Close()
	s, _ := NewService(srv.URL)
	var out map[string]any
	err := s.getJSON("/anything", Control("t"), &out)
	r, ok := AsRefusal(err)
	if !ok {
		t.Fatalf("an unrecognised cause stopped being a refusal: %v", err)
	}
	if r.Known() {
		t.Error("an unrecognised cause reported itself as one of the known nine")
	}
	if !strings.Contains(err.Error(), "estate-suspended-pending-review") {
		t.Errorf("the cause was not printed: %v", err)
	}
	if !strings.Contains(err.Error(), "suspended pending review") {
		t.Errorf("the service's own sentence was not printed: %v", err)
	}
}

// `rate-limited` is not an allowance problem: the client has lost nothing and
// the answer is to wait. There is no Retry-After header yet, so the number is
// in the body.
func TestRateLimitedCarriesItsWaitRatherThanReadingAsExhaustion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
		_ = json.NewEncoder(w).Encode(map[string]any{"refusal": map[string]any{
			"cause": "rate-limited", "retryAfterSeconds": 30,
			"line": "too many requests just now. try again in 30 seconds",
		}})
	}))
	defer srv.Close()
	s, _ := NewService(srv.URL)
	var out map[string]any
	r, ok := AsRefusal(s.getJSON("/anything", Control("t"), &out))
	if !ok {
		t.Fatal("not read as a refusal")
	}
	if r.RetryAfterSeconds != 30 {
		t.Errorf("retryAfterSeconds %d, want 30", r.RetryAfterSeconds)
	}
	if r.Exhaustion() {
		t.Error("rate-limited was classified as an allowance problem, which would tell somebody their account is out of quota when it is not")
	}
}

// Our outage is not the operator's credential. Reading this as authentication
// sends somebody to rotate a token during our incident, which is the exact
// failure the taxonomy exists to prevent.
func TestAuthoriserUnavailableIsNotACredentialProblem(t *testing.T) {
	r := &Refusal{Cause: "authoriser-unavailable"}
	if r.Credential() {
		t.Error("authoriser-unavailable was classified as a credential problem")
	}
	if !(&Refusal{Cause: "credential-invalid"}).Credential() {
		t.Error("credential-invalid was not classified as a credential problem")
	}
	if !(&Refusal{Cause: "credential-unscoped"}).Credential() {
		t.Error("credential-unscoped was not classified as a credential problem")
	}
}

// The ingest surface answers `{"problem": "..."}` rather than a refusal object.
// Both have to reach the reader; neither may be swallowed into a status code.
func TestAProblemBodyIsPrintedWhenThereIsNoRefusalObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"problem":"no upload is open for run env-1"}`))
	}))
	defer srv.Close()
	s, _ := NewService(srv.URL)
	var out map[string]any
	err := s.getJSON("/anything", Control("t"), &out)
	if err == nil {
		t.Fatal("a 409 was read as success")
	}
	if !strings.Contains(err.Error(), "no upload is open for run env-1") {
		t.Errorf("the service's own sentence was not printed: %v", err)
	}
}
