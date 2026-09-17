package main

// The trusted set over a real relay: two people with two keys, a real station,
// and a signed change that lands with nothing but the relay on the path.
//
// WHY THIS EXISTS WHEN tests/test-trusted-set.sh ALREADY PASSES. That one drives
// a git station over a bare repository on a filesystem, so the change and the
// request travel as files. This one drives the relay, where they travel sealed
// inside an envelope through a server neither side trusts, and where the
// station has to verify the request against a SET rather than against the one
// recorded peer it has always used. Those are different code paths on both
// sides of the gap, and the relay's is the one a hostile party can actually
// reach.
//
// It is also the only place the attribution claim is exercised end to end: the
// station has to establish WHICH member signed the request it acted on, and
// write that into the archived log.

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heliograph-io/heliograph/internal/seal"
)

func TestATrustedSetWorksOverTheRelay(t *testing.T) {
	station := stationDir(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}

	const ctlToken, stnToken, relayEstate = "control-token", "station-token", "trust-e2e"
	srv := httptest.NewServer(newRelayDouble(relayEstate, ctlToken, stnToken))
	defer srv.Close()

	base := t.TempDir()
	work := filepath.Join(base, "work")
	cfg := filepath.Join(base, "config")

	sh(t, base, filepath.Join(station, "station", "bootstrap.sh"), work)
	step := "#!/usr/bin/env bash\n# heliograph-mode: read-only\necho THE-SET-ALLOWED-IT\n"
	if err := os.WriteFile(filepath.Join(work, "steps", "probe.sh"), []byte(step), 0o755); err != nil {
		t.Fatal(err)
	}

	sealBin := filepath.Join(work, "heliograph-seal")
	sh(t, ".", "go", "build", "-o", sealBin, "github.com/heliograph-io/heliograph/cmd/heliograph-seal")
	bin := filepath.Join(base, "heliograph")
	sh(t, ".", "go", "build", "-o", bin, "github.com/heliograph-io/heliograph/cmd/heliograph")

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

	// --- the enrolment, unchanged --------------------------------------------
	hg(nil, "init", "trusted", "--transport", "relay",
		"--dir", srv.URL, "--relay-estate", relayEstate, "--scope", "s1")

	stationID := filepath.Join(work, "station-identity.json")
	stationPeer := filepath.Join(work, "station-peer")
	sh(t, work, sealBin, "keygen", "--out", stationID)
	pubOut, err := exec.Command(sealBin, "public", "--identity", stationID).Output()
	if err != nil {
		t.Fatal(err)
	}
	ctlID, err := seal.LoadIdentityFile(filepath.Join(cfg, "heliograph", "keys", "trusted.identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stationPeer, []byte(ctlID.Public().Encode()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hg(nil, "relay", "peer", "-e", "trusted", strings.TrimSpace(string(pubOut)))

	// --- the anchor, planted on the machine ----------------------------------
	//
	// The control side records its copy; the operator plants the same anchor
	// there. THE ANCHOR IS THE CONTROL NODE'S PUBLIC IDENTITY, which is exactly
	// what the operator already recorded as RELAY_PEER - so the chain's first
	// link is the one that was always there, and nothing new is asked of them.
	initOut := hg(nil, "trust", "init", "-e", "trusted")
	if !strings.Contains(initOut, "anchor:") {
		t.Fatalf("trust init printed no anchor to read back:\n%s", initOut)
	}
	if strings.Contains(initOut, ctlID.Encode()) {
		t.Fatal("the plant instructions carry the control's SECRET key")
	}
	setPath := filepath.Join(work, ".station-trusted-set")
	sh(t, work, sealBin, "trust", "init", "--set", setPath,
		"--estate", relayEstate, "--station", "s1",
		"--name", "anchor", "--anchor", ctlID.Public().Encode())

	// Both sides agree on the anchor before anything else happens. If they did
	// not, every change below would be refused for a reason that has nothing to
	// do with what is being tested.
	localSet, err := os.ReadFile(filepath.Join(cfg, "heliograph", "keys", "trusted.trusted-set"))
	if err != nil {
		t.Fatal(err)
	}
	stationDigest := strings.TrimSpace(runOut(t, work, sealBin, "trust", "digest", "--set", setPath))
	if !strings.Contains(string(localSet), stationDigest) {
		t.Fatalf("the control node and the station do not agree on the planted set:\nstation %s\nlocal:\n%s",
			stationDigest, localSet)
	}

	runStation := func(extraEnv ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", "./start.sh", "--", "--once", "--interval", "1")
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
			// THE TRUSTED SET, which is what this test is about. Reserved by the
			// TRUST_ prefix in the env guard, so a request cannot point the
			// station at a different one.
			"TRUST_SET="+setPath,
			"TRUST_SEAL="+sealBin,
		)
		cmd.Env = append(cmd.Env, extraEnv...)
		cmd.Env = append(cmd.Env, noBackgroundGit...)
		o, runErr := cmd.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("the station never completed a pass within 90s. Its own output:\n%s", o)
		}
		if runErr != nil {
			t.Fatalf("the station exited badly: %v\n%s", runErr, o)
		}
		return string(o)
	}

	// runStationBriefly starts the loop, lets it get through startup, and stops
	// it. For asserting what a station says and publishes BEFORE its first poll.
	runStationBriefly := func() string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", "./start.sh", "--", "--interval", "1")
		cmd.WaitDelay = 3 * time.Second
		cmd.Dir = work
		cmd.Env = append(os.Environ(),
			"TRANSPORT=relay", "RELAY_URL="+srv.URL, "RELAY_ESTATE="+relayEstate,
			"RELAY_STATION=s1", "RELAY_TOKEN="+stnToken,
			"RELAY_IDENTITY="+stationID, "RELAY_PEER="+stationPeer,
			"RELAY_SEAL="+sealBin, "TRUST_SET="+setPath, "TRUST_SEAL="+sealBin,
		)
		cmd.Env = append(cmd.Env, noBackgroundGit...)
		o, _ := cmd.CombinedOutput() // killed by the deadline; that is the design
		return string(o)
	}

	// --- alice is enrolled by a signed change, over the relay ----------------
	aliceID := filepath.Join(base, "alice.json")
	sh(t, base, sealBin, "keygen", "--out", aliceID)
	alicePub := strings.TrimSpace(runOut(t, base, sealBin, "public", "--identity", aliceID))

	addOut := hg(withToken, "trust", "add", "-e", "trusted", "alice", alicePub)
	if !strings.Contains(addOut, "signed by anchor") {
		t.Errorf("trust add did not report who signed it:\n%s", addOut)
	}
	// THE LATENCY IS SAID AT THE MOMENT SOMEBODY CHANGES ACCESS, not only in the
	// docs. "Revoked" read as "instant" is the assumption that gets discovered
	// during an incident.
	if !strings.Contains(addOut, "EVENTUAL") {
		t.Errorf("the CLI does not say revocation is eventual:\n%s", addOut)
	}

	out := runStation()
	if !strings.Contains(out, "TRUSTED SET") {
		t.Fatalf("the station did not apply a change that arrived over the relay:\n%s", out)
	}
	after := runOut(t, work, sealBin, "trust", "show", "--set", setPath)
	if !strings.Contains(after, "alice") {
		t.Fatalf("alice is not in the station's set:\n%s", after)
	}

	// The station published it, so the owner can audit without asking us.
	status := hg(withToken, "status")
	if !strings.Contains(status, "alice=") {
		t.Errorf("the published status does not carry the set's members:\n%s", status)
	}

	// --- and doctor agrees, because both sides applied the same change -------
	doc := hg(withToken, "doctor")
	if !strings.Contains(doc, "the trusted set matches") {
		t.Errorf("doctor does not report the sets as matching:\n%s", doc)
	}

	// --- A SET THAT DIFFERS IS REPORTED, which is the alarm ------------------
	//
	// Simulated by moving the STATION's set on, at the machine, without telling
	// the control node - which is what a key somebody added out of band looks
	// like from here.
	bobID := filepath.Join(base, "bob.json")
	sh(t, base, sealBin, "keygen", "--out", bobID)
	forged := filepath.Join(base, "forged-change")
	sh(t, ".", "go", "build", "-o", filepath.Join(base, "forge"), "github.com/heliograph-io/heliograph/tests/forge")
	prev := strings.TrimSpace(runOut(t, work, sealBin, "trust", "digest", "--set", setPath))
	bobPub := strings.TrimSpace(runOut(t, base, sealBin, "public", "--identity", bobID))
	body := runOut(t, base, filepath.Join(base, "forge"),
		"-identity", aliceID, "-author", "alice", "-op", "add", "-name", "bob",
		"-key", bobPub, "-estate", relayEstate, "-station", "s1",
		"-serial", "2", "-prev", prev)
	if err := os.WriteFile(forged, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	sh(t, work, sealBin, "trust", "apply", "--set", setPath, "--in", forged)

	// AND THE STATION IS RESTARTED, because that is what the recovery flow
	// looks like: somebody stands at the machine, changes the set, and starts
	// the loop again. A station whose set moved while it was down publishes it
	// at STARTUP rather than waiting for a request - otherwise `doctor` goes on
	// reporting a match against a digest from before the change, which is the
	// alarm silent in the direction that reassures.
	//
	// NOT runStation(). `--once` waits for a request and there is none coming,
	// so it would poll until the deadline. The publish being tested happens
	// before the first poll, so a few seconds and a signal is the whole of it -
	// and that is also exactly what the operator does when they restart a
	// station and walk away.
	out = runStationBriefly()
	if !strings.Contains(out, "trusted set changed while this station was down") {
		t.Errorf("the station did not notice its set had moved while it was down:\n%s", out)
	}

	doc2, docErr := runFailing(t, base, bin, cfg, withToken, "doctor")
	if docErr == nil {
		t.Error("doctor did not fail on a station whose trusted set differs from ours")
	}
	if !strings.Contains(doc2, "not the one this machine holds") {
		t.Errorf("doctor does not report the divergence:\n%s", doc2)
	}
	if !strings.Contains(doc2, "did not authorise it") {
		t.Errorf("doctor does not say what the divergence means:\n%s", doc2)
	}

	// --- the run itself: verified against the SET, and attributed ------------
	//
	// The request is signed by the anchor's key, which is a member of the set.
	// The station has to find it there rather than by comparing against the one
	// recorded peer, and then say WHO in the log it archives.
	hg(withToken, "send", "steps/probe.sh")
	out = runStation()
	if strings.Contains(out, "REFUSED") {
		t.Fatalf("a request from a set member was refused:\n%s", out)
	}
	logs := hg(withToken, "logs")
	if !strings.Contains(logs, "probe-") {
		t.Fatalf("no log came back over the relay:\n%s", logs)
	}
	logBody := hg(withToken, "logs", "--last")
	if !strings.Contains(logBody, "THE-SET-ALLOWED-IT") {
		t.Errorf("the log does not contain the step's output:\n%s", logBody)
	}
	// EVERY ARCHIVED RUN ATTRIBUTES TO A PERSON. For as long as there was one
	// identity per estate, a log three months old could say the estate asked and
	// never who, which is the first question anybody puts to it in an incident.
	if !strings.Contains(logBody, "requested by: anchor") {
		t.Errorf("the archived log does not say who asked for the run:\n%s", logBody)
	}
}

// runOut runs a command and returns its output, failing the test if it errors.
func runOut(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

// runFailing runs heliograph EXPECTING a non-zero exit, and returns both.
//
// doctor reports blocking problems by failing, so a helper that treats failure
// as fatal cannot be used to assert that it fails.
func runFailing(t *testing.T, dir, bin, cfg string, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "XDG_CONFIG_HOME="+cfg), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
