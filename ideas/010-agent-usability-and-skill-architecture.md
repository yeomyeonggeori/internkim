# Blueclaw Agent Usability And Skill Architecture

## Summary

Hermes is useful reference material, not a template to copy. The goal is a Blueclaw-native agent architecture: small, explicit, appliance-oriented, security-boundary-aware, and useful for daily company automation.

Blueclaw already has the right primitives: connectors, task runs, scheduler, memory, MCP, policy, and skills. The improvement is to make those primitives compose cleanly so the agent can choose the right procedure, tool, memory, and runtime path without dumping every instruction into every prompt.

This should feel like an appliance, not a general-purpose plugin marketplace. The system should be durable, inspectable, auditable, and secure by construction.

## Design Direction

### Skills As Operational Capabilities

Skills should be treated as operational capabilities, not prompt blobs. A skill is a small, inspectable package that describes:

- what it does
- when it should be considered
- which tools it may use
- which profiles may use it
- which references, scripts, and assets support it

`SKILL.md` frontmatter should become the source of truth for capability metadata:

```yaml
name: simple-slides
description: Create presentation decks and attach PPTX/PDF/HTML outputs.
category: document-generation
tags: [slides, pptx, marp, reporting]
allowed-tools: terminal_run file_write file.attach
allowedProfiles: [default]
triggerHints:
  - slides
  - presentation
  - pptx
references:
  - references/design-system.md
scripts:
  - scripts/extract_notes.py
assets:
  - assets/template.md
```

The prompt path should use progressive disclosure:

1. Build a compact skill index from metadata.
2. Select relevant skills for the request and current profile.
3. Load full skill instructions only for selected skills.
4. Record the selected skills and selection reasons in task events.

This preserves Blueclaw's product taste: a small set of explicit bundled skills, not chaotic marketplace sprawl.

### Skills Connected To Runtime Paths

Skill selection should be connected to actual runtime capability. A skill should not be selected if its allowed tools are unavailable or disallowed by profile policy.

Current skill loading should stay grounded in the existing workspace paths:

- `.agents/skills` for hidden or runtime-internal operational skills
- `skills` for bundled or user-visible workspace skills

If a long-term normalized path is desired, `.blueclaw/skills` can be introduced later as a migration target, but the first implementation should respect the current paths.

Connector requests, scheduled tasks, and admin-triggered tasks should all create the same kind of task run and attach selected skill references to that task run. The selected skill name, source path, source hash, allowed tools, and selection reason should be visible in task events.

### Task-Shape Planning

Blueclaw should keep the concepts of planner, researcher, and responder, but stop forcing every request through all of them.

The agent should first classify the task shape:

- immediate reply
- research task
- maintenance task
- scheduled task
- browser-handoff task
- approval-gated task

The shape determines the procedure, budget, memory retrieval, allowed skills, and whether the browser or companion runtime may be used. Profiles should define:

- allowed tools
- allowed skills
- memory scope
- browser handoff permission
- approval requirements

This keeps the system simple while avoiding the cost and noise of one fixed agent loop for every request.

### Memory As Relevant Context

Memory should not be injected as a raw dump. The retrieval path should be:

1. Apply policy and access filtering first.
2. Rank accessible records by embedding similarity and recency.
3. Inject compact, attributed memory summaries.

The prompt should show why memory is relevant without exposing inaccessible provenance or hidden policy details.

### MCP As A Tool Catalog

MCP should behave like a real Blueclaw tool catalog:

- initialize configured MCP servers
- discover tool schemas
- cache tool definitions
- expose only profile-allowed tools
- route all calls through policy and task audit

MCP must not bypass Blueclaw policy, task events, Firecracker boundaries, workspace path guards, or connector reply rules.

## Hermes Ideas To Adapt Carefully

Hermes has useful ideas, but they should be translated into Blueclaw primitives:

- Progressive skill disclosure maps well to metadata-backed skill selection.
- Gateway unification maps to connectors, admin UI, scheduler, and bridge all creating task runs.
- Scheduled agent tasks fit Blueclaw's scheduler.
- Profile isolation should become Blueclaw profile permissions, not copied home-directory personalities.
- Slash-command style affordances can improve chat UX, but should map to skills and task shapes.

Do not copy Hermes structure wholesale. Blueclaw's shape is appliance-native: smaller, more explicit, and more security-boundary-aware.

## First Implementation Slice

The first implementation should be intentionally small:

- Extend skill metadata parsing with `category`, `tags`, `allowedProfiles`, `triggerHints`, and optional `references`, `scripts`, and `assets`.
- Build a compact skill index prompt from metadata.
- Load full skill instructions only after selection.
- Emit task events for selected skills, including source path/hash and selection reason.
- Keep legacy `SKILL.md` files working with a minimal fallback: directory name as `name`, body as instruction, no automatic trigger unless explicitly selected or referenced by existing compatibility logic.

After that slice lands, add task-shape planning, memory ranking, and MCP schema caching as separate increments.

## Test Plan

- Unit test skill metadata parsing and legacy `SKILL.md` fallback.
- Unit test compact skill index prompt construction.
- Unit test selected full skill body injection.
- Integration test connector request creates a task run with selected skill references.
- Integration test scheduled task starts a fresh task run with configured skills.
- Integration test admin-triggered task uses the same task run and skill selection path.
- Integration test memory retrieval applies policy filtering before ranking.
- Integration test MCP tool discovery populates tool schemas and respects profile allowlists.
- Audit test selected skills, tool calls, browser handoff, and approval waits are visible in task events.

## Non-Goals

- Do not copy Hermes structure wholesale.
- Do not turn skills into a chaotic plugin marketplace.
- Do not inject every skill into every prompt.
- Do not let MCP bypass Blueclaw policy, task audit, or Firecracker/workspace boundaries.
- Do not make the main computer the primary execution environment. It remains for browser handoff, approval, login, and local credential transfer.

## Assumptions

- This document is handoff guidance, not an implementation patch.
- Hermes is inspiration only.
- Blueclaw's best shape is durable, auditable, minimal, and secure by construction.
- The first implementation should prioritize metadata-backed skill discovery, compact skill index prompts, and selected skill task events before heavier orchestration.
