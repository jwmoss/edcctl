#!/usr/bin/env python3
"""Read the configured live service through the compiled CLI."""

import json
from datetime import date, timedelta
from pathlib import Path
import subprocess
import sys

binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "bin/edcctl").resolve())


def read(*args):
    try:
        result = subprocess.run(
            [binary, "--no-input", "--timeout", "15s", "--json", *args],
            capture_output=True, text=True, timeout=60, check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        raise SystemExit("Live check failed: executable unavailable or deadline exceeded.") from None
    if result.returncode:
        raise SystemExit(f"Live check failed: CLI exit {result.returncode}. Check service access and saved credentials locally.")
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError:
        raise SystemExit("Live check failed: command did not return JSON.") from None


def require(condition, message):
    if not condition:
        raise SystemExit("Live check failed: " + message)


report = read("doctor")
require(isinstance(report, dict) and report.get("ok") is True, "authenticated schedule request failed")
require(report.get("endpoint") == "/class_calendar-ajax.php", "unexpected schedule endpoint")
require(type(report.get("events")) is int and report["events"] >= 0, "invalid schedule count")
print(f"Live E2E passed: authenticated schedule returned {report['events']} events.")

config = read("config", "show", "--account-id", "123", "--base-url", "https://example.com/online")
require(config.get("account_id") == "123", "config show ignores account override")
require(config.get("base_url") == "https://example.com/online", "config show ignores URL override")

balance = read("balance")
history = read("history")
require(sum(s["balance_cents"] for s in balance["students"]) == balance["balance_cents"], "student balances do not reconcile")
require(history["balance_cents"] == balance["balance_cents"], "ledger and student balances differ")
start = (date.today() - timedelta(days=30)).isoformat()
filtered = read("history", "--from", start)
require(filtered["from"] == start, "history date search did not preserve the requested date")
print(f"Live E2E passed: balance and {len(history['entries'])} ledger entries.")

info = read("app", "info")
require(info["app_id"] == 2555 and bool(info["locations"]), "app identity or locations missing")
groups = read("app", "groups")
require(isinstance(groups, list), "groups is not an array")
public = next((group for group in groups if group["public"]), None)
require(public is not None, "no public group available to verify notifications")
messages = read("app", "notifications", "--group", str(public["id"]))
require(isinstance(messages, list), "notifications is not an array")
profile = read("app", "profile")
require(bool(profile["id"]) and bool(profile["email"]), "app profile is incomplete")
print(f"Live E2E passed: app info, profile, {len(groups)} groups, and {len(messages)} public notifications.")
