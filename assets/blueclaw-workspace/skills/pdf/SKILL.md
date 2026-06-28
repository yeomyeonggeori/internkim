---
name: pdf
description:
  "Generate PDF documents using fpdf2 (Python). Use when creating PDFs, generating documents,
  reports, invoices, forms, or when user mentions PDF generation or document creation. Also use
  pypdf for reading or editing existing PDF files."
when_to_use: Use for PDF, document, report, invoice, 문서, 보고서, 견적서, 청구서, PDF generation, or PDF reading/editing requests.
allowed-tools:
  - file.preview
  - file.read
  - terminal.run
  - file.write
  - file.edit
  - file.patch
  - file.promote
  - file.attach
completion:
  requiredEvidenceTools:
    - file.promote
    - file.attach
---

# Generating PDFs with fpdf2

Pure Python — no Node.js required. Works on any architecture including RISC-V.

## Workflow

1. Create new PDF work under `tmp/<pdf-slug>` relative to the default writable workspace directory; do not use Blueclaw internal temporary paths.
2. When the user asks to read, summarize, extract, OCR, or reuse content from an existing PDF or image, call `file.preview` first; use `file.read` only for exact UTF-8 text ranges after previewing.
3. Keep drafts, downloaded fonts, and generated previews in the task temporary directory while iterating.
4. For straightforward proposals, estimates, reports, invoices, and short source-backed documents, write a JSON spec and run `scripts/create_pdf.py` through `scripts/skill_runtime.py`; use custom Python only when the layout needs features the JSON helper does not support.
5. Run custom PDF generation scripts through `scripts/skill_runtime.py python <file.py>` so `fpdf2` and `pypdf` resolve from the bundled dependency environment.
6. Validate the final PDF with `scripts/skill_runtime.py python scripts/validate_pdf.py` or an equivalent `pypdf` check for page count, extractable text, required source facts, forbidden unsupported facts, encryption, embedded fonts, and Korean-capable font names. Pass source-provided names, dates, totals, and key labels as repeated `--required-text` arguments. Pass likely invented or explicitly disallowed claims as repeated `--forbidden-text` arguments.
7. Promote accepted final PDFs to `artifacts/<pdf-slug>/` with `file.promote` unless the user requested a circle or shared destination.
8. Attach the promoted PDF. Attach intermediate files only if the user asks.

Bundled scripts are responsible for their own Python dependencies. Run them through `scripts/skill_runtime.py`; the wrapper selects the built-in dependency environment first and prepares requester-owned fallback storage with `uv` only when needed. Do not run `pip install` directly. Do not use bare `python3 create_pdf.py` for fpdf2 or pypdf work; it may use a Python environment without the PDF libraries.

Use `timeoutSecond` of at least 300 for PDF generation and validation terminal runs. First-run dependency checks and font embedding can exceed 120 seconds. If generation prints the output PDF path or the PDF file exists but validation times out, do not discard the file and do not repeat the identical command. Run validation as a separate terminal step with a longer timeout, or inspect/promote the existing PDF if the separate validation passes.

Treat supplied files and pasted source data as the source of truth. Preserve source-provided company names, product names, people, dates, amounts, IDs, and units exactly unless the user asks for translation or normalization. Do not invent missing vendors, prices, discounts, totals, dates, contact details, warranties, or external background. Put the source-provided client/project name, proposal date, schedule dates, quote items, and totals in extractable PDF text, not only in image-like decorations or the final reply. When a useful field is missing, write the user's-language equivalent of "Not provided" instead of filling a plausible value.

## CRITICAL REQUIREMENTS

1. **Korean/CJK text requires a TTF font** — built-in fonts (Helvetica, Times, Courier) cannot
   render Korean. Always register a TTF font for any non-Latin text.
2. **fpdf2 and pypdf are pre-installed in the built-in skill runtime** — do not run pip install.
3. **Fonts must be local files** — download TTFs before use; remote URLs do not work at render time.

## Files

- `references/google-fonts.txt` — ~65 Latin + Korean/CJK Google Fonts with TTF URLs.
  Format per line: `font name\tstyle\tcategory\tweight\turl`
  Korean fonts included: Nanum Gothic, Nanum Myeongjo, Nanum Gothic Coding, Do Hyeon,
  Black Han Sans, Gugi, Jua, Gaegu, Stylish
  CJK fonts included: Noto Sans JP, Noto Sans SC, Noto Sans TC

## Helper Script

For standard short PDFs, create a spec file:

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

Run:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/create_pdf.py proposal.json output.pdf && python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/validate_pdf.py output.pdf --min-pages 1 --max-pages 2 --minimum-text-length 200 --required-text \"<source name>\" --required-text \"<source total>\" --forbidden-text \"<unsupported claim>\"",
  "timeoutSecond": 300,
  "workingDirectoryPath": "tmp/<pdf-slug>"
}
```

The helper chooses a Korean-capable font when one is installed and fails clearly when non-Latin text has no usable font. If validation warns about missing required text, unsupported text, too many pages, no text layer, or font problems, revise the spec before attaching.

## Basic Example

```python
from fpdf import FPDF

pdf = FPDF()
pdf.add_page()
pdf.set_font("Helvetica", size=24)
pdf.cell(0, 10, "Document Title", new_line="NEXT")
pdf.set_font("Helvetica", size=12)
pdf.multi_cell(0, 8, "Body text wraps automatically across lines.")
pdf.output("output.pdf")
```

Run task-local generation code through the bundled runtime:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python create_pdf.py",
  "workingDirectoryPath": "tmp/<pdf-slug>"
}
```

Run validation through the same runtime and revise when the JSON contains real warnings:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/validate_pdf.py output.pdf --min-pages 1 --minimum-text-length 200 --required-text \"<source name>\" --required-text \"<source date>\" --forbidden-text \"<unsupported claim>\"",
  "timeoutSecond": 300,
  "workingDirectoryPath": "tmp/<pdf-slug>"
}
```

## Korean / CJK Fonts

Always use a TTF font for Korean. Prefer an installed Korean-capable TTF/TTC when available; if none is available, find the URL in `references/google-fonts.txt`, download it into the task temporary directory, verify the file is a real font, then register:

```bash
# Find URL
grep "^Nanum Gothic" skills/pdf/references/google-fonts.txt | grep "400"

# Download
mkdir -p fonts
curl -sL "<url>" -o fonts/NanumGothic.ttf
```

```python
from fpdf import FPDF

pdf = FPDF()
pdf.add_font("NanumGothic", fname="fonts/NanumGothic.ttf")
pdf.add_page()
pdf.set_font("NanumGothic", size=14)
pdf.multi_cell(0, 8, "한글 텍스트가 잘 출력됩니다.")
pdf.output("korean.pdf")
```

For bold/italic variants, register each separately:

```python
pdf.add_font("NanumGothic", fname="fonts/NanumGothic.ttf")
pdf.add_font("NanumGothic", style="B", fname="fonts/NanumGothic-Bold.ttf")
pdf.set_font("NanumGothic", style="B", size=14)
```

## Text Layout

```python
# Single line — new_line="NEXT" moves cursor down, "RIGHT" stays on same line
pdf.cell(width, height, text, new_line="NEXT", align="C")  # align: L, C, R

# Multi-line wrapping block
pdf.multi_cell(0, 8, long_text, align="J")  # J = justify

# Manual positioning
pdf.set_xy(x, y)

# Add vertical space
pdf.ln(5)
```

## Tables

```python
from fpdf import FPDF

pdf = FPDF()
pdf.add_page()

headers = ["항목", "금액", "비고"]
col_widths = [80, 50, 50]

# Header row
pdf.set_font("NanumGothic", style="B", size=11)
pdf.set_fill_color(230, 230, 230)
for header, w in zip(headers, col_widths):
    pdf.cell(w, 8, header, border=1, align="C", fill=True)
pdf.ln()

# Data rows
pdf.set_font("NanumGothic", size=11)
rows = [("인건비", "5,000,000", "월급"), ("임대료", "1,000,000", "사무실"), ("기타", "500,000", "")]
for row in rows:
    for value, w in zip(row, col_widths):
        pdf.cell(w, 8, value, border=1)
    pdf.ln()

pdf.output("table.pdf")
```

For Korean proposals, estimates, invoices, and operational reports, prefer bordered tables with short wrapped labels over paragraph lists of numbers. Use `multi_cell()` for long text, keep column widths within `pdf.epw`, and put page numbers in a footer when the user asks for a formal document.

Use a compact print type scale: body text around 9.5-11.5 pt, table text around 8.5-10.5 pt, section headings around 12-16 pt, and enough leading that Korean and English mixed lines do not touch. Keep A4 margins around 15-22 mm unless the document has a specific print requirement. Avoid one huge title area, tiny dense tables, edge-hugging text, and excessive empty whitespace.

## Headers and Footers (every page)

```python
from fpdf import FPDF

class MyPDF(FPDF):
    def header(self):
        self.set_font("Helvetica", style="B", size=11)
        self.cell(0, 10, "사업계획서", align="C", new_line="NEXT")
        self.ln(2)

    def footer(self):
        self.set_y(-15)
        self.set_font("Helvetica", style="I", size=8)
        self.cell(0, 10, f"- {self.page_no()} -", align="C")

pdf = MyPDF()
pdf.set_auto_page_break(auto=True, margin=20)
pdf.add_page()
# ... content ...
pdf.output("report.pdf")
```

## Images

```python
pdf.image("photo.jpg", x=10, y=20, w=100)   # width 100mm, auto height
pdf.image("photo.png", w=pdf.epw)            # full page width
```

## Colors and Shapes

```python
pdf.set_text_color(50, 50, 50)
pdf.set_fill_color(240, 240, 240)
pdf.set_draw_color(150, 150, 150)

pdf.rect(x=10, y=30, w=80, h=20, style="FD")  # F=fill, D=border, FD=both
pdf.line(10, 50, 100, 50)
```

## Page Settings

```python
pdf = FPDF(orientation="P", unit="mm", format="A4")
pdf.set_margins(left=20, top=20, right=20)
pdf.set_auto_page_break(auto=True, margin=15)
```

## Reading Existing PDFs (pypdf)

```python
from pypdf import PdfReader, PdfWriter

# Read text
reader = PdfReader("existing.pdf")
for page in reader.pages:
    print(page.extract_text())

# Merge
writer = PdfWriter()
for r in [PdfReader("a.pdf"), PdfReader("b.pdf")]:
    for page in r.pages:
        writer.add_page(page)
with open("merged.pdf", "wb") as f:
    writer.write(f)
```

## Google Fonts

```bash
# Search available fonts
grep "^Nanum" skills/pdf/references/google-fonts.txt
grep "Korean\|gothic\|myeong" skills/pdf/references/google-fonts.txt -i

# Download
curl -sL "<url-from-txt>" -o fonts/MyFont.ttf
file fonts/MyFont.ttf  # must show "TrueType Font data"
```

## Best Practices

1. Always call `set_auto_page_break(auto=True, margin=15)` to avoid content clipping
2. Use `multi_cell()` for body text — handles line wrapping automatically
3. Define column widths that sum to ≤ `pdf.epw` (effective page width, default ~170mm for A4)
4. For Korean/CJK, always register TTF via `add_font()` before `set_font()`
5. Compute totals from source numbers in code and assert the computed total equals the source-provided total before writing the final PDF
6. Open or render the PDF and check for missing glyphs, clipped text, broken tables, uneven margins, oversized title blocks, tiny body text, and page numbers before attaching
7. Run `scripts/skill_runtime.py python scripts/validate_pdf.py` before attaching; revise when it reports missing required text, forbidden unsupported text, no extractable text layer, encryption, or too many pages
8. Call `pdf.output()` last

## Validation

For source-backed proposals, estimates, reports, and invoices, validate the saved PDF before promoting it:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/validate_pdf.py output.pdf --max-pages 2 --required-text Cafe Suyu --required-text 3220000 --forbidden-text warranty --forbidden-text discount",
  "workingDirectoryPath": "tmp/<pdf-slug>"
}
```

Use the actual required and forbidden values from the user-provided source. Treat validation warnings as revision input rather than explaining them away in the final reply.
