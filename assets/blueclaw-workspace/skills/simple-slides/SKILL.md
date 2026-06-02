---
name: simple-slides
description: Generate clean presentation slides with Marp and attach the requested files. Use for slides, slide decks, presentations, pitch decks, research summaries, stakeholder reports, PPTX, PowerPoint, Google Slides, Keynote, 발표자료, 파워포인트, 피피티.
when_to_use: Use for slides, slide decks, presentations, pitch decks, research summaries, stakeholder reports, PPT, PPTX, PowerPoint, Google Slides, Keynote, 슬라이드, 발표, 발표자료, 프레젠테이션, 프리젠테이션, 파워포인트, or 피피티 requests.
allowed-tools:
  - terminal.run
  - file.read
  - file.write
  - file.edit
  - file.patch
  - file.promote
  - file.attach
  - artifact.review
---

# Simple Slides

Create a useful, visually strong slide deck and attach the requested files. This skill is intentionally small: write the deck yourself from the user request, use a short deck brief plus Stitch-compatible `DESIGN.md` as the design source, build with Marp, inspect rendered image evidence, and attach the generated artifacts.

## Workflow

Use this order for normal requests:

1. Decide the deck archetype and main narrative flow before writing source files.
2. Use `file.write` to create `tmp/<deck-slug>/deck-brief.md`; keep it short and natural-language: request intent, audience, deck archetype, main thesis, slide sequence, visual direction, must-show content, and what would be too shallow.
3. Use `file.write` to create `tmp/<deck-slug>/DESIGN.md`; do not use Blueclaw internal temporary paths.
4. Use `file.write` to create `tmp/<deck-slug>/presentation.md`.
5. Use `terminal.run` with `workingDirectoryPath: "tmp/<deck-slug>"` to run the deterministic build script from the skill directory.
6. Inspect `build/review/slide-review.md` or `build/review/slide-review.json`, each contact sheet image, and its matching `fit-review-XX.md`.
7. Call `artifact.review` for each `contact-sheet-XX.png` with the matching expected visible text from `fit-review-XX.md`, the deck brief, and the deck archetype.
8. Write `build/review/review-decision.json` with inspected evidence, issues, changes made, accepted warnings, `remainingNotes`, and a short summary.
9. Run `python3 /workspace/skills/simple-slides/scripts/accept_review.py build/review` to summarize review readiness.
10. If rendered images, deterministic review, or LLM review show useful improvements and the improvement budget remains, revise `presentation.md` and rebuild. Repeat at most three times. If the deck is usable after the budget, attach it and report the top remaining notes instead of failing solely on visual quality.
11. Promote final files from `tmp/<deck-slug>/build/` to `artifacts/<deck-slug>/` with `file.promote` unless the user requested a circle or shared destination.
12. Use `file.attach` on promoted files only, and attach only the files the user requested.

`file.write` creates parent directories, so do not spend a terminal call on `mkdir`. Do not use `file.pick`; it is for user-local file selection, not deck creation. Do not read reference assets during a normal request unless you truly need extra detail after drafting. The baseline below is enough for most decks.

Use this command shape after the source files exist. Do not copy `build.sh` into the task directory:

```json
{
  "command": "NAME=<deck-slug> /workspace/skills/simple-slides/scripts/build.sh",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

If the user explicitly requests one format, narrow the build with `FORMATS`. For `html만`, use `FORMATS=html NAME=<deck-slug> /workspace/skills/simple-slides/scripts/build.sh`. For a normal full deck, omit `FORMATS` so HTML, PPTX, PDF, notes, and review evidence are produced.

Use `file.promote` to promote final outputs after the build succeeds:

```json
{
  "paths": [
    "tmp/<deck-slug>/build/<deck-slug>.pptx",
    "tmp/<deck-slug>/build/<deck-slug>.pdf",
    "tmp/<deck-slug>/build/<deck-slug>.html",
    "tmp/<deck-slug>/build/<deck-slug>-notes.txt"
  ],
  "destinationDirectoryPath": "artifacts/<deck-slug>",
  "overwrite": true
}
```

Adjust the promoted filenames to the formats actually requested. Do not use shell `cp`, do not create an `artifacts` directory from `/workspace`, and do not promote from a skill directory.

The build script is responsible for Marp availability. It uses an existing `marp`, otherwise installs `assets/package.json` into `$BLUECLAW_REQUESTER_TMP/.skill-env/simple-slides/node` and uses `/workspace/shared/cache/dependencies` only as a package cache. Do not stop with a missing Marp message before running the bundled build script.

Do not look for a content generator or layout renderer. There is no template deck to fill in. The content, layout, and Marp source are your responsibility.

## Source Files

`deck-brief.md` is the planning note for the deck. Use it to pin the audience, thesis, slide sequence, visual direction, and the line between a usable deck and a shallow deck. Keep it brief; do not let it become a hidden second deliverable.

`DESIGN.md` is required. It must stay Stitch-compatible: YAML front matter with only `colors`, `typography`, and `layout`, followed by a short rationale. Write it directly for the user's deck instead of copying a template. Keep the default direction minimal and mostly black-and-white unless the topic clearly needs a stronger accent. Read `assets/minimal-design.md` as a reference when you need a sober presentation style.

Pick one deck archetype before writing `DESIGN.md`: pitch, research report, executive briefing, education, portfolio, product proposal, or status report. Infer the archetype from the audience, decision, and requested deliverable when the user does not name one. The archetype should determine first-slide structure, information density, section sequence, fake data style, and whether the deck should end in a recommendation, closing ask, next lesson, or status decision.

Use this compact token shape:

```yaml
---
colors:
  background: "#F8FAFC"
  surface: "#FFFFFF"
  ink: "#111827"
  muted: "#64748B"
  accent: "#111827"
  line: "#CBD5E1"
typography:
  display: "Paperlogy, Noto Sans KR, system Korean sans"
  body: "Paperlogy, Noto Sans KR, system Korean sans"
layout:
  canvas: "16:9"
  margin: "68px"
  rhythm: "8px"
  radius: "8px"
---

# Visual Direction

Short rationale for this deck's tone and layout.
```

`DESIGN.md` is a design brief, not a theme file. Use it while authoring `presentation.md`; do not expect `build.sh` or Marp to apply it automatically.

`presentation.md` is the source of truth for the deck. Write it directly from the user request, include `<!-- design-source: DESIGN.md -->` near the top, mirror the `DESIGN.md` typography/colors/layout into the Marp `style` block yourself, keep one main message per slide, and avoid a trailing slide separator.

Use Marp as the slide compiler, not as the layout designer. Prefer raw HTML inside each slide:

- Use Marp front matter, CSS in `style`, and `---` slide separators.
- Inside slides, use `<div class="frame ...">`, `<div class="card">`, `<div class="comparison">`, `<div class="matrix">`, and similar semantic containers.
- Avoid default Markdown-only title + bullet layouts except for very simple appendices.
- Do not write raw `<section>` tags; Marp owns slide sections.
- Make each slide title a conclusion or claim, not a topic label.
- Do not make a bullet-only deck. Bullets may live inside cards, matrix cells, timelines, or appendix blocks, but each slide needs a visible designed structure.

Use this HTML-first page shape when you need a reliable minimal look:

- `.frame`: full-slide inner container
- `.top-rule`: thin black line near the top
- `.eyebrow`: small section label
- `.takeaway`: one-sentence conclusion band
- `.cards`, `.card`: peer items, risks, strengths, or next steps
- `.comparison`: pros/cons, before/after, alternatives
- `.matrix`: criteria-based decisions or tradeoffs
- `.timeline`: sequence, rollout, or maturity path
- `.recommendation`: final verdict and next action

Use fit-safe CSS by default. `.frame` should use fixed 16:9 slide geometry with grid or flex layout, and content regions should use `minmax(0, 1fr)`, `min-width: 0`, `min-height: 0`, and `overflow-wrap: anywhere`. Do not use `overflow: hidden` or `overflow: clip` on cards, matrix cells, columns, or other variable text containers unless the user explicitly asks for cropped content.

Keep variable text within a line budget: title 1-2 lines, takeaway 1-2 lines, cards 2-4 lines, matrix cells 2-3 lines, and timeline steps 2-3 lines. CSS reduces overflow risk, but it is not a guarantee; final fit must be checked in the render review bundle.

Use these slide patterns as the default vocabulary: title thesis, section divider, comparison, matrix, timeline, evidence card, recommendation, and closing ask. A finished deck should feel like a paced argument, not a sequence of topic pages.

Use `file.write` for `DESIGN.md` and `presentation.md`. Do not create source files with shell heredocs or `echo` inside `terminal.run`; reserve `terminal.run` for running the build.

Iterate on `presentation.md` when the rendered images show useful improvements. Preserve important request constraints directly in the deck, such as `할 수`, `역량`, `capability`, `what I can do`, `6장`, or `html만`.

## Layout Choice

Pick the layout from the content intent, not from a template. Read `assets/layouts.md` when choosing structures or writing CSS classes.

Avoid bullet-only decks. Bullets are acceptable inside a card, column, matrix cell, or takeaway block, but the slide itself should have a visible structure. For a strengths/weaknesses analysis, prefer cards, comparison columns, a matrix, and a recommendation slide over six plain lists. Good pages often have a slim top rule, an assertion title, a one-sentence takeaway band, and a structured content area.

## Fonts

Use fonts that will actually render in the generated HTML/PDF/PPTX.

Use vendored Paperlogy as the default display and body font. Read `assets/webfonts.md` and put the local `@font-face` rules for the four vendored WOFF2 files at the top of the Marp `style` block in `presentation.md`. The build script keeps `--allow-local-files` for PDF/PPTX and inlines local `.woff2`/`.woff` URLs into the HTML artifact.

Use this stack unless the user explicitly asks for a different font:

```css
font-family: "Paperlogy", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji", sans-serif;
```

Pretendard is only a fallback example when requested. If local font files are unavailable in an unusual runtime, use a robust Korean-capable system stack instead.

If the deck uses embedded fonts, mention Paperlogy in `DESIGN.md`; otherwise keep `typography.display` and `typography.body` as system Korean sans.

Do not use emoji as functional icons or bullets in presentation body text. Emoji fallback depends on the browser, PDF renderer, and PowerPoint/Keynote environment, and missing glyphs may render as boxes. Use text labels, CSS markers, inline SVG, or simple geometric PowerPoint shapes instead.

## Review Loop

The full build creates review PNGs, `slide-review.json`, `slide-review.md`, 4-slide contact sheets, `fit-review.json`, and `fit-review-XX.md` files. Check each contact sheet together with its matching fit review file. Every expected visible text item must appear fully inside the slide frame. Treat missing text, clipped text, hidden overflow, right-edge collision, or bottom-edge collision as a reason to revise `presentation.md` and rebuild.

`textOverflowRisk` and `frameFitRisk` are deterministic warning signals, not substitutes for visual review. They identify slides that require extra attention in the contact sheet.

Use `artifact.review` as the visual reviewer for each contact sheet. Include the deck intent, `deck-brief.md`, deck archetype, the contact sheet image path, and the expected visible text from the matching fit review file. Treat the returned issues as LLM review notes. Revise and rebuild when the notes identify a useful improvement and the improvement budget remains; otherwise record accepted warnings or `remainingNotes` in `build/review/review-decision.json`.

Before `file.promote`, run:

```json
{
  "command": "python3 /workspace/skills/simple-slides/scripts/accept_review.py build/review",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

`accept_review.py` reports missing inspection evidence or unresolved deterministic warnings, but those warnings are improvement notes, not a delivery blocker. If the requested PPTX/PDF/HTML exists and is usable, promote and attach it after the improvement budget, then mention the top remaining review notes briefly.

## Output

If the user asks for `html만`, build and attach only the HTML. If the user does not restrict formats, attaching PPTX, PDF, HTML, and notes is a good default.

Marp PPTX output is a visual-fidelity artifact and may represent slides as images instead of fully editable PowerPoint objects. When the user needs a PPTX that preserves design but only a few fields must remain editable, use the `pptx` skill's hybrid pattern: static full-slide image background plus editable text overlays for the specific fields that need later changes.

Do not say file delivery is impossible when the local tools are available. If the result is imperfect but usable, attach it and be honest about limitations. Never expose `sandbox:/mnt/data`, `file://`, `/workspace`, `/tmp`, or other local paths to the user.
