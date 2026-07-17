---
name: mattermost
description: Read, search, post, update, and delete Mattermost messages, manage channels, and inspect approved workspace conversations. Use for Mattermost, chat, 채팅, 메시지, channel, or team requests.
tool-references: message.context message.search message.send message.update message.delete channel.update
---

# Mattermost

Call the typed Mattermost operations directly; descriptors define exact fields and results. Use the returned IDs as the only basis for later reads, edits, or deletes.

## Read and search

- Use `message.context` when the current Mattermost conversation supplies the channel, thread, or requester context. Use `message.search` for channel, person, text, and date clues, then use the observed message IDs for follow-up actions.
- Distinguish no result, an ambiguous match, and a provider error. Never infer a message ID or claim unseen content.

## Write and manage

- Use `message.send` for a new message and `message.update` only with an observed message ID. Do not duplicate a successful post.
- Use `message.delete` only when the user explicitly requests deletion; approval is handled by the runtime.
- Use `channel.update` for supported channel changes. Preserve existing names and membership unless the request says otherwise.

## Reporting

Report successful posts, edits, deletes, and channel changes only after the operation succeeds. For history, summarize observed messages with channel and timestamp context, and keep pagination or approval failures visible.
