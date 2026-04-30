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
7. 이메일 발송, 외부 공유, Google import/publish, 파일 이동/삭제, 브라우저 제출, 터미널 write 명령은 승인 없이는 실행하지 않는다.

## Canonical 실행 경로

| 요청 | canonical path | orchestration 책임 |
|---|---|---|
| 일정 추가/조회 | ICS, CalDAV first, optional `calendar` skill | 시간/참석자/장소 누락 질문, 외부 참석자 승인, Google sync 선택 |
| 문서 생성 | DOCX, PDF, HTML, Markdown first, optional `create-gws-file` skill | 제목/공유 대상/본문 구조 결정, Google Docs import 선택 |
| 시트 생성 | XLSX, CSV first, optional `create-gws-file` skill | 시트 목적, 컬럼, 초기 데이터 구조 결정, Google Sheets import 선택 |
| 이메일 초안/발송 | `.eml` 또는 draft text first, optional Gmail bridge | 발송 전 preview와 `user.confirm` 강제 |
| 슬라이드 deck | `DESIGN.md` + HTML/PPTX/PDF first, optional Google Slides import | 목적/청중/톤/출력 형식 결정 |
| PDF 생성/읽기 | `pdf` skill | 한글 폰트, 템플릿, 출력 파일 연결 |
| 파일 전송 | native reply attachments | 파일 경로 검증, 메시지, 공유 대상 결정 |
| 브라우저 자동화 | `agent-browser` skill, `browser.*` capability | 사용자 입력 대기, 제출 승인, 관찰 결과 요약 |
| 로컬 파일 선택 | Companion `file.pick` | 로컬 경로 비노출, device temp path만 사용 |
| 기억 저장/검색 | Graphiti memory | 개인/직급/팀/회사 scope 선택 |
| 업무/출퇴근 | Blueclaw task DB, future attendance capability | 상태 전이, 담당자, audit 기록 |

## Orchestration wrappers

### `workspace-orchestrator`

일정, 문서, 시트, 이메일 요청을 portable artifact first로 라우팅하고, Google은 명시적 publish/import 단계로만 사용한다.

- Calendar 요청은 ICS/CalDAV를 먼저 만들고, 사용자가 Google Calendar를 원할 때 `calendar` skill을 사용한다.
- Docs 요청은 DOCX/HTML/PDF를 먼저 만들고, 사용자가 Google Docs 공동 편집을 원할 때 `create-gws-file` skill을 사용한다.
- Sheets 요청은 XLSX/CSV를 먼저 만들고, 사용자가 Google Sheets를 원할 때 `create-gws-file` skill을 사용한다.
- Email 요청은 `.eml` 또는 draft text를 먼저 만들고, Gmail 발송은 preview와 `user.confirm` 후 실행한다.
- Slides 요청은 `slide-orchestrator`로 넘긴다.

### `artifact-orchestrator`

파일 생성, 선택, 공유, platform attachment를 연결한다.

- 사용자 로컬 파일은 Companion `file.pick`으로 받고 로컬 경로를 노출하지 않는다.
- Mattermost/Slack/Signal 전송은 Blueclaw `FileAttachment`와 InternKim `reply.send` attachment 경로를 사용한다.
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

- 브라우저는 `browser.*` capability와 `agent-browser` skill 경계를 유지한다.
- 사용자 입력 대기는 `user.input`, 확인은 `user.confirm`으로 처리한다.
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
- "계약서 템플릿 채워줘"는 `document-orchestrator`가 누락 필드를 인터뷰한 뒤 DOCX 또는 PDF 생성으로 위임한다.
- "이 파일 보내줘"는 `artifact-orchestrator`가 기존 attachment 경로를 사용한다.
- "브라우저에서 로그인 기다렸다가 진행해줘"는 `local-orchestrator`가 Companion browser와 `user.input`/`user.confirm`을 사용한다.
- "메일 보내줘"는 preview와 수동 승인 없이는 발송하지 않는다.
