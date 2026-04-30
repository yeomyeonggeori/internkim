---
name: agent-browser
description: Use InternKim browser tools for web navigation, page snapshots, interaction, and screenshots. The runtime is backed by agent-browser, but Blueclaw calls browser.* tools.
---

# Browser Automation

Use the `browser.*` tools for web automation. InternKim runs the browser runtime behind these tools.

Core workflow:

1. `browser.open` with `{ "url": "https://example.com" }` to navigate.
2. `browser.snapshot` with `{}` to read page text and interactive refs such as `@e1`.
3. `browser.click` or `browser.fill` using refs or selectors.
4. Re-run `browser.snapshot` after page changes.
5. `browser.screenshot` when the user asks to capture the visible result.

Never invent missing refs, selectors, URLs, or file paths. If a tool returns an attachment, let the final reply include the attachment instead of exposing a device path.
