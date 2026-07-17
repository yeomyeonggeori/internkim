---
name: internkim-flow
description: Add, find, update, or complete weekly work items when the user asks to add, record, request, find, change, or complete work, todos, 업무, deadlines, or task notes.
tool-references: task.add task.list task.update task.delete
---

# 업무 관리

Use the typed work capability operations for work items; descriptors define fields and results, and the runtime supplies identity and approval. In Korean replies, do not call the product `Flow` unless the user explicitly uses that English name.

## Routing and workflow

- Decide by intent, not by the noun used. Deliverables, deadlines, todos, requests, handoffs, and completion targets are work. Meetings, appointments, attendance blocks, locations, and time blocks are calendar events.
- A deadline-driven deliverable uses `task.add` and, when a due time belongs on the calendar, `calendar.add` too. Completion uses `task.update`, not `calendar.update`.
- Add work directly only to the requester's own list. Work for another person is a `요청`; use `targetPersonHint` only when the person is explicit. Include the requester as a participant only when joint work is implied.
- Use a concise title and add goal, size, status, start date, or end date only when supported. Select size by the established XS–XXL scale: XS is a tiny change, S small, M day-sized, L multi-day, XL complex, and XXL a milestone that must be split.
- Use `task.list` before `task.update` or `task.delete`; copy the exact `taskID` from the result. When a title is supplied instead of an ID, search with `query` and any supported person or week hints; ask when several tasks match.
- Update with at least one mutable field; completion sets status to `완료`. Delete only on explicit request and with the observed ID. Approval authorizes deletion but does not identify the target.
- Current-week listing is the default. Use week offsets for another period, including `weekFrom -1, weekTo -1` for last week, `weekFrom -3, weekTo 0` for the last four weeks, and a wide range such as `weekFrom -520` for history.

## Replies and failures

Report successful created, updated, or listed work in a compact Markdown table, including status, category, kind, size, participants, content, goal, and week code. Use participant presentation data in the order mention, display name, then participant name; never invent an @mention. If an owner is ambiguous, show the returned handle candidates; if an operation fails, explain it honestly and do not fabricate a task.
