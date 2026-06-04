---
name: pptx
description: Create, read, edit, combine, clean, and attach PowerPoint files. Use for existing .pptx files, direct PowerPoint object editing, templates, speaker notes, comments, editable-only decks, 발표자료, 프레젠테이션, 파워포인트, 피피티, or slide file requests. For new designed decks, prefer the simple-slides skill.
when_to_use: Use when the user asks for PowerPoint, .pptx, slide deck, presentation, deck editing, speaker notes, 발표자료, 프레젠테이션, 파워포인트, 피피티, or a PPTX deliverable.
allowed-tools:
  - file.preview
  - file.read
  - terminal.run
  - file.write
  - file.edit
  - file.patch
  - file.promote
  - file.attach
  - artifact.review
---

# PPTX Presentations

Create or modify PowerPoint `.pptx` files as local artifacts, then attach the final deck. This skill prioritizes editability and existing-file fidelity. For beautiful new presentation design, route to `simple-slides` first because it produces HTML/PDF/PPTX plus image review evidence.

Use this skill when the user provides an existing PPTX, asks for direct PowerPoint object editing, needs a specific `.pptx` template preserved, or explicitly asks for editable-only output. If the user asks for a new designed deck and does not require direct PowerPoint editing, use `simple-slides`.

## Workflow

1. Clarify only missing inputs that change the deck, such as audience, slide count, aspect ratio, source file, or required sections.
2. When the user asks to read, summarize, extract, OCR, or reuse content from an existing deck, call `file.preview` first; use `file.read` only for exact UTF-8 text ranges after previewing.
3. Create work under `tmp/<deck-slug>` relative to the default writable workspace directory; do not use Blueclaw internal temporary paths.
4. For straightforward direct-PPTX decks, write a JSON spec and run `scripts/create_pptx.py`.
5. For custom layouts, charts, notes, or edits that exceed the JSON script, write a task-local Python file and run it through `scripts/skill_runtime.py python <file.py>`.
6. Use ZIP/XML inspection only when the library cannot preserve or reach the needed feature.
7. Validate with `scripts/validate_pptx.py` or by reopening the generated deck and checking slide count, titles, text, images, and layout.
8. When layout fidelity matters and exported slide images are available, call `artifact.review` on the rendered images before attaching. Treat blocking issues as reasons to revise and regenerate.
9. Promote accepted final files from `tmp/<deck-slug>/build/` or the generated output path to `artifacts/<deck-slug>/` with `file.promote` unless the user requested a circle or shared destination, then attach the promoted `.pptx`; attach exported PDFs or images only if requested.

Bundled scripts are responsible for their own Python dependencies. Run them through `scripts/skill_runtime.py`; the wrapper selects the built-in dependency environment first and prepares requester-owned fallback storage with `uv` only when needed. `/workspace/shared/cache/dependencies` is only a package cache. Do not run `pip install` directly. Do not call runtime paths outside `/workspace` directly. Do not stop at a missing-library error; run the helper script first. Do not use `python - <<'PY'` or system Python snippets for code that needs the PowerPoint library.

For from-scratch presentation design, use the existing `simple-slides` skill unless the user specifically needs direct PowerPoint object editing, editable-only output, or preservation of an existing PPTX file.

For high-fidelity PPTX where only some text needs later editing, use a hybrid deck: render the static visual design as a full-slide image, then overlay only the required editable titles, labels, numbers, or body text as PowerPoint text boxes. Do not force decorative lines, cards, screenshots, charts, or complex HTML layouts into editable shapes when they are not meant to be changed.

## Helper Scripts

For normal decks, create a spec file:

```json
{
  "widthInches": 13.333,
  "heightInches": 7.5,
  "slides": [
    {
      "layout": "title",
      "title": "Presentation Title",
      "subtitle": "Subtitle"
    },
    {
      "layout": "cards",
      "title": "Main takeaway",
      "body": ["First point", "Second point"],
      "images": [
        {
          "path": "chart.png",
          "leftInches": 7,
          "topInches": 1.5,
          "widthInches": 5
        }
      ]
    },
    {
      "layout": "hybrid",
      "backgroundImage": "rendered-slide-03.png",
      "editableTexts": [
        {
          "text": "Editable decision headline",
          "leftInches": 0.8,
          "topInches": 0.7,
          "widthInches": 8.5,
          "heightInches": 0.7,
          "fontSize": 28,
          "weight": "bold"
        }
      ]
    }
  ]
}
```

Run:

```json
{
  "command": "python3 /workspace/skills/pptx/scripts/skill_runtime.py python /workspace/skills/pptx/scripts/create_pptx.py deck.json output.pptx && python3 /workspace/skills/pptx/scripts/skill_runtime.py python /workspace/skills/pptx/scripts/validate_pptx.py output.pptx",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

## Custom Python

For work that exceeds the JSON script, create a task-local Python file such as `custom_pptx.py`, then run it through the bundled runtime:

```json
{
  "command": "python3 /workspace/skills/pptx/scripts/skill_runtime.py python custom_pptx.py",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

Inside that file, import the PowerPoint library normally; the wrapper has already created and selected the venv.

Use Paperlogy as the default `fontName` when creating new objects. Font embedding depends on the recipient's PowerPoint environment, so treat PDF/HTML from `simple-slides` as the fidelity reference for new designed decks. For direct PPTX output, use a restrained palette, blank-layout slides, conclusion-first titles, and structured title/body/card/comparison/matrix/timeline layouts instead of bullet-only pages.

When preserving design matters more than object editability, prefer `layout: "hybrid"` with `backgroundImage` and `editableTexts`. The full-slide image protects spacing, gradients, icons, charts, and card geometry from PowerPoint/Keynote conversion drift. Keep editable overlays limited to the fields the user is likely to change.

## Editing Existing Files

For user-provided `.pptx` files:

1. Copy the input to a working directory.
2. Write a task-local edit script and run it through `scripts/skill_runtime.py python <file.py>`.
3. Preserve slide masters, layouts, theme colors, and existing images where possible.
4. Save to a new filename unless the user asks to replace the original.

Use direct XML inspection for notes, comments, relationships, or other features that the library does not expose. Put inspection code in a task-local script and run it with Python:

```bash
python3 inspect_package.py
```

Avoid XML rewrites unless required. If XML editing is required, preserve namespaces, relationships, and content types.

## Images

Add local image files with explicit dimensions from a script that runs through `scripts/skill_runtime.py`. Verify image paths exist before saving. Use high-resolution source images when the deck will be projected.

## Validation

Always run `scripts/validate_pptx.py` through `scripts/skill_runtime.py` after saving. Review warnings for empty slides, missing titles, excessive shape count, and lingering default Calibri/Aptos fonts.

When layout fidelity matters and LibreOffice is available, export to PDF or slide images and inspect before attaching. Pass exported slide images to `artifact.review` with the intended deck purpose and editable-field requirements. If LibreOffice is not available, rely on python-pptx validation and be clear that object-level validation was performed.
