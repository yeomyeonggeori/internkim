---
name: share-file
description: "Share any file with the user via a direct download link. Use whenever a file has been created, generated, or downloaded — PDFs, images, CSVs, zips, etc. Always call this instead of printing a local file path."
---

# Sharing Files with the User

**Always share files via `/pico/files/` links. Never say "the file is saved at [path]" — always send a clickable link.**

The web server maps `/pico/files/` → `/root/.picoclaw/workspace/`. Any file in the workspace can be shared as a direct download link.

## How to share a file

- Image: `![alt](/pico/files/RELATIVE_PATH)`
- Any file (PDF, zip, csv, etc.): `[filename](/pico/files/RELATIVE_PATH)`

`RELATIVE_PATH` is the path relative to `/root/.picoclaw/workspace/`.

### Examples

| File location | Markdown link |
|---|---|
| `/root/.picoclaw/workspace/report.pdf` | `[report.pdf](/pico/files/report.pdf)` |
| `/root/.picoclaw/workspace/downloads/photo.jpg` | `![photo](/pico/files/downloads/photo.jpg)` |
| `/root/.picoclaw/workspace/data/export.csv` | `[export.csv](/pico/files/data/export.csv)` |

## Rules

- The moment a file is created or downloaded, share it with a `/pico/files/` link in the same reply.
- `/pico/files/` URLs are real, publicly accessible URLs — not local paths. The user can open them directly in their browser.
- NEVER output a raw file path like `/root/.picoclaw/workspace/business_plan.pdf`. Always convert it to a `/pico/files/` link.

## Downloading a file from a URL

Use the `download` tool (available in PATH):
```
download "https://example.com/file.pdf" ./downloads/file.pdf
```
Then immediately share: `[file.pdf](/pico/files/downloads/file.pdf)`
