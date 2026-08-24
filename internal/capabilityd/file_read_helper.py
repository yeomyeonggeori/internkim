import base64
import contextlib
import io
import json
import sys
import tempfile
from pathlib import Path

OCR_INSTRUCTION = "Extract all visible text. Preserve headings, tables, lists, and reading order. Return Markdown only."

def main():
    request = json.load(sys.stdin)
    with contextlib.redirect_stdout(sys.stderr):
        response = convert(request)
    print(json.dumps(response, ensure_ascii=False))

def convert(request):
    source_path = Path(request["path"])
    max_pages = int(request.get("maxPages") or 0)
    if request.get("ocrMode") == "always":
        return read_through_ocr(request, source_path, max_pages)
    return read_locally(source_path, max_pages)

def read_locally(source_path, max_pages):
    warnings = []
    if max_pages > 0 and not is_pdf(source_path):
        warnings.append("maxPages is only applied to PDF files")
    if not is_convertible_document(source_path):
        return succeeded(markup_markdown(source_path), warnings)
    if max_pages > 0 and is_pdf(source_path):
        with limited_pdf(source_path, max_pages) as limited_path:
            return document_markdown(limited_path, source_path, warnings)
    return document_markdown(source_path, source_path, warnings)

def document_markdown(converted_path, source_path, warnings):
    import anydoc

    try:
        return succeeded(anydoc.to_markdown(str(converted_path)), warnings)
    except anydoc.UnsupportedError as error:
        if is_pdf(source_path):
            return failed("ocr_required", str(error))
        return failed("unsupported", str(error))
    except anydoc.EncryptedError as error:
        return failed("encrypted", str(error))
    except anydoc.ConvertError as error:
        return failed("conversion_failed", str(error))

def markup_markdown(source_path):
    from bs4 import BeautifulSoup
    from markdownify import MarkdownConverter

    soup = BeautifulSoup(source_path.read_bytes(), "html.parser")
    for removable in soup(["script", "style"]):
        removable.decompose()
    return MarkdownConverter(heading_style="ATX").convert_soup(soup.body or soup)

def is_convertible_document(source_path):
    import anydoc

    return anydoc.format_from_path(str(source_path)) is not None

def is_pdf(source_path):
    return source_path.suffix.lower() == ".pdf"

@contextlib.contextmanager
def limited_pdf(source_path, max_pages):
    from pypdf import PdfReader, PdfWriter

    with tempfile.TemporaryDirectory(prefix="internkim-file-read-") as directory:
        output_path = Path(directory) / source_path.name
        reader = PdfReader(str(source_path))
        writer = PdfWriter()
        for page in reader.pages[:max_pages]:
            writer.add_page(page)
        with output_path.open("wb") as file:
            writer.write(file)
        yield output_path

def read_through_ocr(request, source_path, max_pages):
    import pypdfium2

    if not is_pdf(source_path):
        return failed("ocr_unsupported_format", "OCR is only available for PDF files")
    document = pypdfium2.PdfDocument(str(source_path))
    page_count = len(document)
    page_limit = page_count if max_pages <= 0 else min(page_count, max_pages)
    client = build_openai_client(request)
    model = request.get("openRouterModel")
    pages = [read_page_markdown(client, model, render_page_image(document, index)) for index in range(page_limit)]
    warnings = []
    if page_limit < page_count:
        warnings.append("OCR covered the first {} of {} pages".format(page_limit, page_count))
    return succeeded("\n\n".join(page for page in pages if page), warnings)

def build_openai_client(request):
    from openai import OpenAI

    return OpenAI(
        api_key=request.get("openRouterAPIKey"),
        base_url=request.get("openRouterBaseURL"),
    )

def render_page_image(document, index):
    buffer = io.BytesIO()
    document[index].render(scale=2).to_pil().save(buffer, format="PNG")
    return buffer.getvalue()

def read_page_markdown(client, model, page_image):
    completion = client.chat.completions.create(
        model=model,
        messages=[{
            "role": "user",
            "content": [
                {"type": "text", "text": OCR_INSTRUCTION},
                {"type": "image_url", "image_url": {"url": "data:image/png;base64," + base64.b64encode(page_image).decode("ascii")}},
            ],
        }],
    )
    return (completion.choices[0].message.content or "").strip()

def succeeded(content, warnings):
    return {"content": content, "warnings": warnings}

def failed(error_code, message):
    return {"content": "", "warnings": [], "errorCode": error_code, "message": message}

if __name__ == "__main__":
    main()
