package transport

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dbhq-uk/heliograph/internal/wire"
)

// fakeS3 is enough of an object store to hold the transport to its contract,
// and it verifies the signature rather than accepting anything.
//
// Be precise about what that proves. It re-signs with the SAME implementation,
// so it cannot catch a shared misunderstanding of the specification - that is
// what the AWS-published vectors in sigv4_test.go are for. What it does catch
// is the class of bug where the request that gets SIGNED is not the request
// that gets SENT: signing before net/http moves Host onto the URL, hashing a
// body the client then replaces, a query rewritten after signing. Those are
// self-inflicted, common, and invisible without a server that checks.
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	c       creds

	readOnly   bool // reject writes, as a read-only bucket policy does
	noDelete   bool // accept writes, refuse deletes
	pageSize   int  // force pagination when > 0
	skewed     bool // answer 403 RequestTimeTooSkewed
	unsigned   int  // count of requests that arrived without a valid signature
	lastMethod string
}

func newFakeS3() *fakeS3 {
	return &fakeS3{
		objects: map[string][]byte{},
		c: creds{
			accessKey: "AKIDEXAMPLE",
			secretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
			region:    "auto",
			service:   "s3",
		},
	}
}

// verify re-signs the request as the server and compares. This is what a real
// store does, so anything wrong in the canonical form shows up here.
func (f *fakeS3) verify(r *http.Request, body []byte) bool {
	got := r.Header.Get("Authorization")
	if got == "" {
		return false
	}
	stamp, err := time.Parse(isoLayout, r.Header.Get("X-Amz-Date"))
	if err != nil {
		return false
	}
	// Rebuild a request that looks like what the client signed. The client
	// signs before net/http moves Host onto the URL, so it is put back.
	clone := r.Clone(r.Context())
	clone.Host = r.Host
	clone.Header.Del("Authorization")
	f.c.sign(clone, body, stamp)
	return clone.Header.Get("Authorization") == got
}

func (f *fakeS3) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readAll(r)
		f.mu.Lock()
		f.lastMethod = r.Method
		f.mu.Unlock()

		if !f.verify(r, body) {
			f.mu.Lock()
			f.unsigned++
			f.mu.Unlock()
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `<Error><Code>SignatureDoesNotMatch</Code><Message>bad signature</Message></Error>`)
			return
		}
		if f.skewed {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `<Error><Code>RequestTimeTooSkewed</Code><Message>clock is off</Message></Error>`)
			return
		}

		// path is /bucket/key...
		p := strings.TrimPrefix(r.URL.Path, "/")
		i := strings.IndexByte(p, '/')
		key := ""
		if i >= 0 {
			key = p[i+1:]
		}

		f.mu.Lock()
		defer f.mu.Unlock()

		switch r.Method {
		case "GET":
			if r.URL.Query().Get("list-type") == "2" {
				f.list(w, r)
				return
			}
			b, ok := f.objects[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `<Error><Code>NoSuchKey</Code><Message>not found</Message></Error>`)
				return
			}
			_, _ = w.Write(b)
		case "PUT":
			if f.readOnly {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `<Error><Code>AccessDenied</Code><Message>read only</Message></Error>`)
				return
			}
			f.objects[key] = body
			w.WriteHeader(http.StatusOK)
		case "DELETE":
			if f.noDelete {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			delete(f.objects, key)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func (f *fakeS3) list(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	var keys []string
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sortStrings(keys)

	start := 0
	if tok := r.URL.Query().Get("continuation-token"); tok != "" {
		for i, k := range keys {
			if k == tok {
				start = i
				break
			}
		}
	}
	end := len(keys)
	truncated := false
	next := ""
	if f.pageSize > 0 && start+f.pageSize < len(keys) {
		end = start + f.pageSize
		truncated = true
		next = keys[end]
	}

	type content struct {
		Key string `xml:"Key"`
	}
	out := struct {
		XMLName               xml.Name  `xml:"ListBucketResult"`
		Contents              []content `xml:"Contents"`
		IsTruncated           bool      `xml:"IsTruncated"`
		NextContinuationToken string    `xml:"NextContinuationToken,omitempty"`
	}{IsTruncated: truncated, NextContinuationToken: next}
	for _, k := range keys[start:end] {
		out.Contents = append(out.Contents, content{Key: k})
	}
	_ = xml.NewEncoder(w).Encode(out)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func readAll(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	var b []byte
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		b = append(b, buf[:n]...)
		if err != nil {
			break
		}
	}
	if len(b) == 0 {
		return nil
	}
	return b
}

// store wires a transport to a fake. httptest serves http, and the constructor
// refuses anything but https, so the scheme check is exercised separately and
// bypassed here by building the struct directly.
func store(t *testing.T, f *fakeS3) (*ObjStore, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	o := &ObjStore{
		endpoint: srv.URL,
		bucket:   "heliograph",
		prefix:   "probe/",
		lane:     "net",
		c:        f.c,
		http:     srv.Client(),
		now:      time.Now,
	}
	return o, srv
}

func TestObjStoreRoundTripsARequest(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)

	req := wire.Request{Version: wire.Version, ID: "20260101T000000Z-net-probe",
		Step: "net-probe", Env: `HOSTS='sql01 sql02'`}
	if err := o.PutRequest(req); err != nil {
		t.Fatal(err)
	}
	// At the key the STATION reads. This is the contract with pigeonhole.sh,
	// and getting it wrong means a station that polls forever and finds
	// nothing, while this side reports a successful send.
	got, ok := f.objects["probe/requests/net.txt"]
	if !ok {
		t.Fatalf("nothing at probe/requests/net.txt; store has %v", keysOf(f))
	}
	back, err := wire.ParseRequest(got)
	if err != nil {
		t.Fatal(err)
	}
	if back.ID != req.ID || back.Step != req.Step || back.Env != req.Env {
		t.Errorf("the request came back changed: %+v", back)
	}
	if f.unsigned != 0 {
		t.Errorf("%d request(s) failed signature verification", f.unsigned)
	}
}

func TestObjStoreRefusesARequestThatWouldForgeAKey(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)
	// A newline in a value ends the line the station is reading and starts one
	// it was never sent. Refused here, where there is somebody to tell.
	err := o.PutRequest(wire.Request{Version: 1, ID: "x", Env: "A=b\nstop: yes"})
	if err == nil {
		t.Fatal("a request with a newline in a value was published")
	}
	if len(f.objects) != 0 {
		t.Error("it was written anyway")
	}
}

func TestObjStoreReadsStatusFromTheStationsKey(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)
	f.objects["probe/status/net.txt"] = []byte("state: running\nid: abc\nstep: net-probe\n")

	s, err := o.FetchStatus()
	if err != nil {
		t.Fatal(err)
	}
	if s.State != "running" || s.Step != "net-probe" {
		t.Errorf("status = %+v", s)
	}
}

// A station that has never run has published nothing. That is the ordinary
// first minute of an investigation, not a fault.
func TestObjStoreNoStatusIsNotAnError(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)
	s, err := o.FetchStatus()
	if err != nil {
		t.Fatalf("an absent status was reported as an error: %v", err)
	}
	if s.State != "" {
		t.Errorf("state = %q, want empty", s.State)
	}
}

func TestObjStoreListsLogsNewestFirst(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)
	for _, n := range []string{
		"20260101T000000Z-net-probe.txt",
		"20260103T000000Z-disk.txt",
		"20260102T000000Z-dns.txt",
	} {
		f.objects["probe/logs/"+n] = []byte("x")
	}

	names, err := o.ListLogs()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 || names[0] != "20260103T000000Z-disk.txt" {
		t.Errorf("logs are not newest first: %v", names)
	}
}

// A partial log is the in-flight snapshot of a run still going. Listing it
// beside finished logs invites reading a truncated file as the whole answer.
func TestObjStoreHidesPartialLogs(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)
	f.objects["probe/logs/20260101T000000Z-net.txt"] = []byte("done")
	f.objects["probe/logs/20260101T000000Z-net.partial.txt"] = []byte("half")

	names, err := o.ListLogs()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "20260101T000000Z-net.txt" {
		t.Errorf("partial logs are being listed: %v", names)
	}
}

// A page is 1000 keys. A long investigation passes that, and a non-paginating
// implementation silently stops listing the OLDEST logs, which are exactly the
// ones somebody reading back over a week is looking for.
func TestObjStoreFollowsPagination(t *testing.T) {
	f := newFakeS3()
	f.pageSize = 3
	o, _ := store(t, f)
	for i := 0; i < 10; i++ {
		f.objects[fmt.Sprintf("probe/logs/2026010%dT000000Z-run.txt", i)] = []byte("x")
	}
	names, err := o.ListLogs()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 10 {
		t.Errorf("got %d logs, want 10: pagination stopped early. %v", len(names), names)
	}
}

func TestObjStoreReadsALogWhole(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)
	body := "12:00:00 | starting\n12:00:01 | done\n"
	f.objects["probe/logs/20260101T000000Z-net.txt"] = []byte(body)

	b, err := o.ReadLog("20260101T000000Z-net.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != body {
		t.Errorf("the log came back changed: %q", b)
	}
}

// A log name becomes part of an object key. `../` in one would address
// something else in the bucket.
func TestObjStoreRefusesALogNameWithAPath(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)
	for _, bad := range []string{"../status/net.txt", "/etc/passwd", "a/b.txt", ""} {
		if _, err := o.ReadLog(bad); err == nil {
			t.Errorf("ReadLog(%q) was accepted", bad)
		}
	}
}

func TestObjStoreMissingLogSaysWhichOne(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)
	_, err := o.ReadLog("nope.txt")
	if err == nil {
		t.Fatal("a missing log was not reported")
	}
	if !strings.Contains(err.Error(), "nope.txt") {
		t.Errorf("the error does not name the log: %v", err)
	}
}

// Check WRITES. A read-only bucket policy lists and gets perfectly and fails on
// the first PUT, which would be the log, an hour later, with nobody left to
// tell.
func TestCheckWritesAndCleansUp(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)
	if err := o.Check(); err != nil {
		t.Fatalf("check failed against a working store: %v", err)
	}
	if len(f.objects) != 0 {
		t.Errorf("the probe object was left behind: %v", keysOf(f))
	}
	if f.lastMethod != "DELETE" {
		t.Errorf("the probe was not deleted; last method was %s", f.lastMethod)
	}
}

func TestCheckFailsOnAReadOnlyBucket(t *testing.T) {
	f := newFakeS3()
	f.readOnly = true
	o, _ := store(t, f)
	err := o.Check()
	if err == nil {
		t.Fatal("a read-only bucket passed the check")
	}
	// The message has to say what to do. A preflight that names a fault
	// without a remedy is a defect: the reader usually cannot ask anybody.
	if !strings.Contains(err.Error(), "Read access is not write access") {
		t.Errorf("the error does not explain the likely cause: %v", err)
	}
}

// A credential that can write but not delete works for the loop. Saying so, and
// naming the object left behind, is better than either failing or silently
// littering.
func TestCheckReportsAWriteOnlyCredentialHonestly(t *testing.T) {
	f := newFakeS3()
	f.noDelete = true
	o, _ := store(t, f)
	err := o.Check()
	if err == nil {
		t.Fatal("nothing was reported about the undeleted probe")
	}
	if !strings.Contains(err.Error(), "The loop will work") {
		t.Errorf("the error does not say the loop still works: %v", err)
	}
	if !strings.Contains(err.Error(), "probe/requests/.heliograph-write-check") {
		t.Errorf("the error does not name the object left behind: %v", err)
	}
}

// Clock skew is invisible unless something says it. 403 otherwise reads as a
// credential problem, and somebody spends an afternoon rotating a working key.
func TestClockSkewIsNamed(t *testing.T) {
	f := newFakeS3()
	f.skewed = true
	o, _ := store(t, f)
	_, err := o.FetchStatus()
	if err == nil {
		t.Fatal("a skewed clock was not reported")
	}
	if !strings.Contains(err.Error(), "clock") {
		t.Errorf("the error does not mention the clock: %v", err)
	}
}

// The probe goes in the lane's own prefix. A credential scoped to that prefix
// is legitimate, and a probe at the bucket root would report a failure that
// does not exist.
func TestCheckProbesInsideThePrefix(t *testing.T) {
	f := newFakeS3()
	f.noDelete = true // so the object survives for inspection
	o, _ := store(t, f)
	_ = o.Check()
	found := false
	for k := range f.objects {
		if strings.HasPrefix(k, "probe/") {
			found = true
		}
	}
	if !found {
		t.Errorf("the probe was written outside the prefix: %v", keysOf(f))
	}
}

func TestObjStoreDescribeNeverPrintsTheSecret(t *testing.T) {
	f := newFakeS3()
	o, _ := store(t, f)
	d := o.Describe()
	if strings.Contains(d, o.c.secretKey) {
		t.Fatalf("Describe leaked the secret key: %s", d)
	}
	// The full access key is an identifier rather than a secret, but printing
	// it whole into a ticket is still more than anybody needs.
	if strings.Contains(d, o.c.accessKey) {
		t.Errorf("Describe printed the whole access key: %s", d)
	}
	if !strings.Contains(d, "net") || !strings.Contains(d, "heliograph") {
		t.Errorf("Describe does not say which bucket and lane: %s", d)
	}
}

// --- construction ------------------------------------------------------------

func TestNewObjStoreRefusesWhatCannotWork(t *testing.T) {
	base := ObjStoreConfig{
		Endpoint: "https://s3.example.com", Bucket: "b", Lane: "net",
		AccessKey: "AK", SecretKey: "SK",
	}
	for _, c := range []struct {
		name   string
		mutate func(*ObjStoreConfig)
		want   string
	}{
		{"no endpoint", func(c *ObjStoreConfig) { c.Endpoint = "" }, "endpoint"},
		{"not a url", func(c *ObjStoreConfig) { c.Endpoint = "::not a url" }, "endpoint"},
		{"plain http", func(c *ObjStoreConfig) { c.Endpoint = "http://s3.example.com" }, "https"},
		{"no bucket", func(c *ObjStoreConfig) { c.Bucket = "" }, "bucket"},
		{"no lane", func(c *ObjStoreConfig) { c.Lane = "" }, "lane"},
		{"lane with a slash", func(c *ObjStoreConfig) { c.Lane = "a/b" }, "lane"},
		{"lane dotdot", func(c *ObjStoreConfig) { c.Lane = ".." }, "lane"},
		{"no access key", func(c *ObjStoreConfig) { c.AccessKey = "" }, "HELIOGRAPH_S3_ACCESS_KEY"},
		{"no secret", func(c *ObjStoreConfig) { c.SecretKey = "" }, "HELIOGRAPH_S3_ACCESS_KEY"},
	} {
		cfg := base
		c.mutate(&cfg)
		_, err := NewObjStore(cfg)
		if err == nil {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error does not mention %q: %v", c.name, c.want, err)
		}
	}
}

func TestNewObjStoreDefaultsAndNormalises(t *testing.T) {
	o, err := NewObjStore(ObjStoreConfig{
		Endpoint: "https://s3.example.com/", Bucket: "b", Prefix: "probe", Lane: "net",
		AccessKey: "AK", SecretKey: "SK",
	})
	if err != nil {
		t.Fatal(err)
	}
	// A prefix without a trailing slash would produce "probelogs/".
	if o.prefix != "probe/" {
		t.Errorf("prefix = %q, want probe/", o.prefix)
	}
	if strings.HasSuffix(o.endpoint, "/") {
		t.Errorf("endpoint keeps a trailing slash: %q, which would double up in every key", o.endpoint)
	}
	// R2 and MinIO have no regions and refuse a request without one anyway.
	if o.c.region != "auto" {
		t.Errorf("region = %q, want auto", o.c.region)
	}
}

func keysOf(f *fakeS3) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var k []string
	for key := range f.objects {
		k = append(k, key)
	}
	sortStrings(k)
	return k
}
