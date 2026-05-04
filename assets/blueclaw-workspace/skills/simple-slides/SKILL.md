---
name: simple-slides
description: Generate clean presentation slides with Marp and attach the requested files. Use for slides, slide decks, presentations, pitch decks, research summaries, stakeholder reports, PPTX, PowerPoint, Google Slides, Keynote, 발표자료, 파워포인트, 피피티.
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

Create a useful slide deck and attach the requested files. This is a skill, not a mandatory app contract: use the workflow and helper scripts when they help, and keep going when a simpler Marp source is enough.

## Workflow

1. Create a fresh workspace directory such as `/workspace/.blueclaw/tmp/<deck-slug>`.
2. Write the deck source files with `file.write`. A good default is `brief.md`, `DESIGN.md`, and `presentation.md`.
3. Build with Marp through `scripts/create_deck.py` or the bundled `assets/build.sh`.
4. Review the rendered result when possible, especially the contact sheet and `review/slide-review.json`.
5. Attach only the files the user asked for. If the user says `html만`, attach the HTML file only. If they do not restrict formats, PPTX, PDF, HTML, and notes are a good default set.

Do not say PPTX or file delivery is impossible when the local tools are available. If the result is imperfect but usable, attach it and be clear about any limitation in the final reply.

## Source Files

`brief.md` is useful for preserving the user's original request, topic, audience, requested slide count, requested formats, tone, and constraints. Keep the original wording when it affects meaning, such as `할 수`, `역량`, `capability`, `what I can do`, `6장`, or `html만`.

`DESIGN.md` is a design guide for the deck. Prefer the bundled `assets/design.md` and the references in `references/` when you need a stronger visual system.

`presentation.md` is the Marp source. Keep one main message per slide, short titles, scannable bullets, speaker notes in HTML comments, and no trailing slide separator.

## Helper Script

`scripts/create_deck.py` is a convenience builder. It copies the runtime scripts, can turn a `deck_spec` in `brief.md` into `presentation.md`, and then runs the Marp build. It should help produce files, not prevent delivery because optional metadata is missing.

Use it like this from the deck directory:

```json
{
  "command": "python3 /workspace/skills/simple-slides/scripts/create_deck.py --slug <deck-slug> --brief brief.md",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

If the helper is not useful, copy the bundled scripts and run the build directly:

```json
{
  "command": "cp /workspace/skills/simple-slides/assets/build.sh ./build.sh && cp /workspace/skills/simple-slides/scripts/extract_notes.py ./extract_notes.py && cp /workspace/skills/simple-slides/scripts/render_review.py ./render_review.py && chmod +x ./build.sh ./extract_notes.py ./render_review.py && NAME=<deck-slug> ./build.sh",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

## Output Expectations

Good decks preserve the user's request, satisfy explicit format and slide-count requests when practical, apply a coherent design, and attach generated files instead of exposing paths.

Before the final reply, use your own task-specific quality judgment. Mention only attached filenames or plain descriptions. Never expose `sandbox:/mnt/data`, `file://`, `/workspace`, `/tmp`, or other local paths to the user.
