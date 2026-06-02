---
name: simple-slides
description: Generate clean presentation slides from an HTML-first source and attach the requested files. Use for slides, slide decks, presentations, pitch decks, research summaries, stakeholder reports, PPTX, PowerPoint, Google Slides, Keynote, 발표자료, 파워포인트, 피피티.
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

Create a useful, visually strong slide deck and attach the requested files. This skill is intentionally small: write the deck yourself from the user request, use a short deck brief plus Stitch-compatible `DESIGN.md` as the design source, build from `slides.html`, inspect rendered image evidence, and attach the generated artifacts.

The source of truth is `slides.html`. Revisions should edit that same HTML source, then re-export HTML/PDF/PPTX from it.

## Workflow

Use this order for normal requests:

1. Decide the deck archetype and main narrative flow before writing source files.
2. Use `file.write` to create `tmp/<deck-slug>/deck-brief.md`; keep it short and natural-language: request intent, audience, deck archetype, main thesis, story spine, slide sequence, visual direction, must-show content, and what would be too shallow.
3. Use `file.write` to create `tmp/<deck-slug>/DESIGN.md`; do not use Blueclaw internal temporary paths.
4. Use `file.write` to create `tmp/<deck-slug>/slides.html`.
5. Use `terminal.run` with `workingDirectoryPath: "tmp/<deck-slug>"` to run the deterministic build script from the skill directory.
6. Inspect `build/review/slide-review.md` or `build/review/slide-review.json`, each contact sheet image, and its matching `fit-review-XX.md`.
7. If the deck is visually important and budget remains, call `artifact.review` for contact sheets with expected visible text from `fit-review-XX.md`, the deck brief, and the deck archetype. Skip extra review calls when deterministic review is clean and delivery budget is tight.
8. If rendered images, deterministic review, or LLM review show useful improvements and the improvement budget remains, revise `slides.html` and rebuild. Repeat at most three times. If the deck is usable after the budget, attach it and report the top remaining notes instead of failing solely on visual quality.
9. Promote final files from `tmp/<deck-slug>/build/` to `artifacts/<deck-slug>/` with `file.promote` unless the user requested a circle or shared destination.
10. Use one `file.attach` call with a `files` array for all requested promoted files.

`file.write` creates parent directories, so do not spend a terminal call on `mkdir`. Do not use `file.pick`; it is for user-local file selection, not deck creation. Do not read reference assets during a normal request unless you truly need extra detail after drafting. The baseline below is enough for most decks.

Use this command shape after the source files exist. Do not copy `build.sh` into the task directory:

```json
{
  "command": "NAME=<deck-slug> /workspace/skills/simple-slides/scripts/build.sh",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

If the user explicitly requests one format, narrow the build with `FORMATS`. For `html만`, use `FORMATS=html NAME=<deck-slug> /workspace/skills/simple-slides/scripts/build.sh`. For a normal full deck, omit `FORMATS` so HTML, text-preserving PDF, image-backed PPTX, notes, and review evidence are produced.

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

The build script is responsible for export tooling. It uses Playwright with the managed Chromium executable to render slide screenshots, generate a print-CSS PDF with live text and embedded local fonts, and build a visual-fidelity PPTX from the rendered slide images. It installs `assets/package.json` into `$BLUECLAW_REQUESTER_TMP/.skill-env/simple-slides/node` and uses `/workspace/shared/cache/dependencies` only as a package cache. Do not stop with a missing browser package message before running the bundled build script.

Do not look for a content generator or layout renderer. There is no template deck to fill in. The content, layout, and HTML source are your responsibility.

## Content Quality

The deck must be useful before it is beautiful. A good deck gives the audience a decision, explanation, lesson, or next action they did not already have.

Before writing `slides.html`, use `deck-brief.md` to make a compact story spine:

- Situation: what the audience already knows or needs.
- Tension: what is confusing, risky, expensive, or unresolved.
- Thesis: the deck's answer in one sentence.
- Proof: the examples, data, workflow, or reasoning that makes the thesis credible.
- Close: the decision, ask, next step, or lesson the deck should leave behind.

Reject shallow content during your own review. These are not enough for a finished deck: a welcome slide plus generic cards, repeated "overview / features / benefits" labels, claims without examples, vague business adjectives, empty mock data, or slides that only restate the user's prompt.

When the user provides little source material, create realistic but clearly labeled assumptions or example data. Prefer concrete examples over filler. For a capability deck, show actual workflows, sample inputs/outputs, before/after states, risks handled, and when the capability should not be used. For a business deck, show the decision criteria, tradeoffs, scenario numbers, and recommended path. For an education deck, show a worked example and a check-for-understanding moment.

Each slide needs a slide job:

- What should the audience learn, decide, or remember?
- What claim does the title make?
- What proof or example supports it?
- What visual structure makes the claim easier to scan?
- What sentence can be deleted because it only decorates the page?

If a rendered contact sheet looks visually clean but the content feels thin, revise `slides.html` for substance, not decoration. Add a concrete example, sharper contrast, a small matrix, a workflow step, or a final recommendation before changing colors.

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

`DESIGN.md` is a design brief, not a theme file. Use it while authoring `slides.html`; do not expect `build.sh` to apply it automatically.

`slides.html` is the source of truth for the deck. Write it directly from the user request, include `<!-- design-source: DESIGN.md -->` near the top, mirror the `DESIGN.md` typography/colors/layout into CSS yourself, keep one main message per slide, and use one top-level `<section class="slide">` per slide.

Use browser-rendered HTML as the layout surface:

- Use a complete HTML document with `<style>` in the head.
- Use `1600px × 900px` as the canonical slide coordinate system. Define `.slide { width: 1600px; height: 900px; }`.
- Add `@page { size: 1600px 900px; margin: 0; }` so PDF export uses the same coordinate system.
- Use `<section class="slide">` with fixed 16:9 pixel geometry and full-slide CSS.
- Inside slides, use `<div class="frame">`, `<div class="card">`, `<div class="comparison">`, `<div class="matrix">`, and similar semantic containers.
- Make each slide title a conclusion or claim, not a topic label.
- Do not make a bullet-only deck. Bullets may live inside cards, matrix cells, timelines, or appendix blocks, but each slide needs a visible designed structure.
- Put speaker notes in `<aside class="speaker-notes">...</aside>` if notes are useful; they are excluded from visible text review.

The standalone HTML viewer, render-review screenshots, PDF export, and image-backed PPTX export all use the same `1600px × 900px` coordinate system.

Use this page shape when you need a reliable minimal look:

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

Use `file.write` for `DESIGN.md` and `slides.html`. Do not create source files with shell heredocs or `echo` inside `terminal.run`; reserve `terminal.run` for running the build.

Iterate on `slides.html` when the rendered images show useful improvements. Preserve important request constraints directly in the deck, such as `할 수`, `역량`, `capability`, `what I can do`, `6장`, or `html만`.

## Layout Choice

Pick the layout from the content intent, not from a template. Read `assets/layouts.md` when choosing structures or writing CSS classes.

Avoid bullet-only decks. Bullets are acceptable inside a card, column, matrix cell, or takeaway block, but the slide itself should have a visible structure. For a strengths/weaknesses analysis, prefer cards, comparison columns, a matrix, and a recommendation slide over six plain lists. Good pages often have a slim top rule, an assertion title, a one-sentence takeaway band, and a structured content area.

## Fonts

Use fonts that will actually render in the generated HTML/PDF/PPTX.

Use Paperlogy as the default display and body font. External font URLs are acceptable, but the final HTML export always injects vendored `PaperlogyLocal` as a data-URL fallback and adds it after `"Paperlogy"` in font-family lists. This keeps Mattermost attachments readable when CDN fonts are blocked.

Use this stack unless the user explicitly asks for a different font:

```css
font-family: "Paperlogy", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji", sans-serif;
```

Pretendard is only an alternate example when requested. If local font files are unavailable in an unusual runtime, use a robust Korean-capable system stack instead.

If the deck uses embedded fonts, mention Paperlogy in `DESIGN.md`; otherwise keep `typography.display` and `typography.body` as system Korean sans.

Do not use emoji as functional icons or bullets in presentation body text. Emoji glyph support depends on the browser, PDF renderer, and PowerPoint/Keynote environment, and missing glyphs may render as boxes. Use text labels, CSS markers, inline SVG, or simple geometric PowerPoint shapes instead.

## Review Loop

The full build creates review PNGs, `slide-review.json`, `slide-review.md`, 4-slide contact sheets, `fit-review.json`, and `fit-review-XX.md` files. Check each contact sheet together with its matching fit review file. Every expected visible text item must appear fully inside the slide frame. Treat missing text, clipped text, hidden overflow, right-edge collision, or bottom-edge collision as a reason to revise `slides.html` and rebuild.

`textOverflowRisk` and `frameFitRisk` are deterministic warning signals, not substitutes for visual review. They identify slides that require extra attention in the contact sheet.

Use `artifact.review` as the visual reviewer when visual judgment is worth the budget. Include the deck intent, `deck-brief.md`, deck archetype, the contact sheet image path, and the expected visible text from the matching fit review file. Treat returned issues as LLM review notes. Revise and rebuild when the notes identify a useful improvement and the improvement budget remains; otherwise attach the usable deck and briefly mention remaining notes in the final reply.

The review must check both substance and surface:

- Substance: Does the deck answer the request, carry the story spine, use claim titles, and include credible examples or evidence?
- Flow: Does each slide advance the argument instead of repeating the previous slide?
- Visual quality: Does the rendered image look intentional, balanced, and readable?
- Fit: Is every expected visible text item fully inside the frame?

Do not accept a deck just because `slide-review.json` passed. The deterministic review only catches rendering risks; you still need to judge whether the visible artifact is worth sending.

Attach multiple requested files together:

```json
{
  "files": [
    {"path": "artifacts/<deck-slug>/<deck-slug>.html", "contentType": "text/html"},
    {"path": "artifacts/<deck-slug>/<deck-slug>.pptx", "contentType": "application/vnd.openxmlformats-officedocument.presentationml.presentation"}
  ]
}
```

If the requested PPTX/PDF/HTML exists and is usable, promote and attach it after the improvement budget, then mention the top remaining review notes briefly. Visual review notes are not a delivery blocker. Do not spend delivery budget creating or attaching internal review-decision files unless the user explicitly asks for review metadata.

## Output

If the user asks for `html만`, build and attach only the HTML. If the user does not restrict formats, attaching PPTX, PDF, HTML, and notes is a good default.

Default PPTX output is image-backed: each slide is a rendered browser screenshot placed into PowerPoint. This preserves visual fidelity, but it is not fully editable. Default PDF output is browser print PDF from the same HTML, so text remains selectable and local fonts are embedded when Chromium supports it. When the user explicitly needs editable PowerPoint objects or wants to modify an existing PPTX, use the `pptx` skill instead. When a user asks for later design/content changes to a generated deck, edit `slides.html` and re-export the requested formats.

Do not say file delivery is impossible when the local tools are available. If the result is imperfect but usable, attach it and be honest about limitations. Never expose `sandbox:/mnt/data`, `file://`, `/workspace`, `/tmp`, or other local paths to the user.
