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
2. Create work under `tmp/<workbook-slug>` relative to the default writable workspace directory; do not use Blueclaw internal temporary paths.
3. For straightforward new workbooks, write a JSON spec and run `scripts/create_xlsx.py`.
4. For custom charts, advanced formulas, macros, or edits that exceed the JSON script, write a task-local Python file and run it through `scripts/skill_runtime.py python <file.py>`.
5. Use Python `csv` for `.csv` and `.tsv` parsing before writing workbook output.
6. Validate with `scripts/validate_xlsx.py` or by reopening the workbook and checking sheets, dimensions, formulas, and obvious formatting.
7. Move accepted final files to `artifacts/<workbook-slug>/` unless the user requested a circle or shared destination, then attach the final spreadsheet; attach source CSVs only if requested.

Bundled scripts are responsible for their own Python dependencies. They use the built-in `/opt/blueclaw/builtin-skills-venv` first, then bootstrap `scripts/requirements.txt` with `uv` into `$BLUECLAW_REQUESTER_TMP/.skill-env/xlsx` only when needed. `/workspace/shared/cache/dependencies` is only a package cache. Do not run `pip install` directly. Do not stop at `ModuleNotFoundError`; run the helper script first. Do not use `python - <<'PY'` or system Python snippets for code that needs the Excel library.

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
  "command": "python3 /workspace/skills/xlsx/scripts/skill_runtime.py python /workspace/skills/xlsx/scripts/create_xlsx.py workbook.json output.xlsx && python3 /workspace/skills/xlsx/scripts/skill_runtime.py python /workspace/skills/xlsx/scripts/validate_xlsx.py output.xlsx",
  "workingDirectoryPath": "tmp/<workbook-slug>"
}
```

## Custom Python

For work that exceeds the JSON script, create a task-local Python file such as `custom_xlsx.py`, then run it through the bundled runtime:

```json
{
  "command": "python3 /workspace/skills/xlsx/scripts/skill_runtime.py python custom_xlsx.py",
  "workingDirectoryPath": "tmp/<workbook-slug>"
}
```

Inside that file, import the Excel library normally; the wrapper has already created and selected the venv.

Use professional formatting: readable column widths, frozen header rows, number formats, and clear sheet names. Keep raw data and summary views on separate sheets when both are useful.

## Formulas

Write formulas only when the user needs the workbook to remain interactive. Avoid fragile formulas with hard-coded row ranges when a dynamic range is clearer. If the runtime cannot recalculate formulas, preserve formulas and verify that cell references are structurally correct.

## CSV And TSV

Normalize tabular text before creating the workbook. Handle malformed rows explicitly. Do not silently drop columns or rows. If data cannot be repaired with confidence, create a separate `Issues` sheet listing the affected row numbers and reasons.

## Editing Existing Files

For user-provided workbooks:

1. Write a task-local edit script and run it through `scripts/skill_runtime.py python <file.py>`.
2. Preserve existing sheet names, formulas, styles, and macros unless the user asks otherwise.
3. Save to a new filename unless the user asks to replace the original.

Use macro-preserving load options for `.xlsm` files.

## Validation

Always run `scripts/validate_xlsx.py` through `scripts/skill_runtime.py` after saving.

Check for broken references in formulas when formulas were added or moved. For financial or operational sheets, scan for blank required fields and suspicious totals before attaching.
