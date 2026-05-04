---
name: "Stitch Presentation System"
version: "2.0"
colors:
  background: "#F8FAFC"
  surface: "#FFFFFF"
  ink: "#111827"
  muted: "#64748B"
  teal: "#0F766E"
  amber: "#F97316"
  line: "#CBD5E1"
typography:
  display: "Paperlogy, Freesentation, Pretendard, Noto Sans KR"
  body: "Freesentation, Pretendard, Noto Sans KR"
layout:
  canvas: "16:9"
  margin: "68px"
  rhythm: "8px"
  radius: "8px"
---

# Deck Design

Use this file as the source of truth before writing `presentation.md`. It is a Stitch-compatible design contract: YAML token front matter first, concise rationale below.

## Direction

Crisp Korean business slides with strong whitespace, structured cards, restrained color, and visible hierarchy. Avoid Marp default styling, black-and-white scaffold decks, raw brief text, and dense bullet dumping.

## Token Use

- `colors.background`: full slide background.
- `colors.surface`: cards and panels.
- `colors.ink`: titles and primary copy.
- `colors.muted`: captions and secondary copy.
- `colors.teal`: main accent and emphasis.
- `colors.amber`: sequence numbers and action highlights.
- `colors.line`: card borders and dividers.
- `typography.display`: cover titles, section titles, metrics.
- `typography.body`: body copy, cards, notes.
- `layout.margin`: left and right safe area.
- `layout.radius`: cards, bands, and pills.

## Rules

- Start each slide with one message.
- Use cards, grids, numbers, bands, and visual grouping before tables.
- Use tables only for true comparisons.
- Keep Korean lines short enough to avoid wrapping into dense blocks.
- Put `<!-- design-source: DESIGN.md -->` near the top of `presentation.md`.
