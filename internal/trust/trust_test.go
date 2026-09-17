package trust

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/heliograph-io/heliograph/internal/seal"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// A fixture estate: an anchor the machine's owner keeps, and three engineers.
type people struct {
	anchor, alice, bob, carol, mallory *seal.Identity
}

func newPeople(t *testing.T) people {
	t.Helper()
	var p people
	for _, dst := range []**seal.Identity{&p.anchor, &p.alice, &p.bob, &p.carol, &p.mallory} {
		id, err := seal.Generate()
		if err != nil {
			t.Fatal(err)
		}
		*dst = id
	}
	return p
}

func mustSet(t *testing.T, p people) Set {
	t.Helper()
	s, err := NewSet("acme", "db-a", "owner", p.anchor.Public(), at("2026-01-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// sign builds and signs a change from `who`, named `asName` in the set.
func sign(t *testing.T, s Set, who *seal.Identity, asName, op, subjectName string, subject seal.PublicIdentity) Change {
	t.Helper()
	c := NewChange(s, op, subjectName, subject, Member{Name: asName}, at("2026-02-01T00:00:00Z"))
	c, err := SignChange(who, c)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustApply(t *testing.T, s Set, c Change) Set {
	t.Helper()
	next, err := s.Apply(c, at("2026-02-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("a change that should have applied was refused: %v", err)
	}
	return next
}

// --- the anchor. This is the assertion the whole claim rests on --------------

// TestTheAnchorIsUnchangeableByAnyRequest is the load-bearing one.
//
// A compromised key may evict every other engineer. It may not evict the
// anchor, because that is what stops the estate owner being locked out of their
// own machine by anything remote.
func TestTheAnchorIsUnchangeableByAnyRequest(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	s = mustApply(t, s, sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public()))

	cases := []struct {
		what    string
		by      *seal.Identity
		byName  string
		op      string
		name    string
		subject seal.PublicIdentity
	}{
		{"a member revoking the anchor by name", p.alice, "alice", OpRevoke, "owner", p.anchor.Public()},
		{"a member revoking the anchor's key under another name", p.alice, "alice", OpRevoke, "someone", p.anchor.Public()},
		{"a member adding a second member called owner", p.alice, "alice", OpAdd, "owner", p.bob.Public()},
		{"a member re-adding the anchor's key", p.alice, "alice", OpAdd, "backup", p.anchor.Public()},
		// AND NOT EVEN THE ANCHOR ITSELF, over the transport. The anchor changes
		// with a command on the machine, and there is no remote path to it -
		// including for whoever holds the anchor key.
		{"the anchor revoking itself remotely", p.anchor, "owner", OpRevoke, "owner", p.anchor.Public()},
		{"the anchor replacing itself remotely", p.anchor, "owner", OpAdd, "owner", p.bob.Public()},
	}
	for _, tc := range cases {
		c := sign(t, s, tc.by, tc.byName, tc.op, tc.name, tc.subject)
		next, err := s.Apply(c, at("2026-03-01T00:00:00Z"))
		if !errors.Is(err, ErrAnchor) {
			t.Errorf("%s: got %v, want ErrAnchor", tc.what, err)
		}
		if next.Digest() != s.Digest() {
			t.Errorf("%s: a refused change altered the set", tc.what)
		}
		if !next.Anchor.Public.Equal(p.anchor.Public()) || !next.Anchor.Active() {
			t.Errorf("%s: the anchor moved", tc.what)
		}
	}
}

// TestTheAnchorChangesOnlyOnTheMachine is the other half: it CAN be changed,
// through the local command, or the recovery property would be a lock-out.
func TestTheAnchorChangesOnlyOnTheMachine(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	next, err := s.SetAnchor("owner", p.bob.Public(), at("2026-04-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if !next.Anchor.Public.Equal(p.bob.Public()) {
		t.Error("SetAnchor did not move the anchor")
	}
	if next.Serial != s.Serial+1 {
		t.Errorf("the serial did not advance: %d then %d", s.Serial, next.Serial)
	}
	if s.Anchor.Public.Equal(p.bob.Public()) {
		t.Error("SetAnchor mutated the set it was called on")
	}
}

// --- a change is a signed document, verified against the set as it stands ----

func TestAChangeMustBeSignedByACurrentMember(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)

	c := sign(t, s, p.mallory, "mallory", OpAdd, "mallory", p.mallory.Public())
	if _, err := s.Apply(c, at("2026-02-01T00:00:00Z")); !errors.Is(err, ErrUnknownAuthor) {
		t.Errorf("a change signed by a key nobody trusts: got %v, want ErrUnknownAuthor", err)
	}

	// The anchor adds alice; alice adds bob. The chain is the point.
	s = mustApply(t, s, sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public()))
	s = mustApply(t, s, sign(t, s, p.alice, "alice", OpAdd, "bob", p.bob.Public()))
	if len(s.Members()) != 2 {
		t.Fatalf("expected two members, got %d", len(s.Members()))
	}
	if s.Members()[1].AddedBy != "alice" {
		t.Errorf("bob was added by %q, not alice: attribution is the audit record", s.Members()[1].AddedBy)
	}
}

func TestATamperedChangeIsRefused(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	c := sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public())

	// Every field the signature covers, altered one at a time. A signature that
	// covers a field it appears to cover is the only thing being asserted here,
	// and the only way to assert it is to move each field and watch it fail.
	tamper := map[string]func(Change) Change{
		"the subject key": func(c Change) Change { c.Subject = p.mallory.Public(); return c },
		"the name":        func(c Change) Change { c.Name = "mallory"; return c },
		"the operation":   func(c Change) Change { c.Op = OpRevoke; return c },
		"the author name": func(c Change) Change { c.Author = "alice"; return c },
		"the utc":         func(c Change) Change { c.UTC = "2030-01-01T00:00:00Z"; return c },
		"the signature":   func(c Change) Change { c.Sig[0] ^= 0xff; return c },
	}
	for what, f := range tamper {
		// A FRESH COPY OF THE SIGNATURE PER CASE, before the alteration runs.
		// Sharing the backing array made the signature case flip a byte and then
		// flip it back, so it asserted nothing and reported PASS.
		altered := c
		altered.Sig = append([]byte{}, c.Sig...)
		altered = f(altered)
		if _, err := s.Apply(altered, at("2026-02-01T00:00:00Z")); err == nil {
			t.Errorf("altering %s was accepted", what)
		}
	}
}

func TestAChangeIsBoundToOneStation(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	c := sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public())

	other, err := NewSet("acme", "db-b", "owner", p.anchor.Public(), at("2026-01-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Apply(c, at("2026-02-01T00:00:00Z")); !errors.Is(err, ErrWrongTarget) {
		t.Errorf("a change for db-a applied at db-b: got %v, want ErrWrongTarget", err)
	}
}

// TestReplayProtectionSurvivesARestart is the property #29 asks for by name.
//
// The serial lives in the set ON DISK. A station that restarts reloads it, so a
// change it has already applied is refused whether or not anything is still in
// memory - which matters because a station restarts when the machine does, and
// that is exactly when nobody is watching.
func TestReplayProtectionSurvivesARestart(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	c := sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public())
	s = mustApply(t, s, c)

	// The restart: everything in memory is thrown away and the set is read back
	// from the bytes that were written.
	reloaded, err := ParseSet(s.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Digest() != s.Digest() {
		t.Fatalf("a set did not survive a round trip through its own file:\n%s", s.Marshal())
	}
	if _, err := reloaded.Apply(c, at("2026-03-01T00:00:00Z")); !errors.Is(err, ErrOutOfOrder) {
		t.Errorf("the same change replayed after a restart: got %v, want ErrOutOfOrder", err)
	}
}

func TestAChangeAuthoredAgainstAnOlderSetIsRefused(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	// Two administrators author at the same moment, both against serial 0.
	first := sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public())
	second := sign(t, s, p.anchor, "owner", OpAdd, "bob", p.bob.Public())
	s = mustApply(t, s, first)
	if _, err := s.Apply(second, at("2026-02-01T00:00:00Z")); !errors.Is(err, ErrOutOfOrder) {
		t.Errorf("the loser of a race was applied anyway: got %v", err)
	}
}

// --- revocation, and the refusal that names the key --------------------------

func TestARevokedKeyIsRefusedAndNamed(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	s = mustApply(t, s, sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public()))
	s = mustApply(t, s, sign(t, s, p.anchor, "owner", OpAdd, "carol", p.carol.Public()))
	// The leaver flow: alice revokes carol, remotely, with no operator involved.
	s = mustApply(t, s, sign(t, s, p.alice, "alice", OpRevoke, "carol", p.carol.Public()))

	m, ok := s.Lookup(p.carol.Public())
	if !ok {
		t.Fatal("a revoked member vanished from the set: a refusal could then only say `unknown`")
	}
	if m.Active() {
		t.Error("carol is still active after being revoked")
	}
	if m.RevokedBy != "alice" {
		t.Errorf("revoked by %q, not alice", m.RevokedBy)
	}

	c := sign(t, s, p.carol, "carol", OpAdd, "mallory", p.mallory.Public())
	_, err := s.Apply(c, at("2026-05-01T00:00:00Z"))
	if !errors.Is(err, ErrRevokedAuthor) {
		t.Fatalf("a revoked key authored a change: got %v", err)
	}
	// THE REFUSAL NAMES THE KEY. "not trusted" reads as a broken enrolment and
	// sends somebody to re-plant a station that is working perfectly.
	if !strings.Contains(err.Error(), "carol") ||
		!strings.Contains(err.Error(), p.carol.Public().Fingerprint()) {
		t.Errorf("the refusal names neither the person nor the key: %v", err)
	}
}

func TestARevokedKeyCannotBeReAdded(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	s = mustApply(t, s, sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public()))
	s = mustApply(t, s, sign(t, s, p.anchor, "owner", OpAdd, "carol", p.carol.Public()))
	s = mustApply(t, s, sign(t, s, p.alice, "alice", OpRevoke, "carol", p.carol.Public()))

	c := sign(t, s, p.alice, "alice", OpAdd, "carol", p.carol.Public())
	if _, err := s.Apply(c, at("2026-06-01T00:00:00Z")); !errors.Is(err, ErrRevokedKey) {
		t.Errorf("a revoked key was resurrected: got %v, want ErrRevokedKey", err)
	}
}

// --- joiner, mover and leaver, without re-enrolling a station ----------------

// TestJoinerMoverLeaverNeedNoOperator is #28's last box.
//
// Every one of these is a signed change over the transport. The operator who
// planted the station is not involved in any of them, which is the whole
// difference between a tool one person uses and something an organisation can
// run: offboarding one engineer across forty estates cannot mean forty visits.
func TestJoinerMoverLeaverNeedNoOperator(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	planted := s.Digest()

	// Joiner: bob is enrolled by the anchor, alice by bob.
	s = mustApply(t, s, sign(t, s, p.anchor, "owner", OpAdd, "bob", p.bob.Public()))
	s = mustApply(t, s, sign(t, s, p.bob, "bob", OpAdd, "alice", p.alice.Public()))
	// Mover: carol joins under a key of her own, added by alice rather than by
	// the anchor. Nobody had to touch the machine.
	s = mustApply(t, s, sign(t, s, p.alice, "alice", OpAdd, "carol", p.carol.Public()))
	// Leaver: bob goes, revoked by alice.
	s = mustApply(t, s, sign(t, s, p.alice, "alice", OpRevoke, "bob", p.bob.Public()))

	if planted == s.Digest() {
		t.Fatal("four changes and the set did not move")
	}
	active := 0
	for _, m := range s.Members() {
		if m.Active() {
			active++
		}
	}
	if active != 2 {
		t.Errorf("expected alice and carol active, got %d", active)
	}
	if m, _ := s.Lookup(p.bob.Public()); m.Active() {
		t.Error("the leaver can still author")
	}
	if s.Anchor.Public.Equal(p.anchor.Public()) != true {
		t.Error("the anchor moved during ordinary administration")
	}
}

// --- the set as a document ----------------------------------------------------

func TestASetRoundTripsThroughItsOwnFile(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	s = mustApply(t, s, sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public()))
	s = mustApply(t, s, sign(t, s, p.anchor, "owner", OpAdd, "carol", p.carol.Public()))
	s = mustApply(t, s, sign(t, s, p.alice, "alice", OpRevoke, "carol", p.carol.Public()))

	back, err := ParseSet(s.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if back.Digest() != s.Digest() {
		t.Errorf("digest changed across a round trip:\n%s", s.Marshal())
	}
	// READABLE WITH `cat`. The estate owner audits this without tooling.
	txt := string(s.Marshal())
	for _, want := range []string{"anchor: owner ", "member: alice ", "member: carol "} {
		if !strings.Contains(txt, want) {
			t.Errorf("the published set does not contain %q:\n%s", want, txt)
		}
	}
}

func TestAChangeRoundTripsThroughARequestDocument(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	c := sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public())

	// The change rides inside an ordinary request document, and the station has
	// to find it there without the other keys confusing it.
	doc := "version: 1\nid: 20260201T000000Z-trust\nstep:\nenv:\n" + string(c.Marshal()) + "note: adding alice\n"
	back, err := ParseChange([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(back, at("2026-02-01T00:00:00Z")); err != nil {
		t.Errorf("a change did not survive being carried in a request: %v", err)
	}
}

func TestADocumentWithNoChangeIsNotARefusal(t *testing.T) {
	if _, err := ParseChange([]byte("version: 1\nid: x\nstep: probe\n")); !errors.Is(err, ErrNoChange) {
		t.Errorf("an ordinary request looked like a broken change: %v", err)
	}
}

func TestAnUnknownSetVersionIsRefusedRatherThanGuessed(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)
	bad := strings.Replace(string(s.Marshal()), "version: 1", "version: 2", 1)
	if _, err := ParseSet([]byte(bad)); err == nil {
		t.Error("a set from a newer build was parsed anyway, which is guessing at who may command a machine")
	}
}
