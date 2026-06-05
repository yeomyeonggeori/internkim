# Simple Slides Webfonts

Use these when a deck needs Korean typography that will actually render. Put local `@font-face` rules at the top of the `slides.html` style block, before any CSS rules.

Do not write a custom font name unless the deck imports or embeds that font. Prefer vendored local fonts for finished artifacts so HTML/PDF/PPTX rendering does not depend on CDN access.

When embedding fonts, use WOFF2. Do not add TTF/OTF font files or non-WOFF2 fallbacks to generated decks unless the user supplies only those files and explicitly asks to use them. The Paperlogy block below is the canonical embedded font source.

## Paperlogy

Default for display text and body text. Good for title slides, section headers, analytical decks, and confident Korean writing.

```css
@font-face {
  font-family: "Paperlogy";
  src: url("/workspace/skills/simple-slides/assets/fonts/paperlogy/Paperlogy-4Regular.woff2") format("woff2");
  font-weight: 400;
  font-style: normal;
  font-display: swap;
}

@font-face {
  font-family: "Paperlogy";
  src: url("/workspace/skills/simple-slides/assets/fonts/paperlogy/Paperlogy-6SemiBold.woff2") format("woff2");
  font-weight: 600;
  font-style: normal;
  font-display: swap;
}

@font-face {
  font-family: "Paperlogy";
  src: url("/workspace/skills/simple-slides/assets/fonts/paperlogy/Paperlogy-7Bold.woff2") format("woff2");
  font-weight: 700;
  font-style: normal;
  font-display: swap;
}

@font-face {
  font-family: "Paperlogy";
  src: url("/workspace/skills/simple-slides/assets/fonts/paperlogy/Paperlogy-8ExtraBold.woff2") format("woff2");
  font-weight: 800;
  font-style: normal;
  font-display: swap;
}
```

```css
font-family: "Paperlogy", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji", sans-serif;
```

## Freesentation

Good for presentation body text, labels, captions, and compact Korean copy when the user explicitly asks for it and local access is available.

```css
@import url("https://cdn.jsdelivr.net/gh/fonts-archive/Freesentation/Freesentation.css");
```

```css
font-family: "Freesentation", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, sans-serif;
```

## Pretendard

Optional alternate when a familiar neutral sans-serif is explicitly requested.

```css
@import url("https://cdn.jsdelivr.net/gh/orioncactus/pretendard/dist/web/static/pretendard.css");
```

```css
font-family: "Pretendard", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, sans-serif;
```

## Noto Sans KR

Good as the safest alternate for Korean-heavy decks.

```css
@import url("https://fonts.googleapis.com/css2?family=Noto+Sans+KR:wght@400;500;600;700;800&display=swap");
```

```css
font-family: "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", sans-serif;
```

## Minimal Deck Pairings

- Korean analytical deck: Paperlogy headings and body.
- Dense business report: Paperlogy throughout, with tighter sizes and spacing.
- Explicit neutral alternate: Pretendard when the user asks for it.
- Conservative alternate: Noto Sans KR throughout.

Keep the visual system black-and-white first. Font choice should improve hierarchy and readability, not become decoration.
