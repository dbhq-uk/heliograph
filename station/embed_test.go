package station_test

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/dbhq-uk/heliograph/station"
)

// The payload is small. A station is a directory of scripts, and the whole
// point of embedding it is that a release binary plants exactly the station it
// was built against - not that the binary becomes a way to ship anything else.
//
// 4 MB is generous for two payloads of text. It is here to catch a class, not
// to be tuned: if this number has to go up, something has started shipping that
// is not a script, and that is the thing to look at.
const maxPayloadBytes = 4 << 20

// WHY THIS TEST EXISTS, and it is not hypothetical.
//
// `go:embed all:bash` reads the WORKING TREE at build time. It knows nothing
// about .gitignore, so anything sitting in station/ when somebody types
// `go build` goes into the binary - permanently, in every download.
//
// Measured on a developer machine that had run the test suite: the payload was
// 957 MB and the binary 239 MB. 912 MB of that was
// station/bash/azure/*/.terraform, left by `terraform init`, which is exactly
// what `terraform test` runs - so doing the right thing to the templates was
// what created it. The repository never noticed, because those paths are
// gitignored, and CI never noticed, because a CI runner starts clean.
//
// internal/bootstrap already prunes .terraform, and that comment even names
// this risk - but it prunes at INSTALL time. By then the files are in the
// binary; the prune only stops them being written into a transport repo. The
// guard has to be here, where the embed is.
//
// A LOCAL ARTIFACT IS WORSE THAN A LARGE ONE. The same build carried
// station/bash/.station-delivery, which is a runtime record naming a path
// under /tmp on the machine that built it. Small, and it should not be in
// anybody's download.
func TestEmbeddedPayloadCarriesOnlyTheStation(t *testing.T) {
	// The station's own runtime state. Every one of these is written by a
	// station as it runs, is local to one machine, and is in the payload's
	// .gitignore precisely because it is nobody else's.
	localState := []string{
		".station.lock",
		".station-state",
		".station-approved",
		".station-approved-ps",
		".station-delivery",
		".station-env",
		".station-env-ps",
		".station-relay-state",
		".agent-service.pid",
		".station-service.log",
	}

	for _, p := range []struct {
		name string
		fsys fs.FS
		root string
	}{
		{"bash", station.Bash, "bash"},
		{"powershell", station.PowerShell, "powershell"},
	} {
		var total int64
		var bad []string
		err := fs.WalkDir(p.fsys, p.root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			total += fi.Size()

			base := path[strings.LastIndex(path, "/")+1:]
			switch {
			case strings.Contains(path, "/.terraform/"):
				bad = append(bad, fmt.Sprintf("%s (terraform init wrote it; rm -rf station/*/azure/*/.terraform)", path))
			case strings.Contains(path, "/node_modules/"):
				bad = append(bad, path+" (rm -rf it)")
			// A COMPILED ASSEMBLY, which is what the seal is most likely to
			// acquire. `station/powershell/lib/seal` is C# SOURCE on purpose -
			// the whole proposition is plain text an operator can read before
			// running it - and the obvious shortcut when Add-Type feels slow is
			// to drop a built .dll beside it. Two 200 KB DLLs would pass the
			// size cap below and be exactly what this test is for.
			case strings.HasSuffix(base, ".dll"), strings.HasSuffix(base, ".exe"),
				strings.HasSuffix(base, ".so"), strings.HasSuffix(base, ".dylib"),
				strings.HasSuffix(base, ".pdb"), strings.HasSuffix(base, ".nupkg"):
				bad = append(bad, path+" (a compiled artifact. The payload is plain text somebody can read before they run it; ship the source)")
			case strings.Contains(path, "/obj/"), strings.Contains(path, "/bin/Debug/"),
				strings.Contains(path, "/bin/Release/"):
				bad = append(bad, path+" (a .NET build directory; rm -rf it)")
			case strings.Contains(path, "/ops-logs/") && strings.HasSuffix(path, ".txt"):
				// A CAPTURED LOG IS THE WORST OF THESE. It holds whatever a step
				// printed on the machine that built the binary.
				//
				// Only `.txt`, because ops-logs/.gitkeep MUST ship - it is what
				// gives a planted station somewhere to write.
				bad = append(bad, path+" (a captured log - delete it, and check what is in it before you do)")
			default:
				for _, s := range localState {
					if base == s {
						bad = append(bad, path+" (a station's local runtime state; delete it)")
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("%s: %v", p.name, err)
		}

		for _, b := range bad {
			t.Errorf("the %s payload embeds something that is not part of the station: %s", p.name, b)
		}
		if total > maxPayloadBytes {
			t.Errorf("the %s payload is %.1f MB embedded, over the %.0f MB limit.\n"+
				"  go:embed reads the working tree, so this is in the binary and in every download.\n"+
				"  Look for generated directories under station/%s that .gitignore hides from you.",
				p.name, float64(total)/(1<<20), float64(maxPayloadBytes)/(1<<20), p.name)
		}
	}
}

// Both payloads must actually carry their entry points. A pattern change that
// dropped the dotfiles would leave a payload that looks complete and is not,
// and the failure would surface as a station planted without its ops-logs
// directory on a machine nobody can reach.
func TestEmbeddedPayloadCarriesWhatAStationNeeds(t *testing.T) {
	for _, c := range []struct {
		name  string
		fsys  fs.FS
		files []string
	}{
		{"bash", station.Bash, []string{
			"bash/run.sh", "bash/station.sh", "bash/start.sh", "bash/caplib.sh",
			"bash/gitignore", "bash/gitattributes",
			"bash/ops-logs/.gitkeep", "bash/steps/_template.sh",
			// Every transport, by name. A pattern change that dropped one
			// leaves a payload that plants cleanly and refuses the only
			// channel that estate permits, on a machine nobody can reach.
			"bash/transports/git.sh", "bash/transports/share.sh",
			"bash/transports/bundle.sh", "bash/transports/relay.sh",
			"bash/transports/blob.sh", "bash/transports/objstore.sh",
		}},
		{"powershell", station.PowerShell, []string{
			"powershell/run.ps1", "powershell/station.ps1", "powershell/start.ps1",
			"powershell/caplib.psm1", "powershell/lib/transport.psm1",
			"powershell/lib/cancel.psm1", "powershell/lib/stationenv.psm1",
			"powershell/service.ps1", "powershell/transports/git.psm1",
			"powershell/transports/share.psm1", "powershell/transports/relay.psm1",
			"powershell/gitignore", "powershell/gitattributes",
			"powershell/ops-logs/.gitkeep", "powershell/steps/_template.ps1",
			"powershell/station/request",
			// The seal, and it is named file by file rather than as a
			// directory. The relay transport does nothing without it, and a
			// pattern change that dropped one vendored .cs leaves a payload
			// that plants cleanly, starts cleanly, and fails at Add-Type on a
			// machine nobody can log into.
			"powershell/lib/seal.psm1",
			"powershell/lib/seal/ChaCha20Poly1305.cs",
			"powershell/lib/seal/Hkdf.cs",
			"powershell/lib/seal/vendor/Chaos.NaCl/Ed25519.cs",
			"powershell/lib/seal/vendor/Chaos.NaCl/Internal/Poly1305Donna.cs",
			"powershell/lib/seal/vendor/Chaos.NaCl/Internal/Ed25519Ref10/scalarmult.cs",
			// The licence travels with the code it licenses. Shipping vendored
			// MIT source without it is the one thing here that is somebody
			// else's problem as well as ours.
			"powershell/lib/seal/vendor/Chaos.NaCl/LICENSE.md",
			"powershell/lib/seal/vendor/Chaos.NaCl/ORIGIN.md",
		}},
	} {
		for _, f := range c.files {
			if _, err := fs.Stat(c.fsys, f); err != nil {
				t.Errorf("the %s payload does not carry %s: %v", c.name, f, err)
			}
		}
	}
}
