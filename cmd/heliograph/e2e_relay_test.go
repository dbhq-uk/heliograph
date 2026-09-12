package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dbhq-uk/heliograph/internal/seal"
)

// The relay round trip: the real binary, a real station, and a real key
// exchange, over the relay's documented HTTP API.
//
// WHY A LOCAL RELAY RATHER THAN THE DEPLOYED ONE. The server lives in
// dbhq-uk/heliograph-relay, holds no keys, and has its own conformance suite
// asserted over HTTP against both of its implementations. What is unproven in
// THIS repository is different: that our two halves - the control CLI and the
// bash station - interoperate through that API at all. Neither half had ever
// spoken to the other over the relay, because until now no CLI command could
// select it.
//
// So the double below implements the three documented endpoints and nothing
// else. It is not a second relay and makes no claim to be one; it is the wire
// the two halves are measured across. Everything that matters happens in the
// clients: the sealing, the signing, the sequence numbers, the verification.
// The double could not read a message if it tried, which is the whole design.
//
// TestRelayReachesTheDeployedRelay below is the other half of the answer, and
// it runs against heliograph-relay.dbhq.uk when a token is present.

// relayDouble is the documented API: POST and GET on /v1/{estate}/{station}/{dir},
// plus /health. In-memory, first-in-first-out, and deliberately no long poll -
// the station passes wait=0 and the CLI does its own retrying.
type relayDouble struct {
	mu     sync.Mutex
	q      map[string][][]byte
	ctl    string
	stn    string
	estate string
}

func newRelayDouble(estate, ctl, stn string) *relayDouble {
	return &relayDouble{q: map[string][][]byte{}, ctl: ctl, stn: stn, estate: estate}
}

func (r *relayDouble) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/health" {
		_, _ = w.Write([]byte(`{"ok":true}`))
		return
	}
	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "v1" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	est, station, dir := parts[1], parts[2], parts[3]
	if est != r.estate || (dir != "c2s" && dir != "s2c") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// The token scopes are asymmetric ON PURPOSE, and the double honours that
	// because getting it backwards in the client is exactly the mistake worth
	// catching here: a station token may READ requests and WRITE status and
	// logs, and may not queue a request even for itself.
	// ONLY THE TWO METHODS THE API HAS. Treating "anything that is not POST" as
	// a collecting GET would let a client regression from GET to DELETE pass
	// here and fail against the documented relay.
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tok := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	writing := req.Method == http.MethodPost
	var ok bool
	switch {
	case dir == "c2s" && writing: // queueing a request: control only
		ok = tok == r.ctl
	case dir == "c2s" && !writing: // collecting a request: the station
		ok = tok == r.stn
	case dir == "s2c" && writing: // publishing status or a log: the station
		ok = tok == r.stn
	default: // collecting status or a log: control
		ok = tok == r.ctl
	}
	if !ok {
		http.Error(w, "unauthorised", http.StatusUnauthorized)
		return
	}

	key := est + "/" + station + "/" + dir
	r.mu.Lock()
	defer r.mu.Unlock()

	// The wire shape, and it is asymmetric: POST queues ONE message object, GET
	// returns an ARRAY of everything waiting. Both sides of heliograph already
	// expect exactly that - Relay.collect unmarshals []relayMsg and so does
	// `heliograph-seal open` - so getting it wrong here would be a double that
	// disagrees with the thing it stands in for, which is worse than no double.
	if writing {
		var raw json.RawMessage
		if err := json.NewDecoder(req.Body).Decode(&raw); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		r.q[key] = append(r.q[key], raw)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	// DELETED ON COLLECTION. That is the relay's documented behaviour and it is
	// the property that forces the control side to keep a spool: a log arrives
	// exactly once and is then gone.
	msgs := r.q[key]
	r.q[key] = nil
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("["))
	for i, m := range msgs {
		if i > 0 {
			_, _ = w.Write([]byte(","))
		}
		_, _ = w.Write(m)
	}
	_, _ = w.Write([]byte("]"))
}

func TestCLIDrivesARelayStation(t *testing.T) {
	station := stationDir(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}

	const ctlToken, stnToken, relayEstate = "control-token", "station-token", "e2e"
	srv := httptest.NewServer(newRelayDouble(relayEstate, ctlToken, stnToken))
	defer srv.Close()

	base := t.TempDir()
	work := filepath.Join(base, "work")
	cfg := filepath.Join(base, "config")

	sh(t, base, filepath.Join(station, "station", "bootstrap.sh"), work)
	step := "#!/usr/bin/env bash\n# heliograph-mode: read-only\necho THE-RELAY-CARRIED-IT\n"
	if err := os.WriteFile(filepath.Join(work, "steps", "probe.sh"), []byte(step), 0o755); err != nil {
		t.Fatal(err)
	}

	// heliograph-seal is what the station uses, and the relay is the one
	// transport that needs it. Built here rather than stubbed: the sealing is
	// the part of this round trip that can be subtly wrong.
	sealBin := filepath.Join(work, "heliograph-seal")
	sh(t, ".", "go", "build", "-o", sealBin, "github.com/dbhq-uk/heliograph/cmd/heliograph-seal")

	bin := filepath.Join(base, "heliograph")
	sh(t, ".", "go", "build", "-o", bin, "github.com/dbhq-uk/heliograph/cmd/heliograph")
	hg := func(env []string, args ...string) string {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = base
		cmd.Env = append(append(os.Environ(), "XDG_CONFIG_HOME="+cfg), env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("heliograph %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	withToken := []string{"HELIOGRAPH_RELAY_TOKEN=" + ctlToken}

	// --- the control side, and the key exchange it starts --------------------
	out := hg(nil, "init", "relayed", "--transport", "relay",
		"--dir", srv.URL, "--relay-estate", relayEstate, "--scope", "s1")
	if !strings.Contains(out, "fingerprint:") {
		t.Fatalf("init did not print a fingerprint to read back:\n%s", out)
	}

	// Sending before the exchange is finished must name the command that
	// finishes it. This is the failure people actually hit, and the one that
	// otherwise looks like a fault on the far side.
	{
		cmd := exec.Command(bin, "send", "steps/probe.sh")
		cmd.Dir = base
		cmd.Env = append(append(os.Environ(), "XDG_CONFIG_HOME="+cfg), withToken...)
		o, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("send succeeded with no station identity recorded:\n%s", o)
		}
		if !strings.Contains(string(o), "relay peer") {
			t.Errorf("the refusal does not name the command that fixes it:\n%s", o)
		}
	}

	// What the operator is handed. It has to carry the control's PUBLIC half
	// and never its secret.
	instructions := hg(nil, "plant", "-e", "relayed")
	for _, want := range []string{"RELAY_URL", "RELAY_PEER", "keygen", "fingerprint"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("the plant instructions never mention %q:\n%s", want, instructions)
		}
	}
	ctlIDPath := filepath.Join(cfg, "heliograph", "keys", "relayed.identity.json")
	ctlID, err := seal.LoadIdentityFile(ctlIDPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(instructions, ctlID.Encode()) {
		t.Fatal("the plant instructions carry the control's SECRET key")
	}
	if !strings.Contains(instructions, ctlID.Public().Encode()) {
		t.Error("the plant instructions do not carry the control's public identity, which is what RELAY_PEER is")
	}

	// --- the far side does what it was told ----------------------------------
	stationID := filepath.Join(work, "station-identity.json")
	stationPeer := filepath.Join(work, "station-peer")
	sh(t, work, sealBin, "keygen", "--out", stationID)
	pubOut, err := exec.Command(sealBin, "public", "--identity", stationID).Output()
	if err != nil {
		t.Fatal(err)
	}
	stationPublic := strings.TrimSpace(string(pubOut))
	if err := os.WriteFile(stationPeer, []byte(ctlID.Public().Encode()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// ...and sends its public line back, which is recorded here.
	out = hg(nil, "relay", "peer", "-e", "relayed", stationPublic)
	if !strings.Contains(out, "fingerprint:") {
		t.Errorf("relay peer did not print a fingerprint to compare:\n%s", out)
	}

	// THE STATION'S SECRET MUST NOT REACH THE CONTROL NODE, even when somebody
	// hands over the whole identity file rather than the one line. `relay peer`
	// decodes to a public identity and re-encodes it, so this is impossible
	// rather than unlikely.
	peerFile := filepath.Join(cfg, "heliograph", "keys", "relayed.peer")
	pb, err := os.ReadFile(peerFile)
	if err != nil {
		t.Fatal(err)
	}
	stationSecret, err := seal.LoadIdentityFile(stationID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(pb), stationSecret.Encode()) {
		t.Fatal("the recorded peer file holds the STATION'S SECRET KEY")
	}

	// THE ACCIDENT THIS ACTUALLY GUARDS AGAINST: somebody sends the whole
	// identity file rather than the one line. Passing the bare public string
	// above cannot catch a regression that copied its input verbatim, because
	// the bare string has no secret in it to copy.
	hg(nil, "relay", "peer", "-e", "relayed", stationID)
	pb2, err := os.ReadFile(peerFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(pb2), stationSecret.Encode()) {
		t.Fatal("handing over the whole identity FILE put the station's secret key on the control node")
	}
	if strings.TrimSpace(string(pb2)) != stationSecret.Public().Encode() {
		t.Errorf("the peer recorded from an identity file is not the public half of it:\n%s", pb2)
	}

	// --- the round trip ------------------------------------------------------
	if out := hg(withToken, "send", "steps/probe.sh"); !strings.Contains(out, "sent ") {
		t.Fatalf("send did not report an id:\n%s", out)
	}

	// A DEADLINE, and it is not belt and braces. `--once` waits for a request
	// and nothing times it out, so a station that cannot verify what the relay
	// hands it drops the message and polls for ever - which from here is
	// indistinguishable from a station doing its job. Without this the test
	// hangs instead of failing, and a test that hangs teaches nobody anything.
	// This is how the preflight eating the request was found.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "./start.sh", "--", "--once", "--interval", "1")
	// WaitDelay, or CombinedOutput blocks on pipes a killed station's children
	// still hold, and the deadline above buys nothing.
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = work
	cmd.Env = append(os.Environ(),
		"TRANSPORT=relay",
		"RELAY_URL="+srv.URL,
		"RELAY_ESTATE="+relayEstate,
		"RELAY_STATION=s1",
		"RELAY_TOKEN="+stnToken,
		"RELAY_IDENTITY="+stationID,
		"RELAY_PEER="+stationPeer,
		"RELAY_SEAL="+sealBin,
	)
	cmd.Env = append(cmd.Env, noBackgroundGit...)
	o, runErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("the station never completed a run within 90s. Its own output:\n%s", o)
	}
	if runErr != nil {
		t.Fatalf("the relay station did not run: %v\n%s", runErr, o)
	}

	// IDLE, NOT running. The station publishes the finished log and then the
	// idle status from a DIFFERENT PROCESS, and both take a sequence number.
	// They collided, the receiver dropped the second as a replay - correctly,
	// that is the replay defence - and the run reported `running` for ever with
	// its log sitting in the spool. Nothing errored anywhere.
	if s := hg(withToken, "status"); !strings.Contains(s, "idle") {
		t.Errorf("the run never reported idle, so the final status was lost:\n%s", s)
	}
	// Twice, because a relay DELETES ON COLLECTION and the CLI is one-shot.
	// Without a spool the second call reports a station that has gone away.
	if s := hg(withToken, "status"); !strings.Contains(s, "idle") {
		t.Errorf("status is not repeatable: a relay deletes on collection, so it has to be kept:\n%s", s)
	}
	logs := hg(withToken, "logs")
	if !strings.Contains(logs, "probe-") {
		t.Fatalf("no log came back over the relay:\n%s", logs)
	}
	// UNDER THE STATION'S OWN NAME. A relay carries no filename in the sealed
	// envelope, so the name comes from the status's `log:` field - and that
	// field said `<none>` for every step sent by PATH, because station.sh
	// globbed on the step as given rather than on the label run.sh uses.
	if strings.Contains(logs, "<none>") || strings.Contains(logs, "relay-0") {
		t.Errorf("the collected log did not get the station's own name for it:\n%s", logs)
	}
	body := hg(withToken, "logs", "--last")
	if !strings.Contains(body, "THE-RELAY-CARRIED-IT") {
		t.Errorf("the log does not contain the step's output:\n%s", body)
	}
	if !strings.Contains(body, " | THE-RELAY-CARRIED-IT") {
		t.Errorf("the captured line lost its timestamp column:\n%s", body)
	}
	if !strings.Contains(body, "RESULT       : OK") {
		t.Errorf("the log has no footer, so the far side cannot tell a finished run from a hung one:\n%s", body)
	}
}

// The DEPLOYED relay, when there is an estate on it to use.
//
// Everything above proves the two halves interoperate. This proves they reach
// the thing that is actually running at heliograph-relay.dbhq.uk - the TLS, the
// custom domain, the Durable Object routing, the token check as configured
// there - none of which a local double can say anything about.
//
// It SKIPS without credentials rather than failing, because they belong to
// whoever runs the relay and cannot live in a public repository. That is the
// one skip in this file, and it is the honest one: the alternative is a test
// that cannot run anywhere except on one person's machine.
//
//	HELIOGRAPH_RELAY_URL=https://heliograph-relay.dbhq.uk \
//	HELIOGRAPH_RELAY_ESTATE=<estate> \
//	HELIOGRAPH_RELAY_TOKEN=<its CONTROL token> \
//	go test ./cmd/heliograph -run TestRelayReachesTheDeployedRelay -v
func TestRelayReachesTheDeployedRelay(t *testing.T) {
	url := os.Getenv("HELIOGRAPH_RELAY_URL")
	est := os.Getenv("HELIOGRAPH_RELAY_ESTATE")
	tok := os.Getenv("HELIOGRAPH_RELAY_TOKEN")
	if url == "" || est == "" || tok == "" {
		t.Skip("set HELIOGRAPH_RELAY_URL, HELIOGRAPH_RELAY_ESTATE and HELIOGRAPH_RELAY_TOKEN to run this against a real relay")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Health first, so a relay that is simply down is not reported as a
	// credential problem - which is the single most misleading thing a relay
	// can say, because it sends the reader to the far side of the gap.
	resp, err := client.Get(strings.TrimRight(url, "/") + "/health")
	if err != nil {
		t.Fatalf("the relay at %s is not reachable: %v", url, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the relay's health endpoint answered %d", resp.StatusCode)
	}

	// Then the credential, on the route the CLI actually uses.
	// A ROUTING KEY NO STATION CAN HAVE. A GET on this API COLLECTS, and the
	// relay deletes on collection - so probing a plausible station name would
	// delete that station's statuses and logs. A station name is validated to
	// letters, digits, dot, hyphen and underscore, so a leading `~` cannot be
	// one, and `~` is unreserved in a URL path.
	poll := fmt.Sprintf("%s/v1/%s/%s/s2c?wait=0", strings.TrimRight(url, "/"), est, "~heliograph-preflight")
	req, err := http.NewRequest(http.MethodGet, poll, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("collecting from %s failed: %v", poll, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("the relay refused this token for estate %q. It must be the CONTROL token, not the station's", est)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the relay answered %d collecting from %s", resp.StatusCode, poll)
	}
}
