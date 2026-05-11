---
name: site-prototype
description: Create, publish, update, take down, restore, or delete free React + PocketBase website prototypes through InternKim site.app tools.
when_to_use: Use when the user asks InternKim to make, deploy, publish, update, fix, take down, restore, or delete a website, web app, prototype, demo, landing page, dashboard, or app idea.
allowed-tools:
  - terminal.run
  - terminal.session
  - file.write
  - site.app.create
  - site.app.publish
  - site.app.status
  - site.app.logs
  - site.app.rollback
  - site.app.unpublish
  - site.app.restore
  - site.app.delete
  - user.confirm
completion:
  requiredEvidenceTools:
    - site.app.create
    - terminal.run
    - site.app.publish
---

# Site Prototype

Use this skill when a user asks InternKim to create, publish, update, take down, restore, or delete a website or web app prototype.

## Purpose

Create prototypes for non-developers to validate ideas quickly. Do not present the result as production-ready software. Do not add production claims, SLA language, compliance guarantees, payment processing, real customer data workflows, or paid hosted services unless the user explicitly requests that path and confirms the cost or account requirement.

## Stack

- Use React + Vite + TypeScript for the frontend.
- Use PocketBase for local prototype data, auth, files, realtime, and migrations.
- Do not use Next.js, SvelteKit, arbitrary Node servers, cloud databases, hosted backends, or paid APIs for v1 prototypes.
- `site.app.create` reserves an editable source workspace, not a finished website.
- Use the returned `sourceWorkspacePath` as the canonical workspace path for all follow-up source writes.

## Workflow

For create, make, build, deploy, publish, prototype, demo, landing page, dashboard, or app requests, the task is not complete until `site.app.publish` succeeds. Do not stop after `site.app.create`. Do not ask the user to review a draft URL before publishing; draft site URLs return 404 and are not useful for review.

1. For a new prototype, call `site.app.create` with a short DNS-safe slug and title.
2. Pass `prompt`, `designBrief`, or `prototypeScope` when the user gives enough detail.
3. After `site.app.create` succeeds, call `site.app.status` for the same `siteID` and use `sourceWorkspacePath`.
4. Write `DESIGN.md` into the site workspace before writing app source.
5. Create missing `app/package.json`, `app/index.html`, `app/src/main.tsx`, `app/src/App.tsx`, and CSS files, or update the existing source, according to `DESIGN.md`.
6. Run `bun install && bun run build` from `<sourceWorkspacePath>/app`.
7. Call `site.app.publish` with `siteID` and a concise `message`.
8. Call `site.app.status` for the same `siteID` and confirm the status is `published`.
9. Reply in Mattermost with the public URL, a short change summary, how to try the main workflow, and any test login credentials.

Do not publish the uncustomized starter for a website creation request. The starter is only a safe scaffold while the real prototype is being written.

Never say the website is ready, created, prepared, available, previewable, or done unless the site status is `published` after `site.app.publish`. If build or publish fails, report the actual failure and do not provide the draft URL as something the user can open.

Do not ask for approval before `site.app.create`, `terminal.run` builds, `site.app.publish`, `site.app.status`, `site.app.logs`, or `site.app.restore`. `site.app.publish` is a normal part of creating a website prototype and never needs `user.confirm`. Ask for approval before `site.app.rollback`, `site.app.unpublish`, or `site.app.delete`.

Use `user.confirm` only for rollback, unpublish, or delete requests. Do not use `user.confirm` for create, build, publish, status, logs, or restore.

Never ask for publish approval in natural language. Chat replies such as "확인해 주세요", "승인해 주세요", "말씀해 주시면 게시하겠습니다", or "다시 명령해 주세요" do not create a runtime approval job and cannot resume automatically.

For follow-up feedback in the same conversation, call `site.app.status` with an empty input or the known slug. The tool can resolve the current conversation's bound site. Read the existing `DESIGN.md`, update it for the new request, edit the returned workspace, rebuild from `app/`, publish the same site, and reply with the same URL.

If the user replies with a short continuation such as "해줘", "진행", "확인", "좋아", "응", "게시해", "배포해", or "publish", treat it as an instruction to finish the current site workflow. Resolve the current site with `site.app.status`, check whether the source is customized beyond the starter, complete missing implementation work if needed, build, publish, and then reply with the public URL. Do not repeat an approval request for publish.

Use this command pattern after source files are written:

```bash
cd <sourceWorkspacePath>/app
bun install
bun run build
```

After terminal build success:

- call `site.app.publish` with `siteID` and a human-readable `message`
- never claim deployment succeeded until the tool succeeds
- after publish succeeds, call `site.app.status` and make sure `status` is `published`
- if publish says `app/dist is stale`, run `bun run build` from `app/` again before publishing
- if publish fails, summarize the actual failure and stop

## Cost and Integration Guardrails

Do not ask for API keys, OAuth credentials, webhook URLs, cloud database credentials, payment provider credentials, or paid SaaS setup by default.

If the user requests an external integration, explain that it may require a paid account, API key, OAuth setup, or extra permissions. Continue only after explicit confirmation.

Use fake data, local PocketBase collections, and simple local workflows whenever possible.

Safe default dependencies are React, Vite, TypeScript, PocketBase JS SDK, and lucide-react. Avoid adding large UI frameworks, analytics, hosted databases, AI APIs, payment SDKs, email providers, SMS providers, or SaaS clients unless explicitly requested and confirmed.

## Auth Defaults

If login is needed, use PocketBase email or username plus password auth.

Do not add Google login, Slack login, magic links, SMS, SSO, OAuth, or payment-provider login unless explicitly requested.

If a test account is useful, create a prototype seed account and include the credentials in the final Mattermost reply.

Use clearly fake credentials for test users. Do not ask the user for real passwords.

## Design System Workflow

`DESIGN.md` is required for every creation or update. Treat it as the working system prompt for the website. It must be specific to the user's request and must not copy a generic template.

Write `DESIGN.md` with these sections:

- Product: what this site or app is for
- Audience: who uses it and what they are trying to do
- Prototype Scope: what works in this publish and what is intentionally deferred
- Visual Direction: tone, layout density, typography, colors, spacing, and interaction feel
- Screens: each first-version screen or state the user can try
- Workflows: the main interaction paths
- Data Model: local data, fake data, PocketBase collections, or state shape
- Implemented Now: concrete functionality included in this publish
- Next Iterations: useful follow-up work for longer projects
- Acceptance Criteria: checks that must pass before publish

If the user does not specify a design direction, infer one from the domain and audience. A restaurant, portfolio, operations dashboard, campaign site, internal tool, game, and marketplace should not share the same visual structure. The first screen must be the actual usable experience or requested landing page, not generic feature cards.

For long or broad requests, publish a coherent first version instead of a shallow explanation page. Record deferred work in `Next Iterations`, but make the published app usable for at least one complete workflow.

Use familiar controls and lucide icons where helpful. Keep layouts responsive, text readable, and controls stable across mobile and desktop. Avoid decorative filler, generic SaaS copy, and a single default palette across unrelated sites.

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
- Use `site.app.rollback` when the user asks to return to the previous published version.
- Use `site.app.delete` only after `user.confirm` succeeds. Pass `confirm: "DELETE"` and `userConfirmed: true`. Deletion is irreversible except from backups.
