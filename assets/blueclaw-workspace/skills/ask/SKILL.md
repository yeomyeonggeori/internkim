---
name: ask
description: Ask the user for a bounded choice or free-form input only when execution cannot continue without their decision.
---

# Ask

Use `ask_input` only when user input is required to continue.

Do not use it for ordinary replies, calculations, lookups, or tasks that can proceed with the information already available.

The runtime automatically requests confirmation for operations whose structured policy requires approval. Invoke those operations directly. For `terminal_run`, set `approvalRequired=true` with an `approvalReason` when the exact command needs the user's confirmation.

Pass explicit options in `choices`; leave `choices` empty for free-form input. A non-empty `choices` list still allows the user to type a different answer when needed.

For choices, keep labels short and meaningful. The runtime can resolve replies such as `1`, `첫 번째`, `A`, or text matching an option, and can also accept Mattermost buttons or menus.
