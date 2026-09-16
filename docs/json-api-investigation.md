# JSON API investigation

## Requirement and current status

The required replacement must use JSON endpoints, with no HTML parsing, including authentication.
The current CLI does not meet that requirement. This investigation does not change its implementation.

A full parent-portal JSON API is **not verified**. This is not evidence that no such API exists.
Do not replace the current client with guessed endpoint names or describe HTML-fragment AJAX as a REST API.

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
- `class_calendar-ajax.php`, with `action=getclasses`, returns JSON in the earlier authenticated tests.
  Authentication still depends on the HTML login flow, so this alone does not meet the requirement.
- `report_absence-ajax.php`, with `action=list_students`, returns checkbox markup, not student JSON.
- `bulletin_board-ajax.php`, with `action=show_com_log`, returns HTML message content.
- An indexed `api_classes.php` URL is not evidence of REST.
  The vendor's [live schedule guide](https://support.gostudiopro.com/hc/en-us/articles/4405468839565-How-do-I-show-Live-Schedule-of-my-classes-on-my-website)
  describes this integration as iframe or HTML embedding.
- Empty responses from `/api/`, `/api/v1/`, or `/apps/api/` do not identify a usable API contract.

## Remaining evidence needed

Studio Pro publishes a separate [Studio Pro Portal app](https://apps.apple.com/us/app/studio-pro-portal/id1618298488).
Its iOS bundle ID is `com.dancestudio-pro.parents`; its Android package is `com.dancestudiopro.parents`.
Its transport is not yet inspected. It may use another API, or it may embed the same web portal.

An authorized copy of that app's IPA or APK, or a scoped capture from it, is the next useful artifact.
The App Store lookup's supported-device list does not by itself determine Apple Silicon Mac availability.
Do not request or store the user's Apple ID password to obtain the artifact.

## Replacement gate

Before a JSON-only rewrite, verify:

1. Authentication without parsing HTML.
2. Account and studio scoping under the existing parent credentials.
3. Real JSON responses for each promised resource command.
4. Authentication failure, session expiry, and empty-list behavior.

Remove unsupported commands or document the gap explicitly. Do not retain an HTML fallback.
