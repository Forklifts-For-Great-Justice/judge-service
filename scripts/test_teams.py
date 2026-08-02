#!/usr/bin/env python3
"""
Test script for the JudgeService Team API.

Auth flow (M2M client_credentials):
  1. POST https://auth.hackfortress.net/api/oidc/token  (obtain bearer token)
  2. Decode JWT -> extract sub + scp (scope) claims
  3. GET/POST/PUT/DELETE https://judge.hackfortress.net/teams  (with auth headers)

Usage:
  python3 test_teams.py list
  python3 test_teams.py create --name "Red Team" --slug red-crew --alt-name "Red Crew" --clan-tag "RED"
  python3 test_teams.py get <id>
  python3 test_teams.py update <id> --name "Updated Team" --slug updated-crew --clan-tag "UPD"
  python3 test_teams.py delete <id>
  python3 test_teams.py full-cycle
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

# Module-level mutable container for the judge URL so cmd functions can read it.
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
    """GET /teams"""
    print(f"  -> GET {_config['judge_url']}/teams")
    code, data = make_request("GET", "/teams", headers, token)
    if print_response(code, data):
        teams = data.get("teams", [])
        print(f"     Total teams: {len(teams)}")
        for t in teams:
            print(f"       - id={t.get('id')}, name={t.get('name')}, slug={t.get('slug')}, clan_tag={t.get('clan_tag')}")
    return print_response(code, data)


def cmd_get(args, headers: dict, token: str):
    """GET /teams/{id}"""
    print(f"  -> GET {_config['judge_url']}/teams/{args.id}")
    code, data = make_request("GET", f"/teams/{args.id}", headers, token)
    return print_response(code, data)


def cmd_create(args, headers: dict, token: str):
    """POST /teams"""
    payload = {
        "slug": args.slug if hasattr(args, "slug") and args.slug else args.name.replace(" ", "-").lower(),
        "name": args.name,
        "alt_name": args.alt_name if hasattr(args, "alt_name") and args.alt_name else args.name + " Alt",
        "clan_tag": args.clan_tag if hasattr(args, "clan_tag") and args.clan_tag else args.name[:3].upper(),
    }

    print(f"  -> POST {_config['judge_url']}/teams")
    print(f"     body: {json.dumps(payload, indent=2)}")
    code, data = make_request("POST", "/teams", headers, token, payload)
    return print_response(code, data)


def cmd_update(args, headers: dict, token: str):
    """PUT /teams/{id}"""
    payload = {}
    if hasattr(args, "name") and args.name:
        payload["name"] = args.name
    if hasattr(args, "clan_tag") and args.clan_tag:
        payload["clan_tag"] = args.clan_tag
    if hasattr(args, "slug") and args.slug:
        payload["slug"] = args.slug
    if hasattr(args, "alt_name") and args.alt_name:
        payload["alt_name"] = args.alt_name

    if not payload:
        print("ERROR: update requires at least --name, --clan-tag, --slug, or --alt-name", file=sys.stderr)
        sys.exit(1)

    print(f"  -> PUT {_config['judge_url']}/teams/{args.id}")
    print(f"     body: {json.dumps(payload, indent=2)}")
    code, data = make_request("PUT", f"/teams/{args.id}", headers, token, payload)
    return print_response(code, data)


def cmd_delete(args, headers: dict, token: str):
    """DELETE /teams/{id}"""
    print(f"  -> DELETE {_config['judge_url']}/teams/{args.id}")
    code, data = make_request("DELETE", f"/teams/{args.id}", headers, token)
    return print_response(code, None, stdout=False)


def cmd_full_cycle(args, headers: dict, token: str):
    """Full CRUD cycle: create, get, update, verify, delete, confirm."""
    team_name = args.name if hasattr(args, "name") and args.name else "Cycle-Team"
    clan_tag = getattr(args, "clan_tag", "CYC").upper() if hasattr(args, "clan_tag") else "CYC"
    passed = 0
    failed = 0

    def expect_ok(code, label):
        nonlocal passed, failed
        status = "OK" if 200 <= code < 400 else "FAIL"
        if 200 <= code < 400:
            passed += 1
            print(f"     [OK] {label} (HTTP {code})")
        else:
            failed += 1
            print(f"     [FAIL] {label} (HTTP {code})")

    print("[CRUD Cycle] Create")
    # Generate proper slug and alt_name for the test
    base_slug = team_name.lower().replace(" ", "-").replace("_", "-").replace(",", "-")
    slug = f"{base_slug}-test"
    alt_name = f"{team_name} Alternative"
    create_payload = {
        "name": team_name,
        "slug": slug,
        "alt_name": alt_name,
        "clan_tag": clan_tag,
    }
    print(f"  -> POST /teams")
    print(f"     body: {json.dumps(create_payload, indent=2)}")
    code, data = make_request("POST", "/teams", headers, token, create_payload)
    if code == 201:
        team_id = data.get("id") or data.get("team", {}).get("id")
        expect_ok(code, "Create team")
    else:
        print_response(code, data)
        expect_ok(code, "Create team")
        sys.exit(1)

    print()
    print("[CRUD Cycle] Read")
    print(f"  -> GET /teams/{team_id}")
    code, data = make_request("GET", f"/teams/{team_id}", headers, token)
    team = data.get("team", data)
    expect_ok(code, "Get team by ID")
    created_name = team.get("name", "")
    created_slug = team.get("slug", "")
    created_clan_tag = team.get("clan_tag", "")

    print()
    print("[CRUD Cycle] Update")
    updated_name = f"{team_name}-Updated"
    updated_clan_tag = "UPD"
    updated_slug = f"{base_slug}-updated"
    update_payload = {
        "name": updated_name,
        "clan_tag": updated_clan_tag,
        "slug": updated_slug,
    }
    print(f"  -> PUT /teams/{team_id}")
    print(f"     body: {json.dumps(update_payload, indent=2)}")
    code, data = make_request("PUT", f"/teams/{team_id}", headers, token, update_payload)
    expect_ok(code, "Update team")
    if code == 200:
        updated_team = data.get("team", data)
        actual_name = updated_team.get("name", "")
        actual_clan_tag = updated_team.get("clan_tag", "")
        actual_slug = updated_team.get("slug", "")
        print(f"     Verified: name={actual_name} (expected={updated_name}), clan_tag={actual_clan_tag} (expected={updated_clan_tag}), slug={actual_slug} (expected={updated_slug})")
        if actual_name != updated_name:
            print(f"     [FAIL] name mismatch: got '{actual_name}' expected '{updated_name}'")
            failed += 1
            passed -= 1
        else:
            passed += 1
            print(f"     [OK] name verified")
        if actual_clan_tag != updated_clan_tag:
            print(f"     [FAIL] clan_tag mismatch: got '{actual_clan_tag}' expected '{updated_clan_tag}'")
            failed += 1
            passed -= 1
        else:
            passed += 1
            print(f"     [OK] clan_tag verified")
        if actual_slug != updated_slug:
            print(f"     [FAIL] slug mismatch: got '{actual_slug}' expected '{updated_slug}'")
            failed += 1
            passed -= 1
        else:
            passed += 1
            print(f"     [OK] slug verified")

    print()
    print("[CRUD Cycle] Delete")
    print(f"  -> DELETE /teams/{team_id}")
    code, data = make_request("DELETE", f"/teams/{team_id}", headers, token)
    expect_ok(code, "Delete team")

    print()
    print("[CRUD Cycle] Confirm Deletion via List")
    print(f"  -> GET /teams")
    code, data = make_request("GET", "/teams", headers, token)
    expect_ok(code, "List teams (post-delete)")
    if code == 200:
        teams = data.get("teams", [])
        remaining_ids = [t.get("id") for t in teams]
        if team_id not in remaining_ids:
            passed += 1
            print(f"     [OK] Team {team_id} confirmed deleted (not in list)")
        else:
            failed += 1
            print(f"     [FAIL] Team {team_id} still found in list")

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
        description="Test the JudgeService Team API with M2M auth.",
    )
    parser.add_argument("--client-id", default="judge-client")
    parser.add_argument("--client-secret", default="foobar")
    parser.add_argument("--auth-server", default=AUTH_SERVER, help=f"Default: {AUTH_SERVER}")
    parser.add_argument("--judge-url", default=_config["judge_url"], help=f"Default: {_config['judge_url']}")

    sub = parser.add_subparsers(dest="command", required=True)

    # List teams
    list_p = sub.add_parser("list", help="GET /teams")

    # Create team
    create_p = sub.add_parser("create", help="POST /teams")
    create_p.add_argument("--name", required=True, help="Team name")
    create_p.add_argument("--slug", help="Team slug (default: name with spaces replaced by dashes)")
    create_p.add_argument("--alt-name", help="Alternative team name (default: name + ' Alt')")
    create_p.add_argument("--clan-tag", help="Clan tag (default: first 3 chars of name, uppercase)")

    # Get team by ID
    get_p = sub.add_parser("get", help="GET /teams/{id}")
    get_p.add_argument("id", help="Team ID")

    # Update team
    update_p = sub.add_parser("update", help="PUT /teams/{id}")
    update_p.add_argument("id", help="Team ID")
    update_p.add_argument("--name", help="New team name")
    update_p.add_argument("--clan-tag", help="New clan tag")
    update_p.add_argument("--slug", help="New slug")
    update_p.add_argument("--alt-name", help="New alt name")

    # Delete team
    del_p = sub.add_parser("delete", help="DELETE /teams/{id}")
    del_p.add_argument("id", help="Team ID")

    # Full cycle
    cycle_p = sub.add_parser("full-cycle", help="Full CRUD cycle")
    cycle_p.add_argument("--name", default="Cycle-Team", help="Team name for cycle")
    cycle_p.add_argument("--clan_tag", default="CYC", help="Clan tag for cycle")

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
        "list":        cmd_list,
        "create":      cmd_create,
        "get":         cmd_get,
        "update":      cmd_update,
        "delete":      cmd_delete,
        "full-cycle":  cmd_full_cycle,
    }

    success = dispatch[args.command](args, headers, token)
    print()
    if not success:
        print("[RESULT] FAILED")
        sys.exit(1)
    print("[RESULT] OK")


if __name__ == "__main__":
    main()
