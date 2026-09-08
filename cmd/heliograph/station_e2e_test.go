package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dbhq-uk/heliograph/internal/estate"
	"github.com/dbhq-uk/heliograph/internal/transport"
)

// `station add` does three things that have to happen together: create and push
// a branch, check it out into its own directory, and record it as an estate.
// Doing two of them is worse than doing none - a pushed branch with no checkout
// is invisible, and a checkout with no estate cannot be driven.
//
// Against real repositories, because every one of these is a git behaviour and
// a fake would only prove the fake agrees with itself.

func gitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=ci", "GIT_AUTHOR_EMAIL=ci@example.invalid",
		"GIT_COMMITTER_NAME=ci", "GIT_COMMITTER_EMAIL=ci@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v: %s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

// a bare remote and a working clone with one commit on it.
func repoPair(t *testing.T) (clone string) {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	gitAt(t, root, "init", "-q", "--bare", bare)

	clone = filepath.Join(root, "work")
	gitAt(t, root, "clone", "-q", bare, clone)
	if err := os.WriteFile(filepath.Join(clone, "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitAt(t, clone, "add", "-A")
	gitAt(t, clone, "commit", "-qm", "init")
	gitAt(t, clone, "push", "-q", "-u", "origin", "HEAD")
	return clone
}

func TestCreateBranchPushesWithAnUpstream(t *testing.T) {
	clone := repoPair(t)
	g, err := transport.NewGit(clone)
	if err != nil {
		t.Fatal(err)
	}
	before := strings.TrimSpace(gitAt(t, clone, "rev-parse", "--abbrev-ref", "HEAD"))

	if err := g.CreateBranch("station/db-a"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	// On the remote, or the far side has nothing to clone.
	if !g.HasRemoteBranch("station/db-a") {
		t.Error("the branch was created locally and never reached origin")
	}
	// With an upstream, or the station's own first push fails on the far side
	// where nobody can see it.
	up := strings.TrimSpace(gitAt(t, clone, "for-each-ref", "--format=%(upstream:short)", "refs/heads/station/db-a"))
	if up != "origin/station/db-a" {
		t.Errorf("upstream is %q, so the station would push to nowhere", up)
	}
	// And it must NOT have moved the caller, who may be mid-something.
	if after := strings.TrimSpace(gitAt(t, clone, "rev-parse", "--abbrev-ref", "HEAD")); after != before {
		t.Errorf("creating a station moved this checkout from %s to %s", before, after)
	}
}

// A branch origin already has is somebody else's station. Creating it from this
// HEAD would be about to rewrite their history.
func TestCreateBranchRefusesOneTheRemoteAlreadyHas(t *testing.T) {
	clone := repoPair(t)
	g, err := transport.NewGit(clone)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.CreateBranch("station/db-a"); err != nil {
		t.Fatal(err)
	}
	gitAt(t, clone, "branch", "-D", "station/db-a")

	err = g.CreateBranch("station/db-a")
	if err == nil {
		t.Fatal("created a branch origin already had, which could rewrite another station's history")
	}
	if !strings.Contains(err.Error(), "origin already has") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

func TestAddWorktreeGivesTheStationItsOwnCheckout(t *testing.T) {
	clone := repoPair(t)
	g, err := transport.NewGit(clone)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.CreateBranch("station/db-a"); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(filepath.Dir(clone), "work-db-a")
	if err := g.AddWorktree(wt, "station/db-a"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	// The point of the worktree: a transport attached to it reads the station's
	// branch, and the original checkout is untouched.
	sub, err := transport.NewGit(wt)
	if err != nil {
		t.Fatal(err)
	}
	if sub.Branch() != "station/db-a" {
		t.Errorf("the new checkout is on %q, not the station's branch", sub.Branch())
	}
	if g.Branch() == sub.Branch() {
		t.Error("both checkouts are on one branch, which is the accident this exists to prevent")
	}

	// Git refuses one branch in two worktrees. That refusal is wanted here: it
	// is two stations on one branch, caught locally.
	if err := g.AddWorktree(filepath.Join(filepath.Dir(clone), "again"), "station/db-a"); err == nil {
		t.Error("checked the same branch out twice, so two stations could share one channel")
	}
}

func TestAddWorktreeRefusesAnExistingDirectory(t *testing.T) {
	clone := repoPair(t)
	g, err := transport.NewGit(clone)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.CreateBranch("station/db-a"); err != nil {
		t.Fatal(err)
	}
	occupied := filepath.Join(filepath.Dir(clone), "occupied")
	if err := os.MkdirAll(occupied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := g.AddWorktree(occupied, "station/db-a"); err == nil {
		t.Error("wrote a station into a directory that already existed")
	}
}

// The estate is what makes `-e <name>` reach one machine and only that machine.
func TestAStationEstateRecordsItsOwnBranch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clone := repoPair(t)
	g, err := transport.NewGit(clone)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.CreateBranch("station/db-a"); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(filepath.Dir(clone), "work-db-a")
	if err := g.AddWorktree(wt, "station/db-a"); err != nil {
		t.Fatal(err)
	}
	e := estate.Estate{Name: "db-a", Transport: "git", Dir: wt,
		Branch: "station/db-a", Scope: "station/db-a"}
	if err := e.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := estate.Load("db-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Dir != wt {
		t.Errorf("the estate points at %q, not the station's own checkout", got.Dir)
	}
	if got.Scope != "station/db-a" {
		t.Errorf("the estate records scope %q, so routing would fall back to whatever is checked out", got.Scope)
	}
}
