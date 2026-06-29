---
name: direct-message
description: Send or schedule direct messages to approved workspace people through message.send.
when_to_use: Use when the user asks the assistant to send, DM, message, tell, or notify a named person such as 샘플에게 보내줘, DM 보내, 메시지 보내, 알려줘, 전달해줘, or says to send something to a person rather than the current conversation. Do not use when the target is a channel such as 광장 채널, town-square, off-topic, or another Mattermost channel.
completion:
  requiredEvidenceTools:
    - message.send
---

# Direct Message

Use `capability.invoke` with `operation: message.send` and `recipientHint` in the input to send a direct message to a named approved workspace person.

The task is complete after one successful `message.send` observation. Do not send another direct message in the same task after `message.send` succeeds; use that successful observation as completion evidence and reply.

The operation only supports approved workspace people with Mattermost accounts. Do not claim that contacts cannot be found before trying the operation when the user names a person.

## Approval

Immediate direct messages to someone else require confirmation. First call `ask.confirm` with a message naming the recipient and exact text in the same language as the original user request. After the user approves, use `capability.invoke` with `operation: message.send`.

Immediate direct messages to the requester themselves do not require approval.

Scheduled direct messages do not require approval at run time. For a future or recurring message to a named person, use `capability.invoke` with `operation: schedule.create` and make `taskInstruction` the direct future instruction to use DM delivery.

## Workflow

1. Identify the recipient hint from the user's wording.
2. Identify the exact message to send.
3. If this is an immediate send to someone else, call `ask.confirm`.
4. If the latest task context says the user approved the pending action, use `capability.invoke` with `operation: message.send`.
5. If this is scheduled for later, use `capability.invoke` with `operation: schedule.create` with a `taskInstruction` that names the recipient and message.

If `message.send` reports that a recipient was not found or ambiguous, say that the recipient could not be resolved and ask for the missing distinction when needed. Do not combine that with approval language; approval is separate from recipient resolution.

For immediate "샘플에게 테스트라고 보내줘", first use:

```json
{
  "userFacingMessage": "샘플 님에게 다음 DM을 보내도 될까요?\n\n테스트",
  "reasonCode": "external_send"
}
```

After approval, use:

```json
{
  "recipientHint": "샘플",
  "message": "테스트"
}
```

For scheduled "1분 뒤 샘플에게 테스트라고 보내줘", use:

```json
{
  "taskInstruction": "샘플 님에게 \"테스트\"라고 DM으로 보낸다.",
  "kind": "once",
  "runAt": "<RFC3339 timestamp one minute from now>",
  "timeZone": "Asia/Seoul"
}
```

For current-conversation reminders like "나한테 1분마다 죄송합니다 2번 해봐", use the scheduled-task workflow with `taskInstruction` such as `현재 대화에 "죄송합니다"라고 보낸다.`
