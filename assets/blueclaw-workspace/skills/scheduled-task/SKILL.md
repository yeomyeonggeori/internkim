---
name: scheduled-task
description: Create scheduled, recurring, and finite repeated reminders, messages, reports, and follow-up tasks through Blueclaw schedule.create.
when_to_use: Use when the user asks to schedule, remind, repeat, send something every minute, hour, day, week, or month, repeat N times, send a finite repeated message, or says 예약, 알림, 리마인드, 마다, 분마다, 시간마다, 한 번씩, 1분에 한 번씩, 10번, 매일, 매주, or 매월.
allowed-tools:
  - schedule.create
completion:
  requiredEvidenceTools:
    - schedule.create
---

# Scheduled Task

Use `schedule.create` when the user asks InternKim to create a reminder, recurring message, periodic report, future follow-up, finite repeated message, or timed automation.

Creating a schedule is bounded work. Do not reject these requests as unsupported background loops when `schedule.create` is available.

## Execution Mode

Use `executionMode: "message"` when the scheduled run should send a fixed message, reminder, or quoted text to the current conversation. In this mode, `prompt` must be the exact user-visible message body to deliver later. Do not include command verbs such as "send", "tell", "remind", "알려줘", "말해줘", or "보내기" unless those words are part of the message itself.

Use `executionMode: "agent"` when the scheduled run must do work at run time, such as research, checking state, summarizing, using tools, deciding what to say based on future information, or sending a direct message to a named person.

`schedule.create` with `executionMode: "message"` delivers to the current reply target automatically. Do not try to look up contacts, set a different recipient, request approval, or claim direct messaging tools are unavailable for fixed reminders to the current conversation.

If the user asks to send a scheduled message to a named approved person, use `executionMode: "agent"` and make `prompt` an explicit future instruction such as `동하 님에게 "테스트"라고 DM으로 보내세요.` The scheduled agent run can use `platform.dm.send` later without asking for approval again.

## Workflow

1. Decide whether the scheduled run is a fixed message to the current conversation (`message`) or future work / named-person DM (`agent`).
2. Choose `kind: interval` for simple repeats like every minute or every hour.
3. Choose `kind: cron` for calendar-like schedules such as every day at 9 AM or every Monday.
4. Set `timeZone` from user context when known. Use `Asia/Seoul` for Korean-language local-time requests when no better timezone is available.
5. Set `maxRunCount` when the user asks for a finite count such as 10 times, 10번, or repeat N times.
6. For fixed messages to the current conversation, set `executionMode: "message"` and make `prompt` the exact message to deliver.
7. For future work or named-person DM, set `executionMode: "agent"` and make `prompt` the task instruction.
8. Call `schedule.create`.
9. Reply with what was scheduled and when it will run.

For "1분마다" use:

```json
{
  "kind": "interval",
  "intervalSecond": 60
}
```

For "1분에 한 번씩 나한테 죄송합니다 10번 해봐" use:

```json
{
  "prompt": "죄송합니다",
  "executionMode": "message",
  "kind": "interval",
  "intervalSecond": 60,
  "maxRunCount": 10,
  "timeZone": "Asia/Seoul"
}
```

For "매일 오전 9시" use a cron expression for 9 AM in the chosen timezone.

For "매일 오전 9시에 업계 뉴스를 조사해서 알려줘" use:

```json
{
  "prompt": "업계 뉴스를 조사해서 핵심만 보고해줘.",
  "executionMode": "agent",
  "kind": "cron",
  "cronExpression": "0 9 * * *",
  "timeZone": "Asia/Seoul"
}
```

For "1분 뒤 동하에게 테스트라고 보내줘" use:

```json
{
  "prompt": "동하 님에게 \"테스트\"라고 DM으로 보내세요.",
  "executionMode": "agent",
  "kind": "once",
  "runAt": "<RFC3339 timestamp one minute from now>",
  "timeZone": "Asia/Seoul"
}
```

Do not claim the recurring delivery has already happened. The scheduler will run the saved task later.
