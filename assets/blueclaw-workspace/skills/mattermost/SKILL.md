---
name: mattermost
description: Work with Mattermost conversations, posts, threads, attachments, channels, pins, and InternKim bot messages when the user is talking with InternKim inside Mattermost.
when_to_use: Use when the user asks about Mattermost messages, threads, replies, attachments, DMs, channels, pins, headers, invites, or asks to edit/delete messages that InternKim posted.
allowed-tools:
  - ask.confirm
  - platform.message.context
  - platform.message.search
  - platform.message.send
  - platform.message.update
  - platform.message.delete
  - mattermost.channel.update
---

# Mattermost

Use platform message tools for Mattermost messaging. Mattermost is the current platform adapter; do not choose Mattermost-specific post tools for ordinary message work.

If the current conversation is Mattermost, use this skill when the user asks about messages in the current thread, current channel, a DM, a named channel, or bot messages that InternKim already sent.

## Read

Use `platform.message.context` when you need the current channel, thread, DM, post, bot user, or requester context.

Use `platform.message.search` when the user refers to messages without exact IDs.

Search scopes:

- `currentThread`: only the current thread.
- `currentChannel`: the current channel or DM channel.
- `directMessage`: a DM with `personHint`, or the current DM when no person is needed.
- `channel`: a named channel with `channelID` or `channelName`.

Use `authoredBy: "assistant"` when the user asks about messages InternKim sent. Use the returned previews to decide which messages match the user's intent. Do not delete from a natural-language description without first finding candidate message IDs.

## Post

Use `platform.message.send` to send a DM, reply to the current thread, post to the current channel, or post to a named channel. Ask for confirmation first with `ask.confirm`, then call the tool only after approval.

Use `pin: true` when the user asks to post and pin the message in the same request. Do not call `platform.message.update` again after a pinned post succeeds.

## Update Or Delete Posts

Use `platform.message.update` to edit an InternKim bot message or pin/unpin a message. Ask for confirmation first with `ask.confirm`.

Set `isPinned: true` to pin, `isPinned: false` to unpin, and omit `isPinned` when only changing message text.

Use `platform.message.delete` to delete one or more InternKim bot messages. Ask for confirmation first with `ask.confirm`.

Pass all selected IDs in `messageIDs`, even when deleting a single message:

```json
{"messageIDs":["message-id-1","message-id-2"]}
```

Do not claim that a message was changed or deleted until the tool succeeds. If deletion is partial, report the actual deleted and failed counts. The backend blocks edits and deletes of messages that were not written by InternKim, and blocks 업무, 캘린더, and 출결 automated messages.

When the user asks to stop future messages, use `schedule.cancel`. When the user asks to remove messages already sent in Mattermost, use `platform.message.search` and `platform.message.delete`.

## Channel Management

Use `mattermost.channel.update` for channel header changes, display name changes, and channel invites. This tool is admin-only and requires confirmation.

Rules:

- Ask for confirmation with `ask.confirm` before `mattermost.channel.update`.
- Do not use this skill to bypass Flow, calendar, or attendance tools.
- Managed headers containing links like `업무 열기`, `캘린더 열기`, `출결 열기`, `Open Flow`, `Open Calendar`, or `Open Attendance` are protected by the backend.
- Default channel display names are protected by the backend.
