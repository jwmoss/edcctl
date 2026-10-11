#!/usr/bin/env python3
"""Read the configured live service through the compiled CLI."""

import json
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
