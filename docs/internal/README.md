# Internal documents

Documents sit in one of three places, and the directory decides who sees them.

| | Tracked | On the docs site | Holds |
|---|---|---|---|
| `docs/` | yes | yes | what a stranger reads to understand or run this |
| `docs/internal/` | yes | no | what a contributor reads to change it |
| `docs/private/` | no | no | what nobody needs to open again |

`docs/internal/` ships inside the public repository. Design deliberation, ops
runbooks, notes on hardware we are retiring, and working schema notes belong
here: a contributor needs them, a site visitor does not.

`docs/private/` is gitignored, and the reason is clutter. Handoffs written to
one session, positioning drafts, and the record of a migration that finished
pile up until the directory stops being browsable. Nothing there is secret; it
is done with. This repository never carries it, so it is kept where the machine
can still reach it and `tools/sync-worktree-local-state` links it into a new
worktree.

A document that tracked code or `AGENTS.md` links to cannot be private, or the
link dangles for whoever clones. Check with a grep before moving one down.

[`device/`](./device/) holds the rules for the frozen device path, which
`AGENTS.md` no longer carries.

The site that publishes `docs/` is `docs/web/`, a React Router build of
Fumadocs that reads the directory above it. It publishes a named list of
sections rather than everything it finds: `docs.files` in
`docs/web/app/lib/source.ts` names the root pages, `tools` and `api`, and a new
public section is added there and in the matching list in
`react-router.config.ts`. Nothing under `internal/` or `private/` is
reachable, whatever its extension.

English is the default language and lives at `/docs/...`. Korean lives at
`/ko/docs/...`, in sibling files named `<page>.ko.mdx`.

A new document starts in `docs/internal/`. Moving it up is a `git mv` and a
read: check that it says what a stranger needs, that it names no customer and
no person, and that the prose rules in `AGENTS.md` hold.
