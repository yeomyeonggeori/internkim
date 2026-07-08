# Blueclaw CRUD And Render Gate Handoff

작성 시점: 2026-06-29 KST

이 문서는 새 Codex 세션이 현재 작업을 이어받기 위한 실행 기록이다. 사용자와의 대화 맥락보다 이 문서를 우선 읽고, 실제 파일 상태를 다시 확인한 뒤 진행한다.

> 정정 (2026-07-08): 아래 본문의 비실환경 테스트 모델 `google/gemma-4-31b-it:free`는
> 이 문서 작성 시점의 값이다. 현행 기본 핀은 `xiaomi/mimo-v2.5`다
> (`internal/cli/dev_command.go`의 `INTERNKIM_TEST_MODEL`/`--llm-model` 기본값,
> `tools/reprovision-local-fleet`, `tools/e2e-crud`). 본문 로그는 기록 보존을 위해
> 수정하지 않는다.

## 목표

- 기존 변경을 보존한 상태에서 Blueclaw tool/kernel 정리 작업을 계속한다.
- 테스트는 기존에 동작하던 VM + Mattermost 기반 `./internkim test` 경로로만 한다.
- 비실환경 테스트 모델은 `google/gemma-4-31b-it:free`를 사용한다.
- `--real`은 실환경 모델을 쓰는지 여부만 달라야 한다. 모델 외 동작 경로가 달라지면 버그로 본다.
- 태스크 CRUD, 일정 CRUD, 문서 생성/수정/삭제, 웹사이트 생성/배포/수정/삭제를 실제 Mattermost 대화와 결과물 파일 또는 렌더 스크린샷으로 검증한다.
- 결과물이 없으면 `metadata.json` 또는 `scores.json`을 만들지 않는다.

## 핵심 설계 의도

이번 작업의 핵심은 단순히 툴 이름 몇 개를 바꾸는 것이 아니라, Blueclaw의 LLM tool surface를 줄이고 안정화하기 위한 대대적인 kernel 리팩토링이다.

- 툴은 work kind나 skill 선택에 따라 동적으로 바꾸지 않고 항상 같은 compact kernel palette를 노출한다.
- 스킬은 특정 툴 이름이나 특정 서비스 구현에 강하게 의존하지 않아야 한다.
- 스킬은 업무, 일정, 문서, 웹사이트 같은 도메인별 힌트를 주되, 실행은 generic kernel tools와 capability bridge를 통해 이뤄져야 한다.
- Blueclaw core는 InternKim, Mattermost, public `/api/v1` 같은 특정 제품/채널 구현에 직접 묶이면 안 된다.
- InternKim은 Blueclaw 위에서 동작하는 한 서비스일 뿐이며, 다른 서비스가 Blueclaw를 사용할 때도 같은 kernel contract를 유지할 수 있어야 한다.
- 업무, 일정, 문서, 웹사이트 생성/수정/삭제/배포가 모두 안정적으로 성공해야 이 리팩토링이 완료된 것으로 본다.
- 위 게이트가 통과하기 전에는 main 반영, push, 배포를 하지 않는다.

현재 사용자가 허용한 방향:

- 6개 툴 고정은 절대 제약이 아니다. 12개 정도라도 안정성과 명확성이 좋으면 괜찮다.
- `file.read`, `file.write`, `file.edit`, `file.patch`, `file.preview`, `image.read` 같은 coding/artifact 작업용 generic file tools는 복구 또는 유지 가능하다.
- `file.attach`는 모델에게 보이면 안 된다.
- 최종 전달은 `file.deliver`처럼 provider-neutral delivery path를 사용해야 한다.
- `ask.input`은 선택지 배열이 비어 있으면 주관식, 비어 있지 않으면 선택 또는 직접 답변 입력으로 동작한다. 별도 `ask.choice`는 되살리지 않는다.

## 현재 Git 상태

루트 저장소:

- 경로: `/Users/lee/Developer/work/internkim-wt2`
- 브랜치: `feature/blueclaw-minimal-tool-kernel`
- 최근 커밋:
  - `acb84b8f fix: stabilize blueclaw health gate`
  - `ff3d3e17 fix: Update Blueclaw kernel contract`
- 현재 알려진 미추적 항목: `.agents`

Blueclaw 서브모듈:

- 경로: `.dependency/blueclaw`
- 브랜치: `feature/blueclaw-minimal-tool-kernel`
- 최근 커밋:
  - `b6f7e8a fix: clarify kernel capability recovery`
  - `ba401b5 fix: Align kernel delivery tools`
- 마지막 확인 시점에는 clean 상태였다.

새 세션 시작 직후 반드시 확인한다.

```bash
git status --short --branch
git log --oneline -6
git -C .dependency/blueclaw status --short --branch
git -C .dependency/blueclaw log --oneline -6
```

## 이미 적용된 변경

- `file.deliver`를 최종 전달 툴로 유지하고, `file.attach`는 모델에게 보이지 않게 정리했다.
- `artifact.deliver`는 새 계약에서 사용하지 않는다.
- `ask.choice`는 제거하고 `ask.input`의 `choices` 배열로 통합하는 방향으로 정리했다.
- `terminal.session`은 별도 툴로 노출하지 않고 `terminal.run` 세션 모드로 흡수하는 방향이다.
- `site.app.*` 같은 사이트 전용 툴 이름은 모델 프롬프트/스킬 문서에서 제거하는 중이다.
- Blueclaw 내부는 InternKim 브랜드와 public `/api/v1` 의존을 피해야 한다. 전달/렌더/외부 연동은 capabilityd 경유가 맞다.
- skill 문서는 작게 유지해야 한다. 큰 스크립트나 긴 절차를 모델에게 통째로 전달하는 것은 버그로 본다.
- 기존 방식으로 돌아가는 결정이 있었다. 단, 경로와 의미는 조심해야 하며 Blueclaw를 쓰는 다른 서비스에서 InternKim 전용 툴을 건드릴 수 있으면 안 된다.

## 이미 통과한 로컬 검증

아래 명령은 이 작업 중 통과했다.

```bash
go test ./internal/provisioning/steps ./internal/runtime/blueclaw ./internal/localfleet
go test ./internal/agentruntime
go test ./internal/runtime/blueclaw ./internal/blueclawworkspace ./internal/provisioning/steps ./internal/localfleet ./internal/cli
git -C .dependency/blueclaw status --short --branch
cd .dependency/blueclaw && go test ./internal/agentruntime ./internal/app ./internal/security
make build
```

서브모듈의 targeted agent tests도 통과했다.

```bash
cd .dependency/blueclaw
go test ./internal/agent -run 'TestBuildAgentActionRequestPreservesNativeToolCallingWireShape|TestToolExposureUsesFixedKernelOnly|TestFlowTaskRequestSkipsArtifactSkillInstructions|TestDocxArtifactRequestIsNotDominatedBySitePrototype|TestSiteArtifactContractSelectsSitePrototypeOverUnrelatedArtifactSkill|TestAdvanceAgentTaskReturnsAttachExistingArtifactEffect|TestRecoverableWorkflowNextTools|TestCompletionGateRejectsMissingRequiredFileAttachment|TestCompletionGateAllowsFileDeliverEvidence'
```

## 첫 실제 테스트 결과

아래 artifact 디렉터리를 만들고 태스크 CRUD 테스트를 시작했다.

```text
.artifacts/blueclaw-crud-render-gate-20260629T122226KST/
```

현재 세션에서 시작한 명령:

```bash
./internkim test --reuse --keep --no-open --auto-confirm --seed 41 --temperature 0 --timeout 1200 --result-json .artifacts/blueclaw-crud-render-gate-20260629T122226KST/task-crud.json "테스트 ID IK-CRUD-TASK-20260629-122226. 업무 태스크 CRUD를 실제로 검증해줘. 새 태스크를 하나 만들고 제목을 'IK-CRUD-TASK-20260629-122226 초안'으로 해. 그 태스크 제목을 'IK-CRUD-TASK-20260629-122226 수정됨'으로 수정해. 마지막으로 그 태스크를 삭제해. 답변에는 생성, 수정, 삭제가 각각 성공했는지와 삭제 후 같은 테스트 ID 태스크가 남아 있지 않은지 적어줘."
```

관찰된 상태:

- Local Fleet VM IP는 `192.168.64.97`였다.
- setup은 오래 걸렸지만 최종 health는 통과했다.
- 로그에 `INTERNKIM_TEST_MODEL='google/gemma-4-31b-it:free'`가 표시됐다.
- 프롬프트는 Mattermost에 올라갔고, 자동 승인 reply도 처리됐다.
- 테스트는 실패로 종료됐고 `task-crud.json`은 생성되지 않았다.
- artifact 디렉터리 안에는 아직 결과 파일이 없다.

실패 핵심:

- 승인 후 Blueclaw가 `terminal.run`으로 `/workspace/tools/capability invoke flow.task.add`를 실행했다.
- 실행 결과는 `capability bridge unavailable: [Errno -2] Name or service not known`였다.
- 즉 모델이 CRUD를 회피한 것이 아니라, 런타임 안에서 capability bridge 이름 해석 또는 연결 설정이 깨져 있었다.
- 그 뒤 recovery turn에서 `google/gemma-4-31b-it:free`가 OpenRouter 429를 반환했고 task가 paused 상태가 되면서 harness가 실패했다.

실패 당시 tool palette:

```text
terminal.run
ask.input
ask.confirm
file.deliver
file.edit
file.patch
file.preview
file.read
file.write
image.read
skill.search
```

사용자는 6개 고정 제약은 임의였고 12개 정도는 괜찮다고 정정했다. 따라서 이 실패의 첫 조사 대상은 tool 개수보다 `/workspace/tools/capability`가 바라보는 capability bridge endpoint/DNS이다.

새 세션에서는 먼저 결과가 생겼는지 확인한다.

```bash
find .artifacts/blueclaw-crud-render-gate-20260629T122226KST -maxdepth 2 -type f -print
test -f .artifacts/blueclaw-crud-render-gate-20260629T122226KST/task-crud.json && jq '{ok, taskStatus, botMessage, taskRunID, channelID, userPostID, botPostID, fileIDs}' .artifacts/blueclaw-crud-render-gate-20260629T122226KST/task-crud.json
```

만약 이전 테스트 프로세스가 아직 남아 있으면 중복 실행하지 않는다. 이 문서 작성 시점의 Codex tool session은 종료됐지만 OS 프로세스 확인은 다시 한다. 프로세스 확인이 sandbox에서 막히면 escalation을 사용한다.

```bash
ps -axo pid,etime,command | rg './internkim test|blueclaw-crud-render-gate'
```

결과 JSON이 없고 실행 중인 프로세스도 없으면 바로 재실행하지 말고, 먼저 capability bridge 연결 실패를 조사한다. 같은 상태에서 재실행하면 다시 실패할 가능성이 높다.

우선 조사할 항목:

```bash
rg "capability bridge unavailable|/workspace/tools/capability|flow.task.add|CAPABILITY|capabilityd" .dependency/blueclaw internal cmd web tools
rg "INTERNKIM_TEST_MODEL|--real|google/gemma-4-31b-it" .
```

확인할 질문:

- `--real`과 일반 `./internkim test`의 차이가 모델 외 capability endpoint, env, network, DNS를 바꾸고 있지 않은가.
- Local Fleet guest 안에서 `/workspace/tools/capability`가 어떤 host/port를 사용하도록 생성되는가.
- capabilityd가 host에서는 healthy인데 Firecracker guest 또는 Blueclaw terminal namespace에서 이름 해석이 안 되는 이유는 무엇인가.
- 기존 동작하던 테스트와 비교해 capability bridge hostname 또는 env injection이 바뀐 지점이 있는가.

## 실제 검증 원칙

치팅으로 보면 안 되는 것:

- Mattermost에 실제 요청과 bot 응답이 남아 있어야 한다.
- 요청/응답 화면 스크린샷을 저장해야 한다.
- 태스크/일정은 생성, 수정, 삭제가 실제 capability 또는 runtime 이벤트로 확인되어야 한다.
- 문서는 최종 전달 파일이 실제로 존재해야 하고, 가능하면 렌더 또는 파일 구조 검사를 한다.
- 웹사이트는 public URL이 실제로 열려야 하고 desktop/mobile 스크린샷을 저장해야 한다.
- 웹사이트 삭제는 스크린샷 확보 뒤 별도 삭제 요청으로 검증한다. 삭제 후 URL 또는 site 상태가 제거됐는지 확인한다.

`--expect-tool terminal.run`은 필수 조건이 아니다. 특정 회귀를 좁혀 볼 때만 쓴다. 일반 CRUD/렌더 게이트에서는 기존 `./internkim test` 경로를 유지한다.

웹사이트 생성/배포는 오래 걸릴 수 있으므로 필요한 경우에만 `--timeout 1800`을 쓴다. 짧은 CRUD 테스트에 무조건 붙이지 않는다.

## 다음 실행 순서

### 1. 태스크 CRUD 확인

이전 실행 결과가 있으면 먼저 판정한다.

```bash
jq '{ok, taskStatus, botMessage, taskRunID, channelID, userPostID, botPostID, fileIDs}' .artifacts/blueclaw-crud-render-gate-20260629T122226KST/task-crud.json
```

`taskEvents` 안에서 생성, 수정, 삭제에 해당하는 capability 또는 terminal 실행 흔적을 확인한다. 단순한 자연어 답변만으로 통과시키지 않는다.

### 2. 일정 CRUD

태스크 CRUD가 실제로 끝난 뒤 실행한다.

```bash
./internkim test --reuse --keep --no-open --auto-confirm --seed 42 --temperature 0 --timeout 1200 --result-json .artifacts/blueclaw-crud-render-gate-20260629T122226KST/calendar-crud.json "테스트 ID IK-CRUD-CALENDAR-20260629-122226. 일정 CRUD를 실제로 검증해줘. 새 일정을 하나 만들고 제목을 'IK-CRUD-CALENDAR-20260629-122226 초안'으로 해. 날짜는 내일 오전 10시부터 30분으로 잡아. 그 일정 제목을 'IK-CRUD-CALENDAR-20260629-122226 수정됨'으로 수정하고 시간을 내일 오전 11시부터 30분으로 바꿔. 마지막으로 그 일정을 삭제해. 답변에는 생성, 수정, 삭제가 각각 성공했는지와 삭제 후 같은 테스트 ID 일정이 남아 있지 않은지 적어줘."
```

### 3. 문서 생성, 수정, 삭제

최종 파일은 `.artifacts/.../files/` 아래로 받는다. `file.deliver`가 실제 전달 경로여야 한다.

```bash
./internkim test --reuse --keep --no-open --auto-confirm --seed 43 --temperature 0 --timeout 1200 --result-json .artifacts/blueclaw-crud-render-gate-20260629T122226KST/document-crud.json -o .artifacts/blueclaw-crud-render-gate-20260629T122226KST/files/document-crud.docx "테스트 ID IK-DOC-CRUD-20260629-122226. DOCX 문서를 실제로 생성, 수정, 삭제 검증해줘. 먼저 제목이 'IK-DOC-CRUD-20260629-122226 초안 문서'인 DOCX 문서를 만들고 본문에 생성 확인 문장을 넣어. 이어서 같은 문서를 수정해서 제목을 'IK-DOC-CRUD-20260629-122226 수정 문서'로 바꾸고 본문에 수정 확인 문장을 추가해. 중간 초안 파일이나 불필요한 임시 파일은 삭제해. 최종 수정본 DOCX를 전달하고, 답변에는 생성, 수정, 삭제, 최종 파일 전달이 각각 성공했는지 적어줘."
```

확인:

```bash
ls -lh .artifacts/blueclaw-crud-render-gate-20260629T122226KST/files/document-crud.docx
file .artifacts/blueclaw-crud-render-gate-20260629T122226KST/files/document-crud.docx
unzip -l .artifacts/blueclaw-crud-render-gate-20260629T122226KST/files/document-crud.docx | head
```

가능하면 기존 문서 렌더 도구로 페이지 PNG 또는 PDF를 만들고 `screenshots/`에 저장한다.

### 4. 웹사이트 생성, 배포, 수정

생성/배포/수정까지 한 뒤 public URL이 살아 있을 때 desktop/mobile 스크린샷을 먼저 저장한다.

```bash
./internkim test --reuse --keep --no-open --auto-confirm --seed 44 --temperature 0 --timeout 1800 --expect-public-url --result-json .artifacts/blueclaw-crud-render-gate-20260629T122226KST/website-create-edit.json "테스트 ID IK-SITE-CRUD-20260629-122226. 웹사이트를 실제로 생성, 배포, 수정 검증해줘. 새 단일 페이지 웹사이트를 만들고 첫 화면에 'IK-SITE-CRUD-20260629-122226 초안 사이트'라는 큰 제목을 넣어 배포해. 그 다음 같은 사이트를 수정해서 제목을 'IK-SITE-CRUD-20260629-122226 수정 사이트'로 바꾸고 눈에 띄는 보조 문구도 추가한 뒤 다시 배포해. 답변에는 생성, 최초 배포, 수정, 재배포가 각각 성공했는지와 최종 public URL을 적어줘. 아직 삭제하지 마."
```

확인:

- result JSON의 public URL을 연다.
- desktop screenshot 저장: `screenshots/website-desktop.png`
- mobile screenshot 저장: `screenshots/website-mobile.png`
- 화면에 수정 후 제목이 실제로 보여야 한다.

### 5. 웹사이트 삭제

스크린샷을 저장한 뒤에만 삭제 요청을 보낸다.

```bash
./internkim test --reuse --keep --no-open --auto-confirm --seed 45 --temperature 0 --timeout 1200 --result-json .artifacts/blueclaw-crud-render-gate-20260629T122226KST/website-delete.json "테스트 ID IK-SITE-CRUD-20260629-122226. 방금 만든 테스트 웹사이트를 실제로 삭제하거나 공개 중지해줘. 삭제 후 같은 테스트 ID 사이트가 더 이상 배포 목록이나 공개 URL에서 살아 있지 않은지 확인하고, 답변에는 삭제 성공 여부와 남아 있지 않다는 확인을 적어줘."
```

삭제 뒤에는 public URL이 계속 살아 있는지 확인한다. 캐시 때문에 즉시 404가 안 나오면 site 상태 API 또는 배포 목록에서 제거 여부를 확인한다.

## Mattermost 스크린샷

각 JSON에서 `channelID`, `userPostID`, `botPostID`를 확인한다. `--keep`을 사용했으므로 스크린샷을 찍기 전에는 테스트 메시지를 지우지 않는다.

저장할 파일:

- `screenshots/task-crud-mattermost.png`
- `screenshots/calendar-crud-mattermost.png`
- `screenshots/document-crud-mattermost.png`
- `screenshots/website-create-edit-mattermost.png`
- `screenshots/website-delete-mattermost.png`
- `screenshots/website-desktop.png`
- `screenshots/website-mobile.png`

스크린샷 방식은 기존 repo 도구를 먼저 찾는다. 없으면 로컬 브라우저 또는 Playwright로 Mattermost에 로그인해 해당 채널/포스트가 보이는 화면을 캡처한다.

## 최종 산출물

모든 게이트가 실제로 통과한 뒤에만 아래 파일을 만든다.

- `.artifacts/blueclaw-crud-render-gate-20260629T122226KST/metadata.json`
- `.artifacts/blueclaw-crud-render-gate-20260629T122226KST/scores.json`

`metadata.json`에는 실행 명령, 커밋 SHA, 서브모듈 SHA, 모델, VM 정보, result JSON 경로, screenshot 경로, output file 경로를 넣는다.

`scores.json`에는 각 게이트별 pass/fail과 근거 파일을 넣는다. 결과물이 없거나 스크린샷이 없으면 pass로 기록하지 않는다.

## Main 반영과 배포 조건

모든 실제 게이트가 통과한 뒤에만 다음 순서로 진행한다.

1. 루트와 `.dependency/blueclaw`의 작업 트리를 다시 확인한다.
2. 필요한 수정이 있으면 루트와 서브모듈을 논리적으로 커밋한다.
3. 브랜치를 원격에 push한다.
4. main에 반영한다. 프로젝트 관례상 PR이나 merge 방식이 정해져 있으면 그 방식을 따른다.
5. 배포 전 `make build`와 관련 Local Fleet/Mattermost gate 결과를 다시 확인한다.
6. 배포는 필요한 최소 component set으로만 한다.

배포 금지 조건:

- 태스크, 일정, 문서, 웹사이트 중 하나라도 실제 Mattermost + 결과물/스크린샷 검증을 통과하지 못한 경우.
- `--real`과 일반 테스트의 차이가 모델 외 실행 경로를 바꾸는 경우.
- Blueclaw core가 InternKim 전용 public API나 Mattermost 전용 전달 구현에 직접 의존하는 경우.
- 모델에게 제거된 툴 이름이 노출되는 경우.

## Cleanup

스크린샷과 산출물 확인이 끝난 뒤:

- Mattermost 테스트 메시지와 bot reply를 삭제한다.
- 가능한 경우 테스트 사용자도 삭제한다.
- 테스트 태스크, 일정, 웹사이트, 임시 문서가 남아 있지 않은지 확인한다.
- `--reuse`로 남긴 local fleet 상태는 필요 시 `./internkim dev fleet reset`으로 정리한다.

Cleanup 전에 필요한 스크린샷을 모두 확보해야 한다.

## 주의할 회귀

- 모델에게 `file.attach`가 보이면 안 된다.
- 모델에게 `artifact.deliver`가 보이면 안 된다.
- 모델에게 `site.app.*` 같은 제거된 사이트 전용 툴 이름이 보이면 안 된다.
- `ask.choice`를 새 계약의 일반 툴로 되살리면 안 된다.
- `terminal.session`을 별도 툴로 되살리면 안 된다.
- Blueclaw core가 InternKim public API나 Mattermost 전용 전달 구현에 직접 묶이면 안 된다.
- `--real` 여부가 모델 외 실행 경로를 바꾸면 안 된다.
- 기존 `./internkim test` VM + Mattermost 경로 대신 임의의 로컬-only 시뮬레이션으로 통과 처리하면 안 된다.
