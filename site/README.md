# site

The documentation site for `heliograph.dbhq.uk`.

**One canonical source, several renderings.** The 2026 consensus for developer
documentation is not separate content for humans and machines: it is one set of
pages served as HTML, as markdown, as `llms.txt`, and through an MCP server.
This follows it, with one exception that is real rather than convenient.

The exception is the **skill**. `references/method.md` in `heliograph-skill`
says "never truncate", "keep a control", "change one thing between runs". That
is not a description of the product, it is a procedure that changes what an
agent does, and it has no reader on a documentation site. It stays where it is.

## Why markdown mirrors

The same page costs roughly 31 times more bytes as HTML than as markdown, so
chrome is a token tax on every agent that reads the site. Every page is
available at its own URL plus `.md`.

`llms.txt` and `llms-full.txt` sit at the origin root. The honest position on
those: one log study found 408 requests to `llms.txt` out of more than 500
million AI bot visits in ninety days, so they are shipped because IDE agents
fetch them and it costs half a day, not because they will win citations.
