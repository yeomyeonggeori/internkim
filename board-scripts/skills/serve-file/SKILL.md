# File Sharing

Share any file with the user via markdown link. The web server serves `/pico/files/` → `/root/.picoclaw/workspace/`.

## How to share a file

Just put a markdown link in your reply. The user's browser can open it directly.

- Image: `![alt](/pico/files/RELATIVE_PATH)`
- Any file: `[FILENAME](/pico/files/RELATIVE_PATH)`

`RELATIVE_PATH` is relative to `/root/.picoclaw/workspace/`.

### Examples

| File location | Link |
|---|---|
| `/root/.picoclaw/workspace/sample.pdf` | `[sample.pdf](/pico/files/sample.pdf)` |
| `/root/.picoclaw/workspace/downloads/photo.jpg` | `![photo](/pico/files/downloads/photo.jpg)` |

## Downloading from a URL

Use the `download` tool (in PATH):
```
download "https://example.com/file.pdf" ./downloads/file.pdf
```
Then share: `[file.pdf](/pico/files/downloads/file.pdf)`

## CRITICAL RULES

- ALWAYS share files this way. NEVER say "the file is saved locally" or "I cannot send files".
- If a file already exists in the workspace, share it immediately with a `/pico/files/` link.
- `/pico/files/` URLs are real, publicly accessible URLs — not local paths.
