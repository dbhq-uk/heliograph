#!/usr/bin/env python3
# =============================================================================
#  relay-stub.py - a relay, in memory, for tests that must not need a network
# =============================================================================
# The real relay is a Cloudflare Worker in another repository, deployed at
# heliograph-relay.dbhq.uk. Nothing in this repository could exercise the relay
# transport end to end without it, so nothing did: the relay had unit tests for
# its sequence numbers and no test at all of a log actually arriving.
#
# WHAT THIS IS AND IS NOT. It is a queue with an HTTP interface and a bearer
# token, which is the entire contract the station side depends on. It is NOT a
# reimplementation of the Worker: no durable objects, no estates file, no
# retention, no rate limiting. Those are the real relay's problems and testing
# them here would only assert that two of my own programs agree.
#
# It is deliberately IGNORANT OF THE PAYLOAD. Every body is an opaque sealed
# envelope, stored and handed back byte for byte. That is not laziness - it is
# the property worth having. A stub that could read the messages would be a
# stub that could accidentally accept one the real relay would mangle, and the
# seal is the only thing standing between a station and whoever runs the relay.
#
#   GET  /health                          200, no token needed
#   POST /v1/<estate>/<station>/<dir>     202, appends one message
#   GET  /v1/<estate>/<station>/<dir>     200, DRAINS the queue and returns it
#
# GET COLLECTS AND DELETES, exactly as the deployed relay does. That is the
# single most surprising thing about this protocol and the source of a real
# defect - `tp_check` used to probe the station's own request queue and ate the
# operator's pending request every time the station started - so a stub that
# left messages in place would make the test suite agree with a bug.
#
# Usage: relay-stub.py <token> [port]
#        prints the port it bound on, then serves until killed.
# =============================================================================
import json
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

TOKEN = sys.argv[1] if len(sys.argv) > 1 else "test-token"
PORT = int(sys.argv[2]) if len(sys.argv) > 2 else 0

# (estate, station, dir) -> [body, ...]. Under a lock because the station side
# runs the loop and the runner as separate processes on purpose, and a stub
# that serialised them would hide exactly the contention worth testing.
QUEUES = {}
LOCK = threading.Lock()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass  # the test's output is the interesting one

    def _route(self):
        # /v1/<estate>/<station>/<dir>, with the query already stripped.
        path = self.path.split("?", 1)[0]
        parts = [p for p in path.split("/") if p]
        if len(parts) != 4 or parts[0] != "v1":
            return None
        return (parts[1], parts[2], parts[3])

    def _authorised(self):
        return self.headers.get("Authorization") == "Bearer " + TOKEN

    def _send(self, code, body=b"", ctype="application/json"):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if body:
            self.wfile.write(body)

    def do_GET(self):
        if self.path.split("?", 1)[0] == "/health":
            self._send(200, b'{"ok":true}')
            return
        key = self._route()
        if key is None:
            self._send(404)
            return
        if not self._authorised():
            self._send(401)
            return
        with LOCK:
            msgs = QUEUES.pop(key, [])
        # A JSON ARRAY of {seq, body}, which is the shape `heliograph-seal
        # open` parses - it sorts by seq, applies the replay floor, and takes
        # the newest that opens. Anything else is "nothing usable in the
        # relay's answer", and the station treats that as nothing to do rather
        # than as an error, so getting this wrong is silent on both sides.
        #
        # Concatenated rather than re-encoded. Each stored message is already
        # the exact bytes one side sealed, and a stub that round-tripped them
        # through a JSON library would be a stub that could quietly repair a
        # malformed envelope the real relay would pass along untouched.
        self._send(200, ("[" + ",".join(msgs) + "]").encode())

    def do_POST(self):
        key = self._route()
        if key is None:
            self._send(404)
            return
        if not self._authorised():
            self._send(401)
            return
        n = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(n).decode("utf-8", "replace").strip()
        # Rejected here rather than stored, because the real relay rejects it
        # and a stub that accepted anything would let a broken seal look fine.
        try:
            json.loads(body)
        except ValueError:
            self._send(400, b'{"error":"not json"}')
            return
        with LOCK:
            QUEUES.setdefault(key, []).append(body)
        self._send(202, b'{"accepted":true}')


if __name__ == "__main__":
    srv = ThreadingHTTPServer(("127.0.0.1", PORT), Handler)
    # The bound port, printed and flushed, so the caller can bind port 0 and be
    # told which one it got. A fixed port makes tests that cannot run twice at
    # once, and CI runs these suites in parallel.
    print(srv.server_address[1], flush=True)
    srv.serve_forever()
