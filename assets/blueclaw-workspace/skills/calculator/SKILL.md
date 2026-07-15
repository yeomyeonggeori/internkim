---
name: calculator
description: Calculate explicit arithmetic expressions with math.calculate when the user asks a direct arithmetic question or provides an expression.
---

# Calculator

Use `math.calculate` for explicit arithmetic expressions and exact numeric results.

Do not invent a separate calculator operation name. If `math.calculate` is unavailable, answer that the calculator capability is unavailable instead of claiming a calculation operation stopped responding.

For simple greetings or non-numeric explanations, answer directly without using this skill.

Supported v1 syntax:

- numbers and decimals
- parentheses
- `+`, `-`, `*`, `/`, `%`, `^`, `**`

Unsupported v1 syntax:

- functions such as `sqrt(2)`
- variables
- imports
- assignment
- file, network, or process access

For `1+1=`, call:

```json
{
  "expression": "1+1"
}
```

Then answer with the result only unless the user asks for explanation.
