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

기본 설정은 [config/lab.example.json](/Users/lee/Developer/work/internkim/config/lab.example.json)에 있습니다.

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
