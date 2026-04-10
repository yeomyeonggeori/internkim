---
name: share-file
description: "Share files as attachments. MUST use when user asks for files, images, PDFs, etc. Run: send-file <url> <filename>"
---

# Sharing Files

Use the `send-file` command to download a file and send it as an attachment.
The file will automatically be posted in the same thread as your current conversation.

## Usage

```bash
send-file "<url>" "<filename>"
```

Arguments:
1. URL to download
2. Filename for the attachment

## Examples

```bash
send-file "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf" "test.pdf"
send-file "https://placecats.com/300/200" "cat.jpg"
```

## Rules

- ALWAYS use send-file when the user wants a file sent as an attachment.
- Do NOT just paste a URL link. Use send-file to send the actual file.
- The file will appear as a native attachment in the correct thread automatically.
