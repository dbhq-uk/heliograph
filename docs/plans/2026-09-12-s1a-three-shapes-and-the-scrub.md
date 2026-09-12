# S1a: three shapes, the beam characterisation, and the scrub - implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the transport shapes to beacon, flare and beam in the site's
generated source and prose, rewrite the anti-tunnel refusal into an honest beam
characterisation, and remove every "regulated" reference from the repository
behind a CI guard.

**Architecture:** The shape vocabulary lives in one Go source
(`internal/site/matrix.go`) that the matrix page is generated from, so the
words change there first and the pages cannot disagree. Prose changes follow in
`site/content/`, the README and the two dated specs. A new repo-walking Go test
enforces that the old positioning cannot return.

**Tech Stack:** Go 1.x (stdlib `testing`, `path/filepath`), markdown under
`site/content/`, Cloudflare Pages `_redirects`.

## Global Constraints

- **Spec:** `docs/specs/2026-09-11-three-shapes-and-signalling-names-design.md` (issue #86)
- **Names, exactly:** `beacon` (was pigeonhole), `flare` (was intercom), `beam` (new). Lowercase in IDs and slugs; capitalised at the start of a sentence or a nav title.
- **No full stops on headings** - existing repo convention (`AGENTS.md`).
- **Markdown in repo files wraps at ~76 characters** - match surrounding files.
- **S1 must not claim a capability that does not exist.** The beam is designed, not built. No beam row is added to the matrix table; only the shape vocabulary and prose land here. S4 (#90) adds the transport.
- **This plan is the docs half only.** The internal rename (`pigeonhole.sh`, `intercom.sh`, `intercom.py`, and 21 `PIGEONHOLE_*`/`INTERCOM_*` variables across ~55 files) is S1b and has its own plan. Nothing in this plan renames a script or an environment variable.
- **Run all Go tests with:** `go test ./...` from the repository root.
- **Imports are your job.** The test and source snippets below show the code, not the import block. Tasks 2 and 3 add uses of `os`, `path/filepath` and `strings` to `cmd/heliograph-site/main_test.go`, and Task 3 adds `fmt` and `strings` to `cmd/heliograph-site/main.go`. Add any that are missing; a compile error naming an undefined package is this, not a mistake in the plan.

---

### Task 1: The shape vocabulary in the generated source

The matrix is generated from `internal/site/matrix.go`, so the three shapes are
defined there before any page mentions them.

**Files:**
- Modify: `internal/site/matrix.go:22-42` (the `Kind` block), `:110-123` (transport entries), `:131`, `:154-155`, `:166`, `:395`
- Test: `internal/site/matrix_test.go:118-135`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `site.Beacon`, `site.Flare`, `site.Beam` of type `site.Kind`, with string values `"beacon"`, `"flare"`, `"beam"`. The transport formerly `ID: "intercom"` becomes `ID: "flare"`, `Name: "flare"`, `Href: "/flare"`. Tasks 2 and 3 rely on the slug `flare`.

- [ ] **Step 1: Replace the failing shape test**

Replace `TestBothTransportShapesArePresent` in `internal/site/matrix_test.go`
(lines 118-135) with this. It asserts the new vocabulary and that the beam is
*allowed but not required*, because no beam transport ships yet.

```go
// All shipped transports must carry one of the three shapes, or the page's
// central claim is decoration. The flare is the only member of its kind and
// would be the one lost. The beam is designed and not yet built (S4), so it is
// permitted here and not required.
func TestTransportShapesArePresent(t *testing.T) {
	var beacon, flare, beam int
	for _, tr := range Transports {
		switch tr.Kind {
		case Beacon:
			beacon++
		case Flare:
			flare++
		case Beam:
			beam++
		default:
			t.Errorf("transport %q has no shape", tr.ID)
		}
	}
	if beacon == 0 || flare == 0 {
		t.Fatalf("want the beacon and the flare represented, got %d beacon and %d flare", beacon, flare)
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./internal/site/ -run TestTransportShapesArePresent -v`
Expected: FAIL to compile with `undefined: Beacon`, `undefined: Flare`,
`undefined: Beam`.

- [ ] **Step 3: Replace the Kind block**

In `internal/site/matrix.go`, replace lines 22-42 with:

```go
// Kind is the shape of a transport, and it is the distinction the whole
// product rests on. The axis is what is held, and for how long: a beacon holds
// a message, a flare is a single exchange, a beam holds the connection itself.
//
// The names come from signalling, which is what a heliograph is.
type Kind string

const (
	// Beacon is store-and-forward. Both sides dial OUT to one agreed place
	// and neither ever accepts a connection. Everything shipped is one.
	Beacon Kind = "beacon"

	// Flare is direct. You can reach the station's endpoint, so there is
	// no drop in the middle - and the gates change shape, which is the part
	// that matters rather than the latency.
	Flare Kind = "flare"

	// Beam is a live channel held open in both directions until it is torn
	// down. Designed in S4 and not yet built, so nothing carries this Kind
	// yet; it is named here because the vocabulary is one thing.
	Beam Kind = "beam"
)
```

- [ ] **Step 4: Update every transport entry**

In the same file, change `Kind: Pigeonhole` to `Kind: Beacon` on every transport
(lines 110, 112, 114, 116, 118, 120). Then replace the intercom entry at lines
122-123 with:

```go
	{ID: "flare", Name: "flare", Kind: Flare, Control: Works, Station: Partial,
		Note: "The one case where you CAN reach the station. The script travels with the request, so heliograph-mode stops being a control and becomes a claim the caller makes about its own file.", Href: "/flare"},
```

- [ ] **Step 5: Update the three pairing maps and the CSS badge**

Line 131: change the key `"intercom"` to `"flare"`.
Line 155: change `"intercom": ok("the one host with a reachable endpoint")` to
`"flare": ok("the one host with a reachable endpoint")`, and leave
`ok("through pigeonhole.sh")` **unchanged** - that names a file, which S1b
renames, not this plan.
Line 166: change `"intercom": ok("through intercom.sh")` to
`"flare": ok("through intercom.sh")` - again the filename is S1b's.
Line 395: change both occurrences of `intercom` in the CSS selector and
`content:"intercom"` to `flare`.

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./internal/site/ -v`
Expected: PASS, all tests in the package.

- [ ] **Step 7: Commit**

```bash
git add internal/site/matrix.go internal/site/matrix_test.go
git commit -m "feat(site): the three shapes - beacon, flare and beam - in the generated source"
```

---

### Task 2: The navigation, the page metadata, and the flare page

The slug `intercom` becomes `flare` everywhere the generator names it, and the
content file moves with it.

**Files:**
- Modify: `cmd/heliograph-site/main.go:35`, `:314`, `:328`, `:353`, `:994`, `:1019`, `:1021`
- Rename: `site/content/intercom.md` -> `site/content/flare.md`
- Test: `cmd/heliograph-site/main_test.go`

**Interfaces:**
- Consumes: the slug `flare` established in Task 1.
- Produces: a page at slug `flare`; the slug `intercom` no longer exists in `order`, which Task 3 redirects.

- [ ] **Step 1: Write the failing test**

Add to `cmd/heliograph-site/main_test.go`:

```go
// The nav and the content directory must agree. A slug in order with no
// markdown behind it renders an empty page, and a markdown file no slug names
// is never published at all.
func TestFlareReplacesIntercom(t *testing.T) {
	for _, slug := range order {
		if slug == "intercom" {
			t.Error("the nav still lists intercom; the page is /flare now")
		}
	}
	var found bool
	for _, slug := range order {
		if slug == "flare" {
			found = true
		}
	}
	if !found {
		t.Fatal("the nav does not list flare")
	}
	if _, err := os.Stat(filepath.Join("..", "..", "site", "content", "flare.md")); err != nil {
		t.Fatalf("site/content/flare.md is missing: %v", err)
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./cmd/heliograph-site/ -run TestFlareReplacesIntercom -v`
Expected: FAIL with "the nav still lists intercom".

- [ ] **Step 3: Move the content file and rename its subject**

```bash
git mv site/content/intercom.md site/content/flare.md
```

Then in `site/content/flare.md`, change the H1 from
`# Intercom - when you can reach the station` to
`# Flare - when you can reach the station`, and replace the words `intercom`
and `Intercom` where they name **the shape** with `flare` and `Flare`. Leave
`./intercom.sh` and `INTERCOM_URL`/`INTERCOM_KEY` exactly as they are - those
name a script and environment variables that S1b renames, and this page must
keep matching what ships today.

- [ ] **Step 4: Update the generator**

`cmd/heliograph-site/main.go`:
- Line 35: in `order`, change `"intercom"` to `"flare"`.
- Line 314: in the Reference group, change `"intercom"` to `"flare"`.
- Line 328: in the comment, change `"Intercom - when you can reach the station"` to `"Flare - when you can reach the station"`.
- Line 353: change `"intercom":    "Intercom",` to `"flare":       "Flare",`.
- Line 994: change the key to `"flare"` and the value to `"Flare - submit a step over HTTPS when you can reach the station"`.
- Line 1019: change the matrix description to `"Every heliograph transport, station and controller in one place, which combinations work, and the beacon-versus-flare split that decides the rest."`.
- Line 1021: change the key to `"flare"` and the value to `"A flare submits a step to a heliograph station over HTTPS, for the rarer case where you can reach the machine's network but still cannot log into it."`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./cmd/heliograph-site/ ./internal/site/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/heliograph-site/main.go cmd/heliograph-site/main_test.go site/content/flare.md
git commit -m "feat(site): /intercom becomes /flare, in the nav and the metadata"
```

---

### Task 3: The redirect, so no external link dies

`/intercom` has been published and linked. The generator writes a Cloudflare
Pages `_redirects` file so the old path keeps working.

**Files:**
- Modify: `cmd/heliograph-site/main.go` (near the other `os.WriteFile` calls, around line 118-130)
- Test: `cmd/heliograph-site/main_test.go`

**Interfaces:**
- Consumes: the slug `flare` from Task 2.
- Produces: a `redirects` map and a `_redirects` file in the output directory.

- [ ] **Step 1: Write the failing test**

Add to `cmd/heliograph-site/main_test.go`:

```go
// A published path that stops existing is a 404 for everybody who linked it.
func TestRedirectsCarryTheOldIntercomPath(t *testing.T) {
	got := redirectsFile()
	if !strings.Contains(got, "/intercom /flare 301") {
		t.Errorf("_redirects does not carry the intercom redirect, got:\n%s", got)
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./cmd/heliograph-site/ -run TestRedirectsCarryTheOldIntercomPath -v`
Expected: FAIL to compile with `undefined: redirectsFile`.

- [ ] **Step 3: Add the redirects map and its renderer**

In `cmd/heliograph-site/main.go`, above the `main` function:

```go
// redirects keeps a published path alive after its page is renamed. Cloudflare
// Pages reads _redirects; a reader who followed an old link gets the new page
// rather than the 404 handler.
var redirects = [][2]string{
	{"/intercom", "/flare"},
}

func redirectsFile() string {
	var b strings.Builder
	for _, r := range redirects {
		fmt.Fprintf(&b, "%s %s 301\n", r[0], r[1])
	}
	return b.String()
}
```

- [ ] **Step 4: Write the file during generation**

In `main`, beside the other `os.WriteFile` calls (after the `404.html` write at
line 118):

```go
	if err := os.WriteFile(filepath.Join(out, "_redirects"), []byte(redirectsFile()), 0o644); err != nil {
		return err
	}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./cmd/heliograph-site/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/heliograph-site/main.go cmd/heliograph-site/main_test.go
git commit -m "feat(site): _redirects keeps /intercom alive after the rename"
```

---

### Task 4: The three-shape prose

Two pages carry the "two shapes" model in words and must now agree with the
generated source.

**Files:**
- Modify: `site/content/matrix.md:11-26`, `site/content/transports.md:20-30`

**Interfaces:**
- Consumes: the vocabulary from Task 1.
- Produces: no code.

- [ ] **Step 1: Rewrite the matrix page's shape section**

In `site/content/matrix.md`, replace the heading
`## Every transport is one of two shapes` and its table with:

```markdown
## Every transport is one of three shapes

This is the distinction the whole design rests on, and it decides more about an
estate's answer than any other fact on this page. The axis is one thing: **what
is held, and for how long.**

| | |
|---|---|
| **beacon** | A signal left where both can see it. You cannot reach the far side, the far side cannot reach you, and **both can reach one agreed place**. A *message* waits in the middle; nobody is ever connected |
| **flare** | Fired straight at a reachable endpoint. One burst, an answer, gone - a *transaction*, not a drop and not a standing line |
| **beam** | Held steady on the far station, live and two-way until it is torn down. The *connection itself* is what is held |
```

Then change `**Everything shipped is a pigeonhole.**` to
`**Everything shipped is a beacon.**`, and in the sentence that follows change
`The words come from the station itself - pigeonhole.sh and intercom.sh have
carried them since before this page existed` to `The beam is designed and not
yet built, so nothing below carries it; the shapes are named together because
the vocabulary is one thing`.

- [ ] **Step 2: Rewrite the transports page opening**

In `site/content/transports.md`, replace the sentence at lines 23-25 with:

```markdown
answer than anything else here: a **beacon** is a signal left where both can
see it and collected later, a **flare** is fired straight at a station you can
reach, and a **beam** is a live line held open in both directions. All six
below are beacons.
```

- [ ] **Step 3: Add the plain-language "three ways across" section**

The spec asks for one short section in plain language, in both places a reader
arrives. Add this to `site/content/transports.md`, immediately under its H1:

```markdown
## The three ways across

heliograph carries a request to a machine you cannot log into, and brings the
log back. There are three ways across the gap, and they differ in one thing:
**what stays held, and for how long.**

**Beacon.** You cannot reach the machine and it cannot reach you - but you can
both reach one agreed place. You leave the request there and walk away. Later
the machine passes by, picks it up, runs it, and leaves the log for you to
collect. Nobody is ever connected; a *message* waits in the middle. It is the
safest of the three, because the code being run is already on the far side and
can be read before anything happens - and the slowest, because you wait for the
next visit.

**Flare.** You can reach the machine's door directly. You knock, hand over the
request, wait on the step while it runs, and take the log away in the same
visit. Nothing waits in the middle and no line stays open. Faster, because
there is no pickup to wait for. The trade: you bring the code with you, so the
machine trusts *the door* rather than vetting the code in advance.

**Beam.** You and the machine bring up a connection and hold it open. Either
side can speak at any moment and the other hears it at once, until you hang up.
A real session, not a message or a knock - and the most exposed, because while
the line is open anything can travel down it. You turn it on deliberately and
close it when you are done.

In one line: a beacon holds a *message*, a flare is a *single exchange*, a beam
holds the *connection itself*.
```

Add the same section to `README.md`, immediately after the ASCII loop diagram
and before `## Does this sound familiar`, wrapped to match the file.

- [ ] **Step 4: Regenerate and eyeball the pages**

Run: `go run ./cmd/heliograph-site -out /tmp/site-check && grep -c beacon /tmp/site-check/matrix.html`
Expected: a non-zero count, and no error from the generator.

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add site/content/matrix.md site/content/transports.md README.md
git commit -m "docs(site): three shapes in the prose, and the three ways across"
```

---

### Task 5: The refusal becomes a characterisation

The README and the security page currently say heliograph refuses to hold a
connection open. The beam is that connection, so these say what it is instead.

**Files:**
- Modify: `README.md:173-183`, `site/content/security.md`

**Interfaces:**
- Consumes: nothing.
- Produces: no code.

- [ ] **Step 1: Rewrite the README section**

In `README.md`, replace the `## What it will not do` heading and its first
paragraph (lines 173-179) with:

```markdown
## What the beam is, and what it costs

Two of the three shapes never hold a connection open. A **beacon** leaves a
message where both sides can reach it; a **flare** knocks, waits and leaves.
Neither needs anything to be reachable, ever - no inbound port, no endpoint, no
tunnel - and between them they do the whole job.

The **beam** does hold a line open, live and two-way, and that is a tunnel. A
blue team will read a held-open channel as one, because it is one. So it is
**off unless it is explicitly enabled on both ends**, it is sealed and signed,
and the station refuses to establish one unless it was started to allow it.
Where an estate forbids a reverse connection, the beacon and the flare are the
answer and nothing is lost but latency.

Every command still runs on the far side because someone with legitimate access
chose to let it.
```

- [ ] **Step 2: Add the beam section to the security page**

In `site/content/security.md`, after the opening paragraph, add:

```markdown
## The beam, and the blast radius of a held-open line

A beam is a live channel. While it is up, a step beam still runs each line
through `run.sh`, so the read-only gate and the captured log survive - but a
**raw** beam carries opaque bytes, and `run.sh` cannot see inside them. That is
interactive access to the account the station runs as, gated once, at the door.

The controls are therefore at establishment, and there are three:

- a beam does not establish at all unless the station was started to allow one
- a raw beam needs a further, separate permission, because it is the one that
  removes the per-command gate
- an onward forward reaches only destinations the operator listed by name

A raw beam leaves a connection record rather than a captured log: class,
destination, peer, open and close times, and bytes each way. It is not the
content, and the page says so rather than implying otherwise.
```

- [ ] **Step 3: Reopen the survey row that refused this**

`docs/specs/2026-09-10-new-transports-and-stations-design.md:53` currently reads:

```
| raw TCP, reverse tunnel | never | a C2 channel by any blue team's definition. Not being one is why this is permitted at all |
```

A verdict of `never` that the next design reverses is exactly the stale claim
this repository's ranking rule puts first. Replace it with:

```
| raw TCP, reverse tunnel | **superseded** | An unauthenticated always-on reverse connection stays refused, and for the original reason. The **beam** is the answer that was built instead: off unless explicitly enabled on both ends, sealed, signed, and torn down when idle. See the beam design (S4) and the direct beam (S6) |
```

Leave the reasoning in `docs/specs/2026-09-06-heliograph-next-design.md:129`
("TCP is dropped") standing - it is the argument at the time, and a reader
comparing it with S4 can see what changed and why.

- [ ] **Step 4: Regenerate and check both pages render**

Run: `go run ./cmd/heliograph-site -out /tmp/site-check && grep -c "the blast radius of a held-open line" /tmp/site-check/security.html`
Expected: `1`.

- [ ] **Step 5: Commit**

```bash
git add README.md site/content/security.md docs/specs/2026-09-10-new-transports-and-stations-design.md
git commit -m "docs: the refusal becomes a characterisation - what the beam is, and what it costs"
```

---

### Task 6: The scrub, behind a guard

Fourteen references go, and a repo-walking test stops the positioning
returning.

**Files:**
- Create: `cmd/heliograph/vocabulary_test.go`
- Modify: `README.md:35`, `site/content/index.md:21`, `site/content/index.md:66`, `site/content/security.md:5`, `site/content/claude-code.md:66`, `site/content/relay.md:71`, `site/content/dbhq.md:41`, `site/content/roadmap.md:64`, `docs/specs/2026-09-06-heliograph-next-design.md:129`, `:152`, `:443`, `docs/specs/2026-09-10-new-transports-and-stations-design.md:133`, `:380`

**Interfaces:**
- Consumes: nothing.
- Produces: a guard the rest of the repository must keep passing.

**Note on the two exclusions.** The guard cannot police itself or the document
that records the rule, because both must name the word to do their job. The S1
spec tables every replacement, and this test holds the needle. Two exclusions,
both self-referential, both named in the test's own comment. Everything else is
covered with no carve-out.

- [ ] **Step 1: Write the failing guard**

Create `cmd/heliograph/vocabulary_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The positioning this repository used to carry described the estates it
// targets by their regulator rather than by their shape. It was replaced on
// 2026-09-11 with language about the situation - no route in, somebody else
// holds the keys - and this stops it coming back a phrase at a time.
//
// Two files are exempt, and both for the same reason: they are the record of
// the rule rather than subject to it. The spec tables every replacement, and
// this test holds the needle it searches for.
var vocabularyExempt = map[string]bool{
	"docs/specs/2026-09-11-three-shapes-and-signalling-names-design.md": true,
	"cmd/heliograph/vocabulary_test.go":                                 true,
}

func TestTheOldPositioningIsGone(t *testing.T) {
	// Built from parts so this file does not match its own search.
	needle := "regul" + "at"

	root := repoRootForVocabulary(t)
	var found []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if vocabularyExempt[filepath.ToSlash(rel)] {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // unreadable or binary, not our business
		}
		if strings.Contains(strings.ToLower(string(body)), needle) {
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
	if len(found) > 0 {
		t.Errorf("the old positioning survives in %d file(s):\n  %s",
			len(found), strings.Join(found, "\n  "))
	}
}

// repoRootForVocabulary walks up until it finds go.mod, so the test does not
// care which directory it was invoked from.
func repoRootForVocabulary(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory")
		}
		dir = parent
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./cmd/heliograph/ -run TestTheOldPositioningIsGone -v`
Expected: FAIL listing the files that still carry it - the seven site and
README files, the two dated specs, plus `PLAN.md` and the roadmap if the
programme's own notes mention it.

- [ ] **Step 3: Make the nine user-facing replacements**

| file | now says | becomes |
|---|---|---|
| `README.md:35` | `regulated, restricted, change-controlled` | `restricted, change-controlled, or reached only through people who can` |
| `site/content/index.md:21` | `regulated, restricted, change-controlled` | `restricted, change-controlled, or reached only through people who can` |
| `site/content/index.md:66` | `permitted in regulated estates` | `permitted in the estates it targets` |
| `site/content/security.md:5` | `regulated estate` | `an estate you cannot log into` |
| `site/content/claude-code.md:66` | `regulated, restricted, change-controlled` | `restricted, change-controlled, or reached only through people who can` |
| `site/content/relay.md:71` | `regulated customers, who are the customers` | `customers who cannot let a third party read what their machines print, who are the customers` |
| `site/content/dbhq.md:41` | `in regulated estates` | `in estates with no route in` |
| `site/content/roadmap.md:64` | `the regulated market` | `the enterprise market` |

If `README.md:179` still carries the phrase after Task 5, it is inside text
Task 5 replaced - confirm and remove the remainder.

- [ ] **Step 4: Make the five historical-spec replacements**

| file | now says | becomes |
|---|---|---|
| `docs/specs/2026-09-06-heliograph-next-design.md:129` | `permitted in regulated estates` | `permitted in the estates it targets` |
| `:152` | `an unacceptable trust ask for regulated customers, who are the customers` | `an unacceptable trust ask for customers who cannot let a third party read what their machines print, who are the customers` |
| `:443` | `why regulated estates permit this` | `why these estates permit this` |
| `docs/specs/2026-09-10-new-transports-and-stations-design.md:133` | `Every regulated estate that refuses` | `Every estate that refuses` |
| `:380` | `the regulated market` | `the enterprise market` |

Only the label changes. The arguments stay exactly as they were, including the
two this programme overturns.

- [ ] **Step 5: Clear any remaining hits the guard names**

Run: `go test ./cmd/heliograph/ -run TestTheOldPositioningIsGone -v`
If `PLAN.md` or `docs/plans/2026-09-11-signalling-toolkit-roadmap.md` appear,
reword those lines the same way - they describe the change and do not need the
word to do it. Repeat until the list is empty.

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./cmd/heliograph/ -run TestTheOldPositioningIsGone -v`
Expected: PASS.

- [ ] **Step 7: Run the whole suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "docs: the old positioning is gone, and a guard keeps it gone"
```

---

## What this plan does not do

Named so the gap is deliberate rather than forgotten:

- **No script or environment-variable rename.** `pigeonhole.sh`, `intercom.sh`,
  `intercom.py` and the 21 `PIGEONHOLE_*` / `INTERCOM_*` names across ~55 files
  are **S1b**, with its own plan and its own compatibility tests. Pages in this
  plan that quote a filename or a variable keep quoting today's name, because
  today's name is what ships.
- **No beam transport.** The vocabulary lands; the capability is S4 (#90).
- **No matrix row for the beam**, for the same reason - a designed transport in
  a table of what works is the "template that looks authoritative" the roadmap
  warns about.
