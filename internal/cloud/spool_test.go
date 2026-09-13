package cloud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSpool(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const (
	runningStatus = "state: running\nid: 20260913T090000Z-env\nstep: env\nutc: 20260913T090001Z\n"
	idleStatus    = "state: idle\nid: 20260913T090000Z-env\nstep: env\nexit: 0\n" +
		"utc: 20260913T090012Z\nlog: ops-logs/env-20260913T090000Z.txt\n"
	envLog = "============================================================\n" +
		" STEP: env\n started UTC : 20260913T090000Z\n" +
		"============================================================\n09:00:01 | hello\n"
)

// A run is keyed by the `id:` inside its own status document, and by nothing
// else. The archive derives every column from those bytes, so an id invented
// from a filename would be this side asserting something the station never
// said.
func TestARunIsIdentifiedByTheStatusDocumentAndNotByAFilename(t *testing.T) {
	dir := writeSpool(t, map[string]string{
		"statuses/status-000001.txt":        runningStatus,
		"statuses/status-000002.txt":        idleStatus,
		"ops-logs/env-20260913T090000Z.txt": envLog,
		"status":                            idleStatus,
	})
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	runs, unpaired := s.Runs()
	if len(runs) != 1 {
		t.Fatalf("want one run, got %d: %+v", len(runs), runs)
	}
	if runs[0].RunID != "20260913T090000Z-env" {
		t.Errorf("run id %q, want the status document's own id", runs[0].RunID)
	}
	// BOTH transitions are kept. The station publishes on every one, and the
	// archive upserts and never goes backwards, so re-sending is safe and
	// dropping one is a record that never saw the run start.
	if len(runs[0].Statuses) != 2 {
		t.Errorf("want both status documents for the run, got %d", len(runs[0].Statuses))
	}
	if runs[0].Seq != 2 {
		t.Errorf("seq %d, want the newest status's signed sequence", runs[0].Seq)
	}
	if runs[0].Body == nil {
		t.Fatal("the run has no body, and the status named one that is in the spool")
	}
	if runs[0].Body.Name != "env-20260913T090000Z.txt" {
		t.Errorf("body %q, want the file the status's log: field named", runs[0].Body.Name)
	}
	if len(unpaired) != 0 {
		t.Errorf("a paired log was reported as unpaired: %+v", unpaired)
	}
}

// The bytes are the evidence. A status this side re-encoded would be a
// rendering of it.
func TestAStatusIsKeptVerbatim(t *testing.T) {
	dir := writeSpool(t, map[string]string{"statuses/status-000001.txt": idleStatus})
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Statuses) != 1 {
		t.Fatalf("want one status, got %d", len(s.Statuses))
	}
	if string(s.Statuses[0].Body) != idleStatus {
		t.Errorf("the status body was altered:\n got %q\nwant %q", s.Statuses[0].Body, idleStatus)
	}
	if s.Statuses[0].Seq != 1 {
		t.Errorf("seq %d, want the signed sequence out of the file name", s.Statuses[0].Seq)
	}
	if s.Statuses[0].State != "idle" || s.Statuses[0].Log != "ops-logs/env-20260913T090000Z.txt" {
		t.Errorf("the parsed fields are wrong: %+v", s.Statuses[0])
	}
}

// A body no status names cannot be attributed, and is reported rather than
// uploaded under an invented id. `relay-NNNNNN.txt` is the name the spool gives
// a log it collected without a status naming it in the same drain.
func TestALogNoStatusNamesIsReportedRatherThanAttributed(t *testing.T) {
	dir := writeSpool(t, map[string]string{
		"statuses/status-000002.txt":        idleStatus,
		"ops-logs/env-20260913T090000Z.txt": envLog,
		"ops-logs/relay-000005.txt":         "07:00:00 | an earlier run\n",
	})
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	runs, unpaired := s.Runs()
	if len(runs) != 1 {
		t.Fatalf("want one run, got %d", len(runs))
	}
	if len(unpaired) != 1 || unpaired[0].Name != "relay-000005.txt" {
		t.Fatalf("want the unattributable body reported, got %+v", unpaired)
	}
	if unpaired[0].Seq != 5 {
		t.Errorf("seq %d, want 5 read out of the spool's own naming", unpaired[0].Seq)
	}
}

func TestAnEmptySpoolIsNotAnError(t *testing.T) {
	s, err := Read(t.TempDir())
	if err != nil {
		t.Fatalf("an empty spool is the ordinary state of a new estate: %v", err)
	}
	runs, unpaired := s.Runs()
	if len(runs) != 0 || len(unpaired) != 0 {
		t.Fatalf("an empty spool produced %d run(s) and %d unpaired", len(runs), len(unpaired))
	}
}

func TestASpoolReadsOnlyTheCapturedLogs(t *testing.T) {
	dir := writeSpool(t, map[string]string{
		"ops-logs/good.txt":        "x\n",
		"ops-logs/notes.md":        "not a log\n",
		"ops-logs/partial.txt.tmp": "half a log\n",
		"ops-logs/nested/x.txt":    "not ours\n",
	})
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Logs) != 1 || s.Logs[0].Name != "good.txt" {
		t.Fatalf("want only the one .txt at the top of ops-logs, got %+v", s.Logs)
	}
}

// A run in flight is a snapshot, not evidence. Uploading one would put a
// half-written capture in the archive under a name the finished run will claim.
func TestASpoolSkipsAPartialSnapshot(t *testing.T) {
	dir := writeSpool(t, map[string]string{
		"ops-logs/env-20260913T090000Z.partial.txt": "09:00:01 | still going\n",
		"ops-logs/env-20260912T090000Z.txt":         "finished\n",
	})
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Logs) != 1 || s.Logs[0].Name != "env-20260912T090000Z.txt" {
		t.Fatalf("a partial snapshot was offered as a body: %+v", s.Logs)
	}
}

func TestASpoolDigestsABodyExactlyAsItSitsOnDisk(t *testing.T) {
	dir := writeSpool(t, map[string]string{"ops-logs/a.txt": envLog})
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	// sha256 of envLog, computed independently of the code under test.
	want := sha256Hex(t, filepath.Join(dir, "ops-logs/a.txt"))
	if s.Logs[0].SHA256 != want {
		t.Errorf("sha256 %s, want %s", s.Logs[0].SHA256, want)
	}
	if s.Logs[0].Bytes != int64(len(envLog)) {
		t.Errorf("bytes %d, want %d", s.Logs[0].Bytes, len(envLog))
	}
}

// A status document that does not parse is a fault worth reporting, not a
// document to skip. Skipping it would drop a run silently, and a run this side
// never forwarded is indistinguishable from one that never happened.
func TestAStatusThatDoesNotParseIsReportedRatherThanSkipped(t *testing.T) {
	dir := writeSpool(t, map[string]string{"statuses/status-000001.txt": idleStatus})
	// wire.ParseStatus is forgiving by design, so the unreadable case has to be
	// one it genuinely refuses: a document too long to scan.
	huge := make([]byte, 2*1024*1024)
	for i := range huge {
		huge[i] = 'x'
	}
	if err := os.WriteFile(filepath.Join(dir, "statuses", "status-000002.txt"), huge, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir); err == nil {
		t.Error("an unreadable status document was skipped rather than reported")
	}
}

// GAPS ARE A BOUND, NOT A LIST, and this is the correction the first round trip
// forced.
//
// The relay assigns a sequence to EVERY message it carries: a status, a
// progress snapshot and a finished log each take one. The spool records the
// sequence of a status in its name, and records a log's sequence ONLY when no
// status named the log - because a log the status named is written under the
// station's own filename instead. So an ordinary run leaves statuses at 1 and 3
// with the log at 2, and 2 is missing from the names while being sitting right
// there in ops-logs.
//
// Reporting that as "sequence 2 was never collected" is stating an inference as
// an observation, about the one thing in this product that is supposed to be a
// fact. What IS a fact is arithmetic: if there are more holes than there are
// collected bodies whose sequence the spool did not record, the surplus were
// genuinely never collected.
func TestAHoleExplainedByACollectedBodyIsNotReportedAsAGap(t *testing.T) {
	dir := writeSpool(t, map[string]string{
		// The shape an ordinary run leaves: status 1, log at 2, status 3.
		"statuses/status-000001.txt": statusFor("run-1", "running", "", ""),
		"statuses/status-000003.txt": statusFor("run-1", "idle", "", ""),
		"ops-logs/env-1.txt":         envLog,
	})
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Gaps(); len(got) != 0 {
		t.Fatalf("a hole that a collected body accounts for was reported as a gap: %v", got)
	}
}

// And when the holes outnumber what could explain them, the surplus is real and
// is stated as a surplus.
func TestGapsReportsOnlyTheHolesNothingCollectedCanAccountFor(t *testing.T) {
	dir := writeSpool(t, map[string]string{
		"statuses/status-000001.txt": statusFor("run-1", "idle", "", ""),
		"statuses/status-000009.txt": statusFor("run-2", "idle", "", ""),
		"ops-logs/env-1.txt":         envLog,
	})
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Gaps()
	if len(got) != 1 {
		t.Fatalf("want one sentence, got %v", got)
	}
	// Seven holes (2 to 8), one collected body whose sequence the spool did not
	// record, so at least six messages never arrived here.
	if !strings.Contains(got[0], "at least 6") {
		t.Errorf("the sentence does not state the bound it can prove: %q", got[0])
	}
	if !strings.Contains(got[0], "never collected by this control node") {
		t.Errorf("the sentence does not say whose absence this is: %q", got[0])
	}
}

func TestGapsSaysNothingWhenTheSequenceIsWhole(t *testing.T) {
	dir := writeSpool(t, map[string]string{
		"statuses/status-000001.txt": statusFor("run-1", "idle", "", ""),
		"statuses/status-000002.txt": statusFor("run-1", "idle", "", ""),
	})
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Gaps(); len(got) != 0 {
		t.Fatalf("a whole sequence reported a gap: %v", got)
	}
}
