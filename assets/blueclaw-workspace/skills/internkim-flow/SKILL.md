---
name: internkim-flow
description: Add or request Flow weekly work tracker tasks when the user asks Intern Kim to add work, meetings, todos, requests, or task notes.
when_to_use: Use when the user asks to add, record, or request Flow tasks, todos, 업무, 회의, 미팅, 추가, 넣어, 등록, 요청, 할 일, or 할일.
allowed-tools:
  - flow.task.add
---

# InternKim Flow

Use `flow.task.add` when the user asks to add, record, request, or put a work item into Flow.

Rules:

- A user may add work directly only to their own Flow.
- Work for another person must be added as `요청`, not as a direct assignment.
- Do not say a task was added until `flow.task.add` succeeds.
- If `flow.task.add` returns `status: skipped_duplicate`, tell the user the matching task is already in Flow and ask whether to add another copy. Do not call the tool again unless the user explicitly says to add it anyway.
- When the user explicitly confirms adding a duplicate, call `flow.task.add` again with the same input and `allowDuplicate: true`.
- After success, reply with the created task summary: 담당자, 상태, 대분류, 종류, 크기, 내용, 목표, 주간코드.
- If the tool fails, explain the failure honestly and do not fabricate a task.

Size rubric:

| 크기 | 거리(Km) | 최대 예상 시간(H) | 예시(개발) | 예시(기타) | 비고 |
| --- | ---: | ---: | --- | --- | --- |
| XS | 1 | 1 | 아주 사소한 변경 | 전화 / 10분 회의 / 전달 / 정리 / 일정 조율 | 잠깐이면 끝낼 것 |
| S | 2 | 2 | 난이도 낮고 영향 범위 좁은 변경 | 30분 내외 회의 / 간단 문서 초안 / 고객 및 파트너 대응 / 브리핑 | 하루 여러 번도 처리 가능한 것 |
| M | 3 | 8 | 소형 기능 추가 / 영향 있는 변경 | 보고서 작성 / 2시간 이내 회의 / 외부 미팅 / 팀 간 조율 / 문서 작성 | 하루 날 잡고 해야 할 것 |
| L | 5 | 16 | 중형 기능 추가 / 다수 영향 있는 변경 | 중요 외부 미팅 / 중형 리서치 / 기획서 초안 / 정책 변경 | 이틀은 걸릴 것 |
| XL | 8 | 32 | 대형 기능 추가 / 복잡한 변경 / 외부 연동 | 재정비 / 협상 / 장시간 미팅 / 워크샵 | 일주일은 걸릴 것 |
| XXL | 13 | 128 | 마일스톤 | 파트너십 설계 / 계약 구조 설계 / 서비스 기획 / 정책 개편 | 반드시 하위 항목으로 쪼갤 것 |

거리(Km)는 업무의 크기를 나타내는 추상 단위다. 시간이나 난이도를 정확히 측정하려는 것이 아니라, 일이 어느 정도 규모인지 서로 이해하기 쉽게 표현하는 표시다. 거리 단위는 큰 일과 작은 일을 구분해 계획을 현실적으로 만들고, 과부하와 번아웃을 미리 확인하며, 작은 일이 끝없이 늘어지거나 미뤄지는 것을 막는다. 평가가 아니라 일정 안정화, 집중 유지, 과부하와 늘어짐 방지를 위한 최소한의 공통 언어다.
