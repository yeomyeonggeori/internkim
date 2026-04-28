---
name: agent-browser
description: Browser automation CLI for AI agents. Use when the user needs to interact with websites, including navigating pages, filling forms, clicking buttons, taking screenshots, extracting data, testing web apps, or automating any browser task.
allowed-tools: Bash(agent-browser:*), Bash(npx agent-browser:*)
hidden: true
---

# agent-browser

Fast browser automation CLI for AI agents.

Chrome/Chromium via CDP with accessibility-tree snapshots and compact `@eN` element refs.

Install:

```bash
npm i -g agent-browser
agent-browser install
```

## Start Here

This file is a discovery stub, not the full usage guide.

Before running any `agent-browser` command, load the actual workflow content from the installed CLI:

```bash
agent-browser skills get core
agent-browser skills get core --full
```

The CLI serves skill content that matches the installed version, so setup should prefer `agent-browser skills get core --full` when available.

## Core Workflow

1. `agent-browser open <url>` - Navigate to a page.
2. `agent-browser snapshot -i` - Get interactive elements with refs like `@e1`.
3. `agent-browser click @e1` or `agent-browser fill @e2 "text"` - Interact using refs.
4. Re-run `agent-browser snapshot -i` after page changes.

## Specialized Skills

```bash
agent-browser skills get electron
agent-browser skills get slack
agent-browser skills get dogfood
agent-browser skills get vercel-sandbox
agent-browser skills get agentcore
agent-browser skills list
```

## Why agent-browser

- Fast native Rust CLI, not a Node.js wrapper
- Works with any AI agent
- Chrome/Chromium via CDP with no Playwright or Puppeteer dependency
- Accessibility-tree snapshots with element refs for reliable interaction
- Sessions, authentication vault, state persistence, and video recording
