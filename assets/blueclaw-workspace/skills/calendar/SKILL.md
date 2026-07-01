---
name: calendar
description: Read or write the workspace calendar with calendar capability operations. Use this whenever the user asks to add, find, update, cancel, delete, or check meetings, schedules, 일정, 캘린더, 미팅, 회의, 약속, or reminders, even if they do not explicitly say "calendar."
when_to_use: Use when the user asks about calendar, schedule, meeting, 일정, 캘린더, 미팅, 회의, 약속, or reminders.
---

# Workspace Calendar

Use the workspace calendar capability operations for team schedule creation and lookup. Run these operations through the `capability.invoke` tool: set `operation` to the operation name and `input` to its parameters. The runtime supplies requester identity and approval. The Work calendar is exposed through CalDAV and ICS, so Google Calendar, Apple Calendar, iPhone, and Mac clients can subscribe or sync without making Google Calendar the default write path.

## Operations

### `calendar.add`

Create an event. This does not require approval.

Required fields:

- `title`
- `startISO`
- `endISO`

Optional fields:

- `description`
- `location`
- `timeZone`
- `isAllDay`
- `color`
- `people`
- `reminderLeadHours`

Use RFC 3339 timestamps with timezone offsets, for example `2026-05-20T10:00:00+09:00`.

### `calendar.list`

Read events. This does not require approval.

Optional fields:

- `startISO`
- `endISO`
- `query`
- `limit`
- `query`

Use both `startISO` and `endISO` together when narrowing a date range.

### `calendar.update`

Update an existing event. This does not require approval. List first if the user has not provided an `eventID`.

Required fields:

- `eventID`
- `title`
- `startISO`
- `endISO`

Optional fields are the same as `calendar.add`.

### `calendar.delete`

Delete an event by `eventID`. This requires approval. List matching events first when the user refers to an event by title or time.

## Rules

- Prefer the Work calendar operations over Google Workspace operations for ordinary schedule requests.
- Decide by the user's intent, not by the noun they used. A meeting, appointment, attendance block, location-based visit, or time block is a calendar event. A deliverable, deadline, todo, request, handoff, or completion target is work.
- If the user asks to add a schedule but the content is a deadline-driven deliverable, create the work item with `task.add` and also create a calendar deadline/reminder when a due time is given.
- If the user says a task-like item is complete, use `task.update` before considering `calendar.update`.
- Do not mark calendar events with `[완료]` for task-like completion. Update or delete calendar events only when the user clearly asks to change, cancel, delete, or reschedule a calendar event.
- Do not ask for approval before `calendar.add`, `calendar.list`, or `calendar.update`.
- Ask for approval before `calendar.delete`.
- If the user gives a relative date like "tomorrow" or "next Friday", resolve it using the runtime temporal context before invoking an operation.
- If the date, time, or duration is ambiguous, ask one concise question before writing.
- For all-day events, set `isAllDay: true`; use `startISO` at the start date and `endISO` at the next day boundary.
- Put targeted people in `people` as comma-separated nicknames or an array. The calendar stores them as the first note line.
- If the event is for everyone, omit `people`; the backend will notify the `announcements` channel.
- Choose `reminderLeadHours` from `1, 2, 3, 6, 12, 24, 48`.
- Use `48` for overseas travel, long trips, or events needing two-day preparation.
- Use `24` for domestic travel to another city or when uncertain.
- Use `12` or `6` for external meetings or half-day preparation.
- Use `1` for same-day internal online meetings.
- Do not say an event was created, changed, or deleted until the operation succeeds.
- To update or delete an event the user names instead of giving an `eventID`, call `calendar.update` or `calendar.delete` directly with `query` set to a distinctive keyword from the event (a person or topic name). The runtime resolves the event across all dates and fails if the query matches zero or several events — you do not need a separate `calendar.list` step. Only react to the operation's own result: if it reports several candidates, ask which one; if it reports none, tell the user no matching event exists.
- External attendee invitation is not supported by the Work calendar operation yet. If the user asks to invite people, create the event with attendee names in the description and mention that CalDAV clients can add invitations after sync.
- Google Calendar integration is via the Work calendar's CalDAV/ICS sync URL. Do not use Google credential files, shell scripts, or Google-only operations unless the user explicitly asks for a Google Workspace bridge and that operation is available.
