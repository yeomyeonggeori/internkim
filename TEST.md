# Testing with the container Lab

Blueclaw e2e 게이트 시나리오 인벤토리와 커버리지 매트릭스는 [E2E.md](E2E.md)에서 관리합니다.

`internkim`의 macOS 소프트웨어 테스트 환경은 이제 apple/container 기반입니다. 목표는 Blueclaw lab과 비슷한 토폴로지로, macOS host 위에 apple/container ARM Linux VM을 올리고 그 VM 안에서 `blueclaw`, `mattermost`, `cloudflared`, Google 연동을 실제처럼 검증하는 것입니다.

## 요구사항

- Apple Silicon macOS
- `container` CLI 설치: `make deps-sim`
- `ssh`, `sshpass`
- 실서비스 자격증명
  - `INTERNKIM_REGISTER_SECRET`
  - Cloudflare 관련 환경변수
  - Google 설정에 필요한 계정/자격증명

기본 설정은 [lab/config.example.json](/Users/lee/Developer/work/internkim/lab/config.example.json)에 있습니다.

## 기본 흐름

```bash
# 빌드
make build

# 컨테이너 이미지 준비
./internkim lab image-build

# VM 부팅
./internkim lab vm-up

# Ubuntu provision + internkim setup
./internkim lab setup

# 전체 acceptance
./internkim lab scenario-e2e

# 또는 전체 시뮬레이션을 한 번에
./internkim setup --sim
```

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

`scenario-e2e`는 다음을 순서대로 확인합니다.

- VM 기동 및 SSH 가능 여부
- Ubuntu provisioning 스크립트 실행
- `internkim setup --ssh --host <vm-ip>` 실행
- `blueclaw.service`, `mattermost`, `cloudflared` 활성 상태
- `/root/.blueclaw/config/runtime.json`, `/root/.blueclaw/config/policy.json`
- Google 연동 시나리오
- Mattermost bot/channel 시나리오
- Cloudflare URL 도달 가능 여부

## 개별 명령

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

테스트 동안만 살아 있는 로컬 Linux 환경이 필요하면 local fleet의 일회용 실행을
사용합니다. 성공/실패 뒤 기본적으로 VM, 터널, 상태 디렉터리를 정리합니다.

```bash
./internkim dev fleet run --ephemeral --scenario mattermost-direct-message-send
./internkim dev fleet run --ephemeral --without-mattermost --scenario dm_send_confirm_acceptance
```

디버깅용으로 남겨야 하면 `--keep`을 붙입니다. 이 경우 출력된 cleanup 명령으로
VM과 상태를 직접 지웁니다.

기본 설정 파일을 바꾸려면:

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
./internkim verify mattermost --prompt '1분마다 "1분 지났습니다"라고 보내줘' --expect-tool schedule.create --expect-event schedule.created
./internkim verify mattermost --prompt '웹사이트 하나 만들어서 배포해봐' --expect-tool site.app.create --expect-tool site.app.publish
```

Mattermost ask 선택지 attachment와 버튼 ACK 회귀는 container lab에서 별도 smoke로 확인합니다. public 봇 답변에 선택지 attachment가 붙지 않고, requester-only ephemeral post가 생성되며, 버튼 ACK는 빈 ephemeral text 없이 delete update를 반환해야 합니다.

```bash
lab/scripts/run-smoke-mattermost-ask-ephemeral-container.sh internkim-lab
```

배포 전 기능별 확인:

- 스케줄링: Mattermost prompt smoke, `schedule.create`, `schedule.created`, due-run delivery
- 웹사이트: Mattermost prompt smoke, `site.app.create`, `site.app.publish`, public URL 200

## 단계별 검증 모델

### Phase A

macOS + apple/container 기반 소프트웨어 E2E.

- 모든 로컬 개발자는 먼저 이 경로를 통과시킵니다.
- Google, Mattermost, Cloudflare는 실제 자격증명 기준으로 검증합니다.

### Phase B

Raspberry Pi 하드웨어 검증.

- container lab에서 통과한 같은 acceptance 체크리스트를 실제 보드에 적용합니다.
- 하드웨어 검증은 후속 단계이며, 현재 문서는 소프트웨어 E2E까지만 다룹니다.

## 정리

```bash
./internkim lab vm-down
```

기존 `internkim sim ...` 경로는 deprecated alias이며, 새 테스트 문서와 운영 가이드는 모두 `internkim lab ...` 기준입니다.
