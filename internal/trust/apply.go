package trust

import (
	"errors"
	"fmt"
	"time"

	"github.com/heliograph-io/heliograph/internal/seal"
)

// The refusals, as sentinels, because the station publishes the reason and a
// reader on the far side has to be able to act on it.
//
// EVERY ONE OF THESE IS PUBLISHED. A refusal nobody can see is indistinguishable
// from a station that died, and the person reading it cannot log into the
// machine to find out which.
var (
	// ErrNoChange is not a refusal. It means the document carried no change at
	// all, which is the ordinary case for every request that is just a step.
	ErrNoChange = errors.New("trust: this document carries no trusted-set change")

	ErrAnchor        = errors.New("trust: the anchor is changeable only on the machine")
	ErrWrongTarget   = errors.New("trust: this change was authored for a different estate or station")
	ErrOutOfOrder    = errors.New("trust: this change is not the next one")
	ErrForked        = errors.New("trust: this change was authored against a different set")
	ErrUnknownAuthor = errors.New("trust: this change was signed by a key this station does not trust")
	ErrRevokedAuthor = errors.New("trust: this change was signed by a revoked key")
	ErrBadSignature  = errors.New("trust: the signature on this change does not verify")
	ErrDuplicate     = errors.New("trust: that name or key is already in the set")
	ErrRevokedKey    = errors.New("trust: that key was revoked and may not be added again")
	ErrNotAMember    = errors.New("trust: that key is not in the set")
)

// Refusal wraps a sentinel with the sentence a station publishes.
//
// The sentence names the subject, because "refused: the anchor is changeable
// only on the machine" leaves the reader to guess which key they tried to
// touch, on a machine they cannot log into to find out.
type Refusal struct {
	Reason error
	Detail string
}

func (r *Refusal) Error() string {
	if r.Detail == "" {
		return r.Reason.Error()
	}
	return r.Reason.Error() + ": " + r.Detail
}

func (r *Refusal) Unwrap() error { return r.Reason }

func refuse(reason error, format string, a ...any) error {
	return &Refusal{Reason: reason, Detail: fmt.Sprintf(format, a...)}
}

// Apply verifies a change against this set and returns the set that results.
//
// IT NEVER MUTATES THE RECEIVER. A refused change must leave the set exactly as
// it was, and a function that edits in place and then returns an error has
// already half-applied by the time the caller reads it.
//
// The order of the checks is chosen, not incidental. Cheap, unambiguous
// bindings first - target, ordering, fork - so that a message a hostile relay
// made up costs no cryptography. The signature is checked before any decision
// about WHAT the change does, so an unsigned document can never reach the code
// that would apply it. The anchor rule is checked after the signature and
// before the mutation, because it must hold even for a change a trusted key
// signed: that is the entire point of it.
func (s Set) Apply(c Change, now time.Time) (Set, error) {
	if err := c.validateShape(); err != nil {
		return s, &Refusal{Reason: ErrBadSignature, Detail: err.Error()}
	}

	if c.Estate != s.Estate || c.Station != s.Station {
		return s, refuse(ErrWrongTarget, "it names %s/%s and this station is %s/%s",
			c.Estate, c.Station, s.Estate, s.Station)
	}

	// THE REPLAY DEFENCE, AND IT SURVIVES A RESTART BECAUSE THE SERIAL IS IN THE
	// SET ON DISK. An in-memory window forgets on restart, and a station restarts
	// whenever the machine does - which is exactly when nobody is watching.
	if c.Serial != s.Serial+1 {
		return s, refuse(ErrOutOfOrder, "it is serial %d and this station is at %d, so the next one is %d",
			c.Serial, s.Serial, s.Serial+1)
	}

	// Two administrators authoring against the same set produce two changes with
	// the same serial. The first to arrive wins; the second names a set that no
	// longer exists and is refused rather than silently reordered.
	if c.Prev != s.Digest() {
		return s, refuse(ErrForked, "it was authored against set %s and this station holds %s",
			short(c.Prev), short(s.Digest()))
	}

	author, ok := s.Lookup(c.AuthorKey)
	if !ok {
		return s, refuse(ErrUnknownAuthor, "the signing key is %s, which is in neither the anchor nor any member",
			c.AuthorKey.Fingerprint())
	}
	if !author.Active() {
		// NAMES THE REVOKED KEY. "not trusted" reads as a broken enrolment and
		// sends somebody to re-plant a station that is working perfectly.
		return s, refuse(ErrRevokedAuthor, "%s (%s) was revoked on %s by %s",
			author.Name, author.Public.Fingerprint(), author.Revoked, author.RevokedBy)
	}
	// The name is carried for the audit line and is not authority. The KEY is.
	// Checking they agree stops a change being attributed to somebody who did
	// not sign it, which would put the wrong name in an archive nobody can
	// correct afterwards.
	if author.Name != c.Author {
		return s, refuse(ErrUnknownAuthor, "the signature is %s's and the change claims to be from %q",
			author.Name, c.Author)
	}

	if !c.AuthorKey.VerifyDocument(Domain, c.canonical(), c.Sig) {
		return s, refuse(ErrBadSignature, "it claims to be from %s (%s)",
			c.Author, c.AuthorKey.Fingerprint())
	}

	// --- THE ANCHOR RULE -------------------------------------------------------
	//
	// This is the assertion the whole claim rests on. A compromised key may evict
	// every other engineer; it may not evict the anchor, so the estate owner is
	// never locked out of their own machine by anything remote, and recovery is a
	// signed request from whoever holds the anchor rather than a site visit.
	//
	// CHECKED ON BOTH THE NAME AND THE KEY, because either alone leaves a way
	// through: matching only the name lets a change revoke the anchor's KEY under
	// a different name, and matching only the key lets a change add a second
	// member CALLED the anchor, after which "revoke owner" is ambiguous and
	// resolves in whichever direction the reader was not expecting.
	//
	// There is deliberately no flag, no override and no privileged author that
	// gets past this. Not even the anchor's own key: the anchor changes with
	// `heliograph-seal trust anchor` on the machine, and nothing that arrives
	// over a transport reaches that command.
	//
	// THE SENTENCE NAMES NO BINARY, and that was a correction. It read "changes
	// only with `heliograph-seal trust anchor`" until the golden vectors put the
	// two implementations side by side: the PowerShell station has no
	// heliograph-seal and never will, because its verification is managed C#. A
	// refusal that tells half the estate to run a command they do not have is
	// worse than one that names none. The exact command is printed by each
	// station's own startup banner, where it can be true.
	if c.Name == s.Anchor.Name {
		return s, refuse(ErrAnchor, "%q is the anchor, and it changes only on the machine itself, with the station's own trust anchor command",
			c.Name)
	}
	if c.Subject.Equal(s.Anchor.Public) {
		return s, refuse(ErrAnchor, "%s is the anchor's key, and it changes only on the machine itself, with the station's own trust anchor command",
			s.Anchor.Public.Fingerprint())
	}

	next := s
	next.members = s.Members() // a copy: the receiver is never mutated
	next.Serial = c.Serial
	next.UTC = stamp(now)

	switch c.Op {
	case OpAdd:
		for _, m := range next.members {
			if m.Public.Equal(c.Subject) {
				if !m.Active() {
					// A revoked key stays revoked. Re-adding one would let a
					// single compromised key resurrect the leaver whose access
					// was removed, which is the flow revocation exists for. A
					// returning colleague generates a new key, which is what
					// they should do anyway.
					return s, refuse(ErrRevokedKey, "%s (%s) was revoked on %s: a returning member enrols a new key",
						m.Name, m.Public.Fingerprint(), m.Revoked)
				}
				return s, refuse(ErrDuplicate, "%s already holds %s", m.Name, m.Public.Fingerprint())
			}
			if m.Name == c.Name && m.Active() {
				return s, refuse(ErrDuplicate, "%q is already a member", c.Name)
			}
		}
		next.members = append(next.members, Member{
			Name: c.Name, Public: c.Subject,
			Added: stampOr(c.UTC, now), AddedBy: c.Author,
		})
	case OpRevoke:
		found := false
		for i := range next.members {
			if !next.members[i].Public.Equal(c.Subject) {
				continue
			}
			found = true
			if !next.members[i].Active() {
				// Not an error worth refusing over: the outcome the author asked
				// for is already true. Refusing would make a retried leaver flow
				// look like a failure.
				return s, refuse(ErrNotAMember, "%s (%s) was already revoked on %s by %s",
					next.members[i].Name, c.Subject.Fingerprint(),
					next.members[i].Revoked, next.members[i].RevokedBy)
			}
			next.members[i].Revoked = stampOr(c.UTC, now)
			next.members[i].RevokedBy = c.Author
		}
		if !found {
			return s, refuse(ErrNotAMember, "no member holds %s", c.Subject.Fingerprint())
		}
	}
	return next, nil
}

// SetAnchor rotates the anchor, and is reachable ONLY from a command run on the
// machine itself.
//
// There is no document that reaches this, no transport verb that carries it and
// no request field that names it. That absence is the mechanism, and
// `TestNoRequestPathReachesTheAnchor` is what keeps it absent.
//
// The serial advances, because the set has changed and a control node holding
// the old one must notice rather than author a change against a set that no
// longer exists.
func (s Set) SetAnchor(name string, pub seal.PublicIdentity, now time.Time) (Set, error) {
	if name == "" {
		name = s.Anchor.Name
	}
	if err := validName(name); err != nil {
		return s, err
	}
	if len(pub.Sign) == 0 || len(pub.Enc) == 0 {
		return s, fmt.Errorf("trust: the new anchor has no key")
	}
	for _, m := range s.members {
		if m.Public.Equal(pub) {
			return s, fmt.Errorf("trust: %s already holds that key as a member: the anchor must be a key nothing else can revoke", m.Name)
		}
	}
	next := s
	next.members = s.Members()
	next.Serial = s.Serial + 1
	next.UTC = stamp(now)
	next.Anchor = Member{Name: name, Public: pub, Added: stamp(now), AddedBy: "operator"}
	return next, nil
}

func short(d string) string {
	if len(d) > 12 {
		return d[:12]
	}
	return d
}

// stampOr prefers the time the author recorded, so an audit line says when the
// change was authored rather than when this particular station got round to
// applying it. A station that has been offline for a week would otherwise
// record every backdated change as having happened today.
func stampOr(utc string, now time.Time) string {
	if utc != "" {
		return utc
	}
	return stamp(now)
}
