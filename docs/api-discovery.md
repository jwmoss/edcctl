# API discovery

## Source

The installed app is `/Applications/EDC.app/Wrapper/Evolution Dance Complex.app`.
Its bundle ID is `com.evolutiondancecomplex.mi`.
The Cordova `www/index.html` sets `_base` to `https://mobileinventor.com/` and `_appId` to `2555`.

`applib/getVersionInfo.php?app_id=2555&staging=false` lists the server scripts.
`applib/getAppInfo.php?appid=2555` identifies the backend as `dancestudiopro`, with `cmsID` `30834`.
`appdata/js/danceStudioPro.js` embeds `https://app.gostudiopro.com/online/` portal pages.

The repository contains neither vendor scripts nor captured portal responses.
Template source commit: `65972fd9d6845e595a0ef00c174e0dffbc553046`.

## Authentication

1. GET `index.php?account_id=30834&app=1&app_mi=1`.
2. Retain the session cookie and CSRF token.
3. POST the same URL with `email`, `password`, `csrf_token`, and `btn_login=1`.
4. Check for the login confirmation and schedule redirect marker.

The app receives a `dsp-portal` postMessage after login.
That response can contain a reusable credential value. Never log or commit it.
This client consumes the response without printing it and retains cookies only in memory.

## Read endpoints

| Command | Endpoint | Result |
| --- | --- | --- |
| students | GET my_students.php | Student cards and class accordions |
| schedule | POST class_calendar-ajax.php | JSON calendar events |
| balance | GET my_payments.php | Balance heading and student table |
| history | GET/POST my_history.php | Ledger rows |
| announcements | GET bulletin_board.php | Communication IDs, subjects, dates |
| announcements ID | POST bulletin_board-ajax.php | HTML message body |
| files | GET files.php | File and media links |
| account | GET my_account.php | Primary contact form values |

Calendar form fields: `action=getclasses`, `start`, `end`, `selected_view=agendaWeek`, and empty season/teacher/location/room filters.
The server can include events after the requested end date. The client filters and sorts the result.

History search fields: `show_month`, `show_day`, `show_year`, and `btn_search=1`.
Message detail fields: `action=show_com_log` and `mid`.
File links wrap their target in `app_display_file.php?url=...`.

## Limits

This is an HTML portal adapter, not a supported public REST API.
The CLI does not modify payments, registration, waivers, absences, or contact records.
The MobileInventor app also contains studio content outside the parent portal; this CLI does not reproduce all app screens.
Tests use synthetic fixtures. Live checks verify only the authorized account.
