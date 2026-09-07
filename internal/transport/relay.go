package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dbhq-uk/heliograph/internal/seal"
	"github.com/dbhq-uk/heliograph/internal/wire"
)

// Relay carries the documents through a server neither side trusts.
//
// The only transport that needs no estate infrastructure at all: both sides
// dial out over ordinary HTTPS, so there is no git host to provision, no
// storage account, no VNet and no inbound rule.
//
// Everything is sealed before it leaves and verified after it arrives. The
// relay is outside the trust boundary in both directions, and the second
// direction is the one that matters: a relay that could forge a request would
// have code execution inside every estate at once.
type Relay struct {
	base    string
	estate  string
	station string
	token   string
	me      *seal.Identity
	peer    seal.PublicIdentity
	client  *http.Client

	// state persists the sequence numbers across runs. Freshness rests on
	// these, so losing them is not merely inconvenient: a reset would let the
	// relay replay everything it has ever seen.
	statePath string
	st        relayState
}

type relayState struct {
	OutSeq  uint64 `json:"out_seq"`  // the last we sent
	SeenSeq uint64 `json:"seen_seq"` // the highest we have accepted
}

type relayMsg struct {
	Seq  uint64 `json:"seq"`
	Body []byte `json:"body"`
}

// NewRelay attaches to a relay for one estate and station.
func NewRelay(base, estate, station, token string, me *seal.Identity, peer seal.PublicIdentity, statePath string) (*Relay, error) {
	if base == "" || estate == "" || station == "" {
		return nil, errors.New("a relay needs a URL, an estate and a station")
	}
	if token == "" {
		return nil, errors.New("a relay needs a token: without one every request is refused and it reads like a fault at the far end")
	}
	r := &Relay{
		base:    strings.TrimRight(base, "/"),
		estate:  estate,
		station: station,
		token:   token,
		me:      me,
		peer:    peer,
		// The long poll holds for 25 seconds server side, so the client must
		// wait longer than that or it times out its own successful poll.
		client:    &http.Client{Timeout: 90 * time.Second},
		statePath: statePath,
	}
	r.loadState()
	return r, nil
}

func (r *Relay) loadState() {
	b, err := os.ReadFile(r.statePath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, &r.st)
}

func (r *Relay) saveState() error {
	if r.statePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(r.statePath), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(r.st)
	if err != nil {
		return err
	}
	// Write-then-rename: a truncated state file reads as sequence zero, and
	// sequence zero accepts every replay the relay has ever seen.
	tmp := r.statePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.statePath)
}

func (r *Relay) Dir() string    { return r.base }
func (r *Relay) Branch() string { return r.station }

func (r *Relay) Describe() string {
	return fmt.Sprintf("relay %s, estate %s, station %s, peer %s",
		r.base, r.estate, r.station, r.peer.Fingerprint())
}

func (r *Relay) url(dir string, q string) string {
	u := fmt.Sprintf("%s/v1/%s/%s/%s", r.base, r.estate, r.station, dir)
	if q != "" {
		u += "?" + q
	}
	return u
}

func (r *Relay) do(method, url string, body []byte) (*http.Response, error) {
	var rdr *bytes.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	var req *http.Request
	var err error
	if rdr != nil {
		req, err = http.NewRequest(method, url, rdr)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Content-Type", "application/json")
	return r.client.Do(req)
}

// send seals a document and queues it.
func (r *Relay) send(kind string, plaintext []byte) error {
	r.st.OutSeq++
	m := seal.Meta{
		Estate: r.estate, Station: r.station, Dir: "c2s",
		Seq: r.st.OutSeq, Kind: kind,
		Recipient: r.peer.Fingerprint(),
	}
	sealed, err := seal.Seal(r.me, r.peer, m, plaintext)
	if err != nil {
		return err
	}
	body, err := json.Marshal(relayMsg{Seq: m.Seq, Body: sealed})
	if err != nil {
		return err
	}
	resp, err := r.do("POST", r.url("c2s", ""), body)
	if err != nil {
		return fmt.Errorf("cannot reach the relay: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("the relay refused the message: %s", resp.Status)
	}
	return r.saveState()
}

// receive collects and verifies, returning the newest acceptable document.
func (r *Relay) receive(kind string, wait bool) ([]byte, error) {
	q := "wait=0"
	if wait {
		q = ""
	}
	resp, err := r.do("GET", r.url("s2c", q), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot reach the relay: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the relay refused to deliver: %s", resp.Status)
	}
	var msgs []relayMsg
	if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
		return nil, err
	}

	// Oldest first, so a regression is detected against the right baseline.
	sort.Slice(msgs, func(a, b int) bool { return msgs[a].Seq < msgs[b].Seq })

	var out []byte
	for _, msg := range msgs {
		// THE REPLAY CHECK. A gap is tolerated because the relay may
		// legitimately expire a message; a regression is refused, because that
		// is the relay replaying an old one. The attack is concrete:
		// --allow-actions and CONFIRM=yes are decided days before the request
		// that uses them, so a replayed request is a destructive step
		// re-running with all its gates already satisfied.
		if msg.Seq <= r.st.SeenSeq {
			continue
		}
		m := seal.Meta{
			Estate: r.estate, Station: r.station, Dir: "s2c",
			Seq: msg.Seq, Kind: kind,
			Recipient: r.me.Public().Fingerprint(),
		}
		plain, err := seal.Open(r.me, r.peer, m, msg.Body)
		if err != nil {
			// Not fatal for the batch: a message of another kind is ordinary,
			// and a message that does not verify is one this station did not
			// write, which the relay is entitled to have handed us and we are
			// entitled to drop.
			continue
		}
		r.st.SeenSeq = msg.Seq
		out = plain
	}
	if out != nil {
		if err := r.saveState(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Relay) PutRequest(req wire.Request) error {
	if err := req.Validate(); err != nil {
		return err
	}
	return r.send("request", req.Marshal())
}

func (r *Relay) FetchStatus() (wire.Status, error) {
	b, err := r.receive("status", false)
	if err != nil {
		return wire.Status{}, err
	}
	if b == nil {
		return wire.Status{}, nil
	}
	return wire.ParseStatus(b)
}

// ListLogs and ReadLog are not how a relay works.
//
// There is nothing to list: the relay is a queue, not a store, and a log
// arrives once and is then gone from it. The CLI keeps what it collects, and
// saying so is better than presenting an empty list that looks like a station
// which has produced nothing.
func (r *Relay) ListLogs() ([]string, error) {
	return nil, errors.New("a relay is a queue, not a store: logs are kept where you collected them, by `heliograph watch`")
}

func (r *Relay) ReadLog(string) ([]byte, error) {
	return nil, errors.New("a relay is a queue, not a store: logs are kept where you collected them")
}

func (r *Relay) Check() error {
	resp, err := r.do("GET", r.base+"/health", nil)
	if err != nil {
		return fmt.Errorf("cannot reach the relay at %s: %w", r.base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the relay at %s answered %s", r.base, resp.Status)
	}
	// Health needs no token, so it proves reachability and nothing else. Prove
	// the credential too, or a wrong token is discovered by a failed send.
	resp2, err := r.do("GET", r.url("s2c", "wait=0"), nil)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("the relay is reachable but refused this token for estate %q", r.estate)
	}
	if resp2.StatusCode != http.StatusOK {
		return fmt.Errorf("the relay answered %s", resp2.Status)
	}
	return nil
}

var _ Transport = (*Relay)(nil)
