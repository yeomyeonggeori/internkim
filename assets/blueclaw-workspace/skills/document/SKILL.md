---
name: document
description: Create, read, edit, and attach Word documents in .docx format. Use for Word documents, reports, memos, letters, templates, tracked changes review, comments, document cleanup, .docx conversion, 워드, 문서, 보고서, 메모, 서식, or docx requests. Do not use for PDFs, spreadsheets, or slide decks unless the user also asks for a Word output. For standardized company forms and contracts (견적서, 품의서, 증명서, 근로계약서, NDA, quotation, invoice, certificate, contract) follow the paperwork skill, which reuses these scripts with its own document specs.
when_to_use: Use when the user asks for Word, .docx, report, memo, letter, template, document formatting, document editing, 문서, 워드, 보고서, 메모, 서식, or a polished Word deliverable.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# DOCX Documents

Create and modify Word documents as local `.docx` files, then attach the final file.

## Workflow

1. Clarify only when the document cannot be created safely without user input, such as a missing source file for an edit/extraction task, an unknown recipient for a legal/business letter, or a required approval. For open-ended new reports, guides, memos, or templates, choose the title, audience framing, sections, and structure yourself from the user's intent. Treat requests such as "use your judgment", "알아서 해", "파일만 줘", or "잘 만들어줘" as permission to proceed, not as a reason to ask for title or section choices.
2. If the user refers to a document from an earlier task ("방금 만든", "그 문서", "the doc I just made"), the source is the workspace file in `~/documents/`, NOT the delivered attachment. Do NOT call `file.preview` or `file.read` on that attachment. When a `content.md` exists next to the `.docx`, edit the markdown and re-export; otherwise append with `scripts/edit_docx.py` — for "the doc I just made", pass no path and the script targets the newest `.docx` automatically — save in place, and re-deliver from there.
3. Only when the user uploaded a NEW file in THIS conversation and you need to read it (summarize, extract, OCR — not edit a binary you made), call `file.preview` first; use `file.read` only for exact UTF-8 text ranges after previewing.
4. Work directly in `~/documents` (run `mkdir -p ~/documents` once first). Build, validate, and deliver the document there.
5. Content-first documents (reports, memos, minutes, guides — most requests) are markdown-first: write the full document as `~/documents/<title>/content.md`, revise it there until the content is right, then render with `scripts/export_document.py ~/documents/<title>/content.md --output ~/documents/<title>.docx` (headings #–####, **bold**, *italic*, `code`, bullet/numbered lists, pipe tables, > quotes all carry over; `--font`/`--font-size` set the base typeface). The markdown stays as the editable source of truth for follow-up changes.
6. Only when the layout itself is the point (custom margins, orientation, column widths, precise visual structure) use `scripts/create_docx.py` with inline arguments or `--spec <spec.json>`.
7. For edits that go beyond appending (a rewrite, deletion, or style change), write a task-local Python file and run it through `scripts/skill_runtime.py python <file.py>`.
8. Use ZIP/XML inspection only when the library cannot preserve or reach the needed feature.
9. Validate with `scripts/validate_docx.py` or by reopening the `.docx` file and checking the expected paragraphs, tables, headings, images, source facts, fonts, margins, and line spacing. Pass source-provided names, dates, totals, and key labels as repeated `--required-text` arguments. Pass likely invented or explicitly disallowed claims as repeated `--forbidden-text` arguments. Treat validation warnings as revision input; fix missing source text, forbidden unsupported text, empty visible text, very wide tables, empty table cells, dense table cells, missing Korean-capable fonts, odd margins, and unreadable line spacing before attaching unless the source truly requires them.
10. Save the accepted final `.docx` to `~/documents/<title>.docx` (run `mkdir -p ~/documents` first) so it persists for later edit and delete tasks, then deliver it from there with `file.deliver`; deliver intermediate files only if the user asks.

Bundled scripts are responsible for their own Python dependencies. Run them through `scripts/skill_runtime.py`; the wrapper selects the built-in dependency environment first and prepares requester-owned fallback storage with `uv` only when needed. `/workspace/shared/cache/dependencies` is only a package cache. Do not run `pip install` directly. Do not call runtime paths outside `/workspace` directly. Do not stop at a missing-library error; run the helper script first. Do not use `python - <<'PY'` or system Python snippets for code that needs the Word document library.

Treat supplied files and pasted source data as the source of truth. Preserve source-provided company names, product names, people, dates, amounts, IDs, and units exactly unless the user asks for translation or normalization. Do not invent missing customers, vendors, prices, totals, dates, contact details, or external background. Put the source-provided title, organization, period, and key metrics in the document body, not only in the filename or final reply. When a useful field is missing, write the user's-language equivalent of "Not provided" instead of filling a plausible value.


## Saving and managing the document

Save the final Word document to `~/documents/<title>.docx` so it persists across tasks; run `mkdir -p ~/documents` once before writing there. `~` is the requester personal workspace and resolves the same way in a tool path field and in a shell command. To edit or delete a document the user names in a later task, list `~/documents/` (`ls -t ~/documents`) to find the file, then edit it in place or remove it with `file.delete`.

## Creating Documents

For common documents — headings, paragraphs, bullet lists — pass content directly as arguments. No spec file needed:

```json
{
  "command": "python3 /workspace/skills/document/scripts/skill_runtime.py python /workspace/skills/document/scripts/create_docx.py ~/documents/<title>.docx --title \"<title>\" --heading \"안건\" --bullet \"첫 번째 항목\" --bullet \"두 번째 항목\"",
  "workingDirectoryPath": "~/documents"
}
```

Repeat `--heading`, `--paragraph`, `--bullet`, and `--table <path to JSON rows file>` as needed; each block appears in the order its flag was given on the command line, and consecutive `--bullet` flags merge into one bulleted list block. `--table` accepts a JSON file of row arrays, an array of row objects (first object's keys become the header), or `{"rows":[...],"columnWidthsInches":[...]}`.

For rich structure — tables, custom fonts, page margins, landscape orientation — write a spec file and pass it with `--spec`:

```json
{
  "title": "Report Title",
  "fontName": "Arial",
  "fontSize": 10.5,
  "page": { "marginInches": 0.8 },
  "blocks": [
    { "type": "heading", "level": 1, "text": "Executive Summary" },
    { "type": "paragraph", "text": "Body text." },
    { "type": "bullets", "items": ["First point", "Second point"] },
    {
      "type": "table",
      "columnWidthsInches": [2.2, 1.8, 2.6],
      "rows": [
        ["Item", "Owner", "Status"],
        ["Launch", "Kim", "Ready"]
      ]
    }
  ]
}
```

```json
{
  "command": "python3 /workspace/skills/document/scripts/skill_runtime.py python /workspace/skills/document/scripts/create_docx.py ~/documents/<title>.docx --spec document.json",
  "workingDirectoryPath": "~/documents"
}
```

## Editing Existing Files

To edit a document you delivered in an earlier task, the source is the workspace file in `~/documents/` — not the delivered attachment.

1. For "the doc I just made" (the most recently created .docx), pass no path — the script targets the newest `.docx` in `~/documents/` automatically. Only run `ls -t ~/documents` when the user names a specific older file and you need to confirm the exact filename.
2. For the common case — append a heading, paragraphs, or bullet items — run the deterministic editor. Do NOT hand-write Python or use `python -c`:

```json
{
  "command": "python3 /workspace/skills/document/scripts/skill_runtime.py python /workspace/skills/document/scripts/edit_docx.py --heading \"결정사항\" --bullet \"first decision\" --bullet \"second decision\"",
  "workingDirectoryPath": "~/documents"
}
```

For a specific older file named by the user, pass the path explicitly: `edit_docx.py ~/documents/<name>.docx --heading ...`

`edit_docx.py` opens the file, appends in place, and saves back to the same path. Repeat `--heading`, `--paragraph`, and `--bullet` as needed; use `--blocks <spec.json>` for tables or mixed blocks (same block schema as `create_docx.py`). Only write a task-local python script (through `scripts/skill_runtime.py python <file.py>`) when the change is beyond appending — a rewrite, deletion, or style change — and expand `~` with `os.path.expanduser`.
3. Then deliver the edited `~/documents/<name>.docx`. Keep the same filename so the next edit or delete task still finds it; save to a new filename only when the user attached a file in THIS conversation or explicitly wants both versions kept.

When the file contains tracked changes, comments, fields, complex headers, or custom XML, inspect the package structure before editing (`python3 inspect_package.py`). Avoid XML rewrites unless required; if XML editing is required, preserve namespaces, relationships, and content types.

## Validation

After saving, run `scripts/validate_docx.py` through `scripts/skill_runtime.py`:

```json
{
  "command": "python3 /workspace/skills/document/scripts/skill_runtime.py python /workspace/skills/document/scripts/validate_docx.py <title>.docx --required-text \"<source name>\" --required-text \"<source date>\" --forbidden-text \"<unsupported claim>\"",
  "workingDirectoryPath": "~/documents"
}
```

Read the JSON warnings and revise the document when they identify a real quality issue. The validator reports source coverage, forbidden text, page margins, detected font names, normal font size, line spacing, and table density so you can revise before attaching instead of guessing from the filename.

For important layout work, convert to PDF or images when the runtime has LibreOffice available, or use `file.preview` when it is the available visual check. Inspect the result for table clipping, broken Korean text, empty pages, and unreadable spacing before attaching.

## Custom Python

For work that exceeds the bundled scripts, create a task-local Python file such as `custom_docx.py`, then run it through the bundled runtime:

```json
{
  "command": "python3 /workspace/skills/document/scripts/skill_runtime.py python custom_docx.py",
  "workingDirectoryPath": "~/documents"
}
```

Inside that file, import the Word document library normally; the wrapper has already created and selected the venv.

## Style Guide

Prefer clear headings, short paragraphs, and real Word tables over monospaced text. For Korean documents, use a Korean-capable font such as `Noto Sans CJK KR`, `NanumGothic`, `Malgun Gothic`, or the closest installed equivalent.

Use a calm document type scale: body text around 10-11 pt, compact headings around 12-16 pt, readable line spacing around 1.05-1.2, and page margins around 0.6-1.0 inches for business reports. Korean and English mixed text should use a Korean-capable font for `eastAsia` and Latin runs so glyph weight does not look mismatched. Avoid giant title blocks, tiny table text, cramped rows, and excessive blank space.

## Tables

Use Word tables for structured content. Keep table text concise enough to fit the page. Make the first row a short header row, set sensible column widths with `columnWidthsInches` for important tables, and split very wide tables into multiple narrower tables instead of shrinking text until it becomes unreadable. Use landscape orientation only when the content truly needs width.

For source-backed business reports, build a short source checklist before writing the spec: required names, dates, totals, percentages, owners, missing values, and forbidden invented facts. After generating the `.docx`, reopen or validate it and confirm those checklist values appear in the document body. If validation reports dense cells, blank cells, a missing multi-row table, or too little visible text, revise the document before attaching.
