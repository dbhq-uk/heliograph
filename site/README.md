# site

The documentation site for `heliograph.dbhq.uk`.

**One canonical source, several renderings.** The 2026 consensus for developer
documentation is not separate content for humans and machines: it is one set of
pages served as HTML, as markdown, as `llms.txt`, and through an MCP server.
This follows it, with one exception that is real rather than convenient.

The exception is the **skill**. `skills/heliograph/references/method.md`
says "never truncate", "keep a control", "change one thing between runs". That
is not a description of the product, it is a procedure that changes what an
agent does, and it has no reader on a documentation site. It stays where it is.

## Why markdown mirrors

Measured across all 29 pages, the same page costs three to sixteen times more
bytes as HTML than as markdown - about eight times on the median page, six
times over the whole site - so chrome is a token tax on every agent that reads
the site. Every page is available at its own URL plus `.md`. A test measures
the ratio on every build, so the figure in this sentence and the site it
describes cannot drift apart.

`llms.txt` and `llms-full.txt` sit at the origin root. The honest position on
those: one log study found 408 requests to `llms.txt` out of more than 500
million AI bot visits in ninety days, so they are shipped because IDE agents
fetch them and it costs half a day, not because they will win citations.

## Analytics, and what a search result sees

The site reports into the **dbhq.uk GA4 property**, on the same web stream
(`G-3H3NFGSX85`), because Google's guidance is one stream per site including
its subdomains. Separate the docs in reports by the Hostname dimension. The tag
loads only after the consent banner is accepted and only on
`heliograph.dbhq.uk`, which is the promise the dbhq.uk privacy policy makes.
Without JavaScript there is no banner and no tag.

Titles and descriptions are hand-written in `cmd/heliograph-site/main.go`
(`titles`, `descriptions`), and a test holds every description to 70 to 160
characters. The sitemap's `lastmod` is the last commit that touched each page,
so the build refuses a shallow clone. The Open Graph image is rendered from
`site/og/og.html` by `site/og/render.sh` and committed as `assets/og.png`;
re-run the script after changing the template.

Why each of these exists, and what it cost:
[`docs/specs/2026-09-09-analytics-and-seo-design.md`](../docs/specs/2026-09-09-analytics-and-seo-design.md).
