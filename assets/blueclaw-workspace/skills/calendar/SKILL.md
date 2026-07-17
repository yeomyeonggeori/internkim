---
name: calendar
description: Read or write the workspace calendar with calendar capability operations. Use this whenever the user asks to add, find, update, cancel, delete, or check meetings, schedules, 일정, 캘린더, 미팅, 회의, 약속, or reminders, even if they do not explicitly say "calendar."
tool-references: calendar.add calendar.list calendar.update calendar.delete
---

# Workspace Calendar

Call the typed calendar operations directly; their descriptors define fields and results. The Work calendar is exposed through CalDAV and ICS, so clients can subscribe or sync without making Google Calendar the default write path.

## Rules

- Prefer Work calendar operations over Google Workspace operations for ordinary schedule requests.
- Decide by the user's intent, not by the noun they used. Meetings, appointments, attendance blocks, visits, and time blocks are calendar events; deliverables, deadlines, todos, requests, handoffs, and completion targets are work.
- For a deadline-driven deliverable with a due time, call `task.add` and also `calendar.add` for the deadline or reminder.
- For completion of a task-like item, use `task.update`. Do not mark calendar events with `[완료]`.
- Do not ask approval before `calendar.add`, `calendar.list`, or `calendar.update`; ask before `calendar.delete`.
- Resolve relative dates from runtime temporal context. Ask one concise question when date, time, or duration is ambiguous.
- For all-day events, set `isAllDay` and use the next-day boundary for the end. Put attendee hints in `people` using only names, @handles, or emails supplied by the user; the requester is included by default unless the entry is delegated or an announcement.
- Use `people` as `["전체"]` or `["@all"]` for everyone. Choose reminder leads from 1, 2, 3, 6, 12, 24, or 48 hours according to preparation needs.
- When a named event lacks an ID, call update or delete with distinctive `query`; let the operation report none or multiple candidates. Do not claim success until it succeeds.
- Report created, updated, or listed events in a compact Markdown table. External attendee invitations are not supported; mention that CalDAV clients can add them after sync.
