# Testing with Tart Lab

`internkim`의 macOS 소프트웨어 테스트 환경은 이제 Tart 기반입니다. 목표는 Blueclaw lab과 비슷한 토폴로지로, macOS host 위에 Tart ARM Linux VM을 올리고 그 VM 안에서 `blueclaw`, `mattermost`, `cloudflared`, Google 연동을 실제처럼 검증하는 것입니다.

## 요구사항

- Apple Silicon macOS
- `tart` 설치: `make deps-sim`
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

# Tart 이미지 준비
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

## 단계별 검증 모델

### Phase A

macOS + Tart 기반 소프트웨어 E2E.

- 모든 로컬 개발자는 먼저 이 경로를 통과시킵니다.
- Google, Mattermost, Cloudflare는 실제 자격증명 기준으로 검증합니다.

### Phase B

Raspberry Pi 하드웨어 검증.

- Tart에서 통과한 같은 acceptance 체크리스트를 실제 보드에 적용합니다.
- 하드웨어 검증은 후속 단계이며, 현재 문서는 소프트웨어 E2E까지만 다룹니다.

## 정리

```bash
./internkim lab vm-down
```

기존 `internkim sim ...` 경로는 deprecated alias이며, 새 테스트 문서와 운영 가이드는 모두 `internkim lab ...` 기준입니다.
