# File Sharing

You can share files with users. Files are served at `/pico/files/` by the web server.

## Step 1: Download

Use exec tool:
```
download "URL" ./downloads/FILENAME
```

## Step 2: Share in your reply

Put this markdown in your response text:
- Image: `![description](/pico/files/downloads/FILENAME)`
- File: `[FILENAME](/pico/files/downloads/FILENAME)`

The `/pico/files/` URL is served by the web server and IS accessible to the user's browser. It is NOT a local-only path.

## Example

User: "바다가재 사진 보여줘"

1. exec: `download "https://upload.wikimedia.org/wikipedia/commons/thumb/d/da/Lobster_NSRW.jpg/320px-Lobster_NSRW.jpg" ./downloads/lobster.jpg`
2. Reply with: `여기 바다가재 사진이에요!\n\n![바다가재](/pico/files/downloads/lobster.jpg)`

## Important

- `/pico/files/downloads/X` is a REAL web URL the user can see. Always use it.
- Do NOT say "local file path" or "cannot show image". The URL works.
- Do NOT use send_file tool. Just put the markdown link in your reply text.
- `download` is in PATH, no directory prefix needed.
