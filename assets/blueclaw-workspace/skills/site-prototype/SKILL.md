---
name: site-prototype
description: Create, publish, update, take down, restore, or delete free dependency-light website prototypes through site capability tools.
when_to_use: Use when the user asks the assistant to make, deploy, publish, update, fix, take down, restore, or delete a website, web app, prototype, demo, landing page, dashboard, or app idea.
completion:
  requiredEvidenceTools:
    - site.app.status
    - site.app.build
    - artifact.review
    - site.app.publish
---

# Site Prototype

Use this skill when a user asks the assistant to create, publish, update, take down, restore, or delete a website or web app prototype.

Invoke all `site.app.*`, browser, and artifact review operations through `/workspace/tools/capability invoke <tool> '<json>'` unless the step is explicitly a local build command run with `terminal.run`.

## Purpose

Create prototypes for non-developers to validate ideas quickly. Do not present the result as production-ready software. Do not add production claims, SLA language, compliance guarantees, payment processing, real customer data workflows, or paid hosted services unless the user explicitly requests that path and confirms the cost or account requirement.

When the user provides source data, treat it as the source of truth. Preserve source-provided company names, product names, people, dates, prices, menu items, locations, and policies exactly unless the user asks for translation or normalization. Do not invent missing customers, reviews, discounts, delivery options, payment methods, addresses, phone numbers, integrations, or availability. The source-provided brand, dates, options, prices, and confirmation states must appear in the rendered UI, not only in `prototype-data.ts` or the final reply. When a useful value is missing, show the user's-language equivalent of "Not provided" instead of filling a plausible value.

## Stack

- Use the managed dependency-free TypeScript + CSS scaffold for the frontend.
- Use PocketBase files and migrations only when the prototype needs local data, auth, files, realtime, or migrations.
- Do not use Next.js, SvelteKit, arbitrary Node servers, cloud databases, hosted backends, or paid APIs for v1 prototypes unless the user explicitly asks for that stack.
- The managed build is offline-first and does not run `bun install`. Do not add npm dependencies unless the user explicitly asks for a stack change and accepts the reliability cost.
- Use system fonts by default. If the site embeds a custom or local font, use WOFF2 assets, declare them with `@font-face` and `format("woff2")`, and keep the CSS in `app/src/index.css` or app-owned styles. Do not embed TTF/OTF files, paste base64 fonts into TypeScript source, or rely on CDN-only fonts for a finished artifact.
- `site.app.create` initializes the editable website project scaffold, not a finished website.
- Use the returned `sourceWorkspacePath` exactly as the canonical virtual workspace path for all follow-up source writes. Do not rewrite it into a concrete private POSIX path.
- Use the returned `appWorkspacePath` exactly as the build working directory.

## Workflow

For create, make, build, deploy, publish, prototype, demo, landing page, dashboard, or app requests, the task is not complete until `site.app.publish` succeeds. Do not stop after `site.app.create`. Draft changes stay private until publish; use `site.app.preview` only as a temporary review URL when visual QA or user inspection needs a browser-accessible draft.

Start by resolving the existing site for the current conversation or slug with `site.app.status`. Updating the same public URL is the default. Create a new site only when no existing site is resolved or the user explicitly asks for a new site, new URL, or separate prototype.

1. Call `site.app.status` with the known `siteID`, slug, or empty input for the current conversation.
2. If `site.app.status` returns `ambiguous`, do not choose randomly. Show the candidate titles, descriptions, archetypes, owners, and URLs, then ask the user which site to update.
3. If no site is resolved, call `site.app.create` with a short DNS-safe slug, title, description, idea, purpose, audience, archetype, and domain keywords, then call `site.app.status` for the new `siteID`.
4. Use `sourceWorkspacePath` and `appWorkspacePath` exactly as returned. Site source file paths must be rooted at the returned `sourceWorkspacePath`; never pass bare relative paths such as `app/src/App.tsx`, `DESIGN.md`, or `.internkim/artifact-brief.md` to workspace scripts or capabilities.
5. If `workspaceHealth` is `missing` or `permission_problem`, call `site.app.repair`, then call `site.app.status` again. If `workspaceHealth` is `stale_build`, continue to editing or build; do not repair.
6. Choose a UI archetype before editing source: landing, dashboard, admin tool, booking, marketplace, portfolio, or content site.
7. Use `sourceManifest` from `site.app.status` to decide which workspace-local control files exist. Read `<sourceWorkspacePath>/.internkim/site.json`, `<sourceWorkspacePath>/.internkim/idea.md`, `<sourceWorkspacePath>/.internkim/artifact-brief.md`, and `<sourceWorkspacePath>/.internkim/review-log.json` only when the manifest marks them present. Missing optional control files are normal; create or update them under `sourceWorkspacePath` before source edits when they are relevant.
8. Write or update `<sourceWorkspacePath>/.internkim/idea.md` when the user changes the core idea, audience, purpose, or positioning. Keep implementation notes out of `<sourceWorkspacePath>/DESIGN.md`.
9. Write or update `<sourceWorkspacePath>/.internkim/artifact-brief.md` before source edits. Keep it short and natural-language: request intent, audience, archetype, service mode if relevant, main workflow, visual direction, must-show source content, forbidden invented content, and what would be too shallow.
10. Write or update Stitch-compatible `<sourceWorkspacePath>/DESIGN.md`, then create `<sourceWorkspacePath>/app/src/prototype-data.ts` with the supplied source data or clearly fake workflow state before editing UI. Keep source-backed facts in this data module so labels, prices, dates, menus, and policies can be checked instead of scattered through the UI.
11. Replace the starter `<sourceWorkspacePath>/app/src/App.tsx` entirely. Do not preserve starter copy, scaffold structure, or generic feature-card sections.
12. Use task-local scripts run through `terminal.run` to update `<sourceWorkspacePath>/app/src/App.tsx`, `<sourceWorkspacePath>/app/src/main.tsx` only when needed, `<sourceWorkspacePath>/app/src/index.css`, and app-owned data/source files according to `<sourceWorkspacePath>/DESIGN.md` and `<sourceWorkspacePath>/.internkim/artifact-brief.md`. Do not use shell heredocs or redirection for source writes. Do not import React, React DOM, lucide-react, Radix, Tailwind packages, class-variance-authority, tailwind-merge, clsx, or any other package. Use semantic HTML, DOM APIs, CSS classes, inline SVG icons, and local data. Do not write `app/package.json`, `app/index.html`, `app/scripts/build.ts`, `app/scripts/preview.ts`, `app/tsconfig.json`, or `app/vite.config.ts`; those scaffold/build contract files are managed by `site.app.create`.
13. Call `site.app.build`. It resolves the canonical `appWorkspacePath`, runs `bun scripts/build.ts` there, writes `.internkim/build-quality.json`, and returns build evidence plus any quality issues. Quality issues are a revision checklist, not a delivery blocker. Use raw `terminal.run` with `workingDirectoryPath` set to `<appWorkspacePath>` only as a fallback when `site.app.build` is unavailable.
14. Call `site.app.preview` when a browser-accessible draft is useful, or start a local preview with `terminal.run mode=session_start` when local browser tools are the better fit. Capture desktop and mobile screenshots with browser tools when available, and inspect the rendered text for the source checklist before publishing. Then call `artifact.review` with the screenshots, artifact brief, source summary, archetype, and rubric.
15. Write `.internkim/review-log.json` with `attempts`, `reviewedArtifacts`, `issues`, `changesMade`, `remainingNotes`, and either screenshot paths or `visualReviewUnavailable: true`. Revise and rebuild when the rendered images or review notes show useful improvements and the improvement budget remains; repeat at most three times.
16. Call `site.app.publish` with `siteID` and a concise revision message. Same-site updates must publish the same `publishedURL`; successful publish closes the temporary preview. If the build produced a fresh `app/dist` but visual or quality warnings remain after the improvement budget, publish with those warnings and report the top remaining issues.
17. Call `site.app.status` for the same `siteID` and confirm the status is `published`.
18. Reply in Mattermost with the public URL, revision summary, how to try the main workflow, rollback availability, and any test login credentials.

Do not publish the uncustomized starter for a website creation request. The starter is only a safe scaffold while the real prototype is being written.

Never say the website is ready, created, prepared, available, or done unless the site status is `published` after `site.app.publish`. A preview URL is only a temporary draft review URL, not completion. If build or publish fails, report the actual failure and distinguish any preview from the final public URL.

## Owner Site Audit

When the user asks to check all requester-deployed sites, call `site.app.status` with `scope=mine` and `checkLive=true`.

Treat each site as dead when `status` is `failed`, `workspaceHealth` is missing or not usable, or `liveHTTPStatus` is present and not `200`. For each dead site, run `site.app.repair`, then `site.app.build`, then `site.app.publish`. When a previous good published version exists, use `site.app.restore` instead. Report the outcome for each site.

Do not ask for approval before `site.app.create`, `terminal.run` builds, `site.app.preview`, `site.app.publish`, `site.app.status`, `site.app.logs`, or `site.app.restore`. `site.app.publish` is a normal part of creating a website prototype and never needs `ask.confirm`. Ask for approval before `site.app.rollback`, `site.app.unpublish`, or `site.app.delete`.

Use `ask.confirm` only for rollback, unpublish, or delete requests. Do not use `ask.confirm` for create, build, publish, status, logs, or restore.

Never ask for publish approval in natural language. Chat replies such as "확인해 주세요", "승인해 주세요", "말씀해 주시면 게시하겠습니다", or "다시 명령해 주세요" do not create a runtime approval job and cannot resume automatically.

For follow-up feedback in the same conversation, call `site.app.status` with an empty input or the known slug. The tool can resolve the current conversation's bound site. Read the existing `<sourceWorkspacePath>/DESIGN.md`, `<sourceWorkspacePath>/app/src/prototype-data.ts`, `<sourceWorkspacePath>/app/src/App.tsx`, `<sourceWorkspacePath>/app/src/index.css`, and `<sourceWorkspacePath>/.internkim/review-log.json` when present, update the returned workspace, rebuild from `<appWorkspacePath>`, run visual review, publish the same site, and reply with the same URL.

Each site has metadata that explains what it is and who can edit it. Treat `description`, `idea`, `purpose`, `audience`, `archetype`, `domainKeywords`, `createdBy`, `ownerIdentity`, and `collaborators` from `site.app.status` as the decision context for whether a follow-up request should update an existing site. The creator or owner is the default editor. If the current requester cannot edit the site, do not work around the permission boundary; ask the owner to grant access or ask the user to create a separate site.

If the user replies with a short continuation such as "해줘", "진행", "확인", "좋아", "응", "게시해", "배포해", or "publish", treat it as an instruction to finish the current site workflow. Resolve the current site with `site.app.status`, check whether the source is customized beyond the starter, complete missing implementation work if needed, build, publish, and then reply with the public URL. Do not repeat an approval request for publish.

Use this terminal pattern after source files are written:

```json
{
  "workingDirectoryPath": "<appWorkspacePath>",
  "command": "bun scripts/build.ts"
}
```

Do not run this as a single shell command that starts with `cd <appWorkspacePath>`. The working directory belongs in the tool input, not inside the shell command.

The build command itself should stay short:

```bash
bun scripts/build.ts
```

After terminal build success:

- call `site.app.preview` for a temporary draft URL, or start preview from `<appWorkspacePath>` with `bun run preview -- --host 127.0.0.1 --port 4173`; if the port is busy, use the next open port
- use browser tools to inspect `http://127.0.0.1:<port>` at desktop and mobile widths when available
- check for text overflow, overlapping controls, clipped buttons, empty first screens, excessive whitespace, a one-note palette, and missing app-owned control styles
- call `artifact.review` with desktop and mobile screenshots when screenshots are available; include `<sourceWorkspacePath>/.internkim/artifact-brief.md`, the source summary, and the archetype so the LLM judges the rendered result against the intended artifact
- if browser tools or screenshots are unavailable, write `<sourceWorkspacePath>/.internkim/review-log.json` with `visualReviewUnavailable: true` and rely on build and code inspection rather than pretending visual QA ran
- call `site.app.publish` with `siteID` and a human-readable `message`
- never claim deployment succeeded until the tool succeeds
- after publish succeeds, call `site.app.status` and make sure `status` is `published`
- if publish says `app/dist is stale`, run `bun scripts/build.ts` from `app/` again before publishing
- if publish fails, summarize the actual failure and stop

## Cost and Integration Guardrails

Do not ask for API keys, OAuth credentials, webhook URLs, cloud database credentials, payment provider credentials, or paid SaaS setup by default.

If the user requests an external integration, explain that it may require a paid account, API key, OAuth setup, or extra permissions. Continue only after explicit confirmation.

Use fake data, local PocketBase collections, and simple local workflows whenever possible.

When the user supplied concrete source data, use that data instead of generic fake content. Fake workflow state is acceptable only for values the source does not provide and should be visibly marked as fake or "Not provided" when it could be mistaken for a fact.

The default scaffold already includes dependency-free TypeScript, CSS, an offline Bun build, a static preview server, and the DESIGN.md linter. Avoid adding analytics, hosted databases, AI APIs, payment SDKs, email providers, SMS providers, package dependencies, or SaaS clients unless explicitly requested and confirmed.

## Auth Defaults

If login is needed, use PocketBase email or username plus password auth.

Do not add Google login, Slack login, magic links, SMS, SSO, OAuth, or payment-provider login unless explicitly requested.

If a test account is useful, create a prototype seed account and include the credentials in the final Mattermost reply.

Use clearly fake credentials for test users. Do not ask the user for real passwords.

## Design System Workflow

`DESIGN.md` is required for every creation or update. Treat it as the working system prompt for the website. It must be specific to the user's request and must not copy a generic template.

Write `DESIGN.md` in the Stitch canonical format only:

- YAML front matter between `---` delimiters
- token groups: `colors`, `typography`, `rounded`, `spacing`, and `components`
- markdown sections in this order: `Overview`, `Colors`, `Typography`, `Layout`, `Elevation & Depth`, `Shapes`, `Components`, and `Do's and Don'ts`

Do not add Kim Intern-specific implementation sections such as Product, Audience, Prototype Scope, Workflows, Implemented Now, Next Iterations, or Acceptance Criteria. Work those decisions into the canonical sections or keep them in your own execution notes.

The `components` tokens should describe semantic control classes such as `button-primary`, `button-primary-hover`, `card`, `input`, `tabs`, `dialog`, `table`, and `badge`.

If the user does not specify a design direction, default the visual system to black-on-white minimal: white background, near-black text, quiet gray borders, restrained monochrome controls, and no dark navy shell. Infer structure from the domain and audience, but do not invent a blue, slate, purple, or gradient theme unless the request clearly calls for it. A restaurant, portfolio, operations dashboard, campaign site, internal tool, game, and marketplace should not share the same visual structure. The first screen must be the actual usable experience or requested landing page, not generic feature cards.

For long or broad requests, publish a coherent first version instead of a shallow explanation page. Keep deferred work out of `DESIGN.md`; summarize it only in the final reply if useful.

Use familiar controls and simple inline SVG or CSS icons where helpful. Keep layouts responsive, text readable, and controls stable across mobile and desktop. Avoid decorative filler, generic SaaS copy, dark-mode-by-default pages, navy/slate dominance, and a single default palette across unrelated sites.

Use a restrained responsive type scale: body text should stay readable on mobile, labels and values should align without clipping, and headings inside panels should be compact rather than hero-sized. Keep vertical rhythm consistent between Korean and English labels. Check desktop and mobile screenshots for lopsided whitespace, crowded control clusters, text touching card edges, and important actions pushed below the first viewport.

## UI Archetypes

Pick one archetype before writing source:

- landing: first viewport has the offer, proof, primary action, and a visible hint of the next section
- dashboard: dense app shell with metrics, filters, table/list, and a primary workflow
- admin tool: sidebar or topbar, searchable records, detail panel, and destructive actions behind dialogs
- booking: calendar or availability rail, service choices, form inputs, and confirmation state
- marketplace: browse grid/list, filters, detail preview, and selection or checkout placeholder
- portfolio: work index, rich project detail, contact path, and strong visual hierarchy
- content site: article/index structure, clear navigation, readable typography, and related content

Infer the archetype from the domain and user goal when unspecified. App requests should default to a usable app shell instead of a landing page.

## Design Quality Bar

Never publish a generic feature-card page, empty hero, meaningless gradient, or workflow-free app. The first screen must be the requested experience or a meaningful landing page for the requested offer. Use realistic fake data where it helps the user understand the workflow.

Use a simple LLM-led artifact quality loop: artifact brief first, deterministic build-quality second, rendered desktop/mobile screenshot review third, and same-URL publish after improvement attempts. Compile failures, missing dist, stale dist, permission failures, and starter scaffold leakage are hard blockers. Aesthetic, density, information-architecture, and workflow-completeness findings are soft review notes: fix them when useful and budget remains, otherwise publish the usable artifact and report the top remaining notes with the URL.

For booking, reservation, checkout, calculator, dashboard, or admin-tool prompts, the first viewport should expose the actual workflow controls and current state. Do not hide the requested workflow below a large brand hero. Verify desktop and mobile screenshots show usable controls, a source-backed summary, and the requested confirmation or status state without horizontal scrolling or overlapping text.

For service-capable prototypes, keep the same loop. If the request needs login, saved records, files, realtime, reservations, admin state, or CRUD, use PocketBase as the default local backend unless the user explicitly asks for another backend. Let the LLM design the collections, rules, seed data, and test account from the artifact brief. Runtime safety should preserve project data across publish, unpublish, restore, and rollback; do not treat every site as a database app by default.

## Reply Format

For a successful publish, reply with:

- public URL
- what changed
- how to try the main workflow
- test login credentials, only if the app has login
- note that it is a prototype for idea validation, not production software

Keep the reply short. The user is non-technical and wants to try the link.

Do not use this successful publish reply format for draft sites. A draft site is not visible to the user.

## Takedown

- Use `site.app.unpublish` when the user asks to take the prototype down temporarily.
- Use `site.app.restore` when the user asks to bring it back.
- Use `site.app.history` when the user asks what changed or wants versions.
- Use `site.app.diff` when the user asks for a revision comparison.
- Use `site.app.rollback` when the user asks to return to the previous published version or a specific revision.
- Use `site.app.delete` only after `ask.confirm` succeeds. Pass `confirm: "DELETE"` and `userConfirmed: true`. Deletion is irreversible except from backups.
