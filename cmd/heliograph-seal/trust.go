package main

// The trusted set, on the station's side of the gap.
//
// THIS FILE VERIFIES AND NEVER AUTHORS. There is no `trust sign` verb, and
// `TestTheStationsBinaryCannotAuthorAChange` asserts that neither of the two
// signing functions is so much as named anywhere under cmd/heliograph-seal -
// as a substring of the source, comments included, so this paragraph cannot
// spell either of them out. That matters because this binary sits on the far
// side, on a machine we do not control, in an estate where somebody else
// decides who can read it: a binary that could mint a trusted-set change would
// be one that could mint it from there.
//
// WHY THIS IS IN THIS BINARY AND NOT A NEW ONE. `station/bash/` may never grow
// a Go dependency, a binary or a package requirement - see AGENTS.md, "The
// boundary that must not be crossed". Ed25519 cannot be done in bash with
// coreutils, and `openssl` has no verb for it that is present across the
// versions this runs on. heliograph-seal is the one binary already argued for
// on the far side, so the trusted set uses it rather than shipping a second.
// The cost of that choice, and the alternatives, are written up on
// heliograph-io/heliograph-cloud#29.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/heliograph-io/heliograph/internal/seal"
	"github.com/heliograph-io/heliograph/internal/trust"
)

const trustUsage = `heliograph-seal trust - the set of keys this station accepts a request from

  heliograph-seal trust init   --set F --estate E --station S --anchor F|VALUE [--name owner]
  heliograph-seal trust show   --set F
  heliograph-seal trust digest --set F
  heliograph-seal trust verify --set F --in F
  heliograph-seal trust apply  --set F --in F
  heliograph-seal trust anchor --set F --anchor F|VALUE [--name owner]

init and anchor are LOCAL ONLY. Nothing that arrives over a transport reaches
either of them, which is what stops the machine's owner being locked out of
their own estate by a key somebody else holds.

There is no sign verb here on purpose: this binary runs on the far side and
verifies. A change is authored with ` + "`heliograph trust`" + ` on the machine of whoever
holds the key.
`

func cmdTrust(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, trustUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "init":
		return trustInit(args[1:])
	case "show":
		return trustShow(args[1:], false)
	case "digest":
		return trustShow(args[1:], true)
	case "verify":
		return trustApply(args[1:], false)
	case "apply":
		return trustApply(args[1:], true)
	case "anchor":
		return trustAnchor(args[1:])
	default:
		fmt.Fprint(os.Stderr, trustUsage)
		os.Exit(2)
	}
	return nil
}

// readPublic takes a path or the value itself.
//
// BOTH, because both are what happens. The anchor is the control side's public
// identity, and it reaches the operator pasted into a chat message as often as
// it reaches them as a file. Refusing the paste would mean telling somebody on
// a locked-down machine to create a file first, and they would use `echo`,
// which is one shell quoting accident away from an anchor nobody intended.
func readPublic(v string) (seal.PublicIdentity, error) {
	if v == "" {
		return seal.PublicIdentity{}, fmt.Errorf("--anchor is required: it is the key this station's owner keeps")
	}
	if _, err := os.Stat(v); err == nil {
		return seal.LoadPeerFile(v)
	}
	return seal.DecodePublic(strings.TrimSpace(v))
}

func loadSet(path string) (trust.Set, error) {
	if path == "" {
		return trust.Set{}, fmt.Errorf("--set is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return trust.Set{}, err
	}
	return trust.ParseSet(b)
}

// writeSet replaces the file atomically.
//
// WRITE-THEN-RENAME, and it is not tidiness. A truncated trusted set does not
// parse, and a station that cannot parse its trusted set accepts nothing - on a
// machine nobody can log into to fix it. The relay transport learned the same
// lesson with its sequence file, where a truncated one read as sequence zero and
// accepted every replay the relay had ever seen.
func writeSet(path string, s trust.Set) error {
	tmp := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	if err := os.WriteFile(tmp, s.Marshal(), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func trustInit(args []string) error {
	fs := flag.NewFlagSet("trust init", flag.ExitOnError)
	set := fs.String("set", "", "where to write the trusted set")
	estate := fs.String("estate", "", "the estate this station belongs to")
	station := fs.String("station", "", "this station's name")
	anchor := fs.String("anchor", "", "the anchor's public identity, as a file or the value")
	name := fs.String("name", trust.AnchorName, "what to call the anchor")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *set == "" {
		return fmt.Errorf("--set is required")
	}
	// REFUSES TO REPLACE, for the reason seal.WriteIdentityFile does. A trusted
	// set overwritten by accident is a station that no longer accepts anybody it
	// accepted a minute ago, and the only recovery is somebody standing at the
	// machine.
	if _, err := os.Stat(*set); err == nil {
		return fmt.Errorf("%s already exists: refusing to replace a trusted set, because everything currently trusted would stop being trusted. Use `trust anchor` to rotate the anchor", *set)
	}
	pub, err := readPublic(*anchor)
	if err != nil {
		return err
	}
	s, err := trust.NewSet(*estate, *station, *name, pub, time.Now())
	if err != nil {
		return err
	}
	if err := writeSet(*set, s); err != nil {
		return err
	}
	fmt.Printf("trusted set planted at %s\n", *set)
	fmt.Printf("  estate:  %s\n  station: %s\n", s.Estate, s.Station)
	fmt.Printf("  anchor:  %s %s\n", s.Anchor.Name, s.Anchor.Public.Fingerprint())
	fmt.Printf("  digest:  %s\n", s.Digest())
	fmt.Println()
	fmt.Println("Read that anchor fingerprint back to the control side over a channel they")
	fmt.Println("already trust. The anchor changes only here, on this machine.")
	return nil
}

func trustShow(args []string, digestOnly bool) error {
	fs := flag.NewFlagSet("trust show", flag.ExitOnError)
	set := fs.String("set", "", "the trusted set")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := loadSet(*set)
	if err != nil {
		return err
	}
	if digestOnly {
		fmt.Println(s.Digest())
		return nil
	}
	fmt.Printf("estate:  %s\nstation: %s\nserial:  %d\ndigest:  %s\n",
		s.Estate, s.Station, s.Serial, s.Digest())
	// THE AUDIT LINE, AS A FIELD, so the station can read it with sed rather
	// than reassemble it from the human-readable lines below. It did the latter
	// for one commit, and the pipeline lower-cased REVOKED - so the published
	// line differed between the bash and PowerShell stations, and an owner
	// grepping for it would have found it on one and not the other.
	fmt.Printf("members: %s\n", s.Fingerprints())
	fmt.Printf("anchor:  %-16s %s   (changeable only on this machine)\n",
		s.Anchor.Name, s.Anchor.Public.Fingerprint())
	for _, m := range s.Members() {
		state := fmt.Sprintf("added %s by %s", m.Added, m.AddedBy)
		if !m.Active() {
			state = fmt.Sprintf("REVOKED %s by %s", m.Revoked, m.RevokedBy)
		}
		fmt.Printf("member:  %-16s %s   %s\n", m.Name, m.Public.Fingerprint(), state)
	}
	return nil
}

// trustApply verifies a change and, when asked, writes the result.
//
// The refusal goes to stdout rather than stderr, in one line, because the
// caller is station.sh and it publishes that line as the status reason. A
// reason that only reached a log on the far side would be a refusal nobody can
// see, which is indistinguishable from a station that died.
func trustApply(args []string, write bool) error {
	fs := flag.NewFlagSet("trust apply", flag.ExitOnError)
	set := fs.String("set", "", "the trusted set")
	in := fs.String("in", "", "the document carrying the change; a request document is fine")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := loadSet(*set)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	c, err := trust.ParseChange(raw)
	if err != nil {
		// "no change here" is the ordinary case for every request that is only a
		// step. Exit 3 so the caller can tell it apart from a refusal, which is
		// something somebody has to be told about.
		fmt.Printf("none\n")
		os.Exit(3)
	}
	next, err := s.Apply(c, time.Now())
	if err != nil {
		fmt.Printf("refused %s\n", oneLine(err.Error()))
		os.Exit(1)
	}
	if write {
		if err := writeSet(*set, next); err != nil {
			// NOT APPLIED, and said so. A change verified but not recorded would
			// be re-offered at the next poll and refused as out of order, which
			// reads as a broken control side rather than as a full disk.
			fmt.Printf("refused the change verified but could not be recorded at %s: %v\n", *set, err)
			os.Exit(1)
		}
	}
	fmt.Printf("applied %s %s %s by %s serial %d digest %s\n",
		c.Op, c.Name, subjectFingerprint(c), c.Author, next.Serial, next.Digest())
	return nil
}

func subjectFingerprint(c trust.Change) string { return c.Subject.Fingerprint() }

// oneLine keeps a refusal to a single status line. A newline in a reason forges
// a second key in the status document, which is the same defect wire.Validate
// exists to prevent on the way out.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

// trustAnchor is the recovery path, and it runs HERE.
//
// The estate owner keeps the anchor. If every other key is lost or compromised,
// this is what puts the estate back under their control, and it needs nobody's
// permission but physical or account access to the machine. There is
// deliberately no remote equivalent.
func trustAnchor(args []string) error {
	fs := flag.NewFlagSet("trust anchor", flag.ExitOnError)
	set := fs.String("set", "", "the trusted set")
	anchor := fs.String("anchor", "", "the new anchor's public identity, as a file or the value")
	name := fs.String("name", "", "what to call it; unchanged if omitted")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := loadSet(*set)
	if err != nil {
		return err
	}
	pub, err := readPublic(*anchor)
	if err != nil {
		return err
	}
	next, err := s.SetAnchor(*name, pub, time.Now())
	if err != nil {
		return err
	}
	if err := writeSet(*set, next); err != nil {
		return err
	}
	fmt.Printf("anchor is now %s %s\n", next.Anchor.Name, next.Anchor.Public.Fingerprint())
	fmt.Printf("serial %d, digest %s\n", next.Serial, next.Digest())
	fmt.Println("Every member is unchanged. Tell the control side the new digest so it")
	fmt.Println("authors its next change against this set rather than the old one.")
	return nil
}
