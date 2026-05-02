#!/usr/bin/env python3
import argparse
import pathlib
import shutil
import subprocess


def main() -> int:
    arguments = parse_arguments()
    working_directory_path = pathlib.Path.cwd()
    skill_directory_path = pathlib.Path(__file__).resolve().parents[1]
    slug = clean_slug(arguments.slug)
    brief = read_brief(arguments.brief)

    write_text(working_directory_path / "DESIGN.md", design_document(brief))
    write_text(working_directory_path / "presentation.md", presentation_document(brief))
    copy_runtime_file(skill_directory_path / "assets" / "build.sh", working_directory_path / "build.sh")
    copy_runtime_file(skill_directory_path / "scripts" / "extract_notes.py", working_directory_path / "extract_notes.py")

    subprocess.run(["bash", "-lc", f"NAME={shell_quote(slug)} ./build.sh"], cwd=working_directory_path, check=True)
    return 0


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--slug", required=True)
    parser.add_argument("--brief", default="brief.md")
    return parser.parse_args()


def clean_slug(value: str) -> str:
    cleaned = "".join(character.lower() if character.isalnum() else "-" for character in value)
    cleaned = "-".join(part for part in cleaned.split("-") if part)
    return cleaned or "presentation"


def read_brief(path: str) -> str:
    brief_path = pathlib.Path(path)
    if brief_path.exists():
        return brief_path.read_text(encoding="utf-8").strip()
    return "Blueclaw capabilities overview"


def write_text(path: pathlib.Path, content: str) -> None:
    path.write_text(content.strip() + "\n", encoding="utf-8")


def copy_runtime_file(source_path: pathlib.Path, target_path: pathlib.Path) -> None:
    shutil.copyfile(source_path, target_path)
    target_path.chmod(0o755)


def shell_quote(value: str) -> str:
    return "'" + value.replace("'", "'\"'\"'") + "'"


def design_document(brief: str) -> str:
    return "\n".join([
        "# Deck Design",
        "",
        "- Audience: company teammates who need a quick, practical view of Blueclaw.",
        f"- Source brief: {brief}",
        "- Palette: ink #17202a, blue #1f6feb, green #2e7d32, paper #ffffff.",
        "- Typography: system sans-serif with Korean-friendly fallback.",
        "- Layout: compact title, two-column capability cards, clear evidence footer.",
        "- Density: four to six short points per slide.",
    ])


def presentation_document(brief: str) -> str:
    return f"""---
marp: true
theme: default
paginate: true
size: 16:9
style: |
  section {{
    font-family: -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", "Noto Sans CJK KR", "Segoe UI", sans-serif;
    color: #17202a;
    padding: 54px;
  }}
  h1 {{
    color: #1f6feb;
    font-size: 44px;
  }}
  h2 {{
    color: #17202a;
    font-size: 32px;
  }}
  table {{
    font-size: 22px;
  }}
  strong {{
    color: #2e7d32;
  }}
---

# 김인턴이 할 수 있는 일

{brief}

**자료 생성, 조사, 자동화, 승인 흐름**을 하나의 업무 실행 경로로 묶습니다.

<!-- 오늘 요청은 김인턴의 가능 범위를 간단한 발표 자료로 보여주는 것입니다. -->

---

## 빠른 조사와 정리

| 상황 | 지원 방식 |
| --- | --- |
| 최신 정보 확인 | 웹 검색과 출처 기반 요약 |
| 긴 자료 검토 | 핵심 쟁점과 액션 아이템 정리 |
| 회의 준비 | 발표 흐름, 체크리스트, 후속 질문 구성 |

<!-- 단순 답변보다 팀원이 바로 쓸 수 있는 정리물을 만드는 데 초점을 둡니다. -->

---

## 업무 자동화

- 브라우저 기반 업무는 handoff 경계를 통해 처리
- 파일 생성은 workspace 안에서 추적 가능한 artifact로 남김
- 터미널 작업은 Firecracker guest 경계 안에서 실행
- 모든 tool call은 task event로 감사 가능

<!-- 권한이 넓어져도 실행 경계는 사용자의 호스트가 아니라 Blueclaw workspace입니다. -->

---

## 산출물 생성

- 발표자료: PPTX, PDF, HTML, speaker notes
- 문서/표/리포트: 요구 형식에 맞춰 생성
- 결과물은 final reply attachment evidence로 전달
- Google Workspace는 credentials가 있을 때 typed capability로만 사용

<!-- 로컬 파일 첨부와 Google 업로드를 혼동하지 않는 것이 중요합니다. -->

---

## 안전한 운영 원칙

1. Skills는 절차를 설명합니다.
2. Tools가 실제 권한을 부여합니다.
3. Profile allowlist가 노출 범위를 제한합니다.
4. Task events가 선택, 실행, 첨부를 기록합니다.

<!-- 작은 skill과 명확한 tool catalog가 Blueclaw의 장기적인 유지보수성을 만듭니다. -->
"""


if __name__ == "__main__":
    raise SystemExit(main())
