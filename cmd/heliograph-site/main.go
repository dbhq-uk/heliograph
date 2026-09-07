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
var order = []string{"index", "install", "quickstart", "transports", "cli", "method"}

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

func page(p site.Page, all []site.Page) string {
	var nav strings.Builder
	for _, o := range all {
		href := "/" + o.Slug
		if o.Slug == "index" {
			href = "/"
		}
		cls := ""
		if o.Slug == p.Slug {
			cls = ` class="here"`
		}
		label := o.Title
		if o.Slug == "index" {
			label = "Overview"
		}
		fmt.Fprintf(&nav, `<a href="%s"%s>%s</a>`, href, cls, escAttr(label))
	}
	canonical := baseURL + "/" + p.Slug
	if p.Slug == "index" {
		canonical = baseURL + "/"
	}

	// The index carries the hero and the log strip. Every other page is a
	// reading surface and gets neither: a docs page competing with its own
	// header is a docs page nobody finishes.
	hero, wide := "", ""
	if p.Slug == "index" {
		hero, wide = heroHTML, " wide"
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en-GB">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s - heliograph</title>
<meta name="description" content="%s">
<meta name="theme-color" content="#08090B">
<link rel="canonical" href="%s">
<link rel="icon" href="/assets/favicon.svg" type="image/svg+xml">
<!-- The markdown mirror, announced so an agent does not have to guess. -->
<link rel="alternate" type="text/markdown" href="/%s.md">
<link rel="preload" href="/assets/fonts/InstrumentSerif-400.woff2" as="font" type="font/woff2" crossorigin>
<link rel="preload" href="/assets/fonts/InstrumentSans.woff2" as="font" type="font/woff2" crossorigin>
<link rel="stylesheet" href="/style.css">
<header>
  <a class="brand" href="/">%s heliograph</a>
  <nav>%s</nav>
</header>
%s
<main class="doc%s">
%s
</main>
<footer><div class="inner">
<p>A free, open-source tool by <a href="https://dbhq.uk">DBHQ</a>.</p>
<p><a href="https://github.com/dbhq-uk/heliograph">Source</a> &middot; <a href="/%s.md">This page as markdown</a></p>
</div></footer>
<script>%s</script>
`, escAttr(p.Title), escAttr(site.Summary(p.Body)), canonical, p.Slug,
		site.Mark, nav.String(), hero, wide, site.RenderBody(p.Body), p.Slug, site.HeroJS)
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
	b.WriteString("- [heliograph-skill](https://github.com/dbhq-uk/heliograph-skill): the far-side station, plain bash\n")
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
