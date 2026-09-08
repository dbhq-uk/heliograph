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
// station with the real binary and asserts that a log came back with the
// step's output in it.
//
// The station lives in this repository now, so the default is the checkout
// this test is running in and nothing skips. HELIOGRAPH_STATION_DIR overrides
// it, for running against an external station checkout, and the resolved
// directory FAILS rather than skips when it is wrong: a silently skipped
// end-to-end test is the same shape of problem as a green suite that checked
// nothing.

func stationDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("HELIOGRAPH_STATION_DIR")
	if dir == "" {
		// This file lives in cmd/heliograph, two levels below the repo root.
		abs, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		dir = abs
	}
	bootstrap := filepath.Join(dir, "station", "bootstrap.sh")
	if _, err := os.Stat(bootstrap); err != nil {
		t.Fatalf("%s does not carry a station (no station/bootstrap.sh): %v", dir, err)
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
	station := stationDir(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}

	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	work := filepath.Join(base, "work")
	cfg := filepath.Join(base, "config")

	// A transport repo built by the station's own bootstrap.sh. Not a fixture we
	// wrote: if the station changes what it lays down, this notices.
	sh(t, base, filepath.Join(station, "station", "bootstrap.sh"), work)
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

// --gaps is the reason the binary is worth installing, so it is proved against
// a real captured log rather than a fixture we wrote. The step sleeps, and the
// gap has to appear attributed to the line before it.
func TestGapsFindsARealStall(t *testing.T) {
	station := stationDir(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}

	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	work := filepath.Join(base, "work")
	cfg := filepath.Join(base, "config")

	sh(t, base, filepath.Join(station, "station", "bootstrap.sh"), work)
	sh(t, base, "git", "init", "-q", "-b", "main", "--bare", origin)
	sh(t, work, "git", "init", "-q", "-b", "main")
	sh(t, work, "git", "remote", "add", "origin", origin)

	step := "#!/usr/bin/env bash\n# heliograph-mode: read-only\n" +
		"echo STARTING-THE-SLOW-THING\nsleep 4\necho DONE\n"
	if err := os.WriteFile(filepath.Join(work, "steps", "slow.sh"), []byte(step), 0o755); err != nil {
		t.Fatal(err)
	}
	sh(t, work, "git", "add", "-A")
	sh(t, work, "git", "commit", "-qm", "init")
	sh(t, work, "git", "push", "-q", "-u", "origin", "main")

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

	hg("init", "gaps", "--dir", work)
	hg("send", "steps/slow.sh")
	sh(t, work, "bash", "./station.sh", "--once", "--interval", "1")

	out := hg("logs", "--last", "--gaps", "--min", "3s")
	if !strings.Contains(out, "STARTING-THE-SLOW-THING") {
		t.Errorf("the gap was not attributed to the line that was running:\n%s", out)
	}
	// And the ordinary intervals must not be reported, or the signal is buried.
	if strings.Contains(out, "| DONE") {
		t.Errorf("a line that did not stall was reported as a gap:\n%s", out)
	}
}
