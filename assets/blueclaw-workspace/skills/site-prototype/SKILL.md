---
name: site-prototype
description: Create, publish, update, take down, restore, or delete free React + PocketBase website prototypes through InternKim site.app tools.
when_to_use: Use when the user asks InternKim to make, deploy, publish, update, fix, take down, restore, or delete a website, web app, prototype, demo, landing page, dashboard, or app idea.
allowed-tools:
  - terminal.run
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
---

# Site Prototype

Use this skill when a user asks InternKim to create, publish, update, take down, restore, or delete a website or web app prototype.

## Purpose

Create prototypes for non-developers to validate ideas quickly. Do not present the result as production-ready software. Do not add production claims, SLA language, compliance guarantees, payment processing, real customer data workflows, or paid hosted services unless the user explicitly requests that path and confirms the cost or account requirement.

## Stack

- Use React + Vite + TypeScript for the frontend.
- Use PocketBase for local prototype data, auth, files, realtime, and migrations.
- Do not use Next.js, SvelteKit, arbitrary Node servers, cloud databases, hosted backends, or paid APIs for v1 prototypes.
- Build inside the Blueclaw workspace before publishing.

## Workflow

1. For a new prototype, call `site.app.create` with a short DNS-safe slug and title.
2. Work only inside the returned `/workspace/sites/<siteID>` directory.
3. Put the frontend in `app/`.
4. Put PocketBase migrations in `pocketbase/pb_migrations/`.
5. Leave `pocketbase/pb_hooks/` empty unless the user explicitly requested backend hooks and an admin approval path is available.
6. Run `bun install` and `bun run build` from `app/`. Do not publish if the build fails.
7. Call `site.app.publish` with `siteID` and a concise `message`.
8. Reply in Mattermost with the public URL, a short change summary, and any test login credentials.

For follow-up feedback in the same conversation, call `site.app.status` with an empty input or the known slug. The tool can resolve the current conversation's bound site. Edit the returned workspace, rebuild, publish the same site, and reply with the same URL.

Use this command pattern inside the workspace:

```bash
cd /workspace/sites/<siteID>/app
bun install
bun run build
```

After build success:

- call `site.app.publish` with `siteID` and a human-readable `message`
- never claim deployment succeeded until the tool succeeds
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

## Design Defaults

If the user does not specify design direction, use a black-on-white shadcn-style minimal interface:

- white background
- black or neutral text
- thin neutral borders
- 8px or smaller border radius
- restrained spacing
- lucide icons where helpful
- dashboard or tool surfaces instead of marketing pages

The first screen should be usable. Do not create a marketing landing page unless the user asks for one.

Prefer compact, working screens: dashboards, forms, lists, detail views, and stateful controls. Avoid decorative hero sections, generic feature cards, and explanatory in-app copy unless the requested prototype is itself a landing page.

## Reply Format

For a successful publish, reply with:

- public URL
- what changed
- how to try the main workflow
- test login credentials, only if the app has login
- note that it is a prototype for idea validation, not production software

Keep the reply short. The user is non-technical and wants to try the link.

## Takedown

- Use `site.app.unpublish` when the user asks to take the prototype down temporarily.
- Use `site.app.restore` when the user asks to bring it back.
- Use `site.app.rollback` when the user asks to return to the previous published version.
- Use `site.app.delete` only after `user.confirm` succeeds. Pass `confirm: "DELETE"` and `userConfirmed: true`. Deletion is irreversible except from backups.
