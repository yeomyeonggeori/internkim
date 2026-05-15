---
name: docx
description: Create, read, edit, and attach Word documents in .docx format. Use for Word documents, reports, memos, letters, templates, tracked changes review, comments, document cleanup, .docx conversion, 워드, 문서, 보고서, 메모, 서식, or docx requests. Do not use for PDFs, spreadsheets, or slide decks unless the user also asks for a Word output.
when_to_use: Use when the user asks for Word, .docx, report, memo, letter, template, document formatting, document editing, 문서, 워드, 보고서, 메모, 서식, or a polished Word deliverable.
allowed-tools:
  - file.read
  - terminal.run
  - file.write
  - file.attach
---

# DOCX Documents

Create and modify Word documents as local `.docx` files, then attach the final file.

## Workflow

1. Clarify only missing requirements that change the document structure, such as audience, title, language, required sections, or source file.
2. When the user asks to read, summarize, extract, OCR, or reuse content from an existing file, call `file.read` first.
3. Create work under `tmp/<document-slug>` relative to the default writable workspace directory; do not use Blueclaw internal temporary paths.
4. For straightforward new documents, write a JSON spec and run `scripts/create_docx.py`.
5. For custom layouts or edits that exceed the JSON script, write a task-local Python file and run it through `scripts/skill_runtime.py python <file.py>`.
6. Use ZIP/XML inspection only when the library cannot preserve or reach the needed feature.
7. Validate with `scripts/validate_docx.py` or by reopening the `.docx` file and checking the expected paragraphs, tables, headings, and images.
8. Move accepted final files to `artifacts/<document-slug>/` unless the user requested a circle or shared destination, then attach the final `.docx`; attach intermediate files only if the user asks.

Bundled scripts are responsible for their own Python dependencies. Run them through `scripts/skill_runtime.py`; the wrapper selects the built-in dependency environment first and prepares requester-owned fallback storage with `uv` only when needed. `/workspace/shared/cache/dependencies` is only a package cache. Do not run `pip install` directly. Do not call runtime paths outside `/workspace` directly. Do not stop at `ModuleNotFoundError`; run the helper script first. Do not use `python - <<'PY'` or system Python snippets for code that needs the Word document library.

## Helper Scripts

For normal documents, create a spec file:

```json
{
  "title": "Report Title",
  "fontName": "Arial",
  "fontSize": 10.5,
  "page": {
    "marginInches": 0.8
  },
  "blocks": [
    {
      "type": "heading",
      "level": 1,
      "text": "Executive Summary"
    },
    {
      "type": "paragraph",
      "text": "Body text."
    },
    {
      "type": "bullets",
      "items": ["First point", "Second point"]
    },
    {
      "type": "table",
      "rows": [
        ["Item", "Owner", "Status"],
        ["Launch", "Kim", "Ready"]
      ]
    }
  ]
}
```

Run:

```json
{
  "command": "python3 /workspace/skills/docx/scripts/skill_runtime.py python /workspace/skills/docx/scripts/create_docx.py document.json output.docx && python3 /workspace/skills/docx/scripts/skill_runtime.py python /workspace/skills/docx/scripts/validate_docx.py output.docx",
  "workingDirectoryPath": "tmp/<document-slug>"
}
```

## Custom Python

For work that exceeds the JSON script, create a task-local Python file such as `custom_docx.py`, then run it through the bundled runtime:

```json
{
  "command": "python3 /workspace/skills/docx/scripts/skill_runtime.py python custom_docx.py",
  "workingDirectoryPath": "tmp/<document-slug>"
}
```

Inside that file, import the Word document library normally; the wrapper has already created and selected the venv.

Prefer clear headings, short paragraphs, and real Word tables over monospaced text. For Korean documents, use a Korean-capable font such as `Noto Sans CJK KR`, `NanumGothic`, `Malgun Gothic`, or the closest installed equivalent.

## Tables

Use Word tables for structured content. Keep table text concise enough to fit the page. Use landscape orientation only when the content truly needs width.

## Editing Existing Files

For user-provided `.docx` files:

1. Copy the input to a working directory.
2. Write a task-local edit script and run it through `scripts/skill_runtime.py python <file.py>`.
3. Preserve existing styles when possible.
4. Make the smallest changes needed.
5. Save to a new filename unless the user asks to replace the original.

When the file contains tracked changes, comments, fields, complex headers, or custom XML, inspect the package structure before editing:

```bash
python3 inspect_package.py
```

Avoid XML rewrites unless required. If XML editing is required, preserve namespaces, relationships, and content types.

## Validation

After saving, run `scripts/validate_docx.py` through `scripts/skill_runtime.py`.

For important layout work, convert to PDF or images when the runtime has LibreOffice available, then inspect the result before attaching.
