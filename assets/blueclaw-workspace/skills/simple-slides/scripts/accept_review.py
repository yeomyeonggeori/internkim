#!/usr/bin/env python3
import json
import pathlib
import sys


def main() -> int:
    if len(sys.argv) != 2:
        print("Usage: accept_review.py <review-dir>", file=sys.stderr)
        return 2
    review_directory_path = pathlib.Path(sys.argv[1])
    try:
        slide_review = read_json(review_directory_path / "slide-review.json")
        decision = read_optional_json(review_directory_path / "review-decision.json")
        warnings = review_warnings(slide_review, decision)
    except ValueError as error:
        print(f"Review acceptance failed: {error}", file=sys.stderr)
        return 1
    if warnings:
        print("Review acceptance completed with warnings:")
        for warning in warnings:
            print(f"- {warning}")
        return 0
    print("Review acceptance passed.")
    return 0


def read_json(path: pathlib.Path) -> dict:
    if not path.exists():
        raise ValueError(f"{path.name} is required")
    return json.loads(path.read_text(encoding="utf-8"))


def read_optional_json(path: pathlib.Path) -> dict:
    if not path.exists():
        return {}
    return json.loads(path.read_text(encoding="utf-8"))


def review_warnings(slide_review: dict, decision: dict) -> list[str]:
    warnings = []
    if not decision:
        warnings.append("review-decision.json is missing; attach the usable deck with the review report notes if the requested file exists")
        return warnings
    inspected_evidence = set(decision.get("inspectedEvidence", []))
    required_evidence = {
        sheet.get("filename")
        for sheet in slide_review.get("contactSheets", [])
        if sheet.get("filename")
    }
    missing_evidence = sorted(required_evidence - inspected_evidence)
    if missing_evidence:
        warnings.append("contact sheets were not inspected: " + ", ".join(missing_evidence))

    deterministic_review_warnings = deterministic_warnings(slide_review)
    accepted_warnings = set(decision.get("acceptedWarnings", []))
    remaining_notes = decision.get("remainingNotes", [])
    issues = decision.get("issues", [])
    if deterministic_review_warnings and not accepted_warnings and not remaining_notes and not issues:
        warnings.append("deterministic warnings were not addressed in acceptedWarnings, remainingNotes, issues, or a rebuilt clean deck")

    if not str(decision.get("summary", "")).strip():
        warnings.append("review decision summary is missing")
    return warnings


def deterministic_warnings(slide_review: dict) -> list[str]:
    warnings = []
    for slide in slide_review.get("slides", []):
        for warning in slide.get("warnings", []):
            warnings.append(f"slide {slide.get('index')}: {warning}")
    return warnings


if __name__ == "__main__":
    raise SystemExit(main())
