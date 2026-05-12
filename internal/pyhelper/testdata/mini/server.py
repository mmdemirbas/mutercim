"""Minimal stdlib-only test HTTP server for pyhelper tests.

Listens on $PORT and answers GET /health with {"status": "ready"} and
GET /echo?msg=... with the echoed message. Used by Server lifecycle
tests — does NOT require any external Python dependencies so uv sync
in CI / repeat runs is fast and offline-capable.
"""
from __future__ import annotations

import json
import os
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse, parse_qs


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format: str, *args: object) -> None:  # noqa: A002
        # Quiet by default; pyhelper tests don't need access-log noise.
        pass

    def do_GET(self) -> None:  # noqa: N802
        parsed = urlparse(self.path)
        if parsed.path == "/health":
            self._send_json(200, {"status": "ready"})
            return
        if parsed.path == "/echo":
            qs = parse_qs(parsed.query)
            msg = qs.get("msg", [""])[0]
            self._send_json(200, {"echo": msg})
            return
        self._send_json(404, {"error": "not found"})

    def _send_json(self, code: int, body: dict[str, object]) -> None:
        payload = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)


def main() -> None:
    port = int(os.environ.get("PORT", "0"))
    if port == 0:
        print("PORT environment variable required", file=sys.stderr)
        sys.exit(2)
    server = HTTPServer(("127.0.0.1", port), Handler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
