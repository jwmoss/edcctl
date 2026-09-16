# edcctl

Query Evolution Dance Complex schedules and mobile-app resources through verified JSON endpoints.
The CLI uses your existing `EDC_LOGIN` and `EDC_PASSWORD` credentials. No extra app or traffic capture is required.

**Data queries use JSON, not HTML scraping.** Login handles the portal's CSRF form and session cookie internally.
The CLI never scrapes the schedule page or falls back to HTML when a data query fails.

Generated from [restctl-template](https://github.com/jwmoss/restctl-template), with the command structure used by [classreach](https://github.com/jwmoss/classreach).
This is an unofficial client for an undocumented endpoint.

## Use

Run from a shell that exports `EDC_LOGIN` and `EDC_PASSWORD`.

```sh
edcctl --json schedule
edcctl --json schedule --week 2
edcctl --json schedule --from 2026-09-28 --to 2026-10-05
edcctl doctor
```

The default range is the current Monday-to-Sunday week, based on the machine's local date.
`--to` is exclusive. Supply either `--week` or `--from` with `--to`.
The server supplies times in the studio's local time.

The API returns event IDs, student IDs, titles, and start/end times.
The CLI filters out events beyond the requested range and sorts the result by start time.
An empty schedule returns `[]` with `--json`.

Without `--json`, the CLI prints a table. Use `--plain` for tab-separated rows.

## Available commands

| Command | Purpose |
| --- | --- |
| schedule | Query the Studio Pro JSON calendar endpoint |
| app info | Read app versions, store links, and studio locations |
| app groups | List visible notification groups and public access status |
| app notifications --group ID | Read a public group's push notifications |
| app profile | Read your MobileInventor app profile |
| login | Validate credentials |
| doctor | Validate login and a JSON calendar query |
| config show / init | Inspect or create configuration |
| version | Print build information |
| completion | Generate shell completions |

The HTML-based `students`, `balance`, `history`, `announcements`, `files`, and `account` commands are removed.
They will return only when verified JSON endpoints support them. There is no HTML fallback.

## Mobile-app resources

```sh
edcctl --json app info
edcctl --json app groups
edcctl --json app notifications --group 8985
edcctl --json app profile
```

These commands query MobileInventor, the EDC app provider, after the normal Studio Pro login.
They support only the EDC app (`2555`) and studio (`30834`).
The separate app client never receives the Studio Pro password, session cookie, or CSRF token.
Profile queries use only the authenticated email; there is no arbitrary-email option.

Use an ID with `public: true` from `app groups` to read messages.
The CLI rejects private, password-protected, invitation-only, login-required, hidden, and unknown groups.
It does not join groups or change notification settings.

Notifications are app push messages, not Studio Pro bulletin-board messages.
`sent_at` contains Unix seconds. JSON output includes `detail_html`, decoded from the API's base64 field without HTML parsing.
Treat that field as untrusted content. The CLI neither renders it nor opens message links.
Table and plain output show message summaries.

`app profile` is the mobile-app identity, not the Studio Pro contact or billing record.
Output excludes chat tokens, group passwords, member lists, and unknown backend fields.

## Install

Requires Go 1.25 or later and access to this private repository.

```sh
gh repo clone jwmoss/edcctl
cd edcctl
make build
mkdir -p ~/.local/bin
install -m 755 bin/edcctl ~/.local/bin/edcctl
```

Add `~/.local/bin` to your PATH if necessary.

## Configuration

The CLI reads exported environment variables, not `~/.zshrc` directly.
Each command creates a session whose cookies stay in memory.
The default studio account ID is `30834`.

`EDCCTL_USERNAME` and `EDCCTL_PASSWORD` override `EDC_LOGIN` and `EDC_PASSWORD`.
Optional overrides: `EDCCTL_BASE_URL`, `EDCCTL_ACCOUNT_ID`, `--base-url`, and `--account-id`.

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

`--trace-http` logs request methods, paths, status codes, and durations, without bodies or query values.
`--timeout` sets the request timeout.
`--dry-run` refuses every POST, including login and the read-only calendar query; it does not preview data.
There are no payment, enrollment, absence-report, account-edit, or arbitrary-request commands.

## Development

```sh
make check
go test -race ./...
```

Tests use synthetic data and local HTTP servers, not real credentials.
See [API discovery](docs/api-discovery.md) for the protocol and [capture findings](docs/json-api-investigation.md) for supporting evidence.
