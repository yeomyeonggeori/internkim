# Minimal Black-and-White DESIGN.md Reference

Use this as a reference for sober analytical decks. Do not copy it blindly; adapt the rationale, emphasis, and layout choices to the user's topic.

```yaml
---
colors:
  background: "#FAFAFA"
  surface: "#FFFFFF"
  ink: "#111111"
  muted: "#6B7280"
  accent: "#111111"
  line: "#D4D4D8"
typography:
  display: "Paperlogy, Noto Sans KR, system Korean sans"
  body: "Paperlogy, Noto Sans KR, system Korean sans"
layout:
  canvas: "16:9"
  margin: "64px"
  rhythm: "8px"
  radius: "8px"
---

# Visual Direction

Black-and-white analytical presentation with restrained lines, compact hierarchy, vendored Paperlogy typography, and generous whitespace. Use a thin top rule, conclusion-first titles, quiet cards, comparison columns, matrices, and recommendation blocks. Avoid decorative gradients, stock-like backgrounds, and bullet-only slides.
```

## Marp HTML Style Notes

- Keep `<!-- design-source: DESIGN.md -->` near the top of `presentation.md`.
- Put the vendored Paperlogy `@font-face` rules from `assets/webfonts.md` at the top of the Marp `style` block.
- Use `font-family: "Paperlogy", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, sans-serif;` for both slide body and display text unless the user requests another font.
- Keep the palette almost monochrome; reserve accent only for rules, tags, or one key number.
- Use semantic HTML containers inside each slide: `.frame`, `.eyebrow`, `.takeaway`, `.cards`, `.card`, `.comparison`, `.matrix`, `.timeline`, `.recommendation`.
- Keep radius at 8px or less unless the deck topic clearly asks for a softer visual language.
- Prefer one-sentence conclusions over topic labels. For example, write "Operational separation makes failures auditable" instead of "Architecture".
