package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dbhq-uk/heliograph/station"
)

// The Go planter is the third way a payload reaches a machine, and the only one
// whose prune rules could silently drop the seal.
//
// Install walks the embedded tree and skips a named set of runtime artefacts.
// That list is matched on BASE NAME, so a rule added for `.station-state` could
// one day match something under lib/seal without anybody looking - and the
// result is a station that plants cleanly, starts cleanly, and fails at
// Add-Type on a machine nobody can log into.
//
// bootstrap.sh and bootstrap.ps1 copy a directory and are checked by
// tests/test-seal-ps1.sh. This is the one that needs a Go test.
func TestThePlanterCarriesTheSeal(t *testing.T) {
	d := t.TempDir()
	if _, err := Install(station.PowerShell, "powershell", d); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{
		"lib/seal.psm1",
		"lib/seal/ChaCha20Poly1305.cs",
		"lib/seal/Hkdf.cs",
		"lib/seal/vendor/Chaos.NaCl/Ed25519.cs",
		"lib/seal/vendor/Chaos.NaCl/Internal/Poly1305Donna.cs",
		"lib/seal/vendor/Chaos.NaCl/Internal/Ed25519Ref10/scalarmult.cs",
		// The licence travels with the code it licenses.
		"lib/seal/vendor/Chaos.NaCl/LICENSE.md",
		"transports/relay.psm1",
	} {
		if _, err := os.Stat(filepath.Join(d, f)); err != nil {
			t.Errorf("the planter did not carry %s, so the relay transport cannot work: %v", f, err)
		}
	}

	// AND ENOUGH OF IT. Naming six files would pass on a tree that had lost
	// fifty: the ref10 core is one function per file, and a partial copy fails
	// to compile rather than failing a named check.
	n := 0
	_ = filepath.Walk(filepath.Join(d, "lib/seal"), func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() && filepath.Ext(p) == ".cs" {
			n++
		}
		return nil
	})
	if n < 55 {
		t.Errorf("only %d C# files were planted under lib/seal; the ref10 core is one "+
			"function per file and a partial copy will not compile", n)
	}
}
