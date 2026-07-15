---
name: mattermost
description: Work with Mattermost conversations, posts, threads, attachments, channels, pins, and assistant bot messages when the user is talking with the assistant inside Mattermost.
---

# Mattermost

Use platform message capabilities for Mattermost messaging. Run these operations through the `capability.invoke` tool: set `operation` to the operation name and `input` to its parameters. The operations are `message.context`, `message.search`, `message.send`, `message.update`, `message.delete`, and `channel.update`.

If the current conversation is Mattermost, use this skill when the user asks about messages in the current thread, current channel, a DM, a named channel, or bot messages that the assistant already sent.

## Read

Use `message.context` when you need the current channel, thread, DM, post, bot user, or requester context.

Use `message.search` when the user refers to messages without exact IDs. Pass `queries` as a string array; one query is `["keyword"]`, and multiple queries are OR-matched.

Search scopes:

- `currentThread`: only the current thread.
- `currentChannel`: the current channel or DM channel.
- `directMessage`: a DM with `personHint`, or the current DM when no person is needed.
- `channel`: a named channel with `channelID` or `channelName`.

Use `authoredBy: "assistant"` when the user asks about messages assistant sent. Use the returned previews to decide which messages match the user's intent. Do not delete from a natural-language description without first finding candidate message IDs.

## Post

Use `capability.invoke` with `operation: message.send` to send a DM, reply to the current thread, post to the current channel, or post to a named channel. Ask for confirmation first with `ask.confirm`, then invoke only after approval.

Use `pin: true` when the user asks to post and pin the message in the same request. Do not call `message.update` again after a pinned post succeeds.

## Update Or Delete Posts

Use `capability.invoke` with `operation: message.update` to edit an assistant bot message or pin/unpin a message. Ask for confirmation first with `ask.confirm`.

Set `isPinned: true` to pin, `isPinned: false` to unpin, and omit `isPinned` when only changing message text.

Use `capability.invoke` with `operation: message.delete` to delete one or more assistant bot messages. Ask for confirmation first with `ask.confirm`.

For precise deletion, pass all selected IDs in `messageIDs`, even when deleting a single message:

```json
{"messageIDs":["message-id-1","message-id-2"]}
```

Search is paginated. A search returns at most 25 candidates and a compact `messageIDs` array for the deletable assistant bot messages on that page.

Delete does not search internally and does not use pagination. Pass only `messageIDs`:

```json
{"messageIDs":["message-id-1","message-id-2"]}
```

For "delete all matching messages", repeat this loop: search, delete the returned `messageIDs`, then run the same search again until `messageIDs` is empty. Do not advance to `nextCursor` after deleting, because deleted messages can shift later matches into the first page.

Do not claim that a message was changed or deleted until the operation succeeds. If deletion is partial, report the actual deleted and failed counts. The backend blocks edits and deletes of messages that were not written by the assistant, and blocks 업무, 캘린더, and 근태 automated messages.

When the user asks to stop future messages, use `capability.invoke` with `operation: schedule.cancel`. When the user asks to remove messages already sent in Mattermost, use `capability.invoke` with `operation: message.search` then `operation: message.delete`.

## Channel Management

Use `channel.update` for channel header changes, display name changes, and channel invites. This operation is admin-only and requires confirmation.

Rules:

- Ask for confirmation with `ask.confirm` before `channel.update`.
- Do not use this skill to bypass Flow, calendar, or attendance workflows.
- Managed headers containing links like `업무 열기`, `캘린더 열기`, `근태 열기`, `Open Flow`, `Open Calendar`, or `Open Attendance` are protected by the backend.
- Default channel display names are protected by the backend.
