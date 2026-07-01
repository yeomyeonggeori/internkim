---
name: pptx
description: Create, read, edit, combine, clean, and attach PowerPoint files. Use for existing .pptx files, direct PowerPoint object editing, templates, speaker notes, comments, editable-only decks, 발표자료, 프레젠테이션, 파워포인트, 피피티, or slide file requests. For new designed decks, prefer the simple-slides skill.
when_to_use: Use when the user asks for PowerPoint, .pptx, slide deck, presentation, deck editing, speaker notes, 발표자료, 프레젠테이션, 파워포인트, 피피티, or a PPTX deliverable.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# PPTX Presentations

Create or modify PowerPoint `.pptx` files as local artifacts, then attach the final deck. This skill prioritizes editability and existing-file fidelity. For beautiful new presentation design, route to `simple-slides` first because it produces HTML/PDF/PPTX plus image review evidence.

Use this skill when the user provides an existing PPTX, asks for direct PowerPoint object editing, needs a specific `.pptx` template preserved, or explicitly asks for editable-only output. If the user asks for a new designed deck and does not require direct PowerPoint editing, use `simple-slides`.

## Workflow

1. Clarify only missing inputs that change the deck, such as audience, slide count, aspect ratio, source file, or required sections.
2. If the user refers to a deck from an earlier task ("방금 만든", "그 파일", "the deck I just made"), the source is the workspace file in `~/documents/`, NOT the delivered attachment. Do NOT call `file.preview` or `file.read` on that attachment. For slide appends, run `scripts/edit_pptx.py` with no path; it targets the newest `.pptx` in `~/documents/` automatically (see Editing Existing Files). For other edits write a task-local Python file through `scripts/skill_runtime.py`. Save back to the SAME `~/documents/<name>.pptx` and re-deliver from there.
3. Only when the user uploaded a NEW file in THIS conversation and you need to read it (summarize, extract, OCR — not edit a binary you made), call `file.preview` first; use `file.read` only for exact UTF-8 text ranges after previewing.
4. Work directly in `~/documents` (run `mkdir -p ~/documents` once first). Build and deliver the deck there.
5. For straightforward direct-PPTX decks, run `scripts/create_pptx.py` with inline `--deck-title`/`--slide-title`/`--bullet` args. Pass `--spec <json_path>` for rich layouts, themes, or images.
6. For custom layouts, charts, notes, or edits that exceed the JSON script, write a task-local Python file and run it through `scripts/skill_runtime.py python <file.py>`.
7. Use ZIP/XML inspection only when the library cannot preserve or reach the needed feature.
8. Validate with `scripts/validate_pptx.py` or by reopening the generated deck and checking slide count, titles, text, images, and layout.
9. When layout fidelity matters and exported slide images are available, call `artifact.review` on the rendered images before attaching. Treat blocking issues as reasons to revise and regenerate.
10. Save the accepted final `.pptx` to `~/documents/<title>.pptx` (run `mkdir -p ~/documents` first) so it persists for later edit and delete tasks, then deliver it from there with `file.deliver`; deliver exported PDFs or images only if requested.

Bundled scripts are responsible for their own Python dependencies. Run them through `scripts/skill_runtime.py`; the wrapper selects the built-in dependency environment first and prepares requester-owned fallback storage with `uv` only when needed. `/workspace/shared/cache/dependencies` is only a package cache. Do not run `pip install` directly. Do not call runtime paths outside `/workspace` directly. Do not stop at a missing-library error; run the helper script first. Do not use `python - <<'PY'` or system Python snippets for code that needs the PowerPoint library.

For from-scratch presentation design, use the existing `simple-slides` skill unless the user specifically needs direct PowerPoint object editing, editable-only output, or preservation of an existing PPTX file.

For high-fidelity PPTX where only some text needs later editing, use a hybrid deck: render the static visual design as a full-slide image, then overlay only the required editable titles, labels, numbers, or body text as PowerPoint text boxes. Do not force decorative lines, cards, screenshots, charts, or complex HTML layouts into editable shapes when they are not meant to be changed.

## Saving and managing the document

Save the final deck to `~/documents/<title>.pptx` so it persists across tasks; run `mkdir -p ~/documents` once before writing there. `~` is the requester personal workspace and resolves the same way in a tool path field and in a shell command. To edit or delete a deck the user names in a later task, list `~/documents/` (`ls -t ~/documents`) to find the file, then edit it in place or remove it with `file.delete`.

## Creating a New Deck

For common decks, pass inline arguments — no spec file needed:

```json
{
  "command": "python3 /workspace/skills/pptx/scripts/skill_runtime.py python /workspace/skills/pptx/scripts/create_pptx.py ~/documents/<title>.pptx --deck-title \"Presentation Title\" --slide-title \"Key Findings\" --bullet \"Revenue up 12%\" --bullet \"Cost down 8%\" --slide-title \"Next Steps\" --bullet \"Launch Q3\" && python3 /workspace/skills/pptx/scripts/skill_runtime.py python /workspace/skills/pptx/scripts/validate_pptx.py ~/documents/<title>.pptx",
  "workingDirectoryPath": "~/documents"
}
```

`--deck-title` adds a styled title slide. Each `--slide-title` starts a new content slide; each `--bullet` attaches to the most recent `--slide-title`. Repeat `--slide-title`/`--bullet` pairs for multiple slides in one call. This is the symmetric pair to editing: create and edit share the same `--slide-title`/`--bullet` vocabulary.

For rich decks that need themes, custom layouts (cards, comparison, matrix, timeline), or images, pass `--spec <json_path>` instead:

```json
{
  "command": "python3 /workspace/skills/pptx/scripts/skill_runtime.py python /workspace/skills/pptx/scripts/create_pptx.py ~/documents/<title>.pptx --spec deck.json && python3 /workspace/skills/pptx/scripts/skill_runtime.py python /workspace/skills/pptx/scripts/validate_pptx.py ~/documents/<title>.pptx",
  "workingDirectoryPath": "~/documents"
}
```

The spec format: `{"slides": [{"layout": "title", "title": "...", "subtitle": "..."}, {"layout": "cards", "title": "...", "body": ["..."]}, {"layout": "hybrid", "backgroundImage": "...", "editableTexts": [{"text": "...", "leftInches": 0.8, "topInches": 0.7, "widthInches": 8.5, "heightInches": 0.7, "fontSize": 28, "weight": "bold"}]}]}`. Optional top-level keys: `widthInches` (default 13.333), `heightInches` (default 7.5), `style.fontName`, `style.colors`.

## Custom Python

For work that exceeds the JSON script, create a task-local Python file such as `custom_pptx.py`, then run it through the bundled runtime:

```json
{
  "command": "python3 /workspace/skills/pptx/scripts/skill_runtime.py python custom_pptx.py",
  "workingDirectoryPath": "~/documents"
}
```

Inside that file, import the PowerPoint library normally; the wrapper has already created and selected the venv.

Use Paperlogy as the default `fontName` when creating new objects. Font embedding depends on the recipient's PowerPoint environment, so treat PDF/HTML from `simple-slides` as the fidelity reference for new designed decks. For direct PPTX output, use a restrained palette, blank-layout slides, conclusion-first titles, and structured title/body/card/comparison/matrix/timeline layouts instead of bullet-only pages.

When preserving design matters more than object editability, prefer `layout: "hybrid"` with `backgroundImage` and `editableTexts`. The full-slide image protects spacing, gradients, icons, charts, and card geometry from PowerPoint/Keynote conversion drift. Keep editable overlays limited to the fields the user is likely to change.

## Editing Existing Files

To edit a deck you delivered in an earlier task, the source is the workspace file in `~/documents/` — not the delivered attachment.

1. For "the deck I just made" (most recently created .pptx), pass no path — the script targets the newest `.pptx` in `~/documents/` automatically. Only run `ls -t ~/documents` when the user names a specific older file and you need to confirm the exact filename.
2. To append one or more slides — the most common edit — run the bundled helper directly. Do NOT hand-write Python or use `python -c` for a simple append:

```json
{
  "command": "python3 /workspace/skills/pptx/scripts/skill_runtime.py python /workspace/skills/pptx/scripts/edit_pptx.py --slide-title \"Q3 Results\" --bullet \"Revenue up 12%\" --bullet \"Cost down 8%\"",
  "workingDirectoryPath": "~/documents"
}
```

For a specific older file named by the user, pass the path explicitly: `edit_pptx.py ~/documents/<name>.pptx --slide-title ...`

   Each `--slide-title` starts a new appended slide; each `--bullet` attaches to the most recent `--slide-title`. Repeat `--slide-title`/`--bullet` pairs to append multiple slides in one call. For a JSON batch, write `[{"title": "...", "bullets": ["..."]}]` to a file and pass it with `--slides <file.json>`.

3. For edits beyond appending — changing existing slide content, reordering slides, adding notes, modifying charts, or working with XML relationships — write a task-local Python script and run it through `scripts/skill_runtime.py python <file.py>`. `~` is NOT auto-expanded in Python; use `os.path.expanduser` to resolve the path. For features the library does not expose, inspect or rewrite ZIP/XML in the task-local script; preserve namespaces, relationships, and content types.
4. Preserve slide masters, layouts, theme colors, and existing images where possible.
5. Save back to the SAME `~/documents/<name>.pptx` (edit in place) so the next edit or delete task still finds it, then deliver from there. Save to a new filename only when the user attached a file in THIS conversation or explicitly wants both versions kept.

## Images

Add local image files with explicit dimensions from a script that runs through `scripts/skill_runtime.py`. Verify image paths exist before saving. Use high-resolution source images when the deck will be projected.

## Validation

Always run `scripts/validate_pptx.py` through `scripts/skill_runtime.py` after saving. Review warnings for empty slides, missing titles, excessive shape count, and lingering default Calibri/Aptos fonts.

When layout fidelity matters and LibreOffice is available, export to PDF or slide images and inspect before attaching. Pass exported slide images to `artifact.review` with the intended deck purpose and editable-field requirements. If LibreOffice is not available, rely on python-pptx validation and be clear that object-level validation was performed.
