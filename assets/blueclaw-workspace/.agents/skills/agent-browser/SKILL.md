---
name: agent-browser
description: Use InternKim browser capability operations for web navigation, page snapshots, interaction, and screenshots. The runtime is backed by agent-browser, but Blueclaw invokes browser.* through the capability bridge.
---

# Browser Automation

Browser automation is an interactive fallback, not the default web research path. Prefer search/fetch capabilities for ordinary public lookup and source retrieval. Use browser operations only when the user needs to see or operate a browser, user input such as login/MFA/captcha is required, you are guiding the user through a web flow, page state/screenshot/interaction is the actual task, or search/fetch capabilities are unavailable, insufficient, or failing.

Invoke `browser.*` operations through the capability bridge. InternKim runs the browser runtime behind these operations.

Core workflow:

1. `browser_open` with `{ "url": "https://example.com" }` to navigate.
2. `browser_snapshot` with `{}` to read page text and interactive refs such as `@e1`.
3. `browser_click` or `browser_fill` using refs or selectors.
4. Re-run `browser_snapshot` after page changes.
5. `browser_screenshot` when the user asks to capture the visible result.

Never invent missing refs, selectors, URLs, or file paths. If an operation returns an attachment, let the final reply include the attachment instead of exposing a device path.
