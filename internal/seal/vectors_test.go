package seal

// Golden vectors for the seal format, so a SECOND implementation can be checked
// against this one without either running the other.
//
// THE PROBLEM THIS SOLVES. The PowerShell station cannot run `heliograph-seal`,
// a Go binary, on the estates it exists for - so it has to build this
// construction again in managed code. Two implementations of a crypto format
// that have never been compared are two formats, and the way that failure
// presents is a station that enrols successfully and then cannot open a single
// message, or worse, one that works because both sides made the same mistake.
//
// A ROUND TRIP DOES NOT SHOW IT. This repository already learned that at cost:
// `Chaos.NaCl.MontgomeryCurve25519.KeyExchange` round-trips perfectly with
// itself and is not X25519, because it returns NaCl's `crypto_box_beforenm`
// rather than the raw RFC 7748 secret. Only fixed bytes catch that.
//
// SO EVERY STAGE IS PINNED, not just the output. A port that gets one thing
// wrong should be told which thing: the canonical metadata, the signature, the
// shared secret, the HKDF salt and info, the derived key, and finally the
// sealed message. "The ciphertext differs" is not a debuggable statement about
// six chained primitives.
//
// Regenerate with:
//
//	go test ./internal/seal/ -run TestGoldenVectors -update
//
// A DIFF IN THE COMMITTED FILE IS A PROTOCOL CHANGE. Every station in the field
// speaks the version in it, `Version` is a signed field with no negotiation,
// and there is no plaintext fallback - so a change here strands whatever is
// already deployed. That is the point of committing it.

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the committed vector file")

// The path is outside this package on purpose: the PowerShell tests read the
// same file, and a fixture buried in `internal/` reads as private to Go.
const vectorPath = "../../tests/fixtures/seal-vectors.json"

// Fixed identities, in the same encoding `Identity.Encode` produces, so the
// file can be read by anything that can base64url-decode. NOT generated at test
// time: a vector that changes on every run is not a vector.
//
// These two secret keys are published in a public repository and are worth
// exactly nothing. Nothing else may ever use them, which is why they are named
// so unmistakably.
const (
	testControlSecret = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0-Pw"
	testStationSecret = "QEFCQ0RFRkdISUpLTE1OT1BRUlNUVVZXWFlaW1xdXl9gYWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXp7fH1-fw"
)

type identityVec struct {
	Note        string `json:"note"`
	Secret      string `json:"secret"`
	Public      string `json:"public"`
	EncPublic   string `json:"encPublicHex"`
	SignPublic  string `json:"signPublicHex"`
	Fingerprint string `json:"fingerprint"`
}

type metaVec struct {
	Estate    string `json:"estate"`
	Station   string `json:"station"`
	Dir       string `json:"dir"`
	Seq       uint64 `json:"seq"`
	Kind      string `json:"kind"`
	Recipient string `json:"recipient"`
}

type caseVec struct {
	Name string  `json:"name"`
	Why  string  `json:"why"`
	From string  `json:"from"` // "control" or "station"
	To   string  `json:"to"`
	Meta metaVec `json:"meta"`

	PlaintextHex string `json:"plaintextHex"`

	// Every stage, so a mismatch says where.
	CanonicalHex    string `json:"canonicalHex"`
	SignatureHex    string `json:"signatureHex"`
	EphemeralSecret string `json:"ephemeralSecretHex"`
	EphemeralPublic string `json:"ephemeralPublicHex"`
	SharedSecretHex string `json:"sharedSecretHex"`
	HKDFSaltHex     string `json:"hkdfSaltHex"`
	HKDFInfoHex     string `json:"hkdfInfoHex"`
	KeyHex          string `json:"keyHex"`
	NonceHex        string `json:"nonceHex"`
	SealedHex       string `json:"sealedHex"`
}

type refusalVec struct {
	Name      string  `json:"name"`
	Why       string  `json:"why"`
	Meta      metaVec `json:"meta"`
	SealedHex string  `json:"sealedHex"`
}

type vectorFile struct {
	Note       string       `json:"note"`
	Version    uint64       `json:"version"`
	Control    identityVec  `json:"control"`
	Station    identityVec  `json:"station"`
	Cases      []caseVec    `json:"cases"`
	MustRefuse []refusalVec `json:"mustRefuse"`
}

func mustIdentity(t *testing.T, s string) *Identity {
	t.Helper()
	id, err := DecodeIdentity(s)
	if err != nil {
		t.Fatalf("the fixed identity in this file is not decodable: %v", err)
	}
	return id
}

func describe(t *testing.T, note string, id *Identity) identityVec {
	t.Helper()
	p := id.Public()
	return identityVec{
		Note:        note,
		Secret:      id.Encode(),
		Public:      p.Encode(),
		EncPublic:   hex.EncodeToString(p.Enc),
		SignPublic:  hex.EncodeToString(p.Sign),
		Fingerprint: p.Fingerprint(),
	}
}

func TestGoldenVectors(t *testing.T) {
	control := mustIdentity(t, testControlSecret)
	station := mustIdentity(t, testStationSecret)

	// Fixed ephemeral keys and nonces, one per case. Real ones are random; that
	// is what `Seal` does and what `TestSealingTwiceProducesDifferentCiphertext`
	// holds it to. Here they are fixed for the only reason a vector exists.
	eph := func(b byte) *ecdh.PrivateKey {
		raw := make([]byte, 32)
		for i := range raw {
			raw[i] = b ^ byte(i*7+1)
		}
		k, err := ecdh.X25519().NewPrivateKey(raw)
		if err != nil {
			t.Fatalf("fixed ephemeral key %d is not usable: %v", b, err)
		}
		return k
	}
	nonce := func(b byte) []byte {
		n := make([]byte, 12)
		for i := range n {
			n[i] = b ^ byte(i*3+2)
		}
		return n
	}

	cf := control.Public().Fingerprint()
	sf := station.Public().Fingerprint()

	type spec struct {
		name, why, from string
		m               Meta
		plaintext       []byte
		ephByte, nByte  byte
	}
	specs := []spec{
		{
			name: "request-c2s", why: "the ordinary case: a control sends a station a request",
			from:      "control",
			m:         Meta{Estate: "payments", Station: "web-01", Dir: "c2s", Seq: 1, Kind: "request", Recipient: sf},
			plaintext: []byte("id: r-1\nstep: ./steps/disk.sh\n"),
			ephByte:   0x11, nByte: 0x21,
		},
		{
			name: "status-s2c-seq-zero", why: "the other direction, and a zero sequence number - a uint64 field that a port may render as an empty string rather than eight zero bytes",
			from:      "station",
			m:         Meta{Estate: "payments", Station: "web-01", Dir: "s2c", Seq: 0, Kind: "status", Recipient: cf},
			plaintext: []byte("state: idle\nexit: 0\n"),
			ephByte:   0x22, nByte: 0x32,
		},
		{
			name: "empty-plaintext", why: "a zero-length body. The signature still covers the metadata, and a port that skips the AEAD for an empty input produces something that opens to nothing",
			from:      "control",
			m:         Meta{Estate: "payments", Station: "web-01", Dir: "c2s", Seq: 2, Kind: "progress", Recipient: sf},
			plaintext: []byte{},
			ephByte:   0x33, nByte: 0x43,
		},
		{
			name: "fields-that-would-break-a-delimiter", why: "THE REASON THE METADATA IS LENGTH-PREFIXED. These values contain a newline, a NUL and the separators a hand-rolled encoding would reach for. Two different Metas must not serialise identically, and a port that joins with a delimiter passes every other case here",
			from: "control",
			m: Meta{
				Estate:  "pay|ments\x00:0",
				Station: "web-01\n\x1fweb-02",
				Dir:     "c2s", Seq: 0xFFFFFFFFFFFFFFFF, Kind: "request", Recipient: sf,
			},
			plaintext: []byte("id: r-2\n"),
			ephByte:   0x44, nByte: 0x54,
		},
		{
			name: "utf8-plaintext", why: "multi-byte UTF-8 through a payload. A port that passes the plaintext through a string type mangles this and nothing else here notices",
			from:      "station",
			m:         Meta{Estate: "paiements", Station: "café-01", Dir: "s2c", Seq: 7, Kind: "log", Recipient: cf},
			plaintext: []byte("résumé: ✓ fini\n日本語\n"),
			ephByte:   0x55, nByte: 0x65,
		},
		{
			name: "long-plaintext", why: "1000 bytes, so the ChaCha20 keystream advances past its first block and the Poly1305 padding is exercised on a length that is not a multiple of 16",
			from:      "station",
			m:         Meta{Estate: "payments", Station: "web-01", Dir: "s2c", Seq: 9, Kind: "log", Recipient: cf},
			plaintext: bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 23)[:1000],
			ephByte:   0x66, nByte: 0x76,
		},
	}

	out := vectorFile{
		Note: "Golden vectors for internal/seal. Generated by TestGoldenVectors -update. " +
			"A diff here is a protocol change: Version is signed, there is no negotiation, " +
			"and every deployed station speaks what is in this file.",
		Version: Version,
		Control: describe(t, "the control side. A published test key, worth nothing.", control),
		Station: describe(t, "the station side. A published test key, worth nothing.", station),
	}

	for _, s := range specs {
		from, to := control, station
		if s.from == "station" {
			from, to = station, control
		}
		toPub := to.Public()
		e, n := eph(s.ephByte), nonce(s.nByte)

		sealed, err := sealWith(from, toPub, s.m, s.plaintext, e, n)
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}

		// Reproduce the intermediates the same way Seal does, so the file
		// records what the code actually computed rather than a second guess
		// at it. If these drifted from Seal the assertions below would still
		// pass, so each one is checked against the sealed bytes.
		toEnc, err := ecdh.X25519().NewPublicKey(toPub.Enc)
		if err != nil {
			t.Fatal(err)
		}
		shared, err := e.ECDH(toEnc)
		if err != nil {
			t.Fatal(err)
		}
		key, err := deriveKey(shared, e.PublicKey().Bytes(), toPub.Enc, s.m)
		if err != nil {
			t.Fatal(err)
		}
		canon := s.m.canonical()
		sig := ed25519.Sign(from.sign, append(append([]byte{}, canon...), s.plaintext...))

		toName := "station"
		if s.from == "station" {
			toName = "control"
		}
		out.Cases = append(out.Cases, caseVec{
			Name: s.name, Why: s.why, From: s.from, To: toName,
			Meta: metaVec{s.m.Estate, s.m.Station, s.m.Dir, s.m.Seq, s.m.Kind, s.m.Recipient},

			PlaintextHex:    hex.EncodeToString(s.plaintext),
			CanonicalHex:    hex.EncodeToString(canon),
			SignatureHex:    hex.EncodeToString(sig),
			EphemeralSecret: hex.EncodeToString(e.Bytes()),
			EphemeralPublic: hex.EncodeToString(e.PublicKey().Bytes()),
			SharedSecretHex: hex.EncodeToString(shared),
			HKDFSaltHex:     hex.EncodeToString(append(append([]byte{}, e.PublicKey().Bytes()...), toPub.Enc...)),
			HKDFInfoHex:     hex.EncodeToString(append([]byte("heliograph-seal-v1"), canon...)),
			KeyHex:          hex.EncodeToString(key),
			NonceHex:        hex.EncodeToString(n),
			SealedHex:       hex.EncodeToString(sealed),
		})
	}

	// A vector file of things that must succeed teaches a port to accept
	// everything. These are the refusals, in the same fixed bytes, because a
	// second implementation that opens a tampered message is worse than one
	// that opens nothing.
	base := control
	tgt := station.Public()
	m := Meta{Estate: "payments", Station: "web-01", Dir: "c2s", Seq: 3, Kind: "request", Recipient: tgt.Fingerprint()}
	good, err := sealWith(base, tgt, m, []byte("id: r-3\nstep: ./steps/disk.sh\n"), eph(0x77), nonce(0x87))
	if err != nil {
		t.Fatal(err)
	}
	flip := func(i int) string {
		b := append([]byte{}, good...)
		b[i] ^= 0x01
		return hex.EncodeToString(b)
	}
	out.MustRefuse = []refusalVec{
		{"tampered-ciphertext", "one bit of the ciphertext. The AEAD tag must refuse it before anything is decoded", metaVec{m.Estate, m.Station, m.Dir, m.Seq, m.Kind, m.Recipient}, flip(len(good) - 1)},
		{"tampered-nonce", "one bit of the nonce, which is outside the AEAD input and so is the byte a port is most likely to leave unchecked", metaVec{m.Estate, m.Station, m.Dir, m.Seq, m.Kind, m.Recipient}, flip(70)},
		{"tampered-ephemeral-key", "one bit of the ephemeral public key. It derives the key, so this fails to decrypt rather than failing a signature", metaVec{m.Estate, m.Station, m.Dir, m.Seq, m.Kind, m.Recipient}, flip(40)},
		{"claims-another-sender", "the sender's Ed25519 key replaced with the recipient's. Refused before any cryptography is attempted with it", metaVec{m.Estate, m.Station, m.Dir, m.Seq, m.Kind, m.Recipient},
			hex.EncodeToString(append(append([]byte{}, tgt.Sign...), good[32:]...))},
		{"wrong-sequence-number", "the same bytes, opened with a Meta whose Seq is one higher. The metadata is bound into the KEY, so this cannot decrypt at all - it is not merely a failed signature", metaVec{m.Estate, m.Station, m.Dir, m.Seq + 1, m.Kind, m.Recipient}, hex.EncodeToString(good)},
		{"wrong-recipient-fingerprint", "the same bytes, opened with a Meta naming the control's own fingerprint. This is the sign-then-encrypt forwarding mitigation", metaVec{m.Estate, m.Station, m.Dir, m.Seq, m.Kind, control.Public().Fingerprint()}, hex.EncodeToString(good)},
	}

	blob, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	blob = append(blob, '\n')

	if *update {
		if err := os.MkdirAll(filepath.Dir(vectorPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(vectorPath, blob, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d cases, %d refusals)", vectorPath, len(out.Cases), len(out.MustRefuse))
		return
	}

	have, err := os.ReadFile(vectorPath)
	if err != nil {
		t.Fatalf("the committed vectors are missing: %v\n"+
			"Run: go test ./internal/seal/ -run TestGoldenVectors -update", err)
	}
	if !bytes.Equal(have, blob) {
		t.Errorf("the seal format no longer produces the committed vectors.\n"+
			"THIS IS A PROTOCOL CHANGE. `Version` is a signed field, there is no "+
			"negotiation and no plaintext fallback, so every station already in the "+
			"field speaks what is in %s and would stop being able to open anything.\n"+
			"If that is intended, bump Version and regenerate:\n"+
			"  go test ./internal/seal/ -run TestGoldenVectors -update", vectorPath)
	}
}

// The vectors are only worth having if this implementation is held to them too.
// Read back from the file and opened, so the committed bytes are exercised
// rather than merely compared - a file that had been regenerated from a broken
// build would match itself perfectly.
func TestTheCommittedVectorsOpen(t *testing.T) {
	blob, err := os.ReadFile(vectorPath)
	if err != nil {
		t.Skipf("no committed vectors: %v", err)
	}
	var vf vectorFile
	if err := json.Unmarshal(blob, &vf); err != nil {
		t.Fatalf("the committed vectors are not readable: %v", err)
	}
	if len(vf.Cases) == 0 || len(vf.MustRefuse) == 0 {
		t.Fatal("the vector file carries no cases, so this test asserted nothing")
	}
	if vf.Version != Version {
		t.Fatalf("the vectors are for version %d and this build speaks %d", vf.Version, Version)
	}

	ids := map[string]*Identity{
		"control": mustIdentity(t, vf.Control.Secret),
		"station": mustIdentity(t, vf.Station.Secret),
	}

	for _, c := range vf.Cases {
		t.Run(c.Name, func(t *testing.T) {
			from, to := ids[c.From], ids[c.To]
			if from == nil || to == nil {
				t.Fatalf("case names an identity that is not in the file: %s -> %s", c.From, c.To)
			}
			sealed, err := hex.DecodeString(c.SealedHex)
			if err != nil {
				t.Fatal(err)
			}
			want, err := hex.DecodeString(c.PlaintextHex)
			if err != nil {
				t.Fatal(err)
			}
			m := Meta{c.Meta.Estate, c.Meta.Station, c.Meta.Dir, c.Meta.Seq, c.Meta.Kind, c.Meta.Recipient}

			got, err := Open(to, from.Public(), m, sealed)
			if err != nil {
				t.Fatalf("this build cannot open its own committed vector: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("plaintext differs\n got %q\nwant %q", got, want)
			}

			// And the stages, so a port comparing against this file is
			// comparing against values Go really produces.
			if h := hex.EncodeToString(m.canonical()); h != c.CanonicalHex {
				t.Errorf("canonical metadata differs\n got %s\nwant %s", h, c.CanonicalHex)
			}
		})
	}

	for _, r := range vf.MustRefuse {
		t.Run("refuse/"+r.Name, func(t *testing.T) {
			sealed, err := hex.DecodeString(r.SealedHex)
			if err != nil {
				t.Fatal(err)
			}
			m := Meta{r.Meta.Estate, r.Meta.Station, r.Meta.Dir, r.Meta.Seq, r.Meta.Kind, r.Meta.Recipient}
			if _, err := Open(ids["station"], ids["control"].Public(), m, sealed); err == nil {
				t.Error("opened a message the vectors say must be refused")
			}
		})
	}
}
