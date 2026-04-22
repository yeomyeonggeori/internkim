---
name: calendar
description: "Read or write the user's Google Calendar as the user, via the Apps Script bridge. Use for 'what's on my calendar this week', '다음주 금요일 미팅 잡아줘', '이번 달 일정 보여줘', etc. Do NOT use gws calendar — that runs as the service account which has zero calendars and will return empty, misleading you into saying the user's calendar is empty when it is not."
---

# Google Calendar (read + write as the user)

All calendar operations go through this skill's `scripts/gas-call`
script, which POSTs to the user's Apps Script web app. That script
runs under the user's identity, so it sees the user's own events.
The service-account path (`gws calendar ...` / `gws-bot calendar ...`)
sees nothing and must not be used for calendar work.

The `gas-call` script lives at
`skills/calendar/scripts/gas-call` (relative to the zeroclaw
workspace root). Call it through the `shell` tool. A tool call shaped
like `{"name":"gas-call", "arguments":{...}}` is a tool-not-found —
zeroclaw rejects it, nothing runs on the board, and any URL you
report afterwards is fabricated. Same for
`{"name":"calendar.event", ...}`.

Correct invocation:

```json
{
  "name": "shell",
  "arguments": {
    "command": "skills/calendar/scripts/gas-call calendar.event title=\"후쿠오카 여행\" start=\"2026-05-20T00:00:00+09:00\" end=\"2026-05-27T23:59:59+09:00\""
  }
}
```

Parse stdout for the JSON response.

## Actions

### calendar.event — create

| parameter | required | notes |
|---|---|---|
| `title` | ✓ | event title |
| `start` | ✓ | RFC 3339, e.g. `2026-05-20T00:00:00+09:00` |
| `end` | ✓ | RFC 3339 |
| `attendees` | ✗ | comma-separated emails |
| `description` | ✗ | long text |
| `location` | ✗ | place string |

Returns `{"id":"<event-id>","url":"https://www.google.com/calendar/event?eid=..."}`.
Share the `url` with the user so they can open it in their Google
Calendar UI.

```bash
skills/calendar/scripts/gas-call calendar.event title="팀 스탠드업" \
  start="2026-04-23T10:00:00+09:00" \
  end="2026-04-23T10:15:00+09:00" \
  attendees="a@x.com,b@x.com" \
  location="Meet: https://meet.google.com/abc"
```

### calendar.list — read

| parameter | required | notes |
|---|---|---|
| `start` | ✗ | RFC 3339 (default: now) |
| `end` | ✗ | RFC 3339 (default: now + 7 days) |
| `limit` | ✗ | max events returned (default 50) |

Returns `{"calendarId":"...","range":{"start","end"},"events":[{"id","title","start","end","location","description","allDay"}]}`.

Use this, NOT `gws calendar events list`, when the user asks to see
their schedule. Examples:

```bash
# 이번 주 일정
skills/calendar/scripts/gas-call calendar.list

# 5월 20~27일 사이 일정
skills/calendar/scripts/gas-call calendar.list start="2026-05-20T00:00:00+09:00" end="2026-05-28T00:00:00+09:00"

# 다음 30일 전부
skills/calendar/scripts/gas-call calendar.list end="2026-05-21T00:00:00+09:00" limit=200
```

## Do NOT

- Do not call `gws calendar` / `gws-bot calendar` for user-calendar
  queries. SA has no calendars; it will always return empty `items`
  and mislead you into telling the user their calendar is empty.
- Do not invoke `gas-call`, `calendar.event`, or `calendar.list` as
  tool names. They are strings you pass inside a `shell` command.
- Do not fabricate event URLs. Use the `url` field that
  `calendar.event` actually returned.
- Do not tell the user "권한이 없어서 못 봤어요" when the real issue
  is that you queried the wrong identity path. Switch to `gas-call`.
