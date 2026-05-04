# Simple Slides Webfonts

Use these when a deck needs Korean typography that will actually render. Put imports at the top of the Marp `style` block in `presentation.md`, before any CSS rules.

Do not write a custom font name unless the deck imports or embeds that font. If external network access is not appropriate for the output, use the system stack instead.

## Paperlogy

Good for display text, title slides, section headers, and confident Korean headlines.

```css
@import url("https://cdn.jsdelivr.net/gh/fonts-archive/Paperlogy/Paperlogy.css");
```

```css
font-family: "Paperlogy", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, sans-serif;
```

## Freesentation

Good for presentation body text, labels, captions, and compact Korean copy.

```css
@import url("https://cdn.jsdelivr.net/gh/fonts-archive/Freesentation/Freesentation.css");
```

```css
font-family: "Freesentation", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, sans-serif;
```

## Pretendard

Good for quiet business decks when a familiar neutral sans-serif is preferred.

```css
@import url("https://cdn.jsdelivr.net/gh/orioncactus/pretendard/dist/web/static/pretendard.css");
```

```css
font-family: "Pretendard", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, sans-serif;
```

## Noto Sans KR

Good as the safest webfont fallback for Korean-heavy decks.

```css
@import url("https://fonts.googleapis.com/css2?family=Noto+Sans+KR:wght@400;500;600;700;800&display=swap");
```

```css
font-family: "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", sans-serif;
```

## Minimal Deck Pairings

- Korean analytical deck: Paperlogy headings, Freesentation body.
- Dense business report: Pretendard headings and body.
- Conservative fallback: Noto Sans KR throughout.

Keep the visual system black-and-white first. Font choice should improve hierarchy and readability, not become decoration.
