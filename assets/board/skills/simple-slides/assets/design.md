# Design Doc — [Deck Title]

Fill this in before writing slides. The choices here flow into `presentation.md`'s CSS frontmatter. If you later want to redesign the deck, update this doc first and propagate into the Marp CSS.

## Theme / Mood

One or two sentences describing the feel. Anchor to the topic and audience.

> Example: "Editorial minimal — like a long-read magazine feature. For a senior marketing audience who expects density and depth, not flash."

## Audience

Who's looking at this? Internal team, executives, external partners, conference? Affects formality, density, and visual restraint.

## Font pairing

Pick **one display font** (titles, headings, big numbers) and **one body font**. Use Google Fonts unless there's a specific brand font mandate — Google Fonts lets us `@import` with zero install and works across HTML/PDF.

Fill in:

| Role | Family | Weights |
|------|--------|---------|
| Display (titles, big numbers, labels) | e.g. Playfair Display | 700, 900 |
| Body (paragraphs, cards, captions) | e.g. Inter | 400, 500, 600 |

**Google Fonts import URL:**

```
https://fonts.googleapis.com/css2?family=Playfair+Display:wght@700;900&family=Inter:wght@400;500;600&display=swap
```

Generate this at [fonts.google.com](https://fonts.google.com/) — select families, weights, click "Get embed code", copy the `@import` URL.

**Why these two:** (brief note so future-you remembers)

## Color palette

Six colors total. More than that gets noisy.

| Role | Hex | Use |
|------|-----|-----|
| Background | `#FFFFFF` | Page background |
| Text primary | `#__` | Bold body, dark pills, card borders |
| Text body | `#__` | Regular body copy |
| Text muted | `#__` | Captions, footer, tertiary info |
| Accent | `#__` | Titles, highlights, labels, bars — only ONE |
| Divider | `#__` | Card internal dividers, table rules |

**Accent color rationale:** (why this color fits the topic)

**Tip:** test palette at [coolors.co](https://coolors.co/) or [realtimecolors.com](https://realtimecolors.com/) for contrast. Body text on background should hit WCAG AA (4.5:1).

## Component style

Decisions that affect how cards, pills, and layout blocks look:

- **Card style**: `outline` (border only, white fill) OR `filled` (light gray background, no border). Pick one.
- **Card border radius**: `__px` (12–20 is the normal range; higher = softer/friendlier, lower = sharper/editorial)
- **Pill shape**: `rounded` (pill-shape) OR `square` (small radius)
- **Use of accent bars**: yes / no — 3px horizontal accent bars as visual punctuation
- **Table style**: `dark-header` (solid dark bg on th) OR `plain` (underline only)

## Tone for copy

- Formality level: `formal / semi-formal / casual`
- Sentence length: `short & punchy / medium / long-form`
- Jargon tolerance: `assume expertise / explain terms`
- Emoji: `none / sparingly / expressive`

## Anything else

Brand-specific constraints, do-not-use colors, legal footer text, slide numbering preferences, etc.
