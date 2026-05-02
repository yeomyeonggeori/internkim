---
name: bash
description: Use the Blueclaw guest terminal for workspace-scoped command-line work.
category: development
tags: [terminal, bash, shell, workspace]
triggerHints:
  - bash
  - terminal
  - shell
  - command line
  - CLI
  - 명령어
  - 터미널
  - 셸
requiredTools:
  - terminal.run
allowedProfiles: [default]
---

# Bash

Use `terminal.run` for command-line work inside `/workspace`.

Prefer `rg` for searching text or files. Keep commands workspace-scoped. For complex flows, create a script file first, run it, then report the exit code, stdout, and stderr. Do not claim host, root, or user-computer shell access.
