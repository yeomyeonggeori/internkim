---
name: ask
description: Ask the user for confirmation, a bounded choice, or free-form input when execution cannot safely continue without their decision.
when_to_use: Always available. Use only when the user must confirm an action, choose from explicit options, or provide missing input before the task can continue.
---

# Ask

Use ask tools only when user input is required to continue.

Do not use ask tools for ordinary replies, calculations, lookups, or tasks that can proceed safely with the information already available.

Use `ask.confirm` before destructive actions, external sends, permission changes, public deploys, paid actions, credential access, or other sensitive actions.

Use `ask.choice` when the user should pick from explicit options. Provide the options clearly, use short stable keys such as `A`, `B`, and `C`, and always set exactly one `recommendedOptionKey`.

Use `ask.input` when the needed answer is free-form and cannot be represented as a bounded choice.

For choices, keep labels short and meaningful. The runtime can resolve replies such as `1`, `첫 번째`, `A`, or text matching an option, and can also accept Mattermost buttons or menus.
