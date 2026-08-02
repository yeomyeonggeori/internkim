# 기능 확장을 위한 설계 변경안

## 목적

아래 기능들을 Intern Kim이 안정적으로 처리하려면 개별 skill만 늘리는 방식으로는 부족하다.

- 일정, 문서, 시트, 슬라이드, 이메일 작업
- 매일 개인별 일정 알림
- 계약서, 회의록, 사업계획서 같은 문서 작성
- 업무, 출퇴근, DB, 기억, 파일, 이메일 관리
- 사용자 컴퓨터 브라우저 제어, 파일 선택, 문서 인식, 이미지 생성

핵심 변경은 Blueclaw가 작업 의도를 판단하고, InternKim이 실행 위치와 권한을 선택하며, Companion이 사용자 로컬 자원을 안전하게 다루는 구조를 더 명확히 만드는 것이다.

## 현재 기반

이미 있는 기반은 계속 사용한다.

| 영역 | 현재 기반 | 유지할 점 |
|---|---|---|
| 대화 런타임 | Blueclaw | 정책, task, ACL, prompt, skill 선택을 담당 |
| 비밀 정보 실행 | `internkim-capabilityd` | OpenRouter, platform token, device browser 같은 secret-bearing 작업을 직접 보유 |
| 사용자 로컬 실행 | `internkim-companion` | `user_confirm`, `user.input`, `file_pick`, `browser.*`를 사용자 컴퓨터에서 실행 |
| Portable artifacts | ICS, CalDAV, DOCX, XLSX, CSV, HTML, PDF | Google 인증 없이 먼저 생성/공유 가능한 기본 산출물 |
| Google Workspace | `gws`, `gws-bot`, Apps Script bridge | optional import/export/publish target으로 유지 |
| 기억 | Graphiti memory sidecar | 대화, 업무, 사람, 파일 요약을 장기 기억으로 저장 |
| 사용자 플로우 문서 | `docs/flows/user/*` | 기능별 UX 기준으로 유지 |

현재 부족한 것은 기능별 실행 계약, 권한 승인 흐름, 산출물 관리, 개인별 preference 저장, 반복 작업 스케줄러, 문서/파일 ingest pipeline이다.

## 공통 설계 변경

### 0. Portable-first 산출물 원칙

Google Workspace는 편리한 publishing target이지만 기본 실행 경로가 되면 인증, 권한, 계정 정책이 모든 기능의 선행 조건이 된다. 기본값은 인증 없는 표준 파일과 프로토콜로 산출물을 만들고, 필요할 때만 Google로 가져가거나 동기화한다.

| 작업 | 1차 산출물 | 선택적 Google 경로 |
|---|---|---|
| 일정 추가 | `.ics`, CalDAV event | Google Calendar import/sync |
| 개인 일정 알림 | local schedule cache, CalDAV/ICS feed | Google Calendar read bridge |
| 문서 작성 | `.docx`, `.pdf`, `.html`, Markdown | Google Docs import |
| 시트 생성 | `.xlsx`, `.csv` | Google Sheets import |
| 슬라이드 생성 | `DESIGN.md` + standalone `.html`, `.pptx`, `.pdf` | Google Slides import |
| 이메일 작성 | `.eml`, draft text | Gmail draft/send bridge |
| 계약서/회의록/사업계획서 | `.docx` 또는 `.pdf` + metadata | Google Docs/Drive publish |

이 원칙 때문에 orchestration은 Google URL을 최종 결과의 유일한 형태로 보지 않는다. 먼저 artifact를 만들고, 사용자가 Google 공유나 공동 편집을 원할 때 import/publish를 별도 단계로 실행한다.

### 1. Capability registry를 업무 도구 전체로 확장

현재 `internal/capabilities/protocol.go`는 LLM, browser, user input, file pick 중심이다. 여기에 업무 도구를 typed capability로 추가한다.

추가할 capability 예시는 다음과 같다.

| capability | 실행 위치 | 용도 |
|---|---|---|
| `calendar.event.create` | device, CalDAV/ICS provider | 일정 추가 |
| `calendar.event.list` | device, CalDAV/ICS provider | 일정 조회와 알림 계산 |
| `artifact.document.create` | device | DOCX/PDF/HTML/Markdown 문서 생성 |
| `artifact.sheet.create` | device | XLSX/CSV 시트 생성 |
| `artifact.slides.create` | device | `DESIGN.md` 기반 HTML/PPTX/PDF 슬라이드 생성 |
| `artifact.share` | device/platform | 파일 공유 |
| `google.import` | device, Google bridge | portable artifact를 Google Docs/Sheets/Slides/Drive로 가져가기 |
| `google.calendar.sync` | device, Google bridge | portable 일정과 Google Calendar 동기화 |
| `email.draft.create` | device | `.eml` 또는 draft text 생성 |
| `email.message.send` | device, provider bridge | 승인 후 이메일 발송 |
| `task.create`, `task.transition` | Blueclaw DB | 업무 투두 상태 관리 |
| `attendance.clock` | Blueclaw DB | 출퇴근 기록 |
| `artifact.ingest` | device, Companion | 파일 인식과 분석 |
| `terminal_run` `mode=session_start` | device admin profile | 인터랙티브 터미널 |
| `image_generate` | remote or local model | 이미지 생성 |

Blueclaw skill은 shell 명령 문자열을 직접 기억하기보다 이 capability 이름을 기준으로 요청해야 한다. `capabilityd`가 실제 provider를 선택하고, provider는 portable file generator, CalDAV, Apps Script, `gws-bot`, Companion, DB, remote model 중 하나가 된다.

### 2. 사용자 identity와 resource scope를 일급 개념으로 승격

Calendar, email, file sharing, 기억 분리는 모두 "누구의 권한으로 실행하는가"가 핵심이다.

필요한 변경:

- `staff`와 platform account link를 연결하는 identity map 추가
- CalDAV 계정, email 계정, Google OAuth 사용자, Google service account, Mattermost/Slack/Signal user를 같은 staff로 매핑
- capability request에 `actorStaffID`, `targetStaffID`, `resourceScope`를 명시
- 팀 공용 자료와 개인 자료를 분리하는 scope 추가

권장 scope:

| scope | 설명 |
|---|---|
| `personal:{staffID}` | 개인 캘린더, 개인 이메일, 개인 기억 |
| `role:{title}` | 직급별 업무 규칙, 권한, 템플릿 |
| `team:{department}` | 팀 문서, 회의록, 공용 task |
| `company` | 회사 전체 정책, 계약서 템플릿, 공용 DB |
| `task:{taskID}` | 특정 업무의 산출물과 대화 |
| `artifact:{artifactID}` | 특정 파일/문서 분석 결과 |

### 3. 승인 정책을 capability 단위로 관리

이메일 발송, 외부 공유, 파일 정리, 브라우저 클릭, 터미널 실행은 같은 승인 모델을 써야 한다.

필요한 정책:

| 작업 | 기본 정책 |
|---|---|
| 이메일 발송 | 매번 `user_confirm` 필수 |
| 외부 공유 | 수신자, 권한, 파일명 확인 후 승인 |
| 캘린더 참석자 초대 | 참석자 이메일이 외부 도메인이면 승인 |
| Google import/publish | 대상 계정, 파일명, 공유 범위 확인 후 승인 |
| InternKim `site.app.publish` | 프로토타입 생성 완료 단계이므로 승인 불필요 |
| 파일 삭제/이동 | dry-run 요약 후 승인 |
| 터미널 write 명령 | admin/dev profile에서만 승인 후 실행 |
| 브라우저 form submit | observe 결과와 제출 요약을 보여준 뒤 승인 |

Companion의 approval grant는 task-scoped로 유지하고, `user_confirm`과 `user.input`은 재사용 grant 밖에 둔다.

### 4. 스케줄러와 알림 엔진 추가

매일 아침 개인별 일정 알림은 Blueclaw turn만으로는 부족하다. device에 persistent scheduler가 필요하다.

필요한 구성:

- `reminder_preference` 테이블
- `scheduled_job` 테이블
- calendar sync cursor
- 이동/준비/여유 시간 계산기
- Mattermost, Slack, Signal, 이메일 중 알림 채널 선택

알림 계산은 매일 한 번 끝나는 작업이 아니다. 일정 변경, 장소 변경, 참석자 변경이 있으면 다음 알림도 갱신해야 한다.

### 5. Artifact registry 추가

파일 생성, 공유, 분석, 문서 ingest, 슬라이드 export, 계약서 생성은 모두 산출물을 남긴다.

필요한 테이블:

| 테이블 | 용도 |
|---|---|
| `artifact` | 생성/업로드/분석된 파일의 기본 메타데이터 |
| `artifact_version` | 수정 이력과 원본 경로 |
| `artifact_share` | 공유 대상, 권한, 만료일 |
| `artifact_extraction` | markitdown, OCR, VLM, PDF text 추출 결과 |
| `artifact_relation` | task, meeting, staff, email, calendar event와의 연결 |

Companion `file_pick`은 로컬 경로를 노출하지 않고 device temp path만 반환하는 현재 정책을 유지한다. 이후 영구 보관이 필요하면 Drive, DB blob store, 또는 `/root/.internkim/artifacts` 중 하나로 명시적으로 승격한다.

### 6. 업무 DB를 product schema로 확장

이미 `docs/schema/task.md`, `docs/schema/staff.md`, `task_assignee`, `task_event` 문서가 있다. 여기에 출퇴근, 회의, 알림, 파일, 이메일 테이블을 추가한다.

추가 schema:

| 테이블 | 핵심 필드 |
|---|---|
| `attendance_event` | `staff_id`, `kind`, `occurred_at`, `source`, `note` |
| `meeting` | `title`, `started_at`, `ended_at`, `participants`, `source_ref` |
| `meeting_note` | `meeting_id`, `summary`, `decisions`, `action_items` |
| `reminder_preference` | `staff_id`, `preferred_time`, `prep_minutes`, `buffer_minutes`, `default_origin` |
| `scheduled_job` | `kind`, `scope`, `run_at`, `status`, `payload` |
| `email_thread` | `staff_id`, `provider_thread_id`, `classification`, `last_synced_at` |
| `contract_project` | `template_artifact_id`, `party_context`, `question_state`, `status` |

### 7. 인터뷰형 작업 runner 추가

계약서 작성처럼 컨텍스트에 없는 내용을 하나씩 물어봐야 하는 작업은 단일 prompt가 아니라 stateful runner가 필요하다.

동작 방식:

1. 주어진 컨텍스트와 템플릿에서 채울 수 있는 필드를 먼저 채운다.
2. 남은 필드를 중요도순 질문 큐로 만든다.
3. 한 번에 하나만 물어본다.
4. 답변을 검증하고 contract project state에 저장한다.
5. 모든 필수 필드가 채워지면 문서를 생성하고 승인용 preview를 보여준다.

이 runner는 계약서뿐 아니라 사업계획서, 긴 이메일, DB 설계, 복잡한 슬라이드에도 재사용할 수 있다.

### 8. 문서 ingest pipeline 추가

파일 인식/분석과 문서 인식은 다음 단계로 나눈다.

1. 파일 선택 또는 업로드
2. MIME type, size, checksum 확인
3. markitdown으로 텍스트 추출
4. 이미지/PDF page는 OCR 또는 VLM caption 추출
5. chunking과 요약
6. Graphiti memory 저장
7. artifact registry에 extraction 결과 저장

VLM은 민감 문서 여부에 따라 local-only, Companion local model, remote model 중 정책으로 선택한다.

## Existing-first skill 원칙

새 기능을 추가할 때 기존 실행 경로를 대체하는 skill을 먼저 만들지 않는다. 이미 검증된 skill과 capability가 있으면 그것을 canonical path로 유지하고, 새 설계는 라우팅, 질문, 승인, 상태 추적을 얇게 얹는다.

| 기존 축 | 유지할 역할 | 중복 구현 금지 |
|---|---|---|
| portable artifact generator | ICS/DOCX/XLSX/CSV/HTML/PPTX/PDF 기본 산출물 생성 | Google 인증을 기본 전제로 삼지 않는다 |
| `calendar` skill | Google Calendar optional bridge | portable 일정 기능을 Google-only로 만들지 않는다 |
| `create-gws-file` skill | Google Docs, Sheets, Gmail optional bridge | portable 문서/시트/이메일 생성을 Google-only로 만들지 않는다 |
| `simple-slides` skill | Marp 기반 HTML/PPTX/PDF/Google Slides 생성, Korean-first font manifest | HTML/PPTX/PDF 기본 출력과 `DESIGN.md` source of truth를 유지한다 |
| native reply attachments | Mattermost/Slack/Signal 파일 전송 | attachment upload를 별도 tool이나 skill로 중복 구현하지 않는다 |
| `pdf` skill | PDF 생성과 PDF 읽기/편집 | 문서형 PDF 생성기를 중복 구현하지 않는다 |
| `agent-browser` skill, `browser.*` capability | 브라우저 자동화 | raw Playwright, Chrome, `agent-browser` 호출을 제품 코드에 흩뿌리지 않는다 |
| Graphiti memory | 장기 기억 저장과 검색 | 별도 기억 저장소를 만들지 않는다 |
| Blueclaw terminal | dev/admin용 터미널 | 제품 기본 업무 경로로 확대하지 않는다 |

새 skill은 실행 구현이 아니라 orchestration layer여야 한다. 즉, 언제 기존 skill을 호출할지, 무엇을 더 물어볼지, 어떤 승인 규칙을 적용할지, 결과를 어떤 task/artifact/memory scope에 연결할지만 정의한다.

## 기능별 설계 변경과 orchestration 역할

| 기능 | 필요한 설계 변경 | canonical 실행 경로 |
|---|---|---|
| 일정 추가 | 참석자/외부 초대 승인, 알림 preference 연결 | ICS/CalDAV first, optional `calendar` |
| 개인별 아침 일정 알림 | scheduler, preference table, calendar sync, 이동/준비 시간 계산 | local schedule cache + CalDAV/ICS, optional `calendar` |
| 간단한 Google Slides | `DESIGN.md` 작성 후 Korean-first font로 HTML/PPTX/PDF 생성, 필요하면 Google import | `simple-slides` |
| 멋진 HTML 슬라이드 | `DESIGN.md` 기반 HTML artifact generator, Korean-first font, visual QA, share/export | `simple-slides` 확장 |
| 계약서 템플릿 작성 | artifact registry, interview runner, template field mapper | DOCX/PDF first, optional `create-gws-file` |
| Google Sheet 생성 | XLSX/CSV 생성 후 필요하면 Google import | XLSX/CSV first, optional `create-gws-file` |
| Google Docs 생성 | DOCX/HTML/PDF 생성 후 필요하면 Google import | DOCX/HTML first, optional `create-gws-file` |
| 회의록 정리 | meeting schema, transcript ingest, action item to task 연결 | Markdown/DOCX first, optional `create-gws-file` |
| DB 생성 | DB schema proposal, migration runner, approval gate | orchestration + Blueclaw/DB tool |
| 출퇴근 기록 | `attendance_event`, quick clock-in/out command | task/attendance capability |
| 업무 투두리스트 | task transition API, assignee request, status audit | Blueclaw task DB |
| 파일 생성 및 공유 | artifact registry, Drive/platform share approval | native reply attachments |
| 파일 인식/분석 | ingest pipeline, markitdown/OCR/VLM routing | artifact ingest pipeline |
| 직원별/직급별 기억 분리 | memory scope router, staff/role identity map | Graphiti memory |
| 이메일 정리 | provider-neutral search/classify, thread summary, label/archive plan | email provider bridge, optional Gmail |
| 이메일 작성 | draft generator, context retrieval, tone presets | `.eml`/draft text first, optional Gmail |
| 이메일 보내기 | draft preview, mandatory manual approval, audit log | provider send after approval |
| 파일/디렉토리 정리 | local filesystem plan, dry-run, approve then move | Companion local capability |
| 터미널 인터랙티브 모드 | admin/dev profile, transcript/audit | existing Blueclaw terminal only |
| 사업계획서 | interview runner, financial/table helpers, Docs/Slides export | DOCX/XLSX/PPTX/HTML first, optional Google |
| 메인 컴퓨터 브라우저 제어 | Companion browser adapter 강화, wait-for-user state | `agent-browser` + `browser.*` |
| 문서 인식 | ingest pipeline, extraction quality scoring | artifact ingest pipeline |
| 이미지 생성 | image provider capability, prompt/style policy, artifact save | image provider + artifact registry |

## 우선순위

### 1단계: 실행 계약과 안전장치

- capability registry 확장
- approval policy 통합
- artifact registry 추가
- staff identity map 추가
- portable artifact capability를 먼저 만들고 Google import/export를 optional capability로 감싸기

이 단계가 끝나면 Google 인증 없이 파일 생성, 문서/시트/슬라이드, 이메일 초안, 캘린더 추가가 먼저 가능해지고, 필요할 때 Google로 가져갈 수 있다.

### 2단계: 업무 운영 DB

- task, staff schema를 실제 migration으로 연결
- task transition API 추가
- attendance, reminder, meeting schema 추가
- Graphiti memory scope와 staff/role scope 연결

이 단계가 끝나면 출퇴근, 업무 투두, 회의록, 직원별 기억 분리가 가능해진다.

### 3단계: 스케줄러와 알림

- `scheduled_job` 실행기
- CalDAV/ICS sync
- optional Google Calendar sync
- 개인별 reminder preference
- Mattermost/Slack/Signal 알림 provider

이 단계가 끝나면 아침 일정 알림과 반복 업무 알림이 가능해진다.

### 4단계: 문서/파일 고도화

- artifact ingest pipeline
- markitdown, OCR, VLM provider routing
- 계약서 interview runner
- HTML slide visual QA
- Drive 같은 외부 publish/share 권한 audit

이 단계가 끝나면 계약서, 파일 분석, 문서 인식, HTML 슬라이드 품질이 올라간다.

### 5단계: 로컬 컴퓨터와 생성형 작업 확장

- Companion browser wait/input UX 강화
- interactive terminal도 requester actor/POSIX boundary 안에서 실행하고, 외부 연동은 typed capability로 우선 라우팅
- image generation provider 추가
- local-only model routing 강화

이 단계가 끝나면 사용자 컴퓨터 브라우저 제어, 이미지 생성, 로컬 중심 민감 작업이 가능해진다.

## Orchestration 배포 구조

Blueclaw skill은 host `/root/.blueclaw/workspace/skills`에 배치되고 guest에서는 `/workspace/skills`로 실행됩니다. 모델과 skill 문서는 guest virtual/runtime path인 `/workspace/skills`를 canonical 실행 경로로 사용합니다. 새로 추가하는 skill이 있다면 기능별 구현 skill이 아니라 orchestration wrapper여야 합니다. wrapper의 역할은 다음으로 제한합니다.

- 언제 이 skill을 써야 하는지 판단 기준 제공
- 필요한 입력 정보와 누락 정보 질문 순서 정의
- 사용할 기존 skill 또는 typed capability 이름과 payload schema 설명
- 승인 필요 조건 설명
- 결과 URL, artifact, task 상태를 사용자에게 보고하는 형식 정의

권장 orchestration 디렉토리:

```text
assets/blueclaw-workspace/skills/
├── workspace-orchestrator/
├── artifact-orchestrator/
├── document-orchestrator/
├── slide-orchestrator/
├── task-orchestrator/
├── memory-orchestrator/
└── local-orchestrator/
```

기존 `calendar`, `create-gws-file`, `simple-slides`, `pdf`, `agent-browser` skill은 유지한다. 다만 Google 계열 skill은 기본 실행 경로가 아니라 optional import/export/publish 경로로 낮춘다. wrapper는 portable artifact path를 먼저 선택하고, 사용자가 Google 공동 편집이나 공유 URL을 원할 때만 Google path를 선택한다. 플랫폼 파일 전달은 Blueclaw `FileAttachment`와 InternKim `reply.send` attachment 경로가 맡는다.

## 구현 시 주의할 경계

- Blueclaw는 Google token, browser cookie, local file path, local model path를 보지 않는다.
- Companion은 사용자 로컬 자원만 처리하고, device에는 temp path 또는 승인 결과만 넘긴다.
- 이메일 발송, 외부 공유, Google import/publish는 자동 승인하지 않는다.
- local-only mode에서는 OpenRouter나 다른 remote provider로 fallback하지 않는다.
- 브라우저 작업은 raw Playwright, Chrome, `agent-browser` 호출을 제품 코드에 흩뿌리지 않고 typed browser adapter를 통한다.
- 터미널은 기본 제품 기능이 아니라 dev/admin profile 전용으로 둔다.
- 테스트는 Mattermost, Slack, Signal, Google에 남긴 메시지와 파일을 가능한 한 정리한다.

## 다음 문서 작업

다음 단계에서는 이 문서를 기준으로 아래 세부 문서를 추가하면 된다.

- `docs/schema/artifact.md`
- `docs/schema/reminder-preference.md`
- `docs/schema/scheduled-job.md`
- `docs/schema/attendance-event.md`
- `docs/schema/meeting.md`
- `docs/schema/email-thread.md`
- `docs/schema/contract-project.md`
- `docs/capabilities/portable-artifacts.md`
- `docs/capabilities/calendar-sync.md`
- `docs/capabilities/google-workspace-optional.md`
- `docs/capabilities/approval-policy.md`
- `docs/capabilities/artifact-ingestion.md`
