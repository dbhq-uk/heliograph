package site

import (
	"strings"
	"testing"
)

func TestHeadingsCarryLinkableIDs(t *testing.T) {
	// A docs page whose sections cannot be linked to is one nobody can cite in
	// an incident, which is when these pages actually get read.
	got := RenderBody("## Two writers, one branch\n")
	if !strings.Contains(got, `id="two-writers-one-branch"`) {
		t.Errorf("no usable id: %s", got)
	}
}

func TestParagraphsJoinWrappedLines(t *testing.T) {
	// Markdown joins consecutive lines. Rendering each as its own paragraph
	// would break every sentence that happens to wrap, which is all of them.
	got := RenderBody("one line\nand its continuation\n\na second paragraph\n")
	if !strings.Contains(got, "<p>one line and its continuation</p>") {
		t.Errorf("wrapped lines were not joined:\n%s", got)
	}
	if strings.Count(got, "<p>") != 2 {
		t.Errorf("expected two paragraphs:\n%s", got)
	}
}

func TestCodeBlocksAreEscapedAndNotRendered(t *testing.T) {
	// A code block is the one place the content must appear exactly as written.
	got := RenderBody("```\n<script>alert(1)</script>\n**not bold**\n```\n")
	if strings.Contains(got, "<script>") {
		t.Errorf("a script tag survived into the output:\n%s", got)
	}
	if strings.Contains(got, "<strong>") {
		t.Errorf("markdown was rendered inside a code block:\n%s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("the code block lost its content:\n%s", got)
	}
}

// Content is escaped before any tag is introduced. Escaping afterwards would
// escape our own tags; escaping only some of it is how an injection gets in.
func TestContentCannotInjectMarkup(t *testing.T) {
	for _, in := range []string{
		`a <script>alert(1)</script> b`,
		`an <img src=x onerror=alert(1)> image`,
		`**<b>bold</b>**`,
	} {
		got := RenderBody(in + "\n")
		if strings.Contains(got, "<script") || strings.Contains(got, "<img") ||
			strings.Contains(got, "<b>") {
			t.Errorf("%q rendered raw markup:\n%s", in, got)
		}
	}
}

// A javascript: or data: URL in a docs page is either a mistake or an attack,
// and neither should render as a link.
func TestOnlySafeSchemesBecomeLinks(t *testing.T) {
	safe := RenderBody("see [the docs](https://heliograph.dbhq.uk/cli)\n")
	if !strings.Contains(safe, `<a href="https://heliograph.dbhq.uk/cli">the docs</a>`) {
		t.Errorf("an https link did not render:\n%s", safe)
	}
	rel := RenderBody("see [transports](/transports)\n")
	if !strings.Contains(rel, `<a href="/transports">transports</a>`) {
		t.Errorf("a relative link did not render:\n%s", rel)
	}
	for _, bad := range []string{
		"[x](javascript:alert(1))",
		"[x](data:text/html,<script>alert(1)</script>)",
	} {
		got := RenderBody(bad + "\n")
		if strings.Contains(got, "<a href") {
			t.Errorf("%q became a link:\n%s", bad, got)
		}
	}
}

func TestTablesRender(t *testing.T) {
	got := RenderBody("| a | b |\n|---|---|\n| 1 | 2 |\n")
	for _, want := range []string{"<table>", "<th>a</th>", "<td>1</td>", "</table>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	// The |---|---| row carries no content and must not become a row.
	if strings.Contains(got, "<td>---</td>") {
		t.Errorf("the separator row was rendered:\n%s", got)
	}
}

func TestListsRenderAndClose(t *testing.T) {
	got := RenderBody("- one\n- two\n\nafter\n")
	if strings.Count(got, "<li>") != 2 {
		t.Errorf("expected two items:\n%s", got)
	}
	if !strings.Contains(got, "</ul>") {
		t.Errorf("the list was not closed:\n%s", got)
	}
	if strings.Contains(got, "<li>after</li>") {
		t.Errorf("the paragraph after the list was swallowed into it:\n%s", got)
	}
}

func TestTitleIsTheFirstH1(t *testing.T) {
	if got := Title("# heliograph\n\n## not this\n"); got != "heliograph" {
		t.Errorf("got %q", got)
	}
	if got := Title("## only an h2\n"); got != "" {
		t.Errorf("an h2 was taken as the title: %q", got)
	}
}

func TestSummaryIsTheFirstParagraphAfterTheTitle(t *testing.T) {
	md := "# Install\n\nA single static binary, no runtime.\nIt wraps.\n\nSecond para.\n"
	got := Summary(md)
	if got != "A single static binary, no runtime. It wraps." {
		t.Errorf("got %q", got)
	}
}

func TestSummaryStripsEmphasis(t *testing.T) {
	// It goes into llms.txt and a meta description, where markdown is noise.
	got := Summary("# T\n\n**Bold** and `code` and [a link](https://x).\n")
	if strings.ContainsAny(got, "*`[]") {
		t.Errorf("markdown survived into the summary: %q", got)
	}
}

func TestEmptyInputIsNotAPanic(t *testing.T) {
	for _, in := range []string{"", "\n", "```\n", "|\n"} {
		_ = RenderBody(in)
		_ = Title(in)
		_ = Summary(in)
	}
}

// A page whose first content is a code block had its whole synopsis rendered as
// a description, and that went into llms.txt - the one place a wrong
// description is read by something that cannot tell it is wrong.
func TestSummarySkipsACodeBlockEntirely(t *testing.T) {
	md := "# CLI reference\n\n```\nheliograph init\nheliograph send\n```\n\nEvery command takes an estate.\n"
	got := Summary(md)
	if strings.Contains(got, "heliograph init") {
		t.Errorf("the code block became the description: %q", got)
	}
	if got != "Every command takes an estate." {
		t.Errorf("got %q", got)
	}
}
