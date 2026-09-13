package cloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Service is one hosted heliograph service, reached over HTTPS.
//
// THERE IS NO BUILT-IN DEFAULT URL, and that is deliberate rather than
// unfinished. A hostname compiled into a released binary is a claim that
// something answers there, and a binary in the field outlives the claim: the
// service's own hostname is still moving (`heliograph-io/heliograph-cloud#62`
// and `#64`), and a station pointed at a name that used to resolve fails in the
// one way this project exists to avoid - on the far side, silently, to somebody
// who cannot debug it. It comes from `--service` or HELIOGRAPH_CLOUD_URL, and
// `login` says so when neither is set.
type Service struct {
	Base   string
	client *http.Client
}

// NewService attaches to a service, refusing a URL a credential should not
// travel over.
func NewService(base string) (*Service, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil, errors.New("a service needs a URL: pass --service or set HELIOGRAPH_CLOUD_URL")
	}
	// HTTPS, because every call carries a credential in a header. Loopback is
	// the exception and only loopback: a Worker running on this machine has no
	// network to protect. The same rule, and the same narrowness, as
	// `internal/estate`'s relay check - a hostname somebody controls could
	// resolve to loopback today and elsewhere tomorrow.
	if !strings.HasPrefix(base, "https://") && !isLoopback(base) {
		return nil, fmt.Errorf("%s is not https. Every call carries a credential in a header, so it may not travel in clear", base)
	}
	return &Service{
		Base: base,
		// Longer than any single call should take and short enough that a
		// hung service is reported rather than waited on for ever. The device
		// poll does its own waiting between calls rather than inside one.
		client: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func isLoopback(u string) bool {
	for _, p := range []string{"http://127.0.0.1", "http://localhost", "http://[::1]"} {
		if strings.HasPrefix(u, p+"/") || strings.HasPrefix(u, p+":") || u == p {
			return true
		}
	}
	return false
}

// Auth is one credential under one scheme.
//
// THREE SCHEMES AND NO SHARED ONE. `Control` is the control plane, `Station` is
// a station's own endpoints, `Bearer` is the read-only API. Presenting one
// where another belongs is refused at the parser rather than by a lookup that
// misses, which is what keeps two tokens with two scopes from flattening into
// one. Each is a distinct type here for the same reason: a caller cannot reuse
// a variable by accident, because the constructor names which credential it is.
type Auth struct {
	scheme string
	token  string
}

func Control(token string) Auth  { return Auth{"Control", token} }
func Station(token string) Auth  { return Auth{"Station", token} }
func ReadOnly(token string) Auth { return Auth{"Bearer", token} }
func NoAuth() Auth               { return Auth{} }

func (a Auth) header() string {
	if a.scheme == "" || a.token == "" {
		return ""
	}
	return a.scheme + " " + a.token
}

// call sends one request and returns the response for the caller to read.
//
// It never retries. A retry hides a refusal behind a delay, and every refusal
// this service issues is one somebody has to read.
func (s *Service) call(method, path string, auth Auth, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, s.Base+path, body)
	if err != nil {
		return nil, err
	}
	if h := auth.header(); h != "" {
		req.Header.Set("Authorization", h)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %w", s.Base, err)
	}
	return resp, nil
}

// readJSON turns a response into a value, or into the service's own refusal.
func readJSON(resp *http.Response, out any) error {
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return refusalFrom(resp.StatusCode, raw)
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s answered with something that is not JSON: %w", resp.Request.URL.Path, err)
	}
	return nil
}

func (s *Service) getJSON(path string, auth Auth, out any) error {
	resp, err := s.call(http.MethodGet, path, auth, "", nil)
	if err != nil {
		return err
	}
	return readJSON(resp, out)
}

func (s *Service) postJSON(path string, auth Auth, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	resp, err := s.call(http.MethodPost, path, auth, "application/json", body)
	if err != nil {
		return err
	}
	return readJSON(resp, out)
}

// postBytes sends a document as its own bytes.
//
// THE DOCUMENT IS THE EVIDENCE. A status document is line-based `key: value`
// text (`internal/wire/status.go`), and a control node that wrapped it in a
// JSON envelope before uploading would hand the archive a rendering of the
// evidence rather than the evidence. So the context goes in the query string
// and the body is the document, byte for byte.
func (s *Service) postBytes(path string, auth Auth, body []byte, out any) error {
	resp, err := s.call(http.MethodPost, path, auth, "application/octet-stream", bytes.NewReader(body))
	if err != nil {
		return err
	}
	return readJSON(resp, out)
}

func (s *Service) putBytes(path string, auth Auth, body []byte, out any) error {
	resp, err := s.call(http.MethodPut, path, auth, "application/octet-stream", bytes.NewReader(body))
	if err != nil {
		return err
	}
	return readJSON(resp, out)
}
