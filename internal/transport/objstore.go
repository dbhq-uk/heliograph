package transport

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/dbhq-uk/heliograph/internal/wire"
)

// ObjStore is the transport for an estate that has object storage and nothing
// else usable: no git host either side can reach, no mounted share, and no
// route out for a relay.
//
// That combination sounds contrived and is common. A locked-down cloud subnet
// where a firewall appliance holds the default route, with no policy for the
// subnet the station sits in, has no outbound anything - while traffic to a
// private endpoint stays inside the virtual network, never reaches the
// appliance, and works normally. Storage is reachable when nothing else is.
//
// THE LAYOUT IS THE STATION'S, NOT A NEW ONE
//
// The bash station already speaks this, in toolkit/pigeonhole.sh:
//
//	<prefix>requests/<lane>.txt
//	<prefix>status/<lane>.txt
//	<prefix>logs/<name>.txt
//
// A lane is one investigation, so two running at once do not overwrite each
// other. The documents inside are the same `key: value` shape that crosses
// every other transport, which is why a log from here reads identically to one
// that came back over git.
//
// This side was written to match that, rather than the two being designed
// together, and the direction matters: the station is deployed in the field and
// cannot be upgraded to meet a near side that changed its mind.
//
// # S3-COMPATIBLE, WHICH IS MOST OF THEM
//
// AWS S3, Cloudflare R2, MinIO, Backblaze B2, DigitalOcean Spaces, Ceph. Azure
// Blob is NOT S3-compatible and is reached by the station's own pigeonhole
// path; adding a second signing scheme here for it is a separate argument with
// a separate PR.
type ObjStore struct {
	endpoint string // scheme and host, no bucket
	bucket   string
	prefix   string // "" or something ending in /
	lane     string
	c        creds
	http     *http.Client
	now      func() time.Time // injectable, so signing can be tested at a fixed instant
}

// ObjStoreConfig is what NewObjStore needs. A struct rather than eight
// positional strings, because four of them are strings that look alike and
// transposing two would authenticate to the wrong place with the right key.
type ObjStoreConfig struct {
	Endpoint  string // https://s3.eu-west-2.amazonaws.com, https://<acct>.r2.cloudflarestorage.com
	Bucket    string
	Prefix    string // optional, for a bucket shared with something else
	Lane      string // one investigation
	AccessKey string
	SecretKey string
	Region    string // "auto" for R2, which has no regions but demands one anyway
}

func NewObjStore(cfg ObjStoreConfig) (*ObjStore, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("an object store needs an endpoint, such as https://s3.eu-west-2.amazonaws.com")
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("%q is not a usable endpoint: it needs a scheme and a host", cfg.Endpoint)
	}
	if u.Scheme != "https" {
		// Refused rather than warned about. The documents carry step names,
		// hostnames and captured output, and http:// would put all of it on
		// the wire in an estate chosen precisely because it is careful.
		return nil, fmt.Errorf("the endpoint must be https, not %q: these documents carry captured output", u.Scheme)
	}
	if cfg.Bucket == "" {
		return nil, errors.New("an object store needs a bucket")
	}
	if cfg.Lane == "" {
		return nil, errors.New("an object store needs a lane: one per investigation, so two do not overwrite each other")
	}
	if strings.ContainsAny(cfg.Lane, `/\`) || cfg.Lane == "." || cfg.Lane == ".." {
		return nil, fmt.Errorf("%q is not a usable lane: it becomes part of an object key", cfg.Lane)
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		// Named as environment variables because that is where they must come
		// from. An estate file is on disk and gets copied around; these are
		// read at use and never written.
		return nil, errors.New("no credentials: set HELIOGRAPH_S3_ACCESS_KEY and HELIOGRAPH_S3_SECRET_KEY")
	}
	prefix := cfg.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	region := cfg.Region
	if region == "" {
		// R2 and MinIO have no regions and reject a request without one
		// anyway. `auto` is what Cloudflare document and MinIO accepts it.
		region = "auto"
	}
	return &ObjStore{
		endpoint: strings.TrimSuffix(cfg.Endpoint, "/"),
		bucket:   cfg.Bucket,
		prefix:   prefix,
		lane:     cfg.Lane,
		c: creds{
			accessKey: cfg.AccessKey,
			secretKey: cfg.SecretKey,
			region:    region,
			service:   "s3",
		},
		// A timeout, because the failure this must not have is hanging. A
		// station that cannot be reached is news; a control side that waits
		// forever to find that out is a person staring at a terminal.
		http: &http.Client{Timeout: 60 * time.Second},
		now:  time.Now,
	}, nil
}

func (o *ObjStore) key(parts ...string) string {
	return o.prefix + strings.Join(parts, "/")
}

func (o *ObjStore) Dir() string    { return o.endpoint + "/" + o.bucket }
func (o *ObjStore) Branch() string { return o.lane }

// Describe names the credential by kind and length, never by value. The same
// rule as every other transport: this string is printed by `doctor`, and
// `doctor` output ends up pasted into tickets.
func (o *ObjStore) Describe() string {
	return fmt.Sprintf("object store %s/%s, lane %s, credential: access key %s… (%d chars)",
		o.endpoint, o.bucket, o.lane,
		o.c.accessKey[:min(4, len(o.c.accessKey))], len(o.c.secretKey))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// do signs and sends. Path-style addressing (host/bucket/key) rather than
// virtual-host style (bucket.host/key), because MinIO, Ceph and a private
// endpoint with a certificate for the host and not for every bucket name under
// it all need it, and S3 proper still accepts it.
func (o *ObjStore) do(method, key string, body []byte, query url.Values) (*http.Response, error) {
	u := o.endpoint + "/" + o.bucket
	if key != "" {
		u += "/" + key
	}
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}
	o.c.sign(req, body, o.now())
	return o.http.Do(req)
}

// get returns the object, or (nil, nil) when it is simply not there.
//
// Absent is not an error. A station that has never run has published no status,
// and there is no request until somebody sends one. Reporting those as failures
// would make the ordinary first minute of an investigation look broken.
func (o *ObjStore) get(key string) ([]byte, error) {
	resp, err := o.do("GET", key, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %w", o.endpoint, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return io.ReadAll(resp.Body)
	case http.StatusNotFound:
		return nil, nil
	default:
		return nil, o.httpErr("read "+key, resp)
	}
}

func (o *ObjStore) put(key string, body []byte) error {
	resp, err := o.do("PUT", key, body, nil)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", o.endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return o.httpErr("write "+key, resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// httpErr turns a status code into something that says what to do.
//
// S3 error bodies are XML with a Code and a Message, and both are more use than
// the number. 403 in particular means several different things - wrong key,
// right key with no permission on this bucket, or a clock more than fifteen
// minutes out - and the last one is invisible unless somebody says it.
func (o *ObjStore) httpErr(what string, resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var e struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	_ = xml.Unmarshal(b, &e)

	msg := fmt.Sprintf("cannot %s: HTTP %d", what, resp.StatusCode)
	if e.Code != "" {
		msg += " " + e.Code
		if e.Message != "" {
			msg += ": " + e.Message
		}
	}
	switch {
	case resp.StatusCode == http.StatusForbidden && strings.Contains(e.Code, "RequestTimeTooSkewed"):
		msg += "\nThis machine's clock is more than 15 minutes from the store's. " +
			"Signatures are bound to the timestamp, so fix the clock rather than the credentials."
	case resp.StatusCode == http.StatusForbidden:
		msg += "\nThe key may be wrong, or right but without permission on this bucket. " +
			"Read access is not write access: `heliograph doctor` writes, and that is the check that matters."
	case resp.StatusCode == http.StatusNotFound && strings.Contains(e.Code, "NoSuchBucket"):
		msg += "\nThe bucket does not exist at this endpoint. Check the endpoint before the bucket name: " +
			"a bucket that exists in another account or region reports exactly this."
	}
	return errors.New(msg)
}

func (o *ObjStore) FetchStatus() (wire.Status, error) {
	b, err := o.get(o.key("status", o.lane+".txt"))
	if err != nil {
		return wire.Status{}, err
	}
	if b == nil {
		// Never run is not a fault. The zero Status says so.
		return wire.Status{}, nil
	}
	return wire.ParseStatus(b)
}

// PutRequest publishes the request.
//
// A single PUT, which object stores make atomic for a whole object: a reader
// gets the old object or the new one, never a half-written one. That property
// is why there is no write-then-rename dance here as there is on a file share,
// and it is worth stating because its absence would be a bug nobody could see:
// a station polling this key can read it at any instant, and a truncated `env:`
// would be acted on.
func (o *ObjStore) PutRequest(r wire.Request) error {
	if err := r.Validate(); err != nil {
		return err
	}
	return o.put(o.key("requests", o.lane+".txt"), r.Marshal())
}

// listResult is the subset of ListObjectsV2 that matters here.
type listResult struct {
	Contents []struct {
		Key string `xml:"Key"`
	} `xml:"Contents"`
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
}

// ListLogs names the captured logs, newest first.
//
// It follows continuation tokens. A single page is 1000 keys and a long
// investigation passes that, at which point a non-paginating implementation
// silently stops listing the oldest logs - which are exactly the ones somebody
// is looking for when they are reading back over a week.
func (o *ObjStore) ListLogs() ([]string, error) {
	prefix := o.key("logs", "")
	var names []string
	token := ""
	for page := 0; ; page++ {
		q := url.Values{"list-type": {"2"}, "prefix": {prefix}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		resp, err := o.do("GET", "", nil, q)
		if err != nil {
			return nil, fmt.Errorf("cannot reach %s: %w", o.endpoint, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, o.httpErr("list logs", &http.Response{
				StatusCode: resp.StatusCode, Body: io.NopCloser(bytes.NewReader(body))})
		}
		if readErr != nil {
			return nil, readErr
		}
		var lr listResult
		if err := xml.Unmarshal(body, &lr); err != nil {
			return nil, fmt.Errorf("the store's listing was not readable: %w", err)
		}
		for _, c := range lr.Contents {
			name := strings.TrimPrefix(c.Key, prefix)
			// A partial log is the in-flight snapshot of a run still going. It
			// is genuinely useful and it is not a captured log: listing it
			// beside finished ones invites reading a truncated file as the
			// whole answer.
			if name == "" || strings.Contains(name, "/") || strings.HasSuffix(name, ".partial.txt") {
				continue
			}
			names = append(names, name)
		}
		if !lr.IsTruncated || lr.NextContinuationToken == "" {
			break
		}
		token = lr.NextContinuationToken
		// A store that keeps returning a token forever would spin here. The
		// bound is far above any real investigation and well below forever.
		if page > 100 {
			return nil, errors.New("the store kept paginating past 100 pages: refusing to loop")
		}
	}
	// Newest first, because the one you want is almost always the last run.
	// The names begin with a UTC stamp, so lexical descending IS newest first.
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

func (o *ObjStore) ReadLog(name string) ([]byte, error) {
	clean, err := safeLogName(name)
	if err != nil {
		return nil, err
	}
	b, err := o.get(o.key("logs", clean))
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, fmt.Errorf("no log named %q in %s", clean, o.Dir())
	}
	return b, nil
}

// Check answers "will this work from here" by WRITING.
//
// A bucket policy that grants read and not write stats, lists and gets
// perfectly, and fails on the first PUT - which would be the log, an hour
// later, with the evidence captured and nobody left to tell. So the probe is a
// real object, written and then deleted.
//
// It writes into the lane's own prefix rather than the bucket root, because the
// credential may legitimately be scoped to that prefix, and a probe at the root
// would report a failure that does not exist.
func (o *ObjStore) Check() error {
	probe := o.key("requests", ".heliograph-write-check")
	if err := o.put(probe, []byte("heliograph write check\n")); err != nil {
		return err
	}
	resp, err := o.do("DELETE", probe, nil, nil)
	if err != nil {
		return fmt.Errorf("wrote the probe but could not reach the store to remove it: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK, http.StatusNotFound:
		return nil
	default:
		// Reported rather than swallowed. A credential that can write but not
		// delete works for the loop, and leaves a probe object behind on every
		// doctor run, which somebody will eventually find and wonder about.
		return fmt.Errorf("the store accepted the write but refused to remove the probe (HTTP %d). "+
			"The loop will work; %s is left behind", resp.StatusCode, probe)
	}
}

var _ Transport = (*ObjStore)(nil)
