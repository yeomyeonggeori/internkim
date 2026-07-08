---
name: pdf
description:
  "Work with existing PDF files (read, extract, merge, split, edit with pypdf) and build layout-critical
  PDFs with fpdf2 when precise visual placement is the point. For content-first reports, memos, and
  documents — including PDF deliverables — use the document skill, which authors markdown and exports
  docx or pdf. Do not use for standardized company letterhead forms (견적서, 청구서, 발주서, 품의서,
  증명서, quotation, invoice, purchase order, certificate) — the paperwork skill owns those layouts."
when_to_use: Use for 기존 PDF 읽기/요약/병합/분할/편집 or layout-critical PDF composition. For new content-first documents (보고서, 회의록, 메모) delivered as PDF, use the document skill instead.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# Generating PDFs with fpdf2

Use this skill to create, validate, read, merge, or lightly edit PDFs with Python. Work directly in `~/documents` and deliver only accepted final files through `file.deliver`.

## Workflow

1. Work directly in `~/documents` (run `mkdir -p ~/documents` once first). Build, validate, and deliver the PDF there.
2. If the user refers to a document from an earlier task ("방금 만든", "그 문서", "the doc I just made"), the source is the workspace file in `~/documents/`, NOT the delivered attachment. Do NOT call `file.preview` or `file.read` on that attachment. For the common case — appending a section — run the deterministic `scripts/edit_pdf.py` with no path; it targets the newest `.pdf` in `~/documents/` automatically (see Editing Existing Files). Do NOT hand-write Python or use `python -c` for a simple append. Save back to the SAME `~/documents/<name>.pdf` and re-deliver from there.
3. Only when the user uploaded a NEW file in THIS conversation and you need to read it (summarize, extract, OCR — not edit a binary you made), call `file.preview` first; use `file.read` only for exact UTF-8 text ranges after previewing.
4. For short proposals, estimates, reports, invoices, and source-backed documents, run `scripts/create_pdf.py` with inline `--title`/`--heading`/`--paragraph`/`--bullet` flags. Use `--spec <file>` only for rich layouts with tables or complex multi-section content.
5. Use custom Python only when layout needs features the JSON helper does not support.
6. Run every fpdf2 or pypdf command through `python3 /workspace/skills/pdf/scripts/skill_runtime.py python ...`; do not run `pip install` or bare `python3 create_pdf.py`.
7. Validate before delivery with `validate_pdf.py` or an equivalent pypdf check for page count, extractable PDF text, required source facts, forbidden unsupported facts, encryption, embedded fonts, and Korean-capable font names.
8. Save the final PDF to `~/documents/<title>.pdf` (run `mkdir -p ~/documents` first) so it persists for later edit and delete tasks, then deliver it from there with `file.deliver`. Deliver intermediate files only when asked.

Use `timeoutSecond` of at least 300 for generation and validation. First-run dependency checks and font embedding can exceed 120 seconds. If generation produced a PDF but validation timed out, do not repeat the identical command; validate the existing file separately with a longer timeout.

## Source Truth

Treat supplied files and pasted data as authoritative. Preserve company names, product names, people, dates, amounts, IDs, and units exactly unless the user asks for translation or normalization. Do not invent missing vendors, prices, discounts, totals, dates, contacts, warranties, or background. Put source facts in extractable text, not only image-like decoration or the final reply. Use the user's-language equivalent of "Not provided" for missing fields.

Always preserve supplied data as the source of truth.

For numeric documents, compute totals from source numbers in code and assert the computed total equals the source-provided total before writing the final PDF.

## Runtime Rules

- Korean/CJK text requires a TTF or TTC font. Built-in Helvetica, Times, and Courier cannot render Korean.
- `fpdf2` and `pypdf` are preinstalled in the built-in skill runtime.
- Bundled scripts own dependency setup through `skill_runtime.py`.
- Fonts must be local files at render time. Remote URLs are only useful for downloading a TTF into the task directory.
- `references/google-fonts.txt` lists Latin, Korean, and CJK Google Fonts with TTF URLs.


## Saving and managing the document

Save the final PDF to `~/documents/<title>.pdf` so it persists across tasks; run `mkdir -p ~/documents` once before writing there. `~` is the requester personal workspace and resolves the same way in a tool path field and in a shell command. To edit or delete a document the user names in a later task, list `~/documents/` (`ls -t ~/documents`) to find the file, then edit it in place or remove it with `file.delete`.

## Helper Path

For a common PDF, pass content directly as inline arguments — no spec file needed:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/create_pdf.py ~/documents/<title>.pdf --title \"Cafe Suyu 간판 교체 제안서\" --subtitle \"제안일 2026-07-02\" --heading \"제안 개요\" --paragraph \"제공된 소스 데이터만 바탕으로 작성한 제안서입니다.\" --heading \"결론\" --bullet \"항목 1\" --bullet \"항목 2\" && python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/validate_pdf.py <title>.pdf --min-pages 1 --max-pages 2 --minimum-text-length 200 --required-text \"<source name>\" --required-text \"<source total>\" --forbidden-text \"<unsupported claim>\"",
  "timeoutSecond": 300,
  "workingDirectoryPath": "~/documents"
}
```

`--heading` is repeatable and starts a new section; `--paragraph` and `--bullet` accumulate into the last section. `create_pdf.py` and `edit_pdf.py` share the same `--heading`/`--paragraph`/`--bullet` vocabulary.

For rich PDFs with tables or complex multi-section layouts, write a spec file and pass it with `--spec`:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/create_pdf.py ~/documents/<title>.pdf --spec proposal.json && python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/validate_pdf.py <title>.pdf --min-pages 1 --max-pages 2 --minimum-text-length 200 --required-text \"<source name>\" --required-text \"<source total>\" --forbidden-text \"<unsupported claim>\"",
  "timeoutSecond": 300,
  "workingDirectoryPath": "~/documents"
}
```

The spec schema supports `title`, `subtitle`, `pageNumbers`, `format`, `marginMillimeters`, and `sections` (each with `title`, `paragraphs`, `bullets`, `table`).

If validation reports missing required text, unsupported text, too many pages, no text layer, encryption, missing glyphs, or font problems, revise before attaching.

## Custom Python

Use fpdf2 for layout and pypdf for reading or merging. Register a Korean-capable font before `set_font()` whenever text is non-Latin:

```python
from fpdf import FPDF

pdf = FPDF()
pdf.add_font("NanumGothic", fname="fonts/NanumGothic.ttf")
pdf.set_auto_page_break(auto=True, margin=15)
pdf.add_page()
pdf.set_font("NanumGothic", size=11)
pdf.multi_cell(0, 8, "한글 텍스트가 잘 출력됩니다.")
pdf.output("output.pdf")
```

Run task-local generation code through the bundled runtime:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python create_pdf.py",
  "timeoutSecond": 300,
  "workingDirectoryPath": "~/documents"
}
```

Use `multi_cell()` for wrapped body text. Keep A4 margins around 15-22 mm unless the document has a specific print requirement. Body text should usually be 9.5-11.5 pt, table text 8.5-10.5 pt, and section headings 12-16 pt. Prefer bordered tables with short wrapped labels for Korean proposals, estimates, invoices, and operational reports.

For Korean fonts, prefer installed TTF/TTC files. If none are available, find a URL in `references/google-fonts.txt`, download it into `~/documents/fonts`, verify it is a real font, then register it:

```bash
grep "^Nanum Gothic" /workspace/skills/pdf/references/google-fonts.txt | grep "400"
curl -sL "<url>" -o fonts/NanumGothic.ttf
file fonts/NanumGothic.ttf
```

## Editing Existing Files

To edit a PDF you delivered in an earlier task, the source is the workspace file in `~/documents/` — not the delivered attachment.

1. For "the doc I just made" (most recently created .pdf), pass no path — the script targets the newest `.pdf` in `~/documents/` automatically. Only run `ls -t ~/documents` when the user names a specific older file and you need to confirm the exact filename.
2. For the common case — append a heading, paragraphs, or bullet items — run the deterministic editor. Do NOT hand-write Python or use `python -c`:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/edit_pdf.py --heading \"추가 사항\" --paragraph \"첫 번째 내용\" --paragraph \"두 번째 내용\"",
  "timeoutSecond": 300,
  "workingDirectoryPath": "~/documents"
}
```

For a specific older file named by the user, pass the path explicitly: `edit_pdf.py ~/documents/<name>.pdf --heading ...`

`edit_pdf.py` builds a new page with fpdf2 (Korean-capable font auto-selected from the same font candidates as `create_pdf.py`), appends it to the existing PDF using pypdf, and writes back to the same path. Repeat `--paragraph` and `--bullet` as needed. Use `--section <spec.json>` when the content needs a structured section object (same schema as a `create_pdf.py` sections array element, supports `title`, `paragraphs`, `bullets`, `table`). Only write a task-local Python script (through `scripts/skill_runtime.py python <file.py>`) when the edit is beyond appending — a rewrite, page deletion, annotation, or structural change — and expand `~` with `os.path.expanduser`.
3. Deliver the edited `~/documents/<name>.pdf`. Keep the same filename so the next edit or delete task still finds it; save to a new filename only when the user attached a file in THIS conversation or explicitly wants both versions kept.

## Validation

Validate the saved PDF before promotion:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/validate_pdf.py <title>.pdf --max-pages 2 --required-text Cafe Suyu --required-text 3220000 --forbidden-text warranty --forbidden-text discount",
  "timeoutSecond": 300,
  "workingDirectoryPath": "~/documents"
}
```

Use actual required and forbidden values from the source. Treat warnings as revision input rather than explaining them away in the final reply. Open or render the PDF when possible and check missing glyphs, clipped text, broken tables, uneven margins, oversized titles, tiny body text, page numbers, and extractable text before attaching.
