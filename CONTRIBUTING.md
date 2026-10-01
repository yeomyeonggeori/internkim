# Contributing

Nothing lands on `main` or `design/saas` by direct push. Branch, run
`tools/verify`, open a pull request, merge.

The rules for this repository are in [CLAUDE.md](CLAUDE.md), also read as
`AGENTS.md`: commit messages, pull requests, prose, code style, and what each
kind of change must prove. Read it once before your first pull request. It is
written for agents and applies equally to people.

## Set up

```bash
git clone --recurse-submodules https://github.com/yeomyeonggeori/internkim.git
cd internkim
make build
cd web && bun install
```

[Running it locally](https://docs.intern.kim/record#running-locally) covers the local Supabase stack and running
the web app against it. Anything that uses that stack runs under
`tools/with-local-plane <command>`.

## Check your change

```bash
tools/verify
```

`make check` is the same command. It reads what your diff touches and runs only
the groups that diff can break. `--all` runs every group and `--only <names>`
runs the ones you name. No workflow runs it after you push, so a pull request
that skipped it has not been checked.

## Branches

`<type>/<subject-in-kebab-case>`, for example `fix/mattermost-recipient-resolution`.
The type is the word the commit will carry. Branch product work off
`design/saas` and device or runtime work off `main`, and rebase when either
moves.

## Commits

```
<type>: <what changes, imperative, lowercase, no trailing period>

<why it changes; wrap at 72 columns>
```

`type` is one of `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`.
`git config commit.template .gitmessage` installs the template.

## Pull requests

One reviewable change each. The template asks what to look at and what shows it
works. A change to a skill, a tool descriptor or a prompt also says what the
model now sees and roughly what it costs.

Branch names, commits, issues and pull requests are written in English.
Fixtures and documentation use sample names and `example.com` addresses, never
a real person's.
