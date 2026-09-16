# edcctl

> **JSON-only replacement pending.** The current implementation parses HTML and does not meet the revised requirement.
> See the [traffic-capture findings](docs/json-api-investigation.md) for verified endpoints and the remaining evidence needed.

A read-only CLI for the Evolution Dance Complex (EDC) parent portal.
EDC uses a MobileInventor iOS app with an embedded Studio Pro web portal.
This CLI uses the same portal without a browser or simulator.

Generated from [restctl-template](https://github.com/jwmoss/restctl-template), with the command structure used by [classreach](https://github.com/jwmoss/classreach).
This is an unofficial client. Portal changes can break its HTML parsers.

## Install

Requires Go 1.25 or later and access to this private repository.

```sh
gh repo clone jwmoss/edcctl
cd edcctl
make build
install -m 755 bin/edcctl ~/.local/bin/edcctl
```

Add `~/.local/bin` to your PATH if necessary.

## Credentials

The CLI reads your existing `EDC_LOGIN` and `EDC_PASSWORD` environment variables.
Run it from a shell that exports these variables.
It does not read or execute `~/.zshrc` itself.

```sh
edcctl login
edcctl doctor
```

Each command creates a new session. Cookies stay in memory.
The default studio account ID is `30834`.

Optional configuration:

```sh
edcctl config init
edcctl config show
```

The config file uses the OS configuration directory: `~/Library/Application Support/edcctl/config.yaml` on macOS.
Use `--config PATH` for another location.

```yaml
base_url: https://app.gostudiopro.com/online
account_id: "30834"
email: ""
password: ""
```

Keep credentials in environment variables when possible.
`config init --email EMAIL --password-stdin` can store a password from stdin in a mode-0600 file.
Never commit this file.

Environment overrides: `EDCCTL_BASE_URL`, `EDCCTL_ACCOUNT_ID`, `EDCCTL_USERNAME`, and `EDCCTL_PASSWORD`.
`EDCCTL_USERNAME` and `EDCCTL_PASSWORD` take precedence over `EDC_LOGIN` and `EDC_PASSWORD`.

## Commands

```sh
edcctl students
edcctl schedule --week 2
edcctl schedule --from 2026-09-28 --to 2026-10-05
edcctl balance
edcctl history
edcctl history --from 2026-01-01
edcctl announcements
edcctl announcements MESSAGE_ID
edcctl files
edcctl files --category shared
edcctl account
edcctl --json students
edcctl completion zsh
```

- `students` lists dancers and enrolled classes, with instructors and rooms.
- `schedule` defaults to the current Monday-to-Sunday week. `--to` is exclusive. Times use the studio's local time.
- `balance` and `history` show balances, payments, and charges as portal-formatted strings.
- `announcements` lists email history and supported message rows. A message ID returns its text.
- `files` lists shared file and media URLs. It does not download them.
- `account` shows the primary contact record.

`--json` emits structured data. `--plain` selects tab-separated schedule rows.
`--trace-http` logs methods, paths, status codes, and durations without request bodies or query values.
`--timeout` sets the request timeout. `--dry-run` refuses every POST, including login; it does not preview data.

There are no payment, enrollment, absence-report, account-edit, or arbitrary-request commands.
Some reads use POST because the portal requires it.
HTML message formatting and attachments are not reproduced as a full email view.

## Development

```sh
make check
go test -race ./...
goreleaser check
zizmor .github/workflows
```

Tests use synthetic data and local HTTP servers, not real credentials.
See [API discovery](docs/api-discovery.md) for endpoint evidence and limitations.
