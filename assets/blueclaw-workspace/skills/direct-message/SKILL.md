---
name: direct-message
description: Send or schedule direct messages to approved workspace people through message.send.
allowed-tools: message.send schedule.create
---

# Direct Message

Call `message.send` directly with `targetType: directMessage`, `personHint`, and `message` to send a direct message to a named approved workspace person.

The task is complete after one successful `message.send` observation. Do not send another direct message in the same task after `message.send` succeeds; use that successful observation as completion evidence and reply.

The operation only supports approved workspace people with Mattermost accounts. Do not claim that contacts cannot be found before trying the operation when the user names a person.

## Approval

The runtime asks the user to confirm a direct message to someone else automatically before it is sent; do not call `ask.confirm` yourself. Just call `message.send`. After the user approves, the same call runs and the message is delivered. Messages to the requester themselves are sent without confirmation.

Scheduled direct messages do not require approval at run time. For a future or recurring message to a named person, call `schedule.create` directly and make `taskInstruction` the direct future instruction to use DM delivery.

## Workflow

1. Identify the recipient hint from the user's wording.
2. Identify the exact message to send.
3. For an immediate message, call `message.send`. The runtime handles confirmation when the recipient is someone other than the requester.
4. If this is scheduled for later, call `schedule.create` with a `taskInstruction` that names the recipient and message.

If `message.send` reports that a recipient was not found or ambiguous, say that the recipient could not be resolved and ask for the missing distinction when needed. Do not combine that with approval language; approval is separate from recipient resolution.

For immediate "동하에게 테스트라고 보내줘", use:

```json
{
  "targetType": "directMessage",
  "personHint": "동하",
  "message": "테스트"
}
```

For scheduled "1분 뒤 동하에게 테스트라고 보내줘", use:

```json
{
  "taskInstruction": "동하 님에게 \"테스트\"라고 DM으로 보낸다.",
  "kind": "once",
  "runAt": "<RFC3339 timestamp one minute from now>",
  "timeZone": "Asia/Seoul"
}
```

For current-conversation reminders like "나한테 1분마다 죄송합니다 2번 해봐", use the scheduled-task workflow with `taskInstruction` such as `현재 대화에 "죄송합니다"라고 보낸다.`
