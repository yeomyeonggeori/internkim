---
name: website
description: Create, inspect, edit, preview, publish, or delete dependency-light websites, web apps, prototypes, landing pages, dashboards, and demos.
tool-references: terminal.run file.read file.write file.edit browser.open browser.snapshot browser.screenshot browser.click artifact.review site.create site.status site.preview site.publish site.delete
---

# Website

Build validation prototypes, landing pages, dashboards, demos, and small web apps. The five site tools own lifecycle operations. Workspace source changes always use `file.write` or `file.edit`; there is no `site.edit`.

Do not claim production readiness, compliance, SLA, paid hosting, payments, or real integrations unless the user requested and verified them.

## Source and design

User-provided facts are the source of truth. Preserve names, products, people, dates, prices, locations, policies, and confirmation states exactly. Show missing values as the user's-language equivalent of “Not provided”.

Keep a source checklist in `.internkim/required-visible-text.txt`, one required fact per line. The must-show source content belongs in rendered text, not only metadata or the final reply.

Choose an archetype before creating: landing page, dashboard, admin tool, booking, marketplace, portfolio, content site, or app shell. Save the request, audience, visual direction, and success criteria in `.internkim/artifact-brief.md`. Use the managed React, Tailwind, and shadcn scaffold. Keep content in `app/public/site-content.json` when the site is content-driven.

Use a request-specific visual system. Prefer system fonts, restrained color, consistent spacing, and meaningful controls. Avoid generic dark navy shells, gradients, decorative filler, placeholder copy, and repeated cards unless the request calls for them.

## Workflow

1. Derive one stable lowercase slug. Call `site.status` with that exact slug as `siteReference`.
2. If lookup fails as not found, call `site.create` once. If it fails as ambiguous, ask the user to choose from the returned candidates. Keep the returned `siteID`, `sourceWorkspacePath`, and `appWorkspacePath`.
3. Read existing source before changing it. Use `file.write` for new files and `file.edit` for existing files under the returned source path. Never invent paths or replace the managed scaffold with a scratch project. Site text and content live only in `app/public/site-content.json`; never edit `app/src/site-content.ts` (it is the loader code, and touching anything under `app/src/**` forces a full rebuild).
4. Content-only changes require no build. For structural changes under `app/src/**`, run `bun scripts/build.ts` with `terminal.run` from the returned app workspace.
5. Check the rendered text against the source checklist. Call `site.preview` with the exact `siteID`.
6. Verify the preview with `terminal.run`: `curl -sL --http1.1 <previewURL>` piped to a text search for each required phrase and control label, plus the content JSON for button labels. A phrase and control present in the fetched preview is complete verification of visibility and presence. If the fetch fails at the transport or edge layer (connection error, tunnel error page such as `error code: 1033`) rather than with a page-level error, verify each required phrase and control label directly in the rendered source files instead (`app/public/site-content.json` plus any edited source), treat that as sufficient verification, and state in the final reply that the live URL could not be fetched from the device network. When browser tools are available, additionally open the preview and click the primary controls; when screenshots exist, run `artifact.review` on them for visual hierarchy and text fit. Fix source with `file.edit`, then preview again.
7. Call `site.publish` with the exact `siteID` only after review. Confirm the returned public URL, deployed version, and `sourceSHA256`, then call `site.status` with the same `siteID` as `siteReference`.

For follow-up edits, resolve the existing site with `site.status`, reuse its exact identity and paths, edit with `file.edit`, preview, review, and publish. Never create a replacement site for a normal update.

## Health and deletion

Respect status and workspace health exactly. Recheck one transient health failure; otherwise report the unresolved condition without inventing recovery.

Delete only when the user explicitly requests it. Resolve the exact `siteID` and call `site.delete`; the runtime obtains approval before execution. Completion requires the delete result for that exact `siteID`.

## Final reply

After publish, provide the public URL, what changed, and how to try the main interaction. Identify the result as a prototype when appropriate. Report failures and unfinished review honestly.
