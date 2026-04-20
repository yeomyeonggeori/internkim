# internkim-bridge (Google Apps Script)

Google Apps Script webhook the user deploys from their own Google account so
the on-device agent can create Slides / Docs / Sheets / Calendar events / send
Gmail *as the user*, bypassing the "unverified third-party app" block that
kills direct OAuth flows on personal gmail accounts.

## One-time deploy (~30 s)

1. Open <https://script.google.com/home/start> and click **New project**.
2. Paste the contents of `Code.gs` into the editor. Save.
3. Click **Deploy → New deployment → Web app**.
   - *Execute as*: Me
   - *Who has access*: Only myself
4. Authorize the prompted scopes (Drive / Slides / Docs / Sheets / Calendar /
   Gmail). Google's native consent screen is never blocked.
5. Copy the **Web App URL** Apps Script returns after deploying.
6. Paste it back into the internkim terminal prompt when setup asks.

## Actions

All invoked via `POST` with `application/x-www-form-urlencoded` body.

| `action`          | Required params            | Optional params        | Returns                        |
|-------------------|----------------------------|------------------------|--------------------------------|
| `slides.create`   | `title`                    | `share_to` (SA email)  | `{id, url}`                    |
| `docs.create`     | `title`                    | `share_to`             | `{id, url}`                    |
| `sheets.create`   | `title`                    | `share_to`             | `{id, url}`                    |
| `calendar.event`  | `title`, `start`, `end`    | `attendees` (CSV)      | `{id}`                         |
| `gmail.send`      | `to`, `subject`            | `body`                 | `{sent: true}`                 |

## Security

- Only the user who deployed the script has access — *"Only myself"* locks the
  webhook URL to their identity check.
- Share the Web App URL treated as a secret. Anyone who has it can make the
  deploying user create Drive files or send email on their behalf.
