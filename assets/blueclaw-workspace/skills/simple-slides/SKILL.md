---
name: simple-slides
description: Generate clean presentation decks with Marp and return PPTX/PDF/HTML attachments. Use for slides, slide decks, presentations, pitch decks, research summaries, stakeholder reports, PPTX, PowerPoint, Google Slides, Keynote, 발표자료, 파워포인트, 피피티.
category: document-generation
tags: [slides, pptx, marp, reporting]
triggerHints:
  - slides
  - slide deck
  - presentation
  - pitch deck
  - deck
  - ppt
  - pptx
  - powerpoint
  - keynote
  - 슬라이드
  - 발표
  - 발표자료
  - 프레젠테이션
  - 프리젠테이션
  - 파워포인트
  - 피피티
activation:
  keywords:
    - slides
    - slide deck
    - presentation
    - pitch deck
    - deck
    - ppt
    - pptx
    - powerpoint
    - keynote
    - 슬라이드
    - 발표
    - 발표자료
    - 프레젠테이션
    - 프리젠테이션
    - 파워포인트
    - 피피티
requiredTools:
  - terminal.run
  - file.write
  - file.attach
completion:
  requiredEvidenceTools:
    - file.attach
  requiredAttachmentSuffixes:
    - .pptx
    - .pdf
    - .html
    - -notes.txt
quality:
  recommendedChecks:
    - marp_build_log_success
    - parse_pptx
    - pdf_page_count
    - render_nonblank
    - slide_render_images
    - max_text_overflow
    - slide_count_min
    - forbidden_reply_fragments
allowedProfiles: [default]
references:
  - references/design-system.md
  - references/layouts.md
scripts:
  - scripts/create_deck.py
  - scripts/extract_notes.py
  - scripts/render_review.py
assets:
  - assets/build.sh
  - assets/design.md
  - assets/template.md
---

# Simple Slides

Create a complete deck and attach the generated files. Do not claim PPTX is impossible: this runtime has Marp and file attachments.

## Runtime Contract

Use only these built-in tools:

- `file.write` to create files under `/workspace`.
- `terminal.run` to run guarded commands.
- `file.attach` to attach finished artifacts to the final reply.

Do not use legacy tool aliases.

## Working Directory

Create a fresh directory under:

```text
/workspace/.blueclaw/tmp/<deck-slug>
```

Create the directory with `terminal.run`.

First, write a short `brief.md` into that directory with `file.write`. Include the user request, audience, desired tone, and slide topic.

Then use the bundled deck creator. For this bundled InternKim skill, this is the canonical path, not a suggestion. The creator writes a Stitch-compatible `DESIGN.md`, reads that design contract back, generates `presentation.md` from those tokens, copies runtime scripts, builds the deck, and keeps output names stable:

```json
{
  "command": "python3 /workspace/skills/simple-slides/scripts/create_deck.py --slug <deck-slug> --brief brief.md",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

Do not hand-write a full `presentation.md` before running the creator. If the user asks for a visual redesign or content revision after the first build, edit `DESIGN.md` first, then update `presentation.md` to match that design contract before rebuilding.

For custom edits after the creator has produced the initial files, update these files with `file.write`:

- `DESIGN.md`
- `presentation.md`

If the creator was not used and the files are being assembled manually, copy the bundled scripts into the working directory with `terminal.run`:

```json
{
  "command": "cp /workspace/skills/simple-slides/assets/build.sh ./build.sh && cp /workspace/skills/simple-slides/scripts/extract_notes.py ./extract_notes.py && cp /workspace/skills/simple-slides/scripts/render_review.py ./render_review.py && chmod +x ./build.sh ./extract_notes.py ./render_review.py",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

Use this skill's `assets/design.md` and `assets/template.md` as source material for `DESIGN.md` and `presentation.md`. Do not rewrite or simplify `build.sh`.

## Design First

`DESIGN.md` is the source of truth for visual decisions. It must be Stitch-compatible: YAML token front matter followed by concise design rationale. Adapt the design to the topic and audience with concrete choices for:

- colors
- typefaces
- spacing
- slide density
- chart/table/card treatment

Then port those choices into the Marp CSS in `presentation.md`. The final `presentation.md` should contain `<!-- design-source: DESIGN.md -->` near the top.

## Slide Rules

- One message per slide.
- Short titles and short lines.
- Prefer structured cards, grids, numbers, and visual hierarchy over long prose. Use tables only when comparison is the actual point of the slide.
- Use local image files only. Download remote images into the deck directory first.
- Do not end `presentation.md` with a trailing slide separator.
- Put speaker notes in HTML comments.

## Build

Run the build from the deck directory with a stable output name:

```json
{
  "command": "NAME=<deck-slug> ./build.sh",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

The build must produce:

- `<deck-slug>.html`
- `<deck-slug>.pptx`
- `<deck-slug>.pdf`
- `<deck-slug>-notes.txt`
- `review/<deck-slug>*.png`
- `review/contact-sheet-*.png`
- `review/slide-review.json`

If the build fails, fix the source and rerun it. Do not attach stale outputs.

## Visual Review

The build renders per-slide PNGs and contact-sheet collages into `review/`. Use `review/slide-review.json` as the machine-readable check for nonblank slides, safe margins, and edge overflow. Use the contact sheets when a human or model needs a compact visual pass without loading every individual slide image.

Check for clipping, overlap, unreadable text, missing images, and broken fonts. Korean/CJK decks should use the declared font stack: Paperlogy for display, Freesentation for body, then Pretendard or Noto Sans KR as fallback.

## Final Reply

Attach these exact generated files with one `file.attach` call before final reply:

```json
{
  "paths": [
    "/workspace/.blueclaw/tmp/<deck-slug>/<deck-slug>.pptx",
    "/workspace/.blueclaw/tmp/<deck-slug>/<deck-slug>.pdf",
    "/workspace/.blueclaw/tmp/<deck-slug>/<deck-slug>.html",
    "/workspace/.blueclaw/tmp/<deck-slug>/<deck-slug>-notes.txt"
  ]
}
```

Google Workspace export/upload is disabled for now; do not call `google.*` tools and do not block local PPTX delivery on Google credentials. Only cite those generated artifact attachments as completion evidence. Do not cite source files such as `DESIGN.md`, `presentation.md`, `build.sh`, or `extract_notes.py`.
