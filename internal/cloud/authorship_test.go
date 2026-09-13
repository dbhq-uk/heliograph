package cloud

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// THE CLAIM THIS FILE GUARDS, and it is the central claim of the product.
//
// The service is given the RECEIVING half of the control identity and never the
// signing half (`internal/seal/seal.go`: two keypairs, and `Seal()` needs the
// signing one). So a relay, a broker and an archive can all be compromised at
// once and none of them can cause a command to run inside a customer estate: "a
// relay that can read logs is a privacy problem. A relay that can FORGE a
// request has code execution inside every customer estate at once."
//
// `push` is the first code in this repository that talks to a hosted service on
// the customer's behalf, so it is the first place that claim could quietly stop
// being true - not by anybody deciding to break it, but by a helpful import.
//
// THREE ASSERTIONS, BECAUSE ONE IS NOT ENOUGH:
//
//  1. this package cannot REACH the code that signs, transitively
//  2. this package cannot CONSTRUCT the document a station would execute
//  3. driving the real binary sends nothing to a station's request queue
//
// The third lives in `cmd/heliograph/push_e2e_test.go`, because it needs the
// binary and a station. Each of the three has been broken on purpose and
// watched to catch it; the PR that added them carries the output.

// authorshipForbidden is the set of packages that can author or publish a
// request, and therefore the set this one may not reach.
//
// `internal/seal` is the only way to author a request a station will verify
// over a relay: `Seal()` takes an *Identity, and an Identity holds the Ed25519
// signing key.
//
// `internal/transport` is the only way to publish one over anything else. Every
// transport implements `PutRequest`, and on git, a share, a bundle or an object
// store a request is a plain document at a known path - no signature required,
// because the transport itself is the trust boundary there.
//
// Between them they are every route from this process to a station's request
// queue. Without both, no code path in this package can cause a run, whatever
// it is asked to do.
var authorshipForbidden = []string{
	"github.com/dbhq-uk/heliograph/internal/seal",
	"github.com/dbhq-uk/heliograph/internal/transport",
}

// 1. The dependency graph, transitively.
//
// `go list -deps` rather than reading the import block, because an import three
// packages away signs just as well as a direct one. This is the assertion that
// keeps holding when somebody adds a helper in a year's time.
func TestThePushPathCannotReachTheCodeThatSignsARequest(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	deps := strings.Fields(string(out))
	if len(deps) == 0 {
		t.Fatal("go list returned nothing, so this guard looked at no dependencies at all")
	}
	// A liveness check. A guard whose input is empty reports a clean run, which
	// is how a mutation test's passes become silence.
	found := false
	for _, d := range deps {
		if d == "github.com/dbhq-uk/heliograph/internal/wire" {
			found = true
		}
		for _, bad := range authorshipForbidden {
			if d == bad {
				t.Errorf("internal/cloud reaches %s.\n"+
					"  That package can author or publish a request, and the product's central\n"+
					"  claim is that this path cannot. Nothing here needs it: a status document\n"+
					"  is read with internal/wire, and every byte uploaded came off disk", bad)
			}
		}
	}
	if !found {
		t.Fatal("internal/cloud no longer depends on internal/wire, so this list is not the list " +
			"this guard was written against and its silence proves nothing")
	}
}

// 2. The document itself.
//
// `internal/wire` IS imported, and deliberately: a status document is a
// `key: value` text format and a second parser for it is a format that drifts
// silently. But that package also holds `Request`, and `Request.Marshal()`
// renders the exact bytes a station acts on.
//
// Rendering those bytes is not by itself dangerous - nothing here can deliver
// them - but a package that constructs a request is one keystroke from a
// package that hands it somewhere. The assertion is therefore that the type is
// never named at all, which is checkable, unambiguous, and fails loudly the
// moment somebody starts.
func TestThePushPathNeverConstructsARequestDocument(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		// The guard is about SHIPPED code. This file names the type in its own
		// prose and the whole point of it is to catch a production use.
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing the package: %v", err)
	}
	files := 0
	sawWire := false
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			files++
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok || ident.Name != "wire" {
					return true
				}
				sawWire = true
				switch sel.Sel.Name {
				case "Request", "ParseRequest", "NewID":
					t.Errorf("%s names wire.%s.\n"+
						"  Those build or read the document a station executes. This package forwards\n"+
						"  evidence and may not construct a trigger", name, sel.Sel.Name)
				}
				return true
			})
		}
	}
	if files == 0 {
		t.Fatal("no non-test files were parsed, so this guard read nothing")
	}
	if !sawWire {
		t.Fatal("no reference to the wire package was found at all, so this scan would pass " +
			"on a package that had stopped importing it and started writing its own request")
	}
}

// And the shape of the claim, stated where somebody changing this package will
// read it. A comment is not a guard, so this asserts the comment is there:
// removing the explanation is how the next person concludes the import would be
// harmless.
func TestTheClaimIsStatedInTheCodeThatKeepsIt(t *testing.T) {
	b, err := os.ReadFile("spool.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"cannot author a request", "seal.Seal", "PutRequest"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("the package comment no longer says %q, and the next person to add an import "+
				"will not know why it matters", want)
		}
	}
}
