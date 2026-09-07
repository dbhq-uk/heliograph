package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/dbhq-uk/heliograph/internal/seal"
	"github.com/dbhq-uk/heliograph/internal/wire"
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
	s.outSeq++
	meta := seal.Meta{
		Estate: s.estate, Station: s.name, Dir: "s2c", Seq: s.outSeq,
		Kind: "status", Recipient: s.peer.Fingerprint(),
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
