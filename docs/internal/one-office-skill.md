# One office skill

Seven skills write files, and managing seven of anything costs more than
managing one. The request is one `office` skill that teaches how to drive the
document-producing programs, with the scripts and the vendored binaries staying
where they are. This works out what that skill would contain, what it would
cost, and which of the seven belong in it.

[`document-skill-options.md`](./document-skill-options.md) is the inventory of
the seven and what each one uses; this does not repeat it.

## Four mechanics that price everything below

**A skill is a directory holding a `SKILL.md`.** `DiscoverSkill` skips a
directory whose document will not load (`skill_registry.go:34`), while
`SkillDirectories` in this repository counts every directory under a plugin's
`skills/` whether or not one is there (`assets.go:74`). Deleting six `SKILL.md`
files therefore leaves six directories the runtime never sees and the Go
conformance tests still walk. The `Dockerfile`'s
`*/scripts/requirements.txt` glob keeps resolving; the agent stops being
offered the skill. Those two halves would have to be moved together or the
repository gains a set of checks that watch directories nothing reads.

**`<skill>` resolves to the bundle's own path** (`skill_prompt_builder.go:19`).
An `office/SKILL.md` that writes `<skill>/scripts/create_pdf.py` names
`office/scripts/create_pdf.py`. Keeping the scripts where they are means the
merged document writes `<skill>/../pdf/scripts/create_pdf.py`, which works and
reads badly, so a merge of the documents implies moving the scripts under one
directory.

**Availability is all-of, and it hides the whole bundle.** A skill is left out
of the prompt when a declared tool is not offered, a declared environment
variable is empty, or none of the declared files is readable
(`agent_instructions.go:241`). Merging unions all three declarations. This is
the same problem the font raises, one level up and with a wider blast radius:
`website` names eight capabilityd tools and `paperwork` six, and under a merge
the spreadsheet path would require every one of them.

**There are two ceilings.** The repository gate is 15,000 bytes and 300 lines
(`assets_test.go:14`). The runtime silently truncates a skill body at 20,000
runes when the model fetches it by name (`skill_management.go:20`). The gate
refuses; the runtime cuts. Anything that passes the first never reaches the
second.

## 1. The font declaration

### What the three do today

I drove `create_pdf.create_pdf`, `export_document.export_pdf` and
`render_paperwork.resolve_font` against three synthetic hosts, remapping each
script's own candidate list onto a directory whose contents I controlled, and
read the result back with pypdf. fpdf2 2.8.8, pypdf 6.19.0, Hangul face
`AppleSDGothicNeo.ttc`, source text `여명거리 주식회사 견적서`.

| host | `pdf` | `document` | `paperwork` |
|---|---|---|---|
| a Hangul font | writes, Hangul extractable | writes, Hangul extractable | writes, Hangul extractable |
| DejaVu only | writes, exit 0, no Hangul | refuses | writes, exit 0, no Hangul |
| no font | refuses | refuses | refuses |

The premise holds. Two details sharpen it. The DejaVu file is *larger* than the
correct one, 4,672 bytes against 4,449, so nothing about its size gives it
away. And fpdf2 does say something: it writes `Font MPDFAA+DejaVuSansBook is
missing the following glyphs: '여' (여), '명' (명), …` to stderr. The
model running the command sees that line. Nothing else does, and the exit code,
the file and the validator all report success.

### Giving `pdf` and `paperwork` `document`'s shape

With the Latin fallback taken out of the candidate list, on a host with no
font at all:

| | Latin only | Latin with `café` | Korean |
|---|---|---|---|
| `pdf` | writes (1,104 bytes) | refuses | refuses |
| `document` | writes | writes | refuses |
| `paperwork` | refuses | refuses | refuses |

For `pdf` the way out is real: Latin work still comes out on a font-less host,
and Korean refuses loudly, so the declaration can go and a font-less host keeps
the skill. It is not free. `create_pdf.py:288` decides "non-Latin" as
`ord(character) > 127` while `export_document.py:174` uses `> 0x2000`, so with
DejaVu gone `pdf` newly refuses accented Latin that the core Helvetica it falls
back to renders perfectly well. Removing the fallback trades a silent Korean
failure for a loud Latin-1 refusal unless the same change makes the test ask
whether the chosen font covers the text rather than whether the text is ASCII.
That is the same move the declaration gate itself is an instance of: judge the
thing, not a proxy for it.

For `paperwork` the way out does not exist as stated. `resolve_font` raises
whenever no candidate survives (`render_paperwork.py:114`), Korean or not, so
removing DejaVu costs it every English offer letter and NDA on a font-less
host. The declaration costs it the same documents today, earlier and more
legibly, so this is no regression against `main`, and it is a reason the
declaration suits `paperwork`. Making `paperwork` behave
like `document` means giving it a Latin path it has never had, which is a
change to the letterhead renderer, not to a font list.

### What happens to the gate

`TestAFontEmbeddingSkillDeclaresEveryCandidateThatCarriesHangul` already
carries the answer in code: a skill that walks no Latin-only fallback and
declares anyway fails with "declaring %v hides a skill that would have said so
itself". So the test forces the declaration out when the fallback goes, and it
keeps a job afterwards — it is what fails when someone reintroduces a fallback
without declaring. Keep it.

`TestTheFontTheHostImageGuaranteesIsOneTheDeclarationsAccept` loses its
subject. It iterates declaring skills and fails on `declaringSkillCount == 0`,
so it goes red the moment the last declaration is deleted. What it watches is
worth keeping: `fonts-nanum`'s `ReadableFilePath` in `host_dependencies.go` and
`HANGUL_FONT_PATHS` in `skill_runtime.py` are two copies of one fact, and
nothing else binds them. Re-point it at the candidate list the scripts actually
walk instead of at the declaration. That is a repair.

One constraint dissolves. [#1936](https://github.com/yeomyeonggeori/internkim/issues/1936)
records that `is_embeddable_font` and `cached_font_paths` cannot move into the
shared `skill_runtime.py` while `document` must not walk the fallback. With no
fallback anywhere, both functions move unconditionally and the six byte-identical
copies gain two more shared functions instead of three more hand-kept ones.

### A defect this turned up that has nothing to do with merging

`validate_pdf.py --required-text 여명거리…` reports the string missing on the
**correct** PDF as well as the broken one. pypdf extracts fpdf2's CID text with
interleaved NULs, so the substring never matches. Observed on
`AppleSDGothicNeo.ttc`; confirm against NanumGothic on the Debian image before
acting. The skills instruct the model to pass source facts as `--required-text`
and to revise real problems, so on Korean that check fires either way and
teaches the model to ignore it. It cannot be the thing that catches a missing
font. Worth its own issue.

## 2. What one office skill would contain

### What GenOffice does, and why its size is not available

`skills/genoffice/SKILL.md` is 72,162 bytes in one file, with no `references/`,
`scripts/` or `assets/` beside it. It was 23,655 bytes on 2026-09-13 and
tripled in three days. The size is not a design decision:
`packages/cli/tests/help-sync.test.ts` requires every command and option in the
CLI registry to appear in the skill, and 36.6 KB of the file is a single
command table whose rows are padded to roughly 1,576 bytes each. Stripped of
alignment whitespace the file is 39,410 bytes, of which about 68% is per-format
reference. Behind it sit another 97 KB of markdown and schema generated from the
validators, reachable only by running `genoffice guide`.

Four things transfer.

- **The routing table is the second section and is keyed on the task**, with
  columns `Task | Command path | Not for`. Each cell is a whole pipeline rather
  than a command, and the `Not for` column names the neighbour a model actually
  confuses it with — "a presentation from a brief" names "editing a deck that
  exists" and the reverse.
- **One quirk block per program**, five dense paragraphs under
  `Behaviour to know`. Format is an annotation, never a heading.
- **Every field behind one retrieval verb** generated from the validator the
  executor runs, with a `--fingerprint` that changes when any op or field does.
  The guide cannot drift from what `apply` accepts.
- **A verification gate per artifact kind** at the end, with the warning that
  the checker exits 0 whether or not it found something.

What does not transfer is the shape that makes it 72 KB: a hand-kept copy of
the whole interface, held honest by a CI equality test with no budget on the
other side. Our 15 KB gate forbids that, and the gate is right.

### What the good ones share

Anthropic publishes four skills here, not one: docx 6,911 bytes, xlsx 8,598,
pdf 8,072, pptx 20,796, routed entirely by the frontmatter `description` and
bundled as a single plugin by `marketplace.json`. They duplicate the shared
machinery three times on disk, including a 977 KB XSD tree, buying skill
independence with bytes on disk instead of bytes in context.

Across those, GenOffice, `claude-api` and `mcp-builder`, the traits that
recur:

- They route by what the user is trying to do. Format is a column value.
  The one organised by program — `pdf`, with `### pypdf`, `### reportlab` —
  is the least revised and the only one inlining API samples.
- The routing table's payload is the near-miss, spent inline on the row.
- They inline what the model would confidently get wrong and defer what it
  would merely not know. `docx` says it outright: "The model knows the API;
  these are the footguns", then spends its whole body on what corrupts the
  file.
- Deferral targets something that cannot drift: a guide generated from the
  validator, a fetched upstream README. Not a copy.
- Every one ends in a look-at-it gate, because the output is a binary the
  model cannot read.

### Which of ours belong under `office`

The arithmetic decides two of these before the argument starts. Bodies and
descriptions, measured:

| set | bodies | descriptions | naive merge | against the 15 KB gate |
|---|---:|---:|---:|---|
| `document` + `pdf` + `spreadsheet` | 9,848 | 1,518 | ≥ 11,566 | fits, 3.4 KB spare |
| the same plus `paperwork` | 13,115 | 2,108 | ≥ 15,423 | over, before a routing table exists |
| all seven | 28,310 | 2,955 | ≥ 31,465 | twice the gate |

Merging does not compress. The three bodies carry the same four sections
(`Workflow`, a quality section, an editing section, `Final check`) and the same
seven rules — source of truth, "Not provided", `~/documents/`, deliver only the
accepted file, the workspace file over the delivered attachment, bundled
scripts own dependency setup. Normalising every sentence for format nouns and
writing each one once saves 158 bytes of 9,647, under 2%. The repetition is
thematic and the wording is different everywhere, so the merged file is the
sum of its parts until somebody rewrites it. What a merge buys is one routing
decision in place of three descriptions competing inside `skill_search`, not
a smaller prompt.

**`document`, `pdf`, `spreadsheet` — in.** One user intent, three output
formats, one dependency story (python3 and wheels the image preinstalls), and
tool references `read` and `bash` that every host offers, so the union costs
nothing. They already route to each other in prose: `pdf`'s description spends
three sentences pushing layout-indifferent work to `document`. That prose is
exactly a `Not for` column written the long way.

**`paperwork` — out.** The trigger objection is real and is the weaker of the
three reasons: its description carries 25 Korean form names, and a routing
table can carry those. The harder ones are that its 3,267-byte body takes the
merge over the gate on its own, and that its six `company_document_*` tool
references would become requirements of the spreadsheet path. The deepest one
is that it is not a skill about using a program. The program is
`render_paperwork.py`; what the skill teaches is which of eighteen specs under
`references/<ko|en>/` governs, what the spec's density gate requires, and when
to register a document number. That is a catalog over a reference tree, a
different shape from a router over three writers.

**`dataroom` — out.** It files and finds documents and produces none; its own
description says so. Same `company_document_*` union.

**`presentation` — out.** 5,558 bytes alone, and its quality gate cannot pass
without a browser: `render_review.py:143` requires `visual_evidence_reliable`,
true only when `renderSource == "browser"`. Folding it in gives the spreadsheet
path a Chromium dependency. Its deck vocabulary is 900 KB of assets a routing
table cannot stand in for.

**`website` — out.** 5,878 bytes, eight capabilityd tool references, and its
output is a served URL, and no file lands in `~/documents/`.

So `office` is `document` + `pdf` + `spreadsheet`: a routing table keyed on
what the user wants (a report, a workbook, a layout-critical page, an existing
file to edit), one shared block of the seven rules all three already state, one
quirk block per program, and one `Final check` in place of three.

## 3. What it costs and what it breaks

| Check | Under a merge | Verdict |
|---|---|---|
| `TestBundledSkillDocumentsStayWithinPromptBudget` | globs `*/SKILL.md`; sees one file instead of three | unchanged, and the binding constraint |
| `TestArtifactSkillsDocumentGroundedQualityAndValidationWarnings` | reads `pdf/SKILL.md` and `document/SKILL.md` by path; fails to stat | breaks loudly, then four `source of truth` assertions collapse into one read of one file |
| `TestModelFacingWorkspaceDocsDoNotExposeConcretePrivatePaths` | same, five paths to one | breaks loudly, then five assertions become one |
| `TestEverySkillComesFromAPlugin` | names `pdf` as a directory; passes on a directory with no document | **vacuous** unless it asserts a loadable `SKILL.md` |
| `TestAFontEmbeddingSkillDeclaresEveryCandidateThatCarriesHangul` | `declaredAnyFilePaths` reads `<dir>/SKILL.md` | breaks loudly; repair by reading the owning skill's document |
| `TestTheFontTheHostImageGuaranteesIsOneTheDeclarationsAccept` | no skill declares | fails on `declaringSkillCount == 0`; repair by binding the image's font to the candidate list |
| `TestHostImageResolvesEveryBundledSkillRequirement` | resolves `*/scripts/requirements.txt` | unchanged if the script directories move under `office/`, and needs a second glob level if they nest |
| `TestBundledSkillRuntimeScriptsStayIdentical` | six copies | **six stay six** unless the script directories merge too |
| `TestArtifactPythonSkillsBootstrapDependenciesFromBundledScripts` | keyed on `document` and `spreadsheet` directories | mechanical rename |

Two of those are the ones to watch. `TestEverySkillComesFromAPlugin` goes
vacuous silently, because `SkillDirectories` calls a directory a skill and the
runtime calls a loadable document a skill, and after a merge those two
definitions disagree for the first time. Everything else in the table fails
loudly, which is the behaviour to want.

And the management-surface accounting, since that is the motive. Merging the
three documents removes two `SKILL.md` files, two descriptions from
`skill_search`, and two of the twenty `skill_search list` slots (currently 17
skills against a cap of 20, so no listing is truncated today either way). It
does not remove: three `scripts/` directories, three `requirements.txt`, three
validators, or any of the six `skill_runtime.py` copies. Those go only if the
directories merge as well, which the request explicitly does not ask for. The
honest size of the win is three documents becoming one.

## Recommendation

Take the font change on its own, first, and land it before any merge. Remove
`LATIN_FALLBACK_FONT_PATH` from `create_pdf.py`, replace `pdf`'s `ord > 127`
test with one that asks whether the resolved font covers the text, delete both
`requires-any-file` declarations, re-point
`TestTheFontTheHostImageGuaranteesIsOneTheDeclarationsAccept` at the candidate
list, and close #1936's blocked half by moving `is_embeddable_font` and
`cached_font_paths` into `skill_runtime.py`. `paperwork` needs a Latin path
before its declaration can go. Until it has one the declaration stays, which
is a second reason `paperwork` sits outside `office`.

Then merge `document`, `pdf` and `spreadsheet` into `office`, moving their
script directories under it so the six runtime copies become four and
`<skill>` keeps meaning what it says. Write the body as a routing table keyed
on intent with a `Not for` column, one shared rules block, one quirk block per
program, and one verification gate. Budget 11.6 KB before the table and
expect to rewrite where a concatenation would do.

Leave `paperwork`, `dataroom`, `presentation` and `website` as they are. Each
is out for a reason the routing table cannot absorb: a reference catalog, a
filing archive, a browser dependency, a served URL.

Two claims here were read and not run, and both are load-bearing enough to
check before acting. The NUL-interleaving extraction failure was observed
only against `AppleSDGothicNeo.ttc`; run it against NanumGothic on the Debian
image. And whether `capabilityd` ever offers a partial tool set decides how
sharp the tool-reference union is; that takes a host where one of the eight
is missing. Everything else in the three tables came from driving the scripts,
measuring the files, or reading what each test stats.
