---
name: skill-management
description: Create, add, update, or remove user-managed Blueclaw skills. Use for requests about making a new skill, writing SKILL.md, adding a skill, deleting a skill, removing a skill, or managing skills.
when_to_use: Use when the user asks to create a skill, add a skill, update a skill, remove a skill, delete a skill, write SKILL.md, 스킬 만들기, 스킬 추가, 스킬 삭제, 스킬 제거, 스킬 관리, or /skill-management.
allowed-tools:
  - skill.add
  - skill.remove
---

# Skill Management

Use `skill.add` to create or update user-managed skills. Use `skill.remove` to remove user-managed skills.

Before writing a skill, capture the intent:

- What the skill should enable.
- When it should trigger, including realistic user phrases.
- The expected behavior or output.
- Which runtime tools it needs.
- Two or three realistic test prompts the user can try after creation.

Write standard-compatible `SKILL.md` content directly in the `content` argument. Use only standard frontmatter fields. Do not invent custom fields, summaries, tags, trigger hints, custom tool dependency fields, allowed profiles, generated indexes, or full-body embeddings. Prefer `name`, `description`, `when_to_use`, and `allowed-tools`.

Keep `SKILL.md` concise. Put essential workflow in the body. Put long domain knowledge in `references/`, deterministic repeated logic in `scripts/`, and output resources in `assets/`. When adding bundled resources, pass them through the `resources` argument to `skill.add` and mention each referenced resource from `SKILL.md`.

Scripts should be self-contained within the skill folder. Built-in Python skill dependencies should be available from the runtime; helpers should use that first, then use `uv` with `scripts/requirements.txt` into requester-owned temporary storage and reuse `/workspace/shared/cache/dependencies` only as a package cache. The public instruction should tell the model to run the bundled wrapper, not runtime internals or venv Python paths. Asset-local Node dependencies should follow the same pattern with `package.json`. Do not write new skills that ask the model to stop and report missing import libraries as the primary recovery path.

Do not put evals, benchmark metadata, test prompts, or generated summaries in frontmatter. Mention suggested test prompts to the user after the skill is created.

Do not use `file.write`, `terminal.run`, or shell redirection to create, edit, or delete skill files. The skill tools derive the destination path from the skill name and enforce the runtime boundary.

User-managed skills live under `/workspace/.agents/skills/<name>`. Built-in skills are immutable and cannot be overwritten or removed.

After adding or removing a skill, tell the user that the change is available to future turns after skill retrieval refreshes. If `skill.add` returns warnings, report them as improvement suggestions, not as failure.
