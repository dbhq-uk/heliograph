// Package seal is the end-to-end encryption between a control and a station.
//
// The relay is outside the trust boundary IN BOTH DIRECTIONS, and the second
// direction is the one that decides whether this product is safe to exist.
//
// A relay that can read logs is a privacy problem. A relay that can FORGE a
// request has code execution inside every customer estate at once, through a
// channel the customer installed deliberately and trusts. That is a far worse
// position, so authenticity is the first requirement here and confidentiality
// the second.
//
// Everything is standard. X25519 for key agreement, HKDF-SHA256 to derive,
// ChaCha20-Poly1305 to encrypt, Ed25519 to sign - the age construction, with
// signing added. Nothing here is invented, and a design that seemed to need
// something novel would be a design that was wrong.
package seal

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/chacha20poly1305"
)

// Version is the protocol this build speaks. It is a SIGNED field, so a relay
// cannot offer an older one: there is no negotiation, no opportunistic mode and
// no plaintext fallback.
const Version = 1

// Identity is one side's long-lived keys.
//
// Two keypairs, not one. X25519 cannot sign and Ed25519 cannot agree a key, and
// converting between them is the sort of clever that gets a design an
// unfavourable audit.
type Identity struct {
	enc  *ecdh.PrivateKey   // X25519, for receiving
	sign ed25519.PrivateKey // for authorship
}

// PublicIdentity is what the other side needs, and all it needs.
type PublicIdentity struct {
	Enc  []byte // X25519 public key, 32 bytes
	Sign []byte // Ed25519 public key, 32 bytes
}

// Generate makes a new identity.
func Generate() (*Identity, error) {
	enc, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	_, sign, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Identity{enc: enc, sign: sign}, nil
}

func (id *Identity) Public() PublicIdentity {
	return PublicIdentity{
		Enc:  id.enc.PublicKey().Bytes(),
		Sign: id.sign.Public().(ed25519.PublicKey),
	}
}

// Fingerprint is what a human reads aloud to confirm the other side is the
// other side.
//
// This is the ceremony that closes the residual risk in enrolment: the token
// travels through the same channel that carries a repo URL today, and if it
// were intercepted before use, an attacker could enrol a rogue station. Reading
// twelve characters back over a different channel is what settles it.
//
// Grouped in fours because that is how people read a string aloud without
// losing their place, and truncated to 12 hex characters for the same reason
// secret.sh does: long enough to be infeasible to collide deliberately, short
// enough to be said over a phone.
func (p PublicIdentity) Fingerprint() string {
	h := sha256.New()
	h.Write([]byte("heliograph-identity-v1"))
	h.Write(p.Enc)
	h.Write(p.Sign)
	sum := fmt.Sprintf("%x", h.Sum(nil))[:12]
	return sum[0:4] + "-" + sum[4:8] + "-" + sum[8:12]
}

func (p PublicIdentity) Encode() string {
	return base64.RawURLEncoding.EncodeToString(append(append([]byte{}, p.Enc...), p.Sign...))
}

func DecodePublic(s string) (PublicIdentity, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return PublicIdentity{}, fmt.Errorf("not a usable public identity: %w", err)
	}
	if len(b) != 64 {
		return PublicIdentity{}, fmt.Errorf("a public identity is 64 bytes, got %d", len(b))
	}
	return PublicIdentity{Enc: b[:32], Sign: b[32:]}, nil
}

// Meta is everything about a message except its content, and every field here
// is covered by the signature.
//
// Binding all of it is what stops a relay replaying, reflecting, or forwarding
// a message it cannot read. Each field answers one attack:
//
//	Estate     a message from one estate replayed into another
//	Station    a message for one station replayed to a different one
//	Dir        a message reflected back the way it came
//	Seq        an old request replayed to re-run something destructive
//	Recipient  surreptitious forwarding, the known weakness of sign-then-encrypt
//
// THERE IS NO TIMESTAMP HERE, and that was a correction rather than an
// omission. A `Sent` field was signed in the first draft, and it could not
// work: the recipient has to reconstruct the metadata exactly in order to
// verify, and it cannot know a clock it did not read. Carrying it alongside
// would have let the relay tamper with the one field verification depended on.
//
// It was also redundant. The request and status documents already carry `utc:`,
// and that is inside the plaintext, so it is signed without any of this. When a
// field cannot be reconstructed and is already somewhere better, the answer is
// to remove it rather than to invent a way to guess it.
type Meta struct {
	Estate    string
	Station   string
	Dir       string // "c2s" or "s2c"
	Seq       uint64
	Kind      string // request | status | progress | log | payload
	Recipient string // the recipient's fingerprint
}

func (m Meta) canonical() []byte {
	// Length-prefixed, not delimited. A delimiter can appear inside a value, and
	// then two different Metas serialise identically - which is exactly how a
	// signature gets to cover something other than what it appears to.
	var b []byte
	add := func(s string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(s)))
		b = append(b, n[:]...)
		b = append(b, s...)
	}
	var v [8]byte
	binary.BigEndian.PutUint64(v[:], Version)
	b = append(b, v[:]...)
	add(m.Estate)
	add(m.Station)
	add(m.Dir)
	binary.BigEndian.PutUint64(v[:], m.Seq)
	b = append(b, v[:]...)
	add(m.Kind)
	add(m.Recipient)
	return b
}

func (m Meta) validate() error {
	if m.Estate == "" || m.Station == "" || m.Recipient == "" {
		return errors.New("seal: a message must name its estate, station and recipient")
	}
	if m.Dir != "c2s" && m.Dir != "s2c" {
		return fmt.Errorf("seal: direction must be c2s or s2c, got %q", m.Dir)
	}
	if m.Kind == "" {
		return errors.New("seal: a message must say what kind it is")
	}
	return nil
}

// Seal signs the plaintext and metadata, then encrypts both to the recipient.
//
// SIGN THEN ENCRYPT. The signature travels inside the encryption, so the relay
// cannot see who signed what, and a valid signature proves authorship rather
// than merely transmission.
//
// Sign-then-encrypt has a known weakness: a recipient can re-encrypt a validly
// signed message to a third party, who then believes it was sent to them. The
// standard mitigation is applied - the recipient's fingerprint is one of the
// SIGNED fields, so a forwarded message fails verification at the new recipient.
func Seal(from *Identity, to PublicIdentity, m Meta, plaintext []byte) ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	if to.Fingerprint() != m.Recipient {
		// Caught here rather than at the far side, where the failure is a
		// refusal on a machine nobody can reach.
		return nil, fmt.Errorf("seal: Meta.Recipient is %q but the key given is %q",
			m.Recipient, to.Fingerprint())
	}

	inner := append(m.canonical(), plaintext...)
	sig := ed25519.Sign(from.sign, inner)

	// Ephemeral key per message. Without it, every message between one pair of
	// identities shares a key, and one compromise reads all of them.
	eph, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	toEnc, err := ecdh.X25519().NewPublicKey(to.Enc)
	if err != nil {
		return nil, fmt.Errorf("seal: unusable recipient key: %w", err)
	}
	// Go's X25519 ECDH returns an error when the shared secret is all zeroes,
	// which is what a low-order or small-subgroup public key produces. Paseo's
	// relay checks for exactly this by hand; here it comes from the standard
	// library, and it is asserted in the tests rather than assumed.
	shared, err := eph.ECDH(toEnc)
	if err != nil {
		return nil, fmt.Errorf("seal: unusable recipient key: %w", err)
	}
	key, err := deriveKey(shared, eph.PublicKey().Bytes(), to.Enc, m)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	// A random nonce, even though the key is already unique per message: it is
	// derived from an ephemeral key that is fresh and never reused, so a fixed
	// nonce would in fact be safe.
	//
	// It is here because "the nonce is all zeroes" stops a review dead, and
	// correctly - the reviewer then has to reason about ephemeral generation to
	// clear it. Twelve bytes is a cheap price for a line nobody has to argue
	// about. Paseo's relay does the same thing for the same reason.
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	// The metadata is authenticated as additional data as well as signed. The
	// signature proves who wrote it; the AEAD binds it to THIS ciphertext, so
	// the header cannot be swapped between two messages we cannot read.
	body := append(append([]byte{}, sig...), plaintext...)
	ct := aead.Seal(nil, nonce, body, m.canonical())

	out := append([]byte{}, from.Public().Sign...) // 32: who claims to have sent it
	out = append(out, eph.PublicKey().Bytes()...)  // 32: the ephemeral public key
	out = append(out, nonce...)                    // 12
	out = append(out, ct...)
	return out, nil
}

// Open decrypts, then verifies everything.
//
// A refusal here is a hard failure and never a warning. There is no mode in
// which an unverifiable message is acted on, because the whole point is that a
// hostile relay must not be able to cause a run.
func Open(to *Identity, expectFrom PublicIdentity, expect Meta, sealed []byte) ([]byte, error) {
	if err := expect.validate(); err != nil {
		return nil, err
	}
	const hdr = 32 + 32 + chacha20poly1305.NonceSize
	if len(sealed) < hdr+chacha20poly1305.Overhead+ed25519.SignatureSize {
		return nil, errors.New("seal: message is too short to be one")
	}
	claimedSign := sealed[:32]
	ephBytes := sealed[32:64]
	nonce := sealed[64:hdr]
	ct := sealed[hdr:]

	// Constant time, and checked BEFORE any cryptography is attempted with it.
	// The value is public, but a comparison that returns early is a habit worth
	// not having in this file.
	if subtle.ConstantTimeCompare(claimedSign, expectFrom.Sign) != 1 {
		return nil, errors.New("seal: the message claims a different sender than the one expected")
	}

	eph, err := ecdh.X25519().NewPublicKey(ephBytes)
	if err != nil {
		return nil, fmt.Errorf("seal: unusable ephemeral key: %w", err)
	}
	shared, err := to.enc.ECDH(eph)
	if err != nil {
		return nil, err
	}
	key, err := deriveKey(shared, ephBytes, to.Public().Enc, expect)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	body, err := aead.Open(nil, nonce, ct, expect.canonical())
	if err != nil {
		// Deliberately unspecific. Which field disagreed is information a
		// relay probing the protocol would like to have, and nobody honest
		// needs.
		return nil, errors.New("seal: could not open this message: it was not sealed for this identity, or it has been altered")
	}
	if len(body) < ed25519.SignatureSize {
		return nil, errors.New("seal: message carries no signature")
	}
	sig, plaintext := body[:ed25519.SignatureSize], body[ed25519.SignatureSize:]

	if !ed25519.Verify(expectFrom.Sign, append(expect.canonical(), plaintext...), sig) {
		return nil, errors.New("seal: the signature does not verify: this message was not written by the expected sender")
	}
	return plaintext, nil
}

// deriveKey binds the key to the ephemeral key, the recipient, and every field
// of the metadata.
//
// Binding the metadata into the KEY as well as the signature means a message
// whose header has been altered does not merely fail a signature check: it
// fails to decrypt at all, so nothing that has been tampered with ever reaches
// code that might act on it.
func deriveKey(shared, ephPub, recipientPub []byte, m Meta) ([]byte, error) {
	salt := append(append([]byte{}, ephPub...), recipientPub...)
	info := append([]byte("heliograph-seal-v1"), m.canonical()...)
	return hkdf.Key(sha256.New, shared, salt, string(info), chacha20poly1305.KeySize)
}
