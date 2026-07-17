---
name: website
description: Create, publish, update, take down, restore, or delete dependency-light websites, web apps, prototypes, demos, landing pages, dashboards, and app ideas through site capability operations.
tool-references: terminal.run file.read file.write file.edit site.create site.preview artifact.review site.publish site.status site.history site.diff site.logs site.rollback site.unpublish site.restore site.delete
---

# Site Prototype

Use this skill for validation prototypes, web apps, landing pages, dashboards, demos, and app ideas. Call named site and review operations directly; their typed descriptors define payloads and results. Do not claim production readiness, compliance, SLA, paid hosting, payment support, or real integrations unless explicitly requested and confirmed.

## Source truth and scaffold

User-provided data is the source of truth. Preserve names, products, people, dates, prices, locations, policies, and confirmation states exactly; missing values render as the user's-language equivalent of “Not provided”. The must-show source content belongs in rendered text, not only in `app/public/site-content.json` or the final reply. Keep a source checklist and write each required fact one per line to `.internkim/required-visible-text.txt`.

Use the managed React + Tailwind + shadcn scaffold and its reusable blocks. Keep content in `app/public/site-content.json`; use different structures for a restaurant, portfolio, dashboard, or domain-specific app. Use PocketBase only when local data, auth, files, realtime, or migrations are needed. Use system fonts by default; embed fonts only as WOFF2 with `@font-face` and `format("woff2")`. Keep the deliberate black-on-white default unless the request calls for another mood; avoid a generic dark navy shell, gradients, and decorative filler.

## Design contract

Choose a UI archetype before creation: landing, dashboard, admin tool, booking, marketplace, portfolio, content site, or app shell. Write a request-specific `DESIGN.md` in Stitch canonical format at the source root, with `colors`, `typography`, `rounded`, `spacing`, `components`, and the sections Overview, Colors, Typography, Layout, Elevation & Depth, Shapes, Components, and Do's and Don'ts. Decide the visual system before site creation and preserve the source summary in `.internkim/artifact-brief.md` and positioning in `.internkim/idea.md`.

## Create and edit workflow

1. Call `site.status` once. If it is `ambiguous`, show candidate titles, descriptions, archetypes, owners, and URLs and ask which site to update. If it is `not_found`, create directly; do not poll.
2. Call `site.create` once with a complete request-specific record. The typed descriptor supplies required fields and retry rules; do not invent metadata. Keep the returned site identity for the rest of the workflow.
3. Read `references/blocks.md` when block details are needed, then compose content with `file.write` or `file.edit`. Do not edit managed scaffold configuration or package files.
4. Content-only changes need no build step. Structural changes under `app/src/**` or scaffold configuration run `terminal.run` with `bun scripts/build.ts` from the returned app workspace; the build writes `build-quality.json`. Never replace managed source with a scratchpad or use `capability.invoke` for site editing.
5. Run the content review after every content change and improve while the score rises. Inspect rendered text against the source checklist, then use `site.preview` (the site.preview operation) and browser capability operations for visual QA when available. Call `artifact.review` with screenshots, the brief, source summary, archetype, and rubric; record attempts and notes in `.internkim/review-log.json`, using `visualReviewUnavailable` when screenshots are unavailable.
6. Publish only after review and validation. Call `site.publish`, then `site.status` and confirm `published`. A preview is only a draft. Once published, keep follow-up edits on the same URL: resolve with `site.status`, update, review, and publish again.

## Runtime signals and health

Respect `workspaceHealth` and `ownerIdentity` from status results. If health is `missing` or `permission_problem`, recheck once and report the unresolved condition. A `stale_build` requires the structural build workflow. Do not invent local paths, credentials, or external availability.

## Quality

- Make the first screen serve the visitor's errand, and make every pressable element lead somewhere meaningful.
- Keep same-level elements on one spacing, icon, and type grammar; data belongs in lines, items, or tables, not cramped prose.
- Treat a dark navy shell, generic SaaS cards, gradients, and decorative filler as design warnings unless clearly justified by the request.
- Write concrete service names, benefits, and calls to action; never use numbered filler or placeholder copy.
- A source checklist compares supplied facts with rendered text; do not hide must-show source content only in JSON or metadata.

## Follow-ups and destructive actions

For short continuations, resolve status and complete missing work on the existing site; do not call `site.create` again. For rollback, unpublish, or delete, rely on the runtime approval path. Delete only after explicit confirmation and with the descriptor's required confirmation value. Restore is allowed after status resolution; report the resulting status honestly.

## Final reply

After successful publish and status confirmation, provide the public URL, what changed, how to try the main workflow, and rollback availability. Include test credentials only when login exists, and identify the result as a prototype for idea validation rather than production software.
