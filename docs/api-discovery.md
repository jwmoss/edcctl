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

## Unsupported resources

The earlier client parsed HTML for students, financial records, messages, shared files, and contact details.
Those commands and their HTML parser dependencies are removed.
Add resource commands only after a JSON endpoint is verified under the authorized parent account.
The JSON schedule does not require another app, another password, or an entirely JSON login protocol.

This is an undocumented JSON/AJAX endpoint, not a vendor-supported public REST API.
The CLI has no account-mutation commands.
