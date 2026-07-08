---
name: presentation
description: Generate HTML-first presentation slides and attach requested HTML, PDF, or PPTX files. Also validates existing .pptx files. Use for decks, presentations, pitch decks, research summaries, stakeholder reports, PowerPoint, Google Slides, Keynote, 발표자료, 파워포인트, 피피티.
when_to_use: Use for slides, decks, presentations, PPT/PPTX, PowerPoint, Google Slides, Keynote, 슬라이드, 발표자료, 프레젠테이션, 파워포인트, or 피피티 requests, including validation of empty slides, missing titles, or leftover default fonts.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# Presentation

Create a useful, visually strong deck and attach the requested files. This is an HTML-first skill: `slides.html` is the source of truth, `DESIGN.md` is the design brief, and HTML is the default deliverable. PDF and PPTX are derived formats; PPTX is image-backed by default so it preserves the actual HTML render.

## Workflow

1. Decide the requested format and slide spine before writing files.
2. Use the `file.write` tool directly to create `tmp/<deck-slug>/slides.html` first. Do not use `capability.invoke`, `filesystem.mount.write`, `file.pick`, shell heredocs, or `echo` to write deck source files.
3. Make `slides.html` complete in that first write: full HTML document, CSS, all slide sections, all required source facts, `data-visual-system`, and `data-slide-role` on every slide.
4. Immediately after `slides.html`, add the compact helper files `DESIGN.md`, `deck-brief.md`, and `required-visible-text.txt`. The static review checks DESIGN.md substance and ledger coverage, but they must not delay the primary source file.
5. Run `/workspace/skills/presentation/scripts/build.sh` with `terminal.run` from `workingDirectoryPath: "tmp/<deck-slug>"`. HTML is the source of truth for every format: iterate with the default command, which builds and reviews the HTML each time.
6. Inspect `build/review/slide-review.json`, `slide-review.md`, contact sheets, `fit-review.json`, and each `fit-review-XX.md`, including `visualQualityScore`, `staticGatePassed`, `qualityGatePassed`, `visualEvidenceReliable`, `needsDesignRevision`, and design warnings.
7. If `staticGatePassed=false`, `needsDesignRevision=true`, or rendered image evidence shows fit or design problems, revise `slides.html` with targeted `file.edit` patches and rebuild. Keep revising while the score improves, and never deliver without a reviewed build of the final source; stop when the gate passes, the score stalls across two rebuilds, or a budget status observation says consolidate or finalize. Preserve the design-source marker, requested slide count, source-fact ledger intent, `data-visual-system`, and `data-slide-role`.
8. After the loop ends, convert once for PDF/PPTX requests, then deliver accepted outputs from `tmp/<deck-slug>/build/` plus requested source files with `file.deliver`. Use one call and a `files` array when delivering multiple files.

Use this build command shape:

```json
{
  "command": "/workspace/skills/presentation/scripts/build.sh",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

For PPTX requests, convert with one final build after the review loop:

```json
{
  "command": "FORMATS=pptx /workspace/skills/presentation/scripts/build.sh",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

For PDF requests convert with `FORMATS=pdf`, and for all outputs `FORMATS=all`, in that final build only; every conversion reads the same `slides.html`. `build.sh` uses the working directory name as the output name. With no `FORMATS`, it creates `build/<deck-slug>.html` plus review evidence.

Do not call `terminal.run` with an `arguments` array alone.

Deliver generated files such as `tmp/<deck-slug>/build/<deck-slug>.html` or `tmp/<deck-slug>/build/<deck-slug>.pptx` with `file.deliver`. Do not use shell `cp`, do not deliver from a skill directory, and do not expose `/workspace`, `/tmp`, `file://`, or sandbox paths.

Give every slide a speaker script: an `<aside class="notes">` holding 2-4 spoken sentences in the request language that the presenter can read aloud verbatim. The presenter view shows them; they never render on the slide.

There is no template to fill in; the content, layout, and HTML source are your responsibility.

## Company Data

For IR, 회사소개, and proposal decks pull real data instead of inventing: `company.info.get` (identity), `company.metric.list` (revenue/headcount series for growth charts), `company.record.list` (연혁·funding·products timeline). Company logo: `/workspace/circles/staff/company/logo.png` if present.

## Content Quality

The deck must be useful before it is beautiful. A good deck gives the audience a decision, explanation, lesson, or next action they did not already have.

Build the story spine before authoring: situation, tension, thesis, proof, and close.

Reject shallow content: generic cards, claims without examples, or slides that only restate the prompt are not enough. When source material is thin, create realistic but clearly labeled assumptions or example data. Prefer concrete evidence, a worked example, scenario numbers, workflow states, or a recommendation.

Each slide needs a job: what the audience should learn, decide, or remember; what claim the title makes; what proof supports it; and what visual structure makes it easier to scan.

Use the user's language for slide titles and table headers. Do not write English section titles such as `Executive Summary` when the request is Korean. Show only dates from the source material; never stamp today's date.

Pick one deck archetype, such as pitch, executive briefing, or status report, and use it to decide information density, section sequence, and ending.

Use these slide patterns as the default vocabulary: title thesis, section divider, comparison, matrix, timeline, evidence card, recommendation, and closing ask.

For executive, board, quarterly review, roadmap, investor-style, or design-quality decks, choose a named visual style plus composition seed and make the decision visible. Read `assets/visual-styles.md` and `assets/composition-seeds.md` when more design direction is needed, but do not spend tool turns on reference reading before `slides.html` exists. Start with the approval ask, recommendation, or board decision; show the exact organization, product, and period; preserve the original period wording exactly; use source metrics, target-versus-actual metrics, deltas, implications, owners, dates, and `제공된 자료 없음` when values are missing. Preserve exact source values such as `412,000,000 KRW`, mark status as met, missed, at risk, or not provided, use risk/evidence/response/owner for risks, and end with the exact next actions.

Quarterly review decks usually follow the six-slide board spine in `composition-seeds.md`, which carries the board-floor composition set. Use claim-style titles such as `성장은 확인됐지만 품질 스프린트 승인이 필요합니다`, not topic labels such as `요약`.

Do not deliver a board deck made of plain white title slides, unstyled tables, bare bullet lists, or the same 2x2 card dashboard across slides. Use an executive artifact surface: KPI cards, status chips, variance bars, owner-date timelines, risk matrices, approval panels, and at least one recurring primitive that makes the deck recognizable.

## Source Files

`slides.html` is the source of truth and the first file written. Include `<!-- design-source: DESIGN.md -->`, mirror the chosen colors/typography/layout in CSS, keep one main message per slide, set `data-visual-system` on `<body>` or every slide, `data-slide-role` on every `<section class="slide">`, one top-level slide section per slide.

`DESIGN.md` is useful for design-sensitive decks but not more important than the deck. If you create it, use Stitch-compatible YAML front matter with only `colors`, `typography`, and `layout`, followed by Markdown headings: `Style Prompt`, `Visual Identity Gate`, `Scene`, `Design Thesis`, `Visual System`, `Signature Move`, and `Anti-default Check`.

`deck-brief.md` is an optional planning note, not a deliverable: archetype, story spine, slide count, visual system, signature move, what would be too shallow.

`required-visible-text.txt` is an optional source-fact ledger for review, not a token filter: one source fact or must-appear phrase per line, then represent those facts in `slides.html` with natural layout copy, tables, charts, or labels — organization, product, exact period wording, metric and target values, owners, dates, missing-value labels such as `제공된 자료 없음`. Do not replace Korean period wording such as `2026년 2분기` with `2026 Q2`.

HTML is the layout surface; browser rendering feeds PDF, review images, and image-backed PPTX. The build falls back to native text-backed PPTX when Chromium is unavailable or `PRESENTATION_PPTX_MODE=native` is set:

- Complete HTML document with `<style>` in the head.
- Canonical geometry: `.slide { width: 1600px; height: 900px; }`.
- `@page { size: 1600px 900px; margin: 0; }`.
- Fixed 16:9 frame: every slide is the same flex column — header `flex: none`, body `flex: 1 1 0; min-height: 0`, footer `flex: none; margin-top: auto` — so the footer sits on the identical baseline on every slide. Never absolutely position the footer.
- Fill the frame: give the body `display: flex; flex-direction: column; justify-content: space-between; gap` and stretch major blocks (card grids, tables, charts) with `flex: 1; min-height: 0`, centering card interiors vertically; do not stack everything in the top half.
- Fit-safe containers with `minmax(0, 1fr)`, `min-width: 0`, `min-height: 0`, and `overflow-wrap: anywhere`. Use `flex: none` (never `flex: 0`, which collapses the row to zero height) for natural-height rows.
- No `overflow: hidden` on variable text containers unless cropped content is intentional.
- Text floors at 1600x900: body text 20px or larger; captions, labels, and footers 16px or larger.

Avoid bullet-only decks; bullets may live inside cards, columns, matrix cells, or timelines, but each slide needs visible structure. Read `assets/layouts.md` for structures, `assets/visual-styles.md` for stronger identity, `assets/composition-seeds.md` against sparse or generic decks, `assets/minimal-design.md` for a sober style.

If a `file.edit` patch misses its target text, rewrite the file with `file.write` instead of retrying. Preserve the user's explicit constraints: slide count, output format, scope, and their exact wording.

Before building, scan `slides.html`. Revise slides that are only a raw `<table>` or bare `<ul>` into cards, a matrix, a timeline, a scoreboard, or a decision panel. If two slides share the same `.grid`+`.card` surface as the primary composition, convert one into a timeline rail, risk matrix, evidence wall, variance scoreboard, or approval panel. A dark theme is not a visual system; name and render the recurring primitive that makes the deck recognizable — not colored side stripes, tiny rail labels, or border-plus-shadow white cards.

## Fonts

Read `assets/webfonts.md` before writing CSS for Korean-heavy or design-sensitive decks. Paperlogy is the default display and body font, and the exporter injects local WOFF2 fallback before rendering HTML/PDF/PPTX. Also `@import` Pretendard, Freesentation, or Noto Sans KR in `slides.html` so the raw source renders with real typography, keeping Paperlogy first in the stack for offline fallback. Do not paste base64 font data into `slides.html`.

Use the default CSS stack `"Paperlogy", "Noto Sans KR", system-ui, sans-serif`. Do not use emoji as functional icons or bullets; use text labels, CSS markers, inline SVG, or simple shapes.

## Review and Delivery

The default build, `FORMATS=review`, PDF builds, and PPTX builds create PNGs, `slide-review.json`, `slide-review.md`, contact sheets, `fit-review.json`, and `fit-review-XX.md` when browser rendering or native preview rendering is available. Check each contact sheet with its matching fit review. Every expected visible text item must appear fully inside the slide frame. Missing text, clipped text, hidden overflow, right-edge collision, or bottom-edge collision is revision input. If `visualEvidenceReliable=false`, the review came from fallback images rather than the browser and cannot prove visual quality.

Review substance and surface: request fit, story flow, claim titles, credible examples, and readability. A clean export is not acceptance. The static source review always runs, even when browser rendering is unavailable, so `staticGatePassed` and the design warnings stay authoritative without render images. `qualityGatePassed=true` additionally requires browser render evidence and passing fit checks.

If `slide-review.json` says `staticGatePassed=false` or `needsDesignRevision: true`, resolve the listed warnings and rebuild before delivery unless the user only asked for a mechanical export. Change the composition, not just the colors.

Before delivery, compare the deck against the user's source facts. Check that required company names, period wording, metric values, dates, owners, targets, missed targets, and explicit missing-value labels are present. If the source says a value is unavailable, include `제공된 자료 없음` in the relevant table or note rather than omitting the field.

Remaining visual review notes are not a delivery blocker after the required review and revision loop. If requested PPTX/PDF/HTML exists and is usable after the improvement budget, attach it and mention top remaining notes briefly, including when `visualEvidenceReliable=false` meant visual fit could not be verified. Do not spend delivery budget creating or attaching internal review-decision files unless the user asks.

## Revisions and Formats

For revisions, edit the same `<deck-slug>`. If `tmp/<deck-slug>/slides.html` is gone, restore editable source from `artifacts/<deck-slug>/source/`, apply changes, rebuild, and deliver with `overwrite: true`.

If the user does not name a format, build with the default command, run the review loop, and attach only HTML. If the user asks for PDF, build `FORMATS=pdf` and attach HTML plus PDF. If the user asks for PPTX, build `FORMATS=pptx` and attach HTML, PDF when browser rendering worked, and PPTX. Default PPTX is image-backed from slide PNG captures; use `PRESENTATION_PPTX_MODE=native` only when the user explicitly needs editable PowerPoint text. Default PDF keeps selectable text when Chromium is available.

## Validating an Existing PPTX

To check an existing `.pptx` file for empty slides, missing titles, excessive shape count, or leftover default fonts, run `python3 /workspace/skills/presentation/scripts/skill_runtime.py python /workspace/skills/presentation/scripts/validate_pptx.py <path-to-file>.pptx`.

`skill_runtime.py` bootstraps `python-pptx` from `scripts/requirements.txt` on first use.
