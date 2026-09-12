package packaging

// A reproducible build has exactly one toolchain, and this is what stops there
// being two.
//
// `go.mod` said `go 1.27.1` and every workflow said `go-version: '1.27'` with
// `check-latest: true`, which resolves to the newest 1.27.x that exists on the
// day the job runs. The two agreed for as long as 1.27.1 was the newest patch
// and would have stopped agreeing, silently, the morning 1.27.2 shipped - at
// which point the released binary would no longer be the binary anybody could
// reproduce from the tag, and nothing would have said so. A Go patch release
// changes the compiler and the linker, so it changes the bytes.
//
// The pin is `go.mod`'s `go` directive, in full major.minor.patch. Everything
// that builds this repository has to name that exact version:
//
//	.github/workflows/*.yml   the runners
//	Dockerfile                the control-side image
//	station/bash/docker/...   the image that carries heliograph-seal, whose
//	                          checksum a station verifies against SHA256SUMS
//
// packaging/reproduce.sh reads the same line and exports GOTOOLCHAIN from it,
// so a stranger with any Go 1.21 or newer gets the pinned toolchain fetched for
// them rather than a mismatch they have to diagnose.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// pinnedGo is the `go` directive in go.mod, which is the single source of the
// version everything else must match.
func pinnedGo(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatalf("cannot read go.mod: %v", err)
	}
	m := regexp.MustCompile(`(?m)^go (\d+\.\d+\.\d+)$`).FindStringSubmatch(string(b))
	if m == nil {
		t.Fatalf("go.mod's `go` directive is not a full major.minor.patch version.\n" +
			"A two-part version is not a pin: it admits every patch release, and a\n" +
			"patch release of Go changes the bytes the compiler emits.")
	}
	return m[1]
}

// TestEveryBuildUsesThePinnedToolchain reads the workflows rather than trusting
// them. A `go-version` of '1.27' passes CI perfectly and quietly builds with
// whatever 1.27.x is current.
func TestEveryBuildUsesThePinnedToolchain(t *testing.T) {
	want := pinnedGo(t)
	files, err := filepath.Glob("../.github/workflows/*.yml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no workflows found, so this test checked nothing: %v", err)
	}
	// `go-version-file: go.mod` is the other correct answer: it reads the same
	// line this test reads, so it cannot drift from it.
	pin := regexp.MustCompile(`(?m)^\s*go-version:\s*'?"?([0-9][^'"\s]*)'?"?\s*$`)
	seen := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range pin.FindAllStringSubmatch(string(b), -1) {
			seen++
			if m[1] != want {
				t.Errorf("%s pins go-version %q; go.mod pins %q.\n"+
					"The released binary would be built by a toolchain nobody can name from the source.",
					filepath.Base(f), m[1], want)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no go-version line was found in any workflow, so this test checked nothing")
	}
	t.Logf("%d go-version pins checked against %s", seen, want)
}

// TestEveryDockerfileUsesThePinnedToolchain covers the two images that compile
// Go. The station image is the sharper of the two: it carries heliograph-seal,
// and a station verifies that binary against RELAY_SEAL_SHA256 taken from the
// release. A floating toolchain there produces a seal that is correct, works,
// and does not match the published checksum.
func TestEveryDockerfileUsesThePinnedToolchain(t *testing.T) {
	want := pinnedGo(t)
	from := regexp.MustCompile(`(?m)^FROM[^\n]*golang:([^\s-]+)`)
	seen := 0
	for _, f := range []string{"../Dockerfile", "../station/bash/docker/Dockerfile"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("cannot read %s: %v", f, err)
		}
		for _, m := range from.FindAllStringSubmatch(string(b), -1) {
			seen++
			if m[1] != want {
				t.Errorf("%s builds on golang:%s; go.mod pins %s", f, m[1], want)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no golang base image was found, so this test checked nothing")
	}
	t.Logf("%d golang base images checked against %s", seen, want)
}

// TestTheReleaseBuildsThroughTheScriptEverybodyElseRuns is the one that keeps
// the published command honest.
//
// The build loop used to live inline in release.yml. A stranger following the
// documentation ran something written separately, and the two could differ by a
// flag - which is the whole property, since a single missing -trimpath makes
// every hash disagree with no error anywhere. One script, two callers.
func TestTheReleaseBuildsThroughTheScriptEverybodyElseRuns(t *testing.T) {
	b, err := os.ReadFile("../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, "packaging/reproduce.sh") {
		t.Error("release.yml does not build through packaging/reproduce.sh, so the " +
			"documented command and the released artefact are two different builds")
	}
	// The flags that make it reproducible belong in the script, not beside it.
	// A `go build` in the workflow is the drift starting again.
	if regexp.MustCompile(`(?m)^\s*(GOOS=\S+\s+)?go build `).MatchString(body) {
		t.Error("release.yml calls `go build` directly. Every release artefact has to " +
			"come out of packaging/reproduce.sh or the published hashes are not the ones " +
			"the documented command produces")
	}
}
