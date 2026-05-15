import json
import sys
import tempfile
from pathlib import Path


def main():
    request = json.load(sys.stdin)
    source_path = Path(request["path"])
    warnings = []
    converted_path = source_path
    temporary_directory = None

    try:
        max_pages = int(request.get("maxPages") or 0)
        if max_pages > 0 and source_path.suffix.lower() == ".pdf":
            converted_path, temporary_directory = limited_pdf(source_path, max_pages)
        elif max_pages > 0:
            warnings.append("maxPages is only applied to PDF files")

        markdown = build_markitdown(request).convert(str(converted_path))
        content = getattr(markdown, "text_content", "")
        print(json.dumps({"content": content, "warnings": warnings}, ensure_ascii=False))
    finally:
        if temporary_directory is not None:
            temporary_directory.cleanup()


def limited_pdf(source_path, max_pages):
    from pypdf import PdfReader, PdfWriter

    temporary_directory = tempfile.TemporaryDirectory(prefix="internkim-file-read-")
    output_path = Path(temporary_directory.name) / source_path.name
    reader = PdfReader(str(source_path))
    writer = PdfWriter()
    for page in reader.pages[:max_pages]:
        writer.add_page(page)
    with output_path.open("wb") as file:
        writer.write(file)
    return output_path, temporary_directory


def build_markitdown(request):
    from markitdown import MarkItDown

    if request.get("ocrMode") == "never":
        return MarkItDown(enable_plugins=False)

    from openai import OpenAI

    client = OpenAI(
        api_key=request.get("openRouterAPIKey"),
        base_url=request.get("openRouterBaseURL"),
    )
    return MarkItDown(
        enable_plugins=True,
        llm_client=client,
        llm_model=request.get("openRouterModel"),
        llm_prompt="Extract all visible text. Preserve headings, tables, lists, and reading order. Return Markdown only.",
    )


if __name__ == "__main__":
    main()
