# Data room standard

A data room is the company's document archive: the evidence a fact rests on,
kept where a member and the agent can both find it. This document fixes its
shape so that any company can use it and so that the agent can still manage it
after it has grown past anything one context window could hold. Facts with a
number on them live in the company tables; the data room holds the documents
those numbers came from.

## 1. Seven rules

1. **A domain is a folder and a clearance.** Where a document lives is who may
   read it.
2. **Frontmatter is the only thing a person writes.** The index, the catalogs
   and `company.json` are generated from it and checked against it.
3. **A binary does not exist without a sidecar.** The agent reads text; the
   original is opened only when the text cannot answer.
4. **Nothing is overwritten.** A new version is added with `supersedes`.
5. **Publishing is a copy in `00-public` that names its source.**
6. **A member is one number.** Clearance 1 by default; the first administrator
   3; an administrator adjusts only people below them, up to their own.
7. **The record enforces clearance.** A document is a row and an object on
   the record; one policy compares the reader's clearance with the document's
   and nothing else decides.

## 2. Layout

```
dataroom/
├── README.md            the operating rules the agent reads first
├── company.json         the company profile, mirrored from the record
├── INDEX.md             generated: one line per domain with clearance, count, last change
├── inbox/               unfiled arrivals; nothing stays here past seven days
├── 00-public/        0  published copies: brand, press, certifications, the public deck
├── 01-corporate/     1  articles, registry, business registration
├── 02-governance/    3  shareholder register, board minutes, cap table, term sheets, litigation
├── 03-finance/       2  statements, audits, budgets, tax, banking
├── 04-contracts/     2  customer, supplier, NDA, lease; one folder per counterparty
├── 05-people/        1  org chart, work rules, policies
├── 06-hr-records/    3  employment contracts, payroll, reviews
├── 07-ip/            1  patents, trademarks, licences
├── 08-product/       1  product documents, roadmap, architecture
├── 09-marketing/     1  brand assets, references
├── 10-operations/    1  processes, supply chain, facilities
├── 11-compliance/    1  certifications, permits, security policy
├── 12-fundraising/   2  investor material, investor correspondence
└── 13-media/         1  photos, video
```

The domains are the request list a due-diligence team sends before an
acquisition or a financing round, which every law firm and audit firm has
converged on because they run it against hundreds of companies. It is
industry-independent, it is where a company's documents end up being sorted
anyway, and the model already knows which folder an NDA belongs in. The list
is then cut so that each domain holds one clearance: what a due-diligence list
files under corporate and people is split into a member-readable domain and a
board-readable one, tax joins finance, and the pipeline stays in the CRM
tables. A company adds a domain by naming it and its clearance in `company.json`.
The numeric prefixes fix the listing order so the agent sees the same picture
every time.

A document that belongs to a domain by subject and to another by clearance
goes where its clearance is: an acquisition term sheet is governance, an NDA is a
contract at clearance 2 though nobody would mind a member reading it. The one
error this shape allows is a document classified higher than it needs, which
is the safe direction.

Every domain holds its own `catalog.jsonl`. Inside, high-volume domains
(finance, contracts, media) split by year and contracts by counterparty; the
rest stay flat. A directory holds forty entries at most, so a search narrowed
to it returns a bounded slice however large the whole has grown. Each domain
has an `archive/` that the index excludes.

```
03-finance/
├── catalog.jsonl
├── 2026/
│   ├── 2026-03-15-audit-report-fy2025.pdf
│   ├── 2026-03-15-audit-report-fy2025.pdf.md
│   └── .derived/<sha256>/text/{01-summary.md,02-balance-sheet.md}
├── policies/
│   └── expense-policy.md
└── archive/

04-contracts/acme/
├── 2025-11-01-nda.pdf(.md)
└── 2026-01-10-msa.pdf(.md)
```

File names are `YYYY-MM-DD-slug.ext` in lowercase ASCII kebab case. The
document's own title, in whatever language, goes in frontmatter.

## 3. Document metadata

A text document carries frontmatter itself. A binary carries it in a sidecar
named `<file>.md` beside it. The fields are the same.

```yaml
---
id: fin-2025-audit-report        # stable; survives moves; references use it, never a path
title: 2025 회계연도 감사보고서
kind: report                     # contract | policy | report | deck | dataset | image | video
domain: finance
date: 2026-03-15                 # the date the document speaks from
period: 2025                     # the period it covers, when there is one
status: current                  # current | superseded | draft
supersedes: fin-2024-audit-report
source: 삼일회계법인
language: ko
summary: 감사 적정 의견, 매출 42억, 영업손실 3억.   # 200 characters at most
tags: [audit, k-ifrs]
sha256: ab12…                    # binaries only; keys the derived files
published: { at: 2026-04-01, by: member-id, from: fin-2025-audit-report }   # 00-public only
---
```

There is no clearance field: the clearance is the domain's. `summary` is copied
verbatim into the catalog and is the line the agent reads to decide whether to
open the document, so it carries the fact and its size rather than a
description of the document type. A sidecar for a long binary also lists the
parts under `.derived/`.

A `00-public` copy whose `sha256` equals its source's is the source published;
one whose hash differs is a redacted edition. Both keep `published.from`.

## 4. Generated files

| File | Reader | Form |
|---|---|---|
| `INDEX.md` | the agent, a person, the `/files` page | one line per domain; the map, under sixty lines |
| `<domain>/catalog.jsonl` | `rg`, tools | one JSON object per document, `path` included; the directory listing, readable by the domain's group |
| `company.json` | the agent, every turn | §6 |

Any document is reachable with one read and one search before it is opened:
the index for the map, then `rg` across `*/catalog.jsonl` narrowed by `path`,
which skips every domain the reader cannot open. Each generated file carries
the hash of the frontmatter set it was built from, so `check` can tell which is
stale without parsing Markdown.

## 5. Clearance

| Clearance | Reader | Domains |
|---|---|---|
| 0 | anyone outside the company | `00-public`, mirrored to `/workspace/shared/public/dataroom/` |
| 1 | every member | corporate, people, ip, product, marketing, operations, compliance, media |
| 2 | management | finance, contracts, fundraising |
| 3 | representative, board | governance, hr-records |

A member with clearance `n` reads every domain at clearance `n` or below.
Promotion to administrator sets the clearance to the promoter's. An
administrator may set the clearance of a member currently below their own, to
any value up to their own; nobody raises a peer or a superior, a
non-administrator sets nobody's, and lowering oneself is refused when it would
leave the company without a member at 3.

Writing is open to every member, under three rules.

1. **Adding to a domain at or below your own clearance is direct.**
2. **Adding to a domain above your own clearance is a submission.** It is
   registered at your own clearance with `domain` set, in the tree under
   `inbox/`, and a member cleared for the domain raises it.
3. **Moving a document between domains, superseding it, or handing out its
   original requires reading it.** Placing a copy in `00-public` is an
   administrator's act. An original is delivered only to the requester
   themselves, as a message to them or into their `artifacts/`; posting it
   further is their own act and is logged as one.

Circles do not appear here. A circle answers who is in a conversation or a
piece of work, which is the messenger's, the memory's and the workspace's
question. The data room answers who is cleared to read. Keeping the two axes on
separate surfaces is what keeps each of them simple; a document that needs
need-to-know scoping belongs in that circle's workspace folder.

An external data room, opened to an investor or a law firm, is an export of
chosen documents to a separate share. The live data room stays member-only.

## 6. `company.json`

The company profile the agent needs on every turn. In an internkim deployment
it mirrors the central record and is published to the guest workspace through
the same signed path as `identity.json`; nobody edits it by hand. In a
standalone data room the file is the source and `source` is absent.

The mirror is a projection of the `company` row rather than the row. The row
also carries working hours, leave rules, task and CRM vocabularies and the
calendar, which answer how the company is run and are read by the features
that own them.

```json
{
  "schemaVersion": 1,
  "source": { "table": "company", "id": "…", "updatedAt": "2026-09-10T…" },
  "slug": "dawnkim",
  "country": "KR",
  "locale": "ko",
  "timezone": "Asia/Seoul",
  "currencyCode": "KRW",
  "business": "…",
  "profile": { "name": { "ko": "…", "en": "…" }, "legalAttributes": { "ko": { "사업자등록번호": "…" } }, "…": "…" },
  "dataroom": { "domains": [ { "name": "14-clinical", "clearance": 3 } ] }
}
```

`profile` is the `company.profile` column copied verbatim, language slots
included, minus `bankAccount`, which a document generator fetches through
`company_info_get` when a letterhead needs it. Copying verbatim lets `check`
compare the file with the record on one key, and lets the column's structural
check serve as the file's JSON Schema. `dataroom.domains` lists only the
domains a company added beyond the standard set.

## 7. Access

In an internkim deployment a document is a `company_document` row and an
object in the company's asset bucket at
`<company>/dataroom/<clearance>/<sha256>`; the tree of §2 is what an export
writes and an import reads. The row carries the frontmatter of §3 as columns,
plus `clearance`, which is the domain's. Row-level security on the table and
on the bucket admits a reader whose `member.clearance` is at or above the
document's; below it the row and the object do not exist for that session,
whatever tool or script asked. Writing has the same check: a member registers
at their own clearance or below, and an administrator raises. Nothing on the
host needs protecting: a document reaches it only as a copy in the
requester's `tmp/` or `artifacts/`, which they were cleared to read.

The POSIX boundary bounds what requesters run: deriving text from a file is
untrusted computation, so it runs as the requester in their own `tmp/`, the
way `terminal_run` does, and the skill uploads the result. A parser exploit
gains a requester's own home and nothing more.

This protects reading above one's clearance. A cleared person passing on
what they read is answered by the `task_event` ledger, which names each
document `id` a task touched; so an original goes only to the requester.
The ways around it are in
[`dataroom-threats.md`](./dataroom-threats.md).

A search is `company_document_search` over the rows the session can see. The
service exports `00-public` to `/workspace/shared/public/dataroom/`. A
company running none of this reads the tree of §2 with `rg` and an editor.

## 8. Media

`13-media/` splits into `photos/<year>/` and `videos/<year>/`. Ingest derives
text so the agent never puts an original into context.

| Kind | In the sidecar | Under `.derived/<sha256>/` |
|---|---|---|
| image | caption, OCR text, event and place | a thumbnail for vision |
| video | summary, opening of the transcript | full transcript in parts, keyframe contact sheet |
| PDF, DOCX | summary, list of the derived parts | text split by section |
| XLSX | sheet list, one line per sheet | one CSV per sheet |

The original's hash keys the derived folder, so a replaced original invalidates
its derivatives. Binaries stay out of git; the text layer is tracked and the
sidecar hash pins the binary.

## 9. Ingest and check

```
a file arrives                    an attachment, an upload, a Drive sync, a path in tmp/
→ hash, detect kind, derive text  as the requester, in their tmp/
→ draft the sidecar               id, summary, domain proposed
→ a person confirms               required for a clearance 2 or 3 domain
→ the service moves it into place
→ regenerate the index and the domain's catalog
→ company_document_register       the ledger mirrors the catalog
```

A submission to a domain above the requester's clearance waits at the
requester's own clearance, in the tree under `inbox/`, until a member
cleared for the domain raises it.

`dataroom check` fails on: a binary without a sidecar, a missing required
field, a duplicate `id`, a `domain` that disagrees with the path, a directory
over forty entries, a stale generated file, a submission older than seven
days in `inbox/`, a `supersedes` pointing at nothing, a `00-public` document
without `published`, or a published copy whose `from` names nothing.

## 10. The agent's protocol

These lines are the data room's `README.md`.

- Start a session from `company.json` and `INDEX.md`.
- Find with `company_document_search`, or `dataroom search` in a tree,
  narrowed by `path` when browsing a domain or a year. Never list the tree.
- Read the sidecar before the original, and the original only when the sidecar
  cannot answer.
- Never edit an existing document; add one with `supersedes`.
- Add only through `dataroom ingest`, so the catalog is never out of step
  with a directory.
- A number comes from `company_metric_list`; the data room holds its evidence.
  A metric record names its document by `id`.

## 11. Rejected shapes

| Shape | Why not |
|---|---|
| a vector store as the primary | the agent cannot verify completeness or say "there is no such document"; Graphiti already serves memory |
| one manifest file | grows with the document count |
| a database as the only form | portability: a company running none of this reads the tree with `rg` and an editor; in internkim the record holds the document and the tree is its export |
| `index.json` | loses to Markdown on tokens and skimming and to JSONL on search |
| an `INDEX.md` in every directory | restates what a catalog search narrowed by path already returns, and multiplies generated files by the directory count |
| one catalog per clearance at the root | three files where a catalog inside the domain already says what the domain holds |
| circles as the read scope | sets, where the data room's question is ordered; see §5 |
| a clearance on each document, with `l0`–`l3` directories under every folder | a field, a kind-to-clearance default table, reclassification and a mismatch check, all to let a member read an NDA; a domain per clearance costs two extra domains |
| public as a flag beside the clearance | a second axis to keep consistent, for the same rules |
| requesters in data room groups, reading from a terminal | a cleared person's script can copy anything anywhere unlogged; through a tool every read is a ledger entry |
| a Linux user and group per clearance | three users, three groups, a helper path and a mode table, to make the file system repeat a comparison the record makes in one policy |
| a service-owned tree on the host, reached through built-in tools | the same comparison written a second time in Go, plus symlink, link-count and path checks the record never needs; a document is what has to be true after the computer burns down, so it is a row |
| a symlink or hard link as the published copy | a symlink is judged at its target, so it grants nothing; a hard link shares the inode's mode, so publishing opens the original's bits and leaves its protection to directory modes alone |
