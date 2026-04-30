---
name: simple-slides
description: Generate clean, minimal presentation slides from content using Marp — produces standalone HTML (single file with base64-inlined images), PPTX, PDF, and speaker notes .txt in one build. Use this whenever the user asks for slides, a slide deck, a presentation, a brand monitoring report, a pitch deck, a research summary for stakeholders, a competitor analysis deck, or anything like that, even if they just say "make slides about X" without specifying Marp or any format. Also use when the user mentions Marp, PPTX, Keynote, or Google Slides. Optionally uploads the deck as a native Google Slides file via the gas-call bridge only when Google output is requested.
---

# Simple Slides

Create minimal, structured slide decks from content. Every deck starts with a Stitch-compatible **DESIGN.md** that locks in design tokens, rationale, fonts, colors, spacing, components, and visual constraints — then slides are written to follow that design. This keeps the look consistent within a deck and avoids committee-designed one-offs.

Outputs: standalone HTML (images inlined as base64), PPTX (Keynote/PowerPoint compatible), PDF, and optionally a native Google Slides file in the user's Drive when requested, all from one `make-slides` command.

## When to use

Any time the user needs slides — pitch deck, research summary, competitor analysis, brand report, internal report, workshop deck, lightning talk. Even if they never say "Marp", if they want slides, use this. Build all formats — don't ask upfront which one.

## Workflow

### 1. Set up the working directory

Pick a fresh directory under `/tmp/` tied to the topic (e.g., `/tmp/q4-review/`, `/tmp/celimax-monitoring/`). The agent has `file_write` and `shell` — use `file_write` to create the files below directly in that directory; do not try to copy from the skill's `assets/` via shell (the policy may block it).

The files you need to place in the deck directory:

- `DESIGN.md` — Stitch-compatible design system doc. Start from the black-on-white template in `assets/design.md`, then adapt the copy to the deck's actual tone before writing slides.
- `presentation.md` — the Marp source. Copy the structure from `assets/template.md` in this skill.

Images the deck needs go in the same directory.

### 2. Write DESIGN.md FIRST

**Don't skip this.** Write `DESIGN.md` before touching slides. The skill's `assets/design.md` is a neutral black-on-white template, not a finished theme. Copy it, then change the copied `DESIGN.md` to fit the deck's audience, mood, topic, and output format. Treat the adapted file like `AGENTS.md` for visual design: persistent, agent-readable context that the deck must obey. Use the Stitch-style structure: YAML front matter for machine-readable tokens, then Markdown rationale and constraints. The doc defines:

- **YAML tokens** — colors, typography, spacing, radii, components, and motion rules in exact values.
- **Overview** — product/topic, audience, and visual direction in 2-3 sentences.
- **Colors** — semantic roles with hex values and usage rules.
- **Typography** — display/body/label families, sizes, weights, line heights, and CJK fallback.
- **Layout** — spacing scale, grid, density, slide-safe margins.
- **Elevation & Depth** — shadows or flat-surface rules.
- **Shapes** — radius and border rules.
- **Components** — cards, pills, tables, charts, callouts, cover treatments.
- **Do's and Don'ts** — explicit design guardrails, including what not to use.

See `references/design-system.md` in this skill for how to pick good fonts and palettes. The key idea: pick choices that fit the topic and audience, not defaults.

### 3. Port the design into the Marp frontmatter

Edit `presentation.md` so the `:root` CSS variables in the frontmatter reflect `DESIGN.md`:

1. Replace the default `@import url('...')` line with `DESIGN.md`'s `fontSources.primaryImportURL` when the font changes
2. Fill `--display-font` and `--body-font` with the tokenized families from `DESIGN.md`
3. Fill the color variables — `--accent`, `--text-primary`, `--text-body`, `--text-muted`, `--divider`, `--bg`
4. Adjust `--card-radius`, `--card-fill`, `--pill-radius`, shadows, and chart colors from `DESIGN.md`

The scaffold is intentionally pure black-and-white, with Paperlogy/Freesentation/A2Z only as Korean-safe default fonts. It looks visually unfinished — that's the point. If you ever see a deck looking like that at build time, you forgot to propagate `DESIGN.md` into the frontmatter. **Always propagate.** The black-and-white scaffold is not a theme; it's a reminder.

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
HTML output, so `blueclaw-v2.html` becomes a self-contained file.
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
**update the existing Google Slides file in place**. Re-uploading the
pptx as a new Drive file gives the user a new URL and forces them to
re-share, re-bookmark, re-open.

Update flow:

1. Retrieve the existing file ID from the current conversation context or
   prior successful tool observations.
2. Rebuild your `.pptx` from the edited `presentation.md`.
3. Replace the Drive file's contents while keeping its ID and URL:

   ```bash
   gws drive files update \
     --params '{"fileId":"<ID>","supportsAllDrives":true}' \
     --upload ./kim_intern_v3.pptx
   ```

   Google auto-converts the uploaded pptx back into the existing
   Slides file. The URL does not change.

4. Tell the user the link is the same one as before, and summarise
   what changed.

A truly new deck should only be created when the user explicitly asks
for a new one ("새 덱 만들어", "별도 파일로").

### 5.6. Upload + remember (mandatory before delivery)

For a brand-new deck the user wants in Google Slides, upload via
`gws drive files create --convert`:

```bash
gws drive files create \
  --params '{"supportsAllDrives":true}' \
  --json '{"name":"Deck Title","mimeType":"application/vnd.google-apps.presentation"}' \
  --upload ./<name>.pptx
```

Returns `{"id":"...","webViewLink":"..."}`. The `webViewLink` is the
Google Slides URL. Share that — never fabricate a URL and never hand
over a placeholder.

Immediately after the upload succeeds, include the Google Slides URL,
file ID, and title in the tool result or working notes so later turns can
recover the deck from prior successful observations.

When the user later says "방금 만든 거 수정해줘" or "아까 그 덱 슬라이드
하나 추가해줘", retrieve the URL / id from the current conversation context or prior successful tool observations and
update that file in place (see section 5.55 above) rather than creating
a fresh deck.

### 6. Deliver to the user

Deliver **only** the format the user asked for. Build all formats
locally (cheap, handled by `./build.sh`), but send only what was
requested.

Matching:

- **"Google Slides" / "슬라이드 링크" / "구글 슬라이드"** — upload via
  `gws drive files create --convert=true` (section 5.6) and share the
  returned `webViewLink` URL. Do not also attach the pptx/pdf/html.
- **"PowerPoint" / "PPTX" / "Keynote"** — deliver the generated pptx only if the file is available as a native attachment.
- **"PDF"** — deliver the generated pdf only if the file is available as a native attachment.
- **"HTML" / "link I can share"** — deliver the generated html only if the file is available as a native attachment or a real share URL.
- **"전부 다" / "all formats"** — only then send everything.
- **Ambiguous ("make me a deck about X")** — default to portable
  HTML/PPTX/PDF outputs. Upload to Google Slides only when the user asks
  for Google Slides, a share URL backed by Google, or collaborative
  editing.

Don't claim the deck is done before `./build.sh` (or the upload)
finished without error.

**Never link a local file as `sandbox:/tmp/...`, `file:///tmp/...`, or
a plain local path in a platform reply.** Those links are dead for the
user; they resolve to nothing on the user's machine. Only say that a
deck file is attached when the tool result actually produced a native
attachment for final reply delivery.

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

Read these as needed — don't front-load all of them. They live inside this skill's directory (`/root/.blueclaw/workspace/skills/simple-slides/...`).

- `references/design-system.md` — How to pick fonts, colors, and component styles for a new deck. Read before writing `DESIGN.md`.
- `references/layouts.md` — Concrete slide patterns with HTML/class examples. Read while writing slides.
- `assets/design.md` — Fill-in template for Stitch-compatible `DESIGN.md`.
- `assets/template.md` — Marp starter with the default design. Use as reference for `presentation.md`.

## Iteration patterns

Common user requests and where to change them:

- **"Make the accent color X"** — update the token in `DESIGN.md`, then find-replace the accent hex in `presentation.md` (frontmatter + any inline styles).
- **"Try a different font"** — update typography tokens in `DESIGN.md`, then swap the `@import` URL and font-family values. Save the old choice in `DESIGN.md` comments so you remember what was tried.
- **"Add a slide about Y"** — drop in a new slide block between two `---` separators, following one of the layout patterns
- **"Remove the Z section"** — delete the slide block
- **"Make it more formal / more playful"** — this is a design-system-level change; open `DESIGN.md`, revise the tone/palette/font, then port the changes into the frontmatter.

When design-level changes are requested, update `DESIGN.md` first so the doc stays the source of truth, then mirror into the Marp CSS.
