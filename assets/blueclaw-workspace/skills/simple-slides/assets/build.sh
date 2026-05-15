#!/bin/bash
set -e
SCRIPT_DIRECTORY="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

SRC="${SRC:-presentation.md}"
NAME="${NAME:-$(basename "$(pwd)")}"
FORMATS="${FORMATS:-html,pptx,pdf,notes,review}"

if [ ! -f "$SRC" ]; then
  echo "Error: $SRC not found. Create presentation.md or set SRC=yourfile.md"
  exit 1
fi

if [ ! -f DESIGN.md ]; then
  echo "Error: DESIGN.md not found. Create Stitch-compatible DESIGN.md before building."
  exit 1
fi

python3 - "$SRC" <<'PY'
import pathlib
import sys

source_path = pathlib.Path(sys.argv[1])
design_path = source_path.with_name("DESIGN.md")
text = source_path.read_text()
if "design-source: DESIGN.md" not in text:
    print("Error: presentation.md must include design-source: DESIGN.md")
    sys.exit(1)

source_modified_at = source_path.stat().st_mtime
design_modified_at = design_path.stat().st_mtime
if source_modified_at + 1 < design_modified_at:
    print(f"Error: {source_path.name} is older than DESIGN.md. Update the deck source before building.")
    sys.exit(1)
PY

SKILL_ASSET_DIRECTORY="${SKILL_ASSET_DIRECTORY:-/workspace/skills/simple-slides/assets}"
if [ ! -f "${SKILL_ASSET_DIRECTORY}/package.json" ] && [ -f "${SCRIPT_DIRECTORY}/package.json" ]; then
  SKILL_ASSET_DIRECTORY="$SCRIPT_DIRECTORY"
fi
EXTRACT_NOTES_SCRIPT="${EXTRACT_NOTES_SCRIPT:-/workspace/skills/simple-slides/scripts/extract_notes.py}"
RENDER_REVIEW_SCRIPT="${RENDER_REVIEW_SCRIPT:-/workspace/skills/simple-slides/scripts/render_review.py}"
if [ ! -f "$EXTRACT_NOTES_SCRIPT" ] && [ -f "${SCRIPT_DIRECTORY}/../scripts/extract_notes.py" ]; then
  EXTRACT_NOTES_SCRIPT="${SCRIPT_DIRECTORY}/../scripts/extract_notes.py"
fi
if [ ! -f "$RENDER_REVIEW_SCRIPT" ] && [ -f "${SCRIPT_DIRECTORY}/../scripts/render_review.py" ]; then
  RENDER_REVIEW_SCRIPT="${SCRIPT_DIRECTORY}/../scripts/render_review.py"
fi
NODE_RUNTIME_ROOT="${BLUECLAW_REQUESTER_TMP:-$(pwd)}/.skill-env/simple-slides/node"

ensure_local_marp() {
  if ! command -v npm &> /dev/null; then
    echo "Marp CLI is not available and npm is not present for script-managed bootstrap."
    exit 1
  fi
  if [ ! -f "${SKILL_ASSET_DIRECTORY}/package.json" ]; then
    echo "Marp package manifest is missing: ${SKILL_ASSET_DIRECTORY}/package.json"
    exit 1
  fi
  mkdir -p "$NODE_RUNTIME_ROOT"
  if [ -d /workspace/shared/cache/dependencies ]; then
    mkdir -p /workspace/shared/cache/dependencies/npm
    export npm_config_cache=/workspace/shared/cache/dependencies/npm
  fi
  if [ ! -f "${NODE_RUNTIME_ROOT}/package.json" ] || ! cmp -s "${SKILL_ASSET_DIRECTORY}/package.json" "${NODE_RUNTIME_ROOT}/package.json" || [ ! -x "${NODE_RUNTIME_ROOT}/node_modules/.bin/marp" ]; then
    cp "${SKILL_ASSET_DIRECTORY}/package.json" "${NODE_RUNTIME_ROOT}/package.json"
    npm install --prefix "$NODE_RUNTIME_ROOT" --omit=dev --no-audit --no-fund
  fi
  MARP_COMMAND=("${NODE_RUNTIME_ROOT}/node_modules/.bin/marp")
}

if command -v marp &> /dev/null; then
  MARP_COMMAND=(marp)
else
  ensure_local_marp
fi

run_marp() {
  "${MARP_COMMAND[@]}" "$@"
}

format_enabled() {
  case ",${FORMATS}," in
    *",$1,"*) return 0 ;;
    *) return 1 ;;
  esac
}

export CHROME_PATH="${CHROME_PATH:-/usr/bin/chromium}"
export PUPPETEER_EXECUTABLE_PATH="${PUPPETEER_EXECUTABLE_PATH:-$CHROME_PATH}"

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

rm -f "${NAME}.html" "${NAME}.pptx" "${NAME}.pdf" "${NAME}-notes.txt"

echo "Building requested formats: ${FORMATS}"
if format_enabled html; then
  run_marp "$SRC" --html --allow-local-files -o "${NAME}.html"
fi
if format_enabled pptx; then
  run_marp "$SRC" --html --pptx --allow-local-files -o "${NAME}.pptx"
fi
if format_enabled pdf; then
  run_marp "$SRC" --html --pdf --allow-local-files -o "${NAME}.pdf"
fi

if format_enabled html; then
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
fi

if format_enabled notes; then
  echo "Extracting speaker notes..."
  if [ -f "$EXTRACT_NOTES_SCRIPT" ]; then
    python3 "$EXTRACT_NOTES_SCRIPT" "$SRC" "${NAME}-notes.txt"
  else
    echo "  - extract_notes.py not found, skipping notes extraction"
  fi
fi

if format_enabled review; then
  echo "Rendering slide review images..."
  mkdir -p review
  rm -f "review/${NAME}"*.png review/slide-review.json review/slide-review.md
  run_marp "$SRC" --images png --allow-local-files -o "review/${NAME}.png"
  if [ -f "$RENDER_REVIEW_SCRIPT" ]; then
    if ! python3 "$RENDER_REVIEW_SCRIPT" "$SRC" "$NAME" review; then
      echo "  - slide render review reported warnings; see review/slide-review.json"
    fi
  else
    echo "  - render_review.py not found, skipping slide render review"
  fi
fi

echo ""
echo "Done."
if format_enabled html; then echo "  ${NAME}.html            (share this — images inlined, iframes need internet)"; fi
if format_enabled pptx; then echo "  ${NAME}.pptx            (PowerPoint / Keynote)"; fi
if format_enabled pdf; then echo "  ${NAME}.pdf             (PDF — iframes will appear blank, that's expected)"; fi
if format_enabled notes; then echo "  ${NAME}-notes.txt       (speaker notes)"; fi
if format_enabled review; then echo "  review/slide-review.json (per-slide render review)"; fi
