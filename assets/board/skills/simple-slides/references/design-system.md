# Picking a Design for a Deck

This is guidance for filling out `design.md` — how to choose fonts, colors, and component styles that fit the topic. The scaffold template is intentionally pure black-and-white with `system-ui` fonts; it's a placeholder, not a theme. Every deck should replace these values with intentional choices from `design.md`.

## Step 1: Match the mood to the topic

Before picking fonts or colors, name the mood in one sentence. Anchor to:

- **What's the topic?** Competitor analysis, creative brief, financial review, product launch, research summary, pitch deck — each has a different visual register
- **Who's looking?** Internal team tolerates more density and less polish; external/exec audiences want more breathing room and intentionality
- **What's the emotional beat?** Urgent / calm / celebratory / analytical / aspirational

From the mood, you can pick fonts and colors that reinforce it rather than fighting it.

Examples:

- "Editorial long-read for a marketing audience" → serif display + clean sans body, muted palette with one warm accent
- "Techy product launch" → geometric sans for titles + neutral sans body, cool palette with a saturated accent
- "Quarterly finance review for execs" → neutral sans throughout, restrained palette, no decorative accent color (or a very subtle one)
- "Creative agency pitch" → expressive display font + clean body, brighter palette, room for the accent to pop

## Step 2: Pick fonts

Use Google Fonts. Rationale: `@import` works across HTML and PDF output without install, and the catalog is large enough that you'll find something for any mood.

**Pairing principle:** one display font for titles + one neutral sans for body. Don't use three fonts; don't use one font for everything (feels flat at display sizes).

**Display font candidates** (for titles, big numbers, labels):

- **Geometric / modern sans** — Poppins, Manrope, Sora, Space Grotesk, Urbanist, Unbounded
- **Humanist / editorial sans** — Inter Tight, Plus Jakarta Sans, Figtree
- **Serif editorial** — Playfair Display, Fraunces, DM Serif Display, Bricolage Grotesque (expressive)
- **Condensed / poster** — Oswald, Bebas Neue, Archivo Narrow (high-impact titles)
- **Mono / technical** — JetBrains Mono, IBM Plex Mono (for dev/data topics, sparingly)

**Body font candidates** (for paragraphs, cards, captions):

- **Neutral sans** — Inter, DM Sans, Figtree, Plus Jakarta Sans, Work Sans
- **Reading-optimized** — Source Sans 3, Nunito Sans, IBM Plex Sans

**Typical good pairings:**

- Playfair Display + Inter (editorial)
- Poppins + DM Sans (minimal modern — the default template)
- Space Grotesk + Inter (techy)
- Fraunces + Figtree (warm editorial)
- Oswald + Work Sans (bold + neutral)
- Manrope + Manrope (single-font decks can work if you lean on weight contrast: 800 for titles, 400 for body)

**CJK / Korean content:** Latin-only fonts like Inter/Poppins don't
contain hangul/kanji/hanzi glyphs, so PDF rendering falls back to
whatever Chromium finds (tofu boxes if no CJK font is installed), and
Google Slides may substitute a font without CJK support. Pick a CJK
family as the body font (or as the sole font) when the deck has Korean,
Japanese, or Chinese text. All of the Korean fonts below are in Google
Fonts AND ship in Google Slides' built-in font picker, so PDF and
Slides stay consistent.

**Korean body / neutral sans (good default):**

- **Noto Sans KR** — universal default. Neutral, high legibility,
  weights 100–900. Works for any mood; never looks wrong.
- **IBM Plex Sans KR** — corporate/tech feel. Slightly more geometric
  than Noto. Matches IBM Plex Sans Latin for bilingual decks.
- **Nanum Gothic** — widely-used utilitarian Korean sans. Bureaucratic
  in a good way — government, finance, formal reports.
- **Gothic A1** — clean, modern sans with fine-grained weights
  (100–900). Slightly more expressive than Noto.

**Korean serif (editorial / formal):**

- **Noto Serif KR** — editorial long-read, heritage, literary.
  Weights 200–900.
- **Nanum Myeongjo** — traditional Korean serif. Feels older / more
  conservative. Good for legal, classical, memoir.
- **Gowun Batang** — modern serif with warm, calligraphic detailing.
  Softer than Noto Serif; works for wellness, editorial, humanistic.
- **Song Myung** — high-contrast serif with strong display
  presence. Use for titles at large size.
- **Diphylleia** — decorative serif; use sparingly for cover slides
  or pull quotes.

**Korean display / heavy headlines:**

- **Black Han Sans** — extremely heavy grotesque, single weight.
  Poster-level impact titles; unreadable at body sizes.
- **Do Hyeon** — bold geometric sans; poster feel, friendlier than
  Black Han Sans.
- **Jua** — rounded, playful heavy. Consumer / kids / casual.
- **Gugi** — brush-inspired heavy display. Creative / retail.
- **Stylish** — tall condensed display; editorial covers.

**Korean rounded / warm / casual:**

- **Gowun Dodum** — modern rounded sans, warm and soft. Wellness,
  consumer, education.
- **Sunflower** — light rounded sans; cheerful, informal.
- **Gamja Flower** — handwritten-style; children / diary / very
  casual. Use very sparingly.
- **Gaegu** — casual handwritten; similar niche.
- **Single Day** — geometric with rounded ends; light weight only.

**Korean handwriting / brush (display-only, sparing use):**

- **Nanum Pen Script** — thin pen handwriting.
- **Nanum Brush Script** — thick brush handwriting. Lifestyle,
  food, travel.
- **Kirang Haerang** — stylized handwriting.
- **Hi Melody** — childish marker feel.
- **Poor Story** — narrative handwriting.
- **Yeon Sung** — brush with pointed strokes.
- **Cute Font** — rounded informal.
- **East Sea Dokdo / Dokdo** — thick marker.

**Korean monospace:**

- **Nanum Gothic Coding** — Korean-capable code/data font. For dev
  topics, tables, inline code on slides.

**Japanese / Chinese** (add to `@import` alongside Korean if the
content is multilingual):

- **Noto Sans JP / Noto Serif JP** — Japanese counterparts.
- **Noto Sans SC / Noto Serif SC** — Simplified Chinese.
- **Noto Sans TC / Noto Serif TC** — Traditional Chinese.
- **M PLUS 1p, M PLUS Rounded 1c** — modern Japanese sans.
- **Sawarabi Gothic / Sawarabi Mincho** — Japanese serif/sans.
- **Zen Kaku Gothic New, Zen Maru Gothic** — Japanese modern sans.
- **ZCOOL XiaoWei, ZCOOL QingKe HuangYou** — Chinese display.
- **Long Cang, Ma Shan Zheng** — Chinese calligraphic.

**Typical CJK pairings (by mood):**

- **Default / safe Korean** — Noto Sans KR (both title & body, weight
  800 for title, 400 for body).
- **Editorial Korean** — Noto Serif KR (display) + Noto Sans KR (body).
- **Techy / product Korean** — IBM Plex Sans KR (both), or Space Grotesk
  (English titles only) + Noto Sans KR (body).
- **Warm / wellness Korean** — Gowun Dodum (both), or Gowun Batang
  (display) + Gowun Dodum (body).
- **Formal / heritage Korean** — Noto Serif KR (display) + Nanum Myeongjo
  (body), or Nanum Myeongjo both.
- **Pitch / consumer Korean** — Do Hyeon or Jua (display) + Noto Sans KR
  (body).
- **Creative / bold Korean** — Black Han Sans (display) + Gothic A1
  (body). Heavy impact.
- **Bilingual (KR + EN)** — Noto Sans KR for both (covers Latin too), or
  a Latin display (Poppins / Space Grotesk) with Noto Sans KR fallback.

**Mixing Latin + CJK in one run:** CSS `font-family` takes a list; the
browser picks the first family that has the needed glyph. For a Latin
display font with Korean fallback:

```css
--display-font: "Poppins", "Noto Sans KR", sans-serif;
--body-font: "Inter", "Noto Sans KR", sans-serif;
```

Make sure both families are in the `@import` URL. Google Fonts' @import
syntax for multiple CJK families looks like:

```
@import url('https://fonts.googleapis.com/css2?family=Noto+Sans+KR:wght@400;700&family=Noto+Serif+KR:wght@600&family=Inter:wght@400;600&display=swap');
```

Add `subset=korean` or set the `text=` parameter with only the glyphs
you use if you want to trim the download (optional; Noto KR is ~1MB
per weight by default).

## Google Fonts @import URL reference

All URLs use `https://fonts.googleapis.com/css2?family=<Name>[:wght@<weights>]&display=swap`.
Spaces in names become `+`. Multiple families join with `&family=`.
Weights are optional; omit for default regular.

**Common Korean families — copy-paste `@import` URLs:**

```
/* Noto Sans KR (200,400,700,900) */
@import url('https://fonts.googleapis.com/css2?family=Noto+Sans+KR:wght@200;400;700;900&display=swap');

/* Noto Serif KR (400,600,900) */
@import url('https://fonts.googleapis.com/css2?family=Noto+Serif+KR:wght@400;600;900&display=swap');

/* Nanum Gothic (400,700,800) */
@import url('https://fonts.googleapis.com/css2?family=Nanum+Gothic:wght@400;700;800&display=swap');

/* Nanum Myeongjo (400,700,800) */
@import url('https://fonts.googleapis.com/css2?family=Nanum+Myeongjo:wght@400;700;800&display=swap');

/* IBM Plex Sans KR (300,400,600,700) */
@import url('https://fonts.googleapis.com/css2?family=IBM+Plex+Sans+KR:wght@300;400;600;700&display=swap');

/* Gothic A1 (300,400,700,900) */
@import url('https://fonts.googleapis.com/css2?family=Gothic+A1:wght@300;400;700;900&display=swap');

/* Gowun Dodum (regular only) */
@import url('https://fonts.googleapis.com/css2?family=Gowun+Dodum&display=swap');

/* Gowun Batang (400,700) */
@import url('https://fonts.googleapis.com/css2?family=Gowun+Batang:wght@400;700&display=swap');

/* Black Han Sans (display only) */
@import url('https://fonts.googleapis.com/css2?family=Black+Han+Sans&display=swap');

/* Do Hyeon (display) */
@import url('https://fonts.googleapis.com/css2?family=Do+Hyeon&display=swap');

/* Jua (display) */
@import url('https://fonts.googleapis.com/css2?family=Jua&display=swap');

/* Song Myung (display serif) */
@import url('https://fonts.googleapis.com/css2?family=Song+Myung&display=swap');

/* Gugi (display brush) */
@import url('https://fonts.googleapis.com/css2?family=Gugi&display=swap');

/* Stylish (condensed display) */
@import url('https://fonts.googleapis.com/css2?family=Stylish&display=swap');

/* Sunflower (light rounded) */
@import url('https://fonts.googleapis.com/css2?family=Sunflower:wght@300;500;700&display=swap');

/* Diphylleia (decorative serif) */
@import url('https://fonts.googleapis.com/css2?family=Diphylleia&display=swap');

/* Nanum Pen Script (handwriting) */
@import url('https://fonts.googleapis.com/css2?family=Nanum+Pen+Script&display=swap');

/* Nanum Brush Script (brush) */
@import url('https://fonts.googleapis.com/css2?family=Nanum+Brush+Script&display=swap');

/* Nanum Gothic Coding (monospace Korean) */
@import url('https://fonts.googleapis.com/css2?family=Nanum+Gothic+Coding:wght@400;700&display=swap');

/* Gamja Flower (handwritten casual) */
@import url('https://fonts.googleapis.com/css2?family=Gamja+Flower&display=swap');

/* Gaegu (handwritten casual) */
@import url('https://fonts.googleapis.com/css2?family=Gaegu:wght@300;400;700&display=swap');

/* Hi Melody (childish marker) */
@import url('https://fonts.googleapis.com/css2?family=Hi+Melody&display=swap');

/* Kirang Haerang (handwriting) */
@import url('https://fonts.googleapis.com/css2?family=Kirang+Haerang&display=swap');

/* Yeon Sung (brush) */
@import url('https://fonts.googleapis.com/css2?family=Yeon+Sung&display=swap');

/* Single Day (light geometric) */
@import url('https://fonts.googleapis.com/css2?family=Single+Day&display=swap');

/* Cute Font (rounded informal) */
@import url('https://fonts.googleapis.com/css2?family=Cute+Font&display=swap');

/* Dokdo (thick marker) */
@import url('https://fonts.googleapis.com/css2?family=Dokdo&display=swap');

/* East Sea Dokdo (thick marker) */
@import url('https://fonts.googleapis.com/css2?family=East+Sea+Dokdo&display=swap');

/* Poor Story (narrative handwriting) */
@import url('https://fonts.googleapis.com/css2?family=Poor+Story&display=swap');
```

**Common Japanese / Chinese families:**

```
/* Noto Sans JP / Noto Serif JP */
@import url('https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@400;700&family=Noto+Serif+JP:wght@400;700&display=swap');

/* Noto Sans SC / TC — Simplified + Traditional Chinese */
@import url('https://fonts.googleapis.com/css2?family=Noto+Sans+SC:wght@400;700&family=Noto+Sans+TC:wght@400;700&display=swap');

/* M PLUS 1p / M PLUS Rounded 1c (Japanese modern) */
@import url('https://fonts.googleapis.com/css2?family=M+PLUS+1p:wght@400;700&family=M+PLUS+Rounded+1c:wght@400;700&display=swap');

/* Zen Kaku Gothic New / Zen Maru Gothic (Japanese) */
@import url('https://fonts.googleapis.com/css2?family=Zen+Kaku+Gothic+New:wght@400;700&family=Zen+Maru+Gothic:wght@400;700&display=swap');

/* Sawarabi Gothic / Mincho (Japanese) */
@import url('https://fonts.googleapis.com/css2?family=Sawarabi+Gothic&family=Sawarabi+Mincho&display=swap');

/* ZCOOL XiaoWei / QingKe HuangYou (Chinese display) */
@import url('https://fonts.googleapis.com/css2?family=ZCOOL+XiaoWei&family=ZCOOL+QingKe+HuangYou&display=swap');

/* Long Cang / Ma Shan Zheng (Chinese calligraphic) */
@import url('https://fonts.googleapis.com/css2?family=Long+Cang&family=Ma+Shan+Zheng&display=swap');
```

**Combining a Latin + Korean pair in one URL:**

```
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;600;800&family=Noto+Sans+KR:wght@400;700;900&display=swap');
```

Always combine all families into **one** `@import` — fewer HTTP
requests means faster Marp rendering. When in doubt about a font's
exact URL, visit `https://fonts.google.com/specimen/<Font+Name>`
and copy the generated `@import` snippet from the sidebar.

**Letter-spacing tip:** body sans often looks loose at slide sizes. Add `letter-spacing: -0.01em` or `-0.015em` globally to tighten. Display fonts usually need negative tracking too (`-0.02em` to `-0.04em`) especially at large sizes.

**Weights you need:**

- Display: one bold weight (700 or 800 or 900), optionally one medium for sub-display
- Body: 400 (regular), 600 (bold/emphasis), 500 optional for captions

Import only the weights you use — smaller `@import`, faster load.

## Step 3: Pick colors

Lock in 6 colors. Not more.

**Background**: **default to white (`#FFFFFF`) or off-white (`#FAFAFA`, `#F8F8F8`) on every deck unless the user explicitly asks for dark/black/neon/night/"dark mode"/"premium dark"/등**. Light backgrounds are the safe baseline — text contrast is easy, images and charts look clean, PDF prints well, CJK fonts render at their intended weight. Going dark without an explicit ask is a common self-sabotage: text colors and image backgrounds that look fine on white break on dark, legibility drops, and the user then has to ask for the revert. If the user says things like "프리미엄/고급스럽게/세련되게/멋있게/인상적으로" that means visually strong, NOT dark — keep the light background and reach for typography, layout, and accent color instead.

**Changing the background is never a one-line change.** If you swap the background (light → dark, or between two non-white shades), do NOT ship with only `backgroundColor: #XYZ` flipped. You must also re-audit and update, in the same edit:

- `--text-primary`, `--text-body`, `--text-muted` — flipped so they remain legible on the new background (e.g. `#E5E7EB` bodies on dark, `#1F2937` on light). Pure `#FFFFFF` on dark is too harsh; use soft whites like `#F8FAFC` or `#E5E7EB`.
- `--divider` — adjusted so it's visible without fighting content (e.g. `#334155` on dark, `#E5E7EB` on light).
- `--accent` — contrast-check against the new background. A deep navy accent that popped on white disappears on dark; shift to a lighter/saturated variant.
- Card fills (`--card-fill`) — invert or remove so cards don't become invisible rectangles (white cards on white background, or dark cards on dark background, both kill the shape).
- Chart colors — axis labels, grid lines, bar fills, SVG strokes. Any hex you hard-coded for the old background will likely blend into the new one. Inspect the rendered PNG to confirm every label is readable.
- Image borders / frames — images with white edges land visibly on dark, and vice versa. Either add a rounded border or crop cleanly.
- Shadow / glow effects — drop shadows that looked subtle on white become invisible on dark; replace with a soft top-border or skip entirely.
- Tofu risk for CJK — some CJK weights were chosen for light backgrounds; verify they still read cleanly on dark.

After the change, rebuild + `image_info` every content slide (not just the flagged ones) and walk through the "text legibility" checklist in SKILL.md §5.5. Shipping a dark-themed deck that looks right in the source but unreadable in the PNG is the single most common self-inflicted bug in this skill.

**Text primary** (bold text, card borders): deep not pure black. `#1A1A2E`, `#0F172A`, `#1C1917`, `#111827` all look more intentional than `#000000`.

**Text body** (paragraphs): one step lighter than primary. `#374151`, `#4B5563`, `#44403C`.

**Text muted** (captions, footer): two steps lighter. `#6B7280`, `#78716C`, `#94A3B8`.

**Accent**: ONE color. This is the personality of the deck. Pick based on mood:

- Trust / authority → deep blue (`#1E3A8A`, `#1E40AF`)
- Growth / fresh → green (`#059669`, `#10B981`, lime `#e2e562`)
- Energy / attention → orange (`#EA580C`, `#F97316`)
- Creative / bold → magenta, purple (`#BE185D`, `#7C3AED`)
- Warm / editorial → rust, amber (`#B45309`, `#D97706`)
- Calm / tech → cool blue (`#0284C7`, `#0EA5E9`)

Don't pick an accent you'd use in a logo — pick one that reads well as a TITLE color, because it'll be used for all the `h1`s. Test it at 80px: is it legible? Does it feel right?

**Divider**: one light neutral. `#E5E7EB`, `#D1D5DB`, `#E7E5E4`.

**Contrast check:** body text on background needs to hit WCAG AA (4.5:1). Use [webaim.org/resources/contrastchecker](https://webaim.org/resources/contrastchecker/) to verify.

## Step 4: Component style

Three main decisions:

**Card style**

- **Outline** (border-only, white fill) — editorial, minimal, prints cleanly to PDF. Good default.
- **Filled** (light gray background, no border) — softer, friendlier, can feel more like "chips" on a dashboard.
- **Dark filled** (dark bg, light text) — one or two per deck max, reserved for punchline/summary moments.

Stick to one primary style across the deck; use the other as a contrast.

**Border radius**

- 8–12px → sharper, editorial, serious
- 14–20px → friendlier, modern SaaS feel
- 24px+ → playful, consumer, warm

Match the radius to the mood. Consistent everywhere.

**Accent use**

- As titles only (conservative)
- As titles + small labels + bars (balanced — the default template)
- As titles + pills + highlights (bold — use when the topic calls for energy)

## Step 5: Write it down

Fill in `design.md`. Don't skip this — the doc is where you defend the choices to your future self when you come back to edit the deck in two weeks.

Then port the values into `presentation.md`'s frontmatter CSS. The frontmatter has clearly commented roles so you can find-replace cleanly.

## What NOT to do

- **Don't ship the scaffold.** The template is pure B&W with `system-ui` on purpose — it should look obviously unstyled. If a build ever comes out looking like that, you forgot to fill in the design doc and propagate it into the frontmatter.
- **Don't use multiple accents.** Pick one. If you need a second color for emphasis, use weight/size/position, not a new hue.
- **Don't use emoji as design elements.** Inconsistent across PDF and PPTX. Use real icons (inline SVG) if you need them.
- **Don't mix serif and sans within a text block.** Display = serif, body = sans is fine; but don't mix them in the same paragraph.
- **Don't use more than 6 colors.** Slide decks aren't webpages. Restraint reads as intentional.
- **Don't inherit from web branding.** Brand colors that work online often don't work at 80px on a white slide. Adapt.
