---
name: web-search
description: Search the public web or fetch a specific URL when the user's request needs external or current information that is not already available in the conversation, an attachment, or another skill. Use for news, prices, schedules, documentation, or any fact you cannot already answer from provided context.
when_to_use: Use when the answer requires information from the open web that is not already present in the conversation, an attachment, or a more specific skill (Mattermost, calendar, flow, weather, etc). Do not use when the attached image, file, or message already contains the answer.
completion:
  requiredEvidenceTools:
    - web.search
---

# Web Search

Use `capability.invoke` with `operation: "web.search"` to search the public web, and `operation: "web.fetch"` to retrieve the content of specific URLs. Set `operation` to the operation name and `input` to its parameters.

Check the conversation and any attachments first. If the answer is already there, answer directly and do not call this skill just to satisfy a requirement — call it only when it genuinely adds information you do not have.

## Search

Use `web.search` with:

- `query` (required): the search query text.
- `location`, `language` (optional): hints for localized results.
- `limit` (optional, 1-10, default 5): number of results.
- `allowedDomains`, `excludedDomains` (optional): restrict or exclude domains.

Response fields: `query`, `answer` (a synthesized answer), and `results` (array of `title`, `url`, `snippet`, `source`). Read `answer` first, then use `results` for citations or follow-up `web.fetch` calls.

## Fetch

Use `web.fetch` with:

- `urls` (required): up to 10 URLs to fetch.
- `maxContentTokens` (optional, 1000-100000, default 50000): content size limit.
- `allowedDomains`, `blockedDomains` (optional): restrict or exclude domains.

Response fields: `results` (array of `url`, `finalURL`, `title`, `content`) and `errors` (array of `url`, `error`) for URLs that failed.

## Rules

- Both operations require network access; if `status` is `error` (for example `local_only` or `missing_openrouter_key`), say the web tool is unavailable rather than guessing an answer.
- Do not fabricate URLs, titles, or snippets. Only cite what `web.search` or `web.fetch` returned.
- Prefer `web.search` to find sources, then `web.fetch` only when you need the full content of a specific page.
