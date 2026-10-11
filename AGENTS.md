# AGENTS.md

## Purpose

`edcctl` reads EDC schedules and mobile-app resources from verified Studio Pro and MobileInventor JSON endpoints.
It also reads balance and payment history from two portal HTML pages, because no JSON endpoint exists for them. Keep provider-specific behavior in
commands and typed API packages; keep generic transport, config, and output
helpers small and reusable.

## CLI Rules

- Data queries must use verified JSON endpoints. Reject HTML data responses without a scraping fallback.
- The only HTML data exception is `my_payments.php` and `my_history.php` in `internal/api/payments.go`.
  Those parsers accept only the verified structure, reconcile amounts, and fail closed. Do not add other HTML parsers
  without a documented proof that no JSON endpoint exists.
- Keep the CSRF form and HTML login response handling inside authentication; this is not a blocker for JSON data queries.
- Primary command output goes to stdout.
- Errors, traces, and diagnostics go to stderr.
- Keep `--json` stable for scripts.
- Keep `--plain` stable and line-oriented where implemented.
- Keep credentials, session cookies, and captured portal responses out of output and Git.
- Read `docs/api-discovery.md` before changes to authentication or endpoint behavior.
- Do not add token/password command-line flags unless there is a documented
  reason and a safer path is still available.
- Mutating commands must respect `--dry-run`.
- Commands that need credentials should explain the missing env/config value in
  the error.

## Validation

Run before handoff:

```bash
make check
```

For release config changes, also run if GoReleaser is installed:

```bash
goreleaser check
```

## Test policy

- Keep only live end-to-end tests against the configured service and static checks.
- Do not add unit tests, mock transports, fake servers, device simulators, or synthetic subprocess fixtures.
- Run `make check` for formatting, module consistency, vet, script syntax, and builds.
- Run `make test` with existing credentials for read-only live verification.
- Report missing credentials or service failures as blocked live checks, never passes.
- CI runs static checks without credentials. It does not run live service tests.
