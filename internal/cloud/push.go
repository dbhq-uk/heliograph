package cloud

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Push: forwarding the spool the control node already keeps.
//
// WHY THE CLOUD DOES NOT READ THE RELAY. `internal/transport/relay.go:119` -
// `Take` deletes the queue as it returns it, so a hosted poller racing the
// customer's own control node would silently eat their logs. Half the runs
// would reach the archive and half would reach the customer, and neither side
// would know which. So the service never touches the relay, and this side
// forwards what it already has.
//
// WHY THAT IS NEARLY FREE. The archive already exists locally, because a relay
// deletes on collection and the spool has always had to be a store rather than
// a cache. There is nothing to collect, nothing to re-run and nothing to
// decrypt: everything was opened on this machine when it arrived.
//
// THE ORDER IS NOT NEGOTIABLE. The status document first, because the archive
// builds the record from those bytes and a manifest is a summary; then the
// manifest, so the client learns what to skip; then the chunks; then
// completion. A body sent before its status has nothing to attach to, and the
// service says so rather than guessing.
//
// The protocol is `heliograph-io/heliograph-cloud#16` and the record is `#19`.

// DefaultChunkBytes is 1 MiB.
//
// IT MUST NOT CHANGE BETWEEN ATTEMPTS ON ONE RUN. Resume works by asking the
// service which offsets it already holds and sending the ones it does not, so a
// client that used 1 MiB on Monday and 4 MiB on Tuesday would send overlapping
// ranges that the service refuses as "offset N already holds different bytes".
// A constant rather than a tuneable, for exactly that reason.
const DefaultChunkBytes = 1 << 20

// Target is where a push is going.
type Target struct {
	Station    string // the station wire id, which is the estate's station name
	Auth       Auth   // the ESTATE credential, under the Control scheme
	ChunkBytes int    // 0 means DefaultChunkBytes
}

// Receipt is the archive's record of what it accepted, kept so that this side
// can say later what it handed over and when.
type Receipt struct {
	Receipt  string `json:"receipt"`
	RunID    string `json:"runId"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
	Chunks   int    `json:"chunks"`
	StoredAt string `json:"storedAt"`
}

// Result is what a push did, in terms a person can act on.
type Result struct {
	StatusesSent []string // run ids whose status documents the archive accepted
	Uploaded     []string // run ids whose bodies were stored by this push
	Skipped      []string // run ids the archive already held
	Resumed      []string // run ids whose upload continued rather than restarted
	BytesSent    int64

	// Bodies in the spool that no status document names, by name and local
	// path. Reported rather than uploaded under an invented id, and reported
	// rather than passed over in silence.
	Unattributable []string

	// The archive's own words about what is missing, unedited.
	Completeness string
	Missing      []string

	// What this side can see before it has sent anything: sequence numbers the
	// spool does not hold. The archive's report is authoritative; this is
	// honest about a different thing, which is what THIS control node collected.
	Uncollected []string

	Receipts map[string]Receipt
}

type manifestVerdict struct {
	RunID         string  `json:"runId"`
	Verdict       string  `json:"verdict"`
	SHA256        string  `json:"sha256"`
	Bytes         int64   `json:"bytes"`
	Held          []int64 `json:"held"`
	ReceivedBytes int64   `json:"receivedBytes"`
	Problem       string  `json:"problem"`
}

type manifestAnswer struct {
	Station string `json:"station"`
	Runs    struct {
		Items        []manifestVerdict `json:"items"`
		Completeness string            `json:"completeness"`
		Missing      []string          `json:"missing"`
	} `json:"runs"`
}

type chunkReceipt struct {
	Receipt       string `json:"receipt"`
	RunID         string `json:"runId"`
	Offset        int64  `json:"offset"`
	Bytes         int64  `json:"bytes"`
	SHA256        string `json:"sha256"`
	StoredAt      string `json:"storedAt"`
	ReceivedBytes int64  `json:"receivedBytes"`
	DeclaredBytes int64  `json:"declaredBytes"`
}

// Push forwards a spool to the archive.
//
// IT AUTHORS NOTHING. Every byte it sends came off disk, and the only
// documents it constructs are a manifest and a query string. See the package
// comment and `authorship_test.go` for how that is enforced rather than
// promised.
func (s *Service) Push(t Target, sp Spool) (Result, error) {
	res := Result{Receipts: map[string]Receipt{}}
	if t.Station == "" {
		return res, errors.New("a push needs the station it is about: that is what the archive files a run under")
	}
	chunk := t.ChunkBytes
	if chunk <= 0 {
		chunk = DefaultChunkBytes
	}

	runs, unpaired := sp.Runs()
	for _, l := range unpaired {
		// NAMED, not counted. "one log could not be sent" leaves somebody
		// looking for it; the path and the reason let them decide.
		res.Unattributable = append(res.Unattributable,
			fmt.Sprintf("%s (%s): no collected status document names it, so the archive has no run to file it under",
				l.Name, l.Path))
	}
	res.Uncollected = sp.Gaps()

	if len(runs) == 0 {
		return res, nil
	}

	// 1. The status documents, oldest sequence first.
	//
	// EVERY TRANSITION, not just the last. Ingest upserts and applies a
	// transition only when the incoming `utc:` is at least as recent as the
	// stored one, so re-sending is safe and expected - and sending only the
	// final one produces a record that never saw the run start.
	for _, run := range runs {
		for _, st := range run.Statuses {
			if err := s.sendStatus(t, st); err != nil {
				return res, err
			}
			res.StatusesSent = append(res.StatusesSent, run.RunID)
		}
	}

	// 2. The manifest, which is what makes push cheap to run habitually rather
	// than a decision: a station with four hundred runs already uploaded sends
	// one request and is told to send nothing.
	var withBodies []Run
	for _, run := range runs {
		if run.Body != nil {
			withBodies = append(withBodies, run)
		}
	}
	if len(withBodies) == 0 {
		return res, nil
	}
	answer, err := s.sendManifest(t, withBodies)
	if err != nil {
		return res, err
	}
	res.Completeness = answer.Runs.Completeness
	res.Missing = answer.Runs.Missing

	verdicts := map[string]manifestVerdict{}
	for _, v := range answer.Runs.Items {
		verdicts[v.RunID] = v
	}

	// 3. The bodies.
	var problems []string
	for _, run := range withBodies {
		v, ok := verdicts[run.RunID]
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"%s: the archive returned no verdict for this run, so nothing was sent. The log is at %s",
				run.RunID, run.Body.Path))
			continue
		}
		switch v.Verdict {
		case "have":
			res.Skipped = append(res.Skipped, run.RunID)
			// THE DIGESTS ARE COMPARED. A stored body is final and the archive
			// answers `have` with the digest it holds rather than replacing it,
			// which is what makes an archive uneditable after the fact. If the
			// two differ, that is a fact worth raising with a person rather
			// than silently accepting.
			if v.SHA256 != "" && v.SHA256 != run.Body.SHA256 {
				problems = append(problems, fmt.Sprintf(
					"%s: the archive holds a body with digest %s and this spool holds %s.\n"+
						"    The archive does not replace a stored body, so neither copy has been altered.\n"+
						"    The local copy is at %s",
					run.RunID, v.SHA256, run.Body.SHA256, run.Body.Path))
			}
		case "send", "resume":
			if v.Verdict == "resume" {
				res.Resumed = append(res.Resumed, run.RunID)
			}
			sent, uerr := s.sendBody(t, run, v, chunk)
			res.BytesSent += sent
			if uerr != nil {
				// A FAILED PUSH NEVER LOSES A LOG, and it names the local path
				// rather than exiting on it. The project's rule for `cap_push`,
				// and it applies here for the same reason: each round trip
				// through an operator is expensive and none may be wasted by
				// tooling that only reports success.
				return res, fmt.Errorf("%w\n  the log is still here, unchanged: %s", uerr, run.Body.Path)
			}
			receipt, cerr := s.completeBody(t, run)
			if cerr != nil {
				return res, fmt.Errorf("%w\n  the log is still here, unchanged: %s", cerr, run.Body.Path)
			}
			res.Receipts[run.RunID] = receipt
			res.Uploaded = append(res.Uploaded, run.RunID)
		case "unknown-run":
			// Retrying would fail for ever: the archive has no record to attach
			// a body to, and a record is built from a status document.
			problems = append(problems, fmt.Sprintf(
				"%s: the archive has no record of this run, so its log was not sent.\n"+
					"    A record is built from a status document and this push sent %d of them.\n"+
					"    The log is at %s",
				run.RunID, len(run.Statuses), run.Body.Path))
		case "rejected":
			problems = append(problems, fmt.Sprintf("%s: the archive refused the manifest entry: %s.\n    The log is at %s",
				run.RunID, v.Problem, run.Body.Path))
		default:
			// A verdict this build has never heard of is reported verbatim
			// rather than guessed at. Guessing "send" would upload against a
			// refusal; guessing "skip" would report a log as archived that is
			// not.
			problems = append(problems, fmt.Sprintf(
				"%s: the archive answered with a verdict this build does not know, %q, so nothing was sent.\n    The log is at %s",
				run.RunID, v.Verdict, run.Body.Path))
		}
	}
	if len(problems) > 0 {
		return res, fmt.Errorf("%d run(s) were not archived:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
	return res, nil
}

func (s *Service) sendStatus(t Target, st Status) error {
	q := url.Values{}
	q.Set("station", t.Station)
	// `relay` because that is what a spool IS: the store a relay transport has
	// to keep because the relay deletes on collection. Other transports get
	// their own ingest paths, and inventing one here would mislabel provenance,
	// which the archive treats as a claim rather than a hint.
	q.Set("transport", "relay")
	if st.Seq != 0 {
		q.Set("sequence", strconv.FormatUint(st.Seq, 10))
	}
	var out struct {
		Run     string `json:"run"`
		Created bool   `json:"created"`
	}
	if err := s.postBytes("/ingest/status?"+q.Encode(), t.Auth, st.Body, &out); err != nil {
		return fmt.Errorf("the status document at %s was refused: %w", st.Path, err)
	}
	return nil
}

func (s *Service) sendManifest(t Target, runs []Run) (manifestAnswer, error) {
	type entry struct {
		RunID  string `json:"run_id"`
		Seq    uint64 `json:"seq,omitempty"`
		SHA256 string `json:"sha256"`
		Bytes  int64  `json:"bytes"`
	}
	body := struct {
		Station string  `json:"station"`
		Runs    []entry `json:"runs"`
	}{Station: t.Station}
	for _, r := range runs {
		body.Runs = append(body.Runs, entry{
			RunID: r.RunID, Seq: r.Seq, SHA256: r.Body.SHA256, Bytes: r.Body.Bytes,
		})
	}
	var out manifestAnswer
	if err := s.postJSON("/ingest/manifest", t.Auth, body, &out); err != nil {
		return manifestAnswer{}, fmt.Errorf("the archive would not take the manifest: %w", err)
	}
	return out, nil
}

// sendBody sends the chunks the archive does not already hold.
//
// It sends the ones NOT IN `held` rather than everything from a high-water
// mark, because `held` is a set of offsets and a set can have a hole in it. A
// client that resumed from the total byte count would skip a hole and then fail
// at completion, where the whole-body digest is checked - which is the right
// refusal for the wrong reason, and a long way from the chunk that went missing.
func (s *Service) sendBody(t Target, run Run, v manifestVerdict, chunk int) (int64, error) {
	held := map[int64]bool{}
	for _, o := range v.Held {
		held[o] = true
	}

	f, err := os.Open(run.Body.Path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var sent int64
	buf := make([]byte, chunk)
	ordinal := 0
	for offset := int64(0); offset < run.Body.Bytes; offset += int64(chunk) {
		ordinal++
		size := chunk
		if rem := run.Body.Bytes - offset; rem < int64(chunk) {
			size = int(rem)
		}
		if held[offset] {
			continue
		}
		if _, err := f.ReadAt(buf[:size], offset); err != nil {
			return sent, fmt.Errorf("reading %s at offset %d: %w", run.Body.Path, offset, err)
		}
		body := buf[:size]
		sum := sha256.Sum256(body)
		digest := hex.EncodeToString(sum[:])

		q := url.Values{}
		q.Set("offset", strconv.FormatInt(offset, 10))
		q.Set("sha256", digest)
		path := fmt.Sprintf("/ingest/station/%s/run/%s/chunk?%s",
			url.PathEscape(t.Station), url.PathEscape(run.RunID), q.Encode())

		var receipt chunkReceipt
		if err := s.putBytes(path, t.Auth, body, &receipt); err != nil {
			return sent, fmt.Errorf("run %s, chunk %d at offset %d of %d bytes: %w",
				run.RunID, ordinal, offset, run.Body.Bytes, err)
		}
		sent += int64(size)

		// THE RECEIPT IS CHECKED, not filed. A receipt is the archive's record
		// of what it accepted, so one acknowledging bytes that were never sent
		// is a disagreement about the evidence rather than a rounding error,
		// and carrying on would mean completing an upload nobody has agreed the
		// shape of.
		if receipt.Bytes != int64(size) {
			return sent, fmt.Errorf("run %s, chunk %d at offset %d: the archive acknowledged %d bytes and %d were sent",
				run.RunID, ordinal, offset, receipt.Bytes, size)
		}
		if receipt.DeclaredBytes > 0 && receipt.ReceivedBytes > receipt.DeclaredBytes {
			return sent, fmt.Errorf("run %s, chunk %d at offset %d: the archive acknowledged holding %d bytes for a %d byte log",
				run.RunID, ordinal, offset, receipt.ReceivedBytes, receipt.DeclaredBytes)
		}
		if receipt.SHA256 != "" && receipt.SHA256 != digest {
			return sent, fmt.Errorf("run %s, chunk %d at offset %d: the archive acknowledged digest %s and %s was sent",
				run.RunID, ordinal, offset, receipt.SHA256, digest)
		}
	}
	return sent, nil
}

func (s *Service) completeBody(t Target, run Run) (Receipt, error) {
	path := fmt.Sprintf("/ingest/station/%s/run/%s/complete",
		url.PathEscape(t.Station), url.PathEscape(run.RunID))
	var out Receipt
	if err := s.postJSON(path, t.Auth, nil, &out); err != nil {
		return Receipt{}, fmt.Errorf("run %s could not be completed: %w", run.RunID, err)
	}
	// The whole-body digest is verified by the archive before it stores
	// anything, and it answers with what it stored. Comparing it here is what
	// turns "the call succeeded" into "the archive holds the bytes this spool
	// holds".
	if out.SHA256 != "" && out.SHA256 != run.Body.SHA256 {
		return Receipt{}, fmt.Errorf("run %s: the archive stored digest %s and this spool holds %s",
			run.RunID, out.SHA256, run.Body.SHA256)
	}
	return out, nil
}

// Summary is what `heliograph push` prints.
//
// It never says "archived" about anything it did not get a receipt for, and it
// never leaves an unattributable body out of the count. A push that reported
// only its successes would be the tooling this project's own rule forbids.
func (r Result) Summary() []string {
	var out []string
	out = append(out, fmt.Sprintf("%d status document(s) sent, %d body/bodies uploaded, %d already held",
		len(r.StatusesSent), len(r.Uploaded), len(r.Skipped)))
	if len(r.Resumed) > 0 {
		sort.Strings(r.Resumed)
		out = append(out, "  resumed: "+strings.Join(r.Resumed, ", "))
	}
	if r.BytesSent > 0 {
		out = append(out, fmt.Sprintf("  %d bytes sent", r.BytesSent))
	}
	if r.Completeness != "" {
		out = append(out, "  the archive reports completeness: "+r.Completeness)
	}
	for _, m := range r.Missing {
		out = append(out, "  missing: "+m)
	}
	for _, m := range r.Uncollected {
		out = append(out, "  "+m)
	}
	for _, u := range r.Unattributable {
		out = append(out, "  not sent: "+u)
	}
	return out
}
