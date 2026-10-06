# API discovery

## Source

The installed app is `/Applications/EDC.app/Wrapper/Evolution Dance Complex.app`.
Its bundle ID is `com.evolutiondancecomplex.mi`.
The Cordova `www/index.html` sets `_base` to `https://mobileinventor.com/` and `_appId` to `2555`.

`applib/getAppInfo.php?appid=2555` identifies the backend as `dancestudiopro`, with `cmsID` `30834`.
`appdata/js/danceStudioPro.js` embeds `https://app.gostudiopro.com/online/` portal pages.
The portal's `calendar.php` configures FullCalendar to query `class_calendar-ajax.php` for JSON events.

The repository contains neither vendor scripts nor captured portal responses.
Template source commit: `65972fd9d6845e595a0ef00c174e0dffbc553046`.

## Authentication is internal

1. GET `index.php?account_id=30834&app=1&app_mi=1`.
2. Retain the session cookie and CSRF token.
3. POST the same URL with `email`, `password`, `csrf_token`, and `btn_login=1`.
4. Check for the login confirmation and schedule redirect marker without following that redirect.

Only this login step consumes HTML. It does not fetch the HTML schedule page.
Login responses can contain a reusable credential value. Never log or commit them.
The client retains cookies only in memory and sends the CSRF token on subsequent POST requests.

## JSON data endpoint

Both `schedule` and `doctor` use:

```text
POST https://app.gostudiopro.com/online/class_calendar-ajax.php?account_id=30834&app=1&app_mi=1
Accept: application/json
Content-Type: application/x-www-form-urlencoded
X-CSRF-Token: <session token>
Cookie: <session cookie>
```

Form fields: `action=getclasses`, `start`, `end`, `selected_view=agendaWeek`, and empty season/teacher/location/room filters.
Use `YYYY-MM-DD` dates. The CLI treats the end date as exclusive.

Synthetic response:

```json
[
  {
    "id": "456",
    "type": "class",
    "sid": "123",
    "title": "Ballet",
    "start": "2026-09-28T16:00:00",
    "end": "2026-09-28T17:00:00"
  }
]
```

The server can label JSON as `text/html` and include recurrences after the requested end date.
The client decodes the response body as JSON, filters the range, and sorts events.
An HTML response, malformed payload, null array, or invalid event start produces an error, not an HTML fallback or partial schedule.

## MobileInventor JSON resources

The following endpoints are verified with app ID `2555`. Their base URL is `https://mobileinventor.com/applib`.

| CLI command | Method | Endpoint | Parameters |
| --- | --- | --- | --- |
| app info | GET | getAppInfo.php | `appid=2555` |
| app groups | GET | groupsUtil.php | `req=getgroups`, `appID=2555` |
| app notifications | GET | notificationsUtil.php | `req=getMsgs`, `appID=2555`, `groupID=<public group>` |
| app profile | POST | AppProfileManager.php | `req=getacctinfo`, `aid=2555`, `email=<authenticated email>` |

Sources: `pageLoader-v2.js` (`getGroups`) and `common.js` (`GroupNotifications`, `getAcctInfo`).
Live CLI verification returns one studio location, four visible groups, 44 public-group messages, and the configured user's app profile.
Counts can change. Captured responses remain outside Git; tests use synthetic data.

The CLI validates the existing Studio Pro login before app queries.
MobileInventor requests use a separate client and cookie jar, without portal credentials or CSRF headers.
The CLI rejects non-EDC account overrides for app commands.

### Access and output limits

- Group responses contain access rules and may contain passwords. The CLI returns only selected metadata.
- The CLI omits hidden groups and validates all access-rule fields before message access.
- Message access requires `anyoneCanJoin=1`, with no password, invitation, approval, or login requirement.
- A valid portal login does not establish private-group membership. The CLI refuses those groups without querying their messages.
- Notification detail, link, and media fields use base64 inside JSON. The CLI decodes them without HTML parsing or network follow-up.
- Profile queries use only the configured email after successful login. The returned email must match.
- Profile output excludes chat tokens and unknown fields. The client never requests `chattoken=1`.

### Other inspected leads

`getVersionInfo.php` serves JSON asset-update metadata; app versions already appear in `app info`.
The app configuration endpoint, `getAppFeatureData.php`, serves HTML and is not a data command.
The JavaScript includes generic Google Calendar, event, and photo handlers, but no usable EDC-specific JSON contract is verified for them.
`dspClassOpenings.php` loads inside an iframe; that reference alone does not establish JSON.
App statistics, account updates, group changes, and credential-sync endpoints are outside this read-only client.
No payment, enrollment, or notification-setting mutations occur during discovery or verification.

## Balance and payment history

No JSON endpoint serves balances or ledger entries. A probe on 2026-10-06 found:

- `appdata/js/danceStudioPro.js` opens `my_payments.php` and `my_history.php` in the app's web view.
  The EDC app has no other data source for these screens.
- `my_history.php` renders the ledger on the server. It makes no AJAX request.
- `my_payments.php` calls `pay-ajax.php` only for the payment flow, and each action returns an HTML fragment.
  Those actions start or change a payment and are outside this read-only client.
- `getAppUserData.php` and `updateAppUserData.php` synchronize app credentials, not balances.

The CLI therefore parses these two pages with `golang.org/x/net/html`:

| CLI command | Method | Page | Parameters |
| --- | --- | --- | --- |
| balance | GET | my_payments.php | portal query only |
| history | GET | my_history.php | portal query only |
| history --from DATE | POST | my_history.php | `csrf_token`, `show_month`, `show_day`, `show_year`, `btn_search=Search` |

The date search does not persist: a new session shows the default start date again.

`balance` requires exactly one table with the headers `Name`, `Balance`, and `Statement`.
Each row needs a name, an amount, and one `app_statements.php?sid=N` link.
When the `You owe $X` heading is present, it must equal the sum of the student balances.

`history` requires the `Payment History` heading, its `Balance:` line, and one selected start date.
It requires exactly one table with the headers `--`, `Pmts`, `Chrgs`, and `Bal`.
The last row must be the opening balance. Every other row needs one student, one `Mon DD,YYYY` date, and a description.
A long memo appears twice: truncated and in a hidden `sp_full_` span. The CLI uses the full span.
Each running balance must equal the older balance plus the charge minus the payment.
The newest running balance must equal the account balance.
The live ledger on 2026-10-06 reconciles across 141 entries.

Accepted amount formats: `12.34`, `$1,234.56`, `Owe $1.00`, `Owes $1.00`, and `($1.00)` for credit.
A login form, another structure, or another amount format is an error. No partial data is returned.

## Unsupported resources

The earlier client parsed HTML for students, financial records, messages, shared files, and contact details.
Those commands and their HTML parser dependencies are removed. Balance and history return with the strict parsers above.
Add resource commands only after a JSON endpoint is verified under the authorized parent account.
The JSON schedule does not require another app, another password, or an entirely JSON login protocol.

This is an undocumented JSON/AJAX endpoint, not a vendor-supported public REST API.
The CLI has no account-mutation or payment commands.
