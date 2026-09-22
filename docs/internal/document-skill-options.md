# Options for the document-producing skills

The question was whether GenOffice could replace the bundled skills that
produce files. It came up because a company host cannot run all of them, a gap
`docs/internal/single-binary-host.md:247` already records: `host/Dockerfile`
installs python3 and bun and no `uv`, and the skills bootstrap their Python
dependencies with `uv venv` plus `uv pip install`.

This weighs three paths: GenOffice, the incumbent skills, and fixing the
image.

The recommendation below was acted on the day after it was written.
[#1908](https://github.com/yeomyeonggeori/internkim/pull/1908) put the Nanum
font, the resolved wheels, `uv` and Chromium into `host/Dockerfile`, and a
build-time gate now refuses an image whose bundled skills cannot find what
they declare. The premises about the host that this document opens with —
no `uv`, no Korean font — describe the image as it was on 2026-09-21. What
survives is the reasoning about GenOffice, which nothing since has changed.

## What the incumbent actually is

Seven skills write files. Each one is a `SKILL.md` plus Python scripts in
`.dependency/internkim-plugin/skills/<skill>/scripts/`.

| Skill | What writes the file | Needs `uv` | Needs a browser |
|---|---|---|---|
| document | `python-docx`, `fpdf2` | yes | no |
| pdf | `fpdf2`, `pypdf` | yes | no |
| spreadsheet | `openpyxl` | yes | no |
| paperwork | `fpdf2`, `python-docx`, `docxtpl` | yes | no |
| dataroom | `pyyaml` | yes | no |
| presentation | stdlib `zipfile`, `playwright-core` | validation only | for review |
| website | bun, Vite, React | no | for screenshots |

The `uv` column is worth reading before accepting the premise that seven
skills are dead. `skill_runtime.py:19` returns success when the interpreter
already imports everything `requirements.txt` names, so `uv` is reached only
when the system Python is missing a package. Five skills depend on it today,
and `single-binary-host.md:247` should be corrected to say so: website carries
no `skill_runtime.py` at all, and presentation reaches `uv` only in
`validate_pptx.py`.

Presentation is a separate case. `build.sh` calls
`presentation/scripts/html_export.py` with plain `python3`, and that script
imports nothing outside the standard library: it assembles the PPTX with
`zipfile` (`html_export.py:13`) and `render_review.py` draws its fallback
review images with `zlib` and `struct`. `python-pptx` is only for
`validate_pptx.py`. The Chromium dependency is real but soft:
`html_render.mjs:281` looks for `CHROME_PATH` then `/usr/bin/chromium`, and
`build.sh` prints `browser render environment is unavailable; continuing with
fallback review/export` when the launch fails. A deck still builds on the
host, and its quality gate can never pass:
`render_review.py:143` requires `visual_evidence_reliable`, which is true only
when `renderSource == "browser"`.

Website needs nothing the image lacks.

The dependency set is small. The seven direct wheels total about 2.7 MB, and
with lxml, Pillow, fontTools and cryptography underneath them a host pulls
roughly 30 MB on first run, cached per skill under
`~/.cache/internkim-skills/environments/<skill>`. The marker file at
`skill_runtime.py:111` makes the second run free. Chromium is the expensive
item, near 110 MB for the Debian package.

**A larger gap than `uv` sits underneath all of this: the image has no Korean
font.** It installs `libfontconfig1` and no font package. Every PDF path looks
for NanumGothic or Noto CJK (`pdf/scripts/create_pdf.py:282`,
`document/scripts/export_document.py:131`), and `create_pdf.py:257` raises
`non-Latin PDF text requires fontPath or an installed Korean-capable font`
rather than emitting boxes. A Korean PDF fails on a company host today for
that reason alone, and no change of document library fixes it.

## What is hard about what they produce

Reading the SKILL.md files, the bar is not "emit a .docx".

**Korean business paperwork is the deepest part.** `render_paperwork.py` draws
a letterhead from the company profile with an optional logo
(`render_paperwork.py:194`), a 결재란 approval grid (`:246`), an item table
with computed VAT and totals (`:316`, `:376`), and a signature block that
places the company seal image over the `(인)` marker at a measured offset
(`:411-433`). Eighteen document types are specified twice, once per language,
under `paperwork/references/ko/` and `en/`, and five contracts ship as `.docx`
templates filled by `docxtpl`. The specs own required clauses and a density
gate, so an abridged contract is a defect.

**Source fidelity is enforced, not requested.** Every skill carries a
validator that takes `--required-text` and `--forbidden-text` and fails on
missing source facts, unembedded fonts, unextractable text, table overflow,
off-row formulas and unreadable spacing. Spreadsheet treats `offRowFormulaCount`
as a defect. PDF asserts that a computed total equals the source total before
writing.

**The deck has a scored review loop.** `render_review.py` is 1,523 lines and
`html_export.py` 1,857. `render_review.py:21` sets the pass mark at 82, and
`:38` holds a tuned deduction table of nineteen named warnings, from
`weakVisualIdentityWarning` at 24 points down to `missingSpeakerNotesWarning`
at 8. Each of those names was a deck that came out wrong: a side stripe standing
in for a design system, a ghost card, an emoji used as an icon, an absolutely
positioned footer, a vertical dead zone, a claimed current date with no source.
The deck vocabulary (archetype, signature move, anti-default check) and the
bundled Paperlogy WOFF2 faces are Korean-first typography decisions.

## What it cost to get here

The scripts total about 9,900 lines. The plugin submodule holds 45 commits of
packaging churn; the skills lived at `assets/blueclaw-workspace/skills/` in
this repository until `ce542097c` (2026-08-24) moved them out. On `main`:
`presentation` 65 commits since 2026-07-02 on top of 55 for its `simple-slides`
predecessor, `website` 31 on top of 61 for `site-prototype`, `pdf` 28,
`document` 26 counting `docx`, `spreadsheet` 21 counting `xlsx`, `paperwork` 18.

Four groups of those commits are the price of replacing a skill.

**Korean rendering.** `f21b514c9` found that poppler mis-rendered the Korean
CID/CFF font fpdf2 embeds, garbling glyphs in review images while the PDF was
fine in a real viewer, and switched to mupdf; the same commit rebuilt
`add_table`, which had been writing each row as one pipe-joined string in a
single bordered cell. `8e2c6a766` moved contract typography to 맑은 고딕 because
`Noto Sans CJK KR` is rarely installed and Word was falling back to serif
Batang. `create_docx.py:99` writes the font to `w:eastAsia` as well as
`w:ascii`, which is the attribute Hangul actually reads. `c80f6480c` taught
`validate_docx.py:201` to recognize a Korean-capable font name.

**Not thrashing on a revision.** `5eb2b37c8` replaced a fixed three-revision
cap that was delivering failing decks with 87 of 100 tool calls unused; the
loop now runs while the score climbs and stops on a stall. `283efaa3b` added
`restore_source.py` because the delivered HTML carries injected viewer CSS, so
the model had been re-authoring the deck from its own output every round.
`48af66452` moved deck work out of `tmp/` into `artifacts/<deck-slug>/` so a
recovery exit stopped destroying it.

**Weak-tier contract output.** `c76b0bdc1` bundled the docxtpl templates and
closed the abridged-contract failure class, so the weakest model tier produces
the full eleven-item employment contract. `83acbf6a0` fixed document numbering
when the model passed `용역계약서` instead of the slug. `48c7015` fixed a
letterhead that printed no business registration number after the company-info
tool changed its result shape.

**The tests that pin all of it.** `internal/blueclawworkspace/assets_test.go`
is 1,178 lines and about thirty tests, and `:1013` alone pins some fifty
substrings of `html_export.py` by name. `tools/test-presentation-review` drives
`render_review.py` over three fixture decks and asserts eleven specific
warnings fire on a deliberately templated Korean board deck. A replacement
generator invalidates that suite wholesale.

The live evaluation is the 30-task pilot under `web/tests/pilot/`, ten of whose
tasks are judged on a delivered `.docx`, `.xlsx`, `.pptx` or `.pdf`. It checks
extension and byte count, so the `visualQualityScore` gate is currently exercised
only by `tools/test-presentation-review`; the `evals/skill-artifact-quality`
harness that produced most of the July fixes was deleted on 2026-09-03.

## The contract a replacement has to meet

`SKILL.md` frontmatter carries `name`, `description`, `compatibility` and
`metadata.kim.intern.tool-references`; `skill_loader.go:110` reads the vendor
key and ignores `allowed-tools`. The hard gate is 15 KB and 300 lines per
`SKILL.md` (`docs/internal/skill-orchestration-design.md`), because the body
enters the model's initial context.

Delivery is a local path. The skill writes to `~/documents/<title>.docx` in
the requester's workspace, and `file_deliver` carries it as a
`FileAttachment` whose `devicePath` the runtime reads
(`bluecollar/toolcontract/registry.go:146`). Scripts run through `bash` as the
requester's unprivileged POSIX identity, so anything a generator writes beside
the output (caches, state, logs) lands in that person's workspace.

## GenOffice

The name is ambiguous. SourceForge carries an unrelated `genoffice.mirror`,
and Wikipedia's `EOffice` is a different product. The one that fits is
[genspark-ai/genoffice](https://github.com/genspark-ai/genoffice), which
Genspark open-sourced on 2026-07-31 and which advertises exactly this use: a
`genoffice` CLI and an agent skill so Claude Code, Codex and Cursor create and
edit real `.docx`, `.xlsx` and `.pptx` files locally. Apache-2.0, TypeScript,
7.4k stars, released roughly daily (v0.10.915 on 2026-09-21).

It is an Electron desktop suite. The CLI ships inside the application bundle
and runs on the app's own Node runtime through `ELECTRON_RUN_AS_NODE`. The
package `@genoffice/cli` is `"private": true`, as are `@genoffice/docx-engine`,
`@genoffice/pptx-engine` and `@genoffice/xlsx-gateway`, so none of it is on
npm. Distribution is the installer: 145 MB `.deb`, 178 MB AppImage, 187 MB
`.dmg`.

Document work is local. Only `search`, `image` and `media` leave the machine,
and the built-in AI is bring-your-own-key. That part matches this product's
shape.

Three things decide it anyway.

**There is no Linux ARM64 build.** The release carries `amd64.deb`,
`x86_64.rpm` and an `x86_64` AppImage. Issue #199 was closed as completed for
Windows on ARM; Linux ARM64 never shipped. `host/Dockerfile` builds for arm64
and amd64, and the hardware this product runs on is arm64.

**Half the commands need the app's renderer.** `packages/cli/README.md` states
that Word/PowerPoint/Excel/HTML/Markdown to PDF, Word to HTML, HTML to Word
and `create --type pdf` run inside the GenOffice binary through its hidden
`--headless-export` mode, which spawns the Electron executable. Every PDF this
company produces goes through that path. Whether an Electron window opens
without a display server in a Debian container is the question I could not
settle without running it, and the answer is probably Xvfb, which is Chromium
by another name.

**Its skill is 52 KB.** Our gate is 15 KB.

On Korean it is strong. The repository bundles `GenOfficeGothicKR`,
`GenOfficeSansKR` and `GenOfficeSerifKR` subsets with metric tests
(`kr-font-metrics.test.ts`), CJK punctuation shrinking, separate East Asian
and Latin font settings, and a merged PR wiring Hangul print and PDF; HWPX
support is an open request. That work lives in `apps/docs/src/renderer`, the
Electron renderer, which is the half that does not run headless.

What it does not have is 결재란, 인감, the eighteen Korean form specs, or a
validator that fails on a missing source fact. Those are ours and would stay
ours.

**The part worth keeping.** The engines are pure TypeScript over `jszip` and
`fast-xml-parser`, and `packages/cli/bin/genoffice` falls back to system Node
in a checkout. A `docx`/`xlsx`/`pptx` writing core could therefore be vendored
and run on the bun already in the image, with no Python, no `uv` and no
browser. The cost is building a Node 22 plus Rust monorepo (there is an
`xlsx-sidecar` in Rust), tracking a daily-moving upstream that publishes
nothing to npm, and rewriting every skill's script layer against a different
op vocabulary. PDF and PNG would still need the Electron binary, so the pdf
skill and the deck review loop stay where they are.

## Fixing the host

The comparison the question did not include. In `host/Dockerfile`, add to the
existing `apt-get install` line:

- `fonts-nanum` (about 25 MB), which unblocks every Korean PDF
- the six wheels into the system Python, which makes
  `skill_runtime.py:19` succeed and removes `uv` and the first-run network
  fetch together
- `chromium` (about 110 MB installed), without which the deck quality gate
  cannot pass at all

That is one line of Dockerfile, roughly 150 MB of image, and no new runtime.
It fixes the font gap, which GenOffice's headless subset would not.

There is a cheaper third option for the browser. The image already carries
`moli` 1.1.5, a Rust browser engine that serves CDP on 9222 and that Playwright
drives with `chromium.connectOverCDP`. `html_render.mjs:46` calls
`chromium.launch({executablePath})` instead, so using it means a code change,
and moli paints through Vello rather than Blink, so a deck's rendered review
would be scored against a different layout engine than the one the loop was
calibrated on. Measure it before spending the 110 MB; do not block on it.

## Recommendation

Fix the host. Add the fonts first, since a Korean PDF fails today for want of
a 25 MB font package and every document path is affected. Preinstall the
wheels second. Add Chromium third, or try moli over CDP before paying for it.

Do not adopt GenOffice as a service or as an installed application. It has no
build for the architecture we ship, its PDF and render commands need a desktop
binary, and its skill is three and a half times our context budget.

Do not split one skill off to it either. Spreadsheet is the plausible
candidate: `sheet apply` is fully headless, and it has the thinnest history and
the least Korean-specific behavior. Moving it means installing a 145 MB desktop
suite on every host to replace a 0.3 MB wheel, and `openpyxl` is not what is
failing.

Keep GenOffice on the list for one thing. If `python-docx` and `openpyxl`
become the constraint, its engines are Apache-2.0 TypeScript that runs on bun,
and vendoring the writing core is a real option to evaluate then, against a
version that has stopped moving daily.
