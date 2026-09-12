#!/usr/bin/env python3
"""A minimal S3-compatible store that VERIFIES SigV4, for the conformance suite.

Usage: s3-stub.py <access-key> <secret-key> <region> <bucket> <port|0>

WHY IT VERIFIES RATHER THAN ACCEPTS
===================================
A stub that took any Authorization header would let the station's signer be
completely wrong and still pass every property in the suite. The signature is
the one part of this transport that cannot be checked by looking at what came
back: a real store answers a wrong one with 403 and says nothing about why, and
a stub that shrugs would move that failure from CI to somebody's estate.

So this recomputes the signature from the request it actually received, using
python's own hmac and hashlib, and refuses a mismatch with the same opaque 403
a real store gives. That makes it a THIRD independent implementation - after Go
and the station's bash - and the three have to agree.

It deliberately does NOT tell the client which part disagreed, for the same
reason AWS does not: an oracle here would let the suite pass with a signer that
was being guided to the answer. What it does instead is print the two canonical
requests to stderr, where CI can read them and an attacker cannot.

WHAT IT IS NOT
==============
Not a storage service. No multipart, no versioning, no ACLs, no persistence,
and the object store is a dict. It implements exactly what the station and the
CLI use: GET, PUT, and ListObjectsV2.
"""

import hashlib
import hmac
import sys
import threading
import urllib.parse as up
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ACCESS = sys.argv[1] if len(sys.argv) > 1 else "AKIDEXAMPLE"
SECRET = sys.argv[2] if len(sys.argv) > 2 else "secret"
REGION = sys.argv[3] if len(sys.argv) > 3 else "eu-west-2"
BUCKET = sys.argv[4] if len(sys.argv) > 4 else "bucket"
PORT = int(sys.argv[5]) if len(sys.argv) > 5 else 0

OBJECTS = {}
LOCK = threading.Lock()


def aws_escape(s):
    # RFC 3986 unreserved only. `quote` with safe="~" leaves `/` alone, so the
    # caller escapes path segments individually.
    return up.quote(s, safe="~")


def canonical_uri(path):
    return "/".join(aws_escape(seg) for seg in path.split("/"))


def canonical_query(query):
    q = up.parse_qs(query, keep_blank_values=True)
    parts = []
    for k in sorted(q):
        for v in sorted(q[k]):
            parts.append(aws_escape(k) + "=" + aws_escape(v))
    return "&".join(parts)


def signing_key(date):
    k = ("AWS4" + SECRET).encode()
    for part in (date, REGION, "s3", "aws4_request"):
        k = hmac.new(k, part.encode(), hashlib.sha256).digest()
    return k


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *a):  # quiet; the suite's output is the signal
        pass

    def _send(self, code, body=b"", ctype="application/xml"):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if body:
            self.wfile.write(body)

    def _verify(self, body):
        """True when the Authorization header signs THIS request."""
        auth = self.headers.get("Authorization", "")
        if not auth.startswith("AWS4-HMAC-SHA256 "):
            print("stub: no SigV4 Authorization header", file=sys.stderr)
            return False
        try:
            fields = dict(
                p.strip().split("=", 1) for p in auth[len("AWS4-HMAC-SHA256 "):].split(",")
            )
            cred = fields["Credential"]
            signed = fields["SignedHeaders"]
            given = fields["Signature"]
        except Exception as e:
            print("stub: unreadable Authorization header: %s" % e, file=sys.stderr)
            return False

        keyid, date, region, service, terminator = cred.split("/")
        if keyid != ACCESS:
            print("stub: unknown access key %r" % keyid, file=sys.stderr)
            return False
        if region != REGION or service != "s3" or terminator != "aws4_request":
            print("stub: wrong credential scope %r" % cred, file=sys.stderr)
            return False

        amzdate = self.headers.get("X-Amz-Date", "")
        payload_hash = self.headers.get("X-Amz-Content-Sha256", "")
        # THE HASH IS CHECKED AGAINST THE BYTES, not taken on trust. A station
        # that signed one body and sent another would otherwise pass here and
        # fail against a real store - which is the single most confusing
        # failure in this area.
        actual = hashlib.sha256(body).hexdigest()
        if payload_hash != actual:
            print("stub: x-amz-content-sha256 does not match the body sent",
                  file=sys.stderr)
            return False

        u = up.urlsplit(self.path)
        block = ""
        for name in signed.split(";"):
            if name == "host":
                block += "host:%s\n" % self.headers.get("Host", "")
            elif name == "x-amz-date":
                block += "x-amz-date:%s\n" % amzdate
            elif name == "x-amz-content-sha256":
                block += "x-amz-content-sha256:%s\n" % payload_hash
            else:
                block += "%s:%s\n" % (name, (self.headers.get(name) or "").strip())

        canon = "\n".join([
            self.command, canonical_uri(u.path), canonical_query(u.query),
            block, signed, payload_hash,
        ])
        scope = "/".join([date, REGION, "s3", "aws4_request"])
        sts = "\n".join([
            "AWS4-HMAC-SHA256", amzdate, scope,
            hashlib.sha256(canon.encode()).hexdigest(),
        ])
        want = hmac.new(signing_key(date), sts.encode(), hashlib.sha256).hexdigest()
        if not hmac.compare_digest(want, given):
            # TO STDERR, NEVER TO THE CLIENT. A real store says nothing, and a
            # stub that explained would let a wrong signer be guided to the
            # right answer by the test that is supposed to catch it.
            print("stub: SIGNATURE MISMATCH\n--- canonical request the stub built\n%s\n"
                  "--- string to sign\n%s\n--- want %s\n--- got  %s"
                  % (canon, sts, want, given), file=sys.stderr)
            return False
        return True

    def _key(self):
        u = up.urlsplit(self.path)
        parts = [p for p in u.path.split("/") if p]
        if not parts or parts[0] != BUCKET:
            return None
        return up.unquote("/".join(parts[1:]))

    def do_GET(self):
        u = up.urlsplit(self.path)
        q = up.parse_qs(u.query)
        if not self._verify(b""):
            self._send(403, b"<Error><Code>SignatureDoesNotMatch</Code></Error>")
            return
        if q.get("list-type") == ["2"]:
            prefix = (q.get("prefix") or [""])[0]
            with LOCK:
                keys = sorted(k for k in OBJECTS if k.startswith(prefix))
            body = "<?xml version=\"1.0\"?><ListBucketResult>" + "".join(
                "<Contents><Key>%s</Key></Contents>" % k for k in keys
            ) + "<IsTruncated>false</IsTruncated></ListBucketResult>"
            self._send(200, body.encode())
            return
        key = self._key()
        with LOCK:
            blob = OBJECTS.get(key)
        if blob is None:
            self._send(404, b"<Error><Code>NoSuchKey</Code></Error>")
            return
        self._send(200, blob, "text/plain")

    def do_PUT(self):
        n = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(n)
        if not self._verify(body):
            self._send(403, b"<Error><Code>SignatureDoesNotMatch</Code></Error>")
            return
        key = self._key()
        if key is None:
            self._send(404, b"<Error><Code>NoSuchBucket</Code></Error>")
            return
        with LOCK:
            OBJECTS[key] = body
        self._send(200)

    def _refuse(self):
        self._send(405, b"<Error><Code>MethodNotAllowed</Code></Error>")

    do_DELETE = _refuse
    do_POST = _refuse


if __name__ == "__main__":
    srv = ThreadingHTTPServer(("127.0.0.1", PORT), Handler)
    # The bound port, printed and flushed, so the caller can bind 0 and be told
    # which one it got. A fixed port makes a test that cannot run twice at once.
    print(srv.server_address[1], flush=True)
    srv.serve_forever()
