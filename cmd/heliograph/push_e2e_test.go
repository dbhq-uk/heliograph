package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dbhq-uk/heliograph/internal/cloud"
	"github.com/dbhq-uk/heliograph/internal/seal"
)

// The round trip that proves `push` forwards a real spool.
//
// EVERYTHING HERE IS REAL EXCEPT THE ARCHIVE. A real station from
// `station/bash`, a real relay API, a real key exchange, a real capture, and
// the real binary driven the way somebody would type it. The archive is a stub
// because the service lives in another repository and this test is about THIS
// side: what it sends, in what order, and what it refuses to do.
//
// It is the one test that can prove the claim end to end, because the claim is
// about the whole path rather than about a package: with a station and a relay
// both present, a push must send nothing towards the station's request queue.

// ingestDouble is the archive's ingest surface, and a witness.
type ingestDouble struct {
	mu       sync.Mutex
	records  map[string]bool
	declared map[string]string
	held     map[string]map[int64][]byte
	stored   map[string][]byte
	statuses []string
	token    string
}

func newIngestDouble(token string) *ingestDouble {
	return &ingestDouble{
		records: map[string]bool{}, declared: map[string]string{},
		held: map[string]map[int64][]byte{}, stored: map[string][]byte{}, token: token,
	}
}

func (d *ingestDouble) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if got := r.Header.Get("Authorization"); got != "Control "+d.token {
		http.Error(w, `{"refusal":{"cause":"credential-invalid","line":"that credential is not one of ours"}}`, 401)
		return
	}
	run := func() string {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, s := range parts {
			if s == "run" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	switch {
	case r.URL.Path == "/ingest/status":
		body, _ := io.ReadAll(r.Body)
		id := ""
		for _, line := range strings.Split(string(body), "\n") {
			if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "id" {
				id = strings.TrimSpace(v)
			}
		}
		if id == "" {
			http.Error(w, `{"problem":"no id:"}`, 400)
			return
		}
		d.records[id] = true
		d.statuses = append(d.statuses, string(body))
		w.WriteHeader(201)
		_, _ = fmt.Fprintf(w, `{"run":%q,"created":true}`, id)
	case r.URL.Path == "/ingest/manifest":
		var body struct {
			Runs []struct {
				RunID  string `json:"run_id"`
				SHA256 string `json:"sha256"`
			} `json:"runs"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		items := []map[string]any{}
		for _, e := range body.Runs {
			switch {
			case !d.records[e.RunID]:
				items = append(items, map[string]any{"runId": e.RunID, "verdict": "unknown-run"})
			case len(d.stored[e.RunID]) > 0:
				items = append(items, map[string]any{"runId": e.RunID, "verdict": "have",
					"sha256": digestOf(d.stored[e.RunID]), "bytes": len(d.stored[e.RunID])})
			default:
				d.declared[e.RunID] = e.SHA256
				offsets := []int64{}
				for o := range d.held[e.RunID] {
					offsets = append(offsets, o)
				}
				sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })
				if len(offsets) == 0 {
					items = append(items, map[string]any{"runId": e.RunID, "verdict": "send"})
				} else {
					items = append(items, map[string]any{"runId": e.RunID, "verdict": "resume", "held": offsets})
				}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"runs": map[string]any{"items": items, "completeness": "unverified", "missing": []string{}},
		})
	case strings.HasSuffix(r.URL.Path, "/chunk"):
		id := run()
		offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
		body, _ := io.ReadAll(r.Body)
		if digestOf(body) != r.URL.Query().Get("sha256") {
			http.Error(w, `{"problem":"chunk digest mismatch"}`, 422)
			return
		}
		if d.held[id] == nil {
			d.held[id] = map[int64][]byte{}
		}
		d.held[id][offset] = body
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"receipt": "c", "runId": id, "offset": offset, "bytes": len(body),
			"sha256": digestOf(body), "storedAt": "now",
		})
	case strings.HasSuffix(r.URL.Path, "/complete"):
		id := run()
		offsets := []int64{}
		for o := range d.held[id] {
			offsets = append(offsets, o)
		}
		sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })
		var whole []byte
		for _, o := range offsets {
			whole = append(whole, d.held[id][o]...)
		}
		if digestOf(whole) != d.declared[id] {
			http.Error(w, `{"problem":"body digest mismatch"}`, 422)
			return
		}
		d.stored[id] = whole
		_ = json.NewEncoder(w).Encode(map[string]any{
			"receipt": "receipt-" + id, "runId": id, "bytes": len(whole),
			"sha256": digestOf(whole), "chunks": len(offsets), "storedAt": "now",
		})
	default:
		http.Error(w, `{"problem":"not found"}`, 404)
	}
}

func (d *ingestDouble) body(run string) []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]byte{}, d.stored[run]...)
}

func (d *ingestDouble) statusCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.statuses)
}

func (d *ingestDouble) storedCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.stored)
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// countingRelay is the relay double with a witness on the one queue that
// matters: `c2s` POST is how a request reaches a station, and it is the only
// one.
type countingRelay struct {
	inner    http.Handler
	mu       sync.Mutex
	requests int
}

func (c *countingRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/c2s") {
		c.mu.Lock()
		c.requests++
		c.mu.Unlock()
	}
	c.inner.ServeHTTP(w, r)
}

func (c *countingRelay) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests
}

func TestPushForwardsARealSpoolAndAuthorsNothing(t *testing.T) {
	stationRoot := stationDir(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}

	const ctlToken, stnToken, relayEstate = "control-token", "station-token", "push-e2e"
	relay := &countingRelay{inner: newRelayDouble(relayEstate, ctlToken, stnToken)}
	relaySrv := httptest.NewServer(relay)
	defer relaySrv.Close()

	archive := newIngestDouble("estate-credential")
	archiveSrv := httptest.NewServer(archive)
	defer archiveSrv.Close()

	base := t.TempDir()
	work := filepath.Join(base, "work")
	cfg := filepath.Join(base, "config")

	sh(t, base, filepath.Join(stationRoot, "station", "bootstrap.sh"), work)
	step := "#!/usr/bin/env bash\n# heliograph-mode: read-only\necho PUSHED-FROM-THE-SPOOL\n"
	if err := os.WriteFile(filepath.Join(work, "steps", "probe.sh"), []byte(step), 0o755); err != nil {
		t.Fatal(err)
	}
	sealBin := filepath.Join(work, "heliograph-seal")
	sh(t, ".", "go", "build", "-o", sealBin, "github.com/dbhq-uk/heliograph/cmd/heliograph-seal")

	bin := filepath.Join(base, "heliograph")
	sh(t, ".", "go", "build", "-o", bin, "github.com/dbhq-uk/heliograph/cmd/heliograph")
	run := func(env []string, args ...string) (string, error) {
		cmd := exec.Command(bin, args...)
		cmd.Dir = base
		cmd.Env = append(append(os.Environ(), "XDG_CONFIG_HOME="+cfg), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	hg := func(env []string, args ...string) string {
		t.Helper()
		out, err := run(env, args...)
		if err != nil {
			t.Fatalf("heliograph %v: %v\n%s", args, err, out)
		}
		return out
	}
	withToken := []string{"HELIOGRAPH_RELAY_TOKEN=" + ctlToken}

	// --- a real estate, a real key exchange, a real run ----------------------
	hg(nil, "init", "pushed", "--transport", "relay",
		"--dir", relaySrv.URL, "--relay-estate", relayEstate, "--scope", "s1")

	ctlID, err := seal.LoadIdentityFile(filepath.Join(cfg, "heliograph", "keys", "pushed.identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	stationID := filepath.Join(work, "station-identity.json")
	stationPeer := filepath.Join(work, "station-peer")
	sh(t, work, sealBin, "keygen", "--out", stationID)
	pubOut, err := exec.Command(sealBin, "public", "--identity", stationID).Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stationPeer, []byte(ctlID.Public().Encode()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hg(nil, "relay", "peer", "-e", "pushed", strings.TrimSpace(string(pubOut)))
	hg(withToken, "send", "steps/probe.sh")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "./start.sh", "--", "--once", "--interval", "1")
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = work
	cmd.Env = append(os.Environ(),
		"TRANSPORT=relay", "RELAY_URL="+relaySrv.URL, "RELAY_ESTATE="+relayEstate,
		"RELAY_STATION=s1", "RELAY_TOKEN="+stnToken,
		"RELAY_IDENTITY="+stationID, "RELAY_PEER="+stationPeer, "RELAY_SEAL="+sealBin)
	cmd.Env = append(cmd.Env, noBackgroundGit...)
	o, runErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("the station never completed a run within 90s:\n%s", o)
	}
	if runErr != nil {
		t.Fatalf("the station did not run: %v\n%s", runErr, o)
	}
	// Collect it into the spool, which is what `status` does.
	hg(withToken, "status")

	spool := filepath.Join(cfg, "heliograph", "state", "pushed.relay")
	before := treeOf(t, spool)
	if !strings.Contains(before, "statuses/") {
		t.Fatalf("the spool kept no status documents, so there is nothing to key a run by:\n%s", before)
	}

	// --- the credential, stored rather than pasted ---------------------------
	credPath := filepath.Join(cfg, "heliograph", "credentials.json")
	if err := os.MkdirAll(filepath.Dir(credPath), 0o700); err != nil {
		t.Fatal(err)
	}
	creds := fmt.Sprintf(`{"version":1,"estates":{"pushed":{"service":%q,"estate":%q,"token":"estate-credential"}}}`,
		archiveSrv.URL, relayEstate)
	if err := os.WriteFile(credPath, []byte(creds), 0o600); err != nil {
		t.Fatal(err)
	}

	// --- the push ------------------------------------------------------------
	requestsBefore := relay.count()
	out := hg(nil, "push", "-e", "pushed")
	t.Logf("heliograph push:\n%s", out)

	if archive.statusCount() == 0 {
		t.Fatal("no status document reached the archive, so no run was recorded")
	}
	if archive.storedCount() != 1 {
		t.Fatalf("%d body/bodies stored, want the one this run produced", archive.storedCount())
	}

	// A SNAPSHOT IS NOT EVIDENCE, and since #116 the spool holds one: a
	// progress snapshot now lands under the station's own name with a
	// `.partial.txt` suffix rather than under a sequence name, so a plain
	// `*.txt` glob finds two files for one run.
	//
	// That is exactly the distinction `push` has to keep. The snapshot is a
	// half-written capture of a run that was still going, and storing it under
	// a name the finished run will later claim is the one way this path could
	// lose a log. So the finished body is picked deliberately here, and the
	// count below asserts the snapshot was not uploaded.
	all, err := filepath.Glob(filepath.Join(spool, "ops-logs", "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var localLogs, snapshots []string
	for _, p := range all {
		if strings.HasSuffix(p, ".partial.txt") {
			snapshots = append(snapshots, p)
			continue
		}
		localLogs = append(localLogs, p)
	}
	if len(localLogs) != 1 {
		t.Fatalf("want one finished log in the spool, got %v (snapshots: %v)", localLogs, snapshots)
	}
	want, err := os.ReadFile(localLogs[0])
	if err != nil {
		t.Fatal(err)
	}
	var stored []byte
	for _, id := range archiveRunIDs(archive) {
		if b := archive.body(id); len(b) > 0 {
			stored = b
		}
	}
	if string(stored) != string(want) {
		t.Errorf("the archived body is not the collected log byte for byte (%d stored, %d local)",
			len(stored), len(want))
	}
	if !strings.Contains(string(stored), "PUSHED-FROM-THE-SPOOL") {
		t.Error("the archived body does not carry the step's output")
	}
	// THE SNAPSHOT WAS NOT OFFERED AS A BODY, asserted against the real spool
	// this station just produced rather than against the bytes.
	//
	// Comparing bytes was tried first and is UNSOUND: a step that finishes
	// inside one poll leaves a snapshot byte-identical to the finished log, so
	// "the archive holds the snapshot" and "the archive holds the log" are the
	// same string and the check fired on a correct push. What is observable is
	// what the reader offered, so that is what is asserted.
	if len(snapshots) > 0 {
		sp, rerr := cloud.Read(spool)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if len(sp.Logs) != 1 || strings.HasSuffix(sp.Logs[0].Name, ".partial.txt") {
			t.Errorf("the spool reader offered %d body/bodies including a partial capture: %+v",
				len(sp.Logs), sp.Logs)
		}
		if len(sp.Snapshots) != len(snapshots) {
			t.Errorf("%d snapshot(s) on disk and %d accounted for: an unaccounted snapshot took a "+
				"sequence number and would be reported as a message that never arrived",
				len(snapshots), len(sp.Snapshots))
		}
	}

	if !strings.Contains(string(stored), " | PUSHED-FROM-THE-SPOOL") {
		t.Error("the archived body lost the timestamp column, which is the one property these logs have")
	}

	// THE CLAIM. With a station and a relay both present and reachable, a push
	// sent nothing towards the station's request queue. `c2s` POST is the only
	// way a request reaches a station over this transport.
	if after := relay.count(); after != requestsBefore {
		t.Errorf("`push` sent %d message(s) to the station's request queue. "+
			"It may forward evidence and may never author a request", after-requestsBefore)
	}

	// THE SPOOL IS UNTOUCHED. Snapshot the whole tree, not one path.
	if after := treeOf(t, spool); after != before {
		t.Errorf("the push changed the spool.\nbefore:\n%s\nafter:\n%s", before, after)
	}

	// AND IT IS IDEMPOTENT. The second push is what somebody actually does,
	// because a push at the end of every session is the habit this is for.
	second := hg(nil, "push", "-e", "pushed")
	t.Logf("second push:\n%s", second)
	if archive.storedCount() != 1 {
		t.Errorf("a second push produced %d stored bodies", archive.storedCount())
	}
	if !strings.Contains(second, "1 already held") {
		t.Errorf("the second push did not report the body as already held:\n%s", second)
	}
	if after := treeOf(t, spool); after != before {
		t.Error("the second push changed the spool")
	}

	// A refusal is read as a refusal. The archive answers 401 with the object
	// heliograph-io/heliograph-cloud#10 specifies, and the reader has to get the
	// service's own sentence rather than a status code.
	bad, err := run([]string{"HELIOGRAPH_CLOUD_TOKEN=not-the-credential"}, "push", "-e", "pushed")
	if err == nil {
		t.Fatalf("a refused push reported success:\n%s", bad)
	}
	if !strings.Contains(bad, "credential-invalid") {
		t.Errorf("the refusal's cause did not reach the reader:\n%s", bad)
	}
	if !strings.Contains(bad, "that credential is not one of ours") {
		t.Errorf("the service's own sentence did not reach the reader:\n%s", bad)
	}

	// --dry-run changes nothing and says what it would send.
	dry := hg(nil, "push", "-e", "pushed", "--dry-run")
	if !strings.Contains(dry, "nothing was sent") {
		t.Errorf("--dry-run does not say it sent nothing:\n%s", dry)
	}
	if after := treeOf(t, spool); after != before {
		t.Error("--dry-run changed the spool")
	}
}

func archiveRunIDs(d *ingestDouble) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []string
	for id := range d.stored {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// treeOf describes a whole directory: every path, its mode, its size and a
// digest of its contents. "It puts it back afterwards" is not "it changes
// nothing", and a fixed-name probe file is how that was learned here before.
func treeOf(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return rerr
		}
		if info.IsDir() {
			lines = append(lines, fmt.Sprintf("d %s/ %04o", rel, info.Mode().Perm()))
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		lines = append(lines, fmt.Sprintf("f %s %04o %d %s",
			rel, info.Mode().Perm(), info.Size(), digestOf(b)[:16]))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
