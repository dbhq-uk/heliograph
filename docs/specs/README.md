# Specifications

Written before the code, and reviewed on their own. Each one records the
decisions and, more usefully, what they cost.

| | |
|---|---|
| [`2026-08-17-running-the-agent-anywhere-design.md`](2026-08-17-running-the-agent-anywhere-design.md) | hosting the loop somewhere other than a person's terminal |
| [`2026-08-31-default-safe-execution-design.md`](2026-08-31-default-safe-execution-design.md) | read-only by default, and every gate failing closed |
| [`2026-09-03-intercom-design.md`](2026-09-03-intercom-design.md) | the HTTP transport, for the rarer case where you can reach the station |
| [`2026-09-06-heliograph-next-design.md`](2026-09-06-heliograph-next-design.md) | the master design: components, transports, roadmap |
| [`2026-09-06-relay-encryption-design.md`](2026-09-06-relay-encryption-design.md) | C1: what a relay operator can and cannot do |
| [`2026-09-08-station-and-skill-merge.md`](2026-09-08-station-and-skill-merge.md) | why the two repositories became one, and where the boundary went instead |
| [`2026-09-08-powershell-station-and-full-documentation-design.md`](2026-09-08-powershell-station-and-full-documentation-design.md) | A8: the pure PowerShell station, the transport contract's missing verb, and publishing the far side |
| [`2026-09-09-analytics-and-seo-design.md`](2026-09-09-analytics-and-seo-design.md) | GA4 on the same stream as dbhq.uk, Search Console already covered, and the on-page defects the audit found |
| [`2026-09-08-one-repo-many-stations-design.md`](2026-09-08-one-repo-many-stations-design.md) | one repository, many stations: the branch is already the channel, and the blast-radius rule that bounds it |

| [`2026-09-09-site-affordances-design.md`](2026-09-09-site-affordances-design.md) | what a reader does once they arrive: copy buttons, the markdown mirror where a person can reach it, and what paseo.sh had that this did not |

The master design and the relay spec were written in `heliograph-skill` and
moved here when the two-repo split was decided. Their git history lives in that
repository.

Plans live in [`../plans/`](../plans/): a spec says what and why, a plan says in
what order and in how many pieces.
