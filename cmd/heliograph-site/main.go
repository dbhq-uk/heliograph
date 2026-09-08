// Command heliograph-site builds the documentation.
//
// Three renderings of one source: HTML at /page, the markdown mirror at
// /page.md, and llms.txt at the root.
//
// The markdown mirror is not a nicety. The same page costs roughly 31 times
// more bytes as HTML than as markdown, so serving chrome to an agent is a token
// tax on every read, and agents read these pages far more often than people do.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dbhq-uk/heliograph/internal/site"
)

// order fixes the navigation. Alphabetical would put the CLI reference before
// the quick start, which is the wrong way round for somebody arriving.
var order = []string{
	"index", "install", "quickstart",
	"claude-code", "mcp",
	"station", "bootstrap", "steps", "runner", "conformance",
	"hosts", "containers", "service", "azure", "pipelines", "windows",
	"transports", "relay", "intercom", "cli", "secrets", "security", "method",
}

const baseURL = "https://heliograph.dbhq.uk"

func main() {
	src := flagOr(1, "site/content")
	out := flagOr(2, "site/dist")
	if err := build(src, out); err != nil {
		fmt.Fprintln(os.Stderr, "heliograph-site: "+err.Error())
		os.Exit(1)
	}
}

func flagOr(i int, def string) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return def
}

func build(src, out string) error {
	ents, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	var pages []site.Page
	seen := map[string]bool{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		slug := strings.TrimSuffix(e.Name(), ".md")
		body := string(b)
		title := site.Title(body)
		if title == "" {
			// A page with no H1 has no title, no nav entry and no llms.txt
			// line. Better to refuse the build than to publish it nameless.
			return fmt.Errorf("%s has no H1, so it has no title", e.Name())
		}
		pages = append(pages, site.Page{Slug: slug, Title: title, Body: body})
		seen[slug] = true
	}
	if len(pages) == 0 {
		return fmt.Errorf("no pages in %s", src)
	}

	if err := validateNavigation(src, pages, seen); err != nil {
		return err
	}

	sort.Slice(pages, func(a, b int) bool { return rank(pages[a].Slug) < rank(pages[b].Slug) })

	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, p := range pages {
		if err := os.WriteFile(filepath.Join(out, p.Slug+".html"),
			[]byte(page(p, pages)), 0o644); err != nil {
			return err
		}
		// The markdown mirror, byte for byte the source.
		if err := os.WriteFile(filepath.Join(out, p.Slug+".md"),
			[]byte(p.Body), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(out, "llms.txt"), []byte(llms(pages)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "llms-full.txt"), []byte(llmsFull(pages)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "style.css"), []byte(site.CSS), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "sitemap.xml"), []byte(sitemap(pages)), 0o644); err != nil {
		return err
	}
	// robots.txt names the sitemap and the markdown mirrors. Agents are the
	// heavier readership here, and llms.txt is not discoverable on its own.
	robots := "User-agent: *\nAllow: /\n\nSitemap: " + baseURL + "/sitemap.xml\n" +
		"\n# Markdown mirrors of every page at <path>.md, and " + baseURL + "/llms.txt\n"
	if err := os.WriteFile(filepath.Join(out, "robots.txt"), []byte(robots), 0o644); err != nil {
		return err
	}
	// Fonts and the logo. Copied by the build rather than by a step in the
	// deploy workflow: a site that renders locally and ships without its
	// typeface is a failure nobody sees until it is live.
	if err := copyTree(filepath.Join(filepath.Dir(src), "assets"), filepath.Join(out, "assets")); err != nil {
		return fmt.Errorf("copying assets: %w", err)
	}
	fmt.Printf("built %d pages into %s\n", len(pages), out)
	return nil
}

// copyTree copies a directory, and refuses an empty one.
//
// An empty assets directory means the fonts and the mark are missing, and the
// site would still build, deploy, and serve in a fallback typeface. Better to
// fail here than to find out from the live page.
func copyTree(from, to string) error {
	ents, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	n := 0
	for _, e := range ents {
		src, dst := filepath.Join(from, e.Name()), filepath.Join(to, e.Name())
		if e.IsDir() {
			if err := copyTree(src, dst); err != nil {
				return err
			}
			continue
		}
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
		n++
	}
	if n == 0 && len(ents) == 0 {
		return fmt.Errorf("%s is empty", from)
	}
	return nil
}

func sitemap(pages []site.Page) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range pages {
		loc := baseURL + "/" + p.Slug
		if p.Slug == "index" {
			loc = baseURL + "/"
		}
		fmt.Fprintf(&b, "  <url><loc>%s</loc></url>\n", loc)
	}
	b.WriteString("</urlset>\n")
	return b.String()
}

// validateNavigation refuses to build a site somebody could get lost in.
//
// `order` used to be the navigation, so membership in it proved reachability.
// It is now only the sort order and the llms.txt sequence: the SIDEBAR is what
// a reader navigates by, and a page can sit in `order` while appearing in no
// group at all. So all four structures are checked, because each can now be
// wrong on its own.
func validateNavigation(src string, pages []site.Page, seen map[string]bool) error {
	ordered := map[string]bool{}
	for _, slug := range order {
		if ordered[slug] {
			return fmt.Errorf("`order` names %q more than once", slug)
		}
		ordered[slug] = true
		if !seen[slug] {
			return fmt.Errorf("`order` names %q, which does not exist in %s", slug, src)
		}
	}

	// One group per page, and every page in one. A page in two groups appears
	// twice in the sidebar, which reads as two different pages.
	grouped := map[string]string{}
	for _, g := range groups {
		for _, slug := range g.slugs {
			if !seen[slug] {
				return fmt.Errorf("sidebar group %q names %q, which does not exist in %s", g.name, slug, src)
			}
			if prev, ok := grouped[slug]; ok {
				return fmt.Errorf("%q is in both sidebar groups %q and %q", slug, prev, g.name)
			}
			grouped[slug] = g.name
		}
	}

	for _, p := range pages {
		if !ordered[p.Slug] {
			return fmt.Errorf("%s.md is missing from `order`", p.Slug)
		}
		if _, ok := grouped[p.Slug]; !ok {
			return fmt.Errorf("%s.md is in no sidebar group, so nobody would find it: add it to `groups`", p.Slug)
		}
		if strings.TrimSpace(labels[p.Slug]) == "" {
			return fmt.Errorf("%s.md has no short navigation label: add it to `labels`", p.Slug)
		}
	}
	for slug := range labels {
		if !seen[slug] {
			return fmt.Errorf("`labels` names %q, which does not exist in %s", slug, src)
		}
	}

	// The home page's only route into the documentation. Without one, the site
	// has twenty-three pages and no way in.
	docs := false
	for _, item := range homeNav {
		if strings.TrimSpace(item.label) == "" {
			return fmt.Errorf("the home navigation has an empty label for %q", item.slug)
		}
		if !seen[item.slug] {
			return fmt.Errorf("the home navigation names %q, which does not exist in %s", item.slug, src)
		}
		if item.slug != "index" {
			docs = true
		}
	}
	if !docs {
		return fmt.Errorf("the home navigation has no entry into the documentation")
	}
	return nil
}

func inOrder(s string) bool {
	for _, o := range order {
		if o == s {
			return true
		}
	}
	return false
}

func rank(s string) int {
	for i, o := range order {
		if o == s {
			return i
		}
	}
	return len(order)
}

// groups fix the sidebar's sections. A flat list of eight is a list somebody
// scans twice; three short groups is one somebody reads once. The names say
// what a reader is trying to do, not what the pages are about.
var groups = []struct {
	name  string
	slugs []string
}{
	{"Start here", []string{"index", "install", "quickstart"}},
	{"Drive it from an agent", []string{"claude-code", "mcp"}},
	{"The far side", []string{"station", "bootstrap", "steps", "runner", "conformance"}},
	{"Where it runs", []string{"hosts", "containers", "service", "azure", "pipelines", "windows"}},
	{"Reference", []string{"transports", "relay", "intercom", "cli", "secrets", "security", "method"}},
}

// labels are the navigation's own words, and they are a THIRD set of words for
// each page, deliberately. The three have different jobs and nothing is gained
// by making one do all of them:
//
//	H1       explains the page to somebody already reading it
//	<title>  has to work with no page around it, in a tab or a search result
//	label    has to be scannable in a narrow column, at a glance
//
// The navigation used the H1, and at eight pages that was survivable. At
// twenty-three it produced a header reading "Making the loop outlive the
// session", "Intercom - when you can reach the station", "Where a station can
// run" - a sitemap poured into a nav bar, three rows deep.
var labels = map[string]string{
	"index":       "Overview",
	"install":     "Install",
	"quickstart":  "Quick start",
	"claude-code": "Claude Code",
	"mcp":         "MCP server",
	"station":     "Station",
	"bootstrap":   "Plant a station",
	"steps":       "Write a step",
	"runner":      "Runner reference",
	"conformance": "Capture contract",
	"hosts":       "Host requirements",
	"containers":  "Containers",
	"service":     "Survive logout",
	"azure":       "Azure",
	"pipelines":   "Pipelines",
	"windows":     "Windows",
	"transports":  "Transports",
	"relay":       "Relay",
	"intercom":    "Intercom",
	"cli":         "CLI reference",
	"secrets":     "Secrets",
	"security":    "Security",
	"method":      "Debugging method",
}

func label(o site.Page) string { return labels[o.Slug] }

// homeNav is the header on the marketing page, and it is SHORT on purpose.
//
// The header used to render every page. A visitor arriving at the home page was
// met with the entire documentation tree before a single sentence about what
// the thing does, which is the opposite of what a home page is for.
//
// Three links. `Docs` opens the quick start, because that is where somebody who
// has decided to try this actually wants to be, and the docs shell brings its
// grouped sidebar with it - so one link reaches all twenty-three pages. The
// brand links home and the hero already offers Source, so neither is repeated
// here.
type homeNavItem struct {
	label string
	slug  string
}

var homeNav = []homeNavItem{
	{label: "Docs", slug: "quickstart"},
	{label: "Install", slug: "install"},
	{label: "Security", slug: "security"},
}

// headerNav renders the header navigation, which exists only on the index.
//
// It used to be rendered on every page and hidden on docs pages with CSS. That
// worked and was still wrong: every docs page shipped a second copy of the
// whole navigation, which a screen reader still reaches and a stylesheet
// failure would reveal.
func headerNav(p site.Page) string {
	if p.Slug != "index" {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav aria-label="Primary">`)
	for _, item := range homeNav {
		fmt.Fprintf(&b, `<a href="%s">%s</a>`, href(item.slug), escAttr(item.label))
	}
	b.WriteString(`</nav>`)
	return b.String()
}

func href(slug string) string {
	if slug == "index" {
		return "/"
	}
	return "/" + slug
}

// sidebarItems is the documentation navigation, emitted as labelled lists.
//
// It was a flat run of <p> and <a> siblings. A group heading that is only
// visually above its links is a heading to a sighted reader and nothing at all
// to anybody else, so the groups are real lists now, each pointed at its
// heading by aria-labelledby.
//
// `prefix` exists because this is emitted TWICE on a docs page - once in the
// desktop sidebar, once inside the mobile drawer - and two elements may not
// share an id. The duplication is deliberate: a permanent sidebar and a modal
// drawer are not the same component, and pretending they are is what produces
// a drawer that cannot trap focus.
func sidebarItems(p site.Page, all []site.Page, prefix string) string {
	byslug := map[string]site.Page{}
	for _, o := range all {
		byslug[o.Slug] = o
	}
	var b strings.Builder
	for i, g := range groups {
		id := fmt.Sprintf("%s-group-%d", prefix, i)
		fmt.Fprintf(&b, `<div class="side-group"><p class="grp" id="%s">%s</p><ul aria-labelledby="%s">`,
			id, escAttr(g.name), id)
		for _, slug := range g.slugs {
			o, ok := byslug[slug]
			if !ok {
				continue
			}
			// aria-current is the machine-readable half. `here` styles it; on
			// its own it told a screen reader nothing about which page it was
			// already on.
			attrs := ""
			if o.Slug == p.Slug {
				attrs = ` class="here" aria-current="page"`
			}
			fmt.Fprintf(&b, `<li><a href="%s"%s>%s</a></li>`, href(slug), attrs, escAttr(label(o)))
		}
		b.WriteString(`</ul></div>`)
	}
	return b.String()
}

// rail is the "on this page" column. Omitted below three headings: a rail with
// two entries is furniture, and it takes width from the thing it is pointing at.
func rail(p site.Page) string {
	hs := site.Headings(p.Body)
	if len(hs) < 3 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<aside class="rail" aria-labelledby="page-nav-title">` +
		`<p class="grp" id="page-nav-title">On this page</p>` +
		`<nav aria-labelledby="page-nav-title">`)
	for _, h := range hs {
		fmt.Fprintf(&b, `<a href="#%s">%s</a>`, escAttr(h[0]), escAttr(h[1]))
	}
	b.WriteString(`</nav></aside>`)
	return b.String()
}

func page(p site.Page, all []site.Page) string {
	nav := headerNav(p)
	canonical := baseURL + "/" + p.Slug
	if p.Slug == "index" {
		canonical = baseURL + "/"
	}

	// The index carries the hero and the log strip. Every other page is a
	// reading surface and gets neither: a docs page competing with its own
	// header is a docs page nobody finishes.
	// The index carries the hero, the log strip and its own three-link header.
	// A docs page carries none of them: it gets the mobile bar, the drawer and
	// the three-column shell instead.
	hero, wide := "", ""
	header, shellOpen, shellClose, railHTML, navJS := "", "", "", "", ""
	if p.Slug == "index" {
		hero, wide = heroHTML, " wide"
		header = fmt.Sprintf(`<header class="site-header">
  <a class="brand" href="/">%s heliograph</a>
  %s
</header>`, site.Mark, nav)
	} else {
		// THE DOCS HEADER IS GONE. It was a full-width sticky bar carrying only
		// the logo, which cost about 56px of every page and forced both sticky
		// columns onto a magic `top:3.6rem` offset that only approximated its
		// height. The brand moves into the sidebar, where it shares an edge
		// with something, and the sticky offsets become zero.
		header = fmt.Sprintf(`<header class="mobile-bar">
  <a class="brand" href="/">%s heliograph</a>
  <button class="menu-button" id="docs-menu-open" type="button"
    aria-controls="docs-menu" aria-expanded="false" aria-haspopup="dialog">
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M4 7h16M4 12h16M4 17h16"/></svg>
    <span>Menu</span>
  </button>
</header>
<dialog class="nav-dialog" id="docs-menu" aria-labelledby="docs-menu-title">
  <div class="nav-dialog-panel">
    <div class="nav-dialog-head">
      <h2 id="docs-menu-title">Documentation</h2>
      <button class="menu-close" type="button" data-close-menu aria-label="Close the documentation menu">
        <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M6 6l12 12M18 6L6 18"/></svg>
      </button>
    </div>
    <nav class="side-nav" aria-label="Documentation pages">%s</nav>
  </div>
</dialog>`, site.Mark, sidebarItems(p, all, "drawer"))

		shellOpen = `<div class="docs-shell"><aside class="side">` +
			`<a class="brand side-brand" href="/">` + site.Mark + ` heliograph</a>` +
			`<nav class="side-nav" aria-label="Documentation">` +
			sidebarItems(p, all, "desktop") + `</nav></aside><div class="col">`
		shellClose = `</div>`
		railHTML = rail(p) + `</div>`
		navJS = site.NavJS
	}

	title := titles[p.Slug]
	if title == "" {
		title = p.Title + " - heliograph"
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en-GB" class="no-js">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%[1]s</title>
<meta name="description" content="%[2]s">
<meta name="theme-color" content="#080C12">
<meta property="og:title" content="%[1]s">
<meta property="og:description" content="%[2]s">
<meta property="og:type" content="website">
<meta property="og:url" content="%[3]s">
<meta property="og:site_name" content="heliograph">
<meta name="twitter:card" content="summary">
<link rel="canonical" href="%[3]s">
<link rel="icon" href="/assets/favicon.svg" type="image/svg+xml">
<!-- The markdown mirror, announced so an agent does not have to guess. -->
<link rel="alternate" type="text/markdown" href="/%[4]s.md">
<link rel="preload" href="/assets/fonts/InstrumentSerif-400.woff2" as="font" type="font/woff2" crossorigin>
<link rel="preload" href="/assets/fonts/InstrumentSans.woff2" as="font" type="font/woff2" crossorigin>
<link rel="stylesheet" href="/style.css">
<!-- Flipped before first paint, so a no-JS reader never sees a control that
     cannot work. The drawer needs a real dialog; the sidebar does not. -->
<script>document.documentElement.classList.replace('no-js','js')</script>
<a class="skip-link" href="#main-content">Skip to content</a>
%[5]s
%[7]s
%[11]s
<main id="main-content" tabindex="-1" class="doc%[8]s">
%[9]s
</main>
%[12]s
%[13]s
<footer><div class="inner">
<p>A free, open-source tool by <a href="https://dbhq.uk">DBHQ</a>.</p>
<p><a href="https://github.com/dbhq-uk/heliograph">Source</a> &middot; <a href="/%[4]s.md">This page as markdown</a></p>
</div></footer>
<script>%[10]s</script>
%[14]s
`, escAttr(title), escAttr(site.Summary(p.Body)), canonical, p.Slug,
		header, nav, hero, wide, site.RenderBody(p.Body), site.HeroJS,
		shellOpen, shellClose, railHTML, navJS)
}

// titles are written per page rather than derived from the H1.
//
// A title tag is the one piece of copy that has to work with no page around it:
// in a search result, a browser tab, a shared link. "Transports - heliograph"
// says nothing to somebody who has never heard of either word.
//
// They also carry the words people actually search. heliograph ships as a
// Claude Code skill, and that is what a reader is looking for when they find
// this - not a category name nobody types.
var titles = map[string]string{
	"index":       "heliograph - run commands on a machine you cannot log into",
	"install":     "Install heliograph - a single binary, and nothing on the far side",
	"quickstart":  "Quick start - from nothing to a captured log in five steps",
	"claude-code": "heliograph for Claude Code - drive a machine the agent cannot reach",
	"transports":  "Transports - git, relay, file share and bundle",
	"cli":         "CLI reference - send, watch, logs --gaps, plant, doctor",
	"method":      "The method - how to debug across a gap you cannot cross",
	"mcp":         "MCP server - heliograph as typed tools for any agent",
	"station":     "The station - what heliograph runs on the far side",
	"bootstrap":   "Planting a station - with the CLI, or without it",
	"steps":       "Writing a step - one file, one question, and the traps",
	"runner":      "Runner reference - start.sh, station.sh, run.sh and every knob",
	"conformance": "The capture contract - nine properties every implementation must pass",
	"hosts":       "Where a station can run - the host contract, and what is proven",
	"containers":  "Docker and Kubernetes - running a station in a container",
	"service":     "Survive a logout - systemd, launchd and Windows scheduled tasks",
	"azure":       "Azure - five templates, and what deploying them taught us",
	"pipelines":   "Pipelines - running a station on a GitHub or Azure DevOps agent",
	"windows":     "Windows - hosting the loop, and steps written in PowerShell",
	"relay":       "The relay - zero-infrastructure heliograph over ordinary HTTPS",
	"secrets":     "Secrets - redaction, and getting a value to the far side",
	"security":    "Security - the gates, the blast radius, and what this refuses to do",
	"intercom":    "Intercom - submitting a step over HTTPS, when you can reach the station",
}

// heroHTML is the index's opening: the signal crossing the valley, then a real
// captured log with a real gap in its timestamp column.
//
// The log is not decoration. It is the single most distinctive fact about the
// product - a hang shows up as a gap, and nothing else in the category shows
// you that - so it is shown rather than described, above the fold.
const heroHTML = `<section class="hero">
  <canvas id="signal" aria-hidden="true"></canvas>
  <div class="hero-inner">
    <h1>Run it on a machine you <em>cannot log into</em>.</h1>
    <p class="lede">You push a step. It runs on the far side. The whole run comes
    back as a log with every line timestamped in UTC, whether it passed or failed.</p>
    <div class="cta">
      <a class="btn btn-primary" href="/quickstart">Quick start</a>
      <a class="btn btn-ghost" href="https://github.com/dbhq-uk/heliograph">Source</a>
    </div>
  </div>
</section>
<section class="strip"><div class="strip-inner">
  <h2>A hang is a gap, and the gap is the finding</h2>
  <pre class="log"><code><span class="t">09:14:00</span> | ---------- terraform plan ----------
<span class="t">09:14:02</span> | Refreshing state...
<span class="gap">           3m12s   nothing was produced here. This is the answer.</span>
<span class="t">09:17:14</span> | Plan: 3 to add, 0 to change
<span class="t">09:17:15</span> | done</code></pre>
</div></section>
`

func escAttr(s string) string {
	r := strings.NewReplacer(`&`, "&amp;", `"`, "&quot;", `<`, "&lt;", `>`, "&gt;")
	return r.Replace(s)
}

// llms.txt: a curated index, organised by section rather than as one flat list.
//
// Shipped because IDE agents fetch it and it costs almost nothing, not because
// it will win citations: one log study found 408 requests to llms.txt out of
// more than 500 million AI bot visits in ninety days.
func llms(pages []site.Page) string {
	var b strings.Builder
	b.WriteString("# heliograph\n\n")
	b.WriteString("> Remote, captured, auditable execution on a machine you cannot log into. ")
	b.WriteString("You push a step, it runs on the far side, and the whole run comes back as a log ")
	b.WriteString("with every line timestamped in UTC, whether it passed or failed.\n\n")
	b.WriteString("## Docs\n\n")
	for _, p := range pages {
		fmt.Fprintf(&b, "- [%s](%s/%s.md): %s\n", p.Title, baseURL, p.Slug, site.Summary(p.Body))
	}
	b.WriteString("\n## Source\n\n")
	b.WriteString("- [heliograph](https://github.com/dbhq-uk/heliograph): the control CLI and transports\n")
	b.WriteString("- [station/bash](https://github.com/dbhq-uk/heliograph/tree/main/station/bash): the far-side station, plain bash, in this repository\n")
	return b.String()
}

func llmsFull(pages []site.Page) string {
	var b strings.Builder
	for _, p := range pages {
		b.WriteString(p.Body)
		b.WriteString("\n\n---\n\n")
	}
	return b.String()
}
