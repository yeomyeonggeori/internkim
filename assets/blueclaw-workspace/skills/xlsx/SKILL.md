---
name: xlsx
description: Create, read, edit, clean, calculate, format, and attach spreadsheet files. Use for .xlsx, .xlsm, .csv, .tsv, Excel, tables, formulas, charts, spreadsheet cleanup, 엑셀, 스프레드시트, 시트, 표, or 계산표 requests. Do not use when the primary deliverable is a Word document, PDF, slide deck, database pipeline, or Google Sheets file.
when_to_use: Use when the user asks for Excel, .xlsx, .xlsm, .csv, .tsv, spreadsheet, table cleanup, formulas, charts, 엑셀, 스프레드시트, 시트, 표, or a spreadsheet deliverable.
allowed-tools:
  - terminal.run
  - file.write
  - file.attach
---

# XLSX Spreadsheets

Create or modify spreadsheet files as local artifacts, then attach the final workbook.

## Workflow

1. Clarify only missing inputs that affect workbook structure, such as columns, source data, formulas, or output format.
2. Create work under `$BLUECLAW_TASK_TMP`; do not use Blueclaw internal temporary paths.
3. For straightforward new workbooks, write a JSON spec and run `scripts/create_xlsx.py`.
4. Use direct `openpyxl` code for custom charts, advanced formulas, macros, or edits that exceed the script.
5. Use Python `csv` for `.csv` and `.tsv` parsing before writing workbook output.
6. Validate with `scripts/validate_xlsx.py` or by reopening the workbook and checking sheets, dimensions, formulas, and obvious formatting.
7. Move accepted final files to `$BLUECLAW_REQUESTER_ARTIFACTS/<workbook-slug>/` unless the user requested a circle or shared destination, then attach the final spreadsheet; attach source CSVs only if requested.

Bundled scripts are responsible for their own Python dependencies. They bootstrap `scripts/requirements.txt` into `$BLUECLAW_REQUESTER_TMP/.skill-env/xlsx` when needed and use `/workspace/shared/cache/dependencies` only as a package cache. Do not stop at `ModuleNotFoundError`; run the helper script first. For custom Python beyond the helper script, import `scripts/skill_runtime.py` and call `ensure_requirements("xlsx")` before importing `openpyxl`.

## Helper Scripts

For normal workbooks, create a spec file:

```json
{
  "sheets": [
    {
      "title": "Summary",
      "freezePanes": "A2",
      "autoFilter": true,
      "rows": [
        ["Item", "Quantity", "Unit Price", "Total"],
        ["Plan", 2, 15000, "=B2*C2"]
      ]
    },
    {
      "title": "Imported",
      "csvPath": "input.csv",
      "delimiter": ","
    }
  ]
}
```

Run:

```json
{
  "command": "python3 /workspace/skills/xlsx/scripts/create_xlsx.py workbook.json output.xlsx && python3 /workspace/skills/xlsx/scripts/validate_xlsx.py output.xlsx",
  "workingDirectoryPath": "$BLUECLAW_TASK_TMP"
}
```

## Creation

Use `openpyxl` for normal workbook generation:

```python
from openpyxl import Workbook
from openpyxl.styles import Font, PatternFill
from openpyxl.utils import get_column_letter

workbook = Workbook()
worksheet = workbook.active
worksheet.title = "Summary"

headers = ["Item", "Amount", "Notes"]
worksheet.append(headers)
for row in rows:
    worksheet.append(row)

header_fill = PatternFill("solid", fgColor="E5E7EB")
for cell in worksheet[1]:
    cell.font = Font(bold=True)
    cell.fill = header_fill

for column_cells in worksheet.columns:
    width = max(len(str(cell.value or "")) for cell in column_cells)
    worksheet.column_dimensions[get_column_letter(column_cells[0].column)].width = min(max(width + 2, 10), 48)

workbook.save("output.xlsx")
```

Use professional formatting: readable column widths, frozen header rows, number formats, and clear sheet names. Keep raw data and summary views on separate sheets when both are useful.

## Formulas

Write formulas only when the user needs the workbook to remain interactive:

```python
worksheet["D2"] = "=B2*C2"
worksheet["D2"].number_format = '#,##0'
```

Avoid fragile formulas with hard-coded row ranges when a dynamic range is clearer. If the runtime cannot recalculate formulas, preserve formulas and verify that cell references are structurally correct.

## CSV And TSV

Normalize tabular text before creating the workbook:

```python
import csv

with open("input.csv", newline="", encoding="utf-8-sig") as file:
    rows = list(csv.reader(file))
```

Handle malformed rows explicitly. Do not silently drop columns or rows. If data cannot be repaired with confidence, create a separate `Issues` sheet listing the affected row numbers and reasons.

## Editing Existing Files

For user-provided workbooks:

1. Load the file with `openpyxl.load_workbook`.
2. Preserve existing sheet names, formulas, styles, and macros unless the user asks otherwise.
3. Save to a new filename unless the user asks to replace the original.

Use `keep_vba=True` for `.xlsm` files:

```python
from openpyxl import load_workbook

workbook = load_workbook("input.xlsm", keep_vba=True)
workbook.save("updated.xlsm")
```

## Validation

Always reopen the saved file:

```python
from openpyxl import load_workbook

workbook = load_workbook("output.xlsx", data_only=False)
print(workbook.sheetnames)
for worksheet in workbook.worksheets:
    print(worksheet.title, worksheet.max_row, worksheet.max_column)
```

Check for broken references in formulas when formulas were added or moved. For financial or operational sheets, scan for blank required fields and suspicious totals before attaching.
