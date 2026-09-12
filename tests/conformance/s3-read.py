#!/usr/bin/env python3
"""Read the newest delivered log back out of the S3 stub, as the CLI would.

Usage: s3-read.py <url> <key> <secret> <region> <bucket> <prefix>

THE CONTROL SIDE'S MOVE, NOT THE STATION'S. The conformance suite reads a
delivered log from the RECEIVING END, never from the working tree that wrote it
- a station with no far side would otherwise let a delivery that never happened
look identical to one that did.

It signs its own requests, which makes this a fourth place SigV4 is written and
is the point: if the station's signature were wrong, the stub would refuse it;
if this one were wrong, the read would fail rather than quietly returning the
empty string.

`.partial.txt` IS SKIPPED, exactly as internal/transport/objstore.go skips it.
A progress snapshot listed beside finished logs is how a truncated file gets
read as the whole answer - and a reader here that included them would make the
suite assert the opposite of the property.
"""

import datetime
import hashlib
import hmac
import re
import sys
import urllib.parse as up
import urllib.request

URL, KEY, SECRET, REGION, BUCKET, PREFIX = sys.argv[1:7]
EMPTY = hashlib.sha256(b"").hexdigest()


def headers(method, path, query):
    now = datetime.datetime.now(datetime.timezone.utc)
    amz = now.strftime("%Y%m%dT%H%M%SZ")
    day = amz[:8]
    host = up.urlsplit(URL).netloc
    cu = "/".join(up.quote(s, safe="~") for s in path.split("/"))
    block = "host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n" % (host, EMPTY, amz)
    signed = "host;x-amz-content-sha256;x-amz-date"
    canon = "\n".join([method, cu, query, block, signed, EMPTY])
    scope = "/".join([day, REGION, "s3", "aws4_request"])
    sts = "\n".join([
        "AWS4-HMAC-SHA256", amz, scope,
        hashlib.sha256(canon.encode()).hexdigest(),
    ])
    k = ("AWS4" + SECRET).encode()
    for part in (day, REGION, "s3", "aws4_request"):
        k = hmac.new(k, part.encode(), hashlib.sha256).digest()
    sig = hmac.new(k, sts.encode(), hashlib.sha256).hexdigest()
    return {
        "Host": host,
        "X-Amz-Date": amz,
        "X-Amz-Content-Sha256": EMPTY,
        "Authorization":
            "AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s"
            % (KEY, scope, signed, sig),
    }


def get(path, query=""):
    h = headers("GET", path, query)
    url = URL + "/".join(up.quote(s, safe="~") for s in path.split("/"))
    if query:
        url += "?" + query
    return urllib.request.urlopen(urllib.request.Request(url, headers=h), timeout=10).read()


logs_prefix = PREFIX + "logs/"
query = "list-type=2&prefix=" + up.quote(logs_prefix, safe="~")
body = get("/" + BUCKET, query).decode()
keys = [k for k in re.findall(r"<Key>([^<]+)</Key>", body) if not k.endswith(".partial.txt")]
if not keys:
    sys.exit(1)
# Newest last: the names begin with a UTC stamp, so lexical order IS time order.
sys.stdout.write(get("/" + BUCKET + "/" + sorted(keys)[-1]).decode())
