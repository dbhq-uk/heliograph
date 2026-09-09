package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// buildSite builds the real content into a temporary directory once per test.
// The tests assert on what ships, not on helpers, because the defects they
// guard against were all visible only in the built output.
func buildSite(t *testing.T) string {
	t.Helper()
	out := t.TempDir()
	if _, err := build("../../site/content", out); err != nil {
		t.Fatalf("build: %v", err)
	}
	return out
}

func htmlPages(t *testing.T, out string) map[string]string {
	t.Helper()
	pages := map[string]string{}
	ents, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".html") {
			b, err := os.ReadFile(filepath.Join(out, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			pages[e.Name()] = string(b)
		}
	}
	return pages
}

// Every page used to preload two font files that did not exist: the fonts
// had been renamed and the head had not. Two 404s on every page view, and
// nothing failed.
func TestEveryReferencedAssetExists(t *testing.T) {
	out := buildSite(t)
	re := regexp.MustCompile(`(?:href|src|content)="(?:https://heliograph\.dbhq\.uk)?(/assets/[^"]+)"`)
	for name, h := range htmlPages(t, out) {
		for _, m := range re.FindAllStringSubmatch(h, -1) {
			if _, err := os.Stat(filepath.Join(out, m[1])); err != nil {
				t.Errorf("%s references %s, which is not in the build", name, m[1])
			}
		}
	}
}

// A sitemap without lastmod tells Google nothing about what changed. The
// dbhq.uk audit of 2026-08-09 traced a page that was never crawled to
// exactly this.
func TestSitemapCarriesALastmodForEveryURL(t *testing.T) {
	out := buildSite(t)
	b, err := os.ReadFile(filepath.Join(out, "sitemap.xml"))
	if err != nil {
		t.Fatal(err)
	}
	urls := strings.Count(string(b), "<url>")
	dated := regexp.MustCompile(`<lastmod>\d{4}-\d{2}-\d{2}</lastmod>`).FindAllString(string(b), -1)
	if urls == 0 || len(dated) != urls {
		t.Errorf("%d urls, %d with a lastmod:\n%s", urls, len(dated), b)
	}
}

// A shallow checkout reports the deploy date for every page, which reads as
// a site where everything changed today. That is worse than no date, so the
// build refuses it and says what to do.
func TestShallowCloneIsRefusedWithARemedy(t *testing.T) {
	origin := gitRepo(t, "2024-03-04T05:06:07Z")
	shallow := t.TempDir()
	run(t, shallow, "git", "clone", "--quiet", "--depth", "1", "file://"+origin, ".")
	_, err := lastModified(shallow, "page.md")
	if err == nil || !strings.Contains(err.Error(), "fetch-depth") {
		t.Errorf("a shallow clone was accepted, or the error does not say the remedy: %v", err)
	}
}

func TestLastModifiedIsTheLastCommitThatTouchedThePage(t *testing.T) {
	repo := gitRepo(t, "2024-03-04T05:06:07Z")
	got, err := lastModified(repo, "page.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2024-03-04" {
		t.Errorf("got %q, want 2024-03-04", got)
	}
}

// gitRepo makes a repository with one committed page.md at the given date.
func gitRepo(t *testing.T, date string) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init", "--quiet")
	run(t, dir, "git", "config", "user.email", "t@example.com")
	run(t, dir, "git", "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "page.md"), []byte("# p\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", "page.md")
	cmd := exec.Command("git", "commit", "--quiet", "-m", "p")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, b)
	}
	return dir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, b)
	}
}

// The description is the one sentence a search result shows. The first
// paragraph of a page was doing that job: seven pages ran past 200
// characters, two were under 40, and /windows opened with "Two different
// questions get confused here".
func TestDescriptionsAndTitlesFitASearchResult(t *testing.T) {
	out := buildSite(t)
	desc := regexp.MustCompile(`<meta name="description" content="([^"]*)"`)
	title := regexp.MustCompile(`<title>([^<]*)</title>`)
	for name, h := range htmlPages(t, out) {
		if name == "404.html" {
			continue
		}
		d := desc.FindStringSubmatch(h)
		if d == nil {
			t.Errorf("%s has no description", name)
			continue
		}
		if n := len(unescape(d[1])); n < 70 || n > 160 {
			t.Errorf("%s: description is %d characters, want 70 to 160: %q", name, n, d[1])
		}
		if m := title.FindStringSubmatch(h); m == nil || len(unescape(m[1])) > 70 {
			t.Errorf("%s: title missing or over 70 characters: %v", name, m)
		}
	}
}

func unescape(s string) string {
	return strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&lt;", "<", "&gt;", ">").Replace(s)
}

// The dbhq.uk privacy policy promises analytics loads only after consent.
// This asserts the order the promise depends on: consent denied first, the
// config call and the tag only after, and never on a hostname that is not
// production.
func TestAnalyticsIsDeniedUntilConsentAndOnlyInProduction(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		def := strings.Index(h, `gtag("consent", "default"`)
		cfg := strings.Index(h, `gtag("config"`)
		if def < 0 || cfg < 0 || def > cfg {
			t.Errorf("%s: consent default at %d, config at %d", name, def, cfg)
		}
		if !strings.Contains(h, `analytics_storage: "denied"`) {
			t.Errorf("%s: analytics_storage is not denied by default", name)
		}
		if !strings.Contains(h, `location.hostname === "heliograph.dbhq.uk"`) {
			t.Errorf("%s: the tag is not gated on the production hostname", name)
		}
		if strings.Contains(h, `<script async src="https://www.googletagmanager.com`) ||
			strings.Contains(h, `<script src="https://www.googletagmanager.com`) {
			t.Errorf("%s: the tag is loaded statically, before any consent", name)
		}
		if !strings.Contains(h, `G-3H3NFGSX85`) {
			t.Errorf("%s: not the dbhq.uk measurement id", name)
		}
	}
}

func TestStructuredDataIsValidJSONOfTheRightType(t *testing.T) {
	out := buildSite(t)
	re := regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	for name, h := range htmlPages(t, out) {
		if name == "404.html" {
			continue
		}
		blocks := re.FindAllStringSubmatch(h, -1)
		if len(blocks) == 0 {
			t.Errorf("%s: no JSON-LD", name)
			continue
		}
		var types []string
		for _, b := range blocks {
			var v map[string]any
			if err := json.Unmarshal([]byte(b[1]), &v); err != nil {
				t.Errorf("%s: JSON-LD does not parse: %v\n%s", name, err, b[1])
				continue
			}
			types = append(types, v["@type"].(string))
		}
		want := "TechArticle"
		if name == "index.html" {
			want = "SoftwareApplication"
		}
		if !contains(types, want) {
			t.Errorf("%s: types %v, want %s", name, types, want)
		}
		if name != "index.html" && !contains(types, "BreadcrumbList") {
			t.Errorf("%s: no BreadcrumbList", name)
		}
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// GitHub Pages serves 404.html for a missing path. Without one, a mistyped
// URL gets GitHub's page, with no way back into the docs.
func TestNotFoundPageIsNoindexAndCarriesTheSidebar(t *testing.T) {
	out := buildSite(t)
	b, err := os.ReadFile(filepath.Join(out, "404.html"))
	if err != nil {
		t.Fatalf("no 404.html: %v", err)
	}
	h := string(b)
	if !strings.Contains(h, `<meta name="robots" content="noindex">`) {
		t.Error("404.html is indexable")
	}
	if !strings.Contains(h, `class="side-nav"`) {
		t.Error("404.html has no sidebar")
	}
	if strings.Contains(h, `/404.md`) {
		t.Error("404.html announces a markdown mirror that does not exist")
	}
	sm, _ := os.ReadFile(filepath.Join(out, "sitemap.xml"))
	if strings.Contains(string(sm), "/404") {
		t.Error("the 404 page is in the sitemap")
	}
}

func TestSharedLinksCarryAnImage(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		if !strings.Contains(h, `<meta property="og:image" content="https://heliograph.dbhq.uk/assets/og.png">`) {
			t.Errorf("%s: no og:image", name)
		}
		if !strings.Contains(h, `<meta name="twitter:card" content="summary_large_image">`) {
			t.Errorf("%s: twitter:card is not summary_large_image", name)
		}
	}
}
