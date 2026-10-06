# CLI flow tests

Run `npm ci --ignore-scripts`, then `npm run test:e2e`.
The suite builds the real CLI and runs each command as a subprocess.
Use Node 26, Go, and OpenSSL. On macOS, use Go 1.27 or later for the temporary certificate trust override.
Linux supports the Go version in `go.mod`.

Each flow has a temporary home, explicit configuration path, and controlled environment.
The fixture never reads real credentials or changes real accounts.
Each CLI process has a ten-second limit. Each test has a thirty-second limit.

Studio Pro requests use a local HTTP server through the public `--base-url` option.
MobileInventor has a fixed HTTPS host. A local CONNECT proxy directs that host to a local TLS server.
The fixture creates a one-day certificate and gives only the subprocess its trust file.
The proxy refuses other hosts. Production TLS and authentication stay active.
Negative app tests reject an untrusted certificate and an unreachable CONNECT proxy before any mobile HTTP request or credential disclosure.
The fixture checks portal cookies, CSRF, query parameters, and form fields at the HTTP boundary.
It checks that portal secrets never reach MobileInventor.

| Family | Executable success coverage | Failure and safety coverage |
| --- | --- | --- |
| `config init` | stdin password, saved YAML, JSON receipt, private file replacement | existing file survives refusal; hard links retain old contents and mode; live and dangling symlinks fail; dry-run preserves files |
| `config show` | JSON redaction, selected path in text/JSON, environment priority, plain output | malformed YAML; conflicting formats; no network requests |
| `login` | saved credentials, legacy env fallback, account flag, JSON/text/quiet, HTTP trace | missing credentials, refused credentials, missing CSRF, wrong HTML page, redirect, 401, timeout, refused connection, dry-run |
| `schedule` | range sorting, all-day events, exclusive bounds, JSON/plain/table, empty list, week offset | invalid ranges and arguments before login; HTML, malformed JSON, null, object, invalid dates; 503 |
| `doctor` | JSON/text receipt, current Monday week, seven-day range | extra arguments; schedule transport error |
| `app info` | JSON/plain/table, app identity, studio location | incomplete response, 503, wrong studio scope |
| `app groups` | selected public/private metadata, hidden group exclusion, JSON/plain/table, empty list | missing access rules, wrong app identity, null response |
| `app notifications` | public-group lookup, decoded detail/link/media, JSON/plain/table, control-character sanitation, empty list | missing/negative group; private/password/invitation/approval/login/hidden/unknown group; missing messages; invalid base64 |
| `app profile` | authenticated email, JSON/plain/table, selected profile fields | extra email argument; identity mismatch; unavailable profile |
| `version`, `--version` | build receipt, embedded VCS metadata, JSON/plain/text with malformed config | conflicting formats; extra arguments; `--version` applies only to the root |
| `completion` | generated Bash/Zsh/Fish/PowerShell output without credentials | absent or unsupported shell |
| removed commands | subprocess rejection of each removed HTML resource | no authentication or HTML fallback |
| invalid command input | exit code `2` for unknown commands/flags, invalid flag values, and extra arguments | validation precedes config reads, writes, and login |

All data-command failures require a nonzero exit and empty stdout.
Success tests check output and request outcomes. They do not read production source.
The Go config and CLI regressions fail before their respective repairs.
Existing Go tests retain their protocol and package contracts.
`go test ./tests` also installs from a temporary file-based module proxy.
It checks the module-version fallback and explicit build-version priority without external network access.
That test needs dependency archives in the local Go module cache.

No endpoint exposes pagination. The CLI has no pagination command or cursor contract to test.
The suite checks synthetic protocol responses. It does not prove current vendor availability or a real account's data.
Completion tests prove script generation, not shell installation or interactive completion.
The suite does not run real account mutations, send feedback, use a model, or launch a browser.
The runner saves reports in `.e2e/report.json`, `.e2e/junit.xml`, and `.e2e/summary.md`.
