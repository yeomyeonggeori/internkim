---
name: simple-slides
description: Compatibility alias for presentation deck requests. Use the presentation skill workflow and build script for slides, PPTX, PDF, HTML, Keynote, Google Slides, 발표자료, 파워포인트, and 피피티.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# Simple Slides

This skill name is a compatibility alias. For all new slide work, follow the presentation skill contract and use the `presentation` build script.

Default to HTML delivery when the user does not name a format, but still let the presentation build create internal review evidence. Use `FORMATS=pdf` for PDF, `FORMATS=pptx` for image-backed PPTX, and `FORMATS=all` when notes plus all export formats are needed.

Create an HTML-first deck with:

- `tmp/<deck-slug>/deck-brief.md`
- `tmp/<deck-slug>/required-visible-text.txt`
- `tmp/<deck-slug>/DESIGN.md`
- `tmp/<deck-slug>/slides.html`

Keep the structure disciplined without making every deck look identical. Choose a deck archetype, story spine, slide sequence, visual system, and Visual Identity Gate from the user's intent. Preserve the user's exact organization, product, period, metric values, targets, owners, dates, and missing-value labels such as `제공된 자료 없음`.

Use this build command shape:

```json
{
  "command": "/workspace/skills/presentation/scripts/build.sh",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

For PDF, PPTX, or all outputs, set the matching `FORMATS` value on that same command. Use this only for a mechanical no-review HTML export:

```json
{
  "command": "FORMATS=html /workspace/skills/presentation/scripts/build.sh",
  "workingDirectoryPath": "tmp/<deck-slug>"
}
```

Do not run `/workspace/skills/simple-slides/scripts/build.sh` directly unless a legacy path was already selected; that wrapper delegates to `/workspace/skills/presentation/scripts/build.sh`.

Deliver generated files from `tmp/<deck-slug>/build/` with `file.deliver`. Do not deliver internal review files unless the user asks.
