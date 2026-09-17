# Licensing

| | |
|---|---|
| **heliograph** - CLI, station payloads, wire format, transports | [Apache 2.0](https://github.com/heliograph-io/heliograph/blob/main/LICENSE) |
| **This documentation**, and everything under `site/content/` | [CC BY 4.0](https://github.com/heliograph-io/heliograph/blob/main/site/content/LICENSE) |
| **heliograph-relay** | [FSL-1.1-ALv2](https://github.com/heliograph-io/heliograph-relay/blob/main/LICENSE), fair source, converting to Apache 2.0 two years after each release |
| **Heliograph Cloud** | proprietary. Its documentation is public and CC BY 4.0 |

All of that landed on **17 September 2026**. Before that date both repositories
were MIT. This page is the announcement and the reference, because a licence
post that is only an announcement goes stale and a reference with no date
leaves people guessing what applied when.

## Nothing was taken away

**Every commit published under MIT is still available under MIT, permanently.**
That is `b689f9c` and earlier in `heliograph`, `f664ea0` and earlier in
`heliograph-relay`, including the relay's `v0.1.0` and `v0.1.1` tags. Fork any
of them and MIT is what you have, for ever. The `NOTICE` file in each
repository says so rather than leaving you to work it out.

A licence change cannot reach backwards. Anybody telling you otherwise about
any project is wrong about how copyright licences work.

**And nothing was relicensed over anybody's head.** Both repositories have one
author, under a handful of addresses, plus a CI identity and dependabot
(`git shortlog -sne --all`). There was no contributor whose permission was
needed and not obtained.

## Why Apache 2.0 rather than MIT

MIT has no patent language, so patent rights are at best implied. Apache grants
them expressly in section 3, and terminates the grant for a contributor who
turns round and sues over a patent. That is a question an enterprise legal team
asks about an infrastructure dependency, and it gets sharper as the beam pulls
in Noise, ICE and WebRTC.

Two smaller reasons point the same way. Section 6 states that the licence
conveys no trademark rights, which is where the actual boundary sits. Section
4(b) obliges a fork to say it modified the work.

Permissive to permissive, one copyright holder, and **everybody gains a patent
grant they did not have before**. It is the least controversial relicence
available.

## Why the relay is fair source rather than open source

The relay is the component in the data path. Its source stays published and
checkable, because "the code carrying your bytes is code you can read" is the
strongest thing this project says and it has to remain true.

What FSL stops is **Competing Use**: making the software available to others in
a commercial product or service that substitutes for it. Concretely, selling
relay hosting. Hosted relay is a tier we sell, so somebody selling it from our
own source is the one case the licence exists for.

**Calling it open source would be false**, and so would calling it
source-available-and-nothing-more. Each release converts to Apache 2.0 two
years after it ships, on a rolling clock, so a two-year-old relay is a fully
open-source relay.

### Professional services are expressly permitted

FSL's own Permitted Purposes cover professional services provided "to a
licensee using the Software". That requires your client to be a licensee in
their own right, which is narrower than what is intended here, so the copyright
holder grants this in addition, irrevocably, in the relay's
[`NOTICE`](https://github.com/heliograph-io/heliograph-relay/blob/main/NOTICE):

> Deploying the relay to reach estates you or your clients operate, as part of
> professional services you provide, provided you do not offer relay hosting
> itself as a hosted or subscription service to third parties.

Run a relay so you can do a client's work: fine, and always was. Sell relay
hosting: that is the competing use.

### The honest limit

FSL's protection is worth whatever release velocity makes it worth. That cuts
badly for this component, because the relay is **deliberately** stable - a
two-year-old relay is still a good relay, so the conversion clock hands a
competitor something useful.

That is accepted rather than solved, and it is written down here rather than
left in a design document. By the time somebody has a two-year-old beacon
relay, the value has moved to the beam, the governance plane, and operating the
thing.

### Why not AGPL

It does not do the job. Section 13 binds somebody hosting a **modified** copy,
so a competitor running this relay unmodified would owe nothing and could
charge for it. AGPL is reciprocity, not protection against competing hosting.

It is also prohibited outright at a good number of the organisations this is
built for, and section 13 has no case law to lean on.

## What did not change

**No shape is ever gated.** Beacon, flare and beam all work on a relay you host
yourself and on the hosted one. Nothing that crosses the gap moves behind a
paywall. What is sold above that is many-to-many orchestration.

**Nothing is installed on the far side.** Stations stay plain bash and plain
PowerShell, planted as source, readable before they are run.

**The documentation takes pull requests**, including the documentation for the
proprietary service. The [security](/security) page is the one a buyer reads
adversarially, and its whole argument is that each claim is checkable without
trusting us. A security page whose source is private and accepts no corrections
would be weaker for no gain.

## Contributing

Inbound equals outbound. **There is no CLA and no copyright assignment** - you
keep your copyright, and the project gets the same licence everybody else gets.
Sign your commits off with the
[Developer Certificate of Origin](https://developercertificate.org/), which is
`git commit -s`.

FSL has no copyleft, so nothing about keeping the hosted service private
depends on taking anybody's rights away.

## Where the boundary is written down

The [security](/security) page states what each component can and cannot do,
and [provenance](/provenance) is the command for checking the binary you are
holding is the source you read. `v0.4.2` was the first signed release and every
release since carries a Sigstore bundle that verifies.

Third-party code vendored into these repositories keeps its own licence and its
own notice, and this change does not touch any of it.
