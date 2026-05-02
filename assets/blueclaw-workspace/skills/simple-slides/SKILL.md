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
allowedProfiles: [default]
references:
  - references/design-system.md
  - references/layouts.md
scripts:
  - scripts/create_deck.py
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

Create the directory with `terminal.run`.

First, write a short `brief.md` into that directory with `file.write`. Include the user request, audience, desired tone, and slide topic.

Then use the bundled deck creator. This is the preferred path because it writes valid Marp source, copies runtime scripts, builds the deck, and keeps output names stable:

```json
{
  "command": "python3 /workspace/skills/simple-slides/scripts/create_deck.py --slug <deck-slug> --brief brief.md",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

If you need a more custom deck, write these files into that directory with `file.write`:

- `DESIGN.md`
- `presentation.md`

Copy the bundled scripts into the working directory with `terminal.run`:

```json
{
  "command": "cp /workspace/skills/simple-slides/assets/build.sh ./build.sh && cp /workspace/skills/simple-slides/scripts/extract_notes.py ./extract_notes.py && chmod +x ./build.sh ./extract_notes.py",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

Use this skill's `assets/design.md` and `assets/template.md` as source material for `DESIGN.md` and `presentation.md`. Do not rewrite or simplify `build.sh`.

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

If the build fails, fix the source and rerun it. Do not attach stale outputs.

## Visual Review

Render PNGs with Marp and inspect edited slides before shipping when time allows:

```json
{
  "command": "marp presentation.md --images png --allow-local-files -o <name>.png",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

Check for clipping, overlap, unreadable text, missing images, and broken fonts. Korean/CJK decks need a font that renders Korean.

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
