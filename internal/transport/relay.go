package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	// The spool. A relay deletes on collection, so anything not kept here is
	// gone: see spoolDir below for why that is a store rather than a cache.
	spool       string
	lastStatus  []byte
	stateBroken error
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
	return NewRelayWithSpool(base, estate, station, token, me, peer, statePath, "")
}

// NewRelayWithSpool is NewRelay plus the directory collected logs are kept in.
// Separate because the sequence-number state and the spool answer different
// questions and a caller may reasonably have only the first.
func NewRelayWithSpool(base, estate, station, token string, me *seal.Identity, peer seal.PublicIdentity, statePath, spool string) (*Relay, error) {
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
		spool:     spool,
	}
	r.loadState()
	return r, nil
}

// --- the state, across concurrent invocations --------------------------------
//
// THE CLI IS ONE-SHOT AND NOTHING STOPS TWO OF THEM. `heliograph send` twice, or
// a `send` while a `watch` is polling, both load the same OutSeq, both POST
// under the same number, and the station discards everything after the first -
// permanently, because a repeated sequence is what a replay looks like and
// dropping it is the defence working. Two writers also raced on one `.tmp`
// path, so SeenSeq could regress, and a regressed replay counter accepts every
// message the relay has ever seen.
//
// A LOCK FILE with O_EXCL, and every load-modify-save inside it. The station
// takes the same care with a mkdir lock, and for the same reason.
func (r *Relay) lock() (func(), error) {
	if r.statePath == "" {
		return func() {}, nil
	}
	path := r.statePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			// Ten seconds is far longer than a load-modify-save takes, so this
			// is a holder that died. Removing it is the right call for a tool
			// somebody is waiting in front of; the alternative is a CLI that
			// never works again until a file is deleted by hand.
			_ = os.Remove(path)
			deadline = time.Now().Add(10 * time.Second)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (r *Relay) loadState() {
	b, err := os.ReadFile(r.statePath)
	if err != nil {
		return
	}
	// A STATE FILE THAT WILL NOT PARSE IS NOT SEQUENCE ZERO. Treating it as
	// zero silently accepts every message the relay has ever seen, which is the
	// whole attack the sequence numbers exist to stop. It is left alone and
	// reported, so somebody decides.
	var st relayState
	if err := json.Unmarshal(b, &st); err != nil {
		r.stateBroken = fmt.Errorf("the relay state at %s is not readable: %w.\n"+
			"  It holds the replay counters, and treating an unreadable one as zero\n"+
			"  would accept every message this relay has ever seen. Move it aside\n"+
			"  deliberately if the estate is genuinely being started again", r.statePath, err)
		return
	}
	r.st = st
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
	// Named for this process: two invocations sharing one temporary path is how
	// a half-written state file gets renamed into place.
	tmp := fmt.Sprintf("%s.%d.tmp", r.statePath, os.Getpid())
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
	if r.stateBroken != nil {
		return r.stateBroken
	}
	unlock, err := r.lock()
	if err != nil {
		return err
	}
	defer unlock()
	// Re-read under the lock: another invocation may have advanced it since
	// this process started.
	r.loadState()
	if r.stateBroken != nil {
		return r.stateBroken
	}
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
	// RESERVED BEFORE IT IS USED. A crash between the POST and the save would
	// otherwise hand the same number out again, and the station would drop the
	// second message as a replay.
	if err := r.saveState(); err != nil {
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
	return nil
}

// collect drains the station-to-control queue ONCE, verifies everything in it,
// and files each message by kind.
//
// ONE DRAIN, EVERY KIND, and that is not a tidiness point. The relay deletes on
// collection, so a GET that asked only about `status` would verify the status
// message and DISCARD the log that arrived in the same batch - permanently,
// because it is gone from the relay. The station publishes the finished log and
// then the idle status, so those two travel together constantly. A per-kind
// receive lost the log every time.
//
// A message that does not verify as ANY kind is dropped rather than reported.
// The relay is entitled to hand us anything, and a control that stopped on
// rubbish would be one a hostile relay could halt at will.
func (r *Relay) collect() error {
	if r.stateBroken != nil {
		return r.stateBroken
	}
	unlock, err := r.lock()
	if err != nil {
		return err
	}
	defer unlock()
	r.loadState()
	if r.stateBroken != nil {
		return r.stateBroken
	}
	resp, err := r.do("GET", r.url("s2c", "wait=0"), nil)
	if err != nil {
		return fmt.Errorf("cannot reach the relay: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the relay refused to deliver: %s", resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// AN EMPTY BODY IS AN EMPTY QUEUE, not a protocol error. The station side
	// has always treated it that way; this side used json.Decode straight off
	// the reader and turned it into a bare "EOF", which is the least helpful
	// thing a tool can say about a station that simply has nothing to report.
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	var msgs []relayMsg
	if err := json.Unmarshal(trimmed, &msgs); err != nil {
		return fmt.Errorf("the relay answered with something that is not a message list: %w", err)
	}

	// Oldest first, so a regression is detected against the right baseline.
	sort.Slice(msgs, func(a, b int) bool { return msgs[a].Seq < msgs[b].Seq })

	type collected struct {
		seq  uint64
		body []byte
	}
	var logs []collected
	var statusSeq uint64
	changed := false
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
		// The KIND is inside the signature, so this is not guessing: each is
		// tried and only the true one verifies. That is also what makes a
		// relay unable to relabel a progress snapshot as a finished log.
		for _, kind := range []string{"status", "progress", "log"} {
			m := seal.Meta{
				Estate: r.estate, Station: r.station, Dir: "s2c",
				Seq: msg.Seq, Kind: kind,
				Recipient: r.me.Public().Fingerprint(),
			}
			plain, err := seal.Open(r.me, r.peer, m, msg.Body)
			if err != nil {
				continue
			}
			r.st.SeenSeq = msg.Seq
			changed = true
			switch kind {
			case "status":
				r.lastStatus = plain
				statusSeq = msg.Seq
			case "progress":
				// A PROGRESS MESSAGE IS A PARTIAL LOG, NOT A STATUS.
				// tp_put_progress publishes the status document and then the
				// log FILE as a separate `progress` message. Filing it as a
				// status meant FetchStatus parsed captured output as a status
				// document - so a step that printed `state: idle` could make
				// `watch` return while it was still running, and an ordinary
				// log erased the running state entirely.
				logs = append(logs, collected{msg.Seq, plain})
			case "log":
				logs = append(logs, collected{msg.Seq, plain})
			}
			break
		}
	}
	if !changed {
		return nil
	}
	if err := r.spoolStatus(); err != nil {
		return err
	}
	for _, l := range logs {
		// The status may name this log only if it was collected in the SAME
		// drain and SIGNED WITH A HIGHER SEQUENCE. See spoolLog.
		named := len(logs) == 1 && statusSeq > l.seq
		if err := r.spoolLog(l.seq, l.body, named); err != nil {
			return err
		}
	}
	return r.saveState()
}

// --- the spool ---------------------------------------------------------------
//
// A RELAY IS A QUEUE, NOT A STORE, and that is the whole reason this exists.
// Every other transport keeps the logs where it put them, so `heliograph logs`
// can go and look. Here a log arrives exactly once and is then gone from the
// relay for ever, so if the control side does not keep it, nothing does.
//
// ListLogs and ReadLog used to return an error saying so, which was accurate
// and left the relay with NO WAY TO READ A LOG AT ALL through the CLI - on the
// transport whose entire purpose is getting a log back from a machine nobody
// can reach.
func (r *Relay) spoolDir() string   { return r.spool }
func (r *Relay) logsDir() string    { return filepath.Join(r.spool, "ops-logs") }
func (r *Relay) statusPath() string { return filepath.Join(r.spool, "status") }

func (r *Relay) spoolStatus() error {
	if r.lastStatus == nil || r.spool == "" {
		return nil
	}
	if err := os.MkdirAll(r.spool, 0o700); err != nil {
		return err
	}
	return os.WriteFile(r.statusPath(), r.lastStatus, 0o600)
}

// spoolLog writes a collected log under a name a person can recognise.
//
// THE NAME IS NOT IN THE ENVELOPE, and that is the fact everything here follows
// from. The sealed metadata binds the estate, station, direction, kind and
// sequence - deliberately, because every one of those is something a relay must
// not be able to alter - and a filename is none of them. So NO name for a log is
// authenticated, and this side is choosing one.
//
// THE SEQUENCE NUMBER IS ALWAYS SAFE, because it is signed. The status's `log:`
// field is a nicety on top, and it is only used when the status was collected in
// the same drain AND carries a HIGHER sequence than the log - which is what the
// station actually produces, publishing the finished log and then the status
// naming it. Both of those are signed facts, so a relay cannot manufacture the
// pairing; the most it can do is withhold one, and then the log keeps its
// sequence name.
//
// AND IT NEVER OVERWRITES. Without that, a relay could hand over an old signed
// status naming `probe-A.txt` and then a newer signed log on its own, and the
// new body would replace the old evidence under the old name. A collected log is
// the only copy there will ever be - the relay deleted it on collection - so a
// name that is already taken means the sequence name is used instead. Nothing is
// lost and nothing is misattributed.
func (r *Relay) spoolLog(seq uint64, body []byte, mayUseStatusName bool) error {
	if r.spool == "" {
		return nil
	}
	if err := os.MkdirAll(r.logsDir(), 0o700); err != nil {
		return err
	}
	name := fmt.Sprintf("relay-%06d.txt", seq)
	if mayUseStatusName && r.lastStatus != nil {
		if st, err := wire.ParseStatus(r.lastStatus); err == nil {
			// `<none>` is the station's placeholder for "I could not tell which
			// log this was", not a filename. Writing a file called `<none>` and
			// then not listing it - ListLogs filters on `.txt` - produced a
			// relay that collected a log and reported "no logs yet", which is
			// the worst of both answers.
			cand := filepath.Base(st.Log)
			if strings.HasSuffix(cand, ".txt") && !strings.HasPrefix(cand, "<") {
				if clean, cerr := safeLogName(cand); cerr == nil {
					if _, err := os.Stat(filepath.Join(r.logsDir(), clean)); err != nil {
						name = clean
					}
				}
			}
		}
	}
	return os.WriteFile(filepath.Join(r.logsDir(), name), body, 0o600)
}

func (r *Relay) PutRequest(req wire.Request) error {
	if err := req.Validate(); err != nil {
		return err
	}
	return r.send("request", req.Marshal())
}

// FetchStatus drains the queue and reports the newest status in it.
//
// IT READS FROM THE SPOOL WHEN THE QUEUE IS EMPTY, because the CLI is one-shot
// and the relay deletes on collection. `heliograph status` twice in a row would
// otherwise print the station's state and then nothing at all, which reads as a
// station that has gone away.
func (r *Relay) FetchStatus() (wire.Status, error) {
	if err := r.collect(); err != nil {
		return wire.Status{}, err
	}
	if r.lastStatus == nil && r.spool != "" {
		if b, err := os.ReadFile(r.statusPath()); err == nil {
			r.lastStatus = b
		}
	}
	if r.lastStatus == nil {
		return wire.Status{}, nil
	}
	return wire.ParseStatus(r.lastStatus)
}

// ListLogs drains first, then lists what this control node has collected.
//
// DRAINS FIRST, deliberately: `heliograph logs` immediately after a run must
// see the log that run produced, and the only place it exists is a queue that
// has to be asked. Listing a spool without collecting would report an empty
// station moments after it finished.
func (r *Relay) ListLogs() ([]string, error) {
	if err := r.collect(); err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(r.logsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

func (r *Relay) ReadLog(name string) ([]byte, error) {
	clean, err := safeLogName(name)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(r.logsDir(), clean))
	if err != nil {
		return nil, fmt.Errorf("no log named %q has been collected from this relay. A relay is a queue, not a store: it holds a log only until somebody asks for it", clean)
	}
	return b, nil
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
	//
	// THROUGH collect, NOT A RAW GET. This did `GET s2c?wait=0` and dropped the
	// answer, and the relay DELETES ON COLLECTION - so `heliograph doctor` threw
	// away whatever status or finished log was waiting, permanently. Collecting
	// properly proves exactly the same thing about the token and keeps what it
	// finds, which is the only difference that matters.
	if err := r.collect(); err != nil {
		if strings.Contains(err.Error(), "401") {
			return fmt.Errorf("the relay is reachable but refused this token for estate %q.\n"+
				"  It must be the CONTROL token, which may queue requests and read logs.\n"+
				"  The station's token is a different one and may not queue a request", r.estate)
		}
		return err
	}
	return nil
}

var _ Transport = (*Relay)(nil)
