package main

import (
	"os"
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
	var found []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if info.IsDir() {
			// .git and node_modules are never the repository's own content.
			// .superpowers and site/dist are gitignored too - local agent
			// scratch space and generated site output - so a checkout that
			// has run either still passes: this guard polices what the
			// project ships, not a working tree's local byproducts.
			switch {
			case info.Name() == ".git", info.Name() == "node_modules", info.Name() == ".superpowers":
				return filepath.SkipDir
			case filepath.ToSlash(rel) == "site/dist":
				return filepath.SkipDir
			}
			return nil
		}
		if vocabularyExempt[filepath.ToSlash(rel)] {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // unreadable or binary, not our business
		}
		if strings.Contains(strings.ToLower(string(body)), needle) {
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
	if len(found) > 0 {
		t.Errorf("the old positioning survives in %d file(s):\n  %s",
			len(found), strings.Join(found, "\n  "))
	}
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
