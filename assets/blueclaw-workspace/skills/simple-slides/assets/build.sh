#!/bin/bash
# simple-slides build script
# Run ./build.sh from the same directory as presentation.md and extract_notes.py

set -e
cd "$(dirname "$0")"

# Source filename (set SRC=yourfile.md to override)
SRC="${SRC:-presentation.md}"
# Output name without extension (defaults to directory name)
NAME="${NAME:-$(basename "$(pwd)")}"

if [ ! -f "$SRC" ]; then
  echo "Error: $SRC not found. Create it from the template or set SRC=yourfile.md"
  exit 1
fi

if ! command -v marp &> /dev/null; then
  echo "Marp CLI is not installed in the Blueclaw runtime. Re-run local runtime setup."
  exit 1
fi

export CHROME_PATH="${CHROME_PATH:-/usr/bin/chromium}"
export PUPPETEER_EXECUTABLE_PATH="${PUPPETEER_EXECUTABLE_PATH:-$CHROME_PATH}"

# Strip trailing `---` separator(s) / blank lines — otherwise Marp renders
# an empty last slide. Non-destructive: only touches the trailing tail.
python3 - "$SRC" <<'PY'
import pathlib, sys
path = pathlib.Path(sys.argv[1])
text = path.read_text()
stripped = text.rstrip()
while stripped.endswith("---"):
    stripped = stripped[:-3].rstrip()
if stripped != text.rstrip():
    path.write_text(stripped + "\n")
PY

echo "Building HTML + PPTX + PDF..."
marp "$SRC" --html --allow-local-files -o "${NAME}.html"
marp "$SRC" --html --pptx --allow-local-files -o "${NAME}.pptx"
marp "$SRC" --html --pdf --allow-local-files -o "${NAME}.pdf"

echo "Embedding local images as base64 data URLs in HTML..."
python3 - "$NAME" <<'PY'
import base64, re, os, sys
name = sys.argv[1]
path = f"{name}.html"
with open(path) as f:
    html = f.read()
img_refs = set(re.findall(r'src="([^"]+\.(?:png|jpg|jpeg|gif|webp|svg))"', html, re.IGNORECASE))
mime_map = {'png': 'png', 'jpg': 'jpeg', 'jpeg': 'jpeg', 'gif': 'gif', 'webp': 'webp', 'svg': 'svg+xml'}
for img in sorted(img_refs):
    if img.startswith('data:') or img.startswith('http'):
        continue
    if not os.path.exists(img):
        print(f'  - skipped (not found): {img}')
        continue
    with open(img, 'rb') as f:
        data = base64.b64encode(f.read()).decode()
    ext = img.rsplit('.', 1)[1].lower()
    html = html.replace(f'src="{img}"', f'src="data:image/{mime_map[ext]};base64,{data}"')
    print(f'  - embedded {img}')
with open(path, 'w') as f:
    f.write(html)
PY

echo "Extracting speaker notes..."
if [ -f extract_notes.py ]; then
  python3 extract_notes.py "$SRC" "${NAME}-notes.txt"
else
  echo "  - extract_notes.py not found, skipping notes extraction"
fi

echo ""
echo "Done."
echo "  ${NAME}.html            (share this — images inlined, iframes need internet)"
echo "  ${NAME}.pptx            (PowerPoint / Keynote)"
echo "  ${NAME}.pdf             (PDF — iframes will appear blank, that's expected)"
echo "  ${NAME}-notes.txt       (speaker notes)"
