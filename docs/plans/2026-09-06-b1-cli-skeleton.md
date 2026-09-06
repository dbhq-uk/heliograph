# B1 - control CLI skeleton Implementation Plan

**Goal:** A Go CLI that drives an unmodified `heliograph-skill` station over the existing git transport: attach to a transport repo, publish a request, read the logs that come back.

**Architecture:** Three layers with one direction of dependency. `internal/wire` owns the request and status document format and knows nothing else. `internal/transport` owns moving those documents, with `git` as the only implementation, behind an interface the relay will later satisfy unchanged. `cmd/heliograph` is argument parsing and printing, and contains no logic worth testing.

**Tech Stack:** Go 1.27.1, standard library only. No CLI framework: the command surface is small, and a dependency here would be the first of many in a repo whose sibling promises none.

## Global Constraints

- **Go 1.27.1**, standard library only. Every added dependency needs an argument in the PR that introduces it
- The CLI drives a **stock, unmodified station**. Nothing in B1 may require a change to `heliograph-skill`
- Request documents are `key: value` text, readable by an operator without tooling. That is a feature, not an accident
- House style: British English, plain hyphens, **no em dashes**, no trailing full stops on headings
- `gofmt -l` must be empty, `go vet ./...` clean, `go test ./...` green before every commit
- Never print a credential. Report mechanism and length, never value

---

### Task 1: `internal/wire` - the request document

**Files:**
- Create: `go.mod`, `internal/wire/request.go`, `internal/wire/request_test.go`

**Interfaces:**
- Produces:
  - `type Request struct { Version int; ID, Step, Env, Cancel, Stop, Note string }`
  - `func (r Request) Marshal() []byte`
  - `func ParseRequest([]byte) (Request, error)`
  - `func NewID(step string, t time.Time) string`

- [ ] **Step 1: Write the failing test**

The format is fixed by what today's `station.sh` parses with `sed -n "s/^id:[[:space:]]*//p"`. Round-tripping is not enough on its own: the bytes have to be the shape that sed reads.

```go
func TestMarshalIsWhatTheStationParses(t *testing.T) {
	r := Request{Version: 1, ID: "20260906T101500Z-net", Step: "net-probe",
		Env: `HOSTS="a b" PORTS=1433`}
	got := string(r.Marshal())
	for _, want := range []string{
		"version: 1\n", "id: 20260906T101500Z-net\n",
		"step: net-probe\n", "env: HOSTS=\"a b\" PORTS=1433\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("marshal missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestEmptyFieldsArePresentButBlank(t *testing.T) {
	// station.sh reads `cancel:` and `stop:` unconditionally. Omitting the
	// keys is fine for sed, but an operator reading the file should see the
	// full shape of what they can set.
	got := string(Request{Version: 1, ID: "x"}.Marshal())
	for _, want := range []string{"cancel:\n", "stop:\n", "note:\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("marshal missing %q", want)
		}
	}
}

func TestNewIDIsSortableAndNamesTheStep(t *testing.T) {
	ts := time.Date(2026, 9, 6, 10, 15, 0, 0, time.UTC)
	got := NewID("net-probe", ts)
	if got != "20260906T101500Z-net-probe" {
		t.Errorf("got %q", got)
	}
}

func TestParseTolerateseExtraAndMissingKeys(t *testing.T) {
	// A station in the field may write keys this build does not know, and an
	// operator may hand-edit one out. Neither is an error.
	r, err := ParseRequest([]byte("id: abc\nstep: env\nfuture: 42\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "abc" || r.Step != "env" {
		t.Errorf("got %+v", r)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

`cd ~/dbhq-heliograph-product && go test ./internal/wire/` -> FAIL, undefined: Request

- [ ] **Step 3: Implement**

- [ ] **Step 4: Run to verify it passes**

- [ ] **Step 5: `gofmt -l .`, `go vet ./...`, commit**

---

### Task 2: `internal/wire` - the status document

**Files:**
- Create: `internal/wire/status.go`, `internal/wire/status_test.go`

**Interfaces:**
- Produces: `type Status struct { State, ID, Step, Host, Branch, UTC, Started, Finished, Exit, Log, Last, Progress string }`, `func ParseStatus([]byte) (Status, error)`

- [ ] **Step 1: Write the failing test**

Parse a real status document as `station.sh` writes it, taken verbatim from `publish_status` and `publish_progress`.

```go
func TestParseStatusFromARealRunningStation(t *testing.T) {
	in := []byte(`state:    running
id:       20260906T101500Z-net
step:     net-probe
host:     box01.example
branch:   task/dns
utc:      2026-09-06T10:15:02Z
started:  2026-09-06T10:15:00Z
progress: 412 lines
log:      ops-logs/net-probe-20260906T101500Z.txt
last:     11:31:29 | ---------- openssl s_client ----------
`)
	s, err := ParseStatus(in)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != "running" || s.Step != "net-probe" {
		t.Errorf("got %+v", s)
	}
	// The value keeps its own colons. Splitting on every colon would truncate
	// `last:` at the timestamp, which is the field that says where a long run
	// has got to.
	if !strings.HasSuffix(s.Last, "openssl s_client ----------") {
		t.Errorf("last was truncated: %q", s.Last)
	}
}

func TestStatusDoneReportsTerminalStates(t *testing.T) {
	for _, tc := range []struct {
		state string
		done  bool
	}{
		{"running", false}, {"idle", true}, {"cancelled", true},
		{"refused", true}, {"stopped", true},
	} {
		if got := (Status{State: tc.state}).Done(); got != tc.done {
			t.Errorf("%s: got %v", tc.state, got)
		}
	}
}
```

- [ ] **Step 2-5:** as Task 1.

---

### Task 3: `internal/transport` - the interface, and git

**Files:**
- Create: `internal/transport/transport.go`, `internal/transport/git.go`, `internal/transport/git_test.go`

**Interfaces:**
- Produces:
  - `type Transport interface { FetchStatus() (wire.Status, error); PutRequest(wire.Request) error; ListLogs() ([]string, error); ReadLog(string) ([]byte, error); Check() error; Describe() string }`
  - `func NewGit(dir string) (*Git, error)`

The verb names match the station-side contract in the master design, so that when A3 lands the two halves are describable in one sentence.

- [ ] **Step 1: Write the failing test**

Against a real git repo with a real bare origin. No mocking: the thing that breaks here is git's behaviour, and a mock of git tests nothing.

```go
func TestPutRequestCommitsAndPushes(t *testing.T) {
	work, origin := newRepo(t)          // helper: bootstrapped clone + bare origin
	g, err := NewGit(work)
	if err != nil { t.Fatal(err) }
	req := wire.Request{Version: 1, ID: "run-1", Step: "env"}
	if err := g.PutRequest(req); err != nil { t.Fatal(err) }

	// It must reach the REMOTE. A commit that never pushed is invisible to the
	// station, which is the whole failure this transport exists to avoid.
	out := run(t, "git", "-C", origin, "show", "HEAD:station/request")
	if !strings.Contains(out, "id: run-1") {
		t.Errorf("request did not reach origin:\n%s", out)
	}
}

func TestPutRequestRebasesWhenTheRemoteMoved(t *testing.T) {
	// The station pushes far more often than the control does: a status commit
	// on every transition, a progress snapshot every 60s, and the log. So the
	// remote WILL have moved under us, and a plain push is rejected. That is
	// two writers on one branch working as intended, not a fault.
	work, origin := newRepo(t)
	pushFromElsewhere(t, origin, "station/status", "state: running\n")
	g, _ := NewGit(work)
	if err := g.PutRequest(wire.Request{Version: 1, ID: "run-2"}); err != nil {
		t.Fatalf("push after remote moved: %v", err)
	}
}

func TestCheckReportsAnUnreachableRemoteWithoutHanging(t *testing.T) {
	work, origin := newRepo(t)
	os.RemoveAll(origin)
	g, _ := NewGit(work)
	if err := g.Check(); err == nil {
		t.Error("Check passed against a deleted origin")
	}
}
```

- [ ] **Step 2-5:** as Task 1.

---

### Task 4: `cmd/heliograph` - `init`, `send`, `logs`

**Files:**
- Create: `cmd/heliograph/main.go`, `internal/estate/estate.go`, `internal/estate/estate_test.go`

**Interfaces:**
- Produces: `func Load(name string) (Estate, error)`, `func (e Estate) Save() error`, config at `$XDG_CONFIG_HOME/heliograph/estates/<name>.json`

- [ ] **Step 1: Write the failing test for estate config**

```go
func TestEstateRoundTripsThroughDisk(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	e := Estate{Name: "payments", Transport: "git", Dir: "/w/payments", Branch: "task/dns"}
	if err := e.Save(); err != nil { t.Fatal(err) }
	got, err := Load("payments")
	if err != nil { t.Fatal(err) }
	if got != e { t.Errorf("got %+v want %+v", got, e) }
}

func TestLoadNamesTheEstateItCouldNotFind(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := Load("nope")
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("error must name the estate, got %v", err)
	}
}

func TestSaveRefusesAPathTraversingName(t *testing.T) {
	// The name becomes a filename. `../../.ssh/authorized_keys` must not.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := (Estate{Name: "../escape"}).Save(); err == nil {
		t.Error("saved an estate whose name escapes the config directory")
	}
}
```

- [ ] **Step 2-5:** as Task 1, then wire the three commands in `main.go` and verify by hand against a real bootstrapped transport repo.

---

### Task 5: End-to-end against a real station

**Files:**
- Create: `cmd/heliograph/e2e_test.go`

- [ ] **Step 1: Write the test**

The one that matters. Everything above can pass while the CLI and the station disagree about the document they share.

```go
func TestCLIDrivesAStockStation(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil { t.Skip("no bash") }
	skill := os.Getenv("HELIOGRAPH_SKILL_DIR")
	if skill == "" { t.Skip("HELIOGRAPH_SKILL_DIR not set: needs a heliograph-skill checkout") }

	// bootstrap a transport repo with the SKILL's own bootstrap.sh, add a
	// step, `heliograph send` it, run the station once, and assert a log came
	// back with the step's output in it.
}
```

- [ ] **Step 2: Run it**, with `HELIOGRAPH_SKILL_DIR` pointing at the sibling checkout.

- [ ] **Step 3: CI workflow** that checks out both repos and runs it.

- [ ] **Step 4: Commit**

---

## Self-Review

**Spec coverage.** B1 is "Go CLI skeleton, `init`/`send`/`logs` on the existing git transport, cross-compiled releases". Tasks 1-4 cover the commands, Task 5 proves they work against a real station. Cross-compiled releases are deliberately **not** here: a release pipeline before there is anything worth releasing is ceremony, and it lands with B2 when `watch` and `--gaps` make the binary worth installing.

**Type consistency.** `wire.Request` and `wire.Status` are defined in Tasks 1 and 2 and consumed by name in Task 3. `Transport`'s verbs match the station-side contract in the master design.

**The risk this plan carries.** Task 5 is the only test that can catch the CLI and the station disagreeing, and it needs a checkout of another repository. If it is skipped in CI, the most valuable test here silently does not run - the same shape of failure as the loud-skip rule in the skill repo. So CI must check out both repos and the test must fail rather than skip when the variable is set but wrong.
