---
name: mattermost
description: Read Mattermost channel posts, post to channels, pin posts, update or delete InternKim bot posts, and manage Mattermost channels when the user asks about Mattermost messages, channels, pins, headers, names, or invites.
when_to_use: Use when the user asks to post in Mattermost, read channel posts, pin or unpin a Mattermost post, edit or delete a Kim Intern post, update a channel header or name, or invite people to a Mattermost channel.
allowed-tools:
  - user.confirm
  - mattermost.channel.posts.list
  - mattermost.channel.post
  - mattermost.post.update
  - mattermost.post.delete
  - mattermost.channel.update
---

# Mattermost

Use Mattermost tools for Mattermost-only messaging and channel operations.

## Read

Use `mattermost.channel.posts.list` to read posts from a channel. This does not require approval.

Required channel selector:

- `channelID` or `channelName`

Optional pagination:

- `page`
- `perPage`

## Post

Use `mattermost.channel.post` to post to a channel. Ask for confirmation first with `user.confirm`, then call the tool only after approval.

Use `pin: true` when the user asks to post and pin the message in the same request. Do not call `mattermost.post.update` again after a pinned post succeeds.

## Update Or Delete Posts

Use `mattermost.post.update` to edit an InternKim bot post or pin/unpin a post. Ask for confirmation first.

Set `isPinned: true` to pin, `isPinned: false` to unpin, and omit `isPinned` when only changing message text.

Use `mattermost.post.delete` to delete an InternKim bot post. Ask for confirmation first.

Do not claim that a post was changed or deleted until the tool succeeds. The backend blocks edits and deletes of posts that were not written by InternKim, and blocks Flow, calendar, and attendance automated posts.

## Channel Management

Use `mattermost.channel.update` for channel header changes, display name changes, and channel invites. This tool is admin-only and requires confirmation.

Rules:

- Ask for confirmation before `mattermost.channel.update`.
- Do not use this skill to bypass Flow, calendar, or attendance tools.
- Managed headers containing links like `업무 열기`, `캘린더 열기`, `출결 열기`, `Open Flow`, `Open Calendar`, or `Open Attendance` are protected by the backend.
- Default channel display names are protected by the backend.
