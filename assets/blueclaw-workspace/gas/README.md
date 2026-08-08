# internkim-bridge (Google Apps Script)

Google Apps Script webhook the user deploys from their own Google account.
internkim stores the returned Web App URL in root-owned secret storage and
invokes it through typed capabilityd tools. Blueclaw never reads this URL.

## One-time deploy (~30 s)

1. Open <https://script.google.com/home/start> and click **New project**.
2. Paste the contents of `Code.gs` into the editor. Save.
3. Click **Deploy → New deployment → Web app**.
   - *Execute as*: Me
   - *Who has access*: Only myself
4. Authorize the prompted scopes (Drive / Slides / Docs / Sheets / Calendar /
   Gmail). Google's native consent screen is never blocked.
5. Copy the **Web App URL** Apps Script returns after deploying.
6. Install the Web App URL through internkim setup or Companion.

## Actions

All invoked via `POST` with `application/x-www-form-urlencoded` body.

| `action`          | Required params            | Optional params        | Returns                        |
|-------------------|----------------------------|------------------------|--------------------------------|
| `slides.create`   | `title`                    |                        | `{id, url}`                    |
| `docs.create`     | `title`                    | `body`                 | `{id, url}`                    |
| `sheets.create`   | `title`                    | `values`               | `{id, url}`                    |
| `calendar.event`  | `title`, `start`, `end`    | `attendees` (CSV)      | `{id}`                         |
| `gmail.send`      | `to`, `subject`            | `body`                 | `{sent: true}`                 |

## Security

- Only the user who deployed the script has access — *"Only myself"* locks the
  webhook URL to their identity check.
- Share the Web App URL treated as a secret. Anyone who has it can make the
  deploying user create Drive files or send email on their behalf.
