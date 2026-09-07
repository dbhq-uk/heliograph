package transport

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AWS Signature Version 4, implemented here rather than taken from the SDK.
//
// WHY NOT THE SDK
//
// aws-sdk-go-v2 with S3 pulls in something over a hundred packages. This
// product links two modules total, and the CI gate that enforces that exists so
// the argument has to be made out loud before the first one arrives. The
// argument here fails: signing is about a hundred and twenty lines of HMAC over
// a string built to a published recipe, and every one of those lines is in the
// specification. Carrying a hundred packages to avoid writing them would be the
// largest thing in the repository by an order of magnitude, for a binary whose
// whole proposition is that it is one static file you can drop on a machine.
//
// The risk this takes on is real and worth naming: SigV4 is easy to get subtly
// wrong, and wrong looks like a 403 that says nothing useful. That is answered
// with known-answer tests against the examples AWS publishes, in sigv4_test.go.
// Those vectors are the specification's own, so passing them is not a claim
// about my reading of it.
//
// WHAT IS DELIBERATELY NOT HERE
//
// No streaming signature, no chunked upload, no presigned URLs, no STS. The
// payloads are a request document, a status document and a log: small, whole,
// and hashed in memory. Every one of those features is somebody else's problem
// until this product has one.

const (
	sigAlgorithm = "AWS4-HMAC-SHA256"
	isoLayout    = "20060102T150405Z"
	dateLayout   = "20060102"
)

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// creds is what signing needs and nothing more. Held by value and never
// logged: Describe() reports the kind and length of a credential, never a
// character of it.
type creds struct {
	accessKey string
	secretKey string
	region    string
	service   string // "s3" here, but the algorithm is not S3-specific
}

// sign adds the Authorization header to req, and takes the body it will send.
//
// The body is passed rather than read off the request because the hash has to
// cover exactly the bytes that go on the wire, and reading req.Body to hash it
// would consume the reader the transport is about to send. A signature over a
// body that was then sent empty is the most confusing failure in this area:
// everything looks right and the service says the signature does not match.
func (c creds) sign(req *http.Request, body []byte, now time.Time) {
	now = now.UTC()
	amzDate := now.Format(isoLayout)
	scopeDate := now.Format(dateLayout)

	payloadHash := sha256Hex(body)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	// Host is not in Header on an outgoing request: net/http keeps it on the
	// URL. It still has to be signed, so it is put back for the canonical form.
	if req.Host == "" {
		req.Host = req.URL.Host
	}

	canonicalHeaders, signedHeaders := canonicalHeaders(req)

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL),
		canonicalQuery(req.URL),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := strings.Join([]string{scopeDate, c.region, c.service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		sigAlgorithm,
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	key := hmacSHA256([]byte("AWS4"+c.secretKey), scopeDate)
	key = hmacSHA256(key, c.region)
	key = hmacSHA256(key, c.service)
	key = hmacSHA256(key, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(key, stringToSign))

	req.Header.Set("Authorization", fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		sigAlgorithm, c.accessKey, scope, signedHeaders, signature))
}

// canonicalHeaders returns the signed headers block and the list of names.
//
// Host is included explicitly because it never appears in Header. Missing it is
// the single most common way a hand-rolled signer fails, and it fails with a
// 403 that names nothing.
func canonicalHeaders(req *http.Request) (block, signed string) {
	names := []string{"host"}
	values := map[string]string{"host": req.Host}
	for k, v := range req.Header {
		lower := strings.ToLower(k)
		// Only the headers that must be signed. Signing everything means any
		// proxy that adds a header on the way out breaks the signature.
		if lower != "content-type" && !strings.HasPrefix(lower, "x-amz-") {
			continue
		}
		names = append(names, lower)
		// Sequential whitespace collapses, and the value is trimmed. Values
		// here are machine-generated and would not trip this, but the recipe
		// says so and a signer that is right by luck is not right.
		values[lower] = strings.Join(strings.Fields(strings.Join(v, ",")), " ")
	}
	sort.Strings(names)

	var b strings.Builder
	for _, n := range names {
		b.WriteString(n)
		b.WriteByte(':')
		b.WriteString(values[n])
		b.WriteByte('\n')
	}
	return b.String(), strings.Join(names, ";")
}

// canonicalURI encodes each path segment, leaving the separators alone.
//
// url.EscapedPath is not enough: S3 wants each segment encoded with the same
// rules as a query value except that a slash stays a slash, and Go's escaping
// leaves some characters alone that AWS expects encoded.
func canonicalURI(u *url.URL) string {
	p := u.Path
	if p == "" {
		return "/"
	}
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = awsEscape(s)
	}
	return strings.Join(parts, "/")
}

func canonicalQuery(u *url.URL) string {
	q := u.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vs := append([]string(nil), q[k]...)
		sort.Strings(vs)
		for _, v := range vs {
			parts = append(parts, awsEscape(k)+"="+awsEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

// awsEscape is RFC 3986 unreserved-set encoding.
//
// Go's url.QueryEscape encodes a space as `+`, which AWS rejects, and leaves
// `+` itself alone, which changes the signed string. Both are silent. So the
// rule is written out rather than borrowed.
func awsEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
