#!/usr/bin/env python3
"""
set_current_round_teams.py

Sets the teams 'foo' and 'bar' as active in the current round using the judge-service API.
Creates the teams 'foo' and 'bar' first if they do not exist.
"""

import argparse
import sys
import json
import urllib.request
import urllib.error

DEFAULT_JUDGE_URL = "http://localhost:8086"
HEADERS = {
    "Content-Type": "application/json",
    "x-auth-scope": "judge",
    "x-auth-user": "judge",
}

def make_request(url, method="GET", data=None):
    req = urllib.request.Request(url, method=method, headers=HEADERS)
    body_bytes = json.dumps(data).encode("utf-8") if data else None
    try:
        with urllib.request.urlopen(req, data=body_bytes) as resp:
            content = resp.read().decode("utf-8")
            return resp.status, json.loads(content) if content else {}
    except urllib.error.HTTPError as e:
        content = e.read().decode("utf-8")
        try:
            err_json = json.loads(content)
        except Exception:
            err_json = {"error": content}
        return e.code, err_json
    except Exception as e:
        print(f"❌ Connection error: {e}", file=sys.stderr)
        sys.exit(1)

def ensure_team(base_url, slug, name, alt_name, clan_tag):
    status, res = make_request(f"{base_url}/teams")
    if status == 200:
        teams = res.get("teams", [])
        for t in teams:
            if t.get("slug") == slug or t.get("name") == name or t.get("clan_tag") == clan_tag:
                print(f"✅ Found existing team '{t.get('name')}' (ID: {t.get('id')})")
                return t["id"]

    print(f"➕ Creating team '{name}'...")
    create_payload = {
        "slug": slug,
        "name": name,
        "alt_name": alt_name,
        "clan_tag": clan_tag,
    }
    status, res = make_request(f"{base_url}/teams", method="POST", data=create_payload)
    if status in (200, 201):
        team = res.get("team", {})
        print(f"✅ Created team '{team.get('name')}' (ID: {team.get('id')})")
        return team["id"]
    else:
        print(f"❌ Failed to create team '{name}': {res}", file=sys.stderr)
        sys.exit(1)

def main():
    parser = argparse.ArgumentParser(description="Set active teams 'foo' and 'bar' in current round")
    parser.add_argument("--judge-url", default=DEFAULT_JUDGE_URL, help="Judge service base URL")
    args = parser.parse_args()

    base_url = args.judge_url.rstrip("/")
    print(f"🎯 Connecting to Judge Service at {base_url}...")

    # Ensure teams foo and bar exist
    team_a_id = ensure_team(base_url, slug="foo", name="foo", alt_name="foo-alt", clan_tag="[foo]")
    team_b_id = ensure_team(base_url, slug="bar", name="bar", alt_name="bar-alt", clan_tag="[bar]")

    # Set teams in current round
    print(f"🔄 Setting active round teams: Team A ID={team_a_id} ('foo'), Team B ID={team_b_id} ('bar')...")
    payload = {
        "team_a_id": team_a_id,
        "team_b_id": team_b_id,
    }
    status, res = make_request(f"{base_url}/rounds/current/teams", method="POST", data=payload)
    if status == 200:
        print("🎉 Successfully updated active match teams!")
        print(json.dumps(res, indent=2))
    else:
        print(f"❌ Failed to set active round teams ({status}): {res}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
