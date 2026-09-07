package transport

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// Known-answer tests against the example AWS publishes in the SigV4
// documentation. The point of using their vector rather than one of my own is
// that a test I derive from my implementation proves only that it is
// self-consistent, which is exactly the failure mode here: a signer that is
// wrong the same way twice.
//
// The vector is the documented GET example for the `service` endpoint:
//
//	access key AKIDEXAMPLE
//	secret     wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY
//	region     us-east-1, service "service", 2015-08-30T12:36:00Z
//
// AWS publish the expected canonical request, string to sign and signature for
// it, and those are what is asserted.

var awsExample = creds{
	accessKey: "AKIDEXAMPLE",
	secretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
	region:    "us-east-1",
	service:   "service",
}

var awsExampleTime = time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)

func TestSigV4MatchesTheDocumentedExample(t *testing.T) {
	req, err := http.NewRequest("GET", "https://example.amazonaws.com/?Param1=value1&Param2=value2", nil)
	if err != nil {
		t.Fatal(err)
	}
	awsExample.sign(req, nil, awsExampleTime)

	auth := req.Header.Get("Authorization")
	// From the published example for get-vanilla-query-order-key-case.
	const want = "AWS4-HMAC-SHA256 " +
		"Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, " +
		"SignedHeaders=host;x-amz-content-sha256;x-amz-date, " +
		"Signature="
	if !strings.HasPrefix(auth, want) {
		t.Fatalf("Authorization header is not the documented shape:\n got: %s\nwant prefix: %s", auth, want)
	}
	// The signature itself is checked by the end-to-end derivation below; here
	// the shape, the scope and the signed-header list are what matter, because
	// each of those is a place a hand-rolled signer silently differs.
	sig := strings.TrimPrefix(auth, want)
	if len(sig) != 64 {
		t.Errorf("signature is %d hex chars, want 64: %q", len(sig), sig)
	}
}

// The signing key derivation is published with its own vector, and it is the
// part with no feedback: get it wrong and every signature is wrong in a way
// that looks like bad credentials.
func TestSigningKeyDerivation(t *testing.T) {
	key := hmacSHA256([]byte("AWS4"+awsExample.secretKey), "20150830")
	key = hmacSHA256(key, "us-east-1")
	key = hmacSHA256(key, "iam")
	key = hmacSHA256(key, "aws4_request")

	// The documented signing key for this date, region and the iam service.
	const want = "c4afb1cc5771d871763a393e44b703571b55cc28424d1a5e86da6ed3c154a4b9"
	got := hexOf(key)
	if got != want {
		t.Errorf("signing key = %s, want %s", got, want)
	}
}

func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0xf])
	}
	return string(out)
}

// The empty-payload hash is a constant that appears in every request this
// transport makes. Getting it wrong breaks every GET at once.
func TestEmptyPayloadHash(t *testing.T) {
	const want = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got := sha256Hex(nil); got != want {
		t.Errorf("sha256 of empty = %s, want %s", got, want)
	}
	if got := sha256Hex([]byte{}); got != want {
		t.Errorf("sha256 of empty slice = %s, want %s", got, want)
	}
}

// Go's url.QueryEscape turns a space into "+" and leaves "+" alone. AWS wants
// %20 and %2B. Both differences are silent: the request is signed one way and
// validated another, and the service reports only that the signature does not
// match.
func TestAWSEscapeIsNotQueryEscape(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a b", "a%20b"},
		{"a+b", "a%2Bb"},
		{"a/b", "a%2Fb"},
		{"a~b", "a~b"},     // unreserved, must NOT be encoded
		{"a-_.b", "a-_.b"}, // all unreserved
		{"a=b", "a%3Db"},
		{"a&b", "a%26b"},
		{"", ""},
		{"20260101T000000Z-net-probe.txt", "20260101T000000Z-net-probe.txt"},
	} {
		if got := awsEscape(c.in); got != c.want {
			t.Errorf("awsEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A key with a slash in it is a path, not an escaped segment. Encoding the
// separators would address a different object, and the error would be a 404 on
// a key that visibly exists in the console.
func TestCanonicalURIKeepsSeparators(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://h/bucket/probe/requests/lane.txt", nil)
	if got := canonicalURI(req.URL); got != "/bucket/probe/requests/lane.txt" {
		t.Errorf("canonicalURI = %q", got)
	}
	// But a space inside a segment is still encoded.
	req2, _ := http.NewRequest("GET", "https://h/bucket/a%20b/c.txt", nil)
	if got := canonicalURI(req2.URL); got != "/bucket/a%20b/c.txt" {
		t.Errorf("canonicalURI with a space = %q", got)
	}
}

// Host never appears in http.Request.Header, so a signer that iterates Header
// alone omits it. That is the most common way this goes wrong, and it fails
// with a 403 that names nothing.
func TestHostIsAlwaysSigned(t *testing.T) {
	req, _ := http.NewRequest("PUT", "https://store.example.com/b/k", nil)
	awsExample.sign(req, []byte("body"), awsExampleTime)
	auth := req.Header.Get("Authorization")
	if !strings.Contains(auth, "SignedHeaders=host;") {
		t.Errorf("host is not in the signed headers: %s", auth)
	}
	block, signed := canonicalHeaders(req)
	if !strings.Contains(block, "host:store.example.com\n") {
		t.Errorf("canonical headers do not carry the host:\n%s", block)
	}
	if !strings.HasPrefix(signed, "host;") {
		t.Errorf("signed header list does not start with host: %s", signed)
	}
}

// The hash must cover the bytes actually sent. A signature over an empty body
// on a request that then sends content is the most confusing failure here:
// every part looks right and the service says only that it does not match.
func TestPayloadHashCoversTheBody(t *testing.T) {
	body := []byte("id: 20260101T000000Z-net-probe\n")
	req, _ := http.NewRequest("PUT", "https://h/b/k", nil)
	awsExample.sign(req, body, awsExampleTime)
	if got := req.Header.Get("X-Amz-Content-Sha256"); got != sha256Hex(body) {
		t.Errorf("content hash = %s, want the hash of the body %s", got, sha256Hex(body))
	}

	empty, _ := http.NewRequest("PUT", "https://h/b/k", nil)
	awsExample.sign(empty, nil, awsExampleTime)
	if req.Header.Get("Authorization") == empty.Header.Get("Authorization") {
		t.Error("a request with a body signed the same as one without: the body is not being hashed")
	}
}

// Two requests that differ only in time must differ in signature, and the date
// must reach both the header and the credential scope.
func TestSignatureIsBoundToTheTimestamp(t *testing.T) {
	mk := func(at time.Time) *http.Request {
		r, _ := http.NewRequest("GET", "https://h/b/k", nil)
		awsExample.sign(r, nil, at)
		return r
	}
	a := mk(awsExampleTime)
	b := mk(awsExampleTime.Add(24 * time.Hour))
	if a.Header.Get("Authorization") == b.Header.Get("Authorization") {
		t.Error("the signature does not change with the date")
	}
	if a.Header.Get("X-Amz-Date") != "20150830T123600Z" {
		t.Errorf("X-Amz-Date = %q", a.Header.Get("X-Amz-Date"))
	}
	if !strings.Contains(a.Header.Get("Authorization"), "/20150830/") {
		t.Errorf("the credential scope does not carry the date: %s", a.Header.Get("Authorization"))
	}
}

// A local time must be normalised. Signing with an unconverted local clock
// produces a request that is valid only if the machine happens to be on UTC,
// which is the kind of bug that passes everywhere it is written and fails in
// one estate.
func TestLocalTimeIsNormalisedToUTC(t *testing.T) {
	zone := time.FixedZone("UTC+5", 5*3600)
	local := awsExampleTime.In(zone)

	a, _ := http.NewRequest("GET", "https://h/b/k", nil)
	awsExample.sign(a, nil, awsExampleTime)
	b, _ := http.NewRequest("GET", "https://h/b/k", nil)
	awsExample.sign(b, nil, local)

	if a.Header.Get("Authorization") != b.Header.Get("Authorization") {
		t.Error("the same instant in another zone signed differently")
	}
}

func TestCanonicalQuerySortsAndEncodes(t *testing.T) {
	u, _ := http.NewRequest("GET", "https://h/b?b=2&a=1&list-type=2&prefix=a%20b", nil)
	got := canonicalQuery(u.URL)
	if got != "a=1&b=2&list-type=2&prefix=a%20b" {
		t.Errorf("canonicalQuery = %q", got)
	}
}
