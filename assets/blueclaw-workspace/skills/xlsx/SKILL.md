---
name: xlsx
description: Create, read, edit, clean, calculate, format, and attach spreadsheet files. Use for .xlsx, .xlsm, .csv, .tsv, Excel, tables, formulas, charts, spreadsheet cleanup, 엑셀, 스프레드시트, 시트, 표, or 계산표 requests. Do not use when the primary deliverable is a Word document, PDF, slide deck, database pipeline, or Google Sheets file.
when_to_use: Use when the user asks for Excel, .xlsx, .xlsm, .csv, .tsv, spreadsheet, table cleanup, formulas, charts, 엑셀, 스프레드시트, 시트, 표, or a spreadsheet deliverable.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# XLSX Spreadsheets

Create or modify spreadsheet files as local artifacts, then attach the final workbook.

## Workflow

1. Clarify only missing inputs that affect workbook structure, such as columns, source data, formulas, or output format.
2. When the user asks to read, summarize, extract, OCR, or reuse content from an existing workbook, call `file.preview` first; use `file.read` only for exact UTF-8 text ranges after previewing.
3. Create work under `tmp/<workbook-slug>` relative to the default writable workspace directory; do not use Blueclaw internal temporary paths.
4. For straightforward new workbooks, write a JSON spec and run `scripts/create_xlsx.py`.
5. For custom charts, advanced formulas, macros, or edits that exceed the JSON script, write a task-local Python file and run it through `scripts/skill_runtime.py python <file.py>`.
6. Use Python `csv` for `.csv` and `.tsv` parsing before writing workbook output.
7. Validate with `scripts/validate_xlsx.py` or by reopening the workbook and checking sheets, dimensions, formulas, frozen header rows, filters, and obvious formatting. Treat validation warnings as revision input; fix missing filters, missing frozen headers, blank headers, and broken formulas before attaching unless the user explicitly requested a raw dump.
8. Deliver accepted final spreadsheet files from `tmp/<workbook-slug>/build/` or the generated output path with `file.deliver`; deliver source CSVs only if requested.

Bundled scripts are responsible for their own Python dependencies. Run them through `scripts/skill_runtime.py`; the wrapper selects the built-in dependency environment first and prepares requester-owned fallback storage with `uv` only when needed. `/workspace/shared/cache/dependencies` is only a package cache. Do not run `pip install` directly. Do not call runtime paths outside `/workspace` directly. Do not stop at a missing-library error; run the helper script first. Do not use `python - <<'PY'` or system Python snippets for code that needs the Excel library.

Treat the supplied data as the source of truth. Preserve source-provided company names, product names, people, dates, amounts, IDs, and units exactly unless the user asks for translation or normalization. Do not invent missing vendors, prices, tax details, contact details, discounts, totals, or external context. When the source names a project, client, event, campaign, workbook title, or reporting period, put that name in a visible worksheet cell, not only in the filename or final message. When a useful field is missing, write the user's-language equivalent of "Not provided" instead of filling a plausible value.


## Saving and managing the document

Save the final workbook to `~/documents/<title>.xlsx` so it persists across tasks; run `mkdir -p ~/documents` once before writing there. `~` is the requester personal workspace and resolves the same way in a tool path field and in a shell command. To edit or delete a document the user names in a later task, list `~/documents/` (`ls ~/documents`) to find the file, then edit it in place or remove it with `file.delete`.

## Helper Scripts

For normal workbooks, create a spec file. Put the workbook or project title in `title`; the helper keeps it visible in the first sheet if the sheet spec does not already include it.

```json
{
  "title": "Jamsil Studio Pop-up Budget",
  "sheets": [
    {
      "title": "Summary",
      "heading": "Jamsil Studio Pop-up Budget",
      "freezePanes": "A3",
      "autoFilter": true,
      "columnWidths": {
        "A": 22,
        "B": 14,
        "C": 14,
        "D": 14
      },
      "numberFormats": {
        "B": "#,##0",
        "C": "#,##0",
        "D": "#,##0"
      },
      "rows": [
        ["Item", "Quantity", "Unit Price", "Total"],
        ["Plan", 2, 15000, "=B3*C3"]
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

For budget, operations, and finance workbooks, keep a short summary sheet and a separate detail sheet. Use formulas for line totals, grand totals, margins, percentages, and other values the user may need to adjust later. Also include the source total as a visible check value when the source provides one. Keep `autoFilter` enabled on every worksheet that has a row-and-column table, including two-column summary tables; set it to false only for a narrative note sheet with no table.

## Formulas

Write formulas only when the user needs the workbook to remain interactive. Avoid fragile formulas with hard-coded row ranges when a dynamic range is clearer. If the runtime cannot recalculate formulas, preserve formulas and verify that cell references are structurally correct. For row-level totals such as quantity times unit price, the formula cell should reference the same data row. When a sheet uses `heading`, the visible table header starts on row 2, so the first data row is row 3 and a first line-total formula usually looks like `=B3*C3`. Treat `offRowFormulaCount` from `validate_xlsx.py` as a real defect unless the formula intentionally references another sheet or an aggregate range.

The JSON helper repairs simple row-level formulas by default when a heading or title changes the row position, but do not rely on that as the only check. If validation reports off-row formulas, inspect the workbook spec or output and rerun generation after correcting the row references or setting an intentional cross-row formula in custom Python.

## CSV And TSV

Normalize tabular text before creating the workbook. Handle malformed rows explicitly. Do not silently drop columns or rows. If data cannot be repaired with confidence, create a separate `Issues` sheet listing the affected row numbers and reasons.

## Editing Existing Files

For user-provided workbooks:

1. Write a task-local edit script and run it through `scripts/skill_runtime.py python <file.py>`.
2. Preserve existing sheet names, formulas, styles, and macros unless the user asks otherwise.
3. Save to a new filename unless the user asks to replace the original.

Use macro-preserving load options for `.xlsm` files.

## Validation

Always run `scripts/validate_xlsx.py` through `scripts/skill_runtime.py` after saving. Read the JSON warnings and revise the workbook when they identify a real quality issue.

Check for broken references in formulas when formulas were added or moved. For financial or operational sheets, scan for blank required fields, suspicious totals, missing filters, missing frozen header rows, and values that do not match the supplied source data before attaching.
