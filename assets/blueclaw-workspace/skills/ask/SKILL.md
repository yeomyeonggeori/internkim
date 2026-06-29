---
name: ask
description: Ask the user for confirmation, a bounded choice, or free-form input when execution cannot safely continue without their decision.
when_to_use: Always available. Use only when the user must confirm an action, choose from explicit options, or provide missing input before the task can continue.
---

# Ask

Use ask kernel tools only when user input is required to continue.

Do not use ask kernel tools for ordinary replies, calculations, lookups, or tasks that can proceed safely with the information already available.

Use `ask.confirm` before destructive actions, external sends, permission changes, public deploys, paid actions, credential access, or other sensitive actions.

Use `ask.input` when the user should pick from explicit options. Pass those options in `choices`; leave `choices` empty for free-form input. A non-empty `choices` list still allows the user to type a different answer when needed.

For choices, keep labels short and meaningful. The runtime can resolve replies such as `1`, `첫 번째`, `A`, or text matching an option, and can also accept Mattermost buttons or menus.
