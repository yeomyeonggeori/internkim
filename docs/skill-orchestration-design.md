# Existing-first Skill Orchestration Design

## 목적

Intern Kim의 기능을 늘릴 때 기존 설계를 망치지 않기 위한 skill 설계 기준이다. 새 기능마다 구현 skill을 추가하지 않고, 먼저 인증 없는 portable artifact를 만들며, Google Workspace는 필요할 때 가져가거나 공유하는 optional target으로 둔다. 새 skill은 필요한 경우에만 여러 기존 skill과 capability를 조합하는 얇은 orchestration layer로 둔다.

## 설계 원칙

1. 기존 skill이 실행을 맡고, orchestration은 판단과 연결만 맡는다.
2. Blueclaw는 의도 판단, 질문, 계획을 담당한다.
3. `internkim-capabilityd`는 secret-bearing 실행, provider 선택, platform reply를 담당한다.
4. Companion은 사용자 로컬 브라우저, 파일, 입력, 승인을 담당한다.
5. Graphiti는 기억 저장과 검색을 담당한다.
6. Google Workspace는 기본값이 아니라 import/export/publish target이다.
7. 이메일 발송, 외부 공유, Google import/publish, 파일 이동/삭제, 브라우저 제출, 터미널 write 명령은 승인 없이는 실행하지 않는다. Website publish capability는 prototype 생성의 기본 완료 단계이며 descriptor상 승인 불필요 작업이다.

## Skill and Tool Contract

Skill은 절차와 판단 기준이다. Kernel tool은 LLM이 직접 호출할 수 있는 실행 API다. Domain capability는 `/workspace/tools/capability` CLI를 통해 호출하는 로컬 bridge API다. Skill은 tool schema, 승인 정책, side-effect 정책을 다시 정의하지 않는다.

LLM에 노출되는 kernel tool은 compact fixed set으로 유지한다. 기본 kernel은 `terminal_run`, `ask_input`, `ask_confirm`, `file_deliver`, `skill_search`, `file_read`, `file_write`, `file_edit`, `file.patch`, `file_preview`, `image_read`다. `ask.input.choices`가 비어 있으면 주관식 입력이고, 값이 있으면 선택지 또는 직접 입력을 받는다. Interactive terminal session 동작은 `terminal_run`의 `mode=session_start|session_write|session_status|session_close`로만 표현한다. WorkKind, selected skill, pinned recovery, profile별 bundle은 직접 tool palette를 확장하지 않는다.

`SKILL.md`는 선택될 때 초기 LLM context에 들어가는 실행 지침이다. 일반 skill은 8 KB 이하, 복잡한 artifact skill은 12 KB 이하를 목표로 하고, repository hard gate는 15 KB 및 300 lines다. 이 한계를 넘는 skill 문서는 prompt-runtime bug로 간주한다. 긴 reference는 `references/`, 반복 실행 로직은 `scripts/`, 재사용 asset은 `assets/`에 두고 `SKILL.md`에는 언제 읽거나 실행해야 하는지만 쓴다. 초기 prompt builder는 선택된 `SKILL.md` body만 포함해야 하며 scripts, references, assets 내용을 자동으로 붙이면 안 된다.

Portable `SKILL.md` metadata는 Agent Skills 표준을 따른다. Blueclaw bundled skills must not depend on `allowed-tools` as a runtime contract. If legacy skill metadata contains `allowed-tools`, treat it as non-authoritative documentation only; kernel exposure, capability access, approval, and policy are owned by runtime configuration and capability descriptors.

Kernel tool 설명, input schema, output schema, policy resource, side-effect class, approval requirement는 fixed ToolSet이 소유한다. Domain operation 설명, schema, policy, approval requirement는 capability descriptor가 소유한다. Prompt의 "Available tools", structured output schema, runtime invocation은 fixed kernel ToolSet에서 나오고, domain operation discovery는 capability CLI catalog/list/describe에서 나온다.

WorkflowContract는 반복되는 업무 흐름의 공통 계약이다. WorkKind, step working set tool group, intent별 evidence tool 선택을 한 곳에서 정의한다. Intake, skill selection, outcome contract, turn runner는 이 계약을 소비한다. 새 업무 흐름이 생기면 completion gate나 skill prompt에 예외를 추가하지 않고 WorkflowContract와 Tool descriptor를 갱신한다.

완료 판단은 Skill이 아니라 OutcomeContract와 observation이 소유한다. Skill은 사용 절차를 설명할 수 있지만 operation별 hard gate를 소유하지 않는다. 업무 등록은 `flow.task.add`, 업무 목록은 `flow.task.list`, 업무 수정/완료는 `flow.task.update`처럼 WorkflowContract가 현재 intent에 맞는 evidence operation을 고르고, 그 operation의 성공 observation이 있어야 완료다.

CompletionGate는 tool 이름을 특별 취급하지 않는다. `ToolDefinition.SideEffectClass`, `ToolRecoveryCard.SideEffect`, 또는 capability descriptor를 통해 상태 변경이 필요한 evidence requirement인지 판단한다. 상태 변경 operation은 성공 observation 없이 완료할 수 없고, read/computation operation은 답변 대체가 가능한 경우에만 fallback으로 완료할 수 있다.

`no_tool_fallback`은 순수 계산, 설명, 조회처럼 답변으로 대체 가능한 실패에만 허용한다. 메시지 전송, 업무 변경, 일정 변경, 스킬 변경처럼 외부 상태가 바뀌어야 하는 작업은 fallback 문장으로 완료 처리하지 않는다.

Coding-agent 작업은 지원 대상이다. Shell quoting이나 ad-hoc heredoc에 의존하면 코딩 품질과 회복성이 떨어지므로 `file_read`, `file_write`, `file_edit`, `file.patch`는 kernel에 포함한다. Attachment/document entrypoint인 `file_preview`와 visual attachment entrypoint인 `image_read`도 kernel에 포함한다. 이 파일 도구들은 virtual workspace path와 requester permissions를 지켜야 하며, delivery는 `file_deliver`가 담당한다. Legacy delivery aliases are not model-facing kernel tools. Domain operations such as tasks, calendar, mail, browser, and website publish remain capability CLI operations.

## Canonical 실행 경로

| 요청 | canonical path | orchestration 책임 |
|---|---|---|
| 일정 추가/조회 | ICS, CalDAV first, optional `calendar` skill | 시간/참석자/장소 누락 질문, 외부 참석자 승인, Google sync 선택 |
| 문서 생성 | DOCX, PDF, HTML, Markdown first, optional `create-gws-file` skill | 제목/공유 대상/본문 구조 결정, Google Docs import 선택 |
| 시트 생성 | XLSX, CSV first, optional `create-gws-file` skill | 시트 목적, 컬럼, 초기 데이터 구조 결정, Google Sheets import 선택 |
| 이메일 초안/발송 | `.eml` 또는 draft text first, optional Gmail bridge | 발송 전 preview와 `user_confirm` 강제 |
| 슬라이드 deck | `DESIGN.md` + HTML/PPTX/PDF first, optional Google Slides import | 목적/청중/톤/출력 형식 결정 |
| PDF 생성/읽기 | `pdf` skill | 한글 폰트, 템플릿, 출력 파일 연결 |
| 파일 전송 | native reply attachments | 파일 경로 검증, 메시지, 공유 대상 결정 |
| 브라우저 자동화 | `agent-browser` skill, `browser.*` capability | 사용자 입력 대기, 제출 승인, 관찰 결과 요약 |
| 로컬 파일 선택 | Companion `file_pick` | 로컬 경로 비노출, device temp path만 사용 |
| 기억 저장/검색 | Graphiti memory | 개인/직급/팀/회사 scope 선택 |
| 업무/출퇴근 | Blueclaw task DB, future attendance capability | 상태 전이, 담당자, audit 기록 |

## Orchestration wrappers

### `workspace-orchestrator`

일정, 문서, 시트, 이메일 요청을 portable artifact first로 라우팅하고, Google은 명시적 publish/import 단계로만 사용한다.

- Calendar 요청은 ICS/CalDAV를 먼저 만들고, 사용자가 Google Calendar를 원할 때 `calendar` skill을 사용한다.
- Docs 요청은 DOCX/HTML/PDF를 먼저 만들고, 사용자가 Google Docs 공동 편집을 원할 때 `create-gws-file` skill을 사용한다.
- Sheets 요청은 XLSX/CSV를 먼저 만들고, 사용자가 Google Sheets를 원할 때 `create-gws-file` skill을 사용한다.
- Email 요청은 `.eml` 또는 draft text를 먼저 만들고, Gmail 발송은 preview와 `user_confirm` 후 실행한다.
- Slides 요청은 `slide-orchestrator`로 넘긴다.

### `artifact-orchestrator`

파일 생성, 선택, 공유, platform attachment를 연결한다.

- 사용자 로컬 파일은 Companion `file_pick`으로 받고 로컬 경로를 노출하지 않는다.
- Mattermost/Slack/Signal 전송은 Blueclaw `FileAttachment`와 InternKim `reply.send` attachment 경로를 사용한다.
- 생성 작업은 `tmp/<slug>`에서 시작하고, 최종본만 `file_deliver`로 전달한다. 장기 보관이 필요한 경우에만 명시된 `artifacts/<slug>`, circle, 또는 shared 위치에 파일을 만든 뒤 전달한다.
- 외부 공유와 Google Drive publish는 수신자, 권한, 파일명을 요약하고 승인 후 실행한다.
- 장기 보관이 필요할 때만 artifact registry로 승격한다.

### `document-orchestrator`

계약서, 회의록, 사업계획서처럼 컨텍스트 수집이 필요한 문서 작업을 조율한다.

- 주어진 컨텍스트와 템플릿에서 채울 수 있는 필드를 먼저 채운다.
- 누락 필드는 한 번에 하나씩 질문한다.
- 최종 생성은 DOCX/PDF/HTML/Markdown을 먼저 만들고, 필요할 때 `create-gws-file`, `pdf`, `simple-slides` 중 목적에 맞는 기존 skill로 publish한다.
- 회의록 action item은 task 생성 후보로 분리한다.

### `slide-orchestrator`

슬라이드 요청은 Stitch-compatible `DESIGN.md`를 먼저 만들고, 그 design system을 적용해 HTML/PPTX/PDF 산출물을 만든다.

- 간단한 Google Slides와 고급 HTML 슬라이드를 같은 deck pipeline으로 다루되, 기본 출력은 HTML/PPTX/PDF다.
- `DESIGN.md`는 YAML token front matter와 Markdown rationale을 포함한다.
- source files는 `file_write` 또는 bundled skill script로 `tmp/<deck-slug>/DESIGN.md`와 `tmp/<deck-slug>/presentation.md`에 작성한다.
- build는 `terminal_run`으로 `workingDirectoryPath=tmp/<deck-slug>`에서 `/workspace/skills/simple-slides/scripts/build.sh`를 실행한다.
- output은 `tmp/<deck-slug>/build/` 아래에 만들고, 최종본만 `file_deliver`로 전달한다.
- `simple-slides`는 global PATH의 Marp를 선택하지 않는다. Rootfs 선설치 Marp entrypoint를 사용하거나 requester tmp의 skill-local install을 사용하고, runtime temp/cache/home은 task build tmp 아래에 둔다.
- 폰트는 Korean-first로 고른다. 기본 조합은 Paperlogy display + Freesentation body이며, 기술/모빌리티 덱은 A2Z display + Freesentation body를 우선한다. 후보와 import/cache 경로는 `assets/blueclaw-workspace/fonts/korean-fonts.tsv`와 `simple-slides`의 `references/design-system.md`를 따른다.
- 시각 품질 검증과 출력 파일 연결은 `simple-slides` 규칙을 따른다.
- Google Slides는 import target이며, 새 Google Slides 전용 skill을 만들지 않는다.

### `task-orchestrator`

업무 투두와 출퇴근 흐름을 조율한다.

- 업무 상태는 요청, 예정, 중단, 진행, 완료, 기각을 기본 상태로 둔다.
- 상대방 처리 요청은 task assignee와 platform notification으로 표현한다.
- 출퇴근 기록은 별도 attendance schema가 생길 때까지 task DB와 future capability 계약을 문서화한다.

### `memory-orchestrator`

기억 scope 선택을 조율한다.

- 개인 선호, 개인 일정, 개인 이메일 맥락은 `personal:{staffID}`로 둔다.
- 직급별 규칙과 권한은 `role:{title}`로 둔다.
- 팀 회의록과 팀 업무 맥락은 `team:{department}`로 둔다.
- 회사 정책, 계약서 템플릿, 공용 DB 구조는 `company`로 둔다.

### `local-orchestrator`

사용자 컴퓨터에서만 가능한 일을 Companion으로 보낸다.

- 브라우저는 발화자 Companion browser를 우선 사용하고, Companion이 없을 때만 내부 Lightpanda fallback으로 단순 텍스트 탐색을 처리한다.
- 로그인, MFA, captcha, 민감 입력은 `browser_handoff`로 발화자의 Companion browser 안에서 처리한다.
- 일반 사용자 입력 대기는 `user.input`, irreversible action 확인은 `user_confirm`으로 처리한다.
- 파일/디렉토리 정리는 dry-run 결과를 먼저 보여주고 승인 후 실행한다.
- 터미널은 dev/admin profile 전용으로 유지한다.

## 요청별 매핑

| 사용자 요청 | orchestration wrapper | canonical skill/capability |
|---|---|---|
| 일정 추가 | `workspace-orchestrator` | ICS/CalDAV, optional `calendar` |
| 매일 아침 개인 일정 알림 | `workspace-orchestrator` | local schedule cache + ICS/CalDAV, optional `calendar` |
| 간단한 Google Slides | `slide-orchestrator` | `DESIGN.md` + HTML/PPTX/PDF via `simple-slides`, optional Google import |
| 애니메이션 HTML 슬라이드 | `slide-orchestrator` | `DESIGN.md` + `simple-slides` 확장 |
| 계약서 작성 | `document-orchestrator` | DOCX/PDF first, optional `create-gws-file` |
| Google Sheet 생성 | `workspace-orchestrator` | XLSX/CSV first, optional `create-gws-file` |
| Google Docs 생성 | `workspace-orchestrator` | DOCX/HTML/PDF first, optional `create-gws-file` |
| 회의록 정리 | `document-orchestrator` | Markdown/DOCX first + task 후보 |
| DB 생성 | `task-orchestrator` | future DB capability |
| 출퇴근 기록 | `task-orchestrator` | future attendance capability |
| 업무 투두 | `task-orchestrator` | Blueclaw task DB |
| 파일 생성 및 공유 | `artifact-orchestrator` | native reply attachments |
| 파일 인식/분석 | `artifact-orchestrator` | future artifact ingest |
| 직원별/직급별 기억 분리 | `memory-orchestrator` | Graphiti memory |
| 이메일 정리/작성/발송 | `workspace-orchestrator` | `.eml`/draft text first, optional Gmail bridge |
| 파일/디렉토리 정리 | `local-orchestrator` | Companion local capability |
| 터미널 인터랙티브 모드 | `local-orchestrator` | Blueclaw terminal, admin profile |
| 사업계획서 | `document-orchestrator` | DOCX/XLSX/PPTX/HTML first, optional Google import |
| 메인 컴퓨터 브라우저 제어 | `local-orchestrator` | `agent-browser`, `browser.*` |
| 문서 인식 | `artifact-orchestrator` | future artifact ingest |
| 이미지 생성 | `artifact-orchestrator` | future image provider |

## 중복 구현 방지 체크리스트

- 기존 skill로 실행 가능한 작업인지 먼저 확인한다.
- Google 인증을 요구하기 전에 ICS, CalDAV, DOCX, XLSX, CSV, HTML, PDF, PPTX로 처리할 수 있는지 확인한다.
- artifact 생성 실패를 Google/Gamma/Canva 권유로 바꾸기 전에 tool observation의 실제 failure stage와 stderr tail을 확인한다.
- 새 shell bridge를 만들기 전에 portable artifact path와 `create-gws-file`, `calendar`, `simple-slides`가 처리하는지 확인한다.
- Google import/export는 기본 생성 후 선택 단계로 둔다.
- 새 attachment uploader를 만들기 전에 Blueclaw `FileAttachment`와 InternKim `reply.send` attachment 경로가 처리하는지 확인한다.
- 새 browser adapter를 만들기 전에 `browser.*`와 `agent-browser` 경로가 처리하는지 확인한다.
- 새 memory table을 만들기 전에 Graphiti scope로 해결 가능한지 확인한다.
- 터미널을 제품 기능으로 승격하기 전에 typed capability로 표현할 수 있는지 확인한다.

## Acceptance scenarios

- "내일 3시에 미팅 잡아줘"는 `workspace-orchestrator`가 ICS/CalDAV 이벤트를 만들고, 사용자가 원하면 Google Calendar에도 동기화한다.
- "시트 하나 만들어줘"는 `workspace-orchestrator`가 XLSX/CSV를 만들고, 사용자가 원하면 Google Sheets로 가져간다.
- "발표자료 만들어줘"는 `slide-orchestrator`가 `DESIGN.md`를 먼저 만든 뒤 HTML/PPTX/PDF를 만들고, 사용자가 원하면 Google Slides로 가져간다.
- "pptx 파일로 줘"처럼 required artifact 요청이면 `file_deliver` completion evidence가 있어야 성공이다. 텍스트 초안 제안은 완료가 아니다.
- "계약서 템플릿 채워줘"는 `document-orchestrator`가 누락 필드를 인터뷰한 뒤 DOCX 또는 PDF 생성으로 위임한다.
- "이 파일 보내줘"는 `artifact-orchestrator`가 기존 attachment 경로를 사용한다.
- "브라우저에서 로그인 기다렸다가 진행해줘"는 `local-orchestrator`가 Companion browser와 `user.input`/`user_confirm`을 사용한다.
- "메일 보내줘"는 preview와 수동 승인 없이는 발송하지 않는다.
