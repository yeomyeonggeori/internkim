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

1. Decide the deck archetype and story before writing files.
2. Create `tmp/<deck-slug>/deck-brief.md` with request intent, audience, deck archetype, main thesis, story spine, slide count, slide sequence, chosen visual style, visual system, signature move, must-show content, and what would be too shallow.
3. Create `tmp/<deck-slug>/required-visible-text.txt` with one source fact or must-appear phrase per line for required names, periods, values, dates, owners, and missing-value labels.
4. Create `tmp/<deck-slug>/DESIGN.md`. For design-sensitive decks, read `assets/visual-styles.md` first and pass its Visual Identity Gate.
5. Create `tmp/<deck-slug>/slides.html`.
6. Run `/workspace/skills/presentation/scripts/build.sh` with `terminal.run` from `workingDirectoryPath: "tmp/<deck-slug>"` using the exact `command` string shape below.
7. Inspect `build/review/slide-review.json`, `slide-review.md`, contact sheets, `fit-review.json`, and each `fit-review-XX.md`, including `visualQualityScore`, `qualityGatePassed`, `visualEvidenceReliable`, `needsDesignRevision`, and design warnings.
8. Use `artifact.review` for executive, board, quarterly, investor, roadmap, pitch, or design-quality requests. Include deck intent, `deck-brief.md`, deck archetype, contact sheet image, design warnings, and expected visible text. If it is unavailable, manually inspect the contact sheet and review notes before accepting the deck.
9. If deterministic review, `qualityGatePassed=false`, `needsDesignRevision`, rendered image evidence, or LLM notes show useful improvements, revise `slides.html` and rebuild. Repeat at most three times. When rewriting after a failed build, preserve the design-source marker, requested slide count, and source-fact ledger intent; also preserve `data-visual-system` and `data-slide-role`.
10. Deliver accepted outputs from `tmp/<deck-slug>/build/` plus requested source files with `file.deliver`. Use one call and a `files` array when delivering multiple files.

Use this build command shape:

```json
{
  "command": "/workspace/skills/presentation/scripts/build.sh",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

`build.sh` uses the working directory name as the output name, so do not pass `NAME=` unless a different output basename is truly required. With no `FORMATS`, it creates `build/<deck-slug>.html` plus internal review evidence; deliver HTML unless the user names another file format.

Use `FORMATS=pdf` for PDF, `FORMATS=pptx` for PPTX, and `FORMATS=all` for notes plus all export formats. PDF and PPTX builds also create review evidence. Use `FORMATS=html` only for a mechanical no-review HTML export.

Do not call `terminal.run` with an `arguments` array alone. Use the `command` string examples above for this skill's build path.

Deliver generated files such as `tmp/<deck-slug>/build/<deck-slug>.html`, `tmp/<deck-slug>/build/<deck-slug>.pdf`, or `tmp/<deck-slug>/build/<deck-slug>.pptx` with `file.deliver`. Do not use shell `cp`, do not deliver from a skill directory, and do not expose `/workspace`, `/tmp`, `file://`, or sandbox paths.

Do not look for a content generator or template deck. There is no template to fill in. The content, layout, and HTML source are your responsibility.

## Content Quality

The deck must be useful before it is beautiful. A good deck gives the audience a decision, explanation, lesson, or next action they did not already have.

Build the story spine before authoring: situation, tension, thesis, proof, and close.

Reject shallow content: a welcome slide plus generic cards, repeated overview/features/benefits labels, claims without examples, vague business adjectives, empty mock data, or slides that only restate the prompt are not enough. When source material is thin, create realistic but clearly labeled assumptions or example data. Prefer concrete evidence, a worked example, scenario numbers, workflow states, or a recommendation.

Each slide needs a job: what the audience should learn, decide, or remember; what claim the title makes; what proof supports it; and what visual structure makes it easier to scan.

Pick one deck archetype: pitch, research report, executive briefing, education, portfolio, product proposal, or status report. Use that choice to decide information density, section sequence, fake data style, and ending.

Use these slide patterns as the default vocabulary: title thesis, section divider, comparison, matrix, timeline, evidence card, recommendation, and closing ask.

For executive, board, quarterly review, roadmap, investor-style, or design-quality decks, read `assets/visual-styles.md` and `assets/composition-seeds.md`, choose a named style plus seed, and make the decision visible. Start with the approval ask, recommendation, or board decision; show the exact organization, product, and period; preserve the original period wording exactly; use source metrics, target-versus-actual metrics, deltas, implications, owners, dates, and `제공된 자료 없음` when values are missing. Preserve exact source values such as `412,000,000 KRW`, mark status as met, missed, at risk, or not provided, use risk/evidence/response/owner for risks, and end with the exact next actions.

Quarterly review decks should usually use this six-slide spine: decision title, executive summary, target-versus-actual metrics, roadmap timeline, risk-response matrix, and approval/next steps. The board floor in `composition-seeds.md` includes Decision cover, Executive dashboard, Metrics scoreboard, Roadmap timeline, Risk matrix, and Approval panel. Use claim-style titles such as `성장은 확인됐지만 품질 스프린트 승인이 필요합니다`, not topic labels such as `요약`.

Do not deliver a board deck made of plain white title slides, unstyled tables, bare bullet lists, or the same 2x2 card dashboard across slides. Use an executive artifact surface: KPI cards, status chips, variance bars, owner-date timelines, risk matrices, approval panels, and at least one recurring primitive that makes the deck recognizable.

## Source Files

`deck-brief.md` is a brief planning note, not a second deliverable. It pins the audience, thesis, slide sequence, visual direction, and quality bar.

`required-visible-text.txt` is the source-fact ledger for review, not a token filter. Put one source fact or must-appear phrase per line, then intentionally represent those facts in `slides.html` with natural layout copy, tables, charts, or labels. Include organization, product, exact period wording, user-provided metric values, target values, owners, dates, and missing-value labels such as `제공된 자료 없음`. Do not replace Korean period wording such as `2026년 2분기` with `2026 Q2`; include both only if both are visible. Use the ledger during visual review and final self-check rather than forcing awkward exact phrasing into the design.

`DESIGN.md` is a design brief, not a theme file. Write it directly for the user's deck; do not expect `build.sh` to apply it automatically. It must be Stitch-compatible: YAML front matter with only `colors`, `typography`, and `layout`, followed by Markdown design reasoning. Include `## Style Prompt`, `## Visual Identity Gate`, `## Scene`, `## Design Thesis`, `## Visual System`, `## Signature Move`, and `## Anti-default Check`.

Use compact YAML with `colors`, `typography`, and `layout` keys. Then add Markdown headings: `Style Prompt`, `Visual Identity Gate`, `Scene`, `Design Thesis`, `Visual System`, `Signature Move`, and `Anti-default Check`.

`slides.html` is the source of truth for the deck. Include `<!-- design-source: DESIGN.md -->`, mirror `DESIGN.md` colors/typography/layout in CSS, keep one main message per slide, set `data-visual-system` on `<body>` or every slide, set `data-slide-role` on every `<section class="slide">`, and use one top-level slide section per slide.

Use HTML as the layout surface. Browser rendering is the source for PDF, review images, and image-backed PPTX. The build can still create a native text-backed PPTX when Chromium is unavailable or `PRESENTATION_PPTX_MODE=native` is set:

- Complete HTML document with `<style>` in the head.
- Canonical geometry: `.slide { width: 1600px; height: 900px; }`.
- `@page { size: 1600px 900px; margin: 0; }`.
- Fixed 16:9 frame with grid or flex layout.
- Fit-safe containers with `minmax(0, 1fr)`, `min-width: 0`, `min-height: 0`, and `overflow-wrap: anywhere`.
- No `overflow: hidden` on variable text containers unless cropped content is intentional.

Avoid bullet-only decks. Bullets may live inside cards, columns, matrix cells, timelines, or appendix blocks, but each slide needs visible structure. Read `assets/layouts.md` when choosing structures, `assets/visual-styles.md` when the deck needs stronger identity, `assets/composition-seeds.md` when a deck risks looking sparse or generic, and `assets/minimal-design.md` when a sober presentation style is needed.

Use a task-local script for `DESIGN.md` and `slides.html`, then run it with `terminal.run`. Do not create source files with shell heredocs or `echo` inside the command string. Preserve explicit request constraints such as `할 수`, `역량`, `capability`, `what I can do`, `6장`, or `html만`.

Before building, scan `slides.html`. If a slide is only a raw `<table>` or bare `<ul>`, revise it into cards, a matrix, a timeline, a scoreboard, or a decision panel. If two or more slides share the same `.grid` plus `.card` surface as the primary composition, convert one into a timeline rail, risk matrix, evidence wall, variance scoreboard, or approval panel. A dark theme is not a visual system; name and render the recurring primitive that makes the deck recognizable. Do not use colored side stripes, tiny rail labels, or border-plus-shadow white cards as the main identity.

## Fonts

Use web fonts in `slides.html` when they improve Korean typography, brand fit, or visual hierarchy. Read `assets/webfonts.md` before writing CSS for Korean-heavy or design-sensitive decks. Paperlogy is the default display and body font, and the exporter injects local WOFF2 fallback before rendering HTML/PDF/PPTX. You may import Pretendard, Freesentation, or Noto Sans KR, but keep Paperlogy in the stack so offline rendering still has a stable Korean fallback. Do not paste base64 font data into `slides.html`.

Use the default CSS stack `"Paperlogy", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji", sans-serif`. Do not use emoji as functional icons or bullets; use text labels, CSS markers, inline SVG, or simple shapes.

## Review and Delivery

The default build, `FORMATS=review`, PDF builds, and PPTX builds create PNGs, `slide-review.json`, `slide-review.md`, contact sheets, `fit-review.json`, and `fit-review-XX.md` when browser rendering or native preview rendering is available. Check each contact sheet with its matching fit review. Every expected visible text item must appear fully inside the slide frame. Missing text, clipped text, hidden overflow, right-edge collision, or bottom-edge collision is revision input. If `visualEvidenceReliable=false`, the review came from fallback images rather than the browser and cannot prove visual quality.

Review substance and surface: answer to the request, story flow, claim titles, credible examples, visual balance, readability, and fit. A clean export is not acceptance. `qualityGatePassed=true` means the browser render is available, fit checks pass, and the deterministic visual score clears the threshold.

If `slide-review.json` says `qualityGatePassed=false`, `needsDesignRevision: true`, or if warnings mention weak visual identity, unreliable visual evidence, side stripes, ghost cards, generic card grids, raw tables, bare lists, or topic titles, make at least one design pass before delivery unless the user only asked for a mechanical export. Change the composition, not just the colors: convert a repeated card grid into a rail, matrix, evidence wall, variance scoreboard, risk room, or approval panel.

Before delivery, compare the deck against the user's source facts. Check that required company names, period wording, metric values, dates, owners, targets, missed targets, and explicit missing-value labels are present. If the source says a value is unavailable, include `제공된 자료 없음` in the relevant table or note rather than omitting the field.

Remaining visual review notes are not a delivery blocker after the required review and revision loop. If requested PPTX/PDF/HTML exists and is usable after the improvement budget, attach it and mention top remaining notes briefly. Do not spend delivery budget creating or attaching internal review-decision files unless the user asks.

## Revisions and Formats

For revisions, edit the same `<deck-slug>`. If `tmp/<deck-slug>/slides.html` is gone, restore editable source from `artifacts/<deck-slug>/source/`, apply changes, rebuild, and deliver with `overwrite: true`.

If the user does not name a format, build with the default command, run the review loop, and attach only HTML. If the user asks for PDF, build `FORMATS=pdf` and attach HTML plus PDF. If the user asks for PPTX, build `FORMATS=pptx` and attach HTML, PDF when browser rendering worked, and PPTX. Default PPTX is image-backed from slide PNG captures; use `PRESENTATION_PPTX_MODE=native` only when the user explicitly needs editable PowerPoint text. Default PDF keeps selectable text when Chromium is available.

## Validating an Existing PPTX

To check an existing `.pptx` file for empty slides, missing titles, excessive shape count, or leftover default fonts, run `python3 /workspace/skills/presentation/scripts/skill_runtime.py python /workspace/skills/presentation/scripts/validate_pptx.py <path-to-file>.pptx`.

This validator reports structurally, not visually; it does not judge design quality. `skill_runtime.py` bootstraps `python-pptx` from `scripts/requirements.txt` on first use.
