package seal

import (
	"crypto/ed25519"
	"encoding/binary"
)

// A detached signature over a document that travels OUTSIDE a sealed envelope.
//
// Seal covers its plaintext because sign-then-encrypt puts the signature inside
// the ciphertext. A trusted-set change cannot use that: it has to be verifiable
// by a station that does not yet trust the sender's key, and by an estate owner
// reading the transport repo with `cat`. So it is signed in the clear, detached,
// and the verifier is the trusted set rather than one recorded peer.
//
// DOMAIN SEPARATION IS THE WHOLE OF WHY THIS IS NOT `ed25519.Sign` INLINE. Two
// signing surfaces under one key is how a signature made for one purpose gets
// replayed as another, and both of ours are Ed25519 over bytes this package
// chose. The prefix below is length-prefixed and constant, so:
//
//   - Seal's signed input begins with eight bytes of Version, which is 1.
//   - This one begins with eight bytes holding len(documentPrefix), which is 22.
//
// They cannot collide, and neither can two documents with different domains,
// because the domain is length-prefixed rather than concatenated.
//
// Nothing here is invented. Ed25519 from the standard library, over a canonical
// byte string this package builds. If a design here seemed to need more, the
// design would be wrong.
const documentPrefix = "heliograph-document-v1"

// signingInput is the exact bytes both halves agree on.
func signingInput(domain string, body []byte) []byte {
	var b []byte
	add := func(s []byte) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(s)))
		b = append(b, n[:]...)
		b = append(b, s...)
	}
	add([]byte(documentPrefix))
	add([]byte(domain))
	add(body)
	return b
}

// SignDocument signs body under a named domain.
//
// An empty domain is refused by returning nil rather than by panicking: the
// caller is a command that would otherwise print a stack trace to an operator,
// and a nil signature fails verification everywhere it is checked.
func (id *Identity) SignDocument(domain string, body []byte) []byte {
	if domain == "" {
		return nil
	}
	return ed25519.Sign(id.sign, signingInput(domain, body))
}

// VerifyDocument checks one. It is the only way a detached signature is
// accepted anywhere in this repository.
func (p PublicIdentity) VerifyDocument(domain string, body, sig []byte) bool {
	if domain == "" || len(p.Sign) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(p.Sign), signingInput(domain, body), sig)
}

// Equal compares two public identities.
//
// Both halves, not just the signing key. A member of a trusted set is
// identified by the pair, and comparing one half would let a key be re-added
// under a different encryption half - which is a different party holding the
// same authority to author.
func (p PublicIdentity) Equal(q PublicIdentity) bool {
	if len(p.Enc) != len(q.Enc) || len(p.Sign) != len(q.Sign) {
		return false
	}
	for i := range p.Enc {
		if p.Enc[i] != q.Enc[i] {
			return false
		}
	}
	for i := range p.Sign {
		if p.Sign[i] != q.Sign[i] {
			return false
		}
	}
	return true
}
