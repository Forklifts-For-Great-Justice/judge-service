#!/usr/bin/env python3
"""
Test script for the JudgeService Player API.

Endpoints tested:
  - GET  /player/challenges
  - POST /player/challenges/submit
  - GET  /player/shenanigans
  - POST /player/shenanigans/buy

Usage:
  python3 test_player.py list-challenges [--team-id 1]
  python3 test_player.py submit-challenge --challenge-id 1 --flag "FLAG{test}" [--team-id 1] [--player-id player1]
  python3 test_player.py list-shenanigans
  python3 test_player.py buy-shenanigan --shenanigan-id 1 [--team-id 1] [--buyer-id player1]
  python3 test_player.py full-cycle [--team-id 1] [--player-id player1]
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
_config = {"judge_url": "https://judge.hackfortress.net"}


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


def decode_jwt_claims(token: str) -> dict:
    """Decode the payload of a JWT (header + payload) without verifying the signature."""
    parts = token.split(".")
    if len(parts) != 3:
        print("ERROR: access token is not a valid JWT (expected 3 parts)", file=sys.stderr)
        sys.exit(1)

    payload_b64 = parts[1] + "=" * (4 - (len(parts[1]) % 4))
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
# API Request Helper
# ---------------------------------------------------------------------------

def make_request(
    method: str,
    path: str,
    headers: dict | None = None,
    token: str | None = None,
    body: dict | None = None,
) -> tuple[int, dict]:
    """Send an HTTP request to JudgeService and return (status_code, parsed_data)."""
    url = f"{_config['judge_url']}{path}"
    body_bytes = json.dumps(body).encode() if body else None

    req_headers = dict(headers or {})
    if body_bytes:
        req_headers["Content-Type"] = "application/json"
    if token:
        req_headers["Authorization"] = f"Bearer {token}"

    req = urllib.request.Request(
        url,
        data=body_bytes,
        method=method,
        headers=req_headers,
    )

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


def print_response(code: int, data: dict | str, stdout: bool = True) -> bool:
    """Pretty-print a response. Returns True on success."""
    is_error = False
    if isinstance(data, dict) and "error" in data:
        is_error = True

    status_str = "OK" if (200 <= code < 400 and not is_error) else "FAIL/ERROR"
    marker = f"[{status_str}] HTTP {code}"
    if stdout:
        print(f"  <- {marker}")
        if data:
            print(f"     {json.dumps(data, indent=2) if isinstance(data, dict) else data}")
    return 200 <= code < 400 and not is_error


# ---------------------------------------------------------------------------
# Command Handlers
# ---------------------------------------------------------------------------

def cmd_list_challenges(args, headers: dict, token: str):
    """GET /player/challenges"""
    req_headers = dict(headers)
    path = "/player/challenges"
    if hasattr(args, "team_id") and args.team_id:
        req_headers["x-team-id"] = str(args.team_id)
        path += f"?team_id={args.team_id}"

    print(f"  -> GET {_config['judge_url']}{path}")
    code, data = make_request("GET", path, headers=req_headers, token=token)
    if print_response(code, data):
        challenges = data.get("challenges", [])
        print(f"     Available challenges: {len(challenges)}")
        for c in challenges:
            print(f"       - id={c.get('id')}, name={c.get('name')}, points={c.get('points')}, solved={c.get('solved')}")
        return True
    return False


def cmd_submit_challenge(args, headers: dict, token: str):
    """POST /player/challenges/submit"""
    payload = {
        "challenge_id": args.challenge_id,
        "flag": args.flag,
    }
    if hasattr(args, "team_id") and args.team_id:
        payload["team_id"] = args.team_id
    if hasattr(args, "player_id") and args.player_id:
        payload["player_id"] = args.player_id

    req_headers = dict(headers)
    if hasattr(args, "user") and args.user:
        req_headers["x-auth-user"] = args.user

    print(f"  -> POST {_config['judge_url']}/player/challenges/submit")
    print(f"     body: {json.dumps(payload, indent=2)}")
    code, data = make_request("POST", "/player/challenges/submit", headers=req_headers, token=token, body=payload)
    return print_response(code, data)


def cmd_list_shenanigans(args, headers: dict, token: str):
    """GET /player/shenanigans"""
    print(f"  -> GET {_config['judge_url']}/player/shenanigans")
    code, data = make_request("GET", "/player/shenanigans", headers=headers, token=token)
    if print_response(code, data):
        shenanigans = data.get("shenanigans", [])
        print(f"     Available shenanigans: {len(shenanigans)}")
        for s in shenanigans:
            print(f"       - id={s.get('id')}, name={s.get('name')}, price={s.get('price')}, target_type={s.get('target_type')}")
        return True
    return False


def cmd_buy_shenanigan(args, headers: dict, token: str):
    """POST /player/shenanigans/buy"""
    payload = {
        "shenanigan_id": args.shenanigan_id,
    }
    if hasattr(args, "team_id") and args.team_id:
        payload["team_id"] = args.team_id
    if hasattr(args, "buyer_id") and args.buyer_id:
        payload["buyer_id"] = args.buyer_id
    if hasattr(args, "target_team") and args.target_team:
        payload["target_team"] = args.target_team
    if hasattr(args, "metadata") and args.metadata:
        payload["metadata"] = json.loads(args.metadata)

    req_headers = dict(headers)
    if hasattr(args, "user") and args.user:
        req_headers["x-auth-user"] = args.user

    print(f"  -> POST {_config['judge_url']}/player/shenanigans/buy")
    print(f"     body: {json.dumps(payload, indent=2)}")
    code, data = make_request("POST", "/player/shenanigans/buy", headers=req_headers, token=token, body=payload)
    return print_response(code, data)


def cmd_full_cycle(args, headers: dict, token: str):
    """Run verification checks across all player routes."""
    passed = 0
    failed = 0

    def record_result(success: bool, label: str):
        nonlocal passed, failed
        if success:
            passed += 1
            print(f"     [OK] {label}")
        else:
            failed += 1
            print(f"     [FAIL] {label}")

    team_id = getattr(args, "team_id", 1) or 1
    player_id = getattr(args, "player_id", "player1") or "player1"

    print("=== 1. Test GET /player/challenges ===")
    code, data = make_request("GET", f"/player/challenges?team_id={team_id}", headers=headers, token=token)
    ok = print_response(code, data)
    record_result(ok, "List player challenges")

    print("\n=== 2. Test POST /player/challenges/submit (Missing Fields) ===")
    code, data = make_request("POST", "/player/challenges/submit", headers=headers, token=token, body={})
    is_handled_error = (code == 200 and isinstance(data, dict) and "error" in data) or code == 400
    print_response(code, data)
    record_result(is_handled_error, "Reject empty submission payload with JSON error")

    print("\n=== 3. Test POST /player/challenges/submit (Non-existent Challenge) ===")
    code, data = make_request("POST", "/player/challenges/submit", headers=headers, token=token, body={
        "challenge_id": 999999,
        "flag": "FLAG{non_existent}",
        "team_id": team_id,
        "player_id": player_id,
    })
    is_handled_error = isinstance(data, dict) and "error" in data
    print_response(code, data)
    record_result(is_handled_error, "Handled non-existent challenge error")

    print("\n=== 4. Test GET /player/shenanigans ===")
    code, data = make_request("GET", "/player/shenanigans", headers=headers, token=token)
    ok = print_response(code, data)
    record_result(ok, "List player shenanigans")

    print("\n=== 5. Test POST /player/shenanigans/buy (Missing Fields) ===")
    code, data = make_request("POST", "/player/shenanigans/buy", headers=headers, token=token, body={})
    is_handled_error = (code == 200 and isinstance(data, dict) and "error" in data) or code == 400
    print_response(code, data)
    record_result(is_handled_error, "Reject empty buy payload with JSON error")

    print("\n=== 6. Test POST /player/shenanigans/buy (Non-existent Shenanigan) ===")
    code, data = make_request("POST", "/player/shenanigans/buy", headers=headers, token=token, body={
        "shenanigan_id": 999999,
        "team_id": team_id,
        "buyer_id": player_id,
    })
    is_handled_error = isinstance(data, dict) and "error" in data
    print_response(code, data)
    record_result(is_handled_error, "Handled non-existent shenanigan error")

    print(f"\n[RESULT] {passed} passed, {failed} failed out of {passed + failed} checks")
    return failed == 0


# ---------------------------------------------------------------------------
# CLI Entrypoint
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="Test script for JudgeService Player API endpoints.",
    )
    parser.add_argument("--client-id", default="judge-client")
    parser.add_argument("--client-secret", default="foobar")
    parser.add_argument("--auth-server", default=AUTH_SERVER, help=f"Default: {AUTH_SERVER}")
    parser.add_argument("--judge-url", default=_config["judge_url"], help=f"Default: {_config['judge_url']}")

    sub = parser.add_subparsers(dest="command", required=True)

    # GET /player/challenges
    challenges_p = sub.add_parser("list-challenges", help="GET /player/challenges")
    challenges_p.add_argument("--team-id", type=int, help="Optional Team ID")

    # POST /player/challenges/submit
    submit_p = sub.add_parser("submit-challenge", help="POST /player/challenges/submit")
    submit_p.add_argument("--challenge-id", type=int, required=True, help="Challenge ID")
    submit_p.add_argument("--flag", required=True, help="Submitted flag string")
    submit_p.add_argument("--team-id", type=int, help="Optional Team ID")
    submit_p.add_argument("--player-id", help="Optional Player ID")
    submit_p.add_argument("--user", help="Optional x-auth-user header override")

    # GET /player/shenanigans
    shenanigans_p = sub.add_parser("list-shenanigans", help="GET /player/shenanigans")

    # POST /player/shenanigans/buy
    buy_p = sub.add_parser("buy-shenanigan", help="POST /player/shenanigans/buy")
    buy_p.add_argument("--shenanigan-id", type=int, required=True, help="Shenanigan ID")
    buy_p.add_argument("--team-id", type=int, help="Optional Team ID")
    buy_p.add_argument("--buyer-id", help="Optional Buyer ID")
    buy_p.add_argument("--target-team", help="Optional target team string")
    buy_p.add_argument("--metadata", help="Optional metadata JSON string")
    buy_p.add_argument("--user", help="Optional x-auth-user header override")

    # Full Cycle
    cycle_p = sub.add_parser("full-cycle", help="Run full test suite on player routes")
    cycle_p.add_argument("--team-id", type=int, default=1, help="Team ID for cycle tests")
    cycle_p.add_argument("--player-id", default="player1", help="Player ID for cycle tests")

    args = parser.parse_args()

    _config["judge_url"] = args.judge_url

    # --- Auth ---
    token = None
    headers = {}
    if args.auth_server:
        print(f"[*] Fetching OIDC token from {args.auth_server} ...")
        token = fetch_token(args.client_id, args.client_secret)
        payload = decode_jwt_claims(token)
        print(f"[*] Token claims — sub={payload.get('sub', 'N/A')}, scp={payload.get('scp', 'N/A')}")
        headers = build_auth_headers(payload)
        print(f"[*] Auth headers: {json.dumps(headers)}")
        print()

    dispatch = {
        "list-challenges": cmd_list_challenges,
        "submit-challenge": cmd_submit_challenge,
        "list-shenanigans": cmd_list_shenanigans,
        "buy-shenanigan": cmd_buy_shenanigan,
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
