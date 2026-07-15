---
name: create-gws-file
description: Create Google Docs, Sheets, or Gmail messages through typed Google Workspace capability operations for Google Docs, Google Sheets, spreadsheets, Gmail, 구글 문서, 구글 시트, 스프레드시트, or 지메일 requests. Use presentation for slide decks.
---

# Google Workspace Files

Use typed Google Workspace capabilities through the `capability.invoke` tool: set `operation` to the operation name and `input` to its parameters. Blueclaw does not read API keys, service-account JSON, OAuth exports, webhook URLs, or any other Google credential.

## Routing

- Google Docs: `google.docs.create`
- Google Sheets: `google.sheets.create`
- Gmail: `google.gmail.send`
- Calendar: use the `calendar` skill
- Slide decks or Google Slides: use the `presentation` skill

## Rules

- Do not use `gws`, service account wrappers, shell scripts, or credential files.
- Sharing and editing policy is handled by the capability provider.
- Do not claim a URL exists until the typed operation returns it.
- If a credential is missing, tell the user to install Google Workspace credentials through the setup flow or Companion.

## Tool Inputs

### `google.docs.create`

Required:

- `title`

Optional:

- `body`

### `google.sheets.create`

Required:

- `title`

Optional:

- `sheets`
- `values`

### `google.gmail.send`

Required:

- `to`
- `subject`
- `body`

Optional:

- `cc`
- `bcc`
