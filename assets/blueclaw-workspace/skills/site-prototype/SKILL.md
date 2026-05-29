---
name: site-prototype
description: Create, publish, update, take down, restore, or delete free dependency-light website prototypes through InternKim site.app tools.
when_to_use: Use when the user asks InternKim to make, deploy, publish, update, fix, take down, restore, or delete a website, web app, prototype, demo, landing page, dashboard, or app idea.
allowed-tools:
  - terminal.run
  - terminal.session
  - browser.open
  - browser.snapshot
  - browser.screenshot
  - file.write
  - site.app.create
  - site.app.build
  - site.app.repair
  - site.app.publish
  - site.app.status
  - site.app.history
  - site.app.diff
  - site.app.logs
  - site.app.rollback
  - site.app.unpublish
  - site.app.restore
  - site.app.delete
  - artifact.review
  - user.confirm
completion:
  requiredEvidenceTools:
    - site.app.status
    - site.app.build
    - artifact.review
    - site.app.publish
---

# Site Prototype

Use this skill when a user asks InternKim to create, publish, update, take down, restore, or delete a website or web app prototype.

## Purpose

Create prototypes for non-developers to validate ideas quickly. Do not present the result as production-ready software. Do not add production claims, SLA language, compliance guarantees, payment processing, real customer data workflows, or paid hosted services unless the user explicitly requests that path and confirms the cost or account requirement.

## Stack

- Use the managed React + Vite + TypeScript + Tailwind + shadcn/ui scaffold for the frontend.
- Use PocketBase files and migrations only when the prototype needs local data, auth, files, realtime, or migrations.
- Do not use Next.js, SvelteKit, arbitrary Node servers, cloud databases, hosted backends, or paid APIs for v1 prototypes unless the user explicitly asks for that stack.
- The managed build may run `bun install` inside `app/` on first build. Do not add extra dependencies unless the request truly needs them.
- `site.app.create` initializes the editable website project scaffold, not a finished website.
- Use the returned `sourceWorkspacePath` exactly as the canonical virtual workspace path for all follow-up source writes. Do not rewrite it into a concrete private POSIX path.
- Use the returned `appWorkspacePath` exactly as the build working directory.

## Workflow

For create, make, build, deploy, publish, prototype, demo, landing page, dashboard, or app requests, the task is not complete until `site.app.publish` succeeds. Do not stop after `site.app.create`. Do not ask the user to review a draft URL before publishing; draft site URLs return 404 and are not useful for review.

Start by resolving the existing site for the current conversation or slug with `site.app.status`. Updating the same public URL is the default. Create a new site only when no existing site is resolved or the user explicitly asks for a new site, new URL, or separate prototype.

1. Call `site.app.status` with the known `siteID`, slug, or empty input for the current conversation.
2. If `site.app.status` returns `ambiguous`, do not choose randomly. Show the candidate titles, descriptions, archetypes, owners, and URLs, then ask the user which site to update.
3. If no site is resolved, call `site.app.create` with a short DNS-safe slug, title, description, idea, purpose, audience, archetype, and domain keywords, then call `site.app.status` for the new `siteID`.
4. Use `sourceWorkspacePath` and `appWorkspacePath` exactly as returned.
5. If `workspaceHealth` is `missing` or `permission_problem`, call `site.app.repair`, then call `site.app.status` again. If `workspaceHealth` is `stale_build`, continue to editing or build; do not repair.
6. Choose a UI archetype before editing source: landing, dashboard, admin tool, booking, marketplace, portfolio, or content site.
7. Read `.internkim/site.json` and `.internkim/idea.md` when present. The registry metadata is authoritative, but these files are the workspace-local copy of what the site is and why it exists.
8. Write or update `.internkim/idea.md` when the user changes the core idea, audience, purpose, or positioning. Keep implementation notes out of `DESIGN.md`.
9. Write or update Stitch-compatible `DESIGN.md`, then create `app/src/prototype-data.ts` with domain-specific fake data and workflow state before editing UI.
10. Replace the starter `app/src/App.tsx` entirely. Do not preserve starter copy, scaffold structure, or generic feature-card sections.
11. Use `file.write` to update `app/src/App.tsx`, `app/src/index.css`, app-owned components, and app-owned data/source files according to `DESIGN.md`. Do not write `app/package.json`, `app/index.html`, `app/scripts/build.ts`, `app/tsconfig.json`, or `app/vite.config.ts`; those scaffold/build contract files are managed by `site.app.create`.
12. Call `site.app.build`. It resolves the canonical `appWorkspacePath`, runs `bun scripts/build.ts` there, writes `.internkim/build-quality.json`, and returns build evidence. Use raw `terminal.run` with `workingDirectoryPath` set to `<appWorkspacePath>` only as a fallback when `site.app.build` is unavailable.
13. Start a local preview with `terminal.session`, capture desktop and mobile screenshots with browser tools when available, then call `artifact.review` with the screenshots, intent, archetype, and rubric.
14. Write `.internkim/review-log.json` with deterministic checks, vision review issues, attempt count, accepted warnings, and final decision. Revise and rebuild when any blocking issue remains; repeat at most three times.
15. Call `site.app.publish` with `siteID` and a concise revision message. Same-site updates must publish the same `publishedURL`.
16. Call `site.app.status` for the same `siteID` and confirm the status is `published`.
17. Reply in Mattermost with the public URL, revision summary, how to try the main workflow, rollback availability, and any test login credentials.

Do not publish the uncustomized starter for a website creation request. The starter is only a safe scaffold while the real prototype is being written.

Never say the website is ready, created, prepared, available, previewable, or done unless the site status is `published` after `site.app.publish`. If build or publish fails, report the actual failure and do not provide the draft URL as something the user can open.

Do not ask for approval before `site.app.create`, `terminal.run` builds, `site.app.publish`, `site.app.status`, `site.app.logs`, or `site.app.restore`. `site.app.publish` is a normal part of creating a website prototype and never needs `user.confirm`. Ask for approval before `site.app.rollback`, `site.app.unpublish`, or `site.app.delete`.

Use `user.confirm` only for rollback, unpublish, or delete requests. Do not use `user.confirm` for create, build, publish, status, logs, or restore.

Never ask for publish approval in natural language. Chat replies such as "확인해 주세요", "승인해 주세요", "말씀해 주시면 게시하겠습니다", or "다시 명령해 주세요" do not create a runtime approval job and cannot resume automatically.

For follow-up feedback in the same conversation, call `site.app.status` with an empty input or the known slug. The tool can resolve the current conversation's bound site. Read the existing `DESIGN.md`, `app/src/prototype-data.ts`, `app/src/App.tsx`, `app/src/index.css`, and `.internkim/review-log.json` when present, update the returned workspace, rebuild from `app/`, run visual review, publish the same site, and reply with the same URL.

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

- start preview from `<appWorkspacePath>` with `bun run preview -- --host 127.0.0.1 --port 4173`; if the port is busy, use the next open port
- use browser tools to inspect `http://127.0.0.1:<port>` at desktop and mobile widths when available
- check for text overflow, overlapping controls, clipped buttons, empty first screens, excessive whitespace, a one-note palette, and missing shadcn token usage
- call `artifact.review` with desktop and mobile screenshots when screenshots are available; use the returned blocking/warning issues as the revision checklist
- if browser tools or screenshots are unavailable, write `.internkim/review-log.json` with `visionReviewUnavailable: true` and rely on build and code inspection rather than pretending visual QA ran
- call `site.app.publish` with `siteID` and a human-readable `message`
- never claim deployment succeeded until the tool succeeds
- after publish succeeds, call `site.app.status` and make sure `status` is `published`
- if publish says `app/dist is stale`, run `bun scripts/build.ts` from `app/` again before publishing
- if publish fails, summarize the actual failure and stop

## Cost and Integration Guardrails

Do not ask for API keys, OAuth credentials, webhook URLs, cloud database credentials, payment provider credentials, or paid SaaS setup by default.

If the user requests an external integration, explain that it may require a paid account, API key, OAuth setup, or extra permissions. Continue only after explicit confirmation.

Use fake data, local PocketBase collections, and simple local workflows whenever possible.

The default scaffold already includes React, Vite, Tailwind, shadcn-style local components, lucide icons, and the DESIGN.md linter. Avoid adding analytics, hosted databases, AI APIs, payment SDKs, email providers, SMS providers, or SaaS clients unless explicitly requested and confirmed.

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

The `components` tokens should describe shadcn-style primitives such as `button-primary`, `button-primary-hover`, `card`, `input`, `tabs`, `dialog`, `table`, and `badge`.

If the user does not specify a design direction, infer one from the domain and audience. A restaurant, portfolio, operations dashboard, campaign site, internal tool, game, and marketplace should not share the same visual structure. The first screen must be the actual usable experience or requested landing page, not generic feature cards.

For long or broad requests, publish a coherent first version instead of a shallow explanation page. Keep deferred work out of `DESIGN.md`; summarize it only in the final reply if useful.

Use familiar controls and lucide icons where helpful. Keep layouts responsive, text readable, and controls stable across mobile and desktop. Avoid decorative filler, generic SaaS copy, and a single default palette across unrelated sites.

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

Use the shared artifact quality harness from `references/artifact-quality.md` when you need more detail, but keep the site loop self-contained: deterministic build-quality first, rendered screenshot review second, same-URL publish only after blocking issues are gone.

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
- Use `site.app.delete` only after `user.confirm` succeeds. Pass `confirm: "DELETE"` and `userConfirmed: true`. Deletion is irreversible except from backups.
