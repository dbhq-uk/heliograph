// Package cloud is the control node's client for a hosted heliograph service:
// signing in, provisioning an estate, and forwarding the spool.
//
// THIS PACKAGE CANNOT AUTHOR A REQUEST, and that is a property of its
// dependency graph rather than a promise in a comment. It
// cannot author a request because it cannot reach the code that signs one.
//
// The service is given the RECEIVING half of the control identity and never the
// signing half, so "it cannot sign, therefore it cannot cause a run" is the
// central claim of the product. A push path able to produce a sealed request
// would destroy it. `seal.Seal` is the only way to author one over a relay and
// `transport.Transport.PutRequest` is the only way to publish one over anything
// else, so this package imports neither, reaches neither transitively, and
// constructs no `wire.Request` at all. `authorship_test.go` asserts all three
// mechanically and each assertion has been broken on purpose and watched catch
// it.
package cloud

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dbhq-uk/heliograph/internal/wire"
)

// THE SPOOL IS READ AND NEVER WRITTEN, and that is the posture of this file
// rather than a detail of it.
//
// `internal/transport/relay.go:45` says why the spool exists: "A relay deletes
// on collection, so anything not kept here is gone: see spoolDir below for why
// that is a store rather than a cache." It is the customer's own archive, the
// only copy of every log and every status this control node has ever collected,
// and forwarding a copy of it is not a licence to alter it. Nothing in this
// package opens a file for writing, renames one or removes one, and
// `TestThePushLeavesTheSpoolExactlyAsItFoundIt` compares the whole tree before
// and after rather than trusting that sentence.
//
// THE LAYOUT IS THE RELAY'S, spelled here because this side may not import it:
//
//	<spool>/status                    the newest status document, for `heliograph status`
//	<spool>/statuses/status-NNNNNN.txt  every status collected, by signed sequence
//	<spool>/ops-logs/*.txt            every finished log body
//
// `internal/transport/relay.go` writes all three (`spoolStatus`, `keepStatus`,
// `spoolLog`). Two copies of a layout is the drift this repository keeps
// paying for, so `TestTheCloudSpoolReaderAgreesWithTheRelayThatWroteIt` drives
// a real collect and reads it back through here rather than comparing two
// constants.
const (
	logsDirName     = "ops-logs"
	statusesDirName = "statuses"
	statusPrefix    = "status-"
	seqLogPrefix    = "relay-"
)

// Status is one collected status document, exactly as it arrived.
//
// Body is kept verbatim because the bytes ARE the evidence: the archive stores
// them and derives every column from them, so a document this side re-encoded
// would be a rendering of the evidence rather than the evidence.
type Status struct {
	Seq   uint64 // the sequence its sender signed. 0 where the spool records none
	Path  string
	Body  []byte
	RunID string // `id:`, and the only thing that identifies a run
	State string // the station's own word, never rewritten
	UTC   string // the station's clock, not ours
	Log   string // `log:`, a path ON THE STATION. It is not the bytes
}

// Log is one finished capture in the spool.
type Log struct {
	Name   string // as it sits on disk
	Path   string
	Seq    uint64 // where the spool's own naming records one. 0 means absent
	Bytes  int64
	SHA256 string // hex, of the bytes exactly as delivered
}

// Run is what the archive wants: a run identified by its own status documents,
// with the body the spool holds for it.
type Run struct {
	RunID    string
	Seq      uint64   // the newest status's signed sequence. 0 where there is none
	Statuses []Status // oldest first. The station publishes on every transition
	Body     *Log     // nil when this control node holds no body for the run
}

// Spool is everything a spool directory holds, read once.
type Spool struct {
	Dir      string
	Statuses []Status
	Logs     []Log
}

// Read describes a spool without touching it.
//
// An absent or empty spool is not an error. It is the ordinary state of an
// estate configured an hour ago, and a command that refused to run against one
// would be refusing at exactly the moment somebody is checking their setup.
func Read(dir string) (Spool, error) {
	s := Spool{Dir: dir}
	var err error
	if s.Statuses, err = readStatuses(filepath.Join(dir, statusesDirName)); err != nil {
		return Spool{}, err
	}
	if s.Logs, err = readLogs(filepath.Join(dir, logsDirName)); err != nil {
		return Spool{}, err
	}
	return s, nil
}

func readStatuses(dir string) ([]Status, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Status
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		body, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil, rerr
		}
		// ONE PARSER, and it is the one the station is held to.
		// `internal/wire` is the document's own package and a second reader of
		// a `key: value` format is a format that drifts silently. It parses;
		// it cannot publish anything.
		parsed, perr := wire.ParseStatus(body)
		if perr != nil {
			return nil, fmt.Errorf("%s does not parse as a status document: %w", p, perr)
		}
		st := Status{Path: p, Body: body, RunID: parsed.ID, State: parsed.State,
			UTC: parsed.UTC, Log: parsed.Log}
		if n, ok := numberIn(e.Name(), statusPrefix); ok {
			st.Seq = n
		}
		out = append(out, st)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Seq < out[b].Seq })
	return out, nil
}

func readLogs(dir string) ([]Log, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Log
	for _, e := range ents {
		// FLAT, and `.txt` only. A directory under ops-logs is not this
		// transport's layout, and a name that is not `.txt` is not a log:
		// `heliograph logs` filters on exactly that.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		// A RUN IN FLIGHT IS A SNAPSHOT, NOT EVIDENCE. The object store's
		// station side writes a partial capture as `<name>.partial.txt`
		// precisely so the control side hides it rather than listing it beside
		// finished runs, and the archive wants the same distinction: a
		// half-written body stored under a name the finished run will later
		// claim is the one way this path could lose a log.
		if strings.HasSuffix(e.Name(), ".partial.txt") {
			continue
		}
		l, derr := describe(filepath.Join(dir, e.Name()), e.Name())
		if derr != nil {
			return nil, derr
		}
		out = append(out, l)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}

func describe(path, name string) (Log, error) {
	f, err := os.Open(path)
	if err != nil {
		return Log{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Log{}, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return Log{}, err
	}
	l := Log{Name: name, Path: path, Bytes: st.Size(), SHA256: hex.EncodeToString(h.Sum(nil))}
	if n, ok := numberIn(strings.TrimSuffix(name, ".txt"), seqLogPrefix); ok {
		l.Seq = n
	}
	return l, nil
}

// numberIn reads the sequence out of a spooled file's name, where the spool
// wrote one. 0 means absent rather than zero, so an unknown never becomes a
// claim.
func numberIn(name, prefix string) (uint64, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSuffix(name, ".txt"), prefix)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(rest, 10, 64)
	if err != nil || n == 0 {
		return 0, false
	}
	return n, true
}

// Runs groups the spool into what the archive can accept, and names what it
// cannot.
//
// THE RUN ID COMES FROM THE STATUS DOCUMENT AND NOWHERE ELSE. The archive keys
// a run by `id:` (`cloud/src/archive/record.ts`, `normalise`), and that id
// exists on this side only inside a collected status document: the sealed
// envelope binds the estate, station, direction, kind and sequence, and a run
// id is none of them. So a log body with no status naming it cannot be
// attributed, and it is returned separately rather than uploaded under an
// invented id.
//
// THE PAIRING IS THE STATUS'S OWN `log:` FIELD. `Relay.spoolLog` writes a
// collected body under the basename the status named, when the two arrived in
// the same drain and the status carried the higher sequence - which is what the
// station produces, publishing the finished log and then the status naming it.
// Matching on that field is therefore matching on something the station said,
// not on something this side guessed.
func (s Spool) Runs() (runs []Run, unpaired []Log) {
	byName := map[string]*Log{}
	for i := range s.Logs {
		byName[s.Logs[i].Name] = &s.Logs[i]
	}

	claimed := map[string]bool{}
	order := []string{}
	grouped := map[string]*Run{}
	for _, st := range s.Statuses {
		if st.RunID == "" {
			// A status with no id names no run. The archive refuses it for the
			// same reason, saying so rather than inventing a key.
			continue
		}
		r, ok := grouped[st.RunID]
		if !ok {
			r = &Run{RunID: st.RunID}
			grouped[st.RunID] = r
			order = append(order, st.RunID)
		}
		r.Statuses = append(r.Statuses, st)
		if st.Seq > r.Seq {
			r.Seq = st.Seq
		}
		if st.Log != "" {
			if l, have := byName[filepath.Base(st.Log)]; have {
				r.Body = l
				claimed[l.Name] = true
			}
		}
	}
	for _, id := range order {
		runs = append(runs, *grouped[id])
	}
	sort.Slice(runs, func(a, b int) bool {
		if runs[a].Seq != runs[b].Seq {
			return runs[a].Seq < runs[b].Seq
		}
		return runs[a].RunID < runs[b].RunID
	})
	for _, l := range s.Logs {
		if !claimed[l.Name] {
			unpaired = append(unpaired, l)
		}
	}
	return runs, unpaired
}

// Gaps names the sequence numbers this control node never collected.
//
// A relay may legitimately expire a message, so a gap is not proof of
// wrongdoing. It is proof that the archive is not complete, which is the
// difference between "here is what we have" and "here is what we have, and we
// can see something is missing". The service computes the authoritative report;
// this is what `push` can say before it has sent anything, and it says it about
// the sequence numbers the spool records rather than about runs.
func (s Spool) Gaps() []string {
	var seqs []uint64
	for _, st := range s.Statuses {
		if st.Seq != 0 {
			seqs = append(seqs, st.Seq)
		}
	}
	for _, l := range s.Logs {
		if l.Seq != 0 {
			seqs = append(seqs, l.Seq)
		}
	}
	sort.Slice(seqs, func(a, b int) bool { return seqs[a] < seqs[b] })
	var out []string
	for i := 1; i < len(seqs); i++ {
		if seqs[i] == seqs[i-1] || seqs[i] == seqs[i-1]+1 {
			continue
		}
		if seqs[i] == seqs[i-1]+2 {
			out = append(out, fmt.Sprintf("sequence %d was never collected by this control node", seqs[i-1]+1))
			continue
		}
		out = append(out, fmt.Sprintf("sequences %d to %d were never collected by this control node",
			seqs[i-1]+1, seqs[i]-1))
	}
	return out
}
