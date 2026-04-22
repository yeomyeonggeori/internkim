---
name: simple-slides
description: Generate clean, minimal presentation slides from content using Marp — produces standalone HTML (single file with base64-inlined images), PPTX, PDF, and speaker notes .txt in one build. Use this whenever the user asks for slides, a slide deck, a presentation, a brand monitoring report, a pitch deck, a research summary for stakeholders, a competitor analysis deck, or anything like that, even if they just say "make slides about X" without specifying Marp or any format. Also use when the user mentions Marp, PPTX, Keynote, or Google Slides. Optionally uploads the deck as a native Google Slides file via the gas-call bridge.
---

# Simple Slides

Create minimal, structured slide decks from content. Every deck starts with a short **design doc** that locks in fonts, colors, and tone — then slides are written to follow that design. This keeps the look consistent within a deck and avoids committee-designed one-offs.

Outputs: standalone HTML (images inlined as base64), PPTX (Keynote/PowerPoint compatible), PDF, and optionally a native Google Slides file in the user's Drive, all from one `make-slides` command.

## When to use

Any time the user needs slides — pitch deck, research summary, competitor analysis, brand report, internal report, workshop deck, lightning talk. Even if they never say "Marp", if they want slides, use this. Build all formats — don't ask upfront which one.

## Workflow

### 1. Set up the working directory

Pick a fresh directory under `/tmp/` tied to the topic (e.g., `/tmp/q4-review/`, `/tmp/celimax-monitoring/`). The agent has `file_write` and `shell` — use `file_write` to create the files below directly in that directory; do not try to copy from the skill's `assets/` via shell (the policy may block it).

The files you need to place in the deck directory:

- `design.md` — design doc (fill in first). Copy the structure from `assets/design.md` in this skill.
- `presentation.md` — the Marp source. Copy the structure from `assets/template.md` in this skill.

Images the deck needs go in the same directory.

### 2. Write the design doc FIRST

**Don't skip this.** Write `design.md` before touching slides. The doc defines:

- **Theme name + mood** — one sentence on the feel (e.g., "editorial / magazine", "techy minimal", "warm conversational")
- **Font pairing** — pick one display font for titles and one body font. Prefer Google Fonts so the CSS can `@import` them with zero install. Record the exact Google Fonts import URL.
- **Color palette** — background, text primary/secondary/muted, one accent color, dividers. Six colors total is plenty; don't exceed eight. **Default the background to white / off-white unless the user explicitly asked for a dark/black/premium-dark/night theme.** Words like "프리미엄하게 / 멋있게 / 세련되게" mean strong typography + layout + accent, NOT dark mode.
- **Component style** — cards as outline-only vs filled; border radius; pill shapes; whether to use a highlight color or not

See `references/design-system.md` in this skill for how to pick good fonts and palettes. The key idea: pick choices that fit the topic and audience, not defaults.

### 3. Port the design into the Marp frontmatter

Edit `presentation.md` so the `:root` CSS variables in the frontmatter reflect the design doc:

1. Uncomment the `@import url('...')` line and paste the Google Fonts URL from your design doc
2. Fill `--display-font` and `--body-font` with your chosen families (they're `system-ui` placeholders by default)
3. Fill the color variables — `--accent`, `--text-primary`, `--text-body`, `--text-muted`, `--divider`, `--bg`
4. Adjust `--card-radius`, `--card-fill`, `--pill-radius` if your design calls for it

The scaffold is intentionally pure black-and-white with `system-ui` fonts. It looks obviously unfinished — that's the point. If you ever see a deck looking like that at build time, you forgot to propagate the design doc into the frontmatter. **Always propagate.** The black-and-white scaffold is not a theme; it's a reminder.

### 4. Write the slides

Follow the layout patterns in `references/layouts.md` — they're expressed in CSS classes (`.card`, `.grid`, `.pill`, `.big`) so they work regardless of your specific color/font choices. The layouts give you 10+ concrete slide structures (cover, positioning, product grid, timeline, tables, video embeds, scoreboard, takeaways, etc.) — copy and fill in.

Content principles (apply regardless of design):

- **One message per slide.** If a slide has two ideas, split it.
- **Short lines.** If a bullet wraps to three lines, trim it.
- **Cards beat walls of text.** Use the `.card` pattern to visually chunk information.
- **Pair numbers with labels.** A big number alone is vague; give it context.
- **Speaker notes in HTML comments.** Use the `[Transition]` / `[Pause]` / `Key points:` / `Duration:` format.
- **No emoji, no decorative icons/characters** unless the user
  explicitly asked for them. `🚀`, `💼`, `✨`, `📊`, etc. are
  lazy substitutes for real design, and on any system missing the
  color-emoji font they render as tofu boxes (□). Lean on typography,
  color, and layout instead. The same applies to arrows like `→` only
  when they have semantic meaning; never as decoration.
- **No trailing separator.** `---` divides slides. A `---` at the very
  end of the file creates a blank last slide. End `presentation.md` with
  the final slide's content, not with a `---` line. The frontmatter opens
  with `---` and closes with `---`; those are different from slide
  dividers.

### 5. Build

Copy `assets/build.sh` and `scripts/extract_notes.py` into the deck
directory (`file_write` them with identical content, or `cp` from the
skill dir) and run:

```bash
cd /tmp/<deck-dir>
./build.sh
```

**Always go through `./build.sh`.** Do not call `marp` directly —
`build.sh` sets the `--html` flag (required for inline SVG, `<div>`
cards, charts, and any raw HTML you put in slides). Without `--html`,
Marp escapes those as literal text and your charts will render as
visible `<svg ...>` markup on the slide.

Produces:

- `<name>.html` — standalone, images base64-inlined, share as a single file
- `<name>.pptx` — opens in PowerPoint and Keynote natively
- `<name>.pdf` — for printing / archiving
- `<name>-notes.txt` — extracted speaker notes

Iterate: edit `presentation.md`, rerun `./build.sh`. Everything regenerates.

### 5.5. Visual review before shipping (applies to every build)

**This step runs every time you rebuild**, not just on the first
generation. Adding a chart, tweaking a layout, swapping a font, moving
text around — each of those changes can push something off the canvas.
If you called `marp` or `build.sh`, you also have to look at the
output before shipping.

**Every slide you edited this turn MUST be visually verified**, no
exceptions. The "trivial slide can skip" rule (cover, section divider,
single big number, thank-you) applies to slides you did NOT touch.
If your edit landed on a cover slide, inspect that cover slide.
If you changed `backgroundColor` or any global theme variable, every
content slide counts as "edited" because the global change cascades
into each of them. If you swapped a font, same thing. You cannot
assume an untouched slide stayed fine across a global edit, and you
cannot assume a touched slide came out right without looking.

Marp renders each slide at a fixed 1280×720 (16:9) or 960×720 (4:3)
canvas. Text that would render fine on a resizable web page can clip,
wrap weirdly, or spill off the edge here. You cannot predict overflow
from source length alone — font-size, letter-spacing, and CJK line
heights all shift what fits. When in doubt, inspect. The cost of
reviewing a slide that turned out fine is one extra `image_info` call;
the cost of shipping a broken deck is the user redoing your work.

**Charts and images are non-negotiable.** Any slide containing a
chart (CSS bars, inline SVG, QuickChart image, anything with a
`<svg>` / `viewBox` / chart image tag) or an embedded image
(`![...](...)`, `<img>`, iframe thumbnail, icon) must be visually
inspected on every build. Dimensions and aspect ratios interact with
the slide canvas in ways that are impossible to eyeball from the
source — the only way to know the visual didn't clip, overflow, or
sit at the wrong scale is to look at the rendered PNG.

**Always embed images as local files; never reference a remote URL
directly from `presentation.md`.** Remote URLs are flaky — the host
can rate-limit mid-render, the URL can die a week later, Chromium
needs network at build time, and the resulting deck breaks in
unpredictable ways. The rule is simple:

1. If the user provided a local file, use it by relative path
   (e.g. `![](./hero.jpg)`).
2. If the image is remote, download it first with `curl` into the
   deck directory, then reference the local copy.
3. If you cannot obtain a real image for that slide, drop the image.
   Do not invent a path and hope. Leave the slide text-only, or draw
   a CSS/SVG illustration you can generate deterministically.

Before every build, confirm each referenced file is actually on disk.
A missing file renders as a broken-image icon (or silently drops) in
the PNG output.

The default `./build.sh` already base64-inlines local images into the
HTML output, so `zeroclaw-v2.html` becomes a self-contained file.
For PPTX and PDF, Marp's Chromium bakes the local files in at render
time (that's why `--allow-local-files` is required and why the images
must be on disk). Remote URLs referenced directly skip both paths and
are brittle.

Image URLs that came from a previous tool call (QuickChart, web_search
results) still need to be downloaded locally before use — treat them
as the user gave you a link, not as a final embed.

**Render slide PNGs:**

```bash
cd /tmp/<deck-dir>
marp presentation.md --images png --allow-local-files -o <name>.png
# produces <name>.001.png, <name>.002.png, ...
```

**Which slides to inspect:**

Default to inspecting every content slide. Only skip a slide when
it's clearly trivial enough that overflow is impossible:

- Cover / title-only slide (single heading, maybe a one-line subtitle,
  no other content)
- Section-divider slide (single heading)
- Single big-number slide (one `big` number + short label)
- Closing "Thank you" / "Q&A" slide

Every other slide — anything with multiple bullets, a table, a chart,
cards, images, paragraphs, code, or custom CSS font-size overrides —
gets a visual check. A few examples:

- 3 short bullets at default font → inspect (wrap behavior on CJK is
  not predictable; font-size override might be in play)
- Title + one paragraph → inspect (paragraph wrap is unpredictable)
- Custom `<style>` block on the slide → always inspect
- A new layout pattern you haven't validated in this deck before →
  inspect, even if the content looks light

In practice that's most slides. That's fine; running `image_info` on
a handful of PNGs is cheap compared to shipping a broken deck.

**What to check per slide** (via `image_info` with `base64: true`):

- Edge clipping — any text cut off right or bottom
- Wrap lines pushing content past the visible area
- Placeholder tofu boxes (□) — font missing; pick a CJK-capable
  family (see `references/design-system.md`)
- Overlapping cards / elements
- Huge empty lower half — content is too sparse; consolidate or add
  a relevant visual
- Font obviously too small or too large for the canvas
- Images scaled past slide width
- **Text legibility** — you know what text is supposed to be on each
  slide; confirm it's actually visible in the rendered PNG. Common
  failures:
  - Text color matches or is too close to the background (white on
    white, dark-gray on black, accent color on a filled card of the
    same accent)
  - Text hidden behind an image, chart, overlay, or covering shape
  - Low contrast — e.g. `#888` body text on a `#0F172A` background,
    or `#bbb` caption on a light background — aim for WCAG AA
    (4.5:1) at body size, (3:1) at display size
  - Text inside a `<details>` / collapsed element that was supposed
    to be expanded
  - SVG/chart labels painted in the same color as the chart fill,
    disappearing into it
  - Inline SVG rendered as literal `<svg ...>` markup instead of the
    intended illustration (this means `--html` is missing; always go
    through `./build.sh`, never raw `marp`)
  If you see any of these, pick contrasting colors (usually the deck's
  text-primary / text-body variables are safe defaults) and rebuild.

If you spot something, fix `presentation.md`, rebuild, re-render the
affected PNG (or just the whole deck — it's fast), and re-inspect.
Common fixes:

- Title too long → shorten, or add `<br>` to break manually
- Bullet wraps to 3+ lines → tighten the wording; "하나의 메시지" 원칙
  대로 split
- Card grid overflows → `grid-3` → `grid-2`, or move one card to a
  new slide
- Body overflows bottom → cut to 3–4 bullets, or split the slide
- Chart clipped → shrink `viewBox` width or cap SVG `max-width`
- Font too small → raise body font-size, don't try to fit more

### 5.55. Updating an existing deck (do not make a new file)

When the user says "수정해줘", "다시 만들어봐", "v2 더 멋지게",
"차트 더 크게" — or otherwise refers to a deck you already produced —
**update the existing Google Slides file in place**. Do not call
`skills/simple-slides/scripts/gas-call drive.import_pptx` again; that uploads the pptx as a new
file with a new URL and forces the user to re-share, re-bookmark,
re-open. Same for `skills/simple-slides/scripts/gas-call slides.create`.

Update flow:

1. Retrieve the existing file ID — `memory_recall` first, then fall
   back to the session log if needed.
2. Rebuild your `.pptx` from the edited `presentation.md`.
3. Replace the Drive file's contents while keeping its ID and URL:

   ```bash
   gws-bot drive files update \
     --params '{"fileId":"<ID>","supportsAllDrives":true}' \
     --upload ./kim_intern_v3.pptx
   ```

   Google auto-converts the uploaded pptx back into the existing
   Slides file. The URL does not change.

4. Tell the user the link is the same one as before (so they know
   not to re-open), and summarise what changed.

A truly new deck should only be created when the user explicitly asks
for a new one ("새 덱 만들어", "별도 파일로") or when the topic is
unrelated to the prior deck.

**Do not tell the user Google Slides cannot be updated.** It can.
The `gws-bot drive files update --params '{"fileId":"..."}' --upload <pptx>`
command works and you have used it before. If it returns an error,
paste the literal error and retry; do not give up and fall back to
local-file-only delivery while pretending the Slides update is
impossible.

### 5.6. Upload + remember (mandatory before delivery)

The URL you hand the user must be **the URL `skills/simple-slides/scripts/gas-call drive.import_pptx`
actually returned**, not a placeholder, not a "(가상 링크)" annotation,
not a made-up format like `1Xy_Jv9e-p8N-...`. If the user asked for a
Google Slides link and you haven't run `skills/simple-slides/scripts/gas-call drive.import_pptx`
yet, you don't have a link to give. Run it. Parse the JSON. Use the
exact `url` field. Never fabricate.

Immediately after the upload succeeds, call `memory_store` so you (or
a future turn) can recall this deck:

```
memory_store(
  key="deck_<shortslug>_<YYYYMMDD>",
  content="Google Slides URL: <url>\nID: <id>\nTitle: <title>\nLocal pptx: <abs-path>\nTopic: <one-line>"
)
```

Key examples: `deck_zeroclaw_v2_20260421`, `deck_q4_review_20260420`.

This is non-negotiable. When the user later says "방금 만든 거
수정해줘" or "아까 그 덱 슬라이드 하나 추가해줘", you retrieve the
URL / id from memory and update that file in place (see section 5.55
above) rather than creating a fresh deck.

### 6. Deliver to the user

Deliver **only** the format the user asked for. Do not bundle
pptx + pdf + html + Slides together by default — that creates
clutter. Build all formats locally (cheap, already handled by
`./build.sh`), but send only what was requested.

Matching:

- **"Google Slides" / "슬라이드 링크" / "구글 슬라이드"** — upload the
  `.pptx` and share the native Slides URL:
  ```bash
  skills/simple-slides/scripts/gas-call drive.import_pptx title="<Deck Title>" pptx=./<name>.pptx
  ```
  Prints `{"id":"...","url":"https://docs.google.com/presentation/d/.../edit"}`.
  Share just the URL. Do not also attach the pptx/pdf/html.
- **"PowerPoint" / "PPTX" / "Keynote"** — `send-file ./<name>.pptx <name>.pptx`. Just the pptx.
- **"PDF"** — `send-file ./<name>.pdf <name>.pdf`. Just the pdf.
- **"HTML" / "link I can share"** — `send-file ./<name>.html <name>.html`. Just the html.
- **"전부 다" / "all formats"** — only then send everything.
- **Ambiguous ("make me a deck about X")** — default to a Google
  Slides URL only (the most common ask). If the user then wants
  another format, send only that.

Don't claim the deck is done before `./build.sh` (or `gas-call`) finished
without error.

Never fall back to `skills/simple-slides/scripts/gas-call slides.create` or
`gws-bot slides presentations batchUpdate` — those produce empty shells.
Don't run `base64`, `$(cat ...)`, or compose your own curl for the
upload. `skills/simple-slides/scripts/gas-call drive.import_pptx pptx=<path>` handles the encoding.

**Never link a local file as `sandbox:/tmp/...`, `file:///tmp/...`, or
a plain local path in your Mattermost reply.** Those links are dead
for the user; they resolve to nothing on the user's machine. To
deliver a local file you must upload it via the `send-file` shell
command (this is what the `share-file` skill exists for):

```bash
send-file /tmp/kim-intern-v2/kim-intern-v2.pptx kim-intern-v2.pptx "여기 있어요"
```

Run it for each format you're delivering (pptx/pdf/html). The user
gets the file as a Mattermost attachment. Only after `send-file`
prints a success line should your reply reference those files.

**In your delivery message, always ask the user to flag any slide
that looks off.** You didn't inspect every slide visually, so
something could still be wrong. Phrase it so the user knows a quick
re-run is cheap. Example (Korean):

> 초안 완성됐습니다 — [링크 / 첨부].
> 혹시 레이아웃이 깨지거나 어색한 슬라이드가 있다면 번호 알려주세요,
> 해당 슬라이드만 다시 다듬어 드릴게요.

Keep the ask short; don't apologize or over-explain. Just invite the
feedback.

## Video / iframe embeds

Instagram/TikTok/YouTube iframes render in the HTML output (live playback) but NOT in PDF — Chromium's `printToPDF` refuses to render iframes. This is upstream, nothing to fix in Marp.

Two strategies:

- **HTML-primary** — use `<iframe>` embeds. PDF will show blank frames, which is fine for a presentation viewed on screen.
- **PDF-safe** — replace each iframe with a thumbnail image wrapped in a link: `[![cover](thumb.jpg)](https://...)`. Clicks open the source in the browser. Use this when PDF distribution matters.

For TikTok, short URLs like `lite.tiktok.com/t/XXXX/` need to resolve to video IDs before embedding. Embed format: `https://www.tiktok.com/embed/v2/{VIDEO_ID}`.

## Reference files

Read these as needed — don't front-load all of them. They live inside this skill's directory (`/home/zeroclaw/.zeroclaw/workspace/skills/simple-slides/...`).

- `references/design-system.md` — How to pick fonts, colors, and component styles for a new deck. Read before writing the design doc.
- `references/layouts.md` — Concrete slide patterns with HTML/class examples. Read while writing slides.
- `assets/design.md` — Fill-in template for the design doc.
- `assets/template.md` — Marp starter with the default design. Use as reference for `presentation.md`.

## Iteration patterns

Common user requests and where to change them:

- **"Make the accent color X"** — find-replace the accent hex in `presentation.md` (frontmatter + any inline styles) and in `design.md` for reference
- **"Try a different font"** — swap the `@import` URL and the font-family values. Save the old choice in `design.md` comments so you remember what was tried.
- **"Add a slide about Y"** — drop in a new slide block between two `---` separators, following one of the layout patterns
- **"Remove the Z section"** — delete the slide block
- **"Make it more formal / more playful"** — this is a design-doc-level change; open `design.md`, revise the tone/palette/font, then port the changes into the frontmatter

When design-level changes are requested, update `design.md` first so the doc stays the source of truth, then mirror into the Marp CSS.
