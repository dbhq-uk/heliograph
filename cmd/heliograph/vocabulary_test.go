package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The positioning this repository used to carry described the estates it
// targets by their regulator rather than by their shape. It was replaced on
// 2026-09-11 with language about the situation - no route in, somebody else
// holds the keys - and this stops it coming back a phrase at a time.
//
// Three files are exempt, and all three for the same reason: their purpose
// is to record the rule, which cannot be done without naming the word they
// record. The spec tables every replacement, the plan that scheduled the
// scrub tables them again, and this test holds the needle it searches for.
var vocabularyExempt = map[string]bool{
	"docs/specs/2026-09-11-three-shapes-and-signalling-names-design.md": true,
	"docs/plans/2026-09-12-s1a-three-shapes-and-the-scrub.md":           true,
	"cmd/heliograph/vocabulary_test.go":                                 true,
}

func TestTheOldPositioningIsGone(t *testing.T) {
	// Built from parts so this file does not match its own search.
	needle := "regul" + "at"

	root := repoRootForVocabulary(t)

	// The set of files to check comes from `git ls-files` rather than
	// filepath.Walk over the working tree. A hand-maintained SkipDir list
	// (".git", "node_modules", and whatever else happened to be untracked
	// in whoever's working tree wrote the list) is the same exemption
	// discipline this test exists to enforce, just moved to the directory
	// level and reactive: it grows one entry at a time, every time someone's
	// local checkout has a gitignored directory the list doesn't yet name -
	// .superpowers today, .serena or a future site/dist-alike tomorrow.
	// `git ls-files` sidesteps the whole category: anything gitignored was
	// never tracked, so it is absent from the list with no entry required,
	// and anything force-added with `git add -f` stays visible because it
	// genuinely is part of the repository now. The three-file exemption map
	// above is the only carve-out this guard grants, and it stays a map of
	// files, not a list of directories.
	tracked, err := trackedFilesForVocabulary(root)
	if err != nil {
		// A guard that passes because it could not look is worse than no
		// guard at all, so a failed git invocation fails the test loudly
		// rather than silently reporting zero hits.
		t.Fatalf("listing tracked files with git: %v", err)
	}
	if len(tracked) == 0 {
		// `git ls-files -z` can exit 0 with empty output (wrong working
		// directory, a repository with nothing staged yet). Without this
		// guard the loop below would range over zero files and the test
		// would pass having checked nothing.
		t.Fatal("git ls-files returned no tracked files: this guard looked at nothing")
	}

	var found []string
	for _, rel := range tracked {
		if vocabularyExempt[rel] {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if readErr != nil {
			continue // unreadable or binary, not our business
		}
		if strings.Contains(strings.ToLower(string(body)), needle) {
			found = append(found, rel)
		}
	}
	if len(found) > 0 {
		t.Errorf("the old positioning survives in %d file(s):\n  %s",
			len(found), strings.Join(found, "\n  "))
	}
}

// trackedFilesForVocabulary returns every file git tracks in root, as
// slash-separated paths relative to root. `git ls-files -z` is used over the
// newline-separated form because a tracked path can itself contain a
// newline; NUL cannot appear in a path on any platform git supports.
func trackedFilesForVocabulary(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		// cmd.Output only captures stdout, so a plain wrap of err reports
		// nothing but "exit status 128". The ExitError carries stderr,
		// which is where git actually says what went wrong.
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	var files []string
	for _, entry := range strings.Split(string(out), "\x00") {
		if entry == "" {
			continue
		}
		files = append(files, filepath.ToSlash(entry))
	}
	return files, nil
}

// repoRootForVocabulary walks up until it finds go.mod, so the test does not
// care which directory it was invoked from.
func repoRootForVocabulary(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory")
		}
		dir = parent
	}
}
