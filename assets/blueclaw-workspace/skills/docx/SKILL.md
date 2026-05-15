---
name: docx
description: Create, read, edit, and attach Word documents in .docx format. Use for Word documents, reports, memos, letters, templates, tracked changes review, comments, document cleanup, .docx conversion, 워드, 문서, 보고서, 메모, 서식, or docx requests. Do not use for PDFs, spreadsheets, or slide decks unless the user also asks for a Word output.
when_to_use: Use when the user asks for Word, .docx, report, memo, letter, template, document formatting, document editing, 문서, 워드, 보고서, 메모, 서식, or a polished Word deliverable.
allowed-tools:
  - terminal.run
  - file.write
  - file.attach
---

# DOCX Documents

Create and modify Word documents as local `.docx` files, then attach the final file.

## Workflow

1. Clarify only missing requirements that change the document structure, such as audience, title, language, required sections, or source file.
2. Create work under `$BLUECLAW_TASK_TMP`; do not use Blueclaw internal temporary paths.
3. For straightforward new documents, write a JSON spec and run `scripts/create_docx.py`.
4. Use direct `python-docx` code for custom layouts or edits that exceed the script.
5. Use ZIP/XML inspection only when `python-docx` cannot preserve or reach the needed feature.
6. Validate with `scripts/validate_docx.py` or by reopening the `.docx` file and checking the expected paragraphs, tables, headings, and images.
7. Move accepted final files to `$BLUECLAW_REQUESTER_ARTIFACTS/<document-slug>/` unless the user requested a circle or shared destination, then attach the final `.docx`; attach intermediate files only if the user asks.

Bundled scripts are responsible for their own Python dependencies. They bootstrap `scripts/requirements.txt` into `$BLUECLAW_REQUESTER_TMP/.skill-env/docx` when needed and use `/workspace/shared/cache/dependencies` only as a package cache. Do not stop at `ModuleNotFoundError`; run the helper script first. For custom Python beyond the helper script, import `scripts/skill_runtime.py` and call `ensure_requirements("docx")` before importing `docx`.

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
  "command": "python3 /workspace/skills/docx/scripts/create_docx.py document.json output.docx && python3 /workspace/skills/docx/scripts/validate_docx.py output.docx",
  "workingDirectoryPath": "$BLUECLAW_TASK_TMP"
}
```

## Creation

Use `python-docx` for straightforward documents:

```python
from docx import Document
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.shared import Inches, Pt

document = Document()

section = document.sections[0]
section.top_margin = Inches(0.8)
section.right_margin = Inches(0.8)
section.bottom_margin = Inches(0.8)
section.left_margin = Inches(0.8)

styles = document.styles
styles["Normal"].font.name = "Arial"
styles["Normal"].font.size = Pt(10.5)

heading = document.add_heading("Report Title", level=1)
heading.alignment = WD_ALIGN_PARAGRAPH.CENTER

document.add_paragraph("Body text.")
document.save("output.docx")
```

Prefer clear headings, short paragraphs, and real Word tables over monospaced text. For Korean documents, use a Korean-capable font such as `Noto Sans CJK KR`, `NanumGothic`, `Malgun Gothic`, or the closest installed equivalent.

## Tables

Use Word tables for structured content:

```python
table = document.add_table(rows=1, cols=3)
table.style = "Table Grid"
headers = ["Item", "Owner", "Status"]
for cell, header in zip(table.rows[0].cells, headers):
    cell.text = header

for item, owner, status in rows:
    cells = table.add_row().cells
    cells[0].text = item
    cells[1].text = owner
    cells[2].text = status
```

Keep table text concise enough to fit the page. Use landscape orientation only when the content truly needs width.

## Editing Existing Files

For user-provided `.docx` files:

1. Copy the input to a working directory.
2. Open with `python-docx`.
3. Preserve existing styles when possible.
4. Make the smallest changes needed.
5. Save to a new filename unless the user asks to replace the original.

When the file contains tracked changes, comments, fields, complex headers, or custom XML, inspect the package structure before editing:

```bash
python - <<'PY'
from zipfile import ZipFile

with ZipFile("input.docx") as archive:
    for name in archive.namelist():
        if name.startswith("word/"):
            print(name)
PY
```

Avoid XML rewrites unless required. If XML editing is required, preserve namespaces, relationships, and content types.

## Validation

After saving, reopen the file:

```python
from docx import Document

document = Document("output.docx")
print(len(document.paragraphs), len(document.tables))
```

For important layout work, convert to PDF or images when the runtime has LibreOffice available, then inspect the result before attaching.
