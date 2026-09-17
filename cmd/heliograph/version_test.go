package main

import (
	"os"
	"strings"
	"testing"
)

// TestBuildVersionPrefersLdflagsThenTheModuleVersion holds the precedence that
// `go install` depends on.
//
// WATCHED FAILING BEFORE IT WAS KEPT, and it did not need planting:
// `go install github.com/heliograph-io/heliograph/cmd/heliograph@v0.4.3` into
// an empty GOMODCACHE produced a binary reporting `heliograph dev`, while the
// identical binary from the release reported `heliograph v0.4.3`.
// heliograph-cloud#269.
//
// The cases are the three real builds. A release build has ldflags and that
// wins outright, because it is the case where the exact answer is known. A
// `go install module@version` build has no ldflags and a real `Main.Version`.
// A plain `go build` has neither, and must say "dev" rather than "(devel)" -
// two words for the same state is how a support conversation goes wrong.
func TestBuildVersionPrefersLdflagsThenTheModuleVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })

	for _, c := range []struct {
		name    string
		ldflags string
		module  string
		want    string
	}{
		{"a release build, ldflags wins", "v1.2.3", "v9.9.9", "v1.2.3"},
		{"go install, the module version is used", "dev", "v0.4.3", "v0.4.3"},
		{"a plain go build says dev, not (devel)", "dev", "(devel)", "dev"},
		{"no build info at all still says dev", "dev", "", "dev"},
		// A checkout build keeps its pseudo-version. Go stamps VCS by default,
		// so `go build` in a git tree reports something like this rather than
		// "(devel)" - measured, not assumed. It names the base version, the
		// commit and whether the tree was dirty, which is strictly more useful
		// than "dev" for the bug report it will end up in.
		{"a checkout build keeps its pseudo-version", "dev", "v0.4.4-0.20260917122714-d3204fe05ae8+dirty", "v0.4.4-0.20260917122714-d3204fe05ae8+dirty"},
	} {
		t.Run(c.name, func(t *testing.T) {
			version = c.ldflags
			got := resolveVersion(c.ldflags, c.module)
			if got != c.want {
				t.Errorf("ldflags %q and module %q gave %q, want %q", c.ldflags, c.module, got, c.want)
			}
		})
	}
}

// TestVersionCommandUsesBuildVersion stops the precedence being correct in a
// helper nothing calls. `heliograph version` is the command the documentation
// tells people to run, so that is the one that has to be right.
func TestVersionCommandUsesBuildVersion(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `fmt.Printf("heliograph %s\n", buildVersion())`) {
		t.Error("the version command does not print buildVersion(), so `go install` builds would report dev again")
	}
}
