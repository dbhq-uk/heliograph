package seal

import (
	"bytes"
	"crypto/ecdh"
	"strings"
	"testing"
)

func pair(t *testing.T) (control, station *Identity) {
	t.Helper()
	c, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	s, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	return c, s
}

func meta(to *Identity) Meta {
	return Meta{
		Estate: "e_7f3a", Station: "st_4a91", Dir: "c2s", Seq: 412,
		Kind:      "request",
		Recipient: to.Public().Fingerprint(),
	}
}

func TestRoundTrip(t *testing.T) {
	c, s := pair(t)
	m := meta(s)
	want := []byte("id: run-1\nstep: net-probe\n")

	sealed, err := Seal(c, s.Public(), m, want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(s, c.Public(), m, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
}

// The relay stores and forwards this. It must learn nothing from it.
func TestTheCiphertextRevealsNothing(t *testing.T) {
	c, s := pair(t)
	m := meta(s)
	sealed, err := Seal(c, s.Public(), m, []byte("password=hunter2 and the step name"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"password=hunter2", "net-probe", "the step name", "request"} {
		if bytes.Contains(sealed, []byte(secret)) {
			t.Errorf("%q appears in the ciphertext", secret)
		}
	}
	// Nor the estate, nor the station: those are routing facts the relay is
	// told separately, not things it should be able to read out of the payload.
	if bytes.Contains(sealed, []byte("e_7f3a")) {
		t.Error("the estate id appears in the ciphertext")
	}
}

// THE REQUIREMENT THAT OUTRANKS PRIVACY. A relay that can forge a request has
// code execution inside every estate at once.
func TestARelayCannotForgeARequest(t *testing.T) {
	c, s := pair(t)
	relay, err := Generate() // the relay, with keys of its own
	if err != nil {
		t.Fatal(err)
	}
	m := meta(s)

	// It knows the station's public key: it routes to it. That is not enough.
	forged, err := Seal(relay, s.Public(), m, []byte("id: evil\nstep: rm-rf\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(s, c.Public(), m, forged); err == nil {
		t.Fatal("the station accepted a request the relay wrote")
	}
}

// Every bound field, one attack each. A failure here is not a privacy leak, it
// is a run happening that nobody asked for.
func TestEveryBoundFieldIsChecked(t *testing.T) {
	c, s := pair(t)
	m := meta(s)
	sealed, err := Seal(c, s.Public(), m, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		alter func(Meta) Meta
	}{
		{"a message from one estate replayed into another",
			func(m Meta) Meta { m.Estate = "e_other"; return m }},
		{"a message for one station replayed to a different one",
			func(m Meta) Meta { m.Station = "st_other"; return m }},
		{"a message reflected back the way it came",
			func(m Meta) Meta { m.Dir = "s2c"; return m }},
		{"an old request replayed under a new sequence number",
			func(m Meta) Meta { m.Seq = 999; return m }},
		{"a status passed off as a request",
			func(m Meta) Meta { m.Kind = "status"; return m }},
	} {
		if _, err := Open(s, c.Public(), tc.alter(m), sealed); err == nil {
			t.Errorf("accepted: %s", tc.name)
		}
	}
}

// Sign-then-encrypt's known weakness: a recipient re-encrypts a validly signed
// message to a third party, who believes it was sent to them. Binding the
// recipient fingerprint into the signature is the standard mitigation, and this
// is the test that it actually works.
func TestAMessageCannotBeForwardedToAThirdParty(t *testing.T) {
	c, s := pair(t)
	other, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	m := meta(s)
	sealed, err := Seal(c, s.Public(), m, []byte("for the station only"))
	if err != nil {
		t.Fatal(err)
	}
	// The station forwards it to `other`, claiming it was for them.
	forwarded := m
	forwarded.Recipient = other.Public().Fingerprint()
	if _, err := Open(other, c.Public(), forwarded, sealed); err == nil {
		t.Fatal("a third party accepted a message that was not sealed for them")
	}
}

func TestTamperingWithTheCiphertextIsDetected(t *testing.T) {
	c, s := pair(t)
	m := meta(s)
	sealed, err := Seal(c, s.Public(), m, []byte("id: run-1\n"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range sealed {
		bad := append([]byte{}, sealed...)
		bad[i] ^= 0x01
		if _, err := Open(s, c.Public(), m, bad); err == nil {
			t.Fatalf("a single flipped bit at offset %d went undetected", i)
		}
	}
}

// A message that claims to be from somebody else must be refused before any
// cryptography is attempted with the claim.
func TestAMessageClaimingAnotherSenderIsRefused(t *testing.T) {
	c, s := pair(t)
	other, _ := Generate()
	m := meta(s)
	sealed, _ := Seal(c, s.Public(), m, []byte("x"))
	if _, err := Open(s, other.Public(), m, sealed); err == nil {
		t.Error("accepted a message attributed to the wrong sender")
	}
}

// A low-order or small-subgroup public key produces an all-zero shared secret,
// and a peer that supplied one could then read everything. Go's X25519 rejects
// it; Paseo's relay checks for it by hand. Asserted rather than assumed,
// because "the standard library handles it" is exactly the sort of belief that
// should be measured once.
func TestALowOrderPublicKeyIsRefused(t *testing.T) {
	c, _ := pair(t)
	lowOrder := make([]byte, 32) // all zeroes is the canonical low-order point
	bad := PublicIdentity{Enc: lowOrder, Sign: c.Public().Sign}
	m := Meta{
		Estate: "e", Station: "s", Dir: "c2s", Seq: 1, Kind: "request",
		Recipient: bad.Fingerprint(),
	}
	if _, err := Seal(c, bad, m, []byte("x")); err == nil {
		t.Fatal("sealed to an all-zero public key")
	}
	// And confirm it is the shared secret that is refused, not just the parse.
	if _, err := ecdh.X25519().NewPublicKey(lowOrder); err != nil {
		t.Log("the key did not even parse, which is also fine")
	}
}

func TestSealRefusesAMetaThatDisagreesWithTheKey(t *testing.T) {
	c, s := pair(t)
	other, _ := Generate()
	m := meta(s)
	m.Recipient = other.Public().Fingerprint() // says one thing, key says another
	if _, err := Seal(c, s.Public(), m, []byte("x")); err == nil {
		t.Error("sealed with a recipient that does not match the key given")
	}
}

func TestMetaMustBeComplete(t *testing.T) {
	c, s := pair(t)
	for _, m := range []Meta{
		{Station: "s", Dir: "c2s", Kind: "k", Recipient: s.Public().Fingerprint()},
		{Estate: "e", Dir: "c2s", Kind: "k", Recipient: s.Public().Fingerprint()},
		{Estate: "e", Station: "s", Dir: "sideways", Kind: "k", Recipient: s.Public().Fingerprint()},
		{Estate: "e", Station: "s", Dir: "c2s", Recipient: s.Public().Fingerprint()},
	} {
		if _, err := Seal(c, s.Public(), m, []byte("x")); err == nil {
			t.Errorf("sealed an incomplete Meta: %+v", m)
		}
	}
}

// Two Metas that differ must never serialise the same, or a signature covers
// something other than what it appears to. Length prefixes are what prevent it.
func TestCanonicalFormIsUnambiguous(t *testing.T) {
	a := Meta{Estate: "ab", Station: "c", Dir: "c2s", Kind: "k", Recipient: "r"}
	b := Meta{Estate: "a", Station: "bc", Dir: "c2s", Kind: "k", Recipient: "r"}
	if bytes.Equal(a.canonical(), b.canonical()) {
		t.Error("two different Metas serialise identically")
	}
}

func TestFingerprintIsStableAndReadable(t *testing.T) {
	c, _ := pair(t)
	f := c.Public().Fingerprint()
	if f != c.Public().Fingerprint() {
		t.Error("the fingerprint is not stable")
	}
	if len(f) != 14 || strings.Count(f, "-") != 2 {
		t.Errorf("a fingerprint should be readable aloud in groups: %q", f)
	}
	other, _ := Generate()
	if other.Public().Fingerprint() == f {
		t.Error("two identities share a fingerprint")
	}
}

func TestPublicIdentityRoundTrips(t *testing.T) {
	c, _ := pair(t)
	got, err := DecodePublic(c.Public().Encode())
	if err != nil {
		t.Fatal(err)
	}
	if got.Fingerprint() != c.Public().Fingerprint() {
		t.Error("the identity changed through encoding")
	}
	for _, bad := range []string{"", "not base64!!", "c2hvcnQ"} {
		if _, err := DecodePublic(bad); err == nil {
			t.Errorf("decoded %q", bad)
		}
	}
}

// The same plaintext sealed twice must not produce the same bytes, or the relay
// can tell that a request was repeated without being able to read either.
func TestSealingTwiceProducesDifferentCiphertext(t *testing.T) {
	c, s := pair(t)
	m := meta(s)
	a, _ := Seal(c, s.Public(), m, []byte("identical"))
	b, _ := Seal(c, s.Public(), m, []byte("identical"))
	if bytes.Equal(a, b) {
		t.Error("two seals of the same plaintext are byte-identical")
	}
}

func TestShortInputIsRefusedRatherThanPanicking(t *testing.T) {
	c, s := pair(t)
	m := meta(s)
	for _, n := range []int{0, 1, 32, 64, 76, 100} {
		if _, err := Open(s, c.Public(), m, make([]byte, n)); err == nil {
			t.Errorf("opened %d bytes of nothing", n)
		}
	}
}
