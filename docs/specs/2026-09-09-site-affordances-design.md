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
