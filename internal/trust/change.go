package trust

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dbhq-uk/heliograph/internal/seal"
)

// Op is what a change does. One change does exactly one thing.
//
// ONE OPERATION PER CHANGE, and that is a design decision rather than a
// limitation. A change rides in the request document, which is `key: value`
// lines read by `sed -n 's/^op://p'` on the far side - and that reads the FIRST
// match only. A two-operation change would have had its second operation
// silently dropped by every station in the field, which is the shape of defect
// this repository keeps paying for. It also makes the audit trail one signed
// act per serial, which is what somebody reading it afterwards wants anyway.
const (
	OpAdd    = "add"
	OpRevoke = "revoke"
)

// Change is a signed instruction to alter one trusted set.
//
// It is bound to the set it applies to by three fields and not one:
//
//	Estate, Station  a change for one machine replayed at another
//	Serial           an old change replayed after a later one, and the reason
//	                 replay protection survives a restart: the serial is in the
//	                 set on disk, not in a window in memory
//	Prev             a change applied to a set that has since moved on, which is
//	                 how two administrators racing produce a fork nobody notices
type Change struct {
	Version int
	Estate  string
	Station string
	Serial  uint64 // must be exactly the current set's serial plus one
	Prev    string // the digest of the set this was authored against
	UTC     string

	Op      string
	Name    string              // the subject's name
	Subject seal.PublicIdentity // the subject's key

	Author    string // the author's name in the set, for the audit line
	AuthorKey seal.PublicIdentity
	Sig       []byte
}

// canonical is the byte string the signature covers. Everything except the
// signature itself, length-prefixed for the reason Set.canonical is.
func (c Change) canonical() []byte {
	var b []byte
	add := func(s string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(s)))
		b = append(b, n[:]...)
		b = append(b, s...)
	}
	num := func(v uint64) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], v)
		b = append(b, n[:]...)
	}
	num(uint64(c.Version))
	add(c.Estate)
	add(c.Station)
	num(c.Serial)
	add(c.Prev)
	add(c.UTC)
	add(c.Op)
	add(c.Name)
	add(c.Subject.Encode())
	add(c.Author)
	add(c.AuthorKey.Encode())
	return b
}

// SignChange is the ONLY thing in this repository that produces a valid change.
//
// IT TAKES A SECRET KEY, and that is the whole security argument for the
// product claim. A control plane that holds no key cannot call this usefully,
// so there is no code path by which a service adds itself to a trusted set and
// then signs requests legitimately. `TestNoControlPlanePathAuthorsAChange`
// asserts that nothing outside the human-driven commands even names it.
//
// It is a package-level function rather than a method on Change so that the
// call site reads as an act - somebody signed this - rather than as a field
// being filled in.
func SignChange(id *seal.Identity, c Change) (Change, error) {
	if id == nil {
		return Change{}, fmt.Errorf("trust: a change is signed with a secret key, and none was given")
	}
	c.Version = Version
	c.AuthorKey = id.Public()
	if err := c.validateShape(); err != nil {
		return Change{}, err
	}
	c.Sig = id.SignDocument(Domain, c.canonical())
	if c.Sig == nil {
		return Change{}, fmt.Errorf("trust: signing produced nothing")
	}
	return c, nil
}

// validateShape is everything checkable without a set to check it against.
func (c Change) validateShape() error {
	if c.Version != Version {
		return fmt.Errorf("trust: this change is version %d and this build reads version %d", c.Version, Version)
	}
	if c.Estate == "" || c.Station == "" {
		return fmt.Errorf("trust: a change must name its estate and station, or one machine's change applies at another")
	}
	if c.Op != OpAdd && c.Op != OpRevoke {
		return fmt.Errorf("trust: %q is not an operation: a change adds or revokes", c.Op)
	}
	if err := validName(c.Name); err != nil {
		return err
	}
	if err := validName(c.Author); err != nil {
		return err
	}
	if c.Prev == "" {
		return fmt.Errorf("trust: a change must name the set it was authored against, or it can be applied to a set that has moved on")
	}
	if len(c.Subject.Sign) == 0 || len(c.Subject.Enc) == 0 {
		return fmt.Errorf("trust: a change must carry the subject's key, so that a revocation names a key rather than a name somebody could reuse")
	}
	if len(c.AuthorKey.Sign) == 0 || len(c.AuthorKey.Enc) == 0 {
		return fmt.Errorf("trust: a change must carry the author's key, which is what it is verified against")
	}
	return nil
}

// Marshal renders the change as `key: value` lines.
//
// READABLE WITH `cat`, like everything else that crosses the gap. An estate
// owner has to be able to see what was asked of their station without tooling
// and without us, and a base64 blob in a request would defeat exactly the
// audit property this whole mechanism exists to provide.
func (c Change) Marshal() []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "trust-version: %d\n", c.Version)
	fmt.Fprintf(&b, "trust-estate: %s\n", c.Estate)
	fmt.Fprintf(&b, "trust-station: %s\n", c.Station)
	fmt.Fprintf(&b, "trust-serial: %d\n", c.Serial)
	fmt.Fprintf(&b, "trust-prev: %s\n", c.Prev)
	fmt.Fprintf(&b, "trust-utc: %s\n", c.UTC)
	fmt.Fprintf(&b, "trust-op: %s\n", c.Op)
	fmt.Fprintf(&b, "trust-subject: %s %s\n", c.Name, c.Subject.Encode())
	fmt.Fprintf(&b, "trust-author: %s %s\n", c.Author, c.AuthorKey.Encode())
	fmt.Fprintf(&b, "trust-sig: %s\n", base64.RawURLEncoding.EncodeToString(c.Sig))
	return b.Bytes()
}

// ParseChange reads one back, from a whole request document if need be.
//
// The keys are prefixed `trust-` precisely so a change can ride inside the
// request document a transport already carries. That saves a second fetch verb
// on every transport, and a second fetch on the relay would be worse than
// awkward: collection deletes, so asking twice eats the queue.
func ParseChange(b []byte) (Change, error) {
	var c Change
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	seen := false
	for sc.Scan() {
		k, v, ok := splitField(sc.Text())
		if !ok || !strings.HasPrefix(k, "trust-") {
			continue
		}
		seen = true
		var err error
		switch k {
		case "trust-version":
			c.Version, _ = strconv.Atoi(v)
		case "trust-estate":
			c.Estate = v
		case "trust-station":
			c.Station = v
		case "trust-serial":
			c.Serial, err = strconv.ParseUint(v, 10, 64)
			if err != nil {
				return Change{}, fmt.Errorf("trust: serial %q is not a number", v)
			}
		case "trust-prev":
			c.Prev = v
		case "trust-utc":
			c.UTC = v
		case "trust-op":
			c.Op = v
		case "trust-subject":
			c.Name, c.Subject, err = parseNamedKey(v)
			if err != nil {
				return Change{}, fmt.Errorf("trust: the subject line is unusable: %w", err)
			}
		case "trust-author":
			c.Author, c.AuthorKey, err = parseNamedKey(v)
			if err != nil {
				return Change{}, fmt.Errorf("trust: the author line is unusable: %w", err)
			}
		case "trust-sig":
			c.Sig, err = base64.RawURLEncoding.DecodeString(v)
			if err != nil {
				return Change{}, fmt.Errorf("trust: the signature is not decodable")
			}
		}
	}
	if err := sc.Err(); err != nil {
		return Change{}, err
	}
	if !seen {
		return Change{}, ErrNoChange
	}
	return c, nil
}

func parseNamedKey(v string) (string, seal.PublicIdentity, error) {
	f := strings.Fields(v)
	if len(f) != 2 {
		return "", seal.PublicIdentity{}, fmt.Errorf("expected `<name> <key>`, got %q", v)
	}
	p, err := seal.DecodePublic(f[1])
	if err != nil {
		return "", seal.PublicIdentity{}, err
	}
	return f[0], p, nil
}

// NewChange builds an unsigned change against a set. It is deliberately not
// enough on its own: Apply refuses anything whose signature does not verify.
func NewChange(s Set, op, name string, subject seal.PublicIdentity, author Member, now time.Time) Change {
	return Change{
		Version: Version,
		Estate:  s.Estate,
		Station: s.Station,
		Serial:  s.Serial + 1,
		Prev:    s.Digest(),
		UTC:     stamp(now),
		Op:      op,
		Name:    name,
		Subject: subject,
		Author:  author.Name,
	}
}
