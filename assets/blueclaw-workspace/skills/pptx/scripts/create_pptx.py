#!/usr/bin/env python3
import argparse
import json
from pathlib import Path

from skill_runtime import ensure_requirements


def require_text(value, field_name):
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{field_name} must be a non-empty string")
    return value.strip()


def optional_text(value):
    if value is None:
        return ""
    if not isinstance(value, str):
        raise ValueError("text fields must be strings")
    return value


def load_specification(specification_path):
    with open(specification_path, "r", encoding="utf-8") as specification_file:
        specification = json.load(specification_file)
    if not isinstance(specification, dict):
        raise ValueError("presentation specification must be an object")
    return specification


def create_presentation(specification):
    presentation_class, inches, points = load_powerpoint_modules()
    presentation = presentation_class()
    presentation.slide_width = inches(float(specification.get("widthInches", 13.333)))
    presentation.slide_height = inches(float(specification.get("heightInches", 7.5)))

    slides = specification.get("slides", [])
    if not isinstance(slides, list) or not slides:
        raise ValueError("slides must be a non-empty array")

    for slide_specification in slides:
        add_slide(presentation, slide_specification, inches, points)

    return presentation


def load_powerpoint_modules():
    if not ensure_requirements("pptx"):
        raise RuntimeError("pptx dependencies are unavailable after bootstrap")

    from pptx import Presentation
    from pptx.util import Inches, Pt

    return Presentation, Inches, Pt


def add_slide(presentation, slide_specification, inches, points):
    if not isinstance(slide_specification, dict):
        raise ValueError("each slide must be an object")
    layout_name = slide_specification.get("layout", "titleAndBody")
    if layout_name == "title":
        slide = presentation.slides.add_slide(presentation.slide_layouts[0])
        slide.shapes.title.text = optional_text(slide_specification.get("title"))
        slide.placeholders[1].text = optional_text(slide_specification.get("subtitle"))
        return
    slide = presentation.slides.add_slide(presentation.slide_layouts[5])
    if slide.shapes.title:
        slide.shapes.title.text = require_text(slide_specification.get("title", "Slide"), "slide.title")
    add_body(slide, slide_specification, inches, points)
    add_images(slide, slide_specification, inches)


def add_body(slide, slide_specification, inches, points):
    body = normalized_body(slide_specification)
    if not body:
        return
    text_box = slide.shapes.add_textbox(inches(0.8), inches(1.4), inches(11.7), inches(4.8))
    text_frame = text_box.text_frame
    text_frame.word_wrap = True
    text_frame.clear()
    for index, item in enumerate(body):
        paragraph = text_frame.paragraphs[0] if index == 0 else text_frame.add_paragraph()
        paragraph.text = require_text(item, "body item")
        paragraph.font.size = points(24)
        paragraph.space_after = points(10)


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


def parse_arguments():
    parser = argparse.ArgumentParser(description="Create a PPTX deck from a JSON specification.")
    parser.add_argument("specification_path")
    parser.add_argument("output_path")
    return parser.parse_args()


def main():
    arguments = parse_arguments()
    specification = load_specification(arguments.specification_path)
    presentation = create_presentation(specification)
    output_path = Path(arguments.output_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    presentation.save(output_path)
    print(output_path)


if __name__ == "__main__":
    main()
