---
name: scheduled-task
description: Create or cancel scheduled, recurring, and finite repeated reminders, messages, reports, and follow-up tasks through Blueclaw schedule capability operations.
when_to_use: Use when the user asks to schedule, remind, repeat, send something every minute, hour, day, week, or month, repeat N times, cancel schedules, stop reminders, or says 예약, 알림, 리마인드, 취소, 중지, 마다, 분마다, 시간마다, 한 번씩, 1분에 한 번씩, 10번, 매일, 매주, or 매월.
---

# Scheduled Task

Use `capability.invoke` with `operation: schedule.create` when the user asks the assistant to create a reminder, recurring message, periodic report, future follow-up, finite repeated message, or timed automation. Use `capability.invoke` with `operation: schedule.cancel` when the user asks to cancel, stop, clear, or remove scheduled tasks or pending waits.

Creating a schedule is bounded work. Do not reject these requests as unsupported background loops when the schedule capability is available.

Every JSON block below is the `input` object for that `capability.invoke` call, for example:

```json
{"operation": "schedule.create", "input": {"taskInstruction": "업계 뉴스를 조사해서 핵심만 보고해준다.", "kind": "cron", "cronExpression": "0 9 * * *", "timeZone": "Asia/Seoul"}}
```

`input` must be a real object with the fields filled in — never an empty object, an empty string, or a placeholder like `{}`.

## Task Instruction

Every schedule stores one `taskInstruction`. Put only the work to perform at run time in `taskInstruction`.

Do not copy the user's full scheduling request into `taskInstruction`. Cadence and stop conditions belong only in structured schedule fields such as `runAt`, `intervalSecond`, `cronExpression`, `expiresAt`, and `maxRunCount`.

The scheduled agent run can use approved delivery operations later without asking for approval again.

## Workflow

1. If the user asks to cancel schedules or pending waits, use `capability.invoke` with `operation: schedule.cancel` and do not ask for approval.
2. Write `taskInstruction` as the action to perform when the schedule fires.
3. Choose `kind: interval` for simple repeats like every minute or every hour.
4. Choose `kind: cron` for calendar-like schedules such as every day at 9 AM or every Monday.
5. Set `timeZone` from user context when known. Use `Asia/Seoul` for Korean-language local-time requests when no better timezone is available.
6. Set `maxRunCount` when the user asks for a finite count such as 10 times, 10번, or repeat N times.
7. Set `expiresAt` when the user gives an end time such as today 18:00 or until tomorrow.
8. Use `capability.invoke` with `operation: schedule.create`.
9. Reply with what was scheduled and when it will run or stop.

## Cancellation

For "내가 건 모든 예약 다 취소해줘", use:

```json
{
  "scope": "mine"
}
```

For "이 스레드 예약 취소해줘", use:

```json
{
  "scope": "currentConversation"
}
```

For an explicit known schedule ID, use:

```json
{
  "scope": "scheduleIDs",
  "scheduleIDs": ["<taskScheduleID>"]
}
```

After cancellation, reply briefly with the number of schedules or waits cancelled. Do not request approval for cancellation.

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
  "taskInstruction": "현재 대화에 \"죄송합니다\"라고 보낸다.",
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
  "taskInstruction": "업계 뉴스를 조사해서 핵심만 보고해준다.",
  "kind": "cron",
  "cronExpression": "0 9 * * *",
  "timeZone": "Asia/Seoul"
}
```

For "1분 뒤 샘플에게 테스트라고 보내줘" use:

```json
{
  "taskInstruction": "샘플 님에게 \"테스트\"라고 DM으로 보낸다.",
  "kind": "once",
  "runAt": "<RFC3339 timestamp one minute from now>",
  "timeZone": "Asia/Seoul"
}
```

Do not claim the recurring delivery has already happened. The scheduler will run the saved task later.
