#!/usr/bin/env python3
"""
Analyze captured Wayzn API traffic and generate Go client code.

Usage:
    python analyze.py                    # Analyze wayzn_api_log.json
    python analyze.py --generate-go      # Generate Go client from captured traffic
    python analyze.py --file other.json  # Use a different log file
"""

import argparse
import json
import os
import sys
from collections import defaultdict
from urllib.parse import urlparse


def load_log(path: str) -> list:
    with open(path, "r") as f:
        return json.load(f)


def analyze(entries: list):
    """Print a summary of captured API traffic."""
    print(f"\n{'='*60}")
    print(f"  Wayzn API Traffic Analysis — {len(entries)} requests captured")
    print(f"{'='*60}\n")

    # Group by host
    by_host = defaultdict(list)
    for e in entries:
        by_host[e["request"]["host"]].append(e)

    print("Hosts contacted:")
    for host, reqs in sorted(by_host.items(), key=lambda x: -len(x[1])):
        print(f"  {host}: {len(reqs)} requests")

    # Group by endpoint (method + path)
    print("\nEndpoints:")
    by_endpoint = defaultdict(list)
    for e in entries:
        key = f"{e['request']['method']} {e['request']['path']}"
        by_endpoint[key].append(e)

    for endpoint, reqs in sorted(by_endpoint.items(), key=lambda x: -len(x[1])):
        statuses = set(r["response"]["status_code"] for r in reqs)
        print(f"  {endpoint}")
        print(f"    Count: {len(reqs)}, Status codes: {statuses}")

    # Auth patterns
    print("\nAuthentication patterns:")
    auth_headers = set()
    for e in entries:
        headers = e["request"]["headers"]
        for key in headers:
            kl = key.lower()
            if any(
                term in kl
                for term in ("auth", "token", "cookie", "session", "api-key", "x-api")
            ):
                auth_headers.add(key)
    if auth_headers:
        for h in sorted(auth_headers):
            sample = None
            for e in entries:
                if h in e["request"]["headers"]:
                    val = e["request"]["headers"][h]
                    # Mask the value for security
                    if len(val) > 20:
                        sample = val[:10] + "..." + val[-5:]
                    else:
                        sample = val[:5] + "..."
                    break
            print(f"  {h}: {sample}")
    else:
        print("  No obvious auth headers found")

    # Identify open/close commands
    print("\nIdentified actions:")
    for e in entries:
        url = e["request"]["url"].lower()
        body = json.dumps(e["request"].get("body", "")).lower()
        combined = url + " " + body
        for action in ["open", "close", "lock", "unlock", "status", "state"]:
            if action in combined:
                print(
                    f"  [{action.upper()}] {e['request']['method']} {e['request']['path']}"
                )
                if e["request"].get("body"):
                    print(f"    Body: {json.dumps(e['request']['body'])[:200]}")
                break


def generate_go(entries: list):
    """Generate Go client code from captured traffic patterns."""
    if not entries:
        print("No entries to generate from.")
        return

    # Find the base URL
    hosts = defaultdict(int)
    for e in entries:
        parsed = urlparse(e["request"]["url"])
        base = f"{parsed.scheme}://{parsed.netloc}"
        hosts[base] += 1

    base_url = max(hosts, key=hosts.get)

    # Find auth header
    auth_header = None
    auth_value_example = None
    for e in entries:
        for key, val in e["request"]["headers"].items():
            kl = key.lower()
            if any(
                term in kl
                for term in ("authorization", "token", "x-api-key", "x-auth")
            ):
                auth_header = key
                auth_value_example = val
                break
        if auth_header:
            break

    # Find open/close endpoints
    open_endpoint = None
    close_endpoint = None
    status_endpoint = None
    auth_endpoint = None

    for e in entries:
        url = e["request"]["url"].lower()
        body = json.dumps(e["request"].get("body", "")).lower()
        combined = url + " " + body
        path = e["request"]["path"]
        method = e["request"]["method"]

        if "open" in combined and not open_endpoint:
            open_endpoint = {"method": method, "path": path, "body": e["request"].get("body")}
        elif "close" in combined and not close_endpoint:
            close_endpoint = {"method": method, "path": path, "body": e["request"].get("body")}
        elif any(term in combined for term in ("status", "state")) and not status_endpoint:
            status_endpoint = {"method": method, "path": path}
        elif any(term in combined for term in ("auth", "login", "token")) and not auth_endpoint:
            auth_endpoint = {"method": method, "path": path, "body": e["request"].get("body")}

    go_code = _render_go_client(
        base_url, auth_header, auth_value_example,
        open_endpoint, close_endpoint, status_endpoint, auth_endpoint,
    )

    out_path = os.path.join(os.path.dirname(__file__), "..", "pkg", "wayzn", "client_generated.go")
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    with open(out_path, "w") as f:
        f.write(go_code)
    print(f"Generated Go client at: {out_path}")


def _render_go_client(
    base_url, auth_header, auth_value_example,
    open_ep, close_ep, status_ep, auth_ep,
):
    auth_header_go = auth_header or "Authorization"

    def render_method(name, ep, default_method="POST", default_path="/unknown"):
        if ep is None:
            return f"""
func (c *Client) {name}(ctx context.Context) error {{
\t// TODO: Endpoint not yet discovered. Run the capture script and perform
\t// a {name.lower()} action in the Wayzn app, then re-run analyze.py --generate-go
\treturn fmt.Errorf("{name.lower()} endpoint not yet discovered")
}}"""
        body_code = ""
        if ep.get("body"):
            body_json = json.dumps(ep["body"])
            body_code = f'\tbody := bytes.NewBufferString(`{body_json}`)\n'
            reader = "body"
        else:
            reader = "nil"

        return f"""
func (c *Client) {name}(ctx context.Context) error {{
{body_code}\treq, err := http.NewRequestWithContext(ctx, "{ep['method']}", c.baseURL+"{ep['path']}", {reader})
\tif err != nil {{
\t\treturn fmt.Errorf("{name.lower()}: %w", err)
\t}}
\treq.Header.Set("{auth_header_go}", c.token)
\treq.Header.Set("Content-Type", "application/json")

\tresp, err := c.http.Do(req)
\tif err != nil {{
\t\treturn fmt.Errorf("{name.lower()}: %w", err)
\t}}
\tdefer resp.Body.Close()

\tif resp.StatusCode >= 400 {{
\t\treturn fmt.Errorf("{name.lower()}: HTTP %d", resp.StatusCode)
\t}}
\treturn nil
}}"""

    return f"""// Code generated by analyze.py from captured Wayzn API traffic. DO NOT EDIT.
package wayzn

import (
\t"bytes"
\t"context"
\t"fmt"
\t"net/http"
)

const DefaultBaseURL = "{base_url}"

type Client struct {{
\tbaseURL string
\ttoken   string
\thttp    *http.Client
}}

func NewClient(baseURL, token string) *Client {{
\tif baseURL == "" {{
\t\tbaseURL = DefaultBaseURL
\t}}
\treturn &Client{{
\t\tbaseURL: baseURL,
\t\ttoken:   token,
\t\thttp:    &http.Client{{}},
\t}}
}}
{render_method("Open", open_ep)}
{render_method("Close", close_ep)}
{render_method("Status", status_ep, "GET", "/status")}
"""


def main():
    parser = argparse.ArgumentParser(description="Analyze captured Wayzn API traffic")
    parser.add_argument(
        "--file",
        default=os.path.join(os.path.dirname(__file__), "wayzn_api_log.json"),
        help="Path to the API log JSON file",
    )
    parser.add_argument(
        "--generate-go",
        action="store_true",
        help="Generate Go client code from captured traffic",
    )
    args = parser.parse_args()

    if not os.path.exists(args.file):
        print(f"No log file found at {args.file}")
        print("Run the capture script first to collect API traffic.")
        sys.exit(1)

    entries = load_log(args.file)
    if not entries:
        print("Log file is empty. Capture some traffic first.")
        sys.exit(1)

    analyze(entries)

    if args.generate_go:
        print("\nGenerating Go client...")
        generate_go(entries)


if __name__ == "__main__":
    main()
