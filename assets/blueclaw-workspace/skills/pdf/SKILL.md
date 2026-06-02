---
name: pdf
description:
  "Generate PDF documents using fpdf2 (Python). Use when creating PDFs, generating documents,
  reports, invoices, forms, or when user mentions PDF generation or document creation. Also use
  pypdf for reading or editing existing PDF files."
when_to_use: Use for PDF, document, report, invoice, 문서, 보고서, 견적서, 청구서, PDF generation, or PDF reading/editing requests.
allowed-tools:
  - file.read
  - terminal.run
  - file.write
  - file.edit
  - file.patch
  - file.promote
  - file.attach
---

# Generating PDFs with fpdf2

Pure Python — no Node.js required. Works on any architecture including RISC-V.

## Workflow

1. Create new PDF work under `tmp/<pdf-slug>` relative to the default writable workspace directory; do not use Blueclaw internal temporary paths.
2. When the user asks to read, summarize, extract, OCR, or reuse content from an existing PDF or image, call `file.read` first.
3. Keep drafts, downloaded fonts, and generated previews in the task temporary directory while iterating.
4. Promote accepted final PDFs to `artifacts/<pdf-slug>/` with `file.promote` unless the user requested a circle or shared destination.
5. Attach the promoted PDF. Attach intermediate files only if the user asks.

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

## Korean / CJK Fonts

Always use a TTF font for Korean. Find the URL in `references/google-fonts.txt`, download it, then register:

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
5. Call `pdf.output()` last
