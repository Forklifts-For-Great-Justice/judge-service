#!/usr/bin/env python3
"""
Test script for the JudgeService Shenanigan API.

Auth flow (M2M client_credentials):
  1. POST https://auth.hackfortress.net/api/oidc/token  (obtain bearer token)
  2. Decode JWT -> extract sub + scp (scope) claims
  3. GET https://judge.hackfortress.net/shenanigans      (with auth headers)
"""

import argparse
import json
import sys
import urllib.error
import urllib.request
from base64 import b64decode


# ---------------------------------------------------------------------------
# Auth (OIDC client_credentials)
# ---------------------------------------------------------------------------

AUTH_SERVER = "https://auth.hackfortress.net"
TOKEN_ENDPOINT = f"{AUTH_SERVER}/api/oidc/token"
JUDGE_URL = "https://judge.hackfortress.net"


def fetch_token(client_id: str, client_secret: str) -> str:
    """Obtain an OIDC access token via client_credentials grant."""
    body = (
        f"grant_type=client_credentials"
        f"&client_id={urllib.parse.quote(client_id)}"
        f"&client_secret={urllib.parse.quote(client_secret)}"
        f"&scope=profile+groups+judge"
    ).encode()

    req = urllib.request.Request(
        TOKEN_ENDPOINT,
        data=body,
        method="POST",
        headers={"Content-Type": "application/x-www-form-urlencoded"},
    )

    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            token_data = json.loads(resp.read())
            return token_data["access_token"]
    except urllib.error.HTTPError as exc:
        body_text = ""
        try:
            body_text = exc.read().decode()
        except Exception:
            pass
        print(f"ERROR: token request failed ({exc.code}): {body_text}", file=sys.stderr)
        sys.exit(1)


# ---------------------------------------------------------------------------
# JWT decoder (bare-bones — no pyjwt dependency)
# ---------------------------------------------------------------------------

def decode_jwt_claims(token: str) -> dict:
    """Decode the payload of a JWT (header + payload) without verifying the signature."""
    parts = token.split(".")
    if len(parts) != 3:
        print("ERROR: access token is not a valid JWT (expected 3 parts)", file=sys.stderr)
        sys.exit(1)

    payload_b64 = parts[1] + "=" * (4 - (len(parts[1]) % 4))
    payload = json.loads(b64decode(payload_b64))
    return payload


# ---------------------------------------------------------------------------
# JudgeService API
# ---------------------------------------------------------------------------

# Module-level mutable container for the judge URL so cmd_ functions can read it.
_config = {"judge_url": "https://judge.hackfortress.net"}


def make_request(method: str, path: str, headers: dict, token: str | None = None, body: dict | None = None) -> dict:
    """Send an HTTP request to JudgeService and return (status_code, parsed_json)."""
    url = f"{_config['judge_url']}{path}"
    body_bytes = json.dumps(body).encode() if body else None

    req = urllib.request.Request(
        url,
        data=body_bytes,
        method=method,
        headers=headers,
    )
    if body_bytes:
        req.add_header("Content-Type", "application/json")
    # Pass Bearer token so Authelia validates and injects x-auth-* headers
    if token:
        req.add_header("Authorization", f"Bearer {token}")

    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            resp_body = resp.read()
            data = json.loads(resp_body) if resp_body else {}
            return resp.status, data
    except urllib.error.HTTPError as exc:
        body_text = ""
        try:
            body_text = json.loads(exc.read())
        except Exception:
            body_text = exc.read().decode()
        return exc.code, body_text


def build_auth_headers(payload: dict) -> dict:
    """Build the x-auth-* headers that JudgeService expects."""
    # sub might be a string or list (oidc4pp flow), or missing for client_credentials
    user = payload.get("sub")
    if isinstance(user, list):
        user = user[0] if user else ""
    user = user or payload.get("client_id", "unknown")

    scope = payload.get("scp", "")
    if isinstance(scope, list):
        scope = " ".join(scope)
    elif scope:
        scope = ",".join(scope)

    groups = payload.get("groups", "")
    if isinstance(groups, list):
        groups = " ".join(groups)
    elif groups:
        groups = ",".join(groups)

    email = payload.get("email", "")

    # Combine scope + groups
    scope_value = " ".join(filter(None, [scope, groups]))

    return {
        "x-auth-user": user,
        "x-auth-scope": scope_value,
    } | ({"x-auth-email": email} if email else {})


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------

def cmd_list(args, headers: dict, token: str):
    """GET /shenanigans"""
    print(f"  → GET {_config['judge_url']}/shenanigans")
    code, data = make_request("GET", "/shenanigans", headers, token)
    return print_response(code, data)


def cmd_get(args, headers: dict, token: str):
    """GET /shenanigans/{id}"""
    print(f"  → GET {_config['judge_url']}/shenanigans/{args.id}")
    code, data = make_request("GET", f"/shenanigans/{args.id}", headers, token)
    return print_response(code, data)


def cmd_create(args, headers: dict, token: str):
    """POST /shenanigans"""
    payload = {
        "name": args.name,
        "description": args.description,
        "rcon_payload": args.rcon_payload,
        "target_type": args.target_type,
    }
    if args.cost:
        payload["cost"] = args.cost
    if args.metadata:
        payload["metadata"] = json.loads(args.metadata)

    print(f"  → POST {_config['judge_url']}/shenanigans")
    print(f"     body: {json.dumps(payload, indent=2)}")
    code, data = make_request("POST", "/shenanigans", headers, token, payload)
    return print_response(code, data)


def cmd_activate(args, headers: dict, token: str):
    """POST /shenanigans/{id}/activate"""
    payload = {"team": args.team}
    if args.metadata:
        payload["metadata"] = json.loads(args.metadata)

    print(f"  → POST {_config['judge_url']}/shenanigans/{args.id}/activate")
    print(f"     body: {json.dumps(payload, indent=2)}")
    code, data = make_request("POST", f"/shenanigans/{args.id}/activate", headers, token, payload)
    return print_response(code, data)


def cmd_delete(args, headers: dict, token: str):
    """DELETE /shenanigans/{id}"""
    print(f"  → DELETE {_config['judge_url']}/shenanigans/{args.id}")
    code, data = make_request("DELETE", f"/shenanigans/{args.id}", headers, token)
    return print_response(code, None, stdout=False)


def cmd_info(args, headers: dict, token: str):
    """GET / (service info + m2m_connection details)"""
    print(f"  → GET {_config['judge_url']}/")
    code, data = make_request("GET", "/", headers, token)
    return print_response(code, data)


def cmd_health(args, headers: dict, token: str):
    """GET /health"""
    print(f"  → GET {_config['judge_url']}/health")
    code, data = make_request("GET", "/health", headers, token)
    return print_response(code, data)


def print_response(code: int, data: dict | None, stdout: bool = True) -> bool:
    """Pretty-print a response. Returns True on success."""
    status_str = "OK" if 200 <= code < 400 else "FAIL"
    marker = f"[{status_str}] {code}"
    if stdout:
        print(f"  ← {marker}")
    return 200 <= code < 400


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="Test the JudgeService Shenanigan API with M2M auth.",
    )
    parser.add_argument("--client-id", default="judge-client")
    parser.add_argument("--client-secret", default="foobar")
    parser.add_argument("--auth-server", default=AUTH_SERVER, help=f"Default: {AUTH_SERVER}")
    parser.add_argument("--judge-url", default=_config["judge_url"], help=f"Default: {_config['judge_url']}")

    sub = parser.add_subparsers(dest="command", required=True)

    # GET / (info)
    sub.add_parser("info", help="Show service info")

    # GET /health
    sub.add_parser("health", help="Health check")

    # List shenanigans
    list_p = sub.add_parser("list", help="GET /shenanigans")

    # Create shenanigan
    create_p = sub.add_parser("create", help="POST /shenanigans")
    create_p.add_argument("--name", required=True)
    create_p.add_argument("--description", required=True)
    create_p.add_argument("--rcon-payload", required=True, dest="rcon_payload")
    create_p.add_argument("--target-type", required=True, dest="target_type", choices=["team", "all"])
    create_p.add_argument("--cost", type=int)
    create_p.add_argument("--metadata", type=str, help="JSON string")

    # Get by ID
    get_p = sub.add_parser("get", help="GET /shenanigans/{id}")
    get_p.add_argument("id")

    # Activate
    act_p = sub.add_parser("activate", help="POST /shenanigans/{id}/activate")
    act_p.add_argument("id")
    act_p.add_argument("--team", required=True)
    act_p.add_argument("--metadata", type=str, help="JSON string")

    # Delete
    del_p = sub.add_parser("delete", help="DELETE /shenanigans/{id}")
    del_p.add_argument("id")

    args = parser.parse_args()

    _config["judge_url"] = args.judge_url

    # --- Auth ---
    print(f"[*] Fetching OIDC token from {args.auth_server} ...")
    token = fetch_token(args.client_id, args.client_secret)
    payload = decode_jwt_claims(token)

    print(f"[*] Token claims — sub={payload.get('sub', 'N/A')}, scp={payload.get('scp', 'N/A')}")
    headers = build_auth_headers(payload)
    print(f"[*] Auth headers: {json.dumps(headers)}")
    print()

    # --- Dispatch ---
    dispatch = {
        "info":       cmd_info,
        "health":     cmd_health,
        "list":       cmd_list,
        "get":        cmd_get,
        "create":     cmd_create,
        "activate":   cmd_activate,
        "delete":     cmd_delete,
    }

    success = dispatch[args.command](args, headers, token)
    print()
    if not success:
        print(f"[RESULT] Test failed (HTTP {sys.stdin is not None})")
        sys.exit(1)
    print("[RESULT] OK")


if __name__ == "__main__":
    main()
