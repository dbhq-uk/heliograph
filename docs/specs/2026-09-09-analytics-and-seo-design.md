# Analytics and search: heliograph.dbhq.uk from day one

Written 2026-09-09. The site is 24 pages, deployed from `main` to GitHub Pages
under a custom domain. This records how it is measured, how search engines are
told about it, and what was deliberately not done.

## Google Analytics: the same property, the same stream

The question was whether `heliograph.dbhq.uk` needs its own GA4 property or
data stream. It needs neither.

The `dbhq.uk` property (`544327698`) has one web data stream, `DBHQ`,
measurement ID `G-3H3NFGSX85`. Google's own guidance is one web stream per
site including its subdomains: the `_ga` cookie is written at the registrable
domain, so a reader who moves between `dbhq.uk` and `heliograph.dbhq.uk` is
one user rather than two, and nothing has to be configured for that. A second
stream in the same property is the documented way to double-count. A second
property would keep the two sites' readers apart, which is the opposite of
what a company site and its product site want.

So the site carries the same tag, and the docs are separated in reports by the
**Hostname** dimension. Nothing was created or changed in the GA4 account.

**Consent.** The tag is loaded exactly as `dbhq.uk` loads it, and for the same
reason: the dbhq.uk privacy policy promises that analytics "loads only after
you accept via the cookie banner". Consent Mode defaults every storage type to
denied, `gtag/js` is not fetched until a reader accepts, the choice is kept in
`localStorage` under `dbhq-consent`, and Escape or dismiss counts as decline.
The tag also refuses to load anywhere but the production hostname, so a
preview or a local build never reports.

One difference from dbhq.uk: the banner here is **not modal**. A developer who
lands on `/install` from a search result wants the command, and a modal in
front of it costs more readers than the measurement is worth. The banner sits
at the bottom of the viewport and the page stays usable. Without JavaScript
there is no banner and no tag, which is the correct outcome for the agents and
`curl` that are most of this site's readers.

## Search Console: already covered

`sc-domain:dbhq.uk` is a Domain property, which covers every subdomain. The
service account already granted on it can submit sitemaps and inspect URLs, so
`https://heliograph.dbhq.uk/sitemap.xml` is submitted through the API rather
than by hand, and the submission is recorded in `PLAN.md`.

## On-page: what was missing, and what was wrong

The generator already emitted a sitemap, `robots.txt`, canonical URLs, Open
Graph tags and `llms.txt`. The audit found:

| Defect | Fix |
|---|---|
| Every page preloaded two font files that do not exist (`InstrumentSerif-400`, `InstrumentSans`): two 404s per page view, since the fonts were renamed to Archivo and JetBrains Mono | Preload the fonts the CSS actually uses, and a test that every asset the head references is a file in `site/assets` |
| No `lastmod` in the sitemap. The dbhq.uk audit of 2026-08-09 traced a page Google never crawled to exactly this | `lastmod` from the last commit that touched the page. A shallow checkout gives every page the deploy date, which is worse than none, so the build **refuses** a shallow clone and says to set `fetch-depth: 0` |
| Meta descriptions were the first paragraph, verbatim: seven pages over 200 characters, two under 40, and `/windows` opened with "Two different questions get confused here" | A `descriptions` map beside `titles`, written for the search result, and a test that every one is 70 to 160 characters. `llms.txt` keeps the first paragraph, which is the right summary for an agent |
| No `og:image`, so a shared link rendered as bare text | A 1200x630 image rendered from `site/og/og.html` with headless Chrome by `site/og/render.sh`, committed as `site/assets/og.png`, and `twitter:card` promoted to `summary_large_image` |
| No structured data | JSON-LD: `SoftwareApplication` and `Organization` on the home page, `TechArticle` with a `BreadcrumbList` on every docs page. Built with `encoding/json`, which escapes `<` and so cannot be broken out of by page content |
| No `404.html`, so a missing path served GitHub's default page with no way back into the docs | A 404 page in the docs shell, marked `noindex` |

Titles were already hand-written for the search result. The keyword research
below adjusts a handful.

## The baseline, measured before any of this shipped

Lighthouse through the PageSpeed Insights API on 2026-09-09, live site:

| Page | Performance | SEO | Accessibility | Best practices |
|---|---|---|---|---|
| `/` mobile | 98 | 100 | 100 | 96 |
| `/` desktop | 100 | 100 | 100 | 96 |
| `/install` mobile | 100 | 100 | 100 | 96 |
| `/install` desktop | 100 | 100 | 100 | 96 |

The four points off best practices are "errors in console", which are the
two font preloads that 404. Search Console for the same day: zero impressions
and zero queries for any `heliograph.dbhq.uk` page in the previous 90 days,
the home page "unknown to Google", and `/install` "discovered, currently not
indexed". The site had never been crawled, so nothing here is a regression to
measure against; it is the floor.

## Keyword research

Done with the DataForSEO API against UK and US Google, under a hard budget of
USD 3, with every call logged to the shared ledger. The report is
[`../seo/2026-09-09-keyword-research.md`](../seo/2026-09-09-keyword-research.md)
and the raw responses are in `~/dbhq-uk/dbhq-seo/reports/heliograph/`. Its
target-keyword table names which page owns each query, and that is what the
titles and descriptions follow.

## Tests, and which ones were watched to fail

All in `cmd/heliograph-site/main_test.go`, each broken deliberately once
before being kept:

- every `/assets/` reference in a built page is a file that exists
- the sitemap carries a `lastmod` in `YYYY-MM-DD` form for every URL
- every meta description is 70 to 160 characters, every title at most 70
- the head sets Consent Mode to denied before any `config` call, and only
  loads the tag for the production hostname
- the JSON-LD on every page parses as JSON
- `404.html` exists, is `noindex`, and carries the sidebar

## Deliberately not done

- **A second GA4 property or stream.** Above.
- **Google Tag Manager.** One tag, loaded on consent, is the whole
  requirement. A container would add a third party to a site that has none.
- **Bing Webmaster, IndexNow.** dbhq.uk has neither. Add both together when
  there is a reason.
- **`hreflang`, RSS.** One language, and no feed to announce.
- **A cookie banner for agents.** They do not run JavaScript, so there is no
  banner and no tag, and nothing to consent to.

## Recommended, in another repository

The dbhq.uk privacy policy says "personal data we collect through dbhq.uk".
It should say "through dbhq.uk and its subdomains, such as
heliograph.dbhq.uk", in `~/dbhq/website/src/pages/privacy.astro`. That is a
legal document and a separate deploy, so it is recorded here rather than
changed.
