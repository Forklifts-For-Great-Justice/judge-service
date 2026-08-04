#!/usr/bin/env python3
"""
Test script for the JudgeService Challenge API.

Auth flow (M2M client_credentials):
  1. POST https://auth.hackfortress.net/api/oidc/token  (obtain bearer token)
  2. Decode JWT -> extract sub + scp (scope) claims
  3. GET/POST/PUT/DELETE https://judge.hackfortress.net/challenges  (with auth headers)

Challenge model fields:
  - id (int)
  - name (string, required, 1-256 chars)
  - description (string, required)
  - challenge_type (string, optional, nullable)
  - location (string, optional, nullable)
  - points (int, required, > 0)
  - disabled (bool, optional, defaults to false)
  - flag (string, required)
  - created_at (time)
  - updated_at (time)

Soft-delete sets disabled = true (GetByID still returns it; GetAll excludes disabled).

Usage:
  python3 test_challenges.py list
  python3 test_challenges.py create --name "My Challenge" --description "A test" --points 100 --flag "FLAG{test}"
  python3 test_challenges.py get <id>
  python3 test_challenges.py update <id> --name "Updated" --points 200
  python3 test_challenges.py delete <id>
  python3 test_challenges.py full-cycle
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

_config = {"judge_url": "https://judge.hackfortress.net"}


def make_request(method: str, path: str, headers: dict, token: str | None = None, body: dict | None = None) -> tuple:
    """Send an HTTP request to JudgeService and return (status_code, parsed_data)."""
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

    scope_value = " ".join(filter(None, [scope, groups]))

    return {
        "x-auth-user": user,
        "x-auth-scope": scope_value,
    } | ({"x-auth-email": email} if email else {})


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------

def cmd_list(args, headers: dict, token: str):
    """GET /challenges"""
    print(f"  -> GET {_config['judge_url']}/challenges")
    code, data = make_request("GET", "/challenges", headers, token)
    if print_response(code, data):
        challenges = data.get("challenges", [])
        print(f"     Total challenges: {len(challenges)}")
        for c in challenges:
            print(f"       - id={c.get('id')}, name={c.get('name')}, points={c.get('points')}, "
                  f"type={c.get('challenge_type')}, location={c.get('location')}")
    return print_response(code, data)


def cmd_get(args, headers: dict, token: str):
    """GET /challenges/{id}"""
    print(f"  -> GET {_config['judge_url']}/challenges/{args.id}")
    code, data = make_request("GET", f"/challenges/{args.id}", headers, token)
    return print_response(code, data)


def cmd_create(args, headers: dict, token: str):
    """POST /challenges"""
    payload = {
        "name": args.name,
        "description": args.description,
        "points": args.points,
        "flag": args.flag,
    }
    if hasattr(args, "challenge_type") and args.challenge_type:
        payload["challenge_type"] = args.challenge_type
    if hasattr(args, "location") and args.location:
        payload["location"] = args.location
    if hasattr(args, "disabled") and args.disabled is not None:
        payload["disabled"] = args.disabled

    print(f"  -> POST {_config['judge_url']}/challenges")
    print(f"     body: {json.dumps(payload, indent=2)}")
    code, data = make_request("POST", "/challenges", headers, token, payload)
    return print_response(code, data)


def cmd_update(args, headers: dict, token: str):
    """PUT /challenges/{id}"""
    payload = {}
    if hasattr(args, "name") and args.name:
        payload["name"] = args.name
    if hasattr(args, "description") and args.description:
        payload["description"] = args.description
    if hasattr(args, "points") and args.points is not None:
        payload["points"] = args.points
    if hasattr(args, "challenge_type") and args.challenge_type:
        payload["challenge_type"] = args.challenge_type
    if hasattr(args, "location") and args.location:
        payload["location"] = args.location
    if hasattr(args, "disabled") and args.disabled is not None:
        payload["disabled"] = args.disabled
    if hasattr(args, "flag") and args.flag:
        payload["flag"] = args.flag

    if not payload:
        print("ERROR: update requires at least one of --name, --description, --points, "
              "--challenge-type, --location, --disabled, --flag", file=sys.stderr)
        sys.exit(1)

    print(f"  -> PUT {_config['judge_url']}/challenges/{args.id}")
    print(f"     body: {json.dumps(payload, indent=2)}")
    code, data = make_request("PUT", f"/challenges/{args.id}", headers, token, payload)
    return print_response(code, data)


def cmd_delete(args, headers: dict, token: str):
    """DELETE /challenges/{id}"""
    print(f"  -> DELETE {_config['judge_url']}/challenges/{args.id}")
    code, data = make_request("DELETE", f"/challenges/{args.id}", headers, token)
    return print_response(code, None, stdout=False)


def cmd_full_cycle(args, headers: dict, token: str):
    """Full CRUD cycle: create, get, update, verify, delete, confirm."""
    passed = 0
    failed = 0

    def expect_ok(code, label):
        nonlocal passed, failed
        if 200 <= code < 400:
            passed += 1
            print(f"     [OK] {label} (HTTP {code})")
        else:
            failed += 1
            print(f"     [FAIL] {label} (HTTP {code})")

    print("[CRUD Cycle] Create")
    create_payload = {
        "name": args.name,
        "description": args.description,
        "points": args.points,
        "flag": args.flag,
    }
    if args.challenge_type:
        create_payload["challenge_type"] = args.challenge_type
    if args.location:
        create_payload["location"] = args.location

    print(f"  -> POST /challenges")
    print(f"     body: {json.dumps(create_payload, indent=2)}")
    code, data = make_request("POST", "/challenges", headers, token, create_payload)
    if code == 201:
        challenge = data.get("challenge", data)
        challenge_id = challenge.get("id")
        expect_ok(code, "Create challenge")
    else:
        print_response(code, data)
        expect_ok(code, "Create challenge")
        sys.exit(1)

    print()
    print("[CRUD Cycle] Read")
    print(f"  -> GET /challenges/{challenge_id}")
    code, data = make_request("GET", f"/challenges/{challenge_id}", headers, token)
    challenge = data.get("challenge", data)
    expect_ok(code, "Get challenge by ID")
    created_name = challenge.get("name", "")
    created_description = challenge.get("description", "")
    created_points = challenge.get("points", 0)
    created_flag = challenge.get("flag", "")

    print()
    print("[CRUD Cycle] Update")
    updated_name = f"{args.name} (Updated)"
    updated_points = args.points + 50
    update_payload = {
        "name": updated_name,
        "points": updated_points,
    }
    print(f"  -> PUT /challenges/{challenge_id}")
    print(f"     body: {json.dumps(update_payload, indent=2)}")
    code, data = make_request("PUT", f"/challenges/{challenge_id}", headers, token, update_payload)
    expect_ok(code, "Update challenge")
    if code == 200:
        updated = data.get("challenge", data)
        actual_name = updated.get("name", "")
        actual_points = updated.get("points", 0)
        print(f"     Verified: name={actual_name} (expected={updated_name}), "
              f"points={actual_points} (expected={updated_points})")
        if actual_name == updated_name:
            passed += 1
            print(f"     [OK] name verified")
        else:
            failed += 1
            print(f"     [FAIL] name mismatch: got '{actual_name}' expected '{updated_name}'")
        if actual_points == updated_points:
            passed += 1
            print(f"     [OK] points verified")
        else:
            failed += 1
            print(f"     [FAIL] points mismatch: got {actual_points} expected {updated_points}")

    print()
    print("[CRUD Cycle] Delete")
    print(f"  -> DELETE /challenges/{challenge_id}")
    code, data = make_request("DELETE", f"/challenges/{challenge_id}", headers, token)
    expect_ok(code, "Delete challenge")

    print()
    print("[CRUD Cycle] Verify Disabled (GetByID — not excluded by soft-delete)")
    print(f"  -> GET /challenges/{challenge_id}")
    code, data = make_request("GET", f"/challenges/{challenge_id}", headers, token)
    if code == 200:
        disabled_obj = data.get("challenge", data).get("disabled", False)
        if disabled_obj is True:
            passed += 1
            print(f"     [OK] challenge confirmed disabled after soft-delete")
        else:
            failed += 1
            print(f"     [FAIL] expected disabled=true, got disabled={disabled_obj}")
    else:
        failed += 1
        print(f"     [FAIL] GET by ID after soft-delete returned HTTP {code} (expected 200)")
        print_response(code, data)

    print()
    print("[CRUD Cycle] Confirm Excluded from List")
    print(f"  -> GET /challenges")
    code, data = make_request("GET", "/challenges", headers, token)
    expect_ok(code, "List challenges (post-delete)")
    if code == 200:
        challenges = data.get("challenges", [])
        remaining_ids = [c.get("id") for c in challenges]
        if challenge_id not in remaining_ids:
            passed += 1
            print(f"     [OK] Challenge {challenge_id} confirmed excluded from GET /challenges")
        else:
            failed += 1
            print(f"     [FAIL] Challenge {challenge_id} still found in list")

    print()
    print("[CRUD Cycle] Create Invalid Challenge (empty name)")
    invalid_payload = {
        "name": "",
        "description": "Has description but empty name",
        "points": 100,
        "flag": "FLAG{test}",
    }
    print(f"  -> POST /challenges (expected 400)")
    print(f"     body: {json.dumps(invalid_payload, indent=2)}")
    code, data = make_request("POST", "/challenges", headers, token, invalid_payload)
    if code == 400:
        passed += 1
        print(f"     [OK] Empty name rejected with HTTP 400")
    else:
        failed += 1
        print(f"     [FAIL] Expected HTTP 400 for empty name, got {code}")

    print()
    print("[CRUD Cycle] Create Invalid Challenge (points <= 0)")
    invalid_payload2 = {
        "name": "Valid Name",
        "description": "Valid description",
        "points": 0,
        "flag": "FLAG{test}",
    }
    print(f"  -> POST /challenges (expected 400)")
    print(f"     body: {json.dumps(invalid_payload2, indent=2)}")
    code, data = make_request("POST", "/challenges", headers, token, invalid_payload2)
    if code == 400:
        passed += 1
        print(f"     [OK] Zero points rejected with HTTP 400")
    else:
        failed += 1
        print(f"     [FAIL] Expected HTTP 400 for zero points, got {code}")

    print()
    print("[CRUD Cycle] Update with No Fields")
    empty_update = {}
    print(f"  -> PUT /challenges/{challenge_id} (expected 400)")
    code, data = make_request("PUT", f"/challenges/{challenge_id}", headers, token, empty_update)
    if code == 400:
        passed += 1
        print(f"     [OK] Empty update rejected with HTTP 400")
    else:
        failed += 1
        print(f"     [FAIL] Expected HTTP 400 for empty update, got {code}")

    print()
    print(f"[RESULT] {passed} passed, {failed} failed out of {passed + failed} checks")
    return failed == 0


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def print_response(code: int, data, stdout: bool = True) -> bool:
    """Pretty-print a response. Returns True on success."""
    status_str = "OK" if 200 <= code < 400 else "FAIL"
    marker = f"[{status_str}] {code}"
    if stdout:
        print(f"  <- {marker}")
        if data:
            print(f"     {json.dumps(data, indent=2)}")
    return 200 <= code < 400


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="Test the JudgeService Challenge API with M2M auth.",
    )
    parser.add_argument("--client-id", default="judge-client")
    parser.add_argument("--client-secret", default="foobar")
    parser.add_argument("--auth-server", default=AUTH_SERVER, help=f"Default: {AUTH_SERVER}")
    parser.add_argument("--judge-url", default=_config["judge_url"], help=f"Default: {_config['judge_url']}")

    sub = parser.add_subparsers(dest="command", required=True)

    # List challenges
    list_p = sub.add_parser("list", help="GET /challenges")

    # Create challenge
    create_p = sub.add_parser("create", help="POST /challenges")
    create_p.add_argument("--name", required=True, help="Challenge name")
    create_p.add_argument("--description", required=True, help="Challenge description")
    create_p.add_argument("--points", required=True, type=int, help="Points for solving")
    create_p.add_argument("--flag", required=True, help="CFTF flag")
    create_p.add_argument("--challenge-type", dest="challenge_type", help="Type (e.g. 'binary', 'forensics')")
    create_p.add_argument("--location", help="Location on the CTFd platform")
    create_p.add_argument("--disabled", type=lambda x: x.lower() in ("true", "1", "yes"),
                         help="Set disabled flag")

    # Get challenge by ID
    get_p = sub.add_parser("get", help="GET /challenges/{id}")
    get_p.add_argument("id", help="Challenge ID")

    # Update challenge
    update_p = sub.add_parser("update", help="PUT /challenges/{id}")
    update_p.add_argument("id", help="Challenge ID")
    update_p.add_argument("--name", help="New name")
    update_p.add_argument("--description", help="New description")
    update_p.add_argument("--points", type=int, help="New points value")
    update_p.add_argument("--challenge-type", dest="challenge_type", help="New challenge type")
    update_p.add_argument("--location", help="New location")
    update_p.add_argument("--disabled", type=lambda x: x.lower() in ("true", "1", "yes"),
                         help="New disabled value")
    update_p.add_argument("--flag", help="New flag")

    # Delete challenge
    del_p = sub.add_parser("delete", help="DELETE /challenges/{id}")
    del_p.add_argument("id", help="Challenge ID")

    # Full cycle
    cycle_p = sub.add_parser("full-cycle", help="Full CRUD cycle")
    cycle_p.add_argument("--name", default="Cycle-Challenge", help="Challenge name for cycle")
    cycle_p.add_argument("--description", default="Auto-generated cycle test challenge",
                         help="Description for cycle")
    cycle_p.add_argument("--points", type=int, default=100, help="Points for cycle")
    cycle_p.add_argument("--flag", default="FLAG{cycle-test}", help="Flag for cycle")
    cycle_p.add_argument("--challenge-type", dest="challenge_type", help="Optional type")
    cycle_p.add_argument("--location", help="Optional location")

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
        "list":         cmd_list,
        "create":       cmd_create,
        "get":          cmd_get,
        "update":       cmd_update,
        "delete":       cmd_delete,
        "full-cycle":   cmd_full_cycle,
    }

    success = dispatch[args.command](args, headers, token)
    print()
    if not success:
        print("[RESULT] FAILED")
        sys.exit(1)
    print("[RESULT] OK")


if __name__ == "__main__":
    main()
