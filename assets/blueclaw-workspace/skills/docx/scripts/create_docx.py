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
        raise ValueError("document specification must be an object")
    return specification


def create_document(specification):
    if not ensure_requirements("docx"):
        raise RuntimeError("docx dependencies are unavailable after bootstrap")

    from docx import Document
    from docx.enum.section import WD_ORIENT
    from docx.enum.text import WD_ALIGN_PARAGRAPH
    from docx.shared import Inches, Pt

    document = Document()
    page = specification.get("page", {})
    if page is not None and not isinstance(page, dict):
        raise ValueError("page must be an object")

    section = document.sections[0]
    if page.get("orientation") == "landscape":
        section.orientation = WD_ORIENT.LANDSCAPE
        section.page_width, section.page_height = section.page_height, section.page_width

    margin_inches = float(page.get("marginInches", 0.8))
    section.top_margin = Inches(margin_inches)
    section.right_margin = Inches(margin_inches)
    section.bottom_margin = Inches(margin_inches)
    section.left_margin = Inches(margin_inches)

    font_name = require_text(specification.get("fontName", "Arial"), "fontName")
    font_size = float(specification.get("fontSize", 10.5))
    document.styles["Normal"].font.name = font_name
    document.styles["Normal"].font.size = Pt(font_size)

    title = optional_text(specification.get("title"))
    if title:
        heading = document.add_heading(title, level=0)
        heading.alignment = WD_ALIGN_PARAGRAPH.CENTER

    for block in read_blocks(specification):
        add_block(document, block)

    return document


def read_blocks(specification):
    blocks = specification.get("blocks", [])
    if not isinstance(blocks, list):
        raise ValueError("blocks must be an array")
    return blocks


def add_block(document, block):
    if not isinstance(block, dict):
        raise ValueError("each block must be an object")
    block_type = require_text(block.get("type"), "block.type")
    if block_type == "heading":
        add_heading(document, block)
        return
    if block_type == "paragraph":
        document.add_paragraph(optional_text(block.get("text")))
        return
    if block_type == "bullets":
        add_list(document, block, "List Bullet")
        return
    if block_type == "numbered":
        add_list(document, block, "List Number")
        return
    if block_type == "table":
        add_table(document, block)
        return
    if block_type == "pageBreak":
        document.add_page_break()
        return
    raise ValueError(f"unsupported block type: {block_type}")


def add_heading(document, block):
    level = int(block.get("level", 1))
    if level < 1 or level > 4:
        raise ValueError("heading level must be between 1 and 4")
    document.add_heading(optional_text(block.get("text")), level=level)


def add_list(document, block, style_name):
    items = block.get("items", [])
    if not isinstance(items, list):
        raise ValueError("list items must be an array")
    for item in items:
        document.add_paragraph(require_text(item, "list item"), style=style_name)


def add_table(document, block):
    rows = block.get("rows", [])
    if not isinstance(rows, list) or not rows:
        raise ValueError("table rows must be a non-empty array")
    width = len(rows[0])
    if width == 0:
        raise ValueError("table rows must contain at least one column")
    table = document.add_table(rows=0, cols=width)
    table.style = optional_text(block.get("style")) or "Table Grid"
    for row in rows:
        if not isinstance(row, list) or len(row) != width:
            raise ValueError("all table rows must have the same width")
        cells = table.add_row().cells
        for index, value in enumerate(row):
            cells[index].text = "" if value is None else str(value)


def parse_arguments():
    parser = argparse.ArgumentParser(description="Create a DOCX file from a JSON specification.")
    parser.add_argument("specification_path")
    parser.add_argument("output_path")
    return parser.parse_args()


def main():
    arguments = parse_arguments()
    specification = load_specification(arguments.specification_path)
    document = create_document(specification)
    output_path = Path(arguments.output_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    document.save(output_path)
    print(output_path)


if __name__ == "__main__":
    main()
