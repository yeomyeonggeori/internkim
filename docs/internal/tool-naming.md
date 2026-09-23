# Tool naming

A tool's name is the only part of it a model sees before deciding whether to
call it, and the only part a person reads in a ledger. This document is the
convention for the tools we own. `tools/verify-tool-naming` enforces it.

External MCP servers name their own tools and are out of scope. In scope are
the capability catalog and blueclaw's local tool provider.

The kernel is out of scope on purpose. `bash`, `read`, `write`, `edit`, `plan`
and `equip` carry no namespace and no canonical verb because they carry the
names other coding agents use, so a model that learned them elsewhere
recognizes them on its first turn (blueclaw #396). Renaming one to fit this
document would undo that. A new kernel tool takes the name the industry
already gave the act, and when there is none, this convention applies.

## The shape

```
<namespace>_<subject>_<verb>        crm_opportunity_move
<namespace>_<verb>                  task_add
```

The namespace is the area the tool belongs to and matches the descriptor's
`namespace` field. The subject is the thing acted on, and is dropped when the
namespace already names it. The verb comes last, always.

`event_add` and `crm_contact_add` are both right: `calendar`'s subject is the
event, so `event` carries it, while `crm` holds several subjects and has to say
which.

## The verbs

Six verbs cover most of what a tool does. Use them rather than a synonym.

| verb | means | not |
| --- | --- | --- |
| `add` | bring a new record into being | create, register, save, record |
| `list` | several records, filtered | index, all, fetch |
| `get` | one record, or one settings block | read, status, show |
| `update` | change named fields on a record | set, edit, modify, patch |
| `delete` | remove a record | remove, archive, destroy |
| `search` | several records, ranked by a query | find, lookup, query |

`search` earns its own verb because it takes a query and ranks; a filtered
`list` does not.

A seventh verb is allowed when the act is not one of the six and a person would
name it differently. `crm_opportunity_move` moves a deal between stages, which
is neither an update of named fields nor a delete. `person_invite` sends an
invitation, which no record-level verb describes. Reach for one of the six
first, and when you do not, the verb must be a word the domain already uses.

`set` is reserved for a settings block that is written whole, where `update`
would wrongly suggest that unnamed fields survive: `company_info_set`,
`crm_vocabulary_set`. When a call changes only the fields it names, it is
`update`.

## Name the thing the description names

A tool whose name uses a different word than its own description teaches the
model one vocabulary and answers to another. The model then asks for a tool
that does not exist, and the runtime reports that nothing matches.

`crm_opportunity_update` describes itself as changing "what a **deal** is worth",
and never says opportunity. A reader who learns the word deal cannot reach it.

The rule: **every word in the name appears in the description.** When the
product has settled on a word, the name uses that word. When the name is right
and the description drifted, fix the description.

Short grammatical words (`get`, `set`, `add`, `to`, `of`) are exempt, and so are
the standard names of algorithms and formats.

## Parameters

Parameters are camelCase, spelled out. No abbreviations: `reference` not `ref`,
`identifier` not `uid`, `configuration` not `config`. `sha256` and other
standard algorithm names stay as they are spelled outside the repository.

These parameter names carry a fixed meaning and are reused rather than
reinvented:

| parameter | carries |
| --- | --- |
| `<thing>Hint` | a natural reference the model saw, resolved by the ladder in `internal/capabilityd/hint_resolution.go` |
| `<thing>Hints` | several of those, for a call that acts on several |
| `limit` | the largest number of records to answer with |
| `query` | free text a `search` ranks against |
| `scope` | whose records a call reads, when it can read beyond the requester's |
| `startsAt` / `endsAt` | instants, RFC 3339 |
| `date` | a calendar day, `YYYY-MM-DD`, read in the company's time zone |
| `reason` | why a record was written by hand |
| `note` | free text the record carries |

A hint field's description says exactly which referent it takes — "the exact
CURRENT title, never a new or intended title" — because a weak model fills a
vague hint field with the wrong thing.

## What the check enforces

`tools/verify-tool-naming` reads the catalog and blueclaw's local tools, and
fails on:

- a name that does not match `<namespace>_<subject>_<verb>` or
  `<namespace>_<verb>`
- a verb outside the six, unless the tool is listed in `VERBS_BY_EXCEPTION`
  with the reason it needs its own word
- a name whose word never appears in its own description
- a parameter that is not camelCase, or that abbreviates

Local tools are read from Go source, which gives their names and nothing else,
so they are held to the shape and the verb. The vocabulary and parameter rules
need a description and a schema, which only the catalog carries.

The exception lists are in the check itself, one place, and an entry is removed
when its tool is renamed. A list that only grows is a convention nobody follows.

## The names that were already wrong

51 faults predate this convention, and `docs/internal/tool-naming-debt.json`
holds them with the word each name fails to say. The check passes while they
stand and fails on a fault that is not listed, so a new tool is held to the
convention and an old one is paid down when its area is next touched.

The file only shrinks. Renaming a tool means deleting its entry, and the check
fails when an entry names a fault the catalog no longer has, so the baseline
cannot outlive what it excuses.

Most of the debt is a whole namespace at once: `mail_message_*` describes email,
`team_*` describes organizations, `crm_contact_*` describes people. Renaming one
tool in such a group leaves the group less consistent than before, so these are
paid down per namespace.
