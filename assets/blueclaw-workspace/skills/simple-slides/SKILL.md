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
  - google slides
  - 구글 슬라이드
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
    - google slides
    - 구글 슬라이드
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
allowedProfiles: [default]
references:
  - references/design-system.md
  - references/layouts.md
scripts:
  - scripts/extract_notes.py
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

Write these files into that directory with `file.write`:

- `DESIGN.md`
- `presentation.md`
- `build.sh`
- `extract_notes.py`

Use this skill's `assets/design.md`, `assets/template.md`, `assets/build.sh`, and `scripts/extract_notes.py` as source material, but create the working copies with `file.write`.

## Design First

Write `DESIGN.md` before `presentation.md`. Adapt the design to the topic and audience with concrete choices for:

- colors
- typefaces
- spacing
- slide density
- chart/table/card treatment

Then port those choices into the Marp CSS in `presentation.md`.

## Slide Rules

- One message per slide.
- Short titles and short lines.
- Prefer tables, charts, and structured cards over long prose.
- Use local image files only. Download remote images into the deck directory first.
- Do not end `presentation.md` with a trailing slide separator.
- Put speaker notes in HTML comments.

## Build

Run the build from the deck directory:

```json
{
  "command": "./build.sh",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

The build must produce:

- `<name>.html`
- `<name>.pptx`
- `<name>.pdf`
- `<name>-notes.txt`

If the build fails, fix the source and rerun it. Do not attach stale outputs.

## Visual Review

Render PNGs with Marp and inspect edited slides before shipping:

```json
{
  "command": "marp presentation.md --images png --allow-local-files -o <name>.png",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

Check for clipping, overlap, unreadable text, missing images, and broken fonts. Korean/CJK decks need a font that renders Korean.

## Final Reply

Attach the generated PPTX, PDF, HTML, and notes with `file.attach`. If the user explicitly asks for Google Slides, call the typed Google Workspace import tool when available and still attach the local PPTX. Do not confuse a Google Slides URL with a local PPTX attachment.
