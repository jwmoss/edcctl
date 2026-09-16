# AGENTS.md

## Purpose

`edcctl` reads the EDC schedule from Studio Pro's private JSON endpoint. Keep provider-specific behavior in
commands and typed API packages; keep generic transport, config, and output
helpers small and reusable.

## CLI Rules

- Data queries must use verified JSON endpoints. Reject HTML data responses without a scraping fallback.
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
