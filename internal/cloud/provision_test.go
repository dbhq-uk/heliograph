package cloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type provisionStub struct {
	sawAuth     string
	sawBody     map[string]any
	timing      map[string]any
	rotateSays  string
	rotateOther string
}

func (p *provisionStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.sawAuth = r.Header.Get("Authorization")
	switch {
	case r.URL.Path == "/provision/v1/estate" && r.Method == http.MethodPost:
		_ = json.NewDecoder(r.Body).Decode(&p.sawBody)
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"estate": "grp_abc", "controlToken": "estate-ctl", "enrolmentKey": "enr-key",
			"enrolmentKeyExpiresAt": "2026-09-13T10:00:00Z",
			"plantingLine":          "heliograph plant --enrol enr-key",
			"says":                  "estate payments is ready", "next": "send the planting line to whoever can log in",
		})
	case strings.HasSuffix(r.URL.Path, "/rotate") && r.Method == http.MethodPost:
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"controlToken": "estate-ctl-2", "says": p.rotateSays, "stationTokens": p.rotateOther,
		})
	case strings.HasSuffix(r.URL.Path, "/timing"):
		_ = json.NewEncoder(w).Encode(p.timing)
	default:
		http.Error(w, `{"problem":"not found"}`, 404)
	}
}

func provisionService(t *testing.T, p *provisionStub) *Service {
	t.Helper()
	srv := httptest.NewServer(p)
	t.Cleanup(srv.Close)
	s, err := NewService(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Provisioning needs the ACCOUNT credential, under the `Control` scheme. The
// estate credential it hands back is scoped to that estate and cannot provision
// another, which is the asymmetry this whole flow exists to preserve.
func TestProvisioningUsesTheAccountCredentialUnderTheControlScheme(t *testing.T) {
	p := &provisionStub{}
	s := provisionService(t, p)
	got, err := s.ProvisionEstate(Control("account-token"), "payments")
	if err != nil {
		t.Fatal(err)
	}
	if p.sawAuth != "Control account-token" {
		t.Errorf("Authorization %q, want the account credential under Control", p.sawAuth)
	}
	if p.sawBody["name"] != "payments" {
		t.Errorf("body %+v, want the estate name", p.sawBody)
	}
	if got.Estate != "grp_abc" || got.ControlToken != "estate-ctl" || got.EnrolmentKey != "enr-key" {
		t.Errorf("provisioned %+v", got)
	}
	// THE PLANTING LINE IS BUILT BY THE SERVICE AND PRINTED VERBATIM, so that
	// one change to the plant interface does not need every client updated.
	if got.PlantingLine != "heliograph plant --enrol enr-key" {
		t.Errorf("planting line %q, want it verbatim", got.PlantingLine)
	}
}

// ROTATING A CONTROL CREDENTIAL ROTATES NO STATION CREDENTIAL, and the CLI has
// to say so BEFORE somebody acts rather than after. A station credential sits
// on a machine nobody can reach, so rotating one means re-enrolling the
// station: the trap `relay.md` records as "a hosted relay whose token nobody
// recorded can only be re-issued - which means re-enrolling every station on
// it".
func TestRotationPrintsWhatItDoesNotRotate(t *testing.T) {
	p := &provisionStub{
		rotateSays:  "the previous control credential stopped working when this one was issued",
		rotateOther: "this does not rotate any station credential. Rotating one of those means re-enrolling the station",
	}
	s := provisionService(t, p)
	got, err := s.RotateControlToken(Control("estate-ctl"), "grp_abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.ControlToken != "estate-ctl-2" {
		t.Errorf("rotated to %q", got.ControlToken)
	}
	if got.StationTokens == "" {
		t.Fatal("the service's sentence about station credentials was dropped, and it is the one that stops somebody stranding a station")
	}
	if !strings.Contains(got.StationTokens, "re-enrolling") {
		t.Errorf("the station-credential sentence does not mention re-enrolment: %q", got.StationTokens)
	}
}

// A warning that only appears after the fact is not a warning. `RotationWarning`
// is what the CLI prints BEFORE it asks, and it has to be honest without having
// called anything.
func TestTheRotationWarningIsHonestBeforeAnythingIsCalled(t *testing.T) {
	w := RotationWarning()
	for _, want := range []string{"station", "re-enrol", "cannot be reached"} {
		if !strings.Contains(strings.ToLower(w), strings.ToLower(want)) {
			t.Errorf("the warning printed before rotation does not mention %q:\n%s", want, w)
		}
	}
	if strings.Contains(strings.ToLower(w), "rotate the station token") {
		t.Error("the warning offers station-token rotation as an ordinary action, which it is not")
	}
}

// `secondsToFirstRun` is null until a run arrives, and null is "no run yet"
// rather than "no time taken". Rendering it as zero would report a first run
// that has not happened, which is the worst direction for this particular
// guess to go.
func TestNoRunYetIsNotZeroSeconds(t *testing.T) {
	p := &provisionStub{timing: map[string]any{
		"signedUpAt": "2026-09-13T09:00:00Z", "provisionedAt": "2026-09-13T09:00:02Z",
		"firstEnrolAt": nil, "firstRunAt": nil, "secondsToFirstRun": nil,
	}}
	s := provisionService(t, p)
	got, err := s.Timing(Control("estate-ctl"), "grp_abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.SecondsToFirstRun != nil {
		t.Fatalf("secondsToFirstRun is %d, want absent", *got.SecondsToFirstRun)
	}
	if !strings.Contains(got.Line(), "no run has arrived") {
		t.Errorf("the sentence for a null is %q, and it must not read as a measurement", got.Line())
	}

	p.timing["firstRunAt"] = "2026-09-13T09:00:08Z"
	p.timing["secondsToFirstRun"] = 8
	got, err = s.Timing(Control("estate-ctl"), "grp_abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.SecondsToFirstRun == nil || *got.SecondsToFirstRun != 8 {
		t.Fatalf("secondsToFirstRun %v, want 8", got.SecondsToFirstRun)
	}
	if !strings.Contains(got.Line(), "8 seconds") {
		t.Errorf("the measured sentence is %q", got.Line())
	}
}
