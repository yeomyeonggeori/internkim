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
quality:
  acceptanceGuidance:
    - Preserve the user's original request verbatim in brief.md as original_user_request.
    - Write an explicit deck_spec with the exact slide list before running the renderer.
    - Reflect explicit output constraints such as requested slide count, file formats, audience, and tone.
    - Verify generated artifacts against DESIGN.md and the rendered review evidence before attaching them.
    - Do not complete if requested formats, deck intent, or the requested slide count are missing.
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

Create a complete deck and attach the generated files. Do not claim PPTX is impossible: this runtime has Marp and file attachments. Simple Slides is a local skill made of workspace scripts, not an external service integration.

## Runtime Contract

Use only these built-in tools:

- `file.write` to create files under `/workspace`.
- `terminal.run` to run guarded commands.
- `file.attach` to attach finished artifacts to the final reply.

Do not use legacy tool aliases. Do not run `file.write`, `file.attach`, or any other Blueclaw tool name inside a `terminal.run` shell command. They are actions, not executables. If you need to write `brief.md`, `DESIGN.md`, or `presentation.md`, call `file.write` directly once per file, then call `terminal.run` only for real shell commands such as `mkdir`, `python3`, `cp`, `chmod`, or `./build.sh`.

## Working Directory

Create a fresh directory under:

```text
/workspace/.blueclaw/tmp/<deck-slug>
```

Pick one `<deck-slug>` before creating files and use only that directory for the whole task. Do not create alternate directories such as `-v2`, `-final`, `-new`, or `-analysis` after choosing the slug. Do not delete and recreate the directory during the same run; fix the files in place with `file.write`.

Create the directory once with `terminal.run`.

First, write a short `brief.md` into that directory with `file.write`. Include `original_user_request` with the user's request copied verbatim. Also include `topic`, `slide_intent`, `requested_slide_count`, `requested_formats`, and `output_slug`. Do not translate, summarize, or normalize away words like `할 수`, `역량`, `capability`, `what I can do`, or `8장`; the creator uses those signals to choose the deck contract.

For normal decks, include a fenced JSON `deck_spec` with the exact slide list. Each slide must include `title`, `body`, and `speaker_note`; `layout` is optional. The renderer does not invent generic slides for you. If the user asks for "Hermes Agent 장단점 6장", write six Hermes-specific slides in `deck_spec.slides`. Do not run the creator with a missing or placeholder deck spec.

Example:

````markdown
original_user_request: hermes agent의 장단점에 대해 분석한 ppt를 6장으로 만들어서 보내줘. html만 주면 돼
topic: Hermes Agent
slide_intent: 장단점 분석
requested_slide_count: 6
requested_formats: html
output_slug: hermes-analysis
deck_spec:
```json
{
  "title": "Hermes Agent 장단점 분석",
  "slides": [
    {
      "title": "Hermes Agent 판단 프레임",
      "body": ["Hermes Agent의 강점은 도구 실행과 장기 작업 흐름에 있다.", "약점은 환경 계약이 느슨하면 산출물 검증이 흔들릴 수 있다는 점이다."],
      "speaker_note": "Hermes Agent를 단순 채팅 모델이 아니라 실행형 에이전트로 놓고 장단점을 판단합니다."
    }
  ]
}
```
````

Then write `DESIGN.md` and `presentation.md` with `file.write` before running any other terminal command. `DESIGN.md` is the design contract; `presentation.md` is the Marp source. The script below is not a content creator. It is the only build command for normal deck creation: it validates `brief.md`, `DESIGN.md`, and `presentation.md`, writes `<deck-slug>-intent.json`, copies runtime scripts, builds the deck, and keeps output names stable:

```json
{
  "command": "python3 /workspace/skills/simple-slides/scripts/create_deck.py --slug <deck-slug> --brief brief.md",
  "workingDirectoryPath": "/workspace/.blueclaw/tmp/<deck-slug>"
}
```

Do not ask the script to invent slide content. Do not copy `build.sh`, `extract_notes.py`, or `render_review.py` yourself for normal deck creation, and do not run `./build.sh` directly before `create_deck.py` has succeeded. If the script fails because `deck_spec`, `DESIGN.md`, or `presentation.md` is missing or generic, fix those files in the same directory and rerun the same `create_deck.py` command. If the user asks for a visual redesign or content revision after the first build, edit `DESIGN.md` first, then update `presentation.md` to match that design contract before rebuilding.

Golden path:

1. `terminal.run` creates `/workspace/.blueclaw/tmp/<deck-slug>`.
2. `file.write` writes `/workspace/.blueclaw/tmp/<deck-slug>/brief.md`.
3. `file.write` writes `/workspace/.blueclaw/tmp/<deck-slug>/DESIGN.md`.
4. `file.write` writes `/workspace/.blueclaw/tmp/<deck-slug>/presentation.md`.
5. `terminal.run` runs `python3 /workspace/skills/simple-slides/scripts/create_deck.py --slug <deck-slug> --brief brief.md` from the deck directory. This is the only build command.
6. `file.attach` attaches only the requested generated artifact files.

Never replace steps 2-4 with shell redirection, heredocs, `echo`, `cat`, or a fake `file.write` command inside `terminal.run`; that loses tool evidence and often breaks quoting for Korean text, JSON, and CSS. Never change slugs or directories as a recovery strategy. Repeated directory setup is a sign to stop and fix the current files.

For custom edits after the first build, update these files with `file.write`:

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

The build normally produces:

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

Before final reply, declare task-specific quality criteria with `set_quality_criteria`. Include criteria for preserving the original request, satisfying explicit slide count and requested file formats, applying `DESIGN.md` to the final artifacts, and attaching only the requested generated artifacts with evidence. In `final_reply`, pass each criterion with evidence from successful terminal/file observations. Do not cite source files such as `DESIGN.md`, `presentation.md`, `build.sh`, `extract_notes.py`, or `<deck-slug>-intent.json` as completion artifacts.

## Final Reply

Attach the requested generated files with one `file.attach` call before final reply. If the user did not restrict formats, attach the full set:

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

If the user explicitly asks for one format such as `html만`, attach only that requested output:

```json
{
  "paths": [
    "/workspace/.blueclaw/tmp/<deck-slug>/<deck-slug>.html"
  ]
}
```

Google Workspace export/upload is disabled for now; do not call `google.*` tools and do not block local file delivery on Google credentials. Only cite generated artifact attachments as completion evidence.
