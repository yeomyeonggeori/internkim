---
name: mattermost
description: Work with Mattermost conversations, posts, threads, attachments, channels, pins, and InternKim bot messages when the user is talking with InternKim inside Mattermost.
when_to_use: Use when the user asks about Mattermost messages, threads, replies, attachments, DMs, channels, pins, headers, invites, or asks to edit/delete messages that InternKim posted.
allowed-tools:
  - ask.confirm
  - mattermost.context.inspect
  - mattermost.post.search
  - mattermost.channel.posts.list
  - mattermost.channel.post
  - mattermost.post.update
  - mattermost.post.delete
  - mattermost.channel.update
---

# Mattermost

Use Mattermost tools for Mattermost-only messaging and channel operations.

If the current conversation is Mattermost, use this skill when the user asks about messages in the current thread, current channel, a DM, a named channel, or bot messages that InternKim already sent.

## Read

Use `mattermost.context.inspect` when you need the current Mattermost channel, thread, DM, post, bot user, or requester context.

Use `mattermost.post.search` when the user refers to posts without exact post IDs.

Search scopes:

- `currentThread`: only the current thread.
- `currentChannel`: the current channel or DM channel.
- `directMessage`: a DM with `personHint`, or the current DM when no person is needed.
- `channel`: a named channel with `channelID` or `channelName`.

Use `authoredBy: "internkim"` when the user asks about messages InternKim sent. Use the returned previews to decide which posts match the user's intent. Do not delete from a natural-language description without first finding candidate post IDs.

Use `mattermost.channel.posts.list` to read posts from a channel. This does not require approval.

Required channel selector:

- `channelID` or `channelName`

Optional pagination:

- `page`
- `perPage`

## Post

Use `mattermost.channel.post` to post to a channel. Ask for confirmation first with `ask.confirm`, then call the tool only after approval.

Use `pin: true` when the user asks to post and pin the message in the same request. Do not call `mattermost.post.update` again after a pinned post succeeds.

## Update Or Delete Posts

Use `mattermost.post.update` to edit an InternKim bot post or pin/unpin a post. Ask for confirmation first with `ask.confirm`.

Set `isPinned: true` to pin, `isPinned: false` to unpin, and omit `isPinned` when only changing message text.

Use `mattermost.post.delete` to delete one or more InternKim bot posts. Ask for confirmation first with `ask.confirm`.

Pass all selected IDs in `postIDs`, even when deleting a single post:

```json
{"postIDs":["post-id-1","post-id-2"]}
```

Do not claim that a post was changed or deleted until the tool succeeds. If deletion is partial, report the actual deleted and failed counts. The backend blocks edits and deletes of posts that were not written by InternKim, and blocks 업무, 캘린더, and 출결 automated posts.

When the user asks to stop future messages, use `schedule.cancel`. When the user asks to remove messages already sent in Mattermost, use `mattermost.post.search` and `mattermost.post.delete`.

## Channel Management

Use `mattermost.channel.update` for channel header changes, display name changes, and channel invites. This tool is admin-only and requires confirmation.

Rules:

- Ask for confirmation with `ask.confirm` before `mattermost.channel.update`.
- Do not use this skill to bypass Flow, calendar, or attendance tools.
- Managed headers containing links like `업무 열기`, `캘린더 열기`, `출결 열기`, `Open Flow`, `Open Calendar`, or `Open Attendance` are protected by the backend.
- Default channel display names are protected by the backend.
