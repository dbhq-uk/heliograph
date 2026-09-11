# Keyword research for heliograph.dbhq.uk

**Date:** 2026-09-09
**Source:** DataForSEO REST API, live endpoints only. Google Ads volumes
worldwide (English, no location), United States (2840) and United Kingdom
(2826). Keyword difficulty and intent from DataForSEO Labs (US). Country
split from DataForSEO clickstream data.
**Raw data:** `~/dbhq-uk/dbhq-seo/reports/heliograph/raw/` (38 JSON files).
**Cost:** USD 1.0937 across 38 calls. See section 6.
**Read section 7 first.** The volumes and SERPs below are a measurement taken
on one day and are left as they were taken. Where a later finding changed what
to do about one, section 7 says so.

## 1. Summary

Nobody searches for the problem heliograph solves in the words heliograph
uses. Every problem-phrased seed ("run commands on a server without ssh",
"no ssh access to production server", "air gapped debugging", "remote
execution audit log", "timestamp every line of output bash") has no recorded
volume in any market. The "without ssh" phrasing exists only as "ssh without
password". People who cannot log in do not search for a tool. They search for
the thing they can log in with, or for the agent they are already using.

The demand that exists sits in three places. First, the AI-agent cluster:
"claude code skills" (60,500 worldwide, 12,100 US, 1,900 UK), "claude code
mcp server" (2,900, difficulty 3), "claude code ssh" (1,300), "codex cli
skills" (720, difficulty 1) and "codex cli mcp" (1,600, difficulty 5).
heliograph ships as exactly these things, so this is the cluster it can
honestly win, and the difficulty scores say the SERPs are soft. Second, the
air-gap cluster: "air gapped environment" (880), "air gapped server" (170),
"air gapped deployment" (170). heliograph fits, but the site has no page for
it yet. Third, the incumbents: "aws ssm run command", "aws ssm send-command"
and "azure run command" (140 to 320 each), which are navigational to vendor
docs and only reachable through a comparison page.

Demand is spread, not US-concentrated. Worldwide English volume is about five
times US volume across the board. For the agent terms the clickstream split
puts Japan first (18 to 48 per cent of searches for "claude code skills",
"claude code mcp", "claude code sandbox", "codex cli mcp" and "codex cli
skills"), then the United States (11 to 24 per cent), then Germany and India
(8 to 15 per cent each). The UK is 3 to 4 per cent. For the air-gap terms
India leads (36 to 50 per cent). Japan and Germany are large enough to
matter, but the queries themselves are English strings, so the data supports
an English-only site with good international reach rather than translation.
Revisit once Search Console shows whether Japanese impressions convert to
clicks.

The poor fits are clear. "remote command execution" means RCE to Google: the
SERP is Cloudflare, CrowdStrike, Imperva and Rapid7 explaining the
vulnerability. "claude code remote control" (9,900) and "claude code remote"
(6,600) are Anthropic's Remote Control feature, not a category. "bastion
host" (9,900) and "jump host" (4,400) are definitions owned by Teleport, AWS
and Wikipedia. "ansible alternative" and "rundeck alternative" want a
listicle of configuration-management tools. None of these should be a page.

## 2. Target keywords

Volumes are monthly Google Ads averages: worldwide English (WW), United
States (US) and United Kingdom (UK). KD is DataForSEO keyword difficulty
(0 to 100, US). Intent is the DataForSEO primary label. Countries is the
clickstream split for the 25 sampled keywords; "not sampled" means the
keyword was outside that sample. Where several rows share a page, the first
row's title tag is the one to use.

### Primary (home page)

| Keyword | WW | US | UK | KD | Intent | Countries | Page | Title tag (under 60 chars) |
|---|---|---|---|---|---|---|---|---|
| ssh alternative | 320 | 70 | 10 | 3 | informational | US 48%, GB 37%, CL 15% | index | heliograph - run commands on a server without SSH |
| run command on remote machine | 110 | 50 | 10 | 0 | transactional | not sampled | index | as above |
| bastion host alternative | 10 | 10 | 10 | n/a | commercial | no data | index | as above |

The home page keyword set is small because the head terms in this space are
either vendor-owned or mean something else. The current title ("run commands
on a machine you cannot log into") is close. Adding "without SSH" matches the
one positioning query with measurable volume and a soft SERP (Reddit, Stack
Overflow, a GitHub list).

### Secondary (one per docs page where a keyword exists)

| Keyword | WW | US | UK | KD | Intent | Countries | Page | Title tag (under 60 chars) |
|---|---|---|---|---|---|---|---|---|
| claude code skills | 60,500 | 12,100 | 1,900 | 30 | informational | JP 18%, US 15%, IN 8%, DE 8%, GB 3% | claude-code | heliograph Claude Code skill - a machine it cannot reach |
| claude code mcp server | 2,900 | 880 | 140 | 3 | navigational | JP 19%, US 15%, DE 14%, IN 9% | mcp | heliograph MCP server for Claude Code, Codex and any agent |
| air gapped environment | 880 | 210 | 30 | 19 | informational | IN 36%, US 21%, IT 14% | air-gapped | Air-gapped servers - run heliograph with no network path at all |
| codex cli skills | 720 | 140 | 30 | 1 | transactional | JP 35%, DE 15%, VN 9% | codex | heliograph Codex CLI skill - a machine it cannot reach |
| air gapped installation | 590 | 480 | 10 | 0 | informational | sparse (MX, TR) | bootstrap | Plant a station - by CLI, by hand, or air-gapped by bundle |
| azure run command | 140 | 40 | 10 | 9 | navigational | IN 55%, ZA 30% | azure | Azure - five station templates, and Run Command compared |
| air gapped kubernetes | 40 | 10 | 10 | 0 | informational | not sampled | containers | Docker and Kubernetes - a station in a container |
| run script on remote machine | 10 | 10 | 10 | n/a | transactional | no data | method | The method - debugging a server you cannot log into |

One caveat. "air gapped installation" as a head term means installing a
named product offline (k3s, Elastic, OpenShift rank); the bootstrap page can
only take the long tail with heliograph in it.

No keyword with measurable volume was found for install, quickstart,
station, steps, runner, conformance, hosts, service, pipelines, windows,
relay, intercom, cli, secrets or security. Their current titles are fine.
The security page has an adjacent cluster (see long tail and gap 1) that
should be handled in a section, not a title.

### Long tail

| Keyword | WW | US | UK | KD | Intent | Countries | Page | Note |
|---|---|---|---|---|---|---|---|---|
| claude code dangerously skip permissions | 5,400 | 1,600 | 210 | 10 | transactional | US 26%, GB 15%, NO 11%, AU 8% | security | see gap 1; a section, not a page title |
| claude code sandbox | 4,400 | 1,000 | 210 | 8 | navigational | JP 34%, US 20%, DE 8% | security | see gap 1; a section, not a page title |
| claude code permissions | 2,900 | 720 | 110 | 10 | transactional | US 24%, JP 20%, PL 6% | security | see gap 1; a section, not a page title |
| best claude code skills | 1,900 | 590 | 90 | 12 | commercial | US 20%, DE 12%, IN 10% | claude-code | listicle intent; better won by being listed in the existing listicles than by a page |
| codex cli mcp | 1,600 | 260 | 50 | 5 | navigational | JP 48%, US 7% | mcp | H2: "From Codex CLI" |
| claude code ssh | 1,300 | 320 | 40 | n/a | navigational | JP 22%, US 15%, DE 9%, TW 8% | claude-code | H2: "Claude Code without SSH", see gap 3 |
| ssh mcp server | 480 | 90 | 10 | n/a | navigational | US 21%, JP 15%, IN 11%, DE 10% | mcp | one line of disambiguation: this is the no-SSH counterpart |
| aws ssm send-command | 320 | 90 | 20 | 10 | transactional | TR 47%, US 30% | compared | see gap 4 |
| air gapped server | 170 | 70 | 10 | 15 | informational | DE (sparse) | air-gapped | see gap 2 |
| air gapped deployment | 170 | 30 | 10 | 6 | informational | IN 50%, DE 20% | air-gapped | see gap 2 |
| aws ssm run command | 140 | 30 | 10 | 8 | transactional | CA (sparse) | compared | see gap 4 |
| terminal mcp server | 90 | 20 | 10 | 21 | navigational | not sampled | mcp | disambiguation line |
| claude code remote server | 70 | 20 | 10 | n/a | navigational | not sampled | claude-code | see gap 3 |
| remote script execution | 30 | 10 | 10 | n/a | transactional | not sampled | method | see gap 5 |
| claude code remote execution | 30 | 10 | 10 | n/a | transactional | not sampled | claude-code | see gap 3 |
| run command on remote server | 10 | 10 | 10 | n/a | transactional | not sampled | method | see gap 5 |

The following phrases match heliograph's intent exactly (DataForSEO scores
them transactional at 0.96 to 1.00) but have no measured volume in any
market: "run commands on a server without ssh", "no ssh access to production
server", "debug production server without ssh", "production access without
ssh", "run script on machine you cannot access", "remote execution audit
log", "auditable command execution", "add timestamp to bash output",
"timestamp every line of output bash", "capture command output with
timestamps". Keep them in copy and H2s because they describe the product,
but do not expect traffic from them, and do not spend a title tag on one.

## 3. Keywords to avoid

| Keyword | WW | US | Why |
|---|---|---|---|
| remote command execution | 140 | 20 | Google treats it as a synonym of RCE. All nine organic results are vulnerability explainers (Cloudflare, CrowdStrike, Wikipedia "Arbitrary code execution", Bugcrowd, Imperva, Rapid7, Lakera, Arctic Wolf). Never use it in a title or H1. Say "run a command on" instead. |
| remote code execution, rce vulnerability, reverse shell, c2 framework | 49,500; 1,600; 12,100; 880 | 5,400; 390; 1,000; 140 | Offensive-security intent. The README's "not a C2 channel" sentence is fine as a sentence; it must not become a target. |
| claude code remote control, claude code remote, codex cli remote control | 9,900; 6,600; 720 | 3,600; 1,300; 170 | Anthropic's Remote Control feature (control a local session from a phone) and OpenAI's equivalent. SERP is code.claude.com, Reddit, Simon Willison, academy.claude.com. Mention once on the claude-code page to disambiguate; do not target. |
| install claude code, claude code, codex cli, mcp server, claude code plugins, claude code plugin marketplace, claude code hooks, claude code agents, claude code github actions | 74,000; 550,000 (US Labs); 165,000; 246,000; 22,200; 4,400; 12,100; 22,200; 2,900 | | Vendor brand and vendor-doc terms. Anthropic and OpenAI own them and always will. "claude code github actions" is Anthropic's GitHub Action, not the pipelines page. |
| bastion host, jump host, what is bastion, bastion server | 9,900; 4,400; 1,900 (Labs); 2,400 (Labs) | 2,400; 480 | Definitional. Teleport, AWS and Wikipedia rank with explainers. heliograph has no reason to write one. Keep the words in copy for the long tail ("behind a bastion you are not allowed through"). |
| ansible alternative, rundeck alternative, ansible without ssh | 1,300; 170; 20 | 260; 30; 10 | Listicle intent for configuration management and runbook automation. heliograph is neither, and the SERP will be G2 and comparison sites. |
| aws ssm, aws ssm run command (as a head term) | 18,100; 140 | 4,400; 30 | Navigational to AWS. Five of nine organic results are docs.aws.amazon.com. Reachable only through the comparison page in gap 4. |
| air gapped, air gapped computer, air gapped laptop, air gapped wallet, air gapped installation (head term) | 22,200; 2,400; 260 (Labs); 140 (Labs); 590 | 8,100 (Labs); 1,300 (Labs) | The bare term is a definition; the hardware and crypto-wallet terms are shopping queries; "installation" means a named product's offline install. Only the environment, server, deployment and kubernetes variants fit. |
| remote desktop, remote access trojan, git remote, run as administrator | large | | Noise from the category expansion of "run script on remote machine". Listed so nobody mistakes them for opportunities. |

## 4. SERP notes

United States, desktop, top ten organic, 2026-09-09.

**claude code mcp server** (2,900 WW, KD 3). code.claude.com "Connect to MCP
servers" #1, platform.claude.com MCP connector #3, Reddit r/ClaudeAI #4,
TrueFoundry "Best MCP Servers for Claude Code" #5, modelcontextprotocol.io
#6, GitHub auchenberg/claude-code-mcp #7, a Medium listicle #8, claude.com
blog on remote MCP #9, YouTube #11. AI overview and People Also Ask present.
Google reads it as "how do I connect an MCP server to Claude Code" plus
"which MCP servers should I use". A plain GitHub repo sits at #7, which is
the evidence the mcp page can rank: it needs "Claude Code" and "MCP server"
in the title and a `claude mcp add` block above the fold.

**claude code ssh** (1,300 WW). Reddit "SSH with Claude Code, surprisingly
simple" #2, 84em.com blog #4, a LinkedIn post #5, jdhodges.com "Set up SSH
in Claude Code Desktop" #7, a GitHub issue about VS Code Remote-SSH #9,
dev.to "Remote AI coding with Claude Code and ShellHub" #10, Stackademic
#11. Discussions-and-forums and video blocks present. The intent is running
Claude Code on a remote box over SSH, or the desktop app's SSH feature. It
is not "no SSH". The fit is partial: the claude-code page can take the
searcher who reads two results and realises they cannot SSH. The SERP is
personal blogs and Reddit with no vendor page, so it is winnable with a
clear "Claude Code without SSH" section (gap 3).

**ssh alternative** (320 WW, KD 3). Reddit r/linux "We need something better
than OpenSSH" #2, Stack Overflow "substitute for SSH" #4, Unix.SE "non-ssh
based alternatives" #5, GitHub moul/awesome-ssh #6, Gartner PAM
alternatives #7, Quora #8, Wikipedia "Comparison of SSH servers" #9,
Superuser #10, ctrlops "best SSH clients" #11. Three intents collide:
protocol alternatives (mosh, telnet), SSH clients, and privileged-access
vendors. Nobody on the page answers "I am not allowed SSH at all". That is
the gap the home page fills, but expect the query to stay ambiguous.

**air gapped installation** (590 WW, KD 0, CPC USD 55.87). docs.k3s.io
#2, Elastic #4, Red Hat OpenShift #5, Wikipedia "Air gap (networking)" #6,
docs.rke2.io #7, Reddit r/kubernetes #9, HashiCorp Terraform Enterprise
#10, kubeops.net #11. Every result is a specific product's offline install
guide. The high CPC is enterprise vendors bidding. heliograph cannot own the
head term. It can own "heliograph air gapped installation" and the bundle
transport story (gap 2).

**run command on remote machine** (110 WW, KD 0). Spiceworks #2, Superuser
"with ssh, how can you run a command on the remote machine" #3, Microsoft
Learn "Running remote commands - PowerShell" #4, Stack Overflow (Windows)
#5, vsupalov.com #7, Reddit r/bash #8, NetBeez "Execute remote commands with
SSH" #9, Ask Ubuntu #10. Pure how-to, and every answer assumes SSH,
PowerShell Remoting or PsExec. Q&A pages with no vendor. A method-style page
that starts from those answers and then covers "and when you cannot" (gap 5)
fits the SERP shape and the difficulty score.

**aws ssm run command** (140 WW, KD 8). docs.aws.amazon.com at #2, #3, #4,
#6 and #11, YouTube #5 and #10, re:Post #8, OneUptime blog #9. Navigational
to AWS. Not winnable head-on. The comparison page in gap 4 is the only
route, and its job is the secondary intent: people whose estate is not on
AWS, or who cannot install the SSM agent, and want the same thing.

Two supplementary SERPs were pulled to settle the avoid list. **remote
command execution**: Cloudflare, CrowdStrike, Wikipedia, Bugcrowd, Imperva,
Rapid7, Lakera, Arctic Wolf, YouTube. Nine of nine are about the
vulnerability. **claude code remote control**: code.claude.com
remote-control docs #1, Reddit #2, simonwillison.net #3, academy.claude.com
#5, code.claude.com mobile #7. It is a product feature name.

Competitor check (step 5). Teleport's non-brand rankings are dominated by
free developer tools (hex converter, regex tester, Unix timestamp converter,
JSON validator, 18,100 to 74,000 each), not by SSH content. Filtered to our
themes, Teleport ranks for "bastion host" (#19), "bastion server" (#29),
"jump server" (#13), "jump box server" (#23), "ssh tunnel" (#9), "ssh port
forwarding" (#6) and "ssh config" (#12), all from explainer posts. Rundeck
ranks for ten themed non-brand keywords, the largest being "run commands
inside docker container" (210) and "aws ecs execute command" (140). ShellHub
ranks for seven non-brand keywords, all under 100. None of the three
competes for heliograph's terms, and none has found volume in "cannot log
in" phrasing either.

## 5. Content gaps

Ordered by worldwide volume of the cluster each would target.

1. **Read-only gates, not --dangerously-skip-permissions.** A section on the
   security page, linked from claude-code. Targets "claude code dangerously
   skip permissions" (5,400 WW, 1,600 US, 210 UK, KD 10), "claude code
   sandbox" (4,400, 1,000, 210, KD 8) and "claude code permissions" (2,900,
   720, 110, KD 10). The skip-permissions query is unusually English-market
   (US 26%, GB 15%, NO 11%, AU 8%). The honest angle: heliograph is not a
   sandbox for Claude Code. The agent never runs on the far side, so there
   is no prompt to skip; the station's gate is the control, and the operator
   holds it. Write it as that contrast, or do not write it.

2. **Air-gapped: a station with no network at all.** A new page. Targets
   "air gapped environment" (880 WW, 210 US, 30 UK), "air gapped server"
   (170, 70, 10), "air gapped deployment" (170, 30, 10), "air gapped
   kubernetes" (40) and the "air gapped installation" long tail (590 head
   term). India and Germany lead the split. Constraint: the README says the
   bundle transport is control-side only today. The page must say what
   works now and what is on the roadmap, or wait until the station side
   lands.

3. **Claude Code without SSH.** A section on the claude-code page, or a
   page if it grows. Targets "claude code ssh" (1,300 WW, 320 US, 40 UK),
   "claude code remote server" (70) and "claude code remote execution"
   (30), and disambiguates "claude code remote control" (9,900) in one
   line. The SERP is Reddit and personal blogs, so a clear page from the
   tool's own site should place.

4. **heliograph compared with AWS SSM Run Command and Azure Run Command.**
   A new page. Targets "aws ssm send-command" (320 WW, 90 US, 20 UK), "aws
   ssm run command" (140, 30, 10), "azure run command" (140, 40, 10) and
   the zero-volume "aws ssm run command alternative". Small numbers, but
   intent is transactional at 0.99, and it is the first question a platform
   engineer asks. The angle: those need an agent installed and cloud IAM;
   heliograph installs nothing and works in an estate you do not own.

5. **Run a command on a remote machine when SSH is not an option.** A
   method-style explainer. Targets "run command on remote machine" (110 WW,
   50 US, KD 0), "run script on remote machine" (10), "run command on remote
   server" (10), "remote script execution" (30) and every zero-volume
   intent-perfect phrase from section 2. Start from the SSH and PowerShell
   answers the SERP already gives, then cover the case where none of them
   is available.

6. **Lower priority: timestamps on every line, and reading the gaps.** "add
   timestamp to bash output", "ts command linux" and "timestamp every line
   of output bash" have no measured volume in any market, but intent is
   transactional at 1.00 and the Stack Overflow questions exist. A short
   section on the conformance or cli page, not a page.

## 6. Cost

Total spent: **USD 1.0937** across **38 calls** (36 billed; two
`ranked_keywords` calls failed on a filter-count error at USD 0.00 and were
retried). Every call is logged in `~/.config/claude-seo/dataforseo-ledger.json`
with the note "heliograph keyword research", and every response is saved
under `~/dbhq-uk/dbhq-seo/reports/heliograph/raw/`.

| Endpoint | Calls | Cost (USD) |
|---|---|---|
| keywords_data/google_ads/search_volume/live (US seeds, UK seeds, US candidates, UK candidates, worldwide) | 5 | 0.4500 |
| keywords_data/clickstream_data/global_search_volume/live (top 25) | 1 | 0.1800 |
| dataforseo_labs/google/keyword_suggestions/live | 6 | 0.1620 |
| dataforseo_labs/google/keyword_ideas/live | 5 | 0.1200 |
| dataforseo_labs/google/related_keywords/live | 5 | 0.0634 |
| dataforseo_labs/google/ranked_keywords/live | 6 (4 billed) | 0.0620 |
| dataforseo_labs/google/bulk_keyword_difficulty/live | 1 | 0.0202 |
| dataforseo_labs/google/search_intent/live | 1 | 0.0202 |
| serp/google/organic/live/regular | 8 | 0.0160 |
| **Total** | **38** | **1.0937** |

Budget cap was USD 3.00. Nothing was dropped to stay under it. The
`keyword_ideas` endpoint was the least useful spend: category expansion of
"air gapped environment" returned air purifiers, and of "run script on
remote machine" returned git and Remote Desktop. `keyword_suggestions`
(phrases containing the seed) found everything of value in the agent
cluster and should be the first expansion call next time.

## 7. Corrections

Later findings that change what to do about something measured above. The
tables keep the measurement; this section keeps the decision. All five below
come from the nine-dimension audit of 2026-09-11 (#70).

**The `bootstrap` and `azure` title tags must not be applied.** Both were
written before gaps 2 and 4 were built, and neither is true of the page it
names. `bootstrap.md` contains no mention of air gaps or bundles, and
`azure.md` contains no mention of Run Command. Each keyword is already carried
by the page that does earn it: `/air-gapped` for the air-gap cluster, and
`/compared`, whose title is "heliograph vs AWS SSM Run Command and Azure Run
Command", for "azure run command". The rows read as unshipped work and are not
- a baseline run on 10 Sep 2026 applied both before checking the pages, and
reverted. **A title tag has to be true of the page before it is good for the
keyword.**

**The air-gap rows now point at `/air-gapped`.** They were assigned to
`transports` because no air-gapped page existed when this was written. One was
built the same day (#46), so "air gapped environment", "air gapped server" and
"air gapped deployment" belong to it, and gap 2 is built. Its shipped title
runs to 63 characters against the 60 this document asks for. It is left alone
until there are impressions to judge it by.

**The "see gap N" cells were off by a reordering.** Section 5 was ordered by
cluster volume after the tables were written, and the cells still pointed at
the old sequence. They now agree with section 5.

**`claude code skills` (60,500) cannot be won by a page.** A fresh SERP pull
returns no single-vendor product page in the top ten: Anthropic's own
documentation takes four of the ten, and the rest are curated directories,
listicles, a Reddit thread and two videos. No rebuild of `/claude-code` places
in that. This document already had the answer in the long tail - the
neighbouring term is "better won by being listed in the existing listicles
than by a page" - so it is an outreach task, not a content one: the MCP
registry, and the awesome-lists. The effort goes instead to two terms the same
pull says are winnable. `claude code mcp server` (2,900, KD 3), where a single
implementation's repository already ranks at #7 and `/mcp` is built to that
shape. And `codex cli skills` (720, KD 1), where, unlike the skills SERP,
single-tool pages rank at #7 and #8.

**Never publish the bare word off-site.** A Hacker News search for
"heliograph" returns 53 results, every one about the 19th-century optical
signalling mirror, the top two linking to its Wikipedia page. The established
entity owns the word. Every title on this site already pairs the name with
what it does, and the same rule holds for anything published elsewhere - a
directory listing, a pull request to an awesome-list, a Show HN title, a post:
"heliograph - run a command on a machine you can't SSH into", never
"heliograph" on its own.

**Gap 5 is built, and two H2s are questions.** `/method` gains "Running a
command on a remote machine, when SSH is not an option", written the way the
SERP for that query is written: it starts from the `ssh`, `Invoke-Command`,
PsExec and cloud-agent answers every result on that page gives, and then covers
the case where each has been refused. `/claude-code` and `/mcp` get one
question-form H2 each - "Can Claude Code run commands without SSH?" and "How do
I add heliograph as an MCP server?" - and no other page does, because the form
costs the site's voice and wins nothing where no query matches it. Their anchor
slugs changed with the wording: headings are slugified from their text and this
site has no explicit-anchor syntax. Nothing linked to either, on this site or
in the skills, so nothing broke - checked before the change, not after.
