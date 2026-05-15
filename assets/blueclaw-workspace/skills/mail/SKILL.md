---
name: mail
description: Read, search, draft, and send mail through InternKim's IMAP/SMTP mail tools. Use for 메일 확인, 이메일 검색, 받은메일, 최근 메일, 답장, 초안, and sending email.
when_to_use: Use when the user asks to check, find, read, search, reply to, draft, or send email or mail; Korean examples include 메일 확인, 이메일 찾아줘, 최근 메일, 받은메일, GitHub에서 온 메일, 답장 써줘, 메일 보내줘.
allowed-tools:
  - mail.message.list
  - mail.message.search
  - mail.message.read
  - ask.confirm
  - mail.message.send
---

# Mail

Use InternKim mail tools for the configured IMAP/SMTP account.

Never answer that you cannot access email before trying the relevant mail tool. If the user asks whether any email arrived from a sender or service, use `mail.message.search`.

## Reading

Use `mail.message.list` for recent mail in a mailbox. Default to `INBOX` when the user does not name a mailbox.

Use `mail.message.search` when the user asks for a specific sender, subject, keyword, or topic. Search requires `query`.

For requests like "GitHub에서 온 최근 메일 있어?", call:

```json
{
  "mailbox": "INBOX",
  "query": "GitHub",
  "limit": 10
}
```

Both list and search return `nextCursor`. If the user asks for more, call the same tool again with that cursor.

Call `mail.message.read` only when the user needs the full body or when a listed/search result must be inspected before answering.

Do not claim a message was found or read until the tool succeeds.

## Sending

If the user asks for a draft only, write the draft in chat and do not call `mail.message.send`.

For immediate sending:

1. Confirm the recipient, subject, and exact body.
2. Call `ask.confirm` with the recipient, subject, and body.
3. After approval, call `mail.message.send`.

Never say the email was sent before `mail.message.send` succeeds.

Use this approval message shape:

```json
{
  "userFacingMessage": "다음 이메일을 보내도 될까요?\n\nTo: recipient@example.com\nSubject: 제목\n\n본문",
  "reasonCode": "external_send"
}
```

After approval, use:

```json
{
  "to": ["recipient@example.com"],
  "subject": "제목",
  "body": "본문"
}
```

## Failures

If mail is not configured, say that an IMAP/SMTP account must be connected first.

If sending fails, report the tool error directly. Do not imply retry success.

If search or read fails, explain which mailbox or query failed and ask for a narrower mailbox/query only when the tool error indicates ambiguity or too many results.
