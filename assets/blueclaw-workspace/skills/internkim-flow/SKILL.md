---
name: internkim-flow
description: Add, find, update, or complete weekly work items when the user asks to add, record, request, find, change, or complete work, todos, 업무, deadlines, or task notes.
allowed-tools: task.add task.list task.update task.delete
---

# 업무 관리

Use the work capability operations when the user asks to add, record, request, find, update, or complete a work item.

Call `task.add`, `task.list`, `task.update`, or `task.delete` directly with its typed fields. The runtime supplies requester identity and approval, so never put identity in the input.

Rules:

- A user may add work directly only to their own 업무 목록.
- Work for another person must be added as `요청`, not as a direct assignment.
- Decide by intent, not by the noun the user used. Deliverables, deadlines, todos, requests, handoffs, and completion targets are work. Meetings, appointments, attendance blocks, locations, and time blocks are calendar events.
- If the user asks for a schedule but the content is a deadline-driven deliverable, call `task.add` and also call `calendar.add` when a due time should appear on the calendar.
- If the user says something is complete, prefer `task.update`. Use `calendar.update` only when the user clearly means changing a calendar event.
- If the user asks to edit, rename, change, or revise an existing work item, use `task.update`. Do not create a new work item with `task.add` for edits.
- Call `task.add` with a concise `title`. Add `goal`, `size`, `status`, `startDate`, and `endDate` only when the user's request supports them.
- Use `task.list` as the discovery step before `task.update` or `task.delete`. Copy the exact `taskID` from its result; never invent or reconstruct an ID.
- `task.list` lists the current week by default. Pass `weekFrom` and `weekTo` as week offsets from this week (0 this week, -1 last week, 1 next week) when the user asks for another period: 지난주 is `weekFrom -1, weekTo -1`; the last 4 weeks is `weekFrom -3, weekTo 0`; the whole history is a wide range such as `weekFrom -520`. Leave both unset for this week.
- Call `task.update` with the exact `taskID` and at least one explicit mutable field. To complete a task, set `status` to `완료`; an empty update is invalid.
- Call `task.delete` only with the exact `taskID`. Approval authorizes deletion but does not identify the target.
- When the user names a task instead of supplying an ID, call `task.list` with their wording in `query`, plus `targetPersonHint` or week offsets when supported by the request. If the result has one intended task, use its `taskID`; if multiple tasks could satisfy the request, ask the user which one.
- Use `targetPersonHint` only with `task.add` or `task.list` when a person is explicit. The hint may be a real name, a Mattermost `@handle`, or an email if the user provided one.
- If `task.update` or `task.delete` returns `flow_task_not_found`, do not claim success. Use `task.list` to rediscover the current exact ID when retrying is appropriate.
- Do not add the requester as a participant by default when asking another person to do work. Include the requester only when the user implies joint work, such as 같이, 함께, 나랑, 저랑, 우리, with me, with us, together, joint, or collaborate.
- Do not say a task was added until `task.add` succeeds.
- If `task.add` returns `flow_owner_ambiguous`, ask the user which candidate they mean and show the `@handle` candidates returned by the operation.
- Prefer a compact Markdown table when reporting created, updated, or listed tasks.
- After success, reply with the created task summary: 상태, 대분류, 종류, 크기, 참여자, 내용, 목표, 주간코드. Participants already include the owner, so do not list 담당자 separately.
- For 참여자, use `participantPresentations[].mention` when present; otherwise use `participantPresentations[].displayName`, then `participantNames`. Never create a mention by adding `@` to a display name yourself.
- In Korean replies, say `업무`, `업무 목록`, or `업무 관리`; do not call the product `Flow` unless the user explicitly uses that English name.
- If the operation fails, explain the failure honestly and do not fabricate a task.

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
