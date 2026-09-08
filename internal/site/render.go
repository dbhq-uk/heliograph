// Package site renders the documentation from markdown.
//
// A markdown subset rather than a dependency. The content is ours, the subset
// is what the content uses, and a build that pulls in a JavaScript toolchain to
// produce six static pages would be a strange thing to find in a repository
// whose sibling promises no dependencies at all.
//
// It emits three renderings of one source, which is the 2026 consensus for
// developer documentation: HTML for people, a `.md` mirror at the same path for
// agents, and `llms.txt` at the root. The same page costs roughly 31 times more
// bytes as HTML than as markdown, so serving chrome to an agent is a token tax
// on every read.
package site

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// Page is one source document.
type Page struct {
	Slug  string // "install"; "index" is the root
	Title string // the first H1
	Body  string // markdown
}

var (
	reHeading  = regexp.MustCompile(`^(#{1,4})\s+(.*)$`)
	reFence    = regexp.MustCompile("^```")
	reBullet   = regexp.MustCompile(`^[-*]\s+(.*)$`)
	reTableRow = regexp.MustCompile(`^\|(.*)\|$`)
	reTableSep = regexp.MustCompile(`^\|[\s:|-]+\|$`)
	reBold     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItalic   = regexp.MustCompile(`\*([^*]+)\*`)
	reCode     = regexp.MustCompile("`([^`]+)`")
	reLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

// Title extracts the first H1, which is what the page is called everywhere else.
func Title(md string) string {
	for _, l := range strings.Split(md, "\n") {
		if m := reHeading.FindStringSubmatch(l); m != nil && len(m[1]) == 1 {
			return stripInline(m[2])
		}
	}
	return ""
}

// stripInline removes markdown emphasis for use in a title or a summary.
func stripInline(s string) string {
	s = reBold.ReplaceAllString(s, "$1")
	s = reItalic.ReplaceAllString(s, "$1")
	s = reCode.ReplaceAllString(s, "$1")
	s = reLink.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

// inline renders emphasis, code and links.
//
// Escaped BEFORE any tag is introduced, so content can never inject markup. The
// order matters: escaping afterwards would escape our own tags, and escaping
// only some of it is how an injection gets in.
func inline(s string) string {
	s = html.EscapeString(s)
	s = reCode.ReplaceAllString(s, "<code>$1</code>")
	s = reBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = reItalic.ReplaceAllString(s, "<em>$1</em>")
	s = reLink.ReplaceAllStringFunc(s, func(m string) string {
		p := reLink.FindStringSubmatch(m)
		href := p[2]
		// Only http, https and relative links. A javascript: or data: URL in a
		// docs page is either a mistake or an attack, and neither should render.
		if strings.Contains(href, ":") &&
			!strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
			return p[1]
		}
		return fmt.Sprintf(`<a href="%s">%s</a>`, href, p[1])
	})
	return s
}

func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(stripInline(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// RenderBody turns the markdown subset into HTML.
func RenderBody(md string) string {
	var out strings.Builder
	lines := strings.Split(md, "\n")

	inCode, inList, inTable := false, false, false
	closeBlocks := func() {
		if inList {
			out.WriteString("</ul>\n")
			inList = false
		}
		if inTable {
			out.WriteString("</tbody></table>\n")
			inTable = false
		}
	}

	for i := 0; i < len(lines); i++ {
		l := lines[i]

		if reFence.MatchString(l) {
			closeBlocks()
			// ```diagram fences hold a name, not code. The picture lives in Go
			// so it can inherit the theme's colours; the markdown keeps a name
			// a person can read, and the mirror keeps the caption rather than
			// four hundred bytes of path data.
			if info := strings.TrimSpace(strings.TrimPrefix(l, "```")); !inCode &&
				strings.HasPrefix(info, "diagram") {
				name := strings.TrimSpace(strings.TrimPrefix(info, "diagram"))
				var body []string
				for i+1 < len(lines) && !reFence.MatchString(lines[i+1]) {
					i++
					body = append(body, strings.TrimSpace(lines[i]))
				}
				i++ // the closing fence
				caption := strings.TrimSpace(strings.Join(body, " "))
				svg, ok := Diagram(name)
				if !ok {
					// Rendered as visible text rather than dropped. A diagram
					// that silently vanishes is a hole in the page nobody
					// notices until a reader asks what the picture was.
					fmt.Fprintf(&out, "<p class=\"missing\">missing diagram: %s</p>\n",
						html.EscapeString(name))
					continue
				}
				fmt.Fprintf(&out, "<figure class=\"dgw\">%s<figcaption>%s</figcaption></figure>\n",
					svg, inline(caption))
				continue
			}
			if inCode {
				out.WriteString("</code></pre>\n")
			} else {
				out.WriteString("<pre><code>")
			}
			inCode = !inCode
			continue
		}
		if inCode {
			// Escaped, never rendered. A code block is the one place where the
			// content is meant to be seen exactly as written.
			out.WriteString(html.EscapeString(l) + "\n")
			continue
		}

		if m := reHeading.FindStringSubmatch(l); m != nil {
			closeBlocks()
			n := len(m[1])
			id := slugify(m[2])
			// A heading id is what a deep link points at, and a docs page whose
			// sections cannot be linked to is one nobody can cite in an incident.
			fmt.Fprintf(&out, "<h%d id=%q>%s</h%d>\n", n, id, inline(m[2]), n)
			continue
		}

		if reTableSep.MatchString(l) {
			continue // the |---|---| row carries no content
		}
		if m := reTableRow.FindStringSubmatch(l); m != nil {
			cells := strings.Split(m[1], "|")
			if !inTable {
				closeBlocks()
				out.WriteString("<table><thead><tr>")
				for _, c := range cells {
					fmt.Fprintf(&out, "<th>%s</th>", inline(strings.TrimSpace(c)))
				}
				out.WriteString("</tr></thead><tbody>\n")
				inTable = true
				continue
			}
			out.WriteString("<tr>")
			for _, c := range cells {
				fmt.Fprintf(&out, "<td>%s</td>", inline(strings.TrimSpace(c)))
			}
			out.WriteString("</tr>\n")
			continue
		}
		if inTable && strings.TrimSpace(l) == "" {
			closeBlocks()
			continue
		}

		if m := reBullet.FindStringSubmatch(l); m != nil {
			if !inList {
				closeBlocks()
				out.WriteString("<ul>\n")
				inList = true
			}
			fmt.Fprintf(&out, "<li>%s</li>\n", inline(m[1]))
			continue
		}

		if strings.TrimSpace(l) == "" {
			closeBlocks()
			continue
		}

		// A paragraph runs to the next blank line. Markdown joins consecutive
		// lines, and rendering each as its own paragraph would break every
		// sentence that happens to wrap.
		var para []string
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" &&
			!reFence.MatchString(lines[i]) && !reHeading.MatchString(lines[i]) &&
			!reBullet.MatchString(lines[i]) && !reTableRow.MatchString(lines[i]) {
			para = append(para, strings.TrimSpace(lines[i]))
			i++
		}
		i--
		closeBlocks()
		fmt.Fprintf(&out, "<p>%s</p>\n", inline(strings.Join(para, " ")))
	}
	closeBlocks()
	if inCode {
		out.WriteString("</code></pre>\n")
	}
	return out.String()
}

// Summary is the first PROSE paragraph after the title.
//
// Code blocks are skipped entirely rather than merely not started in. The first
// version only refused to begin inside a fence, so a page whose first content
// was a code block had its whole synopsis rendered as a description - and that
// went into llms.txt, which is the one place a wrong description is read by
// something that cannot tell it is wrong.
func Summary(md string) string {
	lines := strings.Split(md, "\n")
	seenH1, inCode := false, false
	for i := 0; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if reFence.MatchString(l) {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		if reHeading.MatchString(l) {
			seenH1 = true
			continue
		}
		if !seenH1 || l == "" {
			continue
		}
		var para []string
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" &&
			!reFence.MatchString(strings.TrimSpace(lines[i])) {
			para = append(para, strings.TrimSpace(lines[i]))
			i++
		}
		return stripInline(strings.Join(para, " "))
	}
	return ""
}

// Mark is the logo, inlined.
//
// Inline rather than an <img> because it is in the sticky header on every page
// and inherits currentColor, so it cannot fall out of step with the palette and
// costs no request.
// The glyph is the product in one line: a solid disc (the mirror, the side
// you are on), a dash and a dot (the flash crossing the gap), and an open
// ring (the far side - open because you cannot get into it). Keep it in step
// with site/assets/mark.svg.
const Mark = `<svg viewBox="0 0 64 64" aria-hidden="true" focusable="false">` +
	`<circle cx="16" cy="47" r="9.5" fill="currentColor"/>` +
	`<path d="M28 35 L34 29" stroke="currentColor" stroke-width="5.5" stroke-linecap="round"/>` +
	`<circle cx="40.5" cy="22.5" r="2.9" fill="currentColor"/>` +
	`<circle cx="51" cy="12" r="6" fill="none" stroke="currentColor" stroke-width="4.6" opacity=".8"/></svg>`

// Headings returns the H2s of a body, in order, as (id, text) pairs.
//
// For the "on this page" rail. Only H2: an H3 rail on a page with several of
// them becomes a second navigation competing with the first, and the reader
// then has two lists and no hierarchy.
func Headings(md string) [][2]string {
	var out [][2]string
	inFence := false
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		// A `## ` inside a fence is shell output or a comment, not a heading.
		// Without this check the rail fills with lines nobody can jump to.
		if inFence || !strings.HasPrefix(line, "## ") {
			continue
		}
		text := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		out = append(out, [2]string{slugify(text), stripInline(text)})
	}
	return out
}
