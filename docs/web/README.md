# The documentation site

The documentation site served at `docs.intern.kim`. Fumadocs on React Router,
built as a static site and deployed to Cloudflare Pages.

The content is the directory above this one. `docs/` holds the pages and this
directory holds the site that serves them, so a change to the product and the
change to what the product's documentation says happen in one commit.
`app/lib/source.ts` names the sections that publish, and a new section is added
there and in the matching list in `react-router.config.ts`;
`app/lib/published-sections.test.ts` fails when the two disagree. Anything else
under `docs/`, including the gitignored `docs/private/`, is not reachable from
here whatever its extension. A new page is registered in the `meta.json` of its
directory and in `meta.ko.json` beside it.

English is the default language and lives at `/docs/...`. Korean lives at
`/ko/docs/...`, in sibling files named `<page>.ko.mdx`, and every English page
changes together with its Korean one.

A page is public the moment it merges. Before adding one, check that it says
what a stranger needs, that it names no customer and no real person, and that it
carries no key, fingerprint, incident or cost. The prose rules in `AGENTS.md`
apply.

```bash
bun install
bun run dev          # http://localhost:5173/docs
bun run build        # static output in build/client
bun run types:check
```

`bun run generate:tools` derives `app/generated/tool-catalog.json` from
`pkg/capabilityprotocol/generated/capability-tools.json` at the repository root, the catalog the agent
itself runs on. `build` and `dev` run it first, so the tool tables in the docs
cannot drift from the tools that exist. Nothing hand-lists a tool.

`app/lib/openapi.ts` generates the public API's OpenAPI document, and this site
is where it is served: `/api-docs` and `/openapi/<language>.json` on every
company host redirect here. A path added to `web/src/routes/api/v1/` is added
there in the same change, and `web/tests/integration/public-api-route.test.ts`
fails when the two disagree.

## Deploying

```bash
bun run build
bun run ../../web/scripts/deploy-pages.ts --project internkim-docs --output build/client --production
```

Custom domains always serve the production deployment, so a preview build
answers only on `*.pages.dev`.
