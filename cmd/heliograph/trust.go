package main

// `heliograph trust` - administering who may command a station.
//
// THIS IS THE ONLY PLACE IN THE PRODUCT THAT AUTHORS A CHANGE, and it runs on
// the machine of whoever holds the key. That is the whole security argument:
// a service could display a set, propose a change and record that one happened,
// and none of those produce a signature. `TestNoControlPlanePathAuthorsAChange`
// in internal/trust asserts that this file and this file alone reaches
// trust.SignChange.
//
// THE CONTROL SIDE KEEPS ITS OWN COPY of each station's set, at
// <keys>/<estate>.trusted-set. It has to: a change names the serial and the
// digest of the set it was authored against, so authoring one requires knowing
// what the station currently holds. The copy is also what `heliograph doctor`
// compares the station's published set against - and a divergence between the
// two is exactly the alarm this mechanism exists to raise.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dbhq-uk/heliograph/internal/estate"
	"github.com/dbhq-uk/heliograph/internal/seal"
	"github.com/dbhq-uk/heliograph/internal/trust"
	"github.com/dbhq-uk/heliograph/internal/wire"
)

const trustUsage = `heliograph trust - who may command a station

  heliograph trust init   [-e <estate>] [--name <anchor name>]
  heliograph trust show   [-e <estate>]
  heliograph trust add    [-e <estate>] <name> <public-identity|file>
  heliograph trust revoke [-e <estate>] <name>

A change is a signed document. It travels over the transport the estate already
uses, is verified by the station against the set as it stands, and needs no
service on the path: Heliograph Cloud can be unreachable and a change still
lands. Nothing here can alter a station's anchor, which changes only on the
machine itself.
`

// trustSetPath is the control side's copy of one station's set.
func trustSetPath(name string) (string, error) {
	dir, err := estate.KeyDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".trusted-set"), nil
}

func loadLocalSet(name string) (trust.Set, error) {
	p, err := trustSetPath(name)
	if err != nil {
		return trust.Set{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return trust.Set{}, fmt.Errorf("no trusted set recorded for estate %q: run `heliograph trust init -e %s`, and plant the same anchor on the station", name, name)
		}
		return trust.Set{}, err
	}
	return trust.ParseSet(b)
}

func saveLocalSet(name string, s trust.Set) error {
	p, err := trustSetPath(name)
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.%d.tmp", p, os.Getpid())
	if err := os.WriteFile(tmp, s.Marshal(), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func cmdTrust(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, trustUsage)
		return fmt.Errorf("give a trust command")
	}
	switch args[0] {
	case "init":
		return cmdTrustInit(args[1:])
	case "show":
		return cmdTrustShow(args[1:])
	case "add":
		return cmdTrustChange(args[1:], trust.OpAdd)
	case "revoke":
		return cmdTrustChange(args[1:], trust.OpRevoke)
	default:
		fmt.Fprint(os.Stderr, trustUsage)
		return fmt.Errorf("unknown trust command %q", args[0])
	}
}

// parseTrustArgs splits flags from positionals WITHOUT letting a positional
// that begins with a hyphen be read as a flag.
//
// THE BUG THIS EXISTS TO FIX WAS FOUND BY DRIVING THE REAL COMMAND. A public
// identity is base64url, so roughly one in every sixty-four of them begins with
// `-`:
//
//	$ heliograph trust add alice -62pYcHnbZjMvfPoxMQH6DKNYM1RLOAdpnUSpOyE0hC...
//	flag provided but not defined: -62pYcHnbZjMvfPoxMQH6DKNYM1RLOAdpnUSpOyE0hC...
//
// The shared parse() interleaves flags and positionals on purpose, so that
// `heliograph send step -e estate` works, and that is what makes a hyphenated
// value indistinguishable from a flag. `--` before the key would fix it and
// nobody would know to type it: the failure lands on somebody enrolling a
// colleague, once every sixty-four colleagues, with an error naming their
// key as though it were a typo.
//
// So this walks the arguments and consults the FlagSet for what is actually a
// flag. An unregistered `-something` is a positional, which is the only reading
// that can be right: this command has no variadic flags and never will.
func parseTrustArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if len(a) > 1 && a[0] == '-' && fs.Lookup(name) != nil {
			flags = append(flags, a)
			// A value follows unless it is a boolean, and unless it was given
			// as --flag=value.
			if !strings.Contains(a, "=") && !isBoolFlag(fs, name) && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return pos, nil
}

func isBoolFlag(fs *flag.FlagSet, name string) bool {
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// identityFor is the key this control node signs with, for one estate.
//
// GENERATED ON DEMAND, for the reason `init --transport relay` generates one:
// a trusted set is not a relay feature, and telling somebody on a git estate to
// go and run a second binary to make a key is a step nobody needs. This side
// already has the same code the station's heliograph-seal runs.
//
// It goes beside the estate file, in the same 0700 directory, at mode 600.
// Never IN the estate file: that gets copied between machines and ends up in
// backups.
func identityFor(e estate.Estate) (*seal.Identity, estate.Estate, error) {
	if e.Identity != "" {
		id, err := seal.LoadIdentityFile(e.Identity)
		return id, e, err
	}
	keys, err := estate.KeyDir()
	if err != nil {
		return nil, e, err
	}
	p := filepath.Join(keys, e.Name+".identity.json")
	if _, statErr := os.Stat(p); statErr != nil {
		id, genErr := seal.Generate()
		if genErr != nil {
			return nil, e, genErr
		}
		if err := seal.WriteIdentityFile(p, id); err != nil {
			return nil, e, err
		}
		fmt.Printf("generated a signing identity for %s at %s (mode 600)\n", e.Name, p)
	}
	id, err := seal.LoadIdentityFile(p)
	if err != nil {
		return nil, e, err
	}
	e.Identity = p
	if err := e.Save(); err != nil {
		return nil, e, err
	}
	return id, e, nil
}

// cmdTrustInit records the control side's copy of a set the operator is about
// to plant, or has just planted.
//
// THE ANCHOR IS THIS CONTROL NODE'S OWN PUBLIC IDENTITY, which is exactly what
// `heliograph plant` already tells the operator to record as RELAY_PEER. So the
// chain's first link is unchanged: the person who owns the machine puts it
// there, by hand, having read the fingerprint back over a channel they already
// trust.
func cmdTrustInit(args []string) error {
	fs := flag.NewFlagSet("trust init", flag.ExitOnError)
	name := estateFlag(fs)
	anchorName := fs.String("name", trust.AnchorName, "what to call the anchor")
	if err := fs.Parse(args); err != nil {
		return err
	}
	e, err := resolve(*name)
	if err != nil {
		return err
	}
	id, e, err := identityFor(e)
	if err != nil {
		return err
	}
	p, err := trustSetPath(e.Name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("%s already exists: this is the copy every change is authored against, and replacing it would author the next change against a set the station has never held", p)
	}
	station := e.Routing()
	relayEstate := e.RelayEstate
	if relayEstate == "" {
		relayEstate = e.Name
	}
	s, err := trust.NewSet(relayEstate, station, *anchorName, id.Public(), time.Now())
	if err != nil {
		return err
	}
	if err := saveLocalSet(e.Name, s); err != nil {
		return err
	}
	fmt.Printf("trusted set recorded at %s\n", p)
	fmt.Printf("  anchor: %s %s\n", s.Anchor.Name, s.Anchor.Public.Fingerprint())
	fmt.Printf("  digest: %s\n", s.Digest())
	fmt.Println()
	fmt.Println("The operator plants the same anchor on the machine, and only there:")
	fmt.Printf("    ./heliograph-seal trust init --set .station-trusted-set \\\n")
	fmt.Printf("      --estate %s --station %s \\\n", s.Estate, s.Station)
	fmt.Printf("      --name %s --anchor %s\n", s.Anchor.Name, s.Anchor.Public.Encode())
	fmt.Printf("    TRUST_SET=.station-trusted-set ./start.sh\n")
	fmt.Println()
	fmt.Println("Read the fingerprint above back to them. It is what stops a transport")
	fmt.Println("substituting an anchor of its own, and it is the last time anybody")
	fmt.Println("needs to touch that machine to change who may command it.")
	return nil
}

func cmdTrustShow(args []string) error {
	fs := flag.NewFlagSet("trust show", flag.ExitOnError)
	name := estateFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	e, err := resolve(*name)
	if err != nil {
		return err
	}
	s, err := loadLocalSet(e.Name)
	if err != nil {
		return err
	}
	fmt.Printf("estate:  %s\nstation: %s\nserial:  %d\ndigest:  %s\n",
		s.Estate, s.Station, s.Serial, s.Digest())
	fmt.Printf("anchor:  %-16s %s   (changeable only on the machine)\n",
		s.Anchor.Name, s.Anchor.Public.Fingerprint())
	for _, m := range s.Members() {
		state := fmt.Sprintf("added %s by %s", m.Added, m.AddedBy)
		if !m.Active() {
			state = fmt.Sprintf("REVOKED %s by %s", m.Revoked, m.RevokedBy)
		}
		fmt.Printf("member:  %-16s %s   %s\n", m.Name, m.Public.Fingerprint(), state)
	}
	fmt.Println()
	fmt.Println("This is what THIS machine believes. `heliograph doctor` compares it")
	fmt.Println("against what the station published, which is the copy that decides.")
	return nil
}

// cmdTrustChange authors, applies locally, and sends.
//
// THE LOCAL COPY IS ADVANCED ONLY AFTER Apply ACCEPTS. Advancing first would
// leave the control node believing in a set the station refused, and every
// later change authored against it would be refused too - a lockout produced by
// bookkeeping rather than by anything anybody decided.
func cmdTrustChange(args []string, op string) error {
	fs := flag.NewFlagSet("trust "+op, flag.ExitOnError)
	name := estateFlag(fs)
	note := fs.String("note", "", "free text for the next human")
	pos, err := parseTrustArgs(fs, args)
	if err != nil {
		return err
	}
	if op == trust.OpAdd && len(pos) != 2 {
		return fmt.Errorf("usage: heliograph trust add [-e <estate>] <name> <public-identity|file>")
	}
	if op == trust.OpRevoke && len(pos) != 1 {
		return fmt.Errorf("usage: heliograph trust revoke [-e <estate>] <name>")
	}
	subjectName := pos[0]

	e, err := resolve(*name)
	if err != nil {
		return err
	}
	id, e, err := identityFor(e)
	if err != nil {
		return err
	}
	s, err := loadLocalSet(e.Name)
	if err != nil {
		return err
	}
	author, ok := s.Lookup(id.Public())
	if !ok {
		return fmt.Errorf("this machine's key (%s) is not in the trusted set for %q, so it cannot author a change to it. Somebody already in the set has to add you, or the anchor holder does it at the machine",
			id.Public().Fingerprint(), e.Name)
	}
	if !author.Active() {
		return fmt.Errorf("this machine's key (%s) was revoked on %s by %s", author.Public.Fingerprint(), author.Revoked, author.RevokedBy)
	}

	var subject seal.PublicIdentity
	if op == trust.OpAdd {
		subject, err = readPublicIdentity(pos[1])
		if err != nil {
			return err
		}
	} else {
		m, found := s.ByName(subjectName)
		if !found {
			return fmt.Errorf("no member named %q in the trusted set for %q", subjectName, e.Name)
		}
		// THE KEY, NOT THE NAME, is what a revocation names on the wire. A name
		// can be reused; a key cannot, and a revocation that matched on a name
		// would revoke whoever happens to hold it when the change lands.
		subject = m.Public
	}

	c := trust.NewChange(s, op, subjectName, subject, author, time.Now())
	c, err = trust.SignChange(id, c)
	if err != nil {
		return err
	}

	// Applied HERE first, against the copy this node holds. If the station will
	// refuse it, we find out now rather than after a round trip through somebody
	// who cannot debug the machine.
	next, err := s.Apply(c, time.Now())
	if err != nil {
		return err
	}

	op2, err := open(e.Name)
	if err != nil {
		return err
	}
	req := wire.Request{
		Version: wire.Version,
		ID:      wire.NewID("trust-"+op+"-"+subjectName, time.Now()),
		Target:  op2.Scope,
		Note:    *note,
		Trust:   c.Marshal(),
	}
	if err := op2.PutRequest(req); err != nil {
		return err
	}
	if err := saveLocalSet(e.Name, next); err != nil {
		return err
	}

	fmt.Printf("sent %s\n", req.ID)
	fmt.Printf("  %s %s %s, signed by %s\n", op, subjectName, subject.Fingerprint(), author.Name)
	fmt.Printf("  serial %d, the set becomes %s\n", c.Serial, next.Digest())
	fmt.Println("  the station applies it on its next poll and publishes the result.")
	fmt.Println("  revocation is EVENTUAL on a polling station: it takes effect when the")
	fmt.Println("  station next fetches, and an already-running step is not interrupted.")
	return nil
}

// readPublicIdentity takes a path or the value.
//
// Both, because both are what happens: a colleague's public identity arrives
// pasted into a chat message as often as it arrives as a file, and the two are
// indistinguishable to whoever is typing the command.
func readPublicIdentity(v string) (seal.PublicIdentity, error) {
	if _, err := os.Stat(v); err == nil {
		return seal.LoadPeerFile(v)
	}
	p, err := seal.DecodePublic(strings.TrimSpace(v))
	if err != nil {
		return seal.PublicIdentity{}, fmt.Errorf("%q is neither a file nor a public identity: %w", v, err)
	}
	return p, nil
}

// doctorTrust compares this machine's copy of the set against what the station
// published, and returns the number of blocking problems.
//
// IT REPORTS RATHER THAN RECONCILES. The station's copy is the one that
// decides who may command it; ours exists so we can author a change against the
// right serial. When they disagree, the interesting question is WHICH WAY, and
// each direction has a different remedy:
//
//	station ahead    somebody else's change landed and we have not seen it. Get
//	                 their set, or re-record ours. Ordinary in a team
//	station behind   our change has not reached it yet, or was refused. The
//	                 published status says which
//	keys differ      a key is trusted there that was never authorised here. That
//	                 is the alarm, and the remedy is the anchor holder at the
//	                 machine
//
// Every line names a remedy, because the person reading it usually cannot ask
// anybody: a preflight line that reports a problem without one is a defect.
func doctorTrust(name string, s wire.Status) int {
	local, err := loadLocalSet(name)
	if err != nil {
		if s.Trust == "" {
			// Neither side has one. Not a problem: it is every station in the
			// field today, and saying so plainly beats a warning.
			fmt.Println("note      no trusted set on either side")
			fmt.Println("          this station accepts whatever its transport verifies. `heliograph trust init` starts one")
			return 0
		}
		fmt.Printf("FAIL      the station publishes a trusted set (%s) and this machine has no copy\n", short12(s.Trust))
		fmt.Println("          run `heliograph trust init`, or get the set from whoever administers it:")
		fmt.Println("          without a copy, nothing here can author a change against the right serial")
		return 1
	}
	if s.Trust == "" {
		fmt.Println("FAIL      this machine holds a trusted set and the station publishes none")
		fmt.Println("          the station was started without TRUST_SET, so it is NOT verifying against the set")
		fmt.Println("          the operator restarts it with TRUST_SET pointing at the planted file")
		return 1
	}
	if s.Trust == local.Digest() {
		fmt.Printf("ok        the trusted set matches: serial %d, %s\n", local.Serial, local.Fingerprints())
		return 0
	}
	fmt.Printf("FAIL      the station's trusted set is not the one this machine holds\n")
	fmt.Printf("          station: %s  serial %s\n", short12(s.Trust), orDash(s.TrustSerial))
	fmt.Printf("          here:    %s  serial %d\n", short12(local.Digest()), local.Serial)
	if s.TrustMembers != "" {
		fmt.Printf("          station members: %s\n", s.TrustMembers)
		fmt.Printf("          our members:     %s\n", local.Fingerprints())
	}
	fmt.Println("          A key trusted there and not here was added by somebody, and this")
	fmt.Println("          machine did not authorise it. If that was not expected, the anchor")
	fmt.Println("          holder revokes it at the machine: heliograph-seal trust show --set <file>")
	return 1
}

func short12(d string) string {
	if len(d) > 12 {
		return d[:12]
	}
	return d
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
