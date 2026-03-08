"""
Wayzn API Traffic Capture Script for mitmproxy.

Usage:
    1. Install mitmproxy: pip install mitmproxy
    2. Start proxy: mitmdump -s wayzn_capture.py -p 8080
    3. Configure your phone to use this proxy (see README)
    4. Open the Wayzn app and perform open/close actions
    5. Captured API calls are saved to wayzn_api_log.json

The script filters for Wayzn-related traffic and logs:
- Request method, URL, headers, body
- Response status, headers, body
- Timestamps for replay
"""

import json
import os
import time
from datetime import datetime
from mitmproxy import http, ctx

LOG_FILE = os.path.join(os.path.dirname(__file__), "wayzn_api_log.json")

# Known and suspected Wayzn-related domains
WAYZN_DOMAINS = [
    "wayzn.com",
    "wayzn.io",
    "api.wayzn",
    "cloud.wayzn",
]

# Additional IoT platform domains the device might use
IOT_PLATFORM_DOMAINS = [
    "particle.io",
    "api.particle.io",
    "amazonaws.com",
    "iot.amazonaws.com",
    "firebaseio.com",
    "googleapis.com",
]

# Domains to always ignore
IGNORE_DOMAINS = [
    "google-analytics.com",
    "googletagmanager.com",
    "facebook.com",
    "crashlytics.com",
    "app-measurement.com",
    "doubleclick.net",
    "googlesyndication.com",
    "appsflyer.com",
    "branch.io",
    "adjust.com",
]


def _load_log() -> list:
    if os.path.exists(LOG_FILE):
        with open(LOG_FILE, "r") as f:
            return json.load(f)
    return []


def _save_log(entries: list):
    with open(LOG_FILE, "w") as f:
        json.dump(entries, f, indent=2, default=str)


def _is_wayzn_related(host: str) -> bool:
    """Check if a host is potentially Wayzn-related."""
    host_lower = host.lower()
    for domain in IGNORE_DOMAINS:
        if domain in host_lower:
            return False
    for domain in WAYZN_DOMAINS:
        if domain in host_lower:
            return True
    for domain in IOT_PLATFORM_DOMAINS:
        if domain in host_lower:
            return True
    return False


def _safe_decode(data: bytes | None) -> str | None:
    if data is None:
        return None
    try:
        text = data.decode("utf-8")
        try:
            return json.loads(text)
        except (json.JSONDecodeError, ValueError):
            return text
    except UnicodeDecodeError:
        return data.hex()


class WayznCapture:
    """Capture and log Wayzn API traffic."""

    def __init__(self):
        self.capture_all = os.environ.get("WAYZN_CAPTURE_ALL", "").lower() in (
            "1",
            "true",
            "yes",
        )
        if self.capture_all:
            ctx.log.info(
                "WAYZN_CAPTURE_ALL=true: capturing ALL traffic (discovery mode)"
            )
        else:
            ctx.log.info(
                "Capturing Wayzn-related traffic only. "
                "Set WAYZN_CAPTURE_ALL=true to capture everything."
            )

    def response(self, flow: http.HTTPFlow):
        host = flow.request.pretty_host
        if not self.capture_all and not _is_wayzn_related(host):
            return

        entry = {
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "request": {
                "method": flow.request.method,
                "url": flow.request.pretty_url,
                "host": host,
                "path": flow.request.path,
                "headers": dict(flow.request.headers),
                "body": _safe_decode(flow.request.get_content()),
            },
            "response": {
                "status_code": flow.response.status_code,
                "headers": dict(flow.response.headers),
                "body": _safe_decode(flow.response.get_content()),
            },
        }

        entries = _load_log()
        entries.append(entry)
        _save_log(entries)

        action = _guess_action(entry)
        label = f" [{action}]" if action else ""
        ctx.log.info(
            f"[WAYZN] {flow.request.method} {flow.request.pretty_url} "
            f"-> {flow.response.status_code}{label}"
        )


def _guess_action(entry: dict) -> str | None:
    """Try to identify what action this API call represents."""
    url = entry["request"]["url"].lower()
    body = entry["request"].get("body")
    body_str = json.dumps(body).lower() if body else ""

    for keyword in ("open", "unlock", "activate"):
        if keyword in url or keyword in body_str:
            return "OPEN"
    for keyword in ("close", "lock", "deactivate"):
        if keyword in url or keyword in body_str:
            return "CLOSE"
    for keyword in ("status", "state", "info"):
        if keyword in url:
            return "STATUS"
    for keyword in ("auth", "login", "token", "session", "oauth"):
        if keyword in url:
            return "AUTH"
    return None


addons = [WayznCapture()]
