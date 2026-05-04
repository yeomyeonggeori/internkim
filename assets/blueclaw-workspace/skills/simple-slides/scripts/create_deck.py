#!/usr/bin/env python3
import argparse
import json
import os
import pathlib
import re
import shutil
import subprocess


def main() -> int:
    arguments = parse_arguments()
    working_directory_path = pathlib.Path.cwd()
    skill_directory_path = pathlib.Path(__file__).resolve().parents[1]
    slug = clean_slug(arguments.slug)

    prepare_runtime_files(skill_directory_path, working_directory_path)
    brief = read_optional_text(working_directory_path / arguments.brief)
    ensure_deck_source(working_directory_path, skill_directory_path, slug, brief)

    if not arguments.no_build:
        build_deck(working_directory_path, slug)
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


def read_optional_text(path: pathlib.Path) -> str:
    if not path.exists():
        return ""
    return path.read_text(encoding="utf-8").strip()


def write_text(path: pathlib.Path, content: str) -> None:
    path.write_text(content.strip() + "\n", encoding="utf-8")


def prepare_runtime_files(skill_directory_path: pathlib.Path, working_directory_path: pathlib.Path) -> None:
    copy_runtime_file(skill_directory_path / "assets" / "build.sh", working_directory_path / "build.sh")
    copy_runtime_file(skill_directory_path / "scripts" / "extract_notes.py", working_directory_path / "extract_notes.py")
    copy_runtime_file(skill_directory_path / "scripts" / "render_review.py", working_directory_path / "render_review.py")


def copy_runtime_file(source_path: pathlib.Path, target_path: pathlib.Path) -> None:
    shutil.copyfile(source_path, target_path)
    target_path.chmod(0o755)


def ensure_deck_source(
    working_directory_path: pathlib.Path,
    skill_directory_path: pathlib.Path,
    slug: str,
    brief: str,
) -> None:
    presentation_path = working_directory_path / "presentation.md"
    if presentation_path.exists() and presentation_path.read_text(encoding="utf-8").strip():
        ensure_design_file(working_directory_path, skill_directory_path)
        return

    deck_spec = parse_deck_spec(brief)
    if deck_spec:
        ensure_design_file(working_directory_path, skill_directory_path)
        write_text(presentation_path, presentation_from_deck_spec(deck_spec, slug))
        return

    fail("presentation.md is missing. Write a Marp source file first, or include deck_spec in brief.md.")


def ensure_design_file(working_directory_path: pathlib.Path, skill_directory_path: pathlib.Path) -> None:
    design_path = working_directory_path / "DESIGN.md"
    if design_path.exists() and design_path.read_text(encoding="utf-8").strip():
        return
    shutil.copyfile(skill_directory_path / "assets" / "design.md", design_path)


def parse_deck_spec(brief: str) -> dict:
    deck_spec_text = extract_deck_spec_text(brief)
    if not deck_spec_text:
        return {}
    try:
        deck_spec = json.loads(deck_spec_text)
    except json.JSONDecodeError as error_value:
        fail(f"deck_spec is not valid JSON: {error_value}")
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


def presentation_from_deck_spec(deck_spec: dict, slug: str) -> str:
    title = clean_text(str(deck_spec.get("title", slug.replace("-", " ").title())))
    slides = normalized_slides(deck_spec)
    slide_sources = "\n\n---\n\n".join(slide_markdown(slide) for slide in slides)
    return front_matter(title) + "\n\n" + slide_sources


def normalized_slides(deck_spec: dict) -> list[dict[str, str]]:
    slides_value = deck_spec.get("slides")
    if not isinstance(slides_value, list) or not slides_value:
        fail("deck_spec.slides must be a non-empty list")
    slides = []
    for index, slide in enumerate(slides_value, start=1):
        if not isinstance(slide, dict):
            fail(f"deck_spec.slides[{index}] must be an object")
        title = clean_text(str(slide.get("title", f"Slide {index}")))
        body = normalize_slide_body(slide.get("body", ""))
        note = clean_text(str(slide.get("speaker_note", slide.get("speakerNote", ""))))
        slides.append({"title": title, "body": body, "note": note})
    return slides


def normalize_slide_body(value: object) -> list[str]:
    if isinstance(value, list):
        return [clean_text(str(item)) for item in value if clean_text(str(item))]
    text = clean_text(str(value))
    return [text] if text else []


def clean_text(value: str) -> str:
    return " ".join(value.strip().split())


def front_matter(title: str) -> str:
    return f"""---
marp: true
theme: default
paginate: false
size: 16:9
html: true
title: {json.dumps(title, ensure_ascii=False)}
style: |
  section {{
    --background: #F8FAFC;
    --surface: #FFFFFF;
    --ink: #111827;
    --muted: #64748B;
    --teal: #0F766E;
    --amber: #F97316;
    --line: #CBD5E1;
    font-family: Freesentation, Paperlogy, "Pretendard Variable", Pretendard, "Noto Sans KR", "Apple SD Gothic Neo", sans-serif;
    background: var(--background);
    color: var(--ink);
    padding: 58px 68px;
    letter-spacing: 0;
  }}
  h1, h2, h3 {{
    font-family: Paperlogy, Freesentation, "Pretendard Variable", Pretendard, "Noto Sans KR", sans-serif;
    letter-spacing: 0;
    margin: 0;
  }}
  h1 {{ font-size: 64px; line-height: 1.02; font-weight: 850; }}
  h2 {{ font-size: 42px; line-height: 1.08; font-weight: 800; }}
  p, li {{ font-size: 22px; line-height: 1.45; }}
  .subtitle {{ color: var(--muted); font-size: 25px; line-height: 1.42; max-width: 900px; margin-top: 24px; }}
  .grid {{ display: grid; gap: 18px; margin-top: 30px; }}
  .card {{ background: var(--surface); border: 1px solid var(--line); border-radius: 8px; padding: 24px; }}
  .accent {{ color: var(--teal); }}
---

<!-- design-source: DESIGN.md -->"""


def slide_markdown(slide: dict[str, object]) -> str:
    lines = [f"## {slide['title']}", ""]
    body = slide["body"]
    if isinstance(body, list) and body:
        lines.extend(f"- {item}" for item in body)
    else:
        lines.append("핵심 내용을 간결하게 정리합니다.")
    note = str(slide.get("note", "")).strip()
    if note:
        lines.extend(["", f"<!-- {note} -->"])
    return "\n".join(lines)


def build_deck(working_directory_path: pathlib.Path, slug: str) -> None:
    environment = os.environ.copy()
    environment["NAME"] = slug
    subprocess.run(["./build.sh"], cwd=working_directory_path, env=environment, check=True)


def fail(message: str) -> None:
    raise SystemExit(message)


if __name__ == "__main__":
    raise SystemExit(main())
