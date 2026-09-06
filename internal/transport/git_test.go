package transport

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dbhq-uk/heliograph/internal/wire"
)

// No mocking. What breaks here is git's behaviour under two writers on one
// branch, and a mock of git would assert only that we wrote the mock we
// expected. These tests use real repositories and a real bare origin.

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=ci", "GIT_AUTHOR_EMAIL=ci@example.invalid",
		"GIT_COMMITTER_NAME=ci", "GIT_COMMITTER_EMAIL=ci@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return string(out)
}

// newRepo builds what the CLI actually meets: a working clone with a bare
// origin, a station/ directory, and an ops-logs/ directory.
func newRepo(t *testing.T) (work, origin string) {
	t.Helper()
	base := t.TempDir()
	origin = filepath.Join(base, "origin.git")
	work = filepath.Join(base, "work")

	run(t, base, "git", "init", "-q", "-b", "main", "--bare", origin)
	if err := os.MkdirAll(filepath.Join(work, "station"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(work, "ops-logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "station", "request"), []byte("id:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, work, "git", "init", "-q", "-b", "main")
	run(t, work, "git", "remote", "add", "origin", origin)
	run(t, work, "git", "add", "-A")
	run(t, work, "git", "commit", "-qm", "init")
	run(t, work, "git", "push", "-q", "-u", "origin", "main")
	return work, origin
}

// pushFromElsewhere is the station: a second writer pushing to the same branch
// while the control side is composing its next request.
func pushFromElsewhere(t *testing.T, origin, path, body string) {
	t.Helper()
	other := t.TempDir()
	run(t, other, "git", "clone", "-q", origin, "c")
	c := filepath.Join(other, "c")
	full := filepath.Join(c, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, c, "git", "add", "-A")
	run(t, c, "git", "commit", "-qm", "from the station")
	run(t, c, "git", "push", "-q")
}

// A commit that never reached the remote is invisible to the station, and the
// CLI would have reported success. That is the whole failure this transport
// exists to avoid, so it is asserted against the ORIGIN and not the clone.
func TestPutRequestReachesTheRemote(t *testing.T) {
	work, origin := newRepo(t)
	g, err := NewGit(work)
	if err != nil {
		t.Fatal(err)
	}
	req := wire.Request{Version: wire.Version, ID: "run-1", Step: "env"}
	if err := g.PutRequest(req); err != nil {
		t.Fatal(err)
	}
	out := run(t, origin, "git", "show", "main:station/request")
	if !strings.Contains(out, "id: run-1") {
		t.Errorf("the request did not reach origin:\n%s", out)
	}
}

// The station pushes far more often than the control does: a status commit on
// every transition, a progress snapshot every 60 seconds, and the log itself.
// So the remote WILL have moved while the next request was being written, and
// a plain push is rejected. That is two writers on one branch working as
// intended, and the CLI has to handle it rather than report it.
func TestPutRequestSucceedsWhenTheRemoteMoved(t *testing.T) {
	work, origin := newRepo(t)
	pushFromElsewhere(t, origin, "station/status", "state: running\nid: earlier\n")

	g, err := NewGit(work)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.PutRequest(wire.Request{Version: wire.Version, ID: "run-2"}); err != nil {
		t.Fatalf("push after the remote moved: %v", err)
	}

	// And the station's commit must survive. Rebasing rather than forcing is
	// the difference between publishing a request and destroying evidence.
	out := run(t, origin, "git", "show", "main:station/status")
	if !strings.Contains(out, "earlier") {
		t.Errorf("the station's status was lost:\n%s", out)
	}
}

func TestFetchStatusReadsWhatTheStationPublished(t *testing.T) {
	work, origin := newRepo(t)
	pushFromElsewhere(t, origin, "station/status",
		"state:    running\nid:       run-9\nstep:     net-probe\nlast:     11:31:29 | probe x:443\n")

	g, err := NewGit(work)
	if err != nil {
		t.Fatal(err)
	}
	s, err := g.FetchStatus()
	if err != nil {
		t.Fatal(err)
	}
	if s.State != "running" || s.ID != "run-9" {
		t.Errorf("got %+v", s)
	}
	if !strings.HasSuffix(s.Last, "probe x:443") {
		t.Errorf("last was truncated: %q", s.Last)
	}
}

// A station that has never run has published no status. That is a state worth
// reporting plainly, not an error to raise.
func TestFetchStatusOfAStationThatHasNeverRun(t *testing.T) {
	work, _ := newRepo(t)
	g, err := NewGit(work)
	if err != nil {
		t.Fatal(err)
	}
	s, err := g.FetchStatus()
	if err != nil {
		t.Fatalf("a missing status must not be an error: %v", err)
	}
	if s.State != "" {
		t.Errorf("got %+v", s)
	}
}

func TestListLogsIsNewestFirst(t *testing.T) {
	work, origin := newRepo(t)
	for _, n := range []string{
		"ops-logs/env-20260906T101500Z.txt",
		"ops-logs/net-20260906T104500Z.txt",
		"ops-logs/env-20260906T090000Z.txt",
	} {
		pushFromElsewhere(t, origin, n, "10:00:00 | a line\n")
	}
	g, err := NewGit(work)
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.ListLogs()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d logs: %v", len(got), got)
	}
	// Newest first, because the one you want is almost always the last run.
	if !strings.Contains(got[0], "104500Z") {
		t.Errorf("not newest first: %v", got)
	}
}

func TestReadLogRefusesAPathOutsideTheLogDirectory(t *testing.T) {
	work, _ := newRepo(t)
	g, err := NewGit(work)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"../../.ssh/id_ed25519", "/etc/passwd", "ops-logs/../../secret"} {
		if _, err := g.ReadLog(bad); err == nil {
			t.Errorf("read a path outside ops-logs: %q", bad)
		}
	}
}

// Check must fail rather than hang. A control node with a wrong remote is
// common, and a hang there is indistinguishable from a slow network.
func TestCheckFailsOnAnUnreachableRemote(t *testing.T) {
	work, origin := newRepo(t)
	if err := os.RemoveAll(origin); err != nil {
		t.Fatal(err)
	}
	g, err := NewGit(work)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Check(); err == nil {
		t.Error("Check passed against a deleted origin")
	}
}

func TestCheckPassesOnAWorkingRemote(t *testing.T) {
	work, _ := newRepo(t)
	g, err := NewGit(work)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Check(); err != nil {
		t.Errorf("Check failed on a working remote: %v", err)
	}
}

func TestNewGitRefusesADirectoryThatIsNotARepo(t *testing.T) {
	if _, err := NewGit(t.TempDir()); err == nil {
		t.Error("accepted a directory with no git repository in it")
	}
}

// Describe reports the mechanism, never the value. This is asserted because a
// helpful print of "the token I am using" is the kind of change that looks
// like an improvement in review.
func TestDescribeNeverPrintsACredential(t *testing.T) {
	work, _ := newRepo(t)
	t.Setenv("GIT_TOKEN", "glpat-SUPERSECRETTOKENVALUE")
	g, err := NewGit(work)
	if err != nil {
		t.Fatal(err)
	}
	if d := g.Describe(); strings.Contains(d, "SUPERSECRETTOKENVALUE") {
		t.Errorf("Describe leaked the credential: %q", d)
	}
}

// A control machine where git has never been configured is entirely ordinary:
// a fresh laptop, a container, a CI runner. Git refuses to commit there with
// "Author identity unknown", which reads like a fault in this tool rather than
// a missing setting.
//
// This is the test that was missing. It passed everywhere it was run by hand,
// because the machine running it had an identity, and failed the moment CI
// touched it.
func TestPutRequestWorksWhereGitHasNoIdentity(t *testing.T) {
	work, origin := newRepo(t)

	// No global config, no user config, no environment identity.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")

	g, err := NewGit(work)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.PutRequest(wire.Request{Version: wire.Version, ID: "no-identity"}); err != nil {
		t.Fatalf("a machine with no git identity must still be able to send: %v", err)
	}
	out := run(t, origin, "git", "show", "main:station/request")
	if !strings.Contains(out, "id: no-identity") {
		t.Errorf("the request did not reach origin:\n%s", out)
	}
}
