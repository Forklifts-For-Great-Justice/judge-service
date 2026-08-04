#!/usr/bin/env python3
"""
Test script for the JudgeService Round API.

Auth flow (M2M client_credentials):
  1. POST https://auth.hackfortress.net/api/oidc/token  (obtain bearer token)
  2. Decode JWT -> extract sub + scp (scope) claims
  3. GET/POST/PUT/DELETE https://judge.hackfortress.net:8086/rounds  (with x-auth headers)

Usage:
  python3 test_rounds.py list
  python3 test_rounds.py get <id>
  python3 test_rounds.py create --name "Opening Match" --team-a 1 --team-b 2
  python3 test_rounds.py update <id> --name "Updated Match"
  python3 test_rounds.py toggle-ready <id>
  python3 test_rounds.py toggle-live <id>
  python3 test_rounds.py full-cycle

Requirements:
  - judge-client OIDC client with "judge" scope (default creds in test_teams.py style: judge-client/foobar)
  - JudgeService deployed and reachable at judge.hackfortress.net:8086
  - Database connected (round routes require DB)
"""

import argparse
import json
import sys
import urllib.error
import urllib.parse
import urllib.request


# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

AUTH_SERVER = "https://auth.hackfortress.net"
TOKEN_ENDPOINT = f"{AUTH_SERVER}/api/oidc/token"
JUDGE_URL = "http://judge.hackfortress.net:8086"


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
    from base64 import b64decode

    parts = token.split(".")
    if len(parts) != 3:
        print("ERROR: access token is not a valid JWT (expected 3 parts)", file=sys.stderr)
        sys.exit(1)

    payload_b64 = parts[1] + "=" * (4 - len(parts[1]) % 4)
    payload = json.loads(b64decode(payload_b64))
    return payload


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
# JudgeService API
# ---------------------------------------------------------------------------

_config = {"judge_url": JUDGE_URL}


def make_request(method: str, path: str, headers: dict, body: dict | None = None) -> tuple:
    """Send an HTTP request to JudgeService and return (status_code, parsed_or_raw_data)."""
    url = f"{_config['judge_url']}{path}"
    body_bytes = json.dumps(body).encode() if body else None

    req = urllib.request.Request(url, data=body_bytes, method=method, headers=headers)
    if body_bytes:
        req.add_header("Content-Type", "application/json")

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


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------

def cmd_list(args, headers: dict, token: str):
    """GET /rounds"""
    print(f"  -> GET {_config['judge_url']}/rounds")
    code, data = make_request("GET", "/rounds", headers)
    if code == 200:
        rounds = data.get("rounds", [])
        print(f"     Total rounds: {len(rounds)}")
        for r in rounds:
            print(f"       - id={r.get('id')}, name={r.get('round_name')}, status={r.get('status')}")
    return print_response(code, data)


def cmd_get(args, headers: dict, token: str):
    """GET /rounds/{id}"""
    print(f"  -> GET {_config['judge_url']}/rounds/{args.id}")
    code, data = make_request("GET", f"/rounds/{args.id}", headers)
    return print_response(code, data)


def cmd_create(args, headers: dict, token: str):
    """POST /rounds"""
    payload = {
        "round_name": args.name,
        "team_a_id": args.team_a,
        "team_b_id": args.team_b,
    }

    print(f"  -> POST {_config['judge_url']}/rounds")
    print(f"     body: {json.dumps(payload, indent=2)}")
    code, data = make_request("POST", "/rounds", headers, payload)
    return print_response(code, data)


def cmd_update(args, headers: dict, token: str):
    """PUT /rounds/{id}"""
    payload = {}
    if hasattr(args, "name") and args.name:
        payload["round_name"] = args.name
    if hasattr(args, "team-a") and args.team_a:
        payload["team_a_id"] = args.team_a
    if hasattr(args, "team-b") and args.team_b:
        payload["team_b_id"] = args.team_b

    if not payload:
        print("ERROR: update requires at least --name, --team-a, or --team-b", file=sys.stderr)
        sys.exit(1)

    print(f"  -> PUT {_config['judge_url']}/rounds/{args.id}")
    print(f"     body: {json.dumps(payload, indent=2)}")
    code, data = make_request("PUT", f"/rounds/{args.id}", headers, payload)
    return print_response(code, data)


def cmd_delete(args, headers: dict, token: str):
    """DELETE /rounds/{id} (soft-delete)"""
    print(f"  -> DELETE {_config['judge_url']}/rounds/{args.id}")
    code, data = make_request("DELETE", f"/rounds/{args.id}", headers)
    return print_response(code, data)


def cmd_toggle_ready(args, headers: dict, token: str):
    """POST /rounds/{id}/ready"""
    print(f"  -> POST {_config['judge_url']}/rounds/{args.id}/ready")
    code, data = make_request("POST", f"/rounds/{args.id}/ready", headers)
    return print_response(code, data)


def cmd_toggle_live(args, headers: dict, token: str):
    """POST /rounds/{id}/live"""
    print(f"  -> POST {_config['judge_url']}/rounds/{args.id}/live")
    code, data = make_request("POST", f"/rounds/{args.id}/live", headers)
    return print_response(code, data)


def cmd_full_cycle(args, headers: dict, token: str):
    """Full CRUD + toggle cycle: create, get, update, verify, toggle-ready, toggle-live, delete, confirm."""
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

    # Step 1: Create
    print("[Cycle] Create")
    team_a = args.team_a
    team_b = args.team_b
    create_payload = {
        "round_name": args.name,
        "team_a_id": team_a,
        "team_b_id": team_b,
    }
    print(f"  -> POST /rounds")
    print(f"     body: {json.dumps(create_payload, indent=2)}")
    code, data = make_request("POST", "/rounds", headers, create_payload)
    if code == 201:
        round_id = data.get("id") or data.get("round", {}).get("id")
        expect_ok(code, "Create round")
    else:
        print_response(code, data)
        expect_ok(code, "Create round")
        return False

    print()
    # Step 2: Read
    print("[Cycle] Read")
    print(f"  -> GET /rounds/{round_id}")
    code, data = make_request("GET", f"/rounds/{round_id}", headers)
    round_ = data.get("round", data)
    expect_ok(code, "Get round by ID")
    created_name = round_.get("round_name", "")
    print(f"     round_name={created_name}, status={round_.get('status', 'N/A')}")

    print()
    # Step 3: Update
    print("[Cycle] Update")
    updated_name = f"{args.name}-Updated"
    update_payload = {"round_name": updated_name}
    print(f"  -> PUT /rounds/{round_id}")
    print(f"     body: {json.dumps(update_payload, indent=2)}")
    code, data = make_request("PUT", f"/rounds/{round_id}", headers, update_payload)
    expect_ok(code, "Update round")
    if code == 200:
        round_ = data.get("round", data)
        actual_name = round_.get("round_name", "")
        if actual_name == updated_name:
            passed += 1
            print(f"     [OK] name verified: {actual_name}")
        else:
            failed += 1
            print(f"     [FAIL] name mismatch: got '{actual_name}' expected '{updated_name}'")

    print()
    # Step 4: Toggle Ready
    print("[Cycle] Toggle Ready")
    print(f"  -> POST /rounds/{round_id}/ready")
    code, data = make_request("POST", f"/rounds/{round_id}/ready", headers)
    expect_ok(code, "Toggle ready ON")
    if code == 200:
        ready = data.get("ready")
        status = data.get("status")
        if ready is True:
            passed += 1
            print(f"     [OK] ready={ready}, status={status}")
        else:
            failed += 1
            print(f"     [FAIL] expected ready=true, got {ready}")

    print()
    # Step 5: Toggle Live
    print("[Cycle] Toggle Live")
    print(f"  -> POST /rounds/{round_id}/live")
    code, data = make_request("POST", f"/rounds/{round_id}/live", headers)
    expect_ok(code, "Toggle live ON")
    if code == 200:
        live = data.get("live")
        status = data.get("status")
        if live is True and status == "in_progress":
            passed += 1
            print(f"     [OK] live={live}, status={status}")
        else:
            failed += 1
            print(f"     [FAIL] expected live=true, status=in_progress, got live={live}, status={status}")

    print()
    # Step 6: Verify via List
    print("[Cycle] Verify via List")
    print(f"  -> GET /rounds")
    code, data = make_request("GET", "/rounds", headers)
    expect_ok(code, "List rounds")
    if code == 200:
        rounds = data.get("rounds", [])
        found = any(r.get("id") == round_id for r in rounds)
        if found:
            passed += 1
            print(f"     [OK] Round {round_id} found in list")
        else:
            failed += 1
            print(f"     [FAIL] Round {round_id} NOT found in list")

    print()
    # Step 7: Delete
    print("[Cycle] Delete")
    print(f"  -> DELETE /rounds/{round_id}")
    code, data = make_request("DELETE", f"/rounds/{round_id}", headers)
    expect_ok(code, "Delete round (soft-delete)")
    if code == 200:
        round_ = data.get("round", data)
        if round_.get("disabled") is True:
            passed += 1
            print(f"     [OK] Round confirmed disabled")
        else:
            failed += 1
            print(f"     [FAIL] expected disabled=true, got {round_.get('disabled')}")

    print()
    # Step 8: Confirm deletion via List
    print("[Cycle] Confirm Deletion via List")
    print(f"  -> GET /rounds")
    code, data = make_request("GET", "/rounds", headers)
    expect_ok(code, "List rounds (post-delete)")
    if code == 200:
        rounds = data.get("rounds", [])
        remaining_ids = [r.get("id") for r in rounds]
        if round_id not in remaining_ids:
            passed += 1
            print(f"     [OK] Round {round_id} confirmed deleted (not in list)")
        else:
            failed += 1
            print(f"     [FAIL] Round {round_id} still found in list")

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
        description="Test the JudgeService Round API with M2M auth.",
    )
    parser.add_argument("--client-id", default="judge-client")
    parser.add_argument("--client-secret", default="foobar")
    parser.add_argument("--auth-server", default=AUTH_SERVER, help=f"Default: {AUTH_SERVER}")
    parser.add_argument("--judge-url", default=_config["judge_url"], help=f"Default: {_config['judge_url']}")

    sub = parser.add_subparsers(dest="command", required=True)

    # List rounds
    sub.add_parser("list", help="GET /rounds")

    # Get round by ID
    get_p = sub.add_parser("get", help="GET /rounds/{id}")
    get_p.add_argument("id", help="Round ID")

    # Create round
    create_p = sub.add_parser("create", help="POST /rounds")
    create_p.add_argument("--name", required=True, help="Round name")
    create_p.add_argument("--team-a", type=int, default=1, help="Team A ID (default: 1)")
    create_p.add_argument("--team-b", type=int, default=2, help="Team B ID (default: 2)")

    # Update round
    update_p = sub.add_parser("update", help="PUT /rounds/{id}")
    update_p.add_argument("id", help="Round ID")
    update_p.add_argument("--name", help="New round name")

    # Delete round
    del_p = sub.add_parser("delete", help="DELETE /rounds/{id}")
    del_p.add_argument("id", help="Round ID")

    # Toggle ready
    ready_p = sub.add_parser("toggle-ready", help="POST /rounds/{id}/ready")
    ready_p.add_argument("id", help="Round ID")

    # Toggle live
    live_p = sub.add_parser("toggle-live", help="POST /rounds/{id}/live")
    live_p.add_argument("id", help="Round ID")

    # Full cycle
    cycle_p = sub.add_parser("full-cycle", help="Full CRUD + toggle cycle")
    cycle_p.add_argument("--name", default="Cycle-Round", help="Round name for cycle")
    cycle_p.add_argument("--team-a", type=int, default=1, help="Team A ID (default: 1)")
    cycle_p.add_argument("--team-b", type=int, default=2, help="Team B ID (default: 2)")

    args = parser.parse_args()

    _config["judge_url"] = args.judge_url

    # --- Auth ---
    print(f"[*] Fetching OIDC token from {args.auth_server} ...")
    token = fetch_token(args.client_id, args.client_secret)
    payload = decode_jwt_claims(token)

    print(f"[*] Token claims - sub={payload.get('sub', 'N/A')}, scp={payload.get('scp', 'N/A')}")
    headers = build_auth_headers(payload)
    print(f"[*] Auth headers: {json.dumps(headers)}")
    print()

    # --- Dispatch ---
    dispatch = {
        "list": cmd_list,
        "get": cmd_get,
        "create": cmd_create,
        "update": cmd_update,
        "delete": cmd_delete,
        "toggle-ready": cmd_toggle_ready,
        "toggle-live": cmd_toggle_live,
        "full-cycle": cmd_full_cycle,
    }

    success = dispatch[args.command](args, headers, token)
    print()
    if not success:
        print("[RESULT] FAILED")
        sys.exit(1)
    print("[RESULT] OK")


if __name__ == "__main__":
    main()
