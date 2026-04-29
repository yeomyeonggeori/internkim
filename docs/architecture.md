# Blueclaw 아키텍처

현재 `internkim`은 Jetson Orin Nano Super를 1차 하드웨어 타깃으로 보고 Blueclaw를 기기 런타임으로 사용합니다. 설치 경로, 서비스 이름, 설정 파일, 헬스체크는 모두 Blueclaw 기준으로 고정되어 있으며, Radxa/RPi SD provisioning은 legacy/test path로 남겨둡니다.

## 런타임 계약

| 항목 | 값 |
|------|----|
| 바이너리 | `/usr/local/bin/blueclaw` |
| 서비스 | `blueclaw.service` |
| 시스템 유저 | `blueclaw` |
| 홈 디렉토리 | `/home/blueclaw` |
| 런타임 루트 | `/root/.blueclaw` |
| 설정 파일 | `/root/.blueclaw/config/runtime.json`, `/root/.blueclaw/config/policy.json` |
| 워크스페이스 | `/root/.blueclaw/workspace` |
| 헬스체크 | `http://127.0.0.1:8080/admin/api/policy` |

## 전체 흐름

```
사용자
  └─ Cloudflare Access
       └─ Cloudflare Tunnel
            └─ Jetson Orin Nano Super
                 ├─ Mattermost :8065
                 │    └─ internkim-capabilityd
                 ├─ internkim-admind
                 │    └─ companion broker / admin UI / backup
                 ├─ internkim-capabilityd
                 │    ├─ OpenRouter / local model / companion LLM routing
                 │    ├─ Mattermost / Slack / Signal platform I/O
                 │    └─ browser capability adapter
                 ├─ graphiti-memoryd :7791
                 └─ Blueclaw HTTP API :8080
                      ├─ managed DB
                      └─ gws / gws-bot helper 경로

Slack workspace
  └─ Slack Socket Mode
       └─ internkim-capabilityd
            └─ Blueclaw

사용자 컴퓨터
  └─ internkim-companion
       └─ internkim-admind companion broker
```

Mattermost와 관리자 UI는 Cloudflare Access 뒤의 외부 진입점이고, Blueclaw는 로컬 루프백에서만 응답합니다. Slack과 Signal은 `internkim-capabilityd`가 외부 이벤트를 받아 Blueclaw 작업으로 정규화합니다. Google, Mattermost, Slack, Signal, Cloudflare 설정은 host-side setup이 관리하고, Blueclaw는 이미 배치된 파일과 capability endpoint를 사용합니다.

## 디렉토리 구조

```text
/root/.blueclaw/
├── config/
│   ├── runtime.json
│   └── policy.json
└── workspace/
    ├── AGENTS.md
    ├── SOUL.md
    ├── IDENTITY.md
    ├── bin/
    ├── downloads/
    ├── sessions/
    └── skills/
```

## 보안 경계

민감 정보는 `/root/.internkim/secrets`, `/root/.internkim/config`, `/root/.internkim/state`에 두고, 필요한 프로세스만 읽을 수 있게 권한을 나눕니다. Blueclaw는 provider token, 브라우저 쿠키, 사용자 로컬 파일 경로, local model path를 직접 보지 않습니다.

| 파일 | 소비자 |
|------|--------|
| `/root/.internkim/secrets/openrouter-api-key` | internkim-capabilityd |
| `/root/.internkim/models/*` | internkim-capabilityd, local model wrapper |
| `/root/.internkim/secrets/google-sa.json` | gws / gws-bot |
| `/root/.internkim/secrets/gas-webhook-url` | GAS bridge helper |
| `/root/.internkim/secrets/slack-*` | internkim-capabilityd |
| `/root/.internkim/config/signal-*` | internkim-capabilityd |
| `/root/.internkim/state/companion-jobs.json` | internkim-admind |

## 설정 생성

`internkim`은 Blueclaw 설정을 템플릿 문자열이 아니라 구조화된 JSON 문서로 생성합니다.

- `runtime.json`
  - loopback listen 주소
  - capabilityd provider endpoint
  - workspace root
  - 실행 허용/차단 명령
- `policy.json`
  - 관리자 identity
  - retention 정책

생성 로직은 `internal/runtime/blueclaw/`에 모여 있고, live setup과 SD firstboot가 같은 계약을 사용합니다.

## 서비스 기동

시스템은 아래 unit 계약으로 Blueclaw를 띄웁니다.

```ini
[Service]
User=blueclaw
Environment=HOME=/home/blueclaw
Environment=BLUECLAW_SESSIONS_DIR=/root/.blueclaw/workspace/sessions
ExecStart=/usr/local/bin/blueclaw -runtime /root/.blueclaw/config/runtime.json -policy /root/.blueclaw/config/policy.json
```

헬스체크는 systemd active 상태와 `curl -fsS http://127.0.0.1:8080/admin/api/policy` 둘 다 통과해야 성공으로 봅니다.

## Google Workspace 연동

Google 생성 플로우는 현재 예전 방식으로 유지되어 있습니다.

- 생성은 Apps Script bridge 또는 사용자 권한 경로
- 후속 편집은 `gws-bot`
- 기기 workspace skill은 `/root/.blueclaw/workspace/skills/*` 아래에 배치

즉, Google 연동은 Blueclaw 내부 설정이 아니라 host-side provisioned helper 집합으로 붙습니다.
