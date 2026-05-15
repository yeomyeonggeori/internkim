#!/usr/bin/env python3
import argparse
import json

from skill_runtime import ensure_requirements


def summarize_presentation(presentation_path):
    if not ensure_requirements("pptx"):
        raise RuntimeError("pptx dependencies are unavailable after bootstrap")

    from pptx import Presentation

    presentation = Presentation(presentation_path)
    slides = []
    for index, slide in enumerate(presentation.slides, start=1):
        title = slide.shapes.title.text if slide.shapes.title else ""
        shape_count = len(slide.shapes)
        slides.append({
            "index": index,
            "title": title,
            "shapeCount": shape_count,
        })
    return {
        "slideCount": len(presentation.slides),
        "slides": slides,
        "validator": "python-pptx",
    }


def parse_arguments():
    parser = argparse.ArgumentParser(description="Validate and summarize a PPTX deck.")
    parser.add_argument("presentation_path")
    return parser.parse_args()


def main():
    arguments = parse_arguments()
    summary = summarize_presentation(arguments.presentation_path)
    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
