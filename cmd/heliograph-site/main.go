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
var order = []string{"index", "install", "quickstart", "claude-code", "mcp", "transports", "cli", "method"}

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

	// Every page must be reachable. A page that exists and is in no navigation
	// is a page nobody finds, which is the same as not having written it.
	for _, s := range order {
		if !seen[s] {
			return fmt.Errorf("the navigation names %q, which does not exist in %s", s, src)
		}
	}
	for _, p := range pages {
		if !inOrder(p.Slug) {
			return fmt.Errorf("%s.md is in no navigation, so nobody would find it: add it to `order`", p.Slug)
		}
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
	// GA4 and its consent prompt, as two files rather than one. analytics.js
	// is denied by default and loads nothing on its own; consent.js is the
	// only thing that can turn it on, and the page loads it second so
	// __dbhqEnableGA is defined by document order rather than by luck.
	if err := os.WriteFile(filepath.Join(out, "analytics.js"), []byte(site.AnalyticsJS), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "consent.js"), []byte(site.ConsentJS), 0o644); err != nil {
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
	{"Reference", []string{"transports", "cli", "method"}},
}

func label(o site.Page) string {
	if o.Slug == "index" {
		return "Overview"
	}
	return o.Title
}

func href(slug string) string {
	if slug == "index" {
		return "/"
	}
	return "/" + slug
}

// sidebar is the docs navigation: every page, grouped, with the current one
// marked. It replaces a top nav that had eight items in a row - which fitted
// on a laptop and wrapped on anything smaller, and left the content with
// nothing to align to.
func sidebar(p site.Page, all []site.Page) string {
	byslug := map[string]site.Page{}
	for _, o := range all {
		byslug[o.Slug] = o
	}
	var b strings.Builder
	for _, g := range groups {
		fmt.Fprintf(&b, `<p class="grp">%s</p>`, escAttr(g.name))
		for _, slug := range g.slugs {
			o, ok := byslug[slug]
			if !ok {
				continue
			}
			cls := ""
			if o.Slug == p.Slug {
				cls = ` class="here"`
			}
			fmt.Fprintf(&b, `<a href="%s"%s>%s</a>`, href(slug), cls, escAttr(label(o)))
		}
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
	b.WriteString(`<aside class="rail"><p class="grp">On this page</p><nav>`)
	for _, h := range hs {
		fmt.Fprintf(&b, `<a href="#%s">%s</a>`, escAttr(h[0]), escAttr(h[1]))
	}
	b.WriteString(`</nav></aside>`)
	return b.String()
}

func page(p site.Page, all []site.Page) string {
	var nav strings.Builder
	for _, o := range all {
		fmt.Fprintf(&nav, `<a href="%s"%s>%s</a>`, href(o.Slug),
			map[bool]string{true: ` class="here"`, false: ""}[o.Slug == p.Slug],
			escAttr(label(o)))
	}
	canonical := baseURL + "/" + p.Slug
	if p.Slug == "index" {
		canonical = baseURL + "/"
	}

	// The index carries the hero and the log strip. Every other page is a
	// reading surface and gets neither: a docs page competing with its own
	// header is a docs page nobody finishes.
	hero, wide := "", ""
	shellOpen, shellClose, railHTML := "", "", ""
	if p.Slug == "index" {
		hero, wide = heroHTML, " wide"
	} else {
		// Three columns, the way a docs site that fills its window works:
		// navigation on the left, the reading column next to it, and the
		// page's own headings on the right.
		//
		// The old layout put a full-bleed header above a centred 68ch column,
		// so the logo sat at the far left while the first word of the body
		// began a third of the way across, and neither shared an edge with
		// anything. That reads as broken even to somebody who could not say
		// why.
		shellOpen = `<div class="shell"><aside class="side"><nav>` +
			sidebar(p, all) + `</nav></aside><div class="col">`
		shellClose = `</div>`
		railHTML = rail(p) + `</div>`
	}

	title := titles[p.Slug]
	if title == "" {
		title = p.Title + " - heliograph"
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en-GB">
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
<header>
  <a class="brand" href="/">%[5]s heliograph</a>
  <nav>%[6]s</nav>
</header>
%[7]s
%[11]s
<main class="doc%[8]s">
%[9]s
</main>
%[12]s
%[13]s
<footer><div class="inner">
<p>A free, open-source tool by <a href="https://dbhq.uk">DBHQ</a>.</p>
<p><a href="https://github.com/dbhq-uk/heliograph">Source</a> &middot; <a href="/%[4]s.md">This page as markdown</a></p>
</div></footer>
%[14]s
<script>%[10]s</script>
<script src="/analytics.js"></script>
<script src="/consent.js"></script>
`, escAttr(title), escAttr(site.Summary(p.Body)), canonical, p.Slug,
		site.Mark, nav.String(), hero, wide, site.RenderBody(p.Body), site.HeroJS,
		shellOpen, shellClose, railHTML, site.ConsentHTML)
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
