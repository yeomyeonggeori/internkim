---
name: create-gws-file
description: Create Google Docs, Sheets, or Gmail messages through typed InternKim Google Workspace capability tools. Use simple-slides for slide decks.
category: google-workspace
tags: [docs, sheets, gmail, drive]
triggerHints:
  - google doc
  - google docs
  - google sheet
  - google sheets
  - spreadsheet
  - gmail
  - 구글 문서
  - 구글 시트
  - 스프레드시트
  - 지메일
activation:
  keywords:
    - google doc
    - google docs
    - google sheet
    - google sheets
    - spreadsheet
    - gmail
    - 구글 문서
    - 구글 시트
    - 스프레드시트
    - 지메일
requiredTools:
  - google.docs.create
  - google.sheets.create
  - google.gmail.send
allowedProfiles: [default]
---

# Google Workspace Files

Use typed Google Workspace capability tools. Blueclaw does not read API keys, service-account JSON, OAuth exports, webhook URLs, or any other Google credential.

## Routing

- Google Docs: `google.docs.create`
- Google Sheets: `google.sheets.create`
- Gmail: `google.gmail.send`
- Calendar: use the `calendar` skill
- Slide decks or Google Slides: use the `simple-slides` skill

## Rules

- Do not use `gws`, service account wrappers, shell scripts, or credential files.
- Sharing and editing policy is handled by the capability provider.
- Do not claim a URL exists until the typed tool returns it.
- If a credential is missing, tell the user to install Google Workspace credentials through InternKim or Companion.

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
