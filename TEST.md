# Testing with Local Fleet

Blueclaw e2e 게이트 시나리오 인벤토리와 커버리지 매트릭스는 [E2E.md](E2E.md)에서 관리합니다.

`internkim`의 macOS 소프트웨어 테스트 환경은 disposable Local Fleet입니다. macOS host 위에 apple/container ARM Linux VM을 올리고, 테스트 동안만 살아 있는 `blueclaw`, `mattermost`, `cloudflared`, Google 연동 경계를 검증한 뒤 기본적으로 정리합니다.

## 요구사항

- Apple Silicon macOS
- `container` CLI 설치: `make deps-sim`
- `ssh`, `sshpass`
- 실서비스 자격증명
  - `INTERNKIM_REGISTER_SECRET`
  - Cloudflare 관련 환경변수
  - Google 설정에 필요한 계정/자격증명

기본 설정은 [lab/config.example.json](lab/config.example.json)에 있습니다.

## 기본 흐름

```bash
# 최초 1회 컨테이너 이미지 준비
./internkim lab image-build

# 전체 predeploy gate: 일회용 Linux+Mattermost+Kim 서버
./internkim dev fleet run

# 실제 DM E2E
./internkim dev fleet run --scenario mattermost-direct-message-send

# 실제 Mattermost 첨부 전달 + 화면 증거 보존
./internkim dev fleet run --scenario mattermost-docx-attachment --keep

# Mattermost 제외 Linux virtual-session
./internkim dev fleet run --without-mattermost --scenario dm_send_confirm_acceptance

# 모델 산출물 직접 확인
./internkim test "저번 달 업무에 대한 보고서 워드 파일로 만들어줘"
./internkim test "저번 달 업무에 대한 보고서 워드 파일로 만들어줘" -o /tmp/internkim-report.docx
./internkim test "웹사이트 만들어줘"
./internkim test "슬라이드 만들어줘" --reuse
```

`./internkim test "<prompt>"`는 disposable Local Fleet에서 실제 Mattermost DM을 보내고 task 완료 후 마지막 봇 메시지를 출력합니다. 산출물 품질 확인용 경로라 웹 UI 빌드는 건너뛰고 Mattermost, Blueclaw, runtime 의존성만 준비합니다. 현재 checkout의 Blueclaw 변경은 Local Fleet setup에서 `INTERNKIM_BLUECLAW_USE_LOCAL=1`로 자동 반영합니다. 첨부 파일이 있으면 `/tmp/internkim-test-<timestamp>/`에 내려받아 macOS `open`으로 열며, `-o <file>`을 주면 단일 첨부 파일을 정확히 그 파일 경로에 씁니다. URL은 메시지 텍스트로 그대로 확인합니다. 첨부 파일 후처리만 수행하므로 웹사이트 URL처럼 파일이 없는 결과도 성공으로 취급합니다.

## Mattermost 화면 증거

사용자에게 보이는 전달 결과가 문제인 회귀는 터미널 성공만으로 끝내지 않습니다. 실제 Mattermost DM 화면에서 사용자 요청, 김인턴 최종 답변, native attachment 카드나 공개 URL이 함께 보이는 스크린샷을 남깁니다.

```bash
./internkim dev fleet run --scenario mattermost-docx-attachment --keep
```

`--keep`은 VM, Mattermost posts/users, 다운로드 파일, run state를 남기므로 화면 확인이 필요한 경우에만 사용합니다. 명령 출력의 Mattermost URL로 접속해 보존된 테스트 사용자 또는 관리자 계정으로 로그인하고, 필요한 DM/thread를 연 뒤 브라우저 도구로 스크린샷을 저장합니다. 파일 산출물 회귀라면 `--download-files-to`가 저장한 첨부 파일도 같이 확인합니다.

증거 파일은 `.local/local-fleet/runs/<run-id>/` 아래에 둡니다. 예:

```text
.local/local-fleet/runs/<run-id>/mattermost-docx-attachment-proof.png
.local/local-fleet/runs/<run-id>/downloads/mattermost-docx-attachment/<filename>.docx
```

스크린샷에는 최소한 다음이 보여야 합니다.

- 사용자가 Mattermost에서 보낸 원 요청
- 김인턴의 최종 사용자-facing 답변
- 첨부 파일 카드, 다운로드 가능한 파일명, 또는 공개 URL
- thread 안에서 완료된 작업이라면 thread reply 영역

화면 증거는 로컬 디버깅 artifact입니다. git에 넣지 말고, 최종 보고에는 절대경로를 남깁니다. 증거 검토가 끝나면 출력된 cleanup 명령이나 `./internkim dev fleet reset`/`down`으로 보존 VM과 Mattermost 테스트 흔적을 정리합니다.

## 테스트 흔적 정리

실제 Mattermost, Slack, Signal 같은 플랫폼 표면을 사용하는 테스트는 테스트 메시지와 봇 답변을 남기면 안 됩니다.

- 테스트 메시지는 `verify ... <timestamp>`처럼 식별 가능한 문자열을 사용합니다.
- 테스트가 만든 사용자 메시지와 봇 답변은 검증 직후 삭제합니다.
- 삭제가 실패하면 어떤 post/message가 남았는지 출력합니다.
- Blueclaw memory를 검증한 뒤 상태를 비워야 하면 아래 명령을 사용합니다.

```bash
./internkim reset blueclaw-history --plan
./internkim reset blueclaw-history --confirm <deviceID>
./internkim reset blueclaw-history --keep-mattermost-posts --confirm <deviceID>
```

기본 reset은 Blueclaw task, raw event, conversation, legacy memory, Graphiti mirror, Kuzu memory files와 Mattermost 화면에 보이는 post/reaction/thread 기록을 함께 지웁니다. 초대 사용자, policy, platform account link, secrets, Mattermost 사용자, 팀, 채널은 유지합니다. 디버깅 때문에 Mattermost 화면 기록만 남겨야 하는 테스트는 `--keep-mattermost-posts`를 함께 사용합니다.

Slack과 Signal은 외부 플랫폼이므로 reset 명령이 원격 서비스의 전체 메시지 기록을 보장해서 지우지는 않습니다. 테스트가 만든 Slack/Signal 메시지는 connector별 삭제 권한이 있는 범위에서 즉시 삭제하고, 삭제 실패 시 남은 artifact를 출력해야 합니다.

`./internkim dev fleet run`은 다음을 순서대로 확인합니다.

- VM 기동 및 SSH 가능 여부
- Ubuntu provisioning과 `internkim setup` 실행
- `blueclaw.service`, `mattermost`, `cloudflared` 활성 상태
- `/root/.blueclaw/config/runtime.json`, `/root/.blueclaw/config/policy.json`
- API health, Mattermost ingress, DM 수신자 해석, browser smoke

## 저수준 Lab 디버깅 명령

일반 테스트에서는 `dev fleet run`을 사용합니다. 아래 명령은 VM을 직접 붙잡고
문제를 확인해야 할 때만 사용합니다.

```bash
./internkim lab image-build
./internkim lab vm-up
./internkim lab vm-down
./internkim lab vm-ssh
./internkim lab status
./internkim lab setup
./internkim lab scenario-mattermost
./internkim lab scenario-google
./internkim lab scenario-cloudflare
./internkim lab scenario-e2e
```

테스트 동안만 살아 있는 로컬 Linux 환경이 필요하면 Local Fleet을 사용합니다.
성공/실패 뒤 기본적으로 VM, 터널, 상태 디렉터리를 정리합니다.

```bash
./internkim dev fleet run --scenario mattermost-direct-message-send
./internkim dev fleet run --without-mattermost --scenario dm_send_confirm_acceptance
```

공유 VM을 직접 붙잡고 디버깅해야 할 때만 `--reuse`를 붙입니다. 일회용 VM을
남겨야 하면 `--keep`을 붙이고, 출력된 cleanup 명령으로 VM과 상태를 직접 지웁니다.

저수준 lab 시나리오의 기본 설정 파일을 바꾸려면:

```bash
./internkim lab scenario-e2e --config /path/to/lab.json
```

타임아웃을 늘리려면:

```bash
./internkim lab scenario-e2e --timeout 90m
```

## VM 내부 검증

```bash
./internkim lab vm-ssh
```

VM에 들어간 뒤 자주 쓰는 검증 명령:

```bash
systemctl status blueclaw
systemctl status mattermost
systemctl status cloudflared

curl -fsS http://127.0.0.1:8080/health

cat /root/.blueclaw/config/runtime.json
cat /root/.blueclaw/config/policy.json

ls /root/.blueclaw/workspace
ls /root/.internkim/env
ls /root/.internkim/secrets
```

## Agent Capability Acceptance

InternKim에서 기능 추가 완료는 내부 API가 아니라 사용자식 요청이 실제 agent 경로를 통과하는 것입니다.

새 tool 기능은 대응 skill을 함께 가져야 합니다. 빠른 회귀는 Blueclaw virtual session에서 확인합니다.

```bash
cd .dependency/blueclaw
go test ./internal/agent ./internal/e2e ./internal/connectors ./internal/agentruntime
```

계약은 다음과 같습니다.

- descriptor, policy, profile을 지나 전체 tool catalog에 tool이 등록됩니다.
- 관련 skill이 자연어 요청에서 선택됩니다.
- 실제 turn catalog는 core tool과 selected skill의 `allowed-tools`만 포함합니다.
- task event에 selected skill, 노출 tool, tool request, tool result가 남습니다.
- 최소 1개 한국어 acceptance prompt가 통과합니다.

현재 빠른 acceptance prompt:

```bash
cd .dependency/blueclaw
go test ./internal/e2e -run 'TestScheduleCreateAcceptance|TestSitePrototypeAcceptance' -v
```

실제 Mattermost ingress smoke는 비용과 platform 상태에 의존하므로 opt-in입니다. 검증 뒤 테스트 메시지와 봇 답변은 삭제해야 합니다.

```bash
./internkim test '1분마다 "1분 지났습니다"라고 보내줘' --expect-tool schedule_create
./internkim test "테스트용 'Local Fleet Studio' 단일 페이지 소개 웹사이트를 만들어서 배포해줘. 첫 화면 제목은 'Local Fleet Studio', 보조 문구는 '로컬 플릿 웹사이트 생성 배포 테스트', 섹션은 서비스 소개, 장점 3개, 문의 CTA만 넣어줘. 추가 질문하지 말고 합리적인 기본값으로 진행해줘." --expect-public-url --expect-tool terminal_run --expect-tool site_serve
```

Mattermost ask 선택지 attachment와 버튼 ACK 회귀는 Local Fleet VM 또는 저수준 lab smoke로 확인합니다. public 봇 답변에 선택지 attachment가 붙지 않고, requester-only ephemeral post가 생성되며, 버튼 ACK는 빈 ephemeral text 없이 delete update를 반환해야 합니다.

```bash
lab/scripts/run-smoke-mattermost-ask-ephemeral-container.sh internkim-lab
```

배포 전 기능별 확인:

- 스케줄링: `./internkim dev fleet run --without-mattermost --scenario schedule_lifecycle_acceptance`
- 웹사이트 생성/수정: `./internkim dev fleet run --without-mattermost --scenario site_edit_redeploy_acceptance`
- 웹사이트 삭제 CRUD: `./internkim dev fleet run --without-mattermost --scenario site_lifecycle_acceptance`
- Mattermost DM: `./internkim dev fleet run --scenario mattermost-direct-message-send`

## 단계별 검증 모델

### Phase A

macOS + apple/container 기반 disposable Local Fleet E2E.

- 모든 로컬 개발자는 먼저 이 경로를 통과시킵니다.
- Mattermost가 필요한 경로는 실제 로컬 Mattermost 서버로 검증합니다.
- Mattermost가 필요 없는 agent 경로는 `--without-mattermost`로 Linux VM 안에서 검증합니다.

### Phase B

Jetson 하드웨어 검증.

- Local Fleet에서 통과한 같은 acceptance 체크리스트를 실제 보드에 적용합니다.
- 하드웨어 검증은 후속 단계이며, 현재 문서는 소프트웨어 E2E까지만 다룹니다.

## 정리

```bash
./internkim lab vm-down
```

기존 `internkim sim ...` 경로는 deprecated alias이며, 새 테스트 문서와 운영 가이드는 모두 `internkim dev fleet ...` 기준입니다. `internkim lab ...`은 저수준 VM 디버깅 명령으로만 사용합니다.
