---
name: site-prototype
description: Create, publish, update, take down, restore, or delete free dependency-light website prototypes through site capability operations.
when_to_use: Use when the user asks the assistant to make, deploy, publish, update, fix, take down, restore, or delete a website, web app, prototype, demo, landing page, dashboard, or app idea.
completion:
  requiredEvidenceTools:
    - site.status
    - artifact.review
    - site.publish
---

# Site Prototype

Use this skill for website and web app prototypes. Run site, browser, and review operations through the capability.invoke tool: set `operation` to the operation name (`site.create`, `site.preview`, `site.publish`, `site.status`, `site.history`, `site.diff`, `site.logs`, `site.rollback`, `site.unpublish`, `site.restore`, `site.delete`, or `artifact.review`) and `input` to that operation's fields as one JSON object written inside a string, e.g. `"{\"siteID\":\"abc123\"}"` — never empty. Basic creation and content edits need no build step; `terminal.run` (`bun scripts/build.ts`) is only for structural changes under `app/src/**` or scaffold config.

Create validation prototypes, not production software. Do not claim production readiness, compliance, SLA, paid hosting, payment support, real customer workflows, or external integrations unless the user explicitly requests and confirms that path.

## Source Truth

User-provided data is the source of truth. Preserve company names, products, people, dates, prices, menu items, locations, policies, and confirmation states exactly unless the user asks for translation or normalization. Do not invent missing vendors, reviews, discounts, delivery options, payment methods, addresses, phone numbers, integrations, or availability. If a useful value is missing, render the user's-language equivalent of "Not provided".

Source-backed facts must appear in the rendered text, not only in `app/public/site-content.json` or the final reply. Keep a source checklist in `.internkim/artifact-brief.md` and verify it against rendered text before publish.

Block copy must be realistic, specific prose for the requested business — real service names, concrete benefits, outcome-focused calls to action. Placeholder or numbered filler such as `서비스 1`/`설명 1`, `Service 1`, or `Lorem ipsum` is never acceptable. When the request gives a hero tagline (보조 문구), use it verbatim as the hero block's tagline; do not paraphrase or drop it.

## Stack

- The scaffold is a working React + Tailwind + shadcn black-on-white template composed from reusable blocks. Start from it and customize content; the build installs npm dependencies online.
- Reuse the shadcn primitives in `app/src/components/ui/*` (Button, Card, Badge, Input, Tabs, Dialog, etc.) and the block components in `app/src/blocks/*`. Keep the black-on-white default unless the request asks otherwise.
- Use PocketBase only when the prototype needs local data, auth, files, realtime, or migrations.
- Do not add Next.js, SvelteKit, Node servers, cloud databases, hosted backends, or paid APIs unless the user explicitly asks and accepts the cost/reliability tradeoff.
- Use system fonts by default; embed fonts only as WOFF2 with `@font-face`/`format("woff2")`, never base64 or CDN-only fonts.
- Treat returned `sourceWorkspacePath` and `appWorkspacePath` as canonical; do not rewrite them to private POSIX paths.

## Block Library

`app/public/site-content.json` composes the page from `siteName`, optional `tagline`, and an ordered `blocks` array. Each block sets `variant` plus whichever of `title`, `body`, `items` (`{ title, body }` pairs), `actionLabel`, and `actionHref` the variant needs. Choose and order variants to fit the request instead of repeating one fixed skeleton — a restaurant, a portfolio, and a dashboard should end up with different block sequences.

- `hero`: opening title, tagline body, and one primary call-to-action; use once, first.
- `features`: titled grid of `items` for capabilities, services, or benefits.
- `prose`: titled block of free-form paragraphs for narrative, policy, or about content.
- `cta`: high-contrast banner with a title, short body, and one action button.
- `faq`: titled list of expandable `items` as question/answer pairs.
- `contact`: titled block with body text and one action for reaching out.

## Create, Update, Publish

Website creation and update requests are incomplete until the site.publish operation succeeds and a final site.status operation returns `published`. A preview URL is only a draft. site.create is synchronous: a successful call returns the full site record immediately (`status: "draft"`, plus `siteID`) — there is no build to wait for, so never call site.status in a loop hoping create "finishes."

1. Call the site.status operation once with the known `siteID`, slug, or empty input for the current conversation. Do not call it again until an action you took (create, publish, or a content change) could change the answer.
2. If status is `ambiguous`, show candidate titles, descriptions, archetypes, owners, and URLs, then ask which site to update.
3. If status is `not_found`, go straight to step 5 (site.create). Calling site.status again first changes nothing — it only becomes something other than `not_found` after site.create succeeds.
4. If no site is resolved, choose the UI archetype — landing, dashboard, admin tool, booking, marketplace, portfolio, content site, or a domain-specific app shell — and write a request-specific `DESIGN.md` in Stitch canonical format: YAML front matter with `colors`, `typography`, `rounded`, `spacing`, and `components`, followed by `Overview`, `Colors`, `Typography`, `Layout`, `Elevation & Depth`, `Shapes`, `Components`, and `Do's and Don'ts`. Write it at `sourceWorkspacePath` (a sibling of `app/`) — never at `app/DESIGN.md`; publish only looks for `DESIGN.md` at the source root, and misplacing it also falsely marks the build stale. Publish renders `colors` and `typography` into `theme.css`, so pick them before site.create on every create or update.

   Publish validates the whole contract at once: all five top-level keys, `colors.primary`/`colors.background` as hex, one `typography` entry with `fontFamily`, and a resolvable `rounded`. Copy this minimal valid skeleton verbatim, then replace the values and add richer colors/typography/spacing/components detail:

   ```yaml
   ---
   colors:
     primary: "#111111"
     background: "#ffffff"
   typography:
     display:
       fontFamily: ui-serif
     body:
       fontFamily: ui-sans-serif
   rounded: 8px
   spacing: 16px
   components:
     button:
       radius: 8px
   ---
   ```
5. Call the site.create operation **exactly once**, with `input` holding every field as one JSON object written inside a string — never empty. `slug` is the only required field, but always include `title`, `description`, `idea`, `purpose`, `audience`, `archetype`, and `domainKeywords` in that same call so the record starts complete. Example call:

   ```json
   {
     "operation": "site.create",
     "input": "{\"slug\":\"banchan-table\",\"title\":\"Banchan Table Reservations\",\"description\":\"Weekly Korean meal-kit pickup reservation prototype\",\"idea\":\"Let busy Seoul households reserve a Thursday meal kit for weekend pickup\",\"purpose\":\"Test whether users understand the reservation flow without explanation\",\"audience\":\"busy single-person households in Seoul\",\"archetype\":\"booking\",\"domainKeywords\":[\"reservation\",\"meal-kit\",\"pickup\"]}"
   }
   ```

   If it fails with `invalid_input`, fill in every field the error lists and call site.create exactly one more time. Never call site.create a third time for the same site, never resend the same failing input unchanged, and never substitute polling site.status for fixing the input — status cannot repair a create that never succeeded. If the second attempt also fails, stop and report the failure instead of retrying further.
6. Compose the page by writing `app/public/site-content.json` with an ordered `blocks` array from the Block Library above — block choice and order is how the page structure gets decided.
7. If `workspaceHealth` is `missing` or `permission_problem`, call site.status once more to recheck; if it stays unhealthy, report the problem honestly instead of guessing. If it is `stale_build`, edit or build.
8. Read control files only when `sourceManifest` marks them present: `.internkim/site.json`, `.internkim/idea.md`, `.internkim/artifact-brief.md`, `.internkim/review-log.json`.
9. Update `.internkim/idea.md` when idea, audience, purpose, or positioning changes.
10. Write `.internkim/artifact-brief.md` before source edits. Include request intent, audience, archetype, workflow, visual direction, must-show source content, forbidden invented content, and what would be too shallow.
11. For a content-only change — copy, tagline, block text, block order, or block additions/removals with no new component — rewrite `app/public/site-content.json` directly and skip straight to step 19 (site.publish); there is no `app/src/**` edit and no build step for a content-only change.
12. For a structural change beyond the block library — a new block variant, a new primitive, or a layout the existing blocks cannot express — edit `app/src/blocks/*` or `app/src/App.tsx` and `app/src/index.css`, reusing the shadcn primitives.
13. Do not edit managed scaffold files: `app/package.json`, `app/index.html`, `app/scripts/build.ts`, `app/scripts/preview.ts`, `app/tsconfig.json`, or `app/vite.config.ts`. `app/public/site-content.json` is the primary content-editing surface and is not on this list.
14. Only after a structural change (step 12) or a scaffold config edit, build the app with `terminal.run` running `bun scripts/build.ts` from `appWorkspacePath`; it writes `.internkim/build-quality.json`. A basic create or a content-only edit (step 11) needs no build step.
15. Use the site.preview operation or local preview only for visual QA. Capture desktop and mobile screenshots when browser capability operations are available.
16. Call `artifact.review` with screenshots, artifact brief, source summary, archetype, and rubric. Inspect rendered text for the source checklist.
17. Write `.internkim/review-log.json` with attempts, reviewed artifacts, issues, changes made, remaining notes, and screenshot paths or `visualReviewUnavailable: true`.
18. Revise and rebuild when screenshots or review notes show useful improvements and budget remains; repeat at most three times.
19. Call the site.publish operation with the `siteID` from step 5's create result (or step 1/3's status result) and a concise revision `message`: `{"operation": "site.publish", "input": "{\"siteID\":\"<siteID>\",\"message\":\"Initial reservation flow\"}"}`.
20. Call the site.status operation again and confirm `status` is `published`.
21. Reply with the public URL, what changed, how to try the main workflow, rollback availability, and test credentials only when login exists.

Use this terminal shape when a structural change needs the fallback build:

```json
{
  "workingDirectoryPath": "<appWorkspacePath>",
  "command": "bun scripts/build.ts"
}
```

Do not run `cd <appWorkspacePath> && bun scripts/build.ts`; the working directory belongs in the tool input.

## Design Quality

`DESIGN.md` is required for every create or update. It must be specific to the request and use Stitch canonical format, decided before site.create so the block composition in step 5 already matches the palette, type, radius, and spacing choices. Its `colors` and `typography` keys are the published theme: publish renders them into `theme.css`, so an unconsidered front matter ships as the visible site.

Default to black-on-white minimal styling unless the request calls for another direction; pick a palette deliberately — editorial (`#111111`/`#ffffff`, accent `#2f6f4f`, serif/sans), playful (`#ff6b4a`/`#fffaf3`, accent `#1c3b52`, rounded-sans), dashboard (`#2563eb`/`#f8fafc`, accent `#0f172a`, sans).

Use no dark navy shell by default, and avoid slate, purple, gradients, decorative filler, generic SaaS cards, or empty heroes. A restaurant, portfolio, dashboard, campaign site, internal tool, game, and marketplace should not share the same block sequence.

The first screen must be the requested usable experience or a meaningful landing page. For booking, checkout, calculator, dashboard, CRUD, or admin-tool prompts, show workflow controls and current state in the first viewport. Verify desktop and mobile screenshots for overlap, clipping, horizontal scroll, excessive whitespace, one-note palette, and missing app-owned control styles.

If login, saved records, files, realtime, reservations, admin state, or CRUD is needed, use PocketBase as the default local backend unless the user explicitly requests another backend. Use fake seed credentials and include them in the final reply.

## Follow-Ups

For feedback in the same conversation, call site.status once with empty input or the known slug, update the resolved site (never site.create again for it), and publish the same URL. Read existing `DESIGN.md` (source root, not `app/`), `app/public/site-content.json`, `app/src/App.tsx`, `app/src/index.css`, and `.internkim/review-log.json` when present. Use `description`, `idea`, `purpose`, `audience`, `archetype`, `domainKeywords`, `createdBy`, `ownerIdentity`, and collaborators from the status result to decide whether a follow-up should update this site.

Short continuations such as "해줘", "진행", "좋아", "응", "게시해", "배포해", or "publish" mean finish the current site workflow: resolve status, complete missing implementation, build, review, publish, and reply with the public URL. Do not ask for publish approval.

## Owner Audit and Destructive Actions

When the user asks to check all requester-deployed sites, call site.status with `scope=mine` and `checkLive=true`. Treat a site as dead when status is `failed`, workspace health is unusable, or live HTTP status is present and not `200`. Repair, build, publish, or restore each site and report outcomes.

Do not ask approval for create, build, preview, publish, status, logs, or restore — site.publish is a normal completion step. Use `ask.confirm` before rollback, unpublish, or delete. For delete, call site.delete only after confirmation succeeds and pass `confirm: "DELETE"` and `userConfirmed: true`.

## Final Reply

For a successful publish, reply briefly with:

- public URL
- what changed
- how to try the main workflow
- test login credentials, only if the app has login
- note that it is a prototype for idea validation, not production software

Never say the website is ready, created, available, or done unless website publish and final website status succeeded.
