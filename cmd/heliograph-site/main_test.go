package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dbhq-uk/heliograph/internal/site"
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

// The nav and the content directory must agree. A slug in order with no
// markdown behind it renders an empty page, and a markdown file no slug names
// is never published at all.
func TestFlareReplacesIntercom(t *testing.T) {
	for _, slug := range order {
		if slug == "intercom" {
			t.Error("the nav still lists intercom; the page is /flare now")
		}
	}
	var found bool
	for _, slug := range order {
		if slug == "flare" {
			found = true
		}
	}
	if !found {
		t.Fatal("the nav does not list flare")
	}
	if _, err := os.Stat(filepath.Join("..", "..", "site", "content", "flare.md")); err != nil {
		t.Fatalf("site/content/flare.md is missing: %v", err)
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

// The sitemap is the list Google works from. Every page in the build is in
// it exactly once, and nothing that is not a page is. The markdown mirrors
// are deliberately absent: they are announced as alternates from each page,
// and listing them would ask Google to index every page twice.
func TestSitemapListsEveryPageOnceAndNothingElse(t *testing.T) {
	out := buildSite(t)
	sm, err := os.ReadFile(filepath.Join(out, "sitemap.xml"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, m := range regexp.MustCompile(`<loc>([^<]+)</loc>`).FindAllStringSubmatch(string(sm), -1) {
		seen[m[1]]++
	}
	for name := range htmlPages(t, out) {
		if name == "404.html" {
			continue
		}
		want := baseURL + "/" + strings.TrimSuffix(name, ".html")
		if name == "index.html" {
			want = baseURL + "/"
		}
		if seen[want] != 1 {
			t.Errorf("%s is in the sitemap %d times, want once", want, seen[want])
		}
		delete(seen, want)
	}
	for extra := range seen {
		t.Errorf("the sitemap lists %s, which is not a page", extra)
	}
}

// A link to a page that does not exist is the defect a content edit
// introduces most easily and the one a crawler scores hardest. Anchors are
// checked too: a heading rename silently breaks every link to it.
func TestInternalLinksResolve(t *testing.T) {
	out := buildSite(t)
	re := regexp.MustCompile(`href="(/[^"#]*)(#[^"]*)?"`)
	for name, h := range htmlPages(t, out) {
		for _, m := range re.FindAllStringSubmatch(h, -1) {
			path, frag := m[1], m[2]
			target := path
			if target == "/" {
				target = "/index"
			}
			file := filepath.Join(out, target)
			if _, err := os.Stat(file); err != nil {
				if _, err := os.Stat(file + ".html"); err != nil {
					t.Errorf("%s links to %s, which is not in the build", name, path)
					continue
				}
				file += ".html"
			}
			if frag == "" || !strings.HasSuffix(file, ".html") {
				continue
			}
			b, _ := os.ReadFile(file)
			if !strings.Contains(string(b), `id="`+frag[1:]+`"`) {
				t.Errorf("%s links to %s%s, and that anchor is not on the page", name, path, frag)
			}
		}
	}
}

// One H1 per page. The home page had two: the hero's, and the source's
// "# heliograph" rendered underneath it.
func TestOneH1PerPage(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		if n := strings.Count(h, "<h1"); n != 1 {
			t.Errorf("%s has %d h1 elements", name, n)
		}
	}
}

// The site's own links to the repository carry the GitHub mark: the footer
// on every page, and the header and hero on the home page. A word on its own
// asks the reader to parse it; the mark is recognised before it is read.
//
// Links inside a page's prose are left alone. A mark mid-sentence is noise,
// and the sentence already says where it goes.
func TestTheChromeLinksToGitHubCarryTheMark(t *testing.T) {
	out := buildSite(t)
	re := regexp.MustCompile(`(?s)<a[^>]*href="https://github\.com/dbhq-uk/heliograph"[^>]*>(.*?)</a>`)
	region := func(h, open, close string) string {
		i := strings.Index(h, open)
		if i < 0 {
			return ""
		}
		j := strings.Index(h[i:], close)
		if j < 0 {
			return ""
		}
		return h[i : i+j]
	}
	for name, h := range htmlPages(t, out) {
		regions := map[string]string{"footer": region(h, "<footer>", "</footer>")}
		if name == "index.html" {
			regions["header"] = region(h, `<header class="site-header">`, "</header>")
			regions["hero"] = region(h, `<div class="cta">`, "</div>")
		}
		for where, frag := range regions {
			links := re.FindAllStringSubmatch(frag, -1)
			if len(links) == 0 {
				t.Errorf("%s: the %s does not link to the repository", name, where)
			}
			for _, l := range links {
				if !strings.Contains(l[1], `class="gh"`) {
					t.Errorf("%s: the %s link has no GitHub mark: %s", name, where, l[1])
				}
			}
		}
	}
}

// This site is a list of commands to run somewhere else. Selecting one by
// hand out of a <pre> is where a stray character enters a step.
func TestEveryCodeBlockHasACopyButton(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		pres := strings.Count(h, "<pre>")
		if pres == 0 {
			continue
		}
		if got := strings.Count(h, `<button class="copy"`); got != pres {
			t.Errorf("%s: %d code blocks, %d copy buttons", name, pres, got)
		}
		if !strings.Contains(h, `<div class="code">`) {
			t.Errorf("%s: code blocks are not wrapped, so the button has nothing to sit in", name)
		}
	}
}

// The markdown mirror was announced in a <link> and named once in the
// footer. An agent finds it there; a person driving one never scrolls that
// far. It belongs at the top of the page, next to the title.
func TestDocsPagesOfferTheMarkdownMirrorAtTheTop(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		if name == "index.html" || name == "404.html" {
			continue
		}
		slug := strings.TrimSuffix(name, ".html")
		head := h[:strings.Index(h, "<h1")]
		if !strings.Contains(head, `data-copy-markdown="/`+slug+`.md"`) {
			t.Errorf("%s has no copy-as-markdown control above its title", name)
		}
		if !strings.Contains(head, `href="/`+slug+`.md"`) {
			t.Errorf("%s has no view-as-markdown link above its title", name)
		}
	}
}

// The JSON-LD has said there is a breadcrumb since the SEO work. Nothing on
// the page did. A crawler was being told about navigation the reader could
// not see, which is the sort of mismatch that is worth nothing at best.
func TestDocsPagesShowTheBreadcrumbTheyClaim(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		if name == "index.html" || name == "404.html" {
			continue
		}
		if !strings.Contains(h, `<nav class="crumbs" aria-label="Breadcrumb">`) {
			t.Errorf("%s claims a BreadcrumbList in JSON-LD and shows no breadcrumb", name)
			continue
		}
		// The crumb is the navigation's label, not the H1. /compared's H1
		// begins with the product's name, so the H1 version read
		// "heliograph / heliograph compared with AWS SSM Run Command and
		// Azure Run Command" - the site's name twice, and a crumb longer
		// than the title it sits above.
		crumb := regexp.MustCompile(`<span class="here">([^<]*)</span>`).FindStringSubmatch(h)
		want := labels[strings.TrimSuffix(name, ".html")]
		if crumb == nil || crumb[1] != want {
			t.Errorf("%s: crumb is %v, want the nav label %q", name, crumb, want)
		}
		// And the JSON-LD says what the reader sees.
		if !strings.Contains(h, `"name":"`+want+`"`) {
			t.Errorf("%s: the BreadcrumbList does not name %q", name, want)
		}
	}
}

// Google reads a favicon for the search result, and iOS wants a PNG for the
// home screen. An SVG alone left both to guess.
func TestTheSiteHasAFaviconEverythingCanRead(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		for _, want := range []string{
			`<link rel="icon" href="/assets/favicon.ico" sizes="48x48">`,
			`<link rel="icon" href="/assets/favicon.svg" type="image/svg+xml">`,
			`<link rel="apple-touch-icon" href="/assets/apple-touch-icon.png">`,
		} {
			if !strings.Contains(h, want) {
				t.Errorf("%s is missing %s", name, want)
			}
		}
	}
}

// The other things DBHQ makes were one line of footer byline. They are now a
// menu on the home page, a group in every docs sidebar, and a page.
func TestTheDBHQMenuIsReachableFromEveryPage(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		if name == "dbhq.html" {
			continue
		}
		if !strings.Contains(h, `href="/dbhq"`) {
			t.Errorf("%s has no way to reach the DBHQ projects page", name)
		}
	}
	home := htmlPages(t, out)["index.html"]
	if !strings.Contains(home, `<details class="org-menu">`) ||
		!strings.Contains(home, `<summary`) {
		t.Error("the home header has no DBHQ menu")
	}
	for _, want := range []string{"https://bbs.dbhq.uk", "https://modem.dbhq.uk", `href="/dbhq"`} {
		if !strings.Contains(home[:strings.Index(home, "</header>")], want) {
			t.Errorf("the DBHQ menu does not list %s", want)
		}
	}
}

// The footer carried a byline and, from #45, a three-item "Also from DBHQ"
// list. Both said the same thing on all 27 pages, at the point a reader has
// already left. The menu and the page say it where somebody is looking.
func TestTheFooterCarriesNoDBHQBlock(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		i := strings.Index(h, "<footer>")
		if i < 0 {
			t.Errorf("%s has no footer", name)
			continue
		}
		foot := h[i:]
		for _, gone := range []string{"free, open-source tool by", "Also from DBHQ", "also-list"} {
			if strings.Contains(foot, gone) {
				t.Errorf("%s still carries %q in its footer", name, gone)
			}
		}
	}
}

// The page itself: every project it names is a link, and it points at the
// company rather than describing it second-hand.
func TestTheDBHQPageLinksToTheProjectsItNames(t *testing.T) {
	out := buildSite(t)
	h, ok := htmlPages(t, out)["dbhq.html"]
	if !ok {
		t.Fatal("there is no dbhq page")
	}
	for _, want := range []string{
		"https://dbhq.uk", "https://bbs.dbhq.uk", "https://modem.dbhq.uk",
		"https://github.com/dbhq-uk/marketplace", "https://skills.dbhq.uk",
	} {
		if !strings.Contains(h, `href="`+want+`"`) {
			t.Errorf("the DBHQ page does not link to %s", want)
		}
	}
}

// The client-side navigation replaces main and the rail wholesale, which
// leaves anything bound to the old elements pointing at nodes that are no
// longer in the document. The rail stopped marking the current section after
// one soft navigation, and only a browser could see it: every page was
// correct on a hard load.
//
// The contract is an event. swap() announces; whatever needs rebinding
// listens, so the next thing that needs it does not have to edit swap().
func TestTheNavigationAnnouncesASwapAndTheRailListens(t *testing.T) {
	if !strings.Contains(site.NavJS, `dispatchEvent(new CustomEvent('hg:swap'`) &&
		!strings.Contains(site.NavJS, `hg:swap`) {
		t.Error("swap() does not announce that it replaced the page")
	}
	if !strings.Contains(site.RailJS, `'hg:swap'`) {
		t.Error("the rail does not rebind after a swap")
	}
	// And it must be able to run twice without stacking observers.
	if !strings.Contains(site.RailJS, "disconnect()") {
		t.Error("the rail does not disconnect its previous observer, so they stack")
	}
}

// llms.txt is the best agent-facing thing on a site whose whole argument is
// that agents read it more than people do, and nothing in the HTML pointed at
// it. The only references were a comment in robots.txt, which nothing parses,
// and the body of the 404 page.
func TestEveryPagePointsAtLLMSTxt(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		if !strings.Contains(h, `<link rel="alternate" type="text/plain" title="llms.txt" href="/llms.txt">`) {
			t.Errorf("%s does not announce llms.txt in its head", name)
		}
		i := strings.Index(h, "<footer>")
		if i < 0 || !strings.Contains(h[i:], `href="/llms.txt"`) {
			t.Errorf("%s has no visible link to llms.txt", name)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "llms.txt")); err != nil {
		t.Fatalf("llms.txt is announced and missing: %v", err)
	}
}

// author and publisher were the same Organization node on every page. So the
// strongest thing this site has - dated, measured, first-hand failure reports
// - was attributed to nobody a search engine or a model could verify. The
// company publishes; a person writes, and the person has a profile to check
// him against.
func TestTheAuthorIsAPersonAndThePublisherIsTheCompany(t *testing.T) {
	out := buildSite(t)
	re := regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	seen := 0
	for name, h := range htmlPages(t, out) {
		if name == "404.html" {
			continue
		}
		for _, b := range re.FindAllStringSubmatch(h, -1) {
			var v map[string]any
			if err := json.Unmarshal([]byte(b[1]), &v); err != nil {
				continue
			}
			author, ok := v["author"].(map[string]any)
			if !ok {
				continue
			}
			seen++
			if author["@type"] != "Person" {
				t.Errorf("%s: author is a %v, not a Person", name, author["@type"])
			}
			if author["name"] != "Daniel Grimes" {
				t.Errorf("%s: author is %v, which names no human", name, author["name"])
			}
			if _, ok := author["sameAs"]; !ok {
				t.Errorf("%s: the author has no sameAs, so nothing can verify him", name)
			}
			pub, ok := v["publisher"].(map[string]any)
			if !ok {
				t.Errorf("%s: an author with no publisher", name)
				continue
			}
			if pub["@type"] != "Organization" || pub["name"] != "DBHQ" {
				t.Errorf("%s: publisher is %v %v, want the DBHQ Organization", name, pub["@type"], pub["name"])
			}
		}
	}
	if seen == 0 {
		t.Error("no page carries an author at all")
	}
}

// The footer byline is back, and it is not the block that was removed with
// #54. That one repeated the company and three of its links on all 27 pages,
// at the point a reader has already left. This one names the human the JSON-LD
// now calls the author, in the one place every page has: a claim a reader can
// check is worth more than a link nobody clicks.
func TestTheFooterNamesTheHumanWhoWroteIt(t *testing.T) {
	out := buildSite(t)
	for name, h := range htmlPages(t, out) {
		i := strings.Index(h, "<footer>")
		if i < 0 {
			t.Errorf("%s has no footer", name)
			continue
		}
		foot := h[i:]
		if !strings.Contains(foot, "Daniel Grimes") {
			t.Errorf("%s: the footer names no human", name)
		}
		if !strings.Contains(foot, `href="/dbhq"`) {
			t.Errorf("%s: the byline does not link the page that says who that is", name)
		}
	}
}

// The mirrors are for agents. Google and Bing are offered the HTML only, so
// the same page is not served to an index twice: rel="alternate" is not a
// documented deduplication signal, the mirrors are linked three times per
// page, and on GitHub Pages they can carry neither a canonical tag nor an
// X-Robots-Tag. The scoping is per-crawler on purpose. Under `User-agent: *`
// it would shut out every AI agent as well, which is the readership the
// mirrors exist for.
func TestRobotsKeepsTheMirrorsFromSearchCrawlersOnly(t *testing.T) {
	out := buildSite(t)
	b, err := os.ReadFile(filepath.Join(out, "robots.txt"))
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string][]string{}
	agent := ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if strings.EqualFold(k, "user-agent") {
			agent = v
			if _, ok := groups[agent]; !ok {
				groups[agent] = nil
			}
			continue
		}
		if agent != "" {
			groups[agent] = append(groups[agent], k+": "+v)
		}
	}
	for _, crawler := range []string{"Googlebot", "Bingbot"} {
		rules, ok := groups[crawler]
		if !ok {
			t.Errorf("robots.txt has no group for %s", crawler)
			continue
		}
		if !containsRule(rules, "Disallow: /*.md$") {
			t.Errorf("%s is not kept off the markdown mirrors: %v", crawler, rules)
		}
	}
	for _, rule := range groups["*"] {
		if strings.HasPrefix(rule, "Disallow:") && rule != "Disallow:" {
			t.Errorf("`User-agent: *` carries %q, which shuts out every agent, not just search", rule)
		}
	}
	if !strings.Contains(string(b), "Sitemap: "+baseURL+"/sitemap.xml") {
		t.Error("robots.txt no longer names the sitemap")
	}
}

func containsRule(rules []string, want string) bool {
	for _, r := range rules {
		if strings.EqualFold(r, want) {
			return true
		}
	}
	return false
}

// dateModified alone says a page changed and never says when it arrived. A
// page written in September and corrected in March reads as a March page, and
// first-hand experience that has been there since the start looks new.
func TestArticlesSayWhenTheyArrivedAsWellAsWhenTheyChanged(t *testing.T) {
	out := buildSite(t)
	re := regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	date := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	for name, h := range htmlPages(t, out) {
		if name == "404.html" || name == "index.html" {
			continue
		}
		for _, b := range re.FindAllStringSubmatch(h, -1) {
			var v map[string]any
			if err := json.Unmarshal([]byte(b[1]), &v); err != nil || v["@type"] != "TechArticle" {
				continue
			}
			pub, _ := v["datePublished"].(string)
			mod, _ := v["dateModified"].(string)
			if !date.MatchString(pub) {
				t.Errorf("%s: datePublished is %q", name, pub)
				continue
			}
			if pub > mod {
				t.Errorf("%s: published %s, modified %s - a page cannot change before it exists", name, pub, mod)
			}
		}
	}
}

func TestFirstPublishedIsTheFirstCommitThatTouchedThePage(t *testing.T) {
	repo := gitRepo(t, "2024-03-04T05:06:07Z")
	commitPage(t, repo, "2025-07-08T09:10:11Z")
	pub, err := firstPublished(repo, "page.md", "2025-07-08")
	if err != nil {
		t.Fatal(err)
	}
	if pub != "2024-03-04" {
		t.Errorf("datePublished is %q, want the first commit 2024-03-04", pub)
	}
	mod, err := lastModified(repo, "page.md")
	if err != nil {
		t.Fatal(err)
	}
	if mod != "2025-07-08" {
		t.Errorf("dateModified is %q, want the last commit 2025-07-08", mod)
	}
}

// A page that is not in a checkout at all, or not committed yet, has one
// honest date and it is the one the sitemap already uses.
func TestFirstPublishedFallsBackToTheModifiedDate(t *testing.T) {
	pub, err := firstPublished(t.TempDir(), "page.md", "2026-01-02")
	if err != nil {
		t.Fatal(err)
	}
	if pub != "2026-01-02" {
		t.Errorf("got %q, want the modified date 2026-01-02", pub)
	}
}

// commitPage rewrites page.md and commits it at the given date.
func commitPage(t *testing.T, dir, date string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "page.md"), []byte("# p\n\nbody, changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", "page.md")
	cmd := exec.Command("git", "commit", "--quiet", "-m", "changed")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, b)
	}
}

// The markdown mirror is justified in four files by a number, and the number
// was wrong: "roughly 31 times more bytes as HTML than as markdown" was never
// measured across the site. It is about 8 times on the median page and never
// more than 17. The floor is the longest page - /transports, at 2.97 - because
// chrome is a fixed cost and long pages dilute it, which is the same reason
// the saving is quoted as a range rather than a single figure. The saving is
// real and worth the mirrors; the figure has to be one somebody can reproduce,
// so this measures it and fails when the comments and the build stop agreeing.
func TestTheMirrorSavingIsTheOneTheCommentsClaim(t *testing.T) {
	out := buildSite(t)
	const lo, hi = 2.9, 17.0
	n := 0
	for name, h := range htmlPages(t, out) {
		if name == "404.html" {
			continue
		}
		md, err := os.ReadFile(filepath.Join(out, strings.TrimSuffix(name, ".html")+".md"))
		if err != nil {
			t.Errorf("%s has no markdown mirror: %v", name, err)
			continue
		}
		r := float64(len(h)) / float64(len(md))
		if r < lo || r > hi {
			t.Errorf("%s is %.1fx its mirror, outside the %.0fx to %.0fx the comments claim: remeasure and change both", name, r, lo, hi)
		}
		n++
	}
	if n == 0 {
		t.Error("no pages were measured")
	}
}
