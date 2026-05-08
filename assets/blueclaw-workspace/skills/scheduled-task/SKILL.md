---
name: scheduled-task
description: Create recurring reminders, periodic reports, and future follow-up tasks through Blueclaw schedule.create.
when_to_use: Use when the user asks to schedule, remind, repeat, send something every minute, hour, day, week, or month, or says 예약, 알림, 리마인드, 마다, 분마다, 시간마다, 매일, 매주, or 매월.
allowed-tools:
  - schedule.create
---

# Scheduled Task

Use `schedule.create` when the user asks InternKim to create a reminder, recurring message, periodic report, future follow-up, or timed automation.

Creating a schedule is bounded work. Do not reject these requests as unsupported background loops when `schedule.create` is available.

## Workflow

1. Convert the user's request into the future task instruction.
2. Choose `kind: interval` for simple repeats like every minute or every hour.
3. Choose `kind: cron` for calendar-like schedules such as every day at 9 AM or every Monday.
4. Set `timeZone` from user context when known. Use `Asia/Seoul` for Korean-language local-time requests when no better timezone is available.
5. Call `schedule.create`.
6. Reply with what was scheduled and when it will run.

For "1분마다" use:

```json
{
  "kind": "interval",
  "intervalSecond": 60
}
```

For "매일 오전 9시" use a cron expression for 9 AM in the chosen timezone.

Do not claim the recurring delivery has already happened. The scheduler will run the saved task later.
