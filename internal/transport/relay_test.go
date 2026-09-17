package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/heliograph-io/heliograph/internal/seal"
	"github.com/heliograph-io/heliograph/internal/wire"
)

// fakeRelay is a queue that does what the real one does, and can also be told
// to misbehave. The point of these tests is not that a good relay works: it is
// that a bad one cannot get anywhere.
type fakeRelay struct {
	mu sync.Mutex
	q  map[string][]relayMsg

	replayEverything bool // never delete, so every poll re-serves old messages
	dropEverything   bool
	tamper           bool // flip a bit in every body
}

func newFakeRelay(t *testing.T) (*fakeRelay, *httptest.Server) {
	t.Helper()
	f := &fakeRelay{q: map[string][]relayMsg{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("POST /v1/{estate}/{station}/{dir}", func(w http.ResponseWriter, r *http.Request) {
		var m relayMsg
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(400)
			return
		}
		k := r.PathValue("estate") + "/" + r.PathValue("station") + "/" + r.PathValue("dir")
		f.mu.Lock()
		f.q[k] = append(f.q[k], m)
		f.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /v1/{estate}/{station}/{dir}", func(w http.ResponseWriter, r *http.Request) {
		k := r.PathValue("estate") + "/" + r.PathValue("station") + "/" + r.PathValue("dir")
		f.mu.Lock()
		out := append([]relayMsg{}, f.q[k]...)
		if !f.replayEverything {
			delete(f.q, k)
		}
		f.mu.Unlock()
		if f.dropEverything {
			out = nil
		}
		if f.tamper {
			for i := range out {
				b := append([]byte{}, out[i].Body...)
				if len(b) > 0 {
					b[len(b)/2] ^= 0x40
				}
				out[i].Body = b
			}
		}
		if out == nil {
			out = []relayMsg{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, srv
}

// station is the other side, doing by hand what station.sh will do.
type station struct {
	id      *seal.Identity
	peer    seal.PublicIdentity
	estate  string
	name    string
	seenSeq uint64
	outSeq  uint64
}

func (s *station) collect(t *testing.T, f *fakeRelay) []wire.Request {
	t.Helper()
	f.mu.Lock()
	msgs := append([]relayMsg{}, f.q[s.estate+"/"+s.name+"/c2s"]...)
	if !f.replayEverything {
		delete(f.q, s.estate+"/"+s.name+"/c2s")
	}
	tamper, drop := f.tamper, f.dropEverything
	f.mu.Unlock()

	// The station reads the queue directly here rather than over HTTP, so the
	// relay's misbehaviour has to be applied on this path too. The first
	// version applied it only in the HTTP handler, and the tamper test passed
	// because nothing had been tampered with - a test that asserted a
	// protection while arranging for it never to be exercised.
	if drop {
		msgs = nil
	}
	if tamper {
		for i := range msgs {
			b := append([]byte{}, msgs[i].Body...)
			if len(b) > 0 {
				b[len(b)/2] ^= 0x40
			}
			msgs[i].Body = b
		}
	}
	sort.Slice(msgs, func(a, b int) bool { return msgs[a].Seq < msgs[b].Seq })

	var got []wire.Request
	for _, m := range msgs {
		if m.Seq <= s.seenSeq {
			continue // the replay check, on the station's side
		}
		meta := seal.Meta{
			Estate: s.estate, Station: s.name, Dir: "c2s", Seq: m.Seq,
			Kind: "request", Recipient: s.id.Public().Fingerprint(),
		}
		plain, err := seal.Open(s.id, s.peer, meta, m.Body)
		if err != nil {
			continue
		}
		s.seenSeq = m.Seq
		r, err := wire.ParseRequest(plain)
		if err != nil {
			continue
		}
		got = append(got, r)
	}
	return got
}

func (s *station) publish(t *testing.T, f *fakeRelay, body []byte) {
	t.Helper()
	s.publishKind(t, f, "status", body)
}

// The same, for the kinds a station sends that are not a status document. The
// kind is a SIGNED field, so a test that wants to exercise the control side's
// handling of a progress snapshot has to seal one as `progress` - relabelling
// afterwards is exactly what the signature exists to prevent.
func (s *station) publishKind(t *testing.T, f *fakeRelay, kind string, body []byte) {
	t.Helper()
	s.outSeq++
	meta := seal.Meta{
		Estate: s.estate, Station: s.name, Dir: "s2c", Seq: s.outSeq,
		Kind: kind, Recipient: s.peer.Fingerprint(),
	}
	sealed, err := seal.Seal(s.id, s.peer, meta, body)
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	k := s.estate + "/" + s.name + "/s2c"
	f.q[k] = append(f.q[k], relayMsg{Seq: meta.Seq, Body: sealed})
	f.mu.Unlock()
}

func setup(t *testing.T) (*Relay, *station, *fakeRelay) {
	t.Helper()
	f, srv := newFakeRelay(t)
	control, err := seal.Generate()
	if err != nil {
		t.Fatal(err)
	}
	st, err := seal.Generate()
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRelay(srv.URL, "e1", "st1", "tok", control, st.Public(),
		filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return r, &station{id: st, peer: control.Public(), estate: "e1", name: "st1"}, f
}

// setup, plus the directory collected logs are kept in. A relay built without
// a spool discards every log it collects - correct for a caller that only wants
// the status, and it makes an assertion about log NAMES pass vacuously against
// broken code, because there are no names. Found by watching both checks below
// report an empty list against the fix that makes them pass.
func setupSpooled(t *testing.T) (*Relay, *station, *fakeRelay) {
	t.Helper()
	f, srv := newFakeRelay(t)
	control, err := seal.Generate()
	if err != nil {
		t.Fatal(err)
	}
	st, err := seal.Generate()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r, err := NewRelayWithSpool(srv.URL, "e1", "st1", "tok", control, st.Public(),
		filepath.Join(dir, "state.json"), filepath.Join(dir, "spool"))
	if err != nil {
		t.Fatal(err)
	}
	return r, &station{id: st, peer: control.Public(), estate: "e1", name: "st1"}, f
}

func TestARequestReachesTheStationAndAStatusComesBack(t *testing.T) {
	r, st, f := setup(t)

	if err := r.PutRequest(wire.Request{Version: wire.Version, ID: "run-1", Step: "net-probe"}); err != nil {
		t.Fatal(err)
	}
	got := st.collect(t, f)
	if len(got) != 1 || got[0].ID != "run-1" || got[0].Step != "net-probe" {
		t.Fatalf("the station got %+v", got)
	}

	st.publish(t, f, []byte("state:    idle\nid:       run-1\nexit:     0\n"))
	s, err := r.FetchStatus()
	if err != nil {
		t.Fatal(err)
	}
	if s.State != "idle" || s.ID != "run-1" {
		t.Errorf("got %+v", s)
	}
}

// THE ATTACK THAT MATTERS. A relay replaying an old request is a destructive
// step re-running with all its gates already satisfied, because --allow-actions
// and CONFIRM=yes were decided days earlier.
func TestARelayReplayingRequestsGetsNowhere(t *testing.T) {
	r, st, f := setup(t)
	f.replayEverything = true // never deletes: every poll re-serves everything

	if err := r.PutRequest(wire.Request{Version: 1, ID: "run-1", Step: "apply"}); err != nil {
		t.Fatal(err)
	}
	if n := len(st.collect(t, f)); n != 1 {
		t.Fatalf("first collect got %d", n)
	}
	// The same messages, served again and again.
	for i := 0; i < 5; i++ {
		if got := st.collect(t, f); len(got) != 0 {
			t.Fatalf("poll %d re-ran a request: %+v", i, got)
		}
	}
	// And a genuinely new one still gets through, so the check is not simply
	// refusing everything.
	if err := r.PutRequest(wire.Request{Version: 1, ID: "run-2", Step: "env"}); err != nil {
		t.Fatal(err)
	}
	got := st.collect(t, f)
	if len(got) != 1 || got[0].ID != "run-2" {
		t.Fatalf("a new request was refused: %+v", got)
	}
}

func TestARelayTamperingIsDetected(t *testing.T) {
	r, st, f := setup(t)
	if err := r.PutRequest(wire.Request{Version: 1, ID: "run-1", Step: "env"}); err != nil {
		t.Fatal(err)
	}
	f.tamper = true
	if got := st.collect(t, f); len(got) != 0 {
		t.Fatalf("the station acted on a tampered request: %+v", got)
	}
}

// A relay that simply refuses to deliver is a denial of service, which it can
// always do. What it must not do is make that look like something else.
func TestARelayDroppingEverythingIsNotMistakenForAnIdleStation(t *testing.T) {
	_, st, f := setup(t)
	st.publish(t, f, []byte("state: idle\n"))
	f.dropEverything = true

	r2, _, _ := setup(t)
	s, err := r2.FetchStatus()
	if err != nil {
		t.Fatal(err)
	}
	// Empty state, which the CLI reports as "has published no status" rather
	// than inventing one.
	if s.State != "" {
		t.Errorf("got %+v", s)
	}
}

// The relay knows the station's public key: it routes to it. That is not enough
// to write a request the station will act on.
func TestARelayCannotWriteARequestItself(t *testing.T) {
	_, st, f := setup(t)
	evil, err := seal.Generate()
	if err != nil {
		t.Fatal(err)
	}
	meta := seal.Meta{
		Estate: "e1", Station: "st1", Dir: "c2s", Seq: 1,
		Kind: "request", Recipient: st.id.Public().Fingerprint(),
	}
	sealed, err := seal.Seal(evil, st.id.Public(), meta, []byte("id: evil\nstep: rm-rf\n"))
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.q["e1/st1/c2s"] = append(f.q["e1/st1/c2s"], relayMsg{Seq: 1, Body: sealed})
	f.mu.Unlock()

	if got := st.collect(t, f); len(got) != 0 {
		t.Fatalf("the station ran a request the relay wrote: %+v", got)
	}
}

// Sequence state has to survive a restart. If it reset, the relay could replay
// everything it had ever seen the moment the CLI was restarted - which is
// ordinary, not exotic.
func TestSequenceStateSurvivesARestart(t *testing.T) {
	f, srv := newFakeRelay(t)
	control, _ := seal.Generate()
	st, _ := seal.Generate()
	statePath := filepath.Join(t.TempDir(), "state.json")

	r1, err := NewRelay(srv.URL, "e1", "st1", "tok", control, st.Public(), statePath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := r1.PutRequest(wire.Request{Version: 1, ID: "run", Step: "env"}); err != nil {
			t.Fatal(err)
		}
	}

	// A fresh client, same state file, as after a restart.
	r2, err := NewRelay(srv.URL, "e1", "st1", "tok", control, st.Public(), statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := r2.PutRequest(wire.Request{Version: 1, ID: "run-4", Step: "env"}); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	msgs := f.q["e1/st1/c2s"]
	f.mu.Unlock()
	if len(msgs) != 4 {
		t.Fatalf("got %d messages", len(msgs))
	}
	if msgs[3].Seq != 4 {
		t.Errorf("the sequence restarted after a reload: %d", msgs[3].Seq)
	}
}

func TestCheckReportsAWrongToken(t *testing.T) {
	control, _ := seal.Generate()
	st, _ := seal.Generate()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("GET /v1/{estate}/{station}/{dir}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	r, err := NewRelay(srv.URL, "e1", "st1", "wrong", control, st.Public(),
		filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	err = r.Check()
	if err == nil {
		t.Fatal("Check passed with a token the relay refuses")
	}
	// Health passes and the credential does not, so the message must say which.
	// "cannot reach the relay" would send the reader to the network.
	if !contains(err.Error(), "refused this token") {
		t.Errorf("the error does not distinguish reachability from authorisation: %v", err)
	}
}

func TestARelayNeedsAToken(t *testing.T) {
	control, _ := seal.Generate()
	st, _ := seal.Generate()
	if _, err := NewRelay("https://r", "e", "s", "", control, st.Public(), ""); err == nil {
		t.Error("built a relay with no token")
	}
}

// A RUN THAT PUBLISHED PROGRESS STILL GETS ITS LOG BACK UNDER ITS OWN NAME.
//
// The station publishes a progress snapshot on the FIRST in-run poll of every
// run, whatever PROGRESS_EVERY is set to, so this is the ordinary shape of a
// relay run and not an edge case. The snapshot arrives as a second log body in
// the same drain, and the rule for "may this log take the name the status gives
// it" counted every body rather than every FINISHED body. So the finished log
// was denied its own name and `heliograph logs` answered with two sequence
// names, one of them a truncated file.
//
// Live before this was fixed: station/powershell/transports/relay.psm1 publishes
// progress the same way, and station.ps1 fires the first snapshot on the first
// in-run poll. A PowerShell relay station hit this on every run.
func TestAProgressSnapshotDoesNotCostTheFinishedLogItsName(t *testing.T) {
	r, st, f := setupSpooled(t)

	const logName = "probe-20260913T051500Z.txt"
	running := "state:    running\nid:       r1\nlog:      ops-logs/" + logName + "\nprogress: 3 lines\n"
	idle := "state:    idle\nid:       r1\nexit:     0\nlog:      ops-logs/" + logName + "\n"

	// The order a real run produces: the progress document and its partial log,
	// then the finished log, then the idle status naming it.
	st.publish(t, f, []byte(running))
	st.publishKind(t, f, "progress", []byte("a line\nand another\nhalf a th"))
	st.publishKind(t, f, "log", []byte("a line\nand another\nhalf a third\nRESULT       : OK\n"))
	st.publish(t, f, []byte(idle))

	names, err := r.ListLogs()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != logName {
		t.Fatalf("the finished log did not come back under the station's own name for it: %v", names)
	}

	// AND IT IS THE FINISHED ONE. Getting the name right and the body wrong
	// would be worse than getting both wrong, because nothing would look amiss.
	body, err := r.ReadLog(logName)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(body), "RESULT       : OK") {
		t.Errorf("the log under the finished name is not the finished log:\n%s", body)
	}

	// THE SNAPSHOT IS KEPT AND HIDDEN, not discarded. It is genuinely useful -
	// it is the only thing that exists while a long step is still running - and
	// it is not a captured log: listing it beside finished ones invites reading
	// a truncated file as the whole answer.
	partial := filepath.Join(r.logsDir(), "probe-20260913T051500Z.partial.txt")
	if _, err := os.Stat(partial); err != nil {
		t.Errorf("the partial snapshot was not spooled at all: %v", err)
	}
	for _, n := range names {
		if contains(n, ".partial.") {
			t.Errorf("a partial snapshot is being listed as a captured log: %v", names)
		}
	}
}

// Two FINISHED logs in one drain is still ambiguous, and the guard that was
// there for it must survive the change above. Neither can be proved to be the
// one the status names, so both keep their signed sequence name rather than one
// of them being misattributed.
func TestTwoFinishedLogsInOneDrainBothKeepTheirSequenceName(t *testing.T) {
	r, st, f := setupSpooled(t)

	st.publishKind(t, f, "log", []byte("the first run\n"))
	st.publishKind(t, f, "log", []byte("the second run\n"))
	st.publish(t, f, []byte("state:    idle\nid:       r2\nexit:     0\nlog:      ops-logs/probe-20260913T052000Z.txt\n"))

	names, err := r.ListLogs()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("expected both logs to be kept, got %v", names)
	}
	for _, n := range names {
		if !contains(n, "relay-") {
			t.Errorf("a log was named from an ambiguous status: %v", names)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// --- the spool keeps every status, not only the newest ------------------------

func setupWithSpool(t *testing.T, spool string) (*Relay, *station, *fakeRelay) {
	t.Helper()
	f, srv := newFakeRelay(t)
	control, err := seal.Generate()
	if err != nil {
		t.Fatal(err)
	}
	st, err := seal.Generate()
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRelayWithSpool(srv.URL, "e1", "st1", "tok", control, st.Public(),
		filepath.Join(t.TempDir(), "state.json"), spool)
	if err != nil {
		t.Fatal(err)
	}
	return r, &station{id: st, peer: control.Public(), estate: "e1", name: "st1"}, f
}

// EVERY STATUS IS KEPT, and that is what makes the spool a store rather than a
// snapshot of the last thing that happened.
//
// `status` held exactly one document, overwritten on every drain, so the spool
// could say what the station is doing now and nothing at all about the eleven
// runs before it. That was enough while the only reader was `heliograph
// status`; it is not enough for anything that has to describe a RUN, because a
// run is keyed by the `id:` inside its own status document and no other copy
// of that id exists anywhere on this side of the gap.
//
// Two of them arriving in one drain is the case that matters. The station
// publishes on every transition, so `running` and `idle` for one run routinely
// travel together, and keeping only the last of a batch loses the first.
func TestTheSpoolKeepsEveryStatusDocumentAndNotOnlyTheNewest(t *testing.T) {
	spool := t.TempDir()
	r, st, f := setupWithSpool(t, spool)

	st.publish(t, f, []byte("state: running\nid: run-1\nstep: env\nutc: 20260913T090000Z\n"))
	st.publish(t, f, []byte("state: idle\nid: run-1\nstep: env\nexit: 0\nutc: 20260913T090012Z\n"))
	if _, err := r.FetchStatus(); err != nil {
		t.Fatal(err)
	}

	st.publish(t, f, []byte("state: running\nid: run-2\nstep: net\nutc: 20260913T091000Z\n"))
	if _, err := r.FetchStatus(); err != nil {
		t.Fatal(err)
	}

	kept, err := os.ReadDir(filepath.Join(spool, "statuses"))
	if err != nil {
		t.Fatalf("the spool kept no statuses directory: %v", err)
	}
	if len(kept) != 3 {
		var names []string
		for _, e := range kept {
			names = append(names, e.Name())
		}
		t.Fatalf("the spool kept %d status document(s), want 3: %v", len(kept), names)
	}

	// Every one of them is readable and carries the run it describes. A file
	// that exists and cannot be parsed is worse than an absent one, because it
	// reads as coverage.
	ids := map[string]int{}
	for _, e := range kept {
		b, rerr := os.ReadFile(filepath.Join(spool, "statuses", e.Name()))
		if rerr != nil {
			t.Fatal(rerr)
		}
		s, perr := wire.ParseStatus(b)
		if perr != nil {
			t.Fatalf("%s does not parse as a status: %v", e.Name(), perr)
		}
		ids[s.ID]++
	}
	if ids["run-1"] != 2 || ids["run-2"] != 1 {
		t.Errorf("the kept statuses describe %v, want two for run-1 and one for run-2", ids)
	}
}

// The SEQUENCE is in the name, because it is the one thing about a collected
// message that the sender signed. A spool that named statuses by arrival order
// would be naming them by something the relay controls.
func TestAKeptStatusIsNamedBySignedSequence(t *testing.T) {
	spool := t.TempDir()
	r, st, f := setupWithSpool(t, spool)

	st.publish(t, f, []byte("state: idle\nid: run-1\n"))
	if _, err := r.FetchStatus(); err != nil {
		t.Fatal(err)
	}
	kept, err := os.ReadDir(filepath.Join(spool, "statuses"))
	if err != nil || len(kept) != 1 {
		t.Fatalf("want one kept status, got %v (%v)", kept, err)
	}
	if kept[0].Name() != "status-000001.txt" {
		t.Errorf("the kept status is named %q, want it named by the sequence the sender signed", kept[0].Name())
	}
}

// AND IT NEVER OVERWRITES, for the reason spoolLog never does: a collected
// document is the only copy there will ever be, because the relay deleted it on
// collection. A second write under a name already taken would replace evidence.
func TestAKeptStatusIsNeverOverwritten(t *testing.T) {
	spool := t.TempDir()
	r, st, f := setupWithSpool(t, spool)

	if err := os.MkdirAll(filepath.Join(spool, "statuses"), 0o700); err != nil {
		t.Fatal(err)
	}
	taken := filepath.Join(spool, "statuses", "status-000001.txt")
	if err := os.WriteFile(taken, []byte("state: idle\nid: earlier\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	st.publish(t, f, []byte("state: idle\nid: run-1\n"))
	if _, err := r.FetchStatus(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(taken)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "earlier") {
		t.Errorf("a kept status was overwritten: %q", b)
	}
}

// The single `status` file is what `heliograph status` reads, and it still has
// to hold the newest document. Keeping a history is an addition, not a move.
func TestTheNewestStatusIsStillAtTheOldPath(t *testing.T) {
	spool := t.TempDir()
	r, st, f := setupWithSpool(t, spool)

	st.publish(t, f, []byte("state: running\nid: run-1\n"))
	st.publish(t, f, []byte("state: idle\nid: run-1\n"))
	s, err := r.FetchStatus()
	if err != nil {
		t.Fatal(err)
	}
	if s.State != "idle" {
		t.Fatalf("FetchStatus reported %q, want the newest", s.State)
	}
	b, err := os.ReadFile(filepath.Join(spool, "status"))
	if err != nil {
		t.Fatalf("the single status file is gone: %v", err)
	}
	if !strings.Contains(string(b), "idle") {
		t.Errorf("the status file holds %q, want the newest document", b)
	}
}
