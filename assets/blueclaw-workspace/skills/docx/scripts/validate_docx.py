#!/usr/bin/env python3
import argparse
import json


def summarize_document(document_path):
    from docx import Document

    document = Document(document_path)
    paragraphs = [paragraph.text for paragraph in document.paragraphs if paragraph.text.strip()]
    tables = []
    for table in document.tables:
        tables.append({
            "rows": len(table.rows),
            "columns": len(table.columns),
        })
    return {
        "paragraphCount": len(paragraphs),
        "tableCount": len(document.tables),
        "tables": tables,
        "firstParagraphs": paragraphs[:5],
    }


def parse_arguments():
    parser = argparse.ArgumentParser(description="Validate and summarize a DOCX file.")
    parser.add_argument("document_path")
    return parser.parse_args()


def main():
    arguments = parse_arguments()
    summary = summarize_document(arguments.document_path)
    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
