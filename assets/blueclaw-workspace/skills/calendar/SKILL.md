---
name: calendar
description: Read or write the user's Google Calendar through typed InternKim Google Workspace capability tools.
when_to_use: Use when the user asks about calendar, schedule, meeting, 일정, 캘린더, or 회의 creation and lookup.
allowed-tools:
  - google.calendar.event
  - google.calendar.list
---

# Google Calendar

Use typed Google Workspace capability tools. Blueclaw must not read Google credentials and must not call Google CLI commands directly.

## Tools

### `google.calendar.event`

Create an event.

Required fields:

- `title`
- `start`
- `end`

Optional fields:

- `attendees`
- `description`
- `location`

Use RFC 3339 timestamps with timezone offsets, for example `2026-05-20T10:00:00+09:00`.

### `google.calendar.list`

Read events.

Optional fields:

- `start`
- `end`
- `limit`
- `query`

## Rules

- Do not use `gws`, service account wrappers, shell scripts, or credential files.
- Do not fabricate event URLs. Use the URL returned by the tool.
- If the tool reports missing credentials, tell the user Google Workspace credentials need to be installed through InternKim or Companion.
