# Changelog

## Unreleased

- Add `app info`, `app groups`, `app notifications --group ID`, and `app profile` through verified JSON endpoints.
- Keep portal authentication separate from MobileInventor requests.
- Reject private-group message access until membership can be verified.
- Filter sensitive and unknown app-response fields from command output.

- Query schedule data only through the verified JSON calendar endpoint.
- Keep CSRF login and session cookies internal, with the existing EDC credentials.
- Make `doctor` validate the JSON endpoint instead of an HTML page.
- Remove HTML-based students, balance, history, announcements, files, and account commands.
- Remove the HTML data parser and its dependencies.
- Reject non-JSON data responses, invalid event dates, and null arrays without a fallback.
- Test login, JSON-only data requests, date filtering, and failure cases with synthetic data.
