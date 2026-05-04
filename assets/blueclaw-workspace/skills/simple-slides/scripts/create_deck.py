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
    deck = deck_brief(brief)

    design_path = working_directory_path / "DESIGN.md"
    write_text(design_path, stitch_design_document(deck, stitch_design_tokens()))
    design = read_stitch_design_tokens(design_path)
    write_text(working_directory_path / "presentation.md", presentation_document(deck, design))
    copy_runtime_file(skill_directory_path / "assets" / "build.sh", working_directory_path / "build.sh")
    copy_runtime_file(skill_directory_path / "scripts" / "extract_notes.py", working_directory_path / "extract_notes.py")
    copy_runtime_file(skill_directory_path / "scripts" / "render_review.py", working_directory_path / "render_review.py")

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
    return "김인턴이 할 수 있는 일을 소개하는 발표 자료"


def write_text(path: pathlib.Path, content: str) -> None:
    path.write_text(content.strip() + "\n", encoding="utf-8")


def copy_runtime_file(source_path: pathlib.Path, target_path: pathlib.Path) -> None:
    shutil.copyfile(source_path, target_path)
    target_path.chmod(0o755)


def shell_quote(value: str) -> str:
    return "'" + value.replace("'", "'\"'\"'") + "'"


def deck_brief(brief: str) -> dict[str, str]:
    normalized_brief = compact_text(brief)
    lower_brief = normalized_brief.lower()
    if any(keyword in lower_brief for keyword in ["capabilities", "할 수", "기능", "역량", "지원 가능"]):
        return {
            "title": "김인턴이 할 수 있는 일",
            "subtitle": "조사에서 산출물 전달까지, 작은 업무를 끝까지 밀어주는 실행 파트너",
            "audience": "샘플 님과 팀원",
            "tone": "명료하고 믿음직한 업무 소개",
            "brief": normalized_brief,
            "mode": "capabilities",
        }
    return {
        "title": title_from_brief(normalized_brief),
        "subtitle": "핵심 메시지, 실행 흐름, 다음 단계를 한눈에 정리한 발표 자료",
        "audience": "업무 이해관계자",
        "tone": "차분하고 선명한 보고",
        "brief": normalized_brief,
        "mode": "generic",
    }


def compact_text(value: str) -> str:
    return " ".join(value.split()) or "발표 자료"


def title_from_brief(brief: str) -> str:
    title = brief
    for separator in [":", " - ", " — ", "\n"]:
        if separator in title:
            title = title.split(separator, 1)[-1]
    title = title.strip(" .")
    if len(title) > 34:
        title = title[:34].rstrip() + "..."
    return title or "발표 자료"


def stitch_design_tokens() -> dict[str, str]:
    return {
        "colors.background": "#F8FAFC",
        "colors.surface": "#FFFFFF",
        "colors.ink": "#111827",
        "colors.muted": "#64748B",
        "colors.teal": "#0F766E",
        "colors.amber": "#F97316",
        "colors.line": "#CBD5E1",
        "typography.display": "Paperlogy, Freesentation, Pretendard, Noto Sans KR",
        "typography.body": "Freesentation, Pretendard, Noto Sans KR",
        "layout.margin": "68px",
        "layout.radius": "8px",
    }


def stitch_design_document(deck: dict[str, str], design: dict[str, str]) -> str:
    return f"""---
name: "{deck["title"]} Presentation System"
version: "2.0"
colors:
  background: "{design["colors.background"]}"
  surface: "{design["colors.surface"]}"
  ink: "{design["colors.ink"]}"
  muted: "{design["colors.muted"]}"
  teal: "{design["colors.teal"]}"
  amber: "{design["colors.amber"]}"
  line: "{design["colors.line"]}"
typography:
  display: "{design["typography.display"]}"
  body: "{design["typography.body"]}"
layout:
  canvas: "16:9"
  margin: "{design["layout.margin"]}"
  rhythm: "8px"
  radius: "{design["layout.radius"]}"
---

# Deck Design

Audience: {deck["audience"]}

Tone: {deck["tone"]}

Visual direction: crisp Korean business slides with strong whitespace, structured cards, and teal/orange accents. Avoid the Marp default theme look: no giant tables as the main visual, no raw placeholder prose, and no unfinished black-and-white scaffold.
"""


def read_stitch_design_tokens(path: pathlib.Path) -> dict[str, str]:
    design = stitch_design_tokens()
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---"):
        return design
    parts = text.split("---", 2)
    if len(parts) < 3:
        return design
    current_section = ""
    for raw_line in parts[1].splitlines():
        line = raw_line.rstrip()
        if not line.strip():
            continue
        if not line.startswith(" ") and line.endswith(":"):
            current_section = line[:-1].strip()
            continue
        if not current_section or ":" not in line:
            continue
        key, value = line.split(":", 1)
        key = key.strip()
        value = value.strip().strip('"')
        if key and value:
            design[current_section + "." + key] = value
    return design


def presentation_document(deck: dict[str, str], design: dict[str, str]) -> str:
    if deck["mode"] == "capabilities":
        return capabilities_presentation(deck, design)
    return generic_presentation(deck, design)


def marp_header(title: str, design: dict[str, str]) -> str:
    return f"""---
marp: true
theme: default
paginate: false
size: 16:9
html: true
title: {title}
style: |
  section {{
    --background: {design["colors.background"]};
    --surface: {design["colors.surface"]};
    --ink: {design["colors.ink"]};
    --muted: {design["colors.muted"]};
    --teal: {design["colors.teal"]};
    --amber: {design["colors.amber"]};
    --line: {design["colors.line"]};
    font-family: {design["typography.body"]}, "Pretendard Variable", Pretendard, "Apple SD Gothic Neo", sans-serif;
    background: var(--background);
    color: var(--ink);
    padding: 58px {design["layout.margin"]};
    letter-spacing: 0;
  }}
  h1, h2, h3 {{
    font-family: {design["typography.display"]}, "Pretendard Variable", Pretendard, sans-serif;
    letter-spacing: 0;
    margin: 0;
  }}
  h1 {{ font-size: 70px; line-height: 0.98; font-weight: 850; color: var(--ink); }}
  h2 {{ font-size: 44px; line-height: 1.08; font-weight: 800; color: var(--ink); }}
  h3 {{ font-size: 23px; line-height: 1.18; font-weight: 780; color: var(--ink); }}
  p, li {{ font-size: 22px; line-height: 1.45; color: var(--ink); }}
  .eyebrow {{ color: var(--teal); font-weight: 800; font-size: 15px; text-transform: uppercase; margin-bottom: 18px; }}
  .subtitle {{ color: var(--muted); font-size: 25px; line-height: 1.42; max-width: 900px; margin-top: 24px; }}
  .accent {{ color: var(--teal); }}
  .mark {{ color: var(--amber); }}
  .grid {{ display: grid; gap: 18px; margin-top: 30px; }}
  .grid-2 {{ grid-template-columns: 1fr 1fr; }}
  .grid-3 {{ grid-template-columns: repeat(3, 1fr); }}
  .card {{ background: var(--surface); border: 1px solid var(--line); border-radius: {design["layout.radius"]}; padding: 24px; min-height: 145px; }}
  .card strong {{ color: var(--teal); }}
  .card p {{ color: var(--muted); font-size: 18px; margin: 10px 0 0; }}
  .compact {{ gap: 14px; margin-top: 0; }}
  .compact .card {{ min-height: 112px; padding: 20px 24px; }}
  .compact .card p {{ font-size: 17px; }}
  .number {{ color: var(--amber); font-size: 18px; font-weight: 850; margin-bottom: 10px; }}
  .band {{ background: var(--ink); color: white; border-radius: {design["layout.radius"]}; padding: 26px 30px; margin-top: 28px; }}
  .band p {{ color: #E5E7EB; margin: 0; }}
  .split {{ display: grid; grid-template-columns: 0.9fr 1.1fr; gap: 34px; align-items: center; margin-top: 30px; }}
  .metric {{ font-size: 54px; font-weight: 850; color: var(--teal); margin: 0; }}
  .pill-row {{ display: flex; flex-wrap: wrap; gap: 10px; margin-top: 24px; }}
  .pill {{ background: white; border: 1px solid var(--line); border-radius: 999px; padding: 9px 15px; font-size: 17px; color: var(--ink); }}
  .footer {{ position: absolute; left: {design["layout.margin"]}; right: {design["layout.margin"]}; bottom: 34px; display: flex; justify-content: space-between; color: var(--muted); font-size: 14px; }}
---
"""


def capabilities_presentation(deck: dict[str, str], design: dict[str, str]) -> str:
    return marp_header(deck["title"], design) + f"""
<!-- design-source: DESIGN.md -->

<div class="eyebrow">InternKim capability deck</div>

# 김인턴이<br><span class="accent">끝까지 처리하는 일</span>

<p class="subtitle">{deck["subtitle"]}</p>

<div class="pill-row">
<span class="pill">조사</span>
<span class="pill">자동화</span>
<span class="pill">파일 생성</span>
<span class="pill">승인 흐름</span>
<span class="pill">첨부 전달</span>
</div>

<div class="footer"><span>{deck["audience"]}</span><span>Blueclaw workspace artifacts</span></div>

<!-- 오늘은 김인턴이 말로만 답하는 도구가 아니라, 실제 산출물까지 만들고 첨부하는 업무 실행 경로임을 보여줍니다. -->

---

## 업무를 <span class="accent">산출물</span>로 바꾸는 흐름

<div class="grid grid-3">
<div class="card"><div class="number">01</div><h3>요청 해석</h3><p>대화 맥락, 목적, 필요한 결과물을 먼저 정리합니다.</p></div>
<div class="card"><div class="number">02</div><h3>도구 선택</h3><p>스킬은 절차를, profile allowlist는 실제 권한을 결정합니다.</p></div>
<div class="card"><div class="number">03</div><h3>증거 첨부</h3><p>완성 파일은 task event와 attachment evidence로 남깁니다.</p></div>
</div>

<div class="band"><p>핵심은 “가능하다고 말하기”가 아니라, workspace 안에서 만들어진 파일을 검증하고 전달하는 것입니다.</p></div>

<!-- 이 흐름 덕분에 요청자는 결과물이 어디서 왔고 어떤 도구가 쓰였는지 추적할 수 있습니다. -->

---

## 바로 맡기기 좋은 일

<div class="grid grid-2">
<div class="card"><h3>조사와 요약</h3><p>웹 검색, 긴 문서 정리, 회의 전 브리핑, 비교표 작성</p></div>
<div class="card"><h3>발표 자료</h3><p>Marp 기반 HTML, PPTX, PDF, speaker notes 생성</p></div>
<div class="card"><h3>반복 업무</h3><p>브라우저 handoff, 파일 정리, 체크리스트 실행</p></div>
<div class="card"><h3>업무 기록</h3><p>선택한 skill, tool call, 산출물 첨부를 task event로 보존</p></div>
</div>

<!-- “피피티 못 만든다”가 아니라, 로컬 파일 산출물은 항상 Marp 경로로 생성하는 것이 현재 계약입니다. -->

---

## 안전한 경계

<div class="split">
<div>
<p class="metric">/workspace</p>
<p class="subtitle">명령 실행과 파일 생성은 Blueclaw workspace 안에서만 이뤄집니다.</p>
</div>
<div class="grid compact">
<div class="card"><h3>Terminal</h3><p>Firecracker guest 경계 안에서 Marp/Bun 빌드를 실행합니다.</p></div>
<div class="card"><h3>Handoff</h3><p>브라우저 로그인이나 승인 작업은 Companion 경계로 넘깁니다.</p></div>
<div class="card"><h3>Audit</h3><p>각 실행은 task event에 남아 재현과 디버깅이 가능합니다.</p></div>
</div>
</div>

<!-- 권한이 커져도 host/root 권한이 아니라 guest/workspace 권한이라는 점이 중요합니다. -->

---

## 좋은 요청 예시

<div class="grid grid-2">
<div class="card"><h3>“이 내용으로 발표자료 만들어줘”</h3><p>PPTX, PDF, HTML, notes를 같이 첨부합니다.</p></div>
<div class="card"><h3>“자료 조사해서 팀 공유용으로 정리해줘”</h3><p>출처, 요약, 의사결정 포인트를 분리합니다.</p></div>
<div class="card"><h3>“브라우저에서 확인이 필요한 부분 넘겨줘”</h3><p>로그인/승인은 사용자 컴퓨터에서 처리합니다.</p></div>
<div class="card"><h3>“완료 여부를 증거로 남겨줘”</h3><p>파일 첨부와 task event를 기준으로 완료를 판단합니다.</p></div>
</div>

<div class="footer"><span>Next action</span><span>업무 목적 → tool path → artifact evidence</span></div>

<!-- 다음에는 원하는 주제, 톤, 청중만 주면 같은 구조로 바로 산출물을 만들 수 있습니다. -->
"""


def generic_presentation(deck: dict[str, str], design: dict[str, str]) -> str:
    return marp_header(deck["title"], design) + f"""
<!-- design-source: DESIGN.md -->

<div class="eyebrow">Presentation brief</div>

# {deck["title"]}

<p class="subtitle">{deck["subtitle"]}</p>

<div class="band"><p>{deck["brief"]}</p></div>

<!-- 이 발표는 요청의 목적과 청중을 먼저 정리한 뒤 핵심 메시지로 들어갑니다. -->

---

## 핵심 메시지

<div class="grid grid-3">
<div class="card"><div class="number">01</div><h3>무엇이 중요한가</h3><p>청중이 기억해야 할 한 문장을 먼저 고정합니다.</p></div>
<div class="card"><div class="number">02</div><h3>왜 지금인가</h3><p>배경과 필요성을 짧은 근거로 연결합니다.</p></div>
<div class="card"><div class="number">03</div><h3>무엇을 할 것인가</h3><p>다음 행동을 명확한 단위로 나눕니다.</p></div>
</div>

<!-- 핵심 메시지는 정보량보다 방향성이 중요합니다. -->

---

## 실행 흐름

<div class="grid grid-2">
<div class="card"><h3>현재 상황</h3><p>문제, 제약, 이미 확보한 정보를 분리합니다.</p></div>
<div class="card"><h3>선택지</h3><p>비교 가능한 기준으로 대안을 정리합니다.</p></div>
<div class="card"><h3>권장안</h3><p>가장 실용적인 경로와 이유를 짧게 제시합니다.</p></div>
<div class="card"><h3>다음 단계</h3><p>담당자와 일정, 필요한 확인 사항을 남깁니다.</p></div>
</div>

<!-- 청중이 회의 후 바로 움직일 수 있도록 구체성을 유지합니다. -->

---

## 다음 단계

<p class="metric">3</p>
<p class="subtitle">확인할 것, 결정할 것, 실행할 것을 세 줄로 마무리합니다.</p>

<div class="pill-row">
<span class="pill">확인</span>
<span class="pill">결정</span>
<span class="pill">실행</span>
</div>

<!-- 발표 후 액션 아이템을 분명하게 합의합니다. -->
"""


if __name__ == "__main__":
    raise SystemExit(main())
