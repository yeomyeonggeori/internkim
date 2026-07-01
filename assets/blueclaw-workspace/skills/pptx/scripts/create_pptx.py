#!/usr/bin/env python3
import argparse
import json
import os
from pathlib import Path

from skill_runtime import ensure_requirements


DEFAULT_FONT_NAME = "Paperlogy"
DEFAULT_COLORS = {
    "background": "FAFAFA",
    "surface": "FFFFFF",
    "ink": "111111",
    "muted": "6B7280",
    "accent": "111111",
    "line": "D4D4D8",
}


def require_text(value, field_name):
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{field_name} must be a non-empty string")
    return value.strip()


def optional_text(value):
    if value is None:
        return ""
    if not isinstance(value, str):
        raise ValueError("text fields must be strings")
    return value.strip()


def load_specification(specification_path):
    with open(specification_path, "r", encoding="utf-8") as specification_file:
        specification = json.load(specification_file)
    if not isinstance(specification, dict):
        raise ValueError("presentation specification must be an object")
    return specification


def create_presentation(specification):
    modules = load_powerpoint_modules()
    presentation = modules["Presentation"]()
    presentation.slide_width = modules["Inches"](float(specification.get("widthInches", 13.333)))
    presentation.slide_height = modules["Inches"](float(specification.get("heightInches", 7.5)))
    style = normalized_style(specification)

    slides = specification.get("slides", [])
    if not isinstance(slides, list) or not slides:
        raise ValueError("slides must be a non-empty array")

    for slide_specification in slides:
        add_slide(presentation, slide_specification, style, modules)

    return presentation


def load_powerpoint_modules():
    if not ensure_requirements("pptx"):
        raise RuntimeError("pptx dependencies are unavailable after bootstrap")

    from pptx import Presentation
    from pptx.dml.color import RGBColor
    from pptx.enum.shapes import MSO_SHAPE
    from pptx.enum.text import PP_ALIGN
    from pptx.util import Inches, Pt

    return {
        "Presentation": Presentation,
        "Inches": Inches,
        "Pt": Pt,
        "RGBColor": RGBColor,
        "MSO_SHAPE": MSO_SHAPE,
        "PP_ALIGN": PP_ALIGN,
    }


def normalized_style(specification):
    style = specification.get("style", {})
    if style is None:
        style = {}
    if not isinstance(style, dict):
        raise ValueError("style must be an object")
    style_colors = style.get("colors", {})
    if not isinstance(style_colors, dict):
        raise ValueError("style.colors must be an object")
    colors = DEFAULT_COLORS | style_colors
    return {
        "fontName": optional_text(style.get("fontName")) or DEFAULT_FONT_NAME,
        "colors": {key: clean_hex_color(value) for key, value in colors.items()},
    }


def clean_hex_color(value):
    if not isinstance(value, str):
        raise ValueError("color values must be strings")
    color = value.strip().removeprefix("#")
    if len(color) != 6:
        raise ValueError(f"invalid color value: {value}")
    return color.upper()


def add_slide(presentation, slide_specification, style, modules):
    if not isinstance(slide_specification, dict):
        raise ValueError("each slide must be an object")
    layout_name = slide_specification.get("layout", "titleAndBody")
    slide = presentation.slides.add_slide(presentation.slide_layouts[6])
    if has_background_image(slide_specification):
        add_full_slide_image(slide, presentation, slide_specification, modules)
    else:
        add_background(slide, presentation, style, modules)
        add_top_rule(slide, presentation, style, modules)
    if layout_name == "hybrid":
        add_editable_texts(slide, slide_specification, style, modules)
    elif layout_name == "title":
        add_title_slide(slide, presentation, slide_specification, style, modules)
    elif layout_name == "cards":
        add_cards_slide(slide, slide_specification, style, modules)
    elif layout_name == "comparison":
        add_comparison_slide(slide, slide_specification, style, modules)
    elif layout_name == "matrix":
        add_matrix_slide(slide, slide_specification, style, modules)
    elif layout_name == "timeline":
        add_timeline_slide(slide, slide_specification, style, modules)
    else:
        add_body_slide(slide, slide_specification, style, modules)
    add_images(slide, slide_specification, modules["Inches"])
    if layout_name != "hybrid":
        add_editable_texts(slide, slide_specification, style, modules)


def has_background_image(slide_specification):
    return bool(optional_text(slide_specification.get("backgroundImage")))


def add_full_slide_image(slide, presentation, slide_specification, modules):
    path = require_text(slide_specification.get("backgroundImage"), "slide.backgroundImage")
    background = slide.shapes.add_picture(path, 0, 0, width=presentation.slide_width, height=presentation.slide_height)
    background.name = "Hybrid Background"


def add_background(slide, presentation, style, modules):
    shape = slide.shapes.add_shape(
        modules["MSO_SHAPE"].RECTANGLE,
        0,
        0,
        presentation.slide_width,
        presentation.slide_height,
    )
    shape.fill.solid()
    shape.fill.fore_color.rgb = rgb_color(style, modules, "background")
    shape.line.fill.background()


def add_top_rule(slide, presentation, style, modules):
    shape = slide.shapes.add_shape(
        modules["MSO_SHAPE"].RECTANGLE,
        modules["Inches"](0.72),
        modules["Inches"](0.48),
        presentation.slide_width - modules["Inches"](1.44),
        modules["Inches"](0.02),
    )
    shape.fill.solid()
    shape.fill.fore_color.rgb = rgb_color(style, modules, "ink")
    shape.line.fill.background()


def add_title_slide(slide, presentation, slide_specification, style, modules):
    add_text_box(
        slide,
        require_text(slide_specification.get("title"), "slide.title"),
        modules["Inches"](0.78),
        modules["Inches"](1.56),
        presentation.slide_width - modules["Inches"](1.56),
        modules["Inches"](1.8),
        44,
        style,
        modules,
        weight="bold",
    )
    subtitle = optional_text(slide_specification.get("subtitle"))
    if subtitle:
        add_text_box(
            slide,
            subtitle,
            modules["Inches"](0.82),
            modules["Inches"](3.58),
            presentation.slide_width - modules["Inches"](3.1),
            modules["Inches"](1.0),
            20,
            style,
            modules,
            color_name="muted",
        )


def add_body_slide(slide, slide_specification, style, modules):
    add_slide_title(slide, slide_specification, style, modules)
    body = normalized_body(slide_specification)
    if not body:
        return
    add_text_list(
        slide,
        body,
        modules["Inches"](0.86),
        modules["Inches"](1.78),
        modules["Inches"](11.62),
        modules["Inches"](4.6),
        style,
        modules,
    )


def add_cards_slide(slide, slide_specification, style, modules):
    add_slide_title(slide, slide_specification, style, modules)
    body = normalized_body(slide_specification)
    if not body:
        return
    columns = min(3, max(1, len(body)))
    card_width = 11.64 / columns
    for index, item in enumerate(body):
        column = index % columns
        row = index // columns
        left = 0.86 + column * card_width
        top = 1.84 + row * 1.5
        add_card(slide, require_text(item, "body item"), left, top, card_width - 0.18, 1.18, style, modules)


def add_comparison_slide(slide, slide_specification, style, modules):
    add_slide_title(slide, slide_specification, style, modules)
    columns = normalized_columns(slide_specification)
    column_width = 5.62
    for index, column in enumerate(columns[:2]):
        left = 0.86 + index * 5.94
        add_panel(slide, left, 1.78, column_width, 4.72, style, modules)
        add_text_box(
            slide,
            column["title"],
            modules["Inches"](left + 0.28),
            modules["Inches"](2.08),
            modules["Inches"](column_width - 0.56),
            modules["Inches"](0.44),
            20,
            style,
            modules,
            weight="bold",
        )
        add_text_list(
            slide,
            column["body"],
            modules["Inches"](left + 0.28),
            modules["Inches"](2.74),
            modules["Inches"](column_width - 0.56),
            modules["Inches"](3.2),
            style,
            modules,
            font_size=16,
        )


def add_matrix_slide(slide, slide_specification, style, modules):
    add_slide_title(slide, slide_specification, style, modules)
    items = normalized_body(slide_specification)
    if not items:
        return
    columns = 2
    cell_width = 5.68
    cell_height = 1.22
    for index, item in enumerate(items[:8]):
        column = index % columns
        row = index // columns
        left = 0.86 + column * 5.94
        top = 1.82 + row * 1.38
        add_card(slide, require_text(item, "matrix item"), left, top, cell_width, cell_height, style, modules, font_size=15)


def add_timeline_slide(slide, slide_specification, style, modules):
    add_slide_title(slide, slide_specification, style, modules)
    items = normalized_body(slide_specification)
    if not items:
        return
    step_width = 10.9 / max(1, len(items))
    for index, item in enumerate(items[:6]):
        left = 0.96 + index * step_width
        add_text_box(
            slide,
            f"{index + 1}",
            modules["Inches"](left),
            modules["Inches"](2.0),
            modules["Inches"](0.5),
            modules["Inches"](0.44),
            18,
            style,
            modules,
            weight="bold",
        )
        add_card(slide, require_text(item, "timeline item"), left, 2.62, step_width - 0.28, 2.22, style, modules, font_size=14)


def add_slide_title(slide, slide_specification, style, modules):
    add_text_box(
        slide,
        require_text(slide_specification.get("title"), "slide.title"),
        modules["Inches"](0.82),
        modules["Inches"](0.78),
        modules["Inches"](11.7),
        modules["Inches"](0.66),
        28,
        style,
        modules,
        weight="bold",
    )


def add_panel(slide, left, top, width, height, style, modules):
    shape = slide.shapes.add_shape(
        modules["MSO_SHAPE"].RECTANGLE,
        modules["Inches"](left),
        modules["Inches"](top),
        modules["Inches"](width),
        modules["Inches"](height),
    )
    shape.fill.solid()
    shape.fill.fore_color.rgb = rgb_color(style, modules, "surface")
    shape.line.color.rgb = rgb_color(style, modules, "line")
    shape.line.width = modules["Pt"](0.75)


def add_card(slide, text, left, top, width, height, style, modules, font_size=16):
    add_panel(slide, left, top, width, height, style, modules)
    add_text_box(
        slide,
        text,
        modules["Inches"](left + 0.22),
        modules["Inches"](top + 0.2),
        modules["Inches"](width - 0.44),
        modules["Inches"](height - 0.34),
        font_size,
        style,
        modules,
    )


def add_text_list(slide, items, left, top, width, height, style, modules, font_size=18):
    text_box = slide.shapes.add_textbox(left, top, width, height)
    text_frame = text_box.text_frame
    text_frame.clear()
    text_frame.word_wrap = True
    for index, item in enumerate(items):
        paragraph = text_frame.paragraphs[0] if index == 0 else text_frame.add_paragraph()
        run = paragraph.add_run()
        run.text = require_text(item, "body item")
        run.font.name = style["fontName"]
        run.font.size = modules["Pt"](font_size)
        run.font.color.rgb = rgb_color(style, modules, "ink")
        paragraph.space_after = modules["Pt"](9)


def add_text_box(slide, text, left, top, width, height, font_size, style, modules, weight="regular", color_name="ink"):
    text_box = slide.shapes.add_textbox(left, top, width, height)
    text_frame = text_box.text_frame
    text_frame.clear()
    text_frame.word_wrap = True
    paragraph = text_frame.paragraphs[0]
    paragraph.alignment = modules["PP_ALIGN"].LEFT
    run = paragraph.add_run()
    run.text = text
    run.font.name = style["fontName"]
    run.font.size = modules["Pt"](font_size)
    run.font.bold = weight == "bold"
    run.font.color.rgb = rgb_color(style, modules, color_name)
    return text_box


def add_editable_texts(slide, slide_specification, style, modules):
    editable_texts = slide_specification.get("editableTexts", [])
    if not isinstance(editable_texts, list):
        raise ValueError("editableTexts must be an array")
    for index, editable_text in enumerate(editable_texts):
        if not isinstance(editable_text, dict):
            raise ValueError("each editableText must be an object")
        text_box = add_text_box(
            slide,
            require_text(editable_text.get("text"), f"editableTexts[{index}].text"),
            modules["Inches"](float(editable_text.get("leftInches", 0.8))),
            modules["Inches"](float(editable_text.get("topInches", 0.8))),
            modules["Inches"](float(editable_text.get("widthInches", 5.0))),
            modules["Inches"](float(editable_text.get("heightInches", 0.8))),
            float(editable_text.get("fontSize", 20)),
            style,
            modules,
            weight=optional_text(editable_text.get("weight")) or "regular",
            color_name=optional_text(editable_text.get("colorName")) or "ink",
        )
        text_box.name = "Editable Overlay"


def add_images(slide, slide_specification, inches):
    images = slide_specification.get("images", [])
    if not isinstance(images, list):
        raise ValueError("images must be an array")
    for image in images:
        if not isinstance(image, dict):
            raise ValueError("each image must be an object")
        path = require_text(image.get("path"), "image.path")
        left = inches(float(image.get("leftInches", 1)))
        top = inches(float(image.get("topInches", 1.5)))
        width = inches(float(image.get("widthInches", 5)))
        slide.shapes.add_picture(path, left, top, width=width)


def normalized_body(slide_specification):
    body = slide_specification.get("body", [])
    if isinstance(body, str):
        return [body]
    if not isinstance(body, list):
        raise ValueError("slide body must be a string or an array")
    return body


def normalized_columns(slide_specification):
    columns = slide_specification.get("columns")
    if columns is None:
        body = normalized_body(slide_specification)
        return [
            {"title": "Option A", "body": body[: max(1, len(body) // 2)]},
            {"title": "Option B", "body": body[max(1, len(body) // 2):]},
        ]
    if not isinstance(columns, list) or len(columns) < 2:
        raise ValueError("columns must contain at least two objects")
    normalized = []
    for index, column in enumerate(columns):
        if not isinstance(column, dict):
            raise ValueError("each column must be an object")
        normalized.append({
            "title": require_text(column.get("title"), f"columns[{index}].title"),
            "body": normalized_body(column),
        })
    return normalized


def rgb_color(style, modules, color_name):
    color = style["colors"][color_name]
    return modules["RGBColor"](int(color[0:2], 16), int(color[2:4], 16), int(color[4:6], 16))


def collect_inline_slides(tokens):
    slides = []
    current_slide = None
    index = 0
    while index < len(tokens):
        if tokens[index] == "--slide-title" and index + 1 < len(tokens):
            if current_slide is not None:
                slides.append(current_slide)
            current_slide = {"title": tokens[index + 1], "bullets": []}
            index += 2
        elif tokens[index] == "--bullet" and index + 1 < len(tokens):
            if current_slide is None:
                current_slide = {"title": "", "bullets": []}
            current_slide["bullets"].append(tokens[index + 1])
            index += 2
        else:
            raise SystemExit(f"unrecognized argument: {tokens[index]}")
    if current_slide is not None:
        slides.append(current_slide)
    return slides


def build_specification_from_arguments(arguments):
    slides = []
    if arguments.deck_title:
        slides.append({"layout": "title", "title": arguments.deck_title})
    for slide in arguments.inline_slides:
        slides.append({"layout": "titleAndBody", "title": slide["title"], "body": slide["bullets"]})
    if not slides:
        raise ValueError("provide --deck-title and/or --slide-title, or pass --spec <file>")
    return {"slides": slides}


def parse_arguments():
    parser = argparse.ArgumentParser(description="Create a PPTX deck from arguments or a JSON spec.")
    parser.add_argument("output_path", help="Path to the output .pptx file")
    parser.add_argument("--deck-title", metavar="TEXT", dest="deck_title", default="", help="Title slide text")
    parser.add_argument("--spec", metavar="JSON_PATH", help="Full spec JSON for rich decks (themes, layouts, images)")
    arguments, extra_tokens = parser.parse_known_args()
    arguments.inline_slides = collect_inline_slides(extra_tokens)
    return arguments


def main():
    arguments = parse_arguments()
    specification = load_specification(arguments.spec) if arguments.spec else build_specification_from_arguments(arguments)
    presentation = create_presentation(specification)
    output_path = Path(os.path.expanduser(arguments.output_path))
    output_path.parent.mkdir(parents=True, exist_ok=True)
    presentation.save(output_path)
    print(output_path)


if __name__ == "__main__":
    main()
