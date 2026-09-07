// Command heliograph-seal does the cryptography for the relay transport, and
// nothing else.
//
// It is a separate, single-purpose binary rather than a mode of the main CLI so
// that the thing which has to be audited stays small. It does no networking:
// curl stays in the shell transport, where its behaviour can be read and
// debugged on a machine nobody can reach.
//
// Every other transport - git, object store, file share, bundle - remains pure
// bash and needs none of this.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dbhq-uk/heliograph/internal/seal"
)

var version = "dev"

const usage = `heliograph-seal - seals and opens heliograph relay messages

  heliograph-seal keygen --out <file>
  heliograph-seal fingerprint [--identity <file> | --peer <file>]
  heliograph-seal public --identity <file>
  heliograph-seal seal   --identity F --peer F --estate E --station S --dir D --kind K --seq N --in F --out F
  heliograph-seal open   --identity F --peer F --estate E --station S --dir D --kind K --min-seq N --in F
  heliograph-seal version

open reads the relay's JSON array on stdin or --in, verifies every message, and
prints the accepted sequence number on the first line followed by the document.
It exits non-zero when nothing verified, which is not an error condition: a
relay is entitled to hand over anything at all.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = cmdKeygen(os.Args[2:])
	case "fingerprint":
		err = cmdFingerprint(os.Args[2:])
	case "public":
		err = cmdPublic(os.Args[2:])
	case "seal":
		err = cmdSeal(os.Args[2:])
	case "open":
		err = cmdOpen(os.Args[2:])
	case "version", "--version":
		fmt.Printf("heliograph-seal %s\n", version)
		return
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "heliograph-seal: "+err.Error())
		os.Exit(1)
	}
}

// storedIdentity is the on-disk form.
//
// The secret half never leaves this file, and the file is written 0600. That is
// the whole of the key management story here, deliberately: anything more
// elaborate would be something else to audit.
type storedIdentity struct {
	Version int    `json:"version"`
	Secret  string `json:"secret"`
	Public  string `json:"public"`
}

func cmdKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := fs.String("out", "", "where to write the identity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("--out is required")
	}
	// Refuse to overwrite. An identity file replaced by accident is a station
	// that can no longer be talked to and a control that can no longer read
	// its logs, on a machine nobody can reach.
	if _, err := os.Stat(*out); err == nil {
		return fmt.Errorf("%s already exists: refusing to replace an identity, because everything sealed to it becomes unreadable", *out)
	}
	id, err := seal.Generate()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(storedIdentity{
		Version: 1, Secret: id.Encode(), Public: id.Public().Encode(),
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, append(b, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Printf("identity written to %s (mode 600)\n", *out)
	fmt.Printf("fingerprint: %s\n", id.Public().Fingerprint())
	fmt.Println()
	fmt.Println("Read that fingerprint back to the other side over a channel they")
	fmt.Println("already trust. It is what stops a relay substituting its own key.")
	return nil
}

func loadIdentity(path string) (*seal.Identity, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s storedIdentity
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s is not an identity file: %w", path, err)
	}
	return seal.DecodeIdentity(s.Secret)
}

func loadPeer(path string) (seal.PublicIdentity, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return seal.PublicIdentity{}, err
	}
	txt := strings.TrimSpace(string(b))
	// A peer file may be a bare public identity, or the JSON an identity file
	// happens to be. Accepting both means nobody has to remember which.
	if strings.HasPrefix(txt, "{") {
		var s storedIdentity
		if err := json.Unmarshal([]byte(txt), &s); err == nil && s.Public != "" {
			return seal.DecodePublic(s.Public)
		}
	}
	return seal.DecodePublic(txt)
}

func cmdPublic(args []string) error {
	fs := flag.NewFlagSet("public", flag.ExitOnError)
	idf := fs.String("identity", "", "identity file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id, err := loadIdentity(*idf)
	if err != nil {
		return err
	}
	fmt.Println(id.Public().Encode())
	return nil
}

func cmdFingerprint(args []string) error {
	fs := flag.NewFlagSet("fingerprint", flag.ExitOnError)
	idf := fs.String("identity", "", "identity file")
	peerf := fs.String("peer", "", "peer public identity file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case *idf != "":
		id, err := loadIdentity(*idf)
		if err != nil {
			return err
		}
		fmt.Println(id.Public().Fingerprint())
	case *peerf != "":
		p, err := loadPeer(*peerf)
		if err != nil {
			return err
		}
		fmt.Println(p.Fingerprint())
	default:
		return fmt.Errorf("give --identity or --peer")
	}
	return nil
}

type metaFlags struct {
	identity, peer, estate, station, dir, kind string
}

func (m *metaFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&m.identity, "identity", "", "this side's identity file")
	fs.StringVar(&m.peer, "peer", "", "the other side's public identity")
	fs.StringVar(&m.estate, "estate", "", "estate id")
	fs.StringVar(&m.station, "station", "", "station id")
	fs.StringVar(&m.dir, "dir", "", "c2s or s2c")
	fs.StringVar(&m.kind, "kind", "", "request | status | progress | log | payload")
}

func cmdSeal(args []string) error {
	fs := flag.NewFlagSet("seal", flag.ExitOnError)
	var mf metaFlags
	mf.register(fs)
	seq := fs.Uint64("seq", 0, "sequence number")
	in := fs.String("in", "", "plaintext file")
	out := fs.String("out", "", "where to write the relay message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id, err := loadIdentity(mf.identity)
	if err != nil {
		return err
	}
	peer, err := loadPeer(mf.peer)
	if err != nil {
		return err
	}
	plain, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	m := seal.Meta{
		Estate: mf.estate, Station: mf.station, Dir: mf.dir,
		Seq: *seq, Kind: mf.kind, Recipient: peer.Fingerprint(),
	}
	sealed, err := seal.Seal(id, peer, m, plain)
	if err != nil {
		return err
	}
	// The relay's own wire shape, so the shell only ever has to POST a file.
	body, err := json.Marshal(map[string]any{"seq": *seq, "body": sealed})
	if err != nil {
		return err
	}
	if *out == "" {
		_, err = os.Stdout.Write(body)
		return err
	}
	return os.WriteFile(*out, body, 0o600)
}

type relayMsg struct {
	Seq  uint64 `json:"seq"`
	Body []byte `json:"body"`
}

func cmdOpen(args []string) error {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	var mf metaFlags
	mf.register(fs)
	minSeq := fs.Uint64("min-seq", 0, "refuse anything at or below this sequence number")
	in := fs.String("in", "", "the relay's JSON array; stdin if absent")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id, err := loadIdentity(mf.identity)
	if err != nil {
		return err
	}
	peer, err := loadPeer(mf.peer)
	if err != nil {
		return err
	}

	var raw []byte
	if *in != "" {
		raw, err = os.ReadFile(*in)
	} else {
		raw, err = readAll(os.Stdin)
	}
	if err != nil {
		return err
	}
	var msgs []relayMsg
	if err := json.Unmarshal(raw, &msgs); err != nil {
		// Not an error worth a stack trace: the relay can return anything.
		return fmt.Errorf("nothing usable in the relay's answer")
	}
	// Oldest first, so a regression is measured against the right baseline.
	sort.Slice(msgs, func(a, b int) bool { return msgs[a].Seq < msgs[b].Seq })

	var bestSeq uint64
	var best []byte
	for _, msg := range msgs {
		// THE REPLAY CHECK. A gap is tolerated because the relay may
		// legitimately expire a message; a regression is refused, because that
		// is the relay replaying an old one. The attack is concrete:
		// --allow-actions and CONFIRM=yes are decided days before the request
		// that uses them, so a replayed request is a destructive step running
		// again with all its gates already satisfied.
		if msg.Seq <= *minSeq {
			continue
		}
		m := seal.Meta{
			Estate: mf.estate, Station: mf.station, Dir: mf.dir,
			Seq: msg.Seq, Kind: mf.kind, Recipient: id.Public().Fingerprint(),
		}
		plain, err := seal.Open(id, peer, m, msg.Body)
		if err != nil {
			continue
		}
		bestSeq, best = msg.Seq, plain
	}
	if best == nil {
		// Exit 1 with no output. The caller treats that as "nothing to do",
		// which is right: a station that stopped on rubbish would be one a
		// hostile relay could halt at will.
		os.Exit(1)
	}
	fmt.Printf("%d\n", bestSeq)
	_, err = os.Stdout.Write(best)
	return err
}

func readAll(f *os.File) ([]byte, error) {
	var out []byte
	buf := make([]byte, 32*1024)
	for {
		n, err := f.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			if err.Error() == "EOF" {
				return out, nil
			}
			if n == 0 {
				return out, nil
			}
		}
		if n == 0 {
			return out, nil
		}
	}
}
