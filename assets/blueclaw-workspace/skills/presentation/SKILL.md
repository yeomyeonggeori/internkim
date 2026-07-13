---
name: presentation
description: Generate, revise, and validate HTML-first presentation decks, including HTML, PDF, PPTX, PowerPoint, Google Slides, Keynote, 발표자료, 파워포인트, and 피피티 requests.
when_to_use: Use for creating or revising slides and for validating existing .pptx files.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# Presentation

Create a useful, visually strong deck and attach the requested files. HTML-first: `slides.html` is the source of truth, `DESIGN.md` the design brief, and HTML is the default deliverable. PDF and PPTX are derived; PPTX is image-backed by default to preserve the HTML render.

## Workflow

1. Classify the request as a new deck or a revision, then decide the format and slide spine.
2. For a revision, choose the target in this order: an explicitly named/current attachment, the latest compatible artifact in recent same-conversation posts, the newest same-slug artifact manifest entry, then workspace source. A newer HTML deck is the edit target; older PDF or Markdown files are supporting material.
3. Preview and read the selected HTML. Reuse its `<deck-slug>` and work only in `artifacts/<deck-slug>/`, with `slides.html` as the canonical controller-free source. If only delivered HTML exists, run `python3 /workspace/skills/presentation/scripts/restore_source.py <delivered.html> slides.html` from `artifacts/<deck-slug>`. Apply content and design changes with targeted `file.edit`; never reconstruct an existing deck with whole-file `file.write`.
4. For a new deck only, use the `file.write` tool directly to create `artifacts/<deck-slug>/slides.html`. Do not use `capability.invoke`, `filesystem.mount.write`, `file.pick`, shell heredocs, or `echo` for deck source.
5. Make a new `slides.html` complete in its first write: full HTML, CSS, slide sections, source facts, `data-visual-system`, and `data-slide-role` on every slide. Then add compact `DESIGN.md`, `deck-brief.md`, and `required-visible-text.txt`; they must not delay the primary source file.
6. Run `/workspace/skills/presentation/scripts/build.sh` with `terminal.run` from `workingDirectoryPath: "artifacts/<deck-slug>"`. The exporter replaces any `data-internkim-slide-viewer` blocks with exactly one canonical controller pair.
7. Inspect `build/review/slide-review.json`, `slide-review.md`, contact sheets, `fit-review.json`, and each `fit-review-XX.md`, including `visualQualityScore`, `staticGatePassed`, `qualityGatePassed`, `visualEvidenceReliable`, `needsDesignRevision`, and design warnings.
8. If a gate or rendered image evidence shows problems, revise `slides.html` with targeted `file.edit` patches and rebuild. Stop when the gate passes, the score stalls across two rebuilds, or budget says finalize. Preserve the design-source marker, requested slide count, source-fact ledger intent, `data-visual-system`, and `data-slide-role`.
9. Convert once for requested PDF/PPTX, then deliver accepted outputs from `artifacts/<deck-slug>/build/` plus requested sources with `file.deliver`. Use one call and a `files` array for multiple files; revisions overwrite and re-deliver the same slug.

Use this build command shape:

```json
{
  "command": "/workspace/skills/presentation/scripts/build.sh",
  "workingDirectoryPath": "artifacts/<deck-slug>"
}
```

For PPTX requests, convert with one final build after the review loop:

```json
{
  "command": "FORMATS=pptx /workspace/skills/presentation/scripts/build.sh",
  "workingDirectoryPath": "artifacts/<deck-slug>"
}
```

For PDF requests convert with `FORMATS=pdf`, and for all outputs `FORMATS=all`, in that final build only; every conversion reads the same `slides.html`. `build.sh` uses the working directory name as the output name. With no `FORMATS`, it creates `build/<deck-slug>.html` plus review evidence.

Do not call `terminal.run` with an `arguments` array alone.

Deliver generated files such as `artifacts/<deck-slug>/build/<deck-slug>.html` or `artifacts/<deck-slug>/build/<deck-slug>.pptx` with `file.deliver`. Do not use shell `cp`, do not deliver from a skill directory, and do not expose `/workspace`, `/tmp`, `file://`, or sandbox paths.

Give every slide `<aside class="notes">` with 2-4 spoken sentences in the request language, readable aloud verbatim; they never render on the slide.

There is no template to fill in; the content, layout, and HTML source are your responsibility.

## Imagery

When a slide needs photography, fetch a CC0 photo in one command and reference the local file — `python3 scripts/fetch_image.py "<english query>" assets/images/<name>.jpg`. Prefer fetched or user-provided files over hotlinks; generation is the last resort.

## Company Data

For IR, 회사소개, and proposal decks pull real data instead of inventing: `company.info.get`, `company.metric.list` (growth series), `company.record.list` (연혁·funding·products). Logo: `/workspace/circles/staff/company/logo.png` if present.

## Content Quality

Useful before beautiful: build a situation, tension, thesis, proof, and close that gives the audience a decision, lesson, or next action.

Reject shallow content: generic cards, claims without examples, or slides that only restate the prompt are not enough. When source material is thin, create realistic but clearly labeled assumptions or example data. Prefer concrete evidence, a worked example, scenario numbers, workflow states, or a recommendation.

Each slide needs a job, claim, proof, and scannable visual structure. Use the user's language and only source dates; never stamp today's date.

Pick one deck archetype, such as pitch, executive briefing, or status report, and use it to decide information density, section sequence, and ending.

Use these slide patterns as the default vocabulary: title thesis, section divider, comparison, matrix, timeline, evidence card, recommendation, and closing ask.

For executive, board, roadmap, investor-style, or design-quality decks, choose a named visual style and composition seed from `assets/visual-styles.md` and `assets/composition-seeds.md`. Show the exact organization, product, and period; preserve the original period wording exactly. Preserve exact source values, target-versus-actual metrics, implications, owners, dates, `제공된 자료 없음`, and risk/evidence/response/owner; end with exact next actions.

Quarterly review decks usually follow the six-slide board spine in `composition-seeds.md`, which carries the board-floor composition set. Use claim-style titles such as `성장은 확인됐지만 품질 스프린트 승인이 필요합니다`, not topic labels such as `요약`.

Do not deliver plain title slides, unstyled tables, bare lists, or the same 2x2 card dashboard. Use KPI cards, status chips, variance bars, timelines, risk matrices, approval panels, and one recognizable recurring primitive.

## Source Files

`slides.html` is the source of truth. New decks write it first; revisions read and edit it in place. Include `<!-- design-source: DESIGN.md -->`, mirrored colors/typography, one message per slide, `data-visual-system` on `<body>`, `data-slide-role` on every `<section class="slide">`, and one top-level section per slide.

`DESIGN.md` matters less than the deck itself. If created, use Stitch-compatible YAML front matter with only `colors`, `typography`, `layout`, then headings: `Style Prompt`, `Visual Identity Gate`, `Scene`, `Design Thesis`, `Visual System`, `Signature Move`, `Anti-default Check`.

`deck-brief.md` is an optional planning note, not a deliverable: archetype, story spine, slide count, visual system, signature move, what would be too shallow.

`required-visible-text.txt` is an optional source-fact ledger for review, not a token filter: one source fact or must-appear phrase per line, then represent those facts in `slides.html` with natural layout copy, tables, charts, or labels — organization, product, exact period wording, metric and target values, owners, dates, missing-value labels such as `제공된 자료 없음`. Do not replace Korean period wording such as `2026년 2분기` with `2026 Q2`.

HTML is the layout surface; browser rendering feeds PDF, review images, and image-backed PPTX. Without Chromium (or with `PRESENTATION_PPTX_MODE=native`) the build falls back to native text-backed PPTX:

- Complete HTML document with `<style>` in the head.
- Canonical geometry: `.slide { width: 1600px; height: 900px; }`.
- `@page { size: 1600px 900px; margin: 0; }`.
- Fixed 16:9 flex column: header `flex: none`, body `flex: 1 1 0; min-height: 0`, footer `flex: none; margin-top: auto`; never absolutely position the footer.
- Fill the body with `justify-content: space-between` and stretch major blocks using `flex: 1; min-height: 0`; do not stack everything in the top half.
- Fit-safe containers with `minmax(0, 1fr)`, `min-width: 0`, `min-height: 0`, and `overflow-wrap: anywhere`. Use `flex: none` (never `flex: 0`, which collapses the row to zero height) for natural-height rows.
- No `overflow: hidden` on variable text containers unless cropped content is intentional.
- Text floors at 1600x900: body text 20px or larger; captions, labels, and footers 16px or larger.

Avoid bullet-only decks; each slide needs visible structure. Read `assets/layouts.md` for structures, `assets/visual-styles.md` for identity, `assets/composition-seeds.md` against sparse decks, `assets/minimal-design.md` for a sober style.

If `file.edit` misses, read the affected range again and retry with exact unique `oldText`. Use `file.write` only when creating the missing `slides.html` for a new deck; never replace an existing deck wholesale. Preserve slide count, format, scope, and exact user wording.

Before building, scan `slides.html`. Revise slides that are only a raw `<table>` or bare `<ul>` into cards, a matrix, a timeline, a scoreboard, or a decision panel. If two slides share the same `.grid`+`.card` surface as the primary composition, convert one into a timeline rail, risk matrix, evidence wall, variance scoreboard, or approval panel. A dark theme is not a visual system; name and render the recurring primitive that makes the deck recognizable — not colored side stripes, tiny rail labels, or border-plus-shadow white cards.

## Fonts

Read `assets/webfonts.md` before writing CSS for Korean-heavy or design-sensitive decks. Paperlogy is the default display and body font, and the exporter injects local WOFF2 fallback before rendering HTML/PDF/PPTX. Also `@import` Pretendard, Freesentation, or Noto Sans KR in `slides.html` so the raw source renders with real typography, keeping Paperlogy first in the stack for offline fallback. Do not paste base64 font data into `slides.html`.

Use the default CSS stack `"Paperlogy", "Noto Sans KR", system-ui, sans-serif`. Do not use emoji as functional icons or bullets; use text labels, CSS markers, inline SVG, or simple shapes.

## Review and Delivery

Every build creates PNGs, `slide-review.json`, `slide-review.md`, contact sheets, `fit-review.json`, and `fit-review-XX.md` when rendering is available. Check expected visible text and fit; clipping, hidden overflow, or edge collision requires revision. `visualEvidenceReliable=false` cannot prove visual quality.

Review substance and surface: request fit, story flow, claim titles, credible examples, and readability. A clean export is not acceptance. The static source review always runs, even when browser rendering is unavailable, so `staticGatePassed` and the design warnings stay authoritative without render images. `qualityGatePassed=true` additionally requires browser render evidence and passing fit checks.

If `slide-review.json` says `staticGatePassed=false` or `needsDesignRevision: true`, resolve the listed warnings and rebuild before delivery unless the user only asked for a mechanical export. Change the composition, not just the colors.

Before delivery, compare the deck against source facts: names, period wording, metrics, dates, owners, targets, misses, and `제공된 자료 없음` labels.

Remaining visual review notes are not a delivery blocker after the required review and revision loop. If requested PPTX/PDF/HTML exists and is usable after the improvement budget, attach it and mention top remaining notes briefly, including when `visualEvidenceReliable=false` meant visual fit could not be verified. Do not spend delivery budget creating or attaching internal review-decision files unless the user asks.

After delivering a below-gate build, judge your trajectory honestly: offer one more improvement round via `ask.confirm` only when the review score was still climbing and you can name the concrete next fix; if it stalled or regressed, say plainly this is your best result with the current approach rather than offering more.

## Formats

If the user does not name a format, build with the default command, run the review loop, and attach only HTML. If the user asks for PDF, build `FORMATS=pdf` and attach HTML plus PDF. If the user asks for PPTX, build `FORMATS=pptx` and attach HTML, PDF when browser rendering worked, and PPTX. Default PPTX is image-backed from slide PNG captures; use `PRESENTATION_PPTX_MODE=native` only when the user explicitly needs editable PowerPoint text. Default PDF keeps selectable text when Chromium is available.

## Validating an Existing PPTX

To check an existing `.pptx` for empty slides, missing titles, excessive shapes, or leftover default fonts: `python3 /workspace/skills/presentation/scripts/skill_runtime.py python /workspace/skills/presentation/scripts/validate_pptx.py <file>.pptx` (bootstraps `python-pptx` on first use).
