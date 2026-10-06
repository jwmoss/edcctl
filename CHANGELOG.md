# Changelog

## Unreleased

## 0.2.0 (2026-10-06)

- Add read-only `balance` and `history [--from DATE]` commands.
- Parse only the verified `my_payments.php` and `my_history.php` structure, because no JSON endpoint exists.
- Reconcile ledger running balances and the account balance; fail closed on unexpected markup.

## 0.1.1 (2026-10-06)

- Publish private config files without destination symlink traversal or unintended overwrites.
- Report selected config paths and emit a JSON config-init receipt.
- Reject invalid arguments with exit code 2 before login or config access.
- Resolve installed module versions while preserving explicit build metadata.
- Use Go 1.27.1 and gate releases on native tests, CLI flows, race checks, and security scans.

## 0.1.0

- Add `app info`, `app groups`, `app notifications --group ID`, and `app profile` through verified JSON endpoints.
- Keep portal authentication separate from MobileInventor requests.
- Reject private-group message access until membership can be verified.
- Filter sensitive and unknown app-response fields from command output.

- Query schedule data only through the verified JSON calendar endpoint.
- Keep CSRF login and session cookies internal, with the existing EDC credentials.
- Make `doctor` validate the JSON endpoint instead of an HTML page.
- Remove HTML-based students, balance, history, announcements, files, and account commands.
- Remove the HTML data parser and its dependencies.
- Reject non-JSON data responses, invalid event dates, and null arrays without a fallback.
- Test login, JSON-only data requests, date filtering, and failure cases with synthetic data.
