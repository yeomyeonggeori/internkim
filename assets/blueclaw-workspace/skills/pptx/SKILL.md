---
name: pptx
description: Create, read, edit, combine, clean, and attach PowerPoint files. Use for .pptx files, PowerPoint decks, slide decks, presentations, templates, speaker notes, comments, 발표자료, 프레젠테이션, 파워포인트, 피피티, or slide file requests. For new designed decks that also need HTML/PDF output, prefer the simple-slides skill.
when_to_use: Use when the user asks for PowerPoint, .pptx, slide deck, presentation, deck editing, speaker notes, 발표자료, 프레젠테이션, 파워포인트, 피피티, or a PPTX deliverable.
allowed-tools:
  - terminal.run
  - file.write
  - file.attach
---

# PPTX Presentations

Create or modify PowerPoint `.pptx` files as local artifacts, then attach the final deck.

## Workflow

1. Clarify only missing inputs that change the deck, such as audience, slide count, aspect ratio, source file, or required sections.
2. Create work under `$BLUECLAW_TASK_TMP`; do not use Blueclaw internal temporary paths.
3. For straightforward new decks, write a JSON spec and run `scripts/create_pptx.py`.
4. Use direct `python-pptx` code for custom layouts, charts, notes, or edits that exceed the script.
5. Use ZIP/XML inspection only when `python-pptx` cannot preserve or reach the needed feature.
6. Validate with `scripts/validate_pptx.py` or by reopening the generated deck and checking slide count, titles, text, images, and layout.
7. Move accepted final files to `$BLUECLAW_REQUESTER_ARTIFACTS/<deck-slug>/` unless the user requested a circle or shared destination, then attach the final `.pptx`; attach exported PDFs or images only if requested.

Do not run package installation unless the required library is missing and the user approves it. If `python-pptx` is unavailable, say that the runtime is missing the PowerPoint library and explain the exact package needed.

For from-scratch presentation design with HTML/PDF/PPTX outputs, use the existing `simple-slides` skill unless the user specifically needs direct PowerPoint object editing.

## Helper Scripts

For normal decks, create a spec file:

```json
{
  "widthInches": 13.333,
  "heightInches": 7.5,
  "slides": [
    {
      "layout": "title",
      "title": "Presentation Title",
      "subtitle": "Subtitle"
    },
    {
      "title": "Main takeaway",
      "body": ["First point", "Second point"],
      "images": [
        {
          "path": "chart.png",
          "leftInches": 7,
          "topInches": 1.5,
          "widthInches": 5
        }
      ]
    }
  ]
}
```

Run:

```json
{
  "command": "python3 /workspace/skills/pptx/scripts/create_pptx.py deck.json output.pptx && python3 /workspace/skills/pptx/scripts/validate_pptx.py output.pptx",
  "workingDirectoryPath": "$BLUECLAW_TASK_TMP"
}
```

## Creation

Use `python-pptx` for standard decks:

```python
from pptx import Presentation
from pptx.util import Inches, Pt

presentation = Presentation()
presentation.slide_width = Inches(13.333)
presentation.slide_height = Inches(7.5)

title_slide_layout = presentation.slide_layouts[0]
slide = presentation.slides.add_slide(title_slide_layout)
slide.shapes.title.text = "Presentation Title"
slide.placeholders[1].text = "Subtitle"

content_slide_layout = presentation.slide_layouts[5]
slide = presentation.slides.add_slide(content_slide_layout)
slide.shapes.title.text = "Main takeaway"

text_box = slide.shapes.add_textbox(Inches(0.8), Inches(1.5), Inches(11.7), Inches(4.8))
text_frame = text_box.text_frame
text_frame.word_wrap = True
paragraph = text_frame.paragraphs[0]
run = paragraph.add_run()
run.text = "Body text."
run.font.size = Pt(24)

presentation.save("output.pptx")
```

Use one clear message per slide. Prefer structured layouts, readable type, and generous margins over dense bullet lists.

## Editing Existing Files

For user-provided `.pptx` files:

1. Copy the input to a working directory.
2. Load with `Presentation("input.pptx")`.
3. Preserve slide masters, layouts, theme colors, and existing images where possible.
4. Save to a new filename unless the user asks to replace the original.

Use direct XML inspection for notes, comments, relationships, or other features that `python-pptx` does not expose:

```bash
python - <<'PY'
from zipfile import ZipFile

with ZipFile("input.pptx") as archive:
    for name in archive.namelist():
        if name.startswith("ppt/"):
            print(name)
PY
```

Avoid XML rewrites unless required. If XML editing is required, preserve namespaces, relationships, and content types.

## Images

Add local image files with explicit dimensions:

```python
slide.shapes.add_picture("image.png", Inches(1), Inches(1.5), width=Inches(5.5))
```

Verify image paths exist before saving. Use high-resolution source images when the deck will be projected.

## Validation

Always reopen the saved deck:

```python
from pptx import Presentation

presentation = Presentation("output.pptx")
print(len(presentation.slides))
for index, slide in enumerate(presentation.slides, start=1):
    title = slide.shapes.title.text if slide.shapes.title else ""
    print(index, title)
```

When layout fidelity matters and LibreOffice is available, export to PDF or slide images and inspect before attaching.
