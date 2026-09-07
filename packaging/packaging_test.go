package packaging

// Version drift between four files is the failure this prevents, and it is a
// quiet one: npm serves a wrapper that downloads a release tag which does not
// exist, and the error a user sees is a 404 from GitHub with no hint that two
// numbers disagree.
//
// The tag is the source of truth. Everything else has to match it.

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func read(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	return m
}

// latestTag is what the release actually published. On a shallow CI checkout
// there may be no tags, and that is a skip rather than a failure: the check
// needs a tag to compare against and cannot invent one.
func latestTag(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "describe", "--tags", "--abbrev=0").Output()
	if err != nil {
		t.Skip("no tags in this checkout, so there is nothing to compare against")
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
}

func TestPackagedVersionsMatchTheLatestTag(t *testing.T) {
	want := latestTag(t)
	for _, c := range []struct{ path, key string }{
		{"npm/package.json", "version"},
		{"mcpb/manifest.json", "version"},
		{"../server.json", "version"},
	} {
		got, _ := read(t, c.path)[c.key].(string)
		if got != want {
			t.Errorf("%s says version %q, but the latest tag is v%s.\n"+
				"npm would serve a wrapper that downloads a release tag which may not exist, "+
				"and the user sees a 404 with no hint that two numbers disagree.", c.path, got, want)
		}
	}
}

// The registry proves ownership of an npm package by matching a marker in the
// published README against the name in server.json. If they drift, publishing
// is rejected with a message about ownership rather than about a typo.
func TestRegistryOwnershipMarkerMatches(t *testing.T) {
	name, _ := read(t, "../server.json")["name"].(string)
	if name == "" {
		t.Fatal("server.json has no name")
	}
	b, err := os.ReadFile("npm/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "mcp-name: "+name) {
		t.Errorf("npm/README.md does not carry `mcp-name: %s`, so the registry cannot "+
			"verify ownership of the package", name)
	}
	pkgName, _ := read(t, "npm/package.json")["mcpName"].(string)
	if pkgName != name {
		t.Errorf("package.json mcpName is %q, server.json name is %q", pkgName, name)
	}
}

// The npm package must point at the identifier server.json advertises, or a
// client following the registry installs something else entirely.
func TestServerJSONPointsAtThePublishedPackage(t *testing.T) {
	srv := read(t, "../server.json")
	pkgs, _ := srv["packages"].([]any)
	if len(pkgs) == 0 {
		t.Fatal("server.json lists no packages, so no client can install it")
	}
	p, _ := pkgs[0].(map[string]any)
	want, _ := read(t, "npm/package.json")["name"].(string)
	if got, _ := p["identifier"].(string); got != want {
		t.Errorf("server.json installs %q but the package is called %q", got, want)
	}
	tr, _ := p["transport"].(map[string]any)
	if got, _ := tr["type"].(string); got != "stdio" {
		t.Errorf("transport type is %q; heliograph mcp speaks stdio", got)
	}
}

// Every tool the bundle advertises has to exist. A manifest listing a tool the
// binary does not have is a promise a client makes on our behalf.
func TestManifestToolsMatchTheServer(t *testing.T) {
	m := read(t, "mcpb/manifest.json")
	tools, _ := m["tools"].([]any)
	if len(tools) == 0 {
		t.Fatal("the manifest advertises no tools")
	}
	seen := map[string]bool{}
	for _, x := range tools {
		tm, _ := x.(map[string]any)
		n, _ := tm["name"].(string)
		if !strings.HasPrefix(n, "heliograph_") {
			t.Errorf("tool %q is not one of ours", n)
		}
		if d, _ := tm["description"].(string); len(d) < 15 {
			t.Errorf("tool %q has no usable description", n)
		}
		seen[n] = true
	}
	// The seven the MCP server actually registers. Named rather than counted,
	// because a count is satisfied by the wrong seven.
	for _, n := range []string{
		"heliograph_estates", "heliograph_send", "heliograph_status",
		"heliograph_logs", "heliograph_read_log", "heliograph_gaps", "heliograph_doctor",
	} {
		if !seen[n] {
			t.Errorf("the manifest does not advertise %s, which the server provides", n)
		}
	}
	if len(seen) != 7 {
		t.Errorf("the manifest advertises %d tools; the server provides 7", len(seen))
	}
}
