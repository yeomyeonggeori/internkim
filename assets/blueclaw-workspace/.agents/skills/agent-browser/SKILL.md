---
name: agent-browser
description: Use internkim browser capability operations for web navigation, page snapshots, and interaction in the company computer's browser. The runtime is backed by agent-browser, but Blueclaw invokes browser.* through the capability bridge.
---

# Browser Automation

Browser automation is an interactive fallback, not the default web research path. Prefer search/fetch capabilities for ordinary public lookup and source retrieval. Use browser operations only when page state or interaction is the actual task, or search/fetch capabilities are unavailable, insufficient, or failing.

You work in the company computer's browser; nobody else is operating it. When a page asks for a login, MFA code or captcha you cannot pass, stop and tell the person which page and what it asked for. Do not guess credentials or try to get around the check.

Invoke `browser.*` operations through the capability bridge. internkim runs the browser runtime behind these operations.

Core workflow:

1. `browser_open` with `{ "url": "https://example.com" }` to navigate.
2. `browser_snapshot` with `{}` to read page text and interactive refs such as `@e1`.
3. `browser_click`, `browser_fill`, `browser_select`, `browser_press` or `browser_wait` using refs or selectors.
4. Re-run `browser_snapshot` after page changes.

Never invent missing refs, selectors, URLs, or file paths. If an operation returns an attachment, let the final reply include the attachment instead of exposing a device path.
