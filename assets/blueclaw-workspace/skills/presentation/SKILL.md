---
name: presentation
description: Generate clean presentation slides from an HTML-first source and attach the requested files. Also validates an existing .pptx file. Use for slides, slide decks, presentations, pitch decks, research summaries, stakeholder reports, PPTX, PowerPoint, Google Slides, Keynote, 발표자료, 파워포인트, 피피티.
when_to_use: Use for slides, slide decks, presentations, pitch decks, research summaries, stakeholder reports, PPT, PPTX, PowerPoint, Google Slides, Keynote, 슬라이드, 발표, 발표자료, 프레젠테이션, 프리젠테이션, 파워포인트, or 피피티 requests, including validating an existing .pptx file for empty slides, missing titles, or leftover default fonts.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# Presentation

Create a useful, visually strong deck and attach the requested files. This is an HTML-first skill: `slides.html` is the source of truth, `DESIGN.md` is the design brief, and the build script exports HTML, text-preserving PDF, image-backed PPTX, notes, and review evidence.

## Workflow

1. Decide the deck archetype and story before writing files.
2. Create `tmp/<deck-slug>/deck-brief.md` with request intent, audience, deck archetype, main thesis, story spine, slide sequence, visual direction, must-show content, and what would be too shallow.
3. Create `tmp/<deck-slug>/DESIGN.md`.
4. Create `tmp/<deck-slug>/slides.html`.
5. Run `/workspace/skills/presentation/scripts/build.sh` with `terminal.run` from `workingDirectoryPath: "tmp/<deck-slug>"`.
6. Inspect `build/review/slide-review.json`, `slide-review.md`, contact sheets, `fit-review.json`, and each `fit-review-XX.md`.
7. Use `artifact.review` when visual judgment is worth the budget. Include deck intent, `deck-brief.md`, deck archetype, contact sheet image, and expected visible text.
8. If deterministic review, rendered image evidence, or LLM notes show useful improvements, revise `slides.html` and rebuild. Repeat at most three times.
9. Deliver accepted outputs from `tmp/<deck-slug>/build/` plus requested source files with `file.deliver`. Use one call and a `files` array when delivering multiple files.

Use this build command shape:

```json
{
  "command": "NAME=<deck-slug> /workspace/skills/presentation/scripts/build.sh",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

For `html만`, use `FORMATS=html NAME=<deck-slug> /workspace/skills/presentation/scripts/build.sh`. For a normal full deck, omit `FORMATS`.

Deliver generated files such as `tmp/<deck-slug>/build/<deck-slug>.pptx` and requested source files with `file.deliver`. Do not use shell `cp`, do not deliver from a skill directory, and do not expose `/workspace`, `/tmp`, `file://`, or sandbox paths.

Do not look for a content generator or template deck. There is no template to fill in. The content, layout, and HTML source are your responsibility.

## Content Quality

The deck must be useful before it is beautiful. A good deck gives the audience a decision, explanation, lesson, or next action they did not already have.

Build the story spine before authoring:

- Situation: what the audience already knows or needs.
- Tension: what is confusing, risky, expensive, or unresolved.
- Thesis: the deck's answer in one sentence.
- Proof: examples, data, workflow, or reasoning.
- Close: decision, ask, next step, or lesson.

Reject shallow content: a welcome slide plus generic cards, repeated overview/features/benefits labels, claims without examples, vague business adjectives, empty mock data, or slides that only restate the prompt are not enough. When source material is thin, create realistic but clearly labeled assumptions or example data. Prefer concrete evidence, a worked example, scenario numbers, workflow states, or a recommendation.

Each slide needs a job: what the audience should learn, decide, or remember; what claim the title makes; what proof supports it; and what visual structure makes it easier to scan.

Pick one deck archetype: pitch, research report, executive briefing, education, portfolio, product proposal, or status report. Use that choice to decide information density, section sequence, fake data style, and ending.

Use these slide patterns as the default vocabulary: title thesis, section divider, comparison, matrix, timeline, evidence card, recommendation, and closing ask.

## Source Files

`deck-brief.md` is a brief planning note, not a second deliverable. It pins the audience, thesis, slide sequence, visual direction, and quality bar.

`DESIGN.md` is a design brief, not a theme file. Write it directly for the user's deck; do not expect `build.sh` to apply it automatically. It must be Stitch-compatible: YAML front matter with only `colors`, `typography`, and `layout`, followed by a short rationale.

Use this compact shape:

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

`slides.html` is the source of truth for the deck. Include `<!-- design-source: DESIGN.md -->`, mirror `DESIGN.md` colors/typography/layout in CSS, keep one main message per slide, and use one top-level `<section class="slide">` per slide.

Use browser-rendered HTML as the layout surface:

- Complete HTML document with `<style>` in the head.
- Canonical geometry: `.slide { width: 1600px; height: 900px; }`.
- `@page { size: 1600px 900px; margin: 0; }`.
- Fixed 16:9 frame with grid or flex layout.
- Fit-safe containers with `minmax(0, 1fr)`, `min-width: 0`, `min-height: 0`, and `overflow-wrap: anywhere`.
- No `overflow: hidden` on variable text containers unless cropped content is intentional.

Avoid bullet-only decks. Bullets may live inside cards, columns, matrix cells, timelines, or appendix blocks, but each slide needs visible structure. Read `assets/layouts.md` when choosing structures and `assets/minimal-design.md` when a sober presentation style is needed.

Use a task-local script for `DESIGN.md` and `slides.html`, then run it with `terminal.run`. Do not create source files with shell heredocs or `echo` inside the command string. Preserve explicit request constraints such as `할 수`, `역량`, `capability`, `what I can do`, `6장`, or `html만`.

## Fonts

Use fonts that render in HTML/PDF/PPTX. Paperlogy is the default display and body font. Finished standalone HTML must have local WOFF2 fallback or inlined WOFF2 data URLs created by the exporter; do not paste base64 font data into `slides.html`.

Default CSS stack:

```css
font-family: "Paperlogy", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji", sans-serif;
```

Do not use emoji as functional icons or bullets. Use text labels, CSS markers, inline SVG, or simple shapes.

## Review and Delivery

The full build creates PNGs, `slide-review.json`, `slide-review.md`, contact sheets, `fit-review.json`, and `fit-review-XX.md`. Check each contact sheet with its matching fit review. Every expected visible text item must appear fully inside the slide frame. Missing text, clipped text, hidden overflow, right-edge collision, or bottom-edge collision is revision input.

Review substance and surface: answer to the request, story flow, claim titles, credible examples, visual balance, readability, and fit. Do not accept a deck just because deterministic review passed.

Visual review notes are not a delivery blocker. If requested PPTX/PDF/HTML exists and is usable after the improvement budget, attach it and mention top remaining notes briefly. Do not spend delivery budget creating or attaching internal review-decision files unless the user asks.

Example delivery shape:

```json
{
  "files": [
    {"path": "artifacts/<deck-slug>/<deck-slug>.html", "contentType": "text/html"},
    {"path": "artifacts/<deck-slug>/<deck-slug>.pptx", "contentType": "application/vnd.openxmlformats-officedocument.presentationml.presentation"}
  ]
}
```

## Revisions and Formats

For revisions, edit the same `<deck-slug>`. If `tmp/<deck-slug>/slides.html` is gone, restore editable source from `artifacts/<deck-slug>/source/`, apply changes, rebuild, and deliver with `overwrite: true`.

If the user asks for `html만`, build and attach only HTML. Otherwise PPTX, PDF, HTML, and notes are a good default. Default PPTX is image-backed for visual fidelity; default PDF keeps selectable text.

## Validating an Existing PPTX

To check an existing `.pptx` file for empty slides, missing titles, excessive shape count, or leftover default fonts, run `scripts/validate_pptx.py` through `scripts/skill_runtime.py`:

```json
{
  "command": "python3 /workspace/skills/presentation/scripts/skill_runtime.py python /workspace/skills/presentation/scripts/validate_pptx.py <path-to-file>.pptx"
}
```

This validator reports structurally, not visually; it does not judge design quality. `skill_runtime.py` bootstraps `python-pptx` from `scripts/requirements.txt` on first use.
