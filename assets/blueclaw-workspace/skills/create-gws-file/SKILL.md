---
name: create-gws-file
description: "Create a Google Doc or Google Sheet, or send a Gmail message, as the user via the Apps Script bridge. Does NOT handle slide decks (use simple-slides) or calendar operations (use calendar)."
---

# Creating Google Workspace items as the user (Docs / Sheets / Gmail)

This skill uses the `gas-call` script in its own `scripts/` directory
to POST to an Apps Script web app the user pre-deployed. Every item
is created under the user's identity and lands in their Drive / Inbox.

## gas-call is a SHELL COMMAND, not a tool

The script lives at `skills/create-gws-file/scripts/gas-call`
(relative to the blueclaw workspace root). Call it through the
`shell` tool. These are wrong:

- `{"name":"gas-call", "arguments":{"title":"..."}}` — tool not found,
  silently fails, and anything you claim afterwards is fabricated.
- `{"name":"docs.create", ...}` — same.

Correct shape — one `shell` tool call whose `command` runs the script:

```json
{
  "name": "shell",
  "arguments": {
    "command": "skills/create-gws-file/scripts/gas-call docs.create title=\"2026 Q2 기획안\""
  }
}
```

Parse the stdout for the `{id, url, ...}` JSON. If you skipped the
shell wrapper, nothing actually ran on the board — do not fabricate a
URL. Retry with the correct invocation instead.

## Routing

If the user's ask isn't Docs / Sheets / Gmail — stop and switch skills:

- Slide decks / presentations / Google Slides → **simple-slides** skill
  (Marp → `.pptx` → `drive.import_pptx`)
- Calendar events (read or write) → **calendar** skill
- File uploads / imports to Drive → this skill's drive.import_* actions

## Actions

| action | required | optional | returns |
|--------|----------|----------|---------|
| `docs.create` | `title` | `share_to` | `{id, url}` |
| `sheets.create` | `title` | `share_to` | `{id, url}` |
| `gmail.send` | `to`, `subject`, `body` | `cc`, `bcc` | `{id}` |

`gas-call` auto-adds `share_to=<SA_EMAIL>` on `*.create` so the bot
can edit the item afterwards via `gws-bot`.

## Examples

```bash
skills/create-gws-file/scripts/gas-call docs.create title="2026 Q2 기획안"
skills/create-gws-file/scripts/gas-call sheets.create title="매출 대시보드"
skills/create-gws-file/scripts/gas-call gmail.send to="lee@example.com" subject="어제 회의 요약" body="..."
```

Each returns JSON. Parse `id` / `url` and hand the URL to the user.

## Follow-up edits — update in place, do not make a new file

When the user asks to modify content in a Doc / Sheet you already
created, update the existing file by its ID. Do NOT re-run
`docs.create` / `sheets.create` — that produces a second unrelated
file and forces the user to re-open a new URL.

1. Retrieve the existing file ID from the current conversation context or
   prior successful tool observations.
2. Use `gws-bot` to apply the edit in place. Because `gas-call` set
   `share_to`, the service account now has writer access:

```bash
gws-bot docs documents batchUpdate   --params '{"documentId":"<ID>"}'    --json '{"requests":[...]}'
gws-bot sheets spreadsheets batchUpdate --params '{"spreadsheetId":"<ID>"}' --json '{"requests":[...]}'
gws-bot sheets spreadsheets values update --params '{"spreadsheetId":"<ID>","range":"Sheet1!A1"}' --json '{"values":[["Hello"]]}'
```

Run `gws schema <service>.<resource>.batchUpdate` first if you're
unsure of the request shape — don't guess.

## Do NOT

- Don't send the URL to the user until the content is actually in the
  file. A successful batchUpdate returns `{"replies": [...]}` with no
  `error` field.
- Don't use `curl` with `$(cat ...)` substitution — shell policy blocks
  it. Always go through the `gas-call` script.
- Don't reach for plain `gws` — it has no credentials. Use `gws-bot`.
- Don't create slide decks here. Use `simple-slides` even if the user
  said "Google Slides".
