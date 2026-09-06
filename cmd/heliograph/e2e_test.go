package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The test that matters.
//
// Everything in internal/ can pass while the CLI and the station quietly
// disagree about the document they share, because both halves are checked
// against our own idea of the format. This one drives a real, unmodified
// heliograph-skill station with the real binary and asserts that a log came
// back with the step's output in it.
//
// It needs a checkout of the skill repository, named by HELIOGRAPH_SKILL_DIR.
// It SKIPS when the variable is unset and FAILS when it is set but wrong: a
// silently skipped end-to-end test is the same shape of problem as a green
// suite that checked nothing, and this is the only test here that can catch
// the two repositories drifting apart.

func skillDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("HELIOGRAPH_SKILL_DIR")
	if dir == "" {
		t.Skip("HELIOGRAPH_SKILL_DIR is not set: needs a heliograph-skill checkout")
	}
	bootstrap := filepath.Join(dir, "skills", "heliograph", "scripts", "bootstrap.sh")
	if _, err := os.Stat(bootstrap); err != nil {
		t.Fatalf("HELIOGRAPH_SKILL_DIR=%s does not look like a heliograph-skill checkout: %v", dir, err)
	}
	return dir
}

func sh(t *testing.T, dir string, args ...string) string {
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

func TestCLIDrivesAStockStation(t *testing.T) {
	skill := skillDir(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}

	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	work := filepath.Join(base, "work")
	cfg := filepath.Join(base, "config")

	// A transport repo built by the SKILL's own bootstrap.sh. Not a fixture we
	// wrote: if the skill changes what it lays down, this notices.
	sh(t, base, filepath.Join(skill, "skills", "heliograph", "scripts", "bootstrap.sh"), work)
	sh(t, base, "git", "init", "-q", "-b", "main", "--bare", origin)
	sh(t, work, "git", "init", "-q", "-b", "main")
	sh(t, work, "git", "remote", "add", "origin", origin)

	step := "#!/usr/bin/env bash\n# heliograph-mode: read-only\necho THE-MEASUREMENT-CAME-BACK\n"
	if err := os.WriteFile(filepath.Join(work, "steps", "probe.sh"), []byte(step), 0o755); err != nil {
		t.Fatal(err)
	}
	sh(t, work, "git", "add", "-A")
	sh(t, work, "git", "commit", "-qm", "init")
	sh(t, work, "git", "push", "-q", "-u", "origin", "main")

	// Build and run the real binary, not the functions behind it. Argument
	// parsing and printing are where a CLI usually goes wrong.
	bin := filepath.Join(base, "heliograph")
	sh(t, ".", "go", "build", "-o", bin, "github.com/dbhq-uk/heliograph/cmd/heliograph")

	hg := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = base
		cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+cfg)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("heliograph %v: %v\n%s", args, err, out)
		}
		return string(out)
	}

	// init
	if out := hg("init", "e2e", "--dir", work); !strings.Contains(out, "main") {
		t.Errorf("init did not report the branch:\n%s", out)
	}

	// send
	out := hg("send", "steps/probe.sh")
	if !strings.Contains(out, "sent ") {
		t.Errorf("send did not report an id:\n%s", out)
	}

	// The station, run once, exactly as an operator would. Nothing about it is
	// modified for this test, and PUSH is left at its default: a log that is
	// captured but never pushed is the failure this whole loop exists to
	// prevent, so the test must not quietly arrange for it.
	sh(t, work, "bash", "./station.sh", "--once", "--interval", "1")

	// status, read back through the CLI
	if s := hg("status"); !strings.Contains(s, "idle") {
		t.Errorf("status after a completed run:\n%s", s)
	}

	// and the log itself
	logs := hg("logs")
	if !strings.Contains(logs, "probe-") {
		t.Fatalf("no log came back:\n%s", logs)
	}
	body := hg("logs", "--last")
	if !strings.Contains(body, "THE-MEASUREMENT-CAME-BACK") {
		t.Errorf("the log does not contain the step's output:\n%s", body)
	}
	// The property the whole toolkit exists for. If the CLI ever reformats a
	// log on the way out, this is what notices.
	if !strings.Contains(body, " | THE-MEASUREMENT-CAME-BACK") {
		t.Errorf("the captured line lost its timestamp column:\n%s", body)
	}
	if !strings.Contains(body, "RESULT       : OK") {
		t.Errorf("the log has no footer:\n%s", body)
	}
}
