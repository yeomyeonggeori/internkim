---
name: direct-message
description: Send or schedule direct messages to approved InternKim people through platform.dm.send.
when_to_use: Use when the user asks InternKim to send, DM, message, tell, or notify a named person such as 샘플에게 보내줘, DM 보내, 메시지 보내, 알려줘, 전달해줘, or says to send something to a person rather than the current conversation.
allowed-tools:
  - approval.request
  - platform.dm.send
  - schedule.create
---

# Direct Message

Use `platform.dm.send` to send a direct message to a named approved InternKim person.

The tool only supports approved InternKim people with Mattermost accounts. Do not claim that contacts cannot be found before trying the tool when the user names a person.

## Approval

Immediate direct messages require approval. First call `approval.request` with a clear message naming the recipient and exact text. After the user approves, call `platform.dm.send`.

Scheduled direct messages do not require approval at run time. For a future or recurring message to a named person, call `schedule.create` with `executionMode: "agent"` and make the prompt a direct future instruction to use DM delivery.

## Workflow

1. Identify the recipient hint from the user's wording.
2. Identify the exact message to send.
3. If this is an immediate send, call `approval.request`.
4. If the latest task context says the user approved the pending action, call `platform.dm.send`.
5. If this is scheduled for later, call `schedule.create` with `executionMode: "agent"`.

For immediate "샘플에게 테스트라고 보내줘", first use:

```json
{
  "message": "샘플 님에게 다음 DM을 보내도 될까요?\n\n테스트",
  "reason": "Direct messages are external sends and require approval before immediate delivery."
}
```

After approval, use:

```json
{
  "recipientHint": "샘플",
  "message": "테스트",
  "platform": "mattermost"
}
```

For scheduled "1분 뒤 샘플에게 테스트라고 보내줘", use:

```json
{
  "prompt": "샘플 님에게 \"테스트\"라고 DM으로 보내세요.",
  "executionMode": "agent",
  "kind": "once",
  "runAt": "<RFC3339 timestamp one minute from now>",
  "timeZone": "Asia/Seoul"
}
```

For current-conversation reminders like "나한테 1분마다 죄송합니다 2번 해봐", use the scheduled-task workflow with `executionMode: "message"` instead of this direct-message workflow.
