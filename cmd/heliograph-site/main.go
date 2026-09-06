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
	if err := os.WriteFile(filepath.Join(out, "style.css"), []byte(css), 0o644); err != nil {
		return err
	}
	fmt.Printf("built %d pages into %s\n", len(pages), out)
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
		fmt.Fprintf(&nav, `<a href="%s"%s>%s</a>`, href, cls, o.Title)
	}
	canonical := baseURL + "/" + p.Slug
	if p.Slug == "index" {
		canonical = baseURL + "/"
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en-GB">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s - heliograph</title>
<meta name="description" content="%s">
<link rel="canonical" href="%s">
<!-- The markdown mirror, announced so an agent does not have to guess. -->
<link rel="alternate" type="text/markdown" href="/%s.md">
<link rel="stylesheet" href="/style.css">
<header><a class="brand" href="/">heliograph</a><nav>%s</nav></header>
<main>
%s
</main>
<footer>
<p>A free, open-source tool by <a href="https://dbhq.uk">DBHQ</a>.
<a href="https://github.com/dbhq-uk/heliograph">Source</a>.
This page as <a href="/%s.md">markdown</a>.</p>
</footer>
`, escAttr(p.Title), escAttr(site.Summary(p.Body)), canonical, p.Slug,
		nav.String(), site.RenderBody(p.Body), p.Slug)
}

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

const css = `:root{--ink:#1a1a1a;--dim:#5a5a5a;--line:#e2e2e2;--bg:#fff;--code:#f6f6f4;--link:#0b5cad}
@media(prefers-color-scheme:dark){:root{--ink:#e8e8e8;--dim:#a0a0a0;--line:#333;--bg:#141414;--code:#1e1e1e;--link:#79b8ff}}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);
  font:16px/1.65 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif}
header{display:flex;gap:1.5rem;align-items:baseline;flex-wrap:wrap;
  padding:1rem 1.5rem;border-bottom:1px solid var(--line)}
.brand{font-weight:700;text-decoration:none;color:var(--ink)}
nav{display:flex;gap:1.1rem;flex-wrap:wrap}
nav a{color:var(--dim);text-decoration:none;font-size:.94rem}
nav a:hover,nav a.here{color:var(--ink)}
nav a.here{font-weight:600}
main{max-width:46rem;margin:0 auto;padding:2rem 1.5rem 4rem}
h1{font-size:2rem;line-height:1.2;margin:.4em 0 .5em}
h2{font-size:1.35rem;margin:2.2em 0 .6em;padding-top:.4em;border-top:1px solid var(--line)}
h3{font-size:1.1rem;margin:1.8em 0 .4em}
h4{font-size:1rem;margin:1.4em 0 .3em;color:var(--dim)}
p,li{margin:.7em 0}
a{color:var(--link)}
code{background:var(--code);padding:.12em .35em;border-radius:3px;font-size:.88em}
pre{background:var(--code);padding:.9rem 1.1rem;border-radius:5px;overflow-x:auto;
  border:1px solid var(--line)}
pre code{background:none;padding:0;font-size:.86rem;line-height:1.55}
table{border-collapse:collapse;width:100%;margin:1.2em 0;font-size:.94rem}
th,td{text-align:left;padding:.5rem .7rem;border-bottom:1px solid var(--line);vertical-align:top}
th{font-weight:600}
footer{border-top:1px solid var(--line);padding:1.5rem;color:var(--dim);font-size:.88rem}
footer p{max-width:46rem;margin:0 auto}
`
