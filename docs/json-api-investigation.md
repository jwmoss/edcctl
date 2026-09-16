# JSON API investigation

## Requirement and current status

The requirement is JSON **data queries**, with the existing login flow handled internally.
The earlier interpretation that authentication must also be JSON was incorrect and caused an unnecessary detour.

The CLI now queries the verified JSON calendar endpoint and removes HTML-based resource commands.
A full parent-portal JSON API is not verified. That does not block the schedule command.

## Actual EDC traffic capture

On 2026-09-16, a scoped mitmproxy capture recorded 27 completed requests after an EDC restart.
The capture intercepted only MobileInventor and Studio Pro domains.
The raw capture stays outside Git because responses contain account data and reusable credentials.

| Host | Method | Path | Parsed response | Count |
| --- | --- | --- | --- | --- |
| app.gostudiopro.com | GET | /online/index.php | HTML login form | 1 |
| app.gostudiopro.com | POST | /online/index.php | Script and redirect markup | 1 |
| app.gostudiopro.com | GET | /online/app_my_class_schedule.php | HTML schedule | 1 |
| mobileinventor.com | GET | /applib/getAppPage.php | HTML | 1 |
| mobileinventor.com | GET | /applib/getVersionInfo.php | JSON | 3 |
| mobileinventor.com | POST | /applib/AppProfileManager.php | JSON | 11 |
| mobileinventor.com | POST | /applib/appStat.php | JSON | 7 |
| mobileinventor.com | POST | /applib/getAppUserData.php | JSON | 1 |
| mobileinventor.com | POST | /applib/updateAppUserData.php | JSON | 1 |

Classification uses response-body JSON decoding, not the Content-Type header.
These services label several valid JSON responses as `text/html`.
The MobileInventor JSON responses contain app-profile, version, and tracking data.
They do not establish JSON endpoints for enrolled students, financial records, shared files, or portal messages.
This capture covers startup and automatic login, not every app screen.

### Capture correction

The first attempt enabled the macOS HTTP proxy but not the HTTPS proxy.
Direct TLS connections in that attempt did not prove that the app ignores system proxy settings.
The corrected HTTPS proxy captured EDC traffic without sudo, packet-filter changes, or a new certificate installation.
Both proxy modes are disabled after the capture.

## Additional evidence

- `appdata/js/danceStudioPro.js` on MobileInventor embeds Studio Pro portal pages and passes login data through postMessage.
- `pageLoader-v2.js` explains that `flutterWrapper.php` belongs to the Jackrabbit-specific `v5/library/jrabbit-new/` tree.
  It is not evidence of a Studio Pro Flutter API.
- `class_calendar-ajax.php`, with `action=getclasses`, returns JSON in authenticated tests.
  Internal CSRF login followed by this JSON query meets the clarified requirement for schedule data.
- `report_absence-ajax.php`, with `action=list_students`, returns checkbox markup, not student JSON.
- `bulletin_board-ajax.php`, with `action=show_com_log`, returns HTML message content.
- An indexed `api_classes.php` URL is not evidence of REST.
  The vendor's [live schedule guide](https://support.gostudiopro.com/hc/en-us/articles/4405468839565-How-do-I-show-Live-Schedule-of-my-classes-on-my-website)
  describes this integration as iframe or HTML embedding.
- Empty responses from `/api/`, `/api/v1/`, or `/apps/api/` do not identify a usable API contract.

## Implementation verification

A live CLI trace on 2026-09-16 confirms this request sequence:

1. GET `/online/index.php` to start login.
2. POST `/online/index.php` with the configured credentials and CSRF token.
3. POST `/online/class_calendar-ajax.php` for JSON events.

The CLI does not follow the login redirect to the HTML schedule page.
The verified date range returns 14 events as JSON.
Tests enforce this request sequence and reject HTML responses, null arrays, and invalid event dates without a fallback.

A separate Studio Pro Portal app is not required for schedule queries.
Further research is necessary only to add resource commands whose JSON endpoints remain unverified.
