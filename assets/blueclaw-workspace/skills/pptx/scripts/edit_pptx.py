#!/usr/bin/env python3
import argparse
import glob
import json
import os

from create_pptx import collect_inline_slides, load_powerpoint_modules


def parse_arguments():
    parser = argparse.ArgumentParser(description="Append slides to an existing PPTX in place.")
    parser.add_argument("deck_path", nargs="?", help="Path to the .pptx; defaults to the newest .pptx in ~/documents")
    parser.add_argument(
        "--slides",
        metavar="JSON_PATH",
        help="JSON file: array of {title, bullets} objects",
    )
    arguments, extra_tokens = parser.parse_known_args()
    arguments.inline_slides = collect_inline_slides(extra_tokens)
    return arguments


def resolve_deck_path(given_path):
    if given_path:
        return os.path.expanduser(given_path)
    documents = sorted(
        glob.glob(os.path.expanduser("~/documents/*.pptx")),
        key=os.path.getmtime,
        reverse=True,
    )
    if not documents:
        raise SystemExit("no .pptx found in ~/documents; pass the deck path explicitly")
    return documents[0]


def load_slides_from_json(slides_path):
    with open(os.path.expanduser(slides_path), "r", encoding="utf-8") as slides_file:
        slides = json.load(slides_file)
    if not isinstance(slides, list):
        raise ValueError("slides JSON must be an array")
    return slides


def populate_text_frame_with_bullets(text_frame, bullets):
    text_frame.clear()
    for index, bullet_text in enumerate(bullets):
        paragraph = text_frame.paragraphs[0] if index == 0 else text_frame.add_paragraph()
        paragraph.text = bullet_text


def append_slide(presentation, slide_specification, modules):
    title_text = slide_specification.get("title", "")
    bullets = slide_specification.get("bullets", [])
    layout = presentation.slide_layouts[1 if len(presentation.slide_layouts) > 1 else 0]
    slide = presentation.slides.add_slide(layout)
    if slide.shapes.title:
        slide.shapes.title.text = title_text
    content_placeholder = next(
        (placeholder for placeholder in slide.placeholders if placeholder.placeholder_format.idx == 1),
        None,
    )
    if content_placeholder and bullets:
        populate_text_frame_with_bullets(content_placeholder.text_frame, bullets)
    elif bullets:
        text_box = slide.shapes.add_textbox(
            modules["Inches"](0.86),
            modules["Inches"](1.8),
            modules["Inches"](11.62),
            modules["Inches"](4.6),
        )
        text_box.text_frame.word_wrap = True
        populate_text_frame_with_bullets(text_box.text_frame, bullets)


def main():
    arguments = parse_arguments()
    modules = load_powerpoint_modules()
    deck_path = resolve_deck_path(arguments.deck_path)
    presentation = modules["Presentation"](deck_path)
    all_slides = list(arguments.inline_slides)
    if arguments.slides:
        all_slides.extend(load_slides_from_json(arguments.slides))
    for slide_specification in all_slides:
        append_slide(presentation, slide_specification, modules)
    presentation.save(deck_path)
    print(deck_path)


if __name__ == "__main__":
    main()
