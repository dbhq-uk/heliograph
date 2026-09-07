package site

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A diagram is an argument the page is making in a second medium. These check
// that the argument is actually there, and that it survives a reader who
// cannot see it.

var reDiagramFence = regexp.MustCompile("(?m)^```diagram\\s*(\\S*)\\s*$")

// Every name used in the content must exist. A missing one renders as visible
// text rather than vanishing, but visible text saying "missing diagram" is a
// thing to catch here, not in production.
func TestEveryDiagramReferencedExists(t *testing.T) {
	files, _ := filepath.Glob("../../site/content/*.md")
	if len(files) == 0 {
		t.Skip("no content here")
	}
	used := map[string]bool{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range reDiagramFence.FindAllStringSubmatch(string(b), -1) {
			name := m[1]
			used[name] = true
			if _, ok := Diagram(name); !ok {
				t.Errorf("%s references diagram %q, which does not exist", filepath.Base(f), name)
			}
		}
	}
	if len(used) == 0 {
		t.Fatal("no diagrams are used anywhere, so this test checked nothing")
	}
	// The other direction. An unused diagram is dead weight in the binary and,
	// worse, one nobody looks at when the thing it describes changes.
	for name := range diagrams {
		if !used[name] {
			t.Errorf("diagram %q is defined but used on no page", name)
		}
	}
}

// A diagram that says nothing to a screen reader is decoration sold as
// explanation. The title has to be the sentence the picture is making, which a
// length floor is a crude but effective proxy for: "the loop" would pass a
// presence check and help nobody.
func TestEveryDiagramIsDescribed(t *testing.T) {
	for name, svg := range diagrams {
		if !strings.Contains(svg, `role="img"`) {
			t.Errorf("%s: no role=img, so it is announced as a graphic with no purpose", name)
		}
		if !strings.Contains(svg, "aria-labelledby=") {
			t.Errorf("%s: nothing links the title to the image", name)
		}
		i := strings.Index(svg, "<title")
		if i < 0 {
			t.Errorf("%s: no <title>", name)
			continue
		}
		j := strings.Index(svg[i:], "</title>")
		title := svg[i : i+j]
		if k := strings.Index(title, ">"); k >= 0 {
			title = title[k+1:]
		}
		if len(title) < 60 {
			t.Errorf("%s: the title is %d characters, too short to be the sentence the picture makes: %q",
				name, len(title), title)
		}
	}
}

// The caption is prose and lives in the markdown, so the mirror an agent reads
// carries the meaning rather than a bare name.
func TestDiagramCaptionsAreInTheMarkdown(t *testing.T) {
	files, _ := filepath.Glob("../../site/content/*.md")
	if len(files) == 0 {
		t.Skip("no content here")
	}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		lines := strings.Split(string(b), "\n")
		for i, l := range lines {
			if !strings.HasPrefix(l, "```diagram") {
				continue
			}
			if i+1 >= len(lines) || len(strings.TrimSpace(lines[i+1])) < 30 {
				t.Errorf("%s:%d: the diagram has no caption, so the .md mirror says nothing",
					filepath.Base(f), i+1)
			}
		}
	}
}

func TestDiagramRendersAsAFigure(t *testing.T) {
	out := RenderBody("```diagram loop\nA caption long enough to be a real sentence about it.\n```")
	for _, want := range []string{"<figure", "<svg", "<figcaption>", "A caption long enough"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output is missing %q:\n%s", want, out[:min(400, len(out))])
		}
	}
}

// A name that does not exist must be loud. A diagram that silently vanishes is
// a hole in the page nobody notices until a reader asks what the picture was.
func TestUnknownDiagramIsVisible(t *testing.T) {
	out := RenderBody("```diagram nosuchthing\ncaption\n```")
	if !strings.Contains(out, "missing diagram") || !strings.Contains(out, "nosuchthing") {
		t.Errorf("an unknown diagram was not reported in the output:\n%s", out)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
