# Block Capabilities Reference

## Icons

`features` and `contact` items accept an `icon` field with one of these lucide names; anything else falls back to a numbered index:

award, book-open, calendar, check-circle, clock, coffee, compass, flame, gift, globe, hammer, heart, instagram, leaf, lightbulb, mail, map-pin, message-circle, package, palette, phone, rocket, shield-check, shopping-bag, sparkles, star, sun, users, wrench, zap

Choose icons that match each item's meaning: contact items usually pair `mail`, `instagram`, `map-pin`, `clock`; feature grids pick per-item metaphors. Do not decorate every block — icons carry meaning, not garnish.

## Images

`hero` and `prose` blocks accept `image` (path or URL) plus `imageAlt`. A hero with an image renders text-left, image-right; a prose image renders as a wide banner above the text.

Sourcing order:
1. Files the user attached or that already exist in the site workspace.
2. License-free photo sources fetched with the web tools (Unsplash, Pexels, Pixabay class licenses that allow commercial use without attribution). Download into `app/public/images/<name>.jpg` and reference as `/images/<name>.jpg` — never hotlink.
3. Image generation skills, only when no suitable license-free photo exists (abstract brand art, product mockups that do not exist yet).

Always set `imageAlt`. Keep files under about 400KB; prefer 1600px-wide JPEG.

## Contact Links

Contact body text and item bodies auto-link emails (mailto:), full URLs, and @instagram handles — write them as plain text like `hello@example.com`, `https://example.com`, `@studio.handle`, and the renderer makes them clickable. Use contact `items` for structured entries: `{ "title": "이메일", "body": "hello@example.com", "icon": "mail" }`.

## Backdrops

`hero` and `cta` blocks accept `backdrop` with one of: `mesh`, `aurora`, `grain`, `grid`, `dots`. Each renders a designer-grade decorative layer from the theme palette — no image needed, colors always harmonize:

- `mesh`: blended multi-point color gradient; warm, contemporary hero default.
- `aurora`: large soft blurred color fields; dreamy, premium.
- `grain`: film-grain texture over a soft tint; crafted, analog.
- `grid`: fine fading line grid; technical, product-focused.
- `dots`: fading dot matrix; playful, precise.

A hero with no image should almost always carry a backdrop that matches the mood. Combine with the `style:` preset in DESIGN.md — e.g. pottery studio: `grain`; tech product: `grid` or `mesh`; kids brand: `dots`.

## Font Catalog

Served families (anything else silently falls back to the platform default): `에이투지체` (default; all-round geometric sans), `Pretendard` (neutral body/UI), `Paperlogy` (display-friendly geometric), `마루부리` (screen serif with brush warmth), `Gowun Batang` (quiet literary serif), `Gowun Dodum` (handwritten-humanist), `Galmuri` (retro pixel), `D2Coding` (mono). Match the voice to the request and say why in DESIGN.md.
