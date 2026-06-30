---
name: pdf
description:
  "Generate PDF documents using fpdf2 (Python). Use when creating PDFs, generating documents,
  reports, invoices, forms, or when user mentions PDF generation or document creation. Also use
  pypdf for reading or editing existing PDF files."
when_to_use: Use for PDF, document, report, invoice, 문서, 보고서, 견적서, 청구서, PDF generation, or PDF reading/editing requests.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# Generating PDFs with fpdf2

Use this skill to create, validate, read, merge, or lightly edit PDFs with Python. Work under `tmp/<pdf-slug>`, keep drafts and previews there, and deliver only accepted final files through `file.deliver`.

## Workflow

1. Create work under `tmp/<pdf-slug>`.
2. For existing PDFs or images, call `file.preview` first; use `file.read` only for exact UTF-8 text ranges after previewing.
3. For short proposals, estimates, reports, invoices, and source-backed documents, write a JSON spec and run `scripts/create_pdf.py` through `scripts/skill_runtime.py`.
4. Use custom Python only when layout needs features the JSON helper does not support.
5. Run every fpdf2 or pypdf command through `python3 /workspace/skills/pdf/scripts/skill_runtime.py python ...`; do not run `pip install` or bare `python3 create_pdf.py`.
6. Validate before delivery with `validate_pdf.py` or an equivalent pypdf check for page count, extractable PDF text, required source facts, forbidden unsupported facts, encryption, embedded fonts, and Korean-capable font names.
7. Deliver final PDFs with `file.deliver`. Deliver intermediate files only when asked.

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

Save the final PDF to `~/documents/<title>.pdf` so it persists across tasks; run `mkdir -p ~/documents` once before writing there. `~` is the requester personal workspace and resolves the same way in a tool path field and in a shell command. To edit or delete a document the user names in a later task, list `~/documents/` (`ls ~/documents`) to find the file, then edit it in place or remove it with `file.delete`.

## Helper Path

For a standard short PDF, create a spec:

```json
{
  "title": "Cafe Suyu 간판 교체 제안서",
  "subtitle": "제안일 2026-07-02",
  "pageNumbers": true,
  "sections": [
    {
      "title": "제안 개요",
      "paragraphs": ["제공된 소스 데이터만 바탕으로 작성한 제안서입니다."]
    },
    {
      "title": "견적",
      "table": {
        "headers": ["항목", "금액", "비고"],
        "rows": [["제작", "350000", "소스 금액 그대로 표기"]]
      }
    }
  ]
}
```

Run generation and validation from `tmp/<pdf-slug>`:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/create_pdf.py proposal.json output.pdf && python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/validate_pdf.py output.pdf --min-pages 1 --max-pages 2 --minimum-text-length 200 --required-text \"<source name>\" --required-text \"<source total>\" --forbidden-text \"<unsupported claim>\"",
  "timeoutSecond": 300,
  "workingDirectoryPath": "tmp/<pdf-slug>"
}
```

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
  "workingDirectoryPath": "tmp/<pdf-slug>"
}
```

Use `multi_cell()` for wrapped body text. Keep A4 margins around 15-22 mm unless the document has a specific print requirement. Body text should usually be 9.5-11.5 pt, table text 8.5-10.5 pt, and section headings 12-16 pt. Prefer bordered tables with short wrapped labels for Korean proposals, estimates, invoices, and operational reports.

For Korean fonts, prefer installed TTF/TTC files. If none are available, find a URL in `references/google-fonts.txt`, download it into `tmp/<pdf-slug>/fonts`, verify it is a real font, then register it:

```bash
grep "^Nanum Gothic" /workspace/skills/pdf/references/google-fonts.txt | grep "400"
curl -sL "<url>" -o fonts/NanumGothic.ttf
file fonts/NanumGothic.ttf
```

## Validation

Validate the saved PDF before promotion:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/validate_pdf.py output.pdf --max-pages 2 --required-text Cafe Suyu --required-text 3220000 --forbidden-text warranty --forbidden-text discount",
  "timeoutSecond": 300,
  "workingDirectoryPath": "tmp/<pdf-slug>"
}
```

Use actual required and forbidden values from the source. Treat warnings as revision input rather than explaining them away in the final reply. Open or render the PDF when possible and check missing glyphs, clipped text, broken tables, uneven margins, oversized titles, tiny body text, page numbers, and extractable text before attaching.
