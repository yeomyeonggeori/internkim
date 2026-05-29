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
        decision = read_json(review_directory_path / "review-decision.json")
        validate_decision(slide_review, decision)
    except ValueError as error:
        print(f"Review acceptance failed: {error}", file=sys.stderr)
        return 1
    print("Review acceptance passed.")
    return 0


def read_json(path: pathlib.Path) -> dict:
    if not path.exists():
        raise ValueError(f"{path.name} is required")
    return json.loads(path.read_text(encoding="utf-8"))


def validate_decision(slide_review: dict, decision: dict) -> None:
    blocking_issues = [
        issue for issue in decision.get("issues", [])
        if issue.get("severity") == "blocking"
    ]
    if blocking_issues:
        raise ValueError("blocking issues remain in review-decision.json")

    inspected_evidence = set(decision.get("inspectedEvidence", []))
    required_evidence = {
        sheet.get("filename")
        for sheet in slide_review.get("contactSheets", [])
        if sheet.get("filename")
    }
    missing_evidence = sorted(required_evidence - inspected_evidence)
    if missing_evidence:
        raise ValueError("contact sheets were not inspected: " + ", ".join(missing_evidence))

    warnings = deterministic_warnings(slide_review)
    accepted_warnings = set(decision.get("acceptedWarnings", []))
    if warnings and not accepted_warnings:
        raise ValueError("deterministic warnings require acceptedWarnings or a rebuilt clean deck")

    if not str(decision.get("summary", "")).strip():
        raise ValueError("review decision summary is required")


def deterministic_warnings(slide_review: dict) -> list[str]:
    warnings = []
    for slide in slide_review.get("slides", []):
        for warning in slide.get("warnings", []):
            warnings.append(f"slide {slide.get('index')}: {warning}")
    return warnings


if __name__ == "__main__":
    raise SystemExit(main())
