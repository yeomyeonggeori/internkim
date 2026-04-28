---
name: share-file
description: "Share files as native attachments when a tool result provides an attachment. Use when user asks for files, images, PDFs, etc."
---

# Sharing Files

When a tool result includes a file attachment, the final reply delivery system sends it as a native platform attachment.

## Rules

- Do not paste local paths, temporary URLs, or markdown links as the final delivery.
- Do not claim a file was attached unless the tool result produced an attachment.
- If no attachment-producing tool is available, say which file delivery step is unavailable.
