// Package trust is the set of keys a station will accept a request from, and
// the signed documents that change it.
//
// THE HOLE THIS CLOSES. "The service holds no signing key, so it cannot cause a
// station to run anything" is true and insufficient. Signature verification
// only means the station trusts whatever key it was told to trust, so a service
// that could administer trust roots could add a key it controls and then sign
// legitimately. Nothing stolen, nothing forged, every gate passed, claim false.
//
// Four rules close it, and each does exactly one job:
//
//  1. A change to the set is itself a signed document, verified against the set
//     as it stands. There is no unsigned path: see the mutation rule below.
//  2. Any trusted key may add or revoke any key EXCEPT the anchor. Remote
//     revocation is the whole point of a leaver flow; requiring the operator on
//     each of forty estates to offboard one engineer would make it useless.
//  3. The anchor changes only on the machine. A compromised key can evict every
//     other engineer and cannot evict the anchor, so recovery is a signed
//     request from whoever holds it rather than a site visit. The estate owner
//     is never locked out of their own machine by anything remote.
//  4. The station publishes the set's fingerprints, so the owner can audit who
//     may command their estate without asking us.
//
// THE MUTATION RULE, WHICH IS WHAT MAKES RULE 1 A PROPERTY RATHER THAN A HABIT.
// Set.members is unexported and this package exports no mutator except Apply,
// which verifies a signature before it returns a changed set. No package
// outside this one can add a member to a set, and Go's compiler is what
// enforces that rather than a review. `TestNothingOutsideThisPackageMutatesASet`
// asserts the other half: that inside it, nothing writes to members except the
// verified path.
//
// NO BESPOKE CRYPTOGRAPHY. Ed25519 through the standard library, over a
// length-prefixed canonical encoding built here. The encoding is not
// cryptography; it exists so that two different documents can never serialise
// to the same bytes, which is how a signature comes to cover something other
// than what it appears to.
package trust

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dbhq-uk/heliograph/internal/seal"
)

// Version is the document version this build writes and the only one it reads.
// A station in the field is upgraded by whoever owns the machine, which may be
// never, so a set it cannot parse must be refused loudly rather than guessed at.
const Version = 1

// Domain is the signing domain for a change. It is length-prefixed into the
// signed bytes, so a signature made over a change can never verify as anything
// else this repository signs.
const Domain = "heliograph-trusted-set-change-v1"

// AnchorName is reserved. A member may not be called this, because a set whose
// anchor and whose ordinary member share a name is a set where "revoke owner"
// is ambiguous, and an ambiguity in an authorisation decision resolves in
// whichever direction the reader was not expecting.
const AnchorName = "anchor"

// Member is one key the station will accept a request from.
//
// Revoked is a tombstone rather than a deletion, and that is deliberate twice
// over: a refusal can name the revoked key rather than saying "unknown", which
// is the difference between "your access was removed" and "something is broken";
// and a key that is merely absent could be re-added by anyone trusted, whereas
// one that is revoked is refused for good.
type Member struct {
	Name      string
	Public    seal.PublicIdentity
	Added     string // RFC3339 UTC
	AddedBy   string // the name of the member who signed the change that added it
	Revoked   string // RFC3339 UTC, empty while active
	RevokedBy string
}

// Active reports whether this member may author a request today.
func (m Member) Active() bool { return m.Revoked == "" }

// Set is the whole trusted set for one station.
//
// members is unexported. See the package comment: it is the compiler-enforced
// half of "a change to the set is itself a signed document".
type Set struct {
	Version int
	Estate  string
	Station string
	Serial  uint64
	UTC     string
	Anchor  Member

	members []Member
}

// NewSet plants the anchor.
//
// THIS IS THE ONLY WAY AN ANCHOR IS EVER SET, and it is reachable only from a
// command run on the machine itself. There is no document, no request and no
// transport path that reaches it. That is rule 3, and it is what stops a
// compromised key locking the estate owner out of their own machine.
func NewSet(estate, station, anchorName string, anchor seal.PublicIdentity, now time.Time) (Set, error) {
	if estate == "" || station == "" {
		return Set{}, fmt.Errorf("trust: a set must name its estate and station, or a set from one machine verifies on another")
	}
	if anchorName == "" {
		anchorName = AnchorName
	}
	if err := validName(anchorName); err != nil {
		return Set{}, err
	}
	if len(anchor.Sign) == 0 || len(anchor.Enc) == 0 {
		return Set{}, fmt.Errorf("trust: the anchor has no key: it is the one thing a set cannot be created without")
	}
	return Set{
		Version: Version,
		Estate:  estate,
		Station: station,
		Serial:  0,
		UTC:     stamp(now),
		Anchor: Member{
			Name: anchorName, Public: anchor,
			Added: stamp(now), AddedBy: "operator",
		},
	}, nil
}

// Members returns a copy, ordered as they were added.
//
// A COPY, and not the slice. Handing out the backing array would give every
// caller a mutator, which is precisely the thing the unexported field exists to
// prevent - and it would do it silently, because appending to a returned slice
// usually does not alter the original and sometimes does.
func (s Set) Members() []Member {
	out := make([]Member, len(s.members))
	copy(out, s.members)
	return out
}

// Everyone is the anchor followed by every member, which is the set a request
// is actually verified against.
func (s Set) Everyone() []Member {
	return append([]Member{s.Anchor}, s.Members()...)
}

// Lookup finds a key in the set, anchor included.
//
// It returns the member whether or not it is revoked, because the two answers
// need different sentences: an unknown key is somebody who was never trusted,
// and a revoked key is somebody who was. Saying the first when it is the second
// sends the reader hunting for a broken enrolment.
func (s Set) Lookup(p seal.PublicIdentity) (Member, bool) {
	for _, m := range s.Everyone() {
		if m.Public.Equal(p) {
			return m, true
		}
	}
	return Member{}, false
}

// LookupSigning matches on the Ed25519 half alone.
//
// IT EXISTS FOR ONE PURPOSE AND MUST NOT BE USED FOR ANY OTHER: turning a
// failed verification into a sentence that names somebody. A sealed envelope
// carries the claimed signing key in the clear and does not carry the
// encryption half, so a station that has just refused a message can find out
// whose key was claimed - which is the difference between "your access was
// removed" and "something is broken".
//
// It decides nothing. Every caller has already exhausted verification against
// the active members before it is reached, so a match here can only produce a
// better error message. Using it to authorise would mean accepting a party who
// holds a signing key and a different encryption key, which is a different
// party.
func (s Set) LookupSigning(sign []byte) (Member, bool) {
	if len(sign) == 0 {
		return Member{}, false
	}
	for _, m := range s.Everyone() {
		if len(m.Public.Sign) == len(sign) && string(m.Public.Sign) == string(sign) {
			return m, true
		}
	}
	return Member{}, false
}

// ByName is the same question asked the other way, for a change that names its
// author rather than carrying the key.
func (s Set) ByName(name string) (Member, bool) {
	for _, m := range s.Everyone() {
		if m.Name == name {
			return m, true
		}
	}
	return Member{}, false
}

// canonical is the byte string a digest and a signature are taken over.
//
// Length-prefixed, never delimited. A delimiter can appear inside a value, and
// then two different sets serialise identically - which is exactly how a
// signature gets to cover something other than what it appears to. seal.Meta
// does the same thing for the same reason.
func (s Set) canonical() []byte {
	var b []byte
	add := func(str string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(str)))
		b = append(b, n[:]...)
		b = append(b, str...)
	}
	num := func(v uint64) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], v)
		b = append(b, n[:]...)
	}
	add("heliograph-trusted-set-v1")
	num(uint64(s.Version))
	add(s.Estate)
	add(s.Station)
	num(s.Serial)
	member := func(m Member) {
		add(m.Name)
		add(m.Public.Encode())
		add(m.Added)
		add(m.AddedBy)
		add(m.Revoked)
		add(m.RevokedBy)
	}
	member(s.Anchor)
	num(uint64(len(s.members)))
	for _, m := range s.members {
		member(m)
	}
	return b
}

// Digest is what a change names as the set it applies to, and what the station
// publishes so the owner can compare without reading every key.
//
// UTC IS NOT IN IT, deliberately. The digest has to be reproducible on the
// control node from a copy of the same set, and a timestamp recording when a
// particular machine last wrote the file is not a property of the set. Putting
// it in would make every station report a different digest for identical trust.
func (s Set) Digest() string {
	sum := sha256.Sum256(s.canonical())
	return hex.EncodeToString(sum[:])
}

// Fingerprints is the one-line audit view: who may command this estate.
//
// THIS IS THE ONLY RENDERER OF THAT LINE, and it is why `trust show` prints it
// as a field rather than leaving the station to assemble one. It used to be
// built three times - here, by heliograph-seal's show output, and by a sed
// pipeline in station.sh - and the three disagreed: `revoked` here, `REVOKED`
// there. An owner grepping their estate for REVOKED would have found it on the
// PowerShell stations and not on the bash ones, which is the audit failing
// silently in the direction that reassures.
//
// Upper case, because it is the word somebody scanning a status is looking for.
func (s Set) Fingerprints() string {
	var parts []string
	for _, m := range s.Everyone() {
		state := ""
		if !m.Active() {
			state = " REVOKED"
		}
		parts = append(parts, fmt.Sprintf("%s=%s%s", m.Name, m.Public.Fingerprint(), state))
	}
	return strings.Join(parts, " ")
}

// Marshal writes the set in the same `key: value` shape as every other document
// that crosses the gap.
//
// The estate owner reads this with `cat` on a machine we cannot reach, and that
// is the whole point of publishing it: they can audit who may command their
// estate without asking us and without tooling. JSON would be tidier to parse
// and worse at the only job that matters.
//
// A member line is SIX SPACE-SEPARATED TOKENS, always, with `-` where a value is
// absent. A variable-length line is one awk gets wrong on the far side, on a
// machine nobody can log into to find out why.
func (s Set) Marshal() []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "version: %d\n", s.Version)
	fmt.Fprintf(&b, "estate: %s\n", s.Estate)
	fmt.Fprintf(&b, "station: %s\n", s.Station)
	fmt.Fprintf(&b, "serial: %d\n", s.Serial)
	fmt.Fprintf(&b, "utc: %s\n", s.UTC)
	fmt.Fprintf(&b, "digest: %s\n", s.Digest())
	fmt.Fprintf(&b, "anchor: %s\n", marshalMember(s.Anchor))
	for _, m := range s.members {
		fmt.Fprintf(&b, "member: %s\n", marshalMember(m))
	}
	return b.Bytes()
}

func marshalMember(m Member) string {
	return strings.Join([]string{
		m.Name, m.Public.Encode(), dash(m.Added), dash(m.AddedBy),
		dash(m.Revoked), dash(m.RevokedBy),
	}, " ")
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func undash(s string) string {
	if s == "-" {
		return ""
	}
	return s
}

// ParseSet reads one back.
//
// UNLIKE wire.ParseRequest, AN UNKNOWN VERSION IS AN ERROR. A request that a
// station half-understands runs a step and produces a log somebody can read; a
// trusted set that a station half-understands decides who may command the
// machine, and the failure mode of guessing is trusting a key nobody meant to
// trust. Refuse and say so.
func ParseSet(b []byte) (Set, error) {
	var s Set
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	seenAnchor := false
	for sc.Scan() {
		k, v, ok := splitField(sc.Text())
		if !ok {
			continue
		}
		switch k {
		case "version":
			s.Version, _ = strconv.Atoi(v)
		case "estate":
			s.Estate = v
		case "station":
			s.Station = v
		case "serial":
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return Set{}, fmt.Errorf("trust: serial %q is not a number, and the serial is what stops a change being replayed", v)
			}
			s.Serial = n
		case "utc":
			s.UTC = v
		case "digest":
			// Recomputed rather than trusted. A digest read out of the file it
			// describes proves nothing; it is written for the human reading it
			// against what the station published.
		case "anchor":
			m, err := parseMember(v)
			if err != nil {
				return Set{}, err
			}
			s.Anchor = m
			seenAnchor = true
		case "member":
			m, err := parseMember(v)
			if err != nil {
				return Set{}, err
			}
			s.members = append(s.members, m)
		}
	}
	if err := sc.Err(); err != nil {
		return Set{}, err
	}
	if s.Version != Version {
		return Set{}, fmt.Errorf("trust: this set is version %d and this build reads version %d: refusing to guess at who may command this station", s.Version, Version)
	}
	if !seenAnchor {
		return Set{}, fmt.Errorf("trust: this set has no anchor, and a set without one has nothing that survives a compromised key")
	}
	if s.Estate == "" || s.Station == "" {
		return Set{}, fmt.Errorf("trust: this set names no estate or no station, so it cannot be bound to one machine")
	}
	return s, nil
}

func parseMember(v string) (Member, error) {
	f := strings.Fields(v)
	if len(f) != 6 {
		return Member{}, fmt.Errorf("trust: a member line is six fields (name key added added-by revoked revoked-by), got %d: %q", len(f), v)
	}
	pub, err := seal.DecodePublic(f[1])
	if err != nil {
		return Member{}, fmt.Errorf("trust: member %q has an unusable key: %w", f[0], err)
	}
	if err := validName(f[0]); err != nil {
		return Member{}, err
	}
	return Member{
		Name: f[0], Public: pub,
		Added: undash(f[2]), AddedBy: undash(f[3]),
		Revoked: undash(f[4]), RevokedBy: undash(f[5]),
	}, nil
}

func splitField(line string) (key, value string, ok bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
}

// validName keeps a member name to one space-free token.
//
// The name is a field in a space-separated line and a value in a published
// status, so a space in it forges a field and a newline forges a line. Refused
// here, where there is somebody to tell, rather than on the far side.
func validName(n string) error {
	if n == "" {
		return fmt.Errorf("trust: a member needs a name, because a refusal that cannot say who is a refusal nobody can act on")
	}
	if len(n) > 64 {
		return fmt.Errorf("trust: %q is longer than 64 characters", n)
	}
	for _, r := range n {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
		default:
			return fmt.Errorf("trust: %q is not a usable member name: letters, digits, dot, hyphen and underscore only", n)
		}
	}
	return nil
}

func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }
