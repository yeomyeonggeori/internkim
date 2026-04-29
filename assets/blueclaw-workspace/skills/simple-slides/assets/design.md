---
name: "[Deck Title] Presentation System"
version: "1.0"
colors:
  background: "#FFFFFF"
  surface: "#FFFFFF"
  textPrimary: "#000000"
  textBody: "#000000"
  textMuted: "#666666"
  accent: "#000000"
  divider: "#CCCCCC"
typography:
  display:
    fontFamily: "Paperlogy"
    fontSize: "44px"
    fontWeight: 700
    lineHeight: 1.05
  alternateDisplay:
    fontFamily: "A2Z"
    fontSize: "44px"
    fontWeight: 700
    lineHeight: 1.05
  body:
    fontFamily: "Freesentation"
    fontSize: "22px"
    fontWeight: 400
    lineHeight: 1.35
  label:
    fontFamily: "Freesentation"
    fontSize: "14px"
    fontWeight: 600
    lineHeight: 1.2
fontSources:
  primaryImportURL: "https://cdn.jsdelivr.net/gh/fonts-archive/Paperlogy/Paperlogy.css"
  localFamilies: ["Freesentation", "A2Z"]
  fallbackStack: "Freesentation, Paperlogy, A2Z, \"Pretendard Variable\", Pretendard, \"Noto Sans KR\", \"Apple SD Gothic Neo\", \"Malgun Gothic\", sans-serif"
  cachePreference: "local-first"
spacing:
  unit: "8px"
  scale: ["8px", "16px", "24px", "32px", "48px", "64px"]
radii:
  card: "12px"
  pill: "999px"
components:
  cards: "outlined"
  tables: "plain"
  charts: "black-and-white"
motion:
  htmlSlides: "subtle"
---

# Design System

Copy this file before writing slides, then adapt the copy to the specific presentation tone. This is the deck's Stitch-compatible `DESIGN.md` template: machine-readable tokens in YAML front matter, followed by human-readable rationale. The default state is intentionally black on white. It is a neutral starting point, not a finished theme. After adapting it, propagate the values into Marp CSS.

## Overview

Describe the deck topic, audience, and visual direction in 2-3 sentences. Replace this neutral template with choices that fit the actual deck.

Example: "Editorial minimal for senior operators. Dense, restrained, and easy to scan, with one confident accent color and no decorative noise."

## Colors

Use the YAML color tokens above as the source of truth.

| Token | Hex | Use |
|---|---|---|
| `background` | `#FFFFFF` | Slide background |
| `surface` | `#FFFFFF` | Neutral card and panel surface |
| `textPrimary` | `#000000` | Titles, major numbers, high-emphasis labels |
| `textBody` | `#000000` | Body copy and table text |
| `textMuted` | `#666666` | Captions, metadata, footers |
| `accent` | `#000000` | Placeholder accent; replace when adapting the copy |
| `divider` | `#CCCCCC` | Rules, card borders, table dividers |

Accent rationale:

Template state: black on white. For a real deck, replace `accent`, `surface`, and text colors to match the desired tone before building.

Contrast requirements:

- Body text on background should meet WCAG AA.
- Accent text on background should meet WCAG AA when used for readable text.
- Do not place low-contrast muted text over filled cards.

## Typography

For Korean or bilingual decks, choose a Korean-capable family first. Modern Korean fonts usually cover Latin well enough for business slides, so a single Korean superfamily is often cleaner than mixing Latin display fonts with Korean body text.

Primary font import URL:

```text
https://cdn.jsdelivr.net/gh/fonts-archive/Paperlogy/Paperlogy.css
```

| Role | Family | Weight | Use |
|---|---|---|---|
| Display | Paperlogy | 700, 800 | Cover titles, section titles, big numbers |
| Alternate display | A2Z | 700, 800 | Tech, mobility, robotics, future-facing section titles |
| Body | Freesentation | 400, 500 | Paragraphs, cards, captions |
| Label | Freesentation | 600 | Eyebrows, table headers, chart labels |

Korean-first fallback:

```css
Freesentation, Paperlogy, A2Z, "Pretendard Variable", Pretendard, "Noto Sans KR", "Apple SD Gothic Neo", "Malgun Gothic", sans-serif
```

Other strong Korean candidates:

- Paperlogy: default display font; presentation-oriented, geometric, strong for slide titles.
- Freesentation: default body font; PowerPoint-heavy practical business decks.
- SUIT: polished UI/body text, good for product and dashboards.
- A2Z: default alternate display font for tech, mobility, and futuristic corporate decks.
- Wanted Sans: modern work/business tone.
- Noto Sans KR: safest installed fallback.
- Noto Serif KR or MaruBuri: formal/editorial decks.

## Layout

- Use an 8px base spacing unit.
- Keep slide-safe margins at 56-72px.
- Prefer 2-column or 3-column grids over dense freeform placement.
- Keep one main message per slide.
- Do not rely on viewport-responsive text sizing; Marp slides are fixed canvas outputs.

## Elevation & Depth

- Prefer flat surfaces, borders, and background contrast over heavy shadows.
- Use shadows only for modal-like overlays or intentionally layered HTML slides.
- Do not use decorative gradient blobs, bokeh, or floating orbs.

## Shapes

- Cards use the `card` radius token.
- Pills use the `pill` radius token.
- Tables use straight rules and restrained fills.
- Do not mix sharp editorial cards and soft rounded controls in the same deck.

## Components

- Cards: outlined or lightly filled, never nested.
- Pills: short metadata only, not full sentences.
- Tables: compact, readable, and no more than 5 columns unless split.
- Charts: one accent-led story, muted secondary series.
- Cover slide: strong title, concise subtitle, no decorative card around the main title.
- HTML slides: animations should be subtle and support comprehension.

## Do's and Don'ts

- Do copy this template into the deck workspace before `presentation.md`.
- Do adapt the copied `DESIGN.md` to the presentation tone before building.
- Do propagate these tokens into Marp CSS variables before building.
- Do update the copied `DESIGN.md` first when the user requests a visual redesign.
- Do use the accent color sparingly.
- Do choose Korean-capable fonts first when the deck contains Korean.
- Do keep Korean/CJK text readable with proper font fallback.
- Don't treat this black-on-white template as a finished visual theme.
- Don't use emoji or decorative icon characters unless explicitly requested.
- Don't use Google Slides as the default output; create HTML/PPTX/PDF first, then import if requested.
- Don't hardcode random hex values in slide content when a token exists.
