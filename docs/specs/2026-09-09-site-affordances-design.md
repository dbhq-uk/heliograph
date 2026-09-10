# The docs affordances, measured against paseo.sh

Written 2026-09-09, after the analytics and SEO work landed. That pass made
the site findable. This one is about what a reader does once they arrive, and
it was scoped by comparing the site against
[paseo.sh](https://paseo.sh), a documentation site for the same audience,
built by people who thought about it.

## What the comparison found

**Where heliograph was already ahead**, and nothing was done:

| | |
|---|---|
| `robots.txt` | paseo.sh returns its 404 page for `/robots.txt` |
| structured data | paseo.sh emits no JSON-LD at all |
| a skip link | it has none; this site has had one since it was built |
| markdown mirrors | both ship them, and `llms.txt` |
| sitemap `lastmod` | both carry one on every URL |

**Where it was behind**, and what was adopted:

| Missing | Now |
|---|---|
| a copy button on a code block. This site is a list of commands to run on somebody else's machine, and selecting one by hand out of a `<pre>` is exactly where a stray character enters a step | Every code block carries one. Hidden without JavaScript, always visible on touch, which has no hover |
| the markdown mirror was a `<link>` in the head and one line in the footer. An agent finds it there; a person driving one never scrolls that far | **Copy as markdown** and **View as markdown**, above the title. The copy control fetches the page's own `.md` rather than scraping the DOM back into markdown: the mirror is byte for byte the source, and the DOM is not |
| no visible breadcrumb, while the JSON-LD had claimed a `BreadcrumbList` since the SEO work. A crawler was being told about navigation that nothing on the page showed | A breadcrumb above the title, saying what the JSON-LD says |
| the "on this page" rail never changed while the page moved under it | The current section is marked, through an `IntersectionObserver` rather than a scroll handler |
| a favicon only as SVG | `favicon.ico` at three sizes and a 180px `apple-touch-icon.png`, both rendered from the same SVG |
| the links to the repository were the word "Source" | They carry the GitHub mark, in the header, the hero and the footer. Links inside a page's prose are left alone: a mark mid-sentence is noise, and the sentence already says where it goes |

**Deliberately not adopted:**

- **A star count in the header.** paseo.sh shows one. It costs a request to
  the GitHub API on every page load, or a build-time number that is wrong the
  next day, and this site has no third-party requests at all.
- **A Discord icon.** There is no Discord.
- **Per-platform download buttons with OS detection.** The install page is
  three commands and covers six platforms in one line each.

## What it cost

Five tests in `cmd/heliograph-site/main_test.go`, each broken deliberately
and watched to fail: every code block has exactly one copy button, every docs
page carries the two markdown controls above its title, every docs page shows
the breadcrumb its JSON-LD claims, the head references all three favicons, and
the site's own links to the repository carry the mark while prose links do not.

The copy controls were then proved in a real browser against the built site:
the code button puts the block's exact text on the clipboard, and the markdown
button puts the 2,244 bytes of `install.md` on it. A test that asserts a
button exists is not a test that it copies anything.

## The DBHQ menu, and what it replaced (added later the same day)

PR #45 put an "Also from DBHQ" list in the footer: three links, on all 27
pages, at the point a reader has already stopped reading. It is gone. In its
place:

- **A `DBHQ` menu in the home header**, a `<details>` so it opens with no
  JavaScript and gets its keyboard behaviour from the browser. The script only
  closes it on an outside click or Escape, which is the one thing `<details>`
  does not do and whose absence reads as broken. It names two siblings and the
  skills, then points at the page.
- **`/dbhq`**, a page: the sites you can open now, the skills marketplace with
  its install command, the two free browser tools, and the company. It links
  to `dbhq.uk/skills` rather than copying that list, because a copy of
  twenty-odd skills would be wrong within the week and nothing here could
  catch it.
- **A `More from DBHQ` group in every docs sidebar**, since docs pages carry
  no header.

The footer byline went with it. Attribution did not: the JSON-LD on every page
still names DBHQ as author and publisher.

**A defect found while checking the new page.** `| | |` is this repository's
idiom for a table with no header, used on ten pages. It matches the
`|---|---|` separator pattern, so it was skipped, and the first row of *data*
was promoted into `<thead>` - rendered in small caps by the CSS, and no longer
data to a screen reader. `/index`'s "control, transport, station" table
shipped like that. Fixed in `RenderBody`, with a test for both shapes.

## What only a browser found (2026-09-10)

The site gained client-side navigation between these two passes. It replaces
`main` and the rail wholesale, so the rail's `IntersectionObserver` was left
watching headings that had left the document: **the rail stopped marking the
current section after one soft navigation**, on every page, while every page
was correct on a hard load. No test in this repository could have seen it, and
none did.

`swap()` now announces with an `hg:swap` event and the rail rebinds on it,
disconnecting its previous observer first so they cannot stack. The copy
controls were checked at the same time and need nothing: they are delegated
from `document`, so replacing the page under them changes nothing. Proved
across two swaps and the back button.
