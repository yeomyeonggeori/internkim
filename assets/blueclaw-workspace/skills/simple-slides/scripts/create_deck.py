#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
import shutil
import subprocess


def main() -> int:
    arguments = parse_arguments()
    working_directory_path = pathlib.Path.cwd()
    skill_directory_path = pathlib.Path(__file__).resolve().parents[1]
    slug = clean_slug(arguments.slug)
    brief = read_required_text(pathlib.Path(arguments.brief), "brief file is required")
    presentation = read_required_text(working_directory_path / "presentation.md", "presentation.md is required")
    design = read_required_text(working_directory_path / "DESIGN.md", "DESIGN.md is required")
    contract = deck_contract(brief, slug)

    validate_design(design)
    validate_presentation(contract, presentation)
    write_text(working_directory_path / f"{slug}-intent.json", deck_intent_manifest(contract))
    copy_runtime_file(skill_directory_path / "assets" / "build.sh", working_directory_path / "build.sh")
    copy_runtime_file(skill_directory_path / "scripts" / "extract_notes.py", working_directory_path / "extract_notes.py")
    copy_runtime_file(skill_directory_path / "scripts" / "render_review.py", working_directory_path / "render_review.py")

    if not arguments.no_build:
        subprocess.run(["bash", "-lc", f"NAME={shell_quote(slug)} ./build.sh"], cwd=working_directory_path, check=True)
    return 0


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--slug", required=True)
    parser.add_argument("--brief", default="brief.md")
    parser.add_argument("--no-build", action="store_true")
    return parser.parse_args()


def clean_slug(value: str) -> str:
    cleaned = "".join(character.lower() if character.isalnum() else "-" for character in value)
    cleaned = "-".join(part for part in cleaned.split("-") if part)
    return cleaned or "presentation"


def read_required_text(path: pathlib.Path, message: str) -> str:
    if not path.exists():
        fail(f"{message}: {path}")
    content = path.read_text(encoding="utf-8").strip()
    if not content:
        fail(f"{path.name} is empty")
    return content


def write_text(path: pathlib.Path, content: str) -> None:
    path.write_text(content.strip() + "\n", encoding="utf-8")


def copy_runtime_file(source_path: pathlib.Path, target_path: pathlib.Path) -> None:
    shutil.copyfile(source_path, target_path)
    target_path.chmod(0o755)


def shell_quote(value: str) -> str:
    return "'" + value.replace("'", "'\"'\"'") + "'"


def fail(message: str) -> None:
    raise SystemExit(message)


def deck_contract(brief: str, slug: str) -> dict:
    values = brief_key_values(brief)
    original_request = required_value(values, "original_user_request")
    topic = required_value(values, "topic")
    slide_intent = required_value(values, "slide_intent")
    output_slug = clean_slug(required_value(values, "output_slug"))
    if output_slug != slug:
        fail(f"output_slug {output_slug!r} does not match --slug {slug!r}")
    deck_spec = parse_required_deck_spec(brief)
    slides = validate_deck_spec(deck_spec)
    requested_slide_count = requested_slide_count_from_values(values, original_request)
    if requested_slide_count and requested_slide_count != len(slides):
        fail(f"requested {requested_slide_count} slides but deck_spec contains {len(slides)}")
    return {
        "output_slug": slug,
        "mode": deck_mode(original_request, topic, slide_intent),
        "original_user_request": original_request,
        "topic": topic,
        "slide_intent": slide_intent,
        "requested_slide_count": requested_slide_count or len(slides),
        "requested_formats": requested_formats_from_values(values, original_request),
        "slides": slides,
    }


def brief_key_values(brief: str) -> dict[str, str]:
    values: dict[str, str] = {}
    for line in brief.splitlines():
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        normalized_key = key.strip().lower()
        if normalized_key and normalized_key != "deck_spec":
            values[normalized_key] = clean_value(value)
    return values


def clean_value(value: str) -> str:
    return " ".join(value.strip().strip("\"'").split())


def required_value(values: dict[str, str], key: str) -> str:
    value = clean_value(values.get(key, ""))
    if not value:
        fail(f"{key} is required in brief.md")
    return value


def deck_mode(*values: str) -> str:
    text = " ".join(values).lower()
    if any(keyword in text for keyword in ["capability", "capabilities", "what i can do", "can do", "할 수", "역량", "지원 가능"]):
        return "capabilities"
    return "generic"


def parse_required_deck_spec(brief: str) -> dict:
    deck_spec_text = extract_deck_spec_text(brief)
    if not deck_spec_text:
        fail("deck_spec is required")
    try:
        deck_spec = json.loads(deck_spec_text)
    except json.JSONDecodeError as error_value:
        fail(f"deck_spec must be valid JSON: {error_value}")
    if not isinstance(deck_spec, dict):
        fail("deck_spec must be a JSON object")
    return deck_spec


def extract_deck_spec_text(brief: str) -> str:
    lines = brief.splitlines()
    for index, line in enumerate(lines):
        if line.strip().lower() == "deck_spec:":
            return extract_deck_spec_after_line(lines[index + 1 :])
    return ""


def extract_deck_spec_after_line(lines: list[str]) -> str:
    while lines and not lines[0].strip():
        lines = lines[1:]
    if lines and lines[0].strip().startswith("```"):
        return extract_fenced_block(lines[1:])
    return "\n".join(line[2:] if line.startswith("  ") else line for line in lines).strip()


def extract_fenced_block(lines: list[str]) -> str:
    block_lines = []
    for line in lines:
        if line.strip().startswith("```"):
            break
        block_lines.append(line)
    return "\n".join(block_lines).strip()


def validate_deck_spec(deck_spec: dict) -> list[dict[str, str]]:
    slides_value = deck_spec.get("slides")
    if not isinstance(slides_value, list) or not slides_value:
        fail("deck_spec.slides must be a non-empty list")
    return [normalize_slide(slide, index) for index, slide in enumerate(slides_value, start=1)]


def normalize_slide(slide: object, index: int) -> dict[str, str]:
    if not isinstance(slide, dict):
        fail(f"deck_spec.slides[{index}] must be an object")
    title = clean_value(str(slide.get("title", "")))
    body = normalize_slide_body(slide.get("body", ""))
    speaker_note = clean_value(str(slide.get("speaker_note", slide.get("speakerNote", ""))))
    if not title:
        fail(f"deck_spec.slides[{index}].title is required")
    if not body:
        fail(f"deck_spec.slides[{index}].body is required")
    if not speaker_note:
        fail(f"deck_spec.slides[{index}].speaker_note is required")
    return {"title": title, "body": body, "speaker_note": speaker_note}


def normalize_slide_body(value: object) -> str:
    if isinstance(value, list):
        return "\n".join(clean_value(str(item)) for item in value if clean_value(str(item)))
    return clean_value(str(value))


def requested_slide_count_from_values(values: dict[str, str], request: str) -> int:
    requested_slide_count = clean_value(values.get("requested_slide_count", ""))
    if requested_slide_count:
        if not requested_slide_count.isdigit():
            fail("requested_slide_count must be a number")
        return int(requested_slide_count)
    return requested_slide_count_from_text(request)


def requested_slide_count_from_text(value: str) -> int:
    match = re.search(r"(\d{1,2})\s*(?:장|slides?|pages?)", value, re.IGNORECASE)
    if match:
        return int(match.group(1))
    korean_numbers = {"한": 1, "두": 2, "세": 3, "네": 4, "다섯": 5, "여섯": 6, "일곱": 7, "여덟": 8, "아홉": 9, "열": 10}
    for word, count in korean_numbers.items():
        if word + "장" in value:
            return count
    return 0


def requested_formats_from_values(values: dict[str, str], request: str) -> list[str]:
    raw_value = clean_value(values.get("requested_formats", ""))
    if raw_value:
        return validate_requested_formats([format_value.strip().lower().lstrip(".") for format_value in re.split(r"[,/ ]+", raw_value) if format_value.strip()])
    normalized_request = request.lower()
    if "html만" in normalized_request or "html only" in normalized_request:
        return ["html"]
    return ["pptx", "pdf", "html", "notes"]


def validate_requested_formats(formats: list[str]) -> list[str]:
    normalized_formats = []
    for format_value in formats:
        if format_value in ["note", "notes", "txt", "speaker-notes"]:
            format_value = "notes"
        if format_value not in ["pptx", "pdf", "html", "notes"]:
            fail(f"unsupported requested format: {format_value}")
        if format_value not in normalized_formats:
            normalized_formats.append(format_value)
    if not normalized_formats:
        fail("requested_formats must include at least one format")
    return normalized_formats


def validate_design(design: str) -> None:
    required_fragments = ["colors:", "typography:", "layout:"]
    for fragment in required_fragments:
        if fragment not in design:
            fail(f"DESIGN.md is missing {fragment}")


def validate_presentation(contract: dict, presentation: str) -> None:
    if "design-source: DESIGN.md" not in presentation:
        fail("presentation.md must include design-source: DESIGN.md")
    if contract["mode"] != "capabilities":
        for token in ["InternKim capability deck", "김인턴이 할 수 있는 일"]:
            if token in presentation:
                fail(f"non-capabilities deck contains sample token: {token}")
    validate_presentation_slide_count(contract, presentation)
    validate_presentation_intent(contract, presentation)
    validate_presentation_slides(contract, presentation)


def validate_presentation_slide_count(contract: dict, presentation: str) -> None:
    actual_slide_count = count_marp_slides(presentation)
    expected_slide_count = int(contract["requested_slide_count"])
    if actual_slide_count != expected_slide_count:
        fail(f"requested {expected_slide_count} slides but presentation.md contains {actual_slide_count}")


def count_marp_slides(presentation: str) -> int:
    parts = presentation.split("---", 2)
    body = parts[2] if len(parts) >= 3 and parts[0].strip() == "" else presentation
    body = body.strip()
    if not body:
        return 0
    return len(re.split(r"\n---\n", body))


def validate_presentation_intent(contract: dict, presentation: str) -> None:
    normalized_presentation = presentation.lower()
    if not contains_intent_token(normalized_presentation, contract["topic"]):
        fail("presentation.md does not contain topic tokens")
    if not contains_intent_token(normalized_presentation, contract["slide_intent"]):
        fail("presentation.md does not contain slide_intent tokens")


def validate_presentation_slides(contract: dict, presentation: str) -> None:
    normalized_presentation = presentation.lower()
    for index, slide in enumerate(contract["slides"], start=1):
        if slide["title"].lower() not in normalized_presentation:
            fail(f"presentation.md is missing deck_spec slide {index} title")


def contains_intent_token(text: str, intent: str) -> bool:
    tokens = intent_tokens(intent)
    return not tokens or any(token in text for token in tokens)


def intent_tokens(value: str) -> list[str]:
    tokens = []
    for token in re.split(r"[^0-9A-Za-z가-힣]+", value.lower()):
        if len(token) >= 3 or re.search(r"[가-힣]", token):
            tokens.append(token)
    return tokens


def deck_intent_manifest(contract: dict) -> str:
    manifest = {
        "output_slug": contract["output_slug"],
        "mode": contract["mode"],
        "topic": contract["topic"],
        "slide_intent": contract["slide_intent"],
        "requested_slide_count": contract["requested_slide_count"],
        "requested_formats": contract["requested_formats"],
        "slide_count": len(contract["slides"]),
    }
    return json.dumps(manifest, ensure_ascii=False, indent=2)


if __name__ == "__main__":
    raise SystemExit(main())
