---
name: pptx
description: Create, read, edit, combine, clean, and attach PowerPoint files. Use for .pptx files, PowerPoint decks, slide decks, presentations, templates, speaker notes, comments, 발표자료, 프레젠테이션, 파워포인트, 피피티, or slide file requests. For new designed decks that also need HTML/PDF output, prefer the simple-slides skill.
when_to_use: Use when the user asks for PowerPoint, .pptx, slide deck, presentation, deck editing, speaker notes, 발표자료, 프레젠테이션, 파워포인트, 피피티, or a PPTX deliverable.
allowed-tools:
  - terminal.run
  - file.write
  - file.attach
---

# PPTX Presentations

Create or modify PowerPoint `.pptx` files as local artifacts, then attach the final deck.

## Workflow

1. Clarify only missing inputs that change the deck, such as audience, slide count, aspect ratio, source file, or required sections.
2. Create work under `tmp/<deck-slug>` relative to the default writable workspace directory; do not use Blueclaw internal temporary paths.
3. For straightforward new decks, write a JSON spec and run `scripts/create_pptx.py`.
4. For custom layouts, charts, notes, or edits that exceed the JSON script, write a task-local Python file and run it through `scripts/skill_runtime.py python <file.py>`.
5. Use ZIP/XML inspection only when the library cannot preserve or reach the needed feature.
6. Validate with `scripts/validate_pptx.py` or by reopening the generated deck and checking slide count, titles, text, images, and layout.
7. Move accepted final files to `artifacts/<deck-slug>/` unless the user requested a circle or shared destination, then attach the final `.pptx`; attach exported PDFs or images only if requested.

Bundled scripts are responsible for their own Python dependencies. Run them through `scripts/skill_runtime.py`; the wrapper selects the built-in dependency environment first and prepares requester-owned fallback storage with `uv` only when needed. `/workspace/shared/cache/dependencies` is only a package cache. Do not run `pip install` directly. Do not call runtime paths outside `/workspace` directly. Do not stop at `ModuleNotFoundError`; run the helper script first. Do not use `python - <<'PY'` or system Python snippets for code that needs the PowerPoint library.

For from-scratch presentation design with HTML/PDF/PPTX outputs, use the existing `simple-slides` skill unless the user specifically needs direct PowerPoint object editing.

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

Use one clear message per slide. Prefer structured layouts, readable type, and generous margins over dense bullet lists.

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

Always run `scripts/validate_pptx.py` through `scripts/skill_runtime.py` after saving.

When layout fidelity matters and LibreOffice is available, export to PDF or slide images and inspect before attaching.
