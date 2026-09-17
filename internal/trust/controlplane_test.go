package trust

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The tests in this file are the ones that carry the product claim, so they
// assert over SOURCE rather than over behaviour.
//
// "The service cannot cause a station to run anything" is not a statement about
// what the code does when you run it. It is a statement about what code exists.
// A behavioural test can only show that the paths somebody thought of do not
// author a change; these show that no path does, because the identifier that
// authors one appears nowhere it could be reached from a service.
//
// They will fail when somebody adds a legitimate new call site, and that is the
// point: adding one is a decision that has to be made deliberately, in a diff
// that says so, rather than by an import.

// repoRoot walks up from this package to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find the module root from the trust package")
	return ""
}

// eachGoFile visits every .go file in the module, test files included.
//
// TEST FILES INCLUDED, deliberately. A helper in a test that mints a change is
// still a code path, and a service that vendored it would have one too.
func eachGoFile(t *testing.T, root string, fn func(rel string, src []byte, f *ast.File, fset *token.FileSet)) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "site":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fn(filepath.ToSlash(rel), src, f, fset)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestNoControlPlanePathAuthorsAChange is the assertion the whole claim rests
// on, in its structural form.
//
// SignChange is the only function in this repository that produces a valid
// trusted-set change, and it takes a secret key. This asserts that it is named
// in exactly three places, all of them commands a person types on their own
// machine, and nowhere on any path a hosted service could run.
//
// The allowlist is short on purpose. `internal/transport/` is what moves
// documents and is what a service would embed; it is not on the list, and
// TestTheDocumentMoversCannotEvenNameAChange below states that separately.
func TestNoControlPlanePathAuthorsAChange(t *testing.T) {
	root := repoRoot(t)

	// Where a human, holding their own key, signs a change. Nothing else.
	//
	// cmd/heliograph-seal IS NOT ON THIS LIST, and that is deliberate. That
	// binary runs on the far side, on a machine we do not control; it verifies
	// and it does not author. See TestTheStationsBinaryCannotAuthorAChange.
	allowed := map[string]bool{
		"internal/trust/change.go":            true, // the definition
		"internal/trust/trust_test.go":        true, // this package's own tests
		"internal/trust/controlplane_test.go": true, // this file, which names it
		"internal/trust/vectors_test.go":      true, // the golden vectors, which must sign to pin a signature
		"cmd/heliograph/trust.go":             true, // `heliograph trust add|revoke`
		"cmd/heliograph/trust_test.go":        true,
		// THE ADVERSARY, and it is on the list deliberately. It authors changes
		// no legitimate command would - revoking an anchor, signing as somebody
		// else - so that tests/test-trusted-set.sh asserts what the STATION
		// refuses rather than what our CLI declines to send. Under tests/, not
		// a cmd/, and nothing installs it.
		"tests/forge/main.go": true,
	}

	var offenders []string
	eachGoFile(t, root, func(rel string, src []byte, f *ast.File, fset *token.FileSet) {
		if allowed[rel] {
			return
		}
		ast.Inspect(f, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || id.Name != "SignChange" {
				return true
			}
			offenders = append(offenders, rel+":"+strconv.Itoa(fset.Position(id.Pos()).Line))
			return true
		})
	})
	if len(offenders) > 0 {
		t.Errorf("SignChange is reachable from code that is not a person's own command:\n  %s\n"+
			"A trusted-set change may only be authored by somebody holding a secret key on their own machine. "+
			"If this is a legitimate new command, add it to the allowlist in this test and say why in the commit.",
			strings.Join(offenders, "\n  "))
	}
}

// TestTheDocumentMoversCannotEvenNameAChange keeps the transports ignorant.
//
// internal/transport is what a hosted control plane embeds in order to move
// requests, statuses and logs. If it could name a trusted-set change it could
// construct one, and the only remaining barrier would be a key it might one day
// be given. Not importing the package at all is a stronger statement than not
// calling the function.
func TestTheDocumentMoversCannotEvenNameAChange(t *testing.T) {
	root := repoRoot(t)
	const trustPkg = `"github.com/heliograph-io/heliograph/internal/trust"`

	var offenders []string
	eachGoFile(t, root, func(rel string, src []byte, f *ast.File, fset *token.FileSet) {
		if !strings.HasPrefix(rel, "internal/transport/") {
			return
		}
		for _, imp := range f.Imports {
			if imp.Path.Value == trustPkg {
				offenders = append(offenders, rel)
			}
		}
	})
	if len(offenders) > 0 {
		t.Errorf("the transport layer imports internal/trust: %s\n"+
			"Transports move documents and must not be able to construct a trusted-set change. "+
			"Verification belongs in the station and in heliograph-seal.", strings.Join(offenders, ", "))
	}
}

// TestTheStationsBinaryCannotAuthorAChange keeps the far-side binary a verifier.
//
// heliograph-seal is the one binary permitted past the station boundary. It
// lives on a machine we do not control, in an estate where somebody else
// decides who can read it. If it could mint a trusted-set change, then anybody
// who could run it there could add a key - and the whole chain, whose first
// link is the operator planting an anchor, would have a second entrance.
func TestTheStationsBinaryCannotAuthorAChange(t *testing.T) {
	root := repoRoot(t)
	var offenders []string
	eachGoFile(t, root, func(rel string, src []byte, f *ast.File, fset *token.FileSet) {
		if !strings.HasPrefix(rel, "cmd/heliograph-seal/") {
			return
		}
		for _, name := range []string{"SignChange", "SignDocument"} {
			if strings.Contains(string(src), name) {
				offenders = append(offenders, rel+" names "+name)
			}
		}
	})
	if len(offenders) > 0 {
		t.Errorf("the station's binary can author, not only verify:\n  %s\n"+
			"heliograph-seal runs on a machine we do not control. Verification belongs there; signing does not.",
			strings.Join(offenders, "\n  "))
	}
}

// TestNothingOutsideThisPackageMutatesASet is the compiler-enforced half, made
// explicit so it cannot be undone without a failing test.
//
// Set.members is unexported, so no package outside this one can add a member.
// This asserts the field really is unexported - a rename to Members would make
// every trusted set in the product editable by anything that holds one, and it
// is exactly the sort of change that looks like tidying.
func TestNothingOutsideThisPackageMutatesASet(t *testing.T) {
	ty := reflect.TypeOf(Set{})
	f, ok := ty.FieldByName("members")
	if !ok {
		t.Fatal("Set no longer has a members field: if it was renamed, the trusted set is now mutable by any holder")
	}
	if f.PkgPath == "" {
		t.Error("Set.members is exported, so any package holding a set can add a key to it without a signature")
	}
	for i := 0; i < ty.NumField(); i++ {
		name := ty.Field(i).Name
		if name == "Anchor" || ty.Field(i).PkgPath != "" {
			continue
		}
		// The remaining exported fields are scalars describing the set, and
		// altering one changes the digest, so a tampered copy fails Apply's
		// Prev check at the next change. A slice or a map here would not.
		switch ty.Field(i).Type.Kind() {
		case reflect.Slice, reflect.Map, reflect.Ptr:
			t.Errorf("Set.%s is an exported %s: a holder could alter the set without a signature",
				name, ty.Field(i).Type.Kind())
		}
	}
}

// TestAnUnsignedChangeIsRefused states the behavioural half of the same claim.
//
// A control plane that assembled a change document by hand, with every field
// right and no signature, gets nowhere. There is no permissive mode, no
// "unverified but recorded" state and no flag that turns one on.
func TestAnUnsignedChangeIsRefused(t *testing.T) {
	p := newPeople(t)
	s := mustSet(t, p)

	unsigned := NewChange(s, OpAdd, "cloud", p.mallory.Public(), Member{Name: "owner"}, at("2026-02-01T00:00:00Z"))
	unsigned.AuthorKey = p.anchor.Public() // it even claims the anchor
	if _, err := s.Apply(unsigned, at("2026-02-01T00:00:00Z")); err == nil {
		t.Fatal("a change with no signature was applied")
	}

	// And a change signed for a DIFFERENT domain, which is what a service with
	// a key for some other purpose would be able to produce.
	other := unsigned
	other.Sig = p.anchor.SignDocument("heliograph-something-else-v1", other.canonical())
	if _, err := s.Apply(other, at("2026-02-01T00:00:00Z")); err == nil {
		t.Error("a signature made under another domain was accepted as a trusted-set change")
	}
}

// TestNoRequestPathReachesTheAnchor keeps rule 3 a property of the code.
//
// SetAnchor is the only thing that moves an anchor. It must be named only by
// the command that runs ON the machine. A call from anywhere that parses a
// document would mean an anchor changeable from the far side, and the recovery
// property - the estate owner can never be locked out remotely - would be gone
// with nothing in the tests to say so.
func TestNoRequestPathReachesTheAnchor(t *testing.T) {
	root := repoRoot(t)
	allowed := map[string]bool{
		"internal/trust/apply.go":             true,
		"internal/trust/trust_test.go":        true,
		"internal/trust/controlplane_test.go": true,
		"cmd/heliograph-seal/trust.go":        true, // `heliograph-seal trust anchor`, run ON the machine
		"cmd/heliograph-seal/trust_test.go":   true,
	}
	var offenders []string
	eachGoFile(t, root, func(rel string, src []byte, f *ast.File, fset *token.FileSet) {
		if allowed[rel] {
			return
		}
		ast.Inspect(f, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || id.Name != "SetAnchor" {
				return true
			}
			offenders = append(offenders, rel+":"+strconv.Itoa(fset.Position(id.Pos()).Line))
			return true
		})
	})
	if len(offenders) > 0 {
		t.Errorf("SetAnchor is named outside the command that runs on the machine:\n  %s\n"+
			"The anchor is what stops an estate owner being locked out of their own machine remotely.",
			strings.Join(offenders, "\n  "))
	}
}

// TestAChangeSucceedsWithNoServiceInvolved is #29's "works with heliograph
// cloud unreachable" box, asserted where it is actually decided.
//
// Nothing in this package dials anything. A change is authored from a set and a
// key, applied against a set, and verified with no network, no clock authority
// and no third party. If a service were required, losing it would lock a
// customer out of their own estate AND compromising it would be equivalent to
// holding a key.
func TestAChangeSucceedsWithNoServiceInvolved(t *testing.T) {
	root := repoRoot(t)
	banned := []string{`"net/http"`, `"net"`, `"net/url"`, `"os/exec"`}
	var offenders []string
	eachGoFile(t, root, func(rel string, src []byte, f *ast.File, fset *token.FileSet) {
		if !strings.HasPrefix(rel, "internal/trust/") {
			return
		}
		for _, imp := range f.Imports {
			for _, b := range banned {
				if imp.Path.Value == b {
					offenders = append(offenders, rel+" imports "+b)
				}
			}
		}
	})
	if len(offenders) > 0 {
		t.Errorf("the trusted set reaches outside the machine it runs on:\n  %s", strings.Join(offenders, "\n  "))
	}

	// And the round trip itself, with nothing but bytes.
	p := newPeople(t)
	s := mustSet(t, p)
	c := sign(t, s, p.anchor, "owner", OpAdd, "alice", p.alice.Public())
	// The change is written out, carried by hand, read back, and applied. That
	// is a USB stick, a chat message or a bundle: no service on the path.
	back, err := ParseChange(c.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.Apply(back, time.Now())
	if err != nil {
		t.Fatalf("a change carried by hand was refused: %v", err)
	}
	if len(next.Members()) != 1 {
		t.Error("the offline change did not take effect")
	}
}
