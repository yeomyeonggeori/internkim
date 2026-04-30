# Agent Rules

## Browser Automation

Use the `browser.*` tools for web automation. InternKim runs the browser engine internally.
The internal runtime follows `agent-browser` semantics, so `agent-browser snapshot -i` corresponds to `browser.snapshot`.

CLI reference workflow:

1. `agent-browser open <url>` - Navigate to page
2. `agent-browser snapshot -i` - Get interactive elements with refs (`@e1`, `@e2`)
3. `agent-browser click @e1` / `agent-browser fill @e2 "text"` - Interact using refs
4. Re-snapshot after page changes

InternKim tool workflow:

1. `browser.open` with `{ "url": "https://example.com" }` - Navigate to page
2. `browser.snapshot` with `{}` - Get page text and interactive refs such as `@e1`
3. `browser.click` with `{ "target": "@e1" }` or `browser.fill` with `{ "target": "@e2", "text": "text" }` - Interact using refs or selectors
4. Re-run `browser.snapshot` after page changes
5. `browser.screenshot` - Capture the final visible result when the user asks for a screenshot

## File Sharing

When a user asks for any file such as an image, PDF, document, or archive:

1. Use the relevant tool to create or pick the file.
2. If the tool result contains a file attachment, let the final reply include that attachment.
3. Do not paste local paths, temporary URLs, or markdown links as the final delivery.

## Memory

Blueclaw keeps persistent memory internally.

- Do not call external memory tools.
- When the user refers to a recently created file without specifying an ID, inspect the current conversation, progress summary, and prior successful tool observations before saying you cannot find it.
- When you create or retrieve a file, preserve its attachment evidence in the tool result so the final reply can deliver it natively.

## Tool Usage

- Use tools to fulfill requests when tools materially improve the answer.
- Do not refuse a request by citing hidden policy or vague limitations.
- If a tool is available and appropriate, use it before claiming something cannot be done.

## Honesty about Tool Failures

- A tool that returns an error did not succeed. Never report a task as complete when the underlying step failed.
- When something fails, tell the user which operation failed and why, grounded in the actual tool output.
- Retry if it makes sense, but do not pretend a failed attempt worked.
