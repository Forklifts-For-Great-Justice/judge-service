#!/usr/bin/env python3
"""
test_foo_player.py

Tests team 'foo' submitting a challenge flag to earn HackCoins,
and then using those HackCoins to purchase a shenanigan against judge-service.
"""

import argparse
import json
import sys
import urllib.error
import urllib.request

DEFAULT_JUDGE_URL = "http://localhost:8086"


def make_request(url: str, method: str = "GET", headers: dict = None, data: dict = None) -> tuple[int, dict]:
    req_headers = {"Content-Type": "application/json"}
    if headers:
        req_headers.update(headers)

    body_bytes = json.dumps(data).encode("utf-8") if data else None
    req = urllib.request.Request(url, method=method, headers=req_headers)

    try:
        with urllib.request.urlopen(req, data=body_bytes, timeout=15) as resp:
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
        print(f"❌ Network error: {e}", file=sys.stderr)
        sys.exit(1)


def main():
    parser = argparse.ArgumentParser(description="Test team 'foo' challenge submission and shenanigan purchase")
    parser.add_argument("--judge-url", default=DEFAULT_JUDGE_URL, help="Judge service base URL")
    args = parser.parse_args()

    base_url = args.judge_url.rstrip("/")
    print(f"🎯 Target Judge Service: {base_url}")

    # 1. Resolve Team ID for 'foo'
    print("\n1️⃣  Resolving Team ID for 'foo'...")
    status, res = make_request(f"{base_url}/teams", headers={"x-auth-scope": "judge", "x-auth-user": "judge"})
    team_foo_id = None
    if status == 200:
        teams = res.get("teams", [])
        for t in teams:
            if t.get("slug") == "foo" or t.get("name") == "foo" or t.get("clan_tag") == "[foo]":
                team_foo_id = t["id"]
                break

    if not team_foo_id:
        print("❌ Team 'foo' not found. Creating team 'foo'...")
        create_payload = {"slug": "foo", "name": "foo", "alt_name": "foo Alt", "clan_tag": "[foo]"}
        status, res = make_request(f"{base_url}/teams", method="POST", headers={"x-auth-scope": "judge", "x-auth-user": "judge"}, data=create_payload)
        if status in (200, 201):
            team_foo_id = res.get("team", {}).get("id") or res.get("id")
        else:
            print(f"❌ Failed to create team 'foo': {res}", file=sys.stderr)
            sys.exit(1)

    print(f"✅ Found Team 'foo' with ID: {team_foo_id}")

    # 2. Ensure Team 'foo' is active in current match
    print("\n2️⃣  Setting Team 'foo' as active match Team A...")
    set_match_payload = {"team_a_id": team_foo_id, "team_b_id": team_foo_id + 1 if team_foo_id != 8 else 7}
    make_request(f"{base_url}/rounds/current/teams", method="POST", headers={"x-auth-scope": "judge", "x-auth-user": "judge"}, data=set_match_payload)

    # 3. Create a unique challenge for Team 'foo'
    import time
    unique_name = f"Foo-Challenge-{int(time.time())}"
    unique_flag = f"FLAG{{foo_{int(time.time())}}}"
    print(f"\n3️⃣  Creating a new challenge '{unique_name}' (50 points)...")
    ch_payload = {
        "name": unique_name,
        "description": "Test challenge for team foo",
        "points": 50,
        "flag": unique_flag,
    }
    status, ch_res = make_request(f"{base_url}/challenges", method="POST", headers={"x-auth-scope": "judge", "x-auth-user": "judge"}, data=ch_payload)
    target_ch_id = ch_res.get("id") or ch_res.get("challenge", {}).get("id")
    target_ch_points = ch_res.get("points") or ch_res.get("challenge", {}).get("points", 50)
    print(f"✅ Created Challenge ID={target_ch_id} ('{unique_name}'), Points={target_ch_points}")

    # 4. Submit Flag for Challenge
    print(f"\n4️⃣  Submitting flag '{unique_flag}' for Challenge {target_ch_id} as Team 'foo'...")
    sub_payload = {
        "challenge_id": target_ch_id,
        "flag": unique_flag,
        "team_id": team_foo_id,
        "player_id": "foo_player_1",
    }
    status, submit_res = make_request(f"{base_url}/player/challenges/submit", method="POST", data=sub_payload)
    print(f"   Response ({status}): {json.dumps(submit_res, indent=2)}")
    if not submit_res.get("correct"):
        print("⚠️ Flag submission was not accepted (might have already been solved).")
    else:
        print(f"🎉 Flag accepted! Awarded {submit_res.get('points_awarded')} points/hackcoins.")

    # 5. Fetch available shenanigans
    print("\n5️⃣  Fetching available shenanigans...")
    status, res = make_request(f"{base_url}/player/shenanigans")
    shenanigans = res.get("shenanigans", [])
    if not shenanigans:
        print("❌ No shenanigans available to buy.")
        sys.exit(1)

    # Pick a shenanigan with cost <= awarded points/hackcoins (or cost = 5)
    target_shen = None
    for s in shenanigans:
        cost = s.get("cost") if s.get("cost") is not None else s.get("price", 0)
        if cost <= 10:
            target_shen = s
            break

    if not target_shen:
        target_shen = shenanigans[0]

    cost_val = target_shen.get("cost") if target_shen.get("cost") is not None else target_shen.get("price", 0)
    print(f"✅ Selected Shenanigan ID={target_shen['id']} ('{target_shen['name']}'), Cost={cost_val}")

    # 6. Buy Shenanigan
    print(f"\n6️⃣  Purchasing Shenanigan ID={target_shen['id']} as Team 'foo'...")
    buy_payload = {
        "shenanigan_id": target_shen["id"],
        "team_id": team_foo_id,
        "buyer_id": "foo_player_1",
    }
    status, buy_res = make_request(f"{base_url}/player/shenanigans/buy", method="POST", data=buy_payload)
    print(f"   Response ({status}): {json.dumps(buy_res, indent=2)}")

    if "error" in buy_res:
        print(f"❌ Purchase failed: {buy_res['error']}")
        sys.exit(1)

    print(f"🎉 Shenanigan successfully purchased! Purchase ID: {buy_res.get('purchase_id')}, Remaining Coins: {buy_res.get('remaining_coins')}")


if __name__ == "__main__":
    main()
