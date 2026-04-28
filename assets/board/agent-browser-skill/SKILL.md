---
name: agent-browser
description: Browser automation through InternKim browser capability tools backed by agent-browser. Use when the user needs to navigate websites, fill forms, click controls, capture screenshots, extract visible page information, or test web apps.
hidden: true
---

# Browser Automation

Use Blueclaw's `browser.*` tools by default. InternKim maps those tools to the installed `agent-browser` runtime internally, so the CLI workflow below remains the conceptual source of truth without exposing the CLI as Blueclaw's public tool surface.

## InternKim Tool Mapping

- `agent-browser open <url>` maps to `browser.open` with `"https://example.com"`
- `agent-browser snapshot -i` maps to `browser.snapshot` with `"-i"`
- `agent-browser fill @e2 "text"` maps to `browser.fill` with `{ "target": "@e2", "text": "text" }`
- `agent-browser click @e1` maps to `browser.click` with `{ "target": "@e1" }`
- `agent-browser screenshot <path>` maps to `browser.screenshot` with `{}`

## Core Workflow

1. Call `browser.open` to open the target page.
2. Call `browser.snapshot` to get visible text and interactive refs like `@e1`.
3. Call `browser.fill`, `browser.click`, `browser.select`, `browser.press`, or `browser.wait` with refs or selectors from observation.
4. Re-run `browser.snapshot` after page changes.
5. Call `browser.screenshot` when the user asks for a screenshot or visual proof.

## Tool Inputs

- `browser.open`: `"https://www.google.com"` or `{ "url": "https://www.google.com" }`
- `browser.snapshot`: `"-i"` or `{}`
- `browser.fill`: `{ "target": "@e1", "text": "hello world" }`
- `browser.click`: `{ "target": "@e2" }`
- `browser.select`: `{ "target": "@e3", "value": "option" }`
- `browser.press`: `{ "key": "Enter" }`
- `browser.wait`: `{ "milliseconds": 1000 }` or `{ "target": "@e4" }`
- `browser.screenshot`: `{}`

## Agent-Browser CLI Reference

Use this reference to understand the runtime behavior behind InternKim's `browser.*` tools.

1. `agent-browser open <url>` - Navigate to a page.
2. `agent-browser snapshot -i` - Get interactive elements with refs like `@e1`.
3. `agent-browser click @e1` or `agent-browser fill @e2 "text"` - Interact using refs.
4. Re-run `agent-browser snapshot -i` after page changes.

## Rules

- Use refs returned by `browser.snapshot` whenever possible.
- If a tool fails, observe again before choosing a new ref.
- Do not guess hidden browser state, cookies, profile paths, CDP URLs, or local screenshot paths.
- Do not claim a browser step succeeded unless the tool result shows success.
