# Blueclaw 아키텍처

현재 `internkim`은 Blueclaw를 보드 런타임으로 사용합니다. 설치 경로, 서비스 이름, 설정 파일, 헬스체크는 모두 Blueclaw 기준으로 고정되어 있습니다.

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
            └─ Mattermost :8065
                 └─ Blueclaw connector
                      └─ Blueclaw HTTP API :8080
                           ├─ OpenRouter
                           ├─ managed DB
                           └─ gws / gws-bot helper 경로

Slack workspace
  └─ Slack Events API
       └─ Blueclaw connector
```

Mattermost는 Cloudflare Access 뒤의 외부 진입점이고, Blueclaw는 로컬 루프백에서만 응답합니다. Slack은 Slack App/Event API를 통해 Blueclaw connector로 들어옵니다. Google, Mattermost, Slack, Cloudflare 설정은 host-side setup이 관리하고, Blueclaw는 이미 배치된 파일과 환경변수를 사용합니다.

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

민감 정보는 `/root/.internkim/secrets`와 `/root/.internkim/env`에 두고, 필요한 프로세스만 읽을 수 있게 권한을 나눕니다.

| 파일 | owner | mode | 소비자 |
|------|-------|------|--------|
| `/root/.internkim/secrets/openrouter-api-key` | `blueclaw` | `640` | Blueclaw |
| `/root/.internkim/secrets/google-sa.json` | `gws` | `640` | gws / gws-bot |
| `/root/.internkim/secrets/gas-webhook-url` | `root:blueclaw` | `640` | GAS bridge helper |
| `/root/.internkim/env/sa-email` | `root:blueclaw` | `640` | GAS bridge helper |
| `/root/.internkim/env/mattermost-url` | `root` | `600` | host setup, Blueclaw env |
| `/root/.internkim/env/bot-token` | `root:blueclaw` | `640` | Blueclaw env |

## 설정 생성

`internkim`은 Blueclaw 설정을 템플릿 문자열이 아니라 구조화된 JSON 문서로 생성합니다.

- `runtime.json`
  - loopback listen 주소
  - OpenRouter 기본 provider
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
EnvironmentFile=/root/.internkim/secrets/openrouter-api-key
Environment=HOME=/home/blueclaw
Environment=BLUECLAW_SESSIONS_DIR=/root/.blueclaw/workspace/sessions
Environment=MM_URL_FILE=/root/.internkim/env/mattermost-url
Environment=MM_BOT_TOKEN_FILE=/root/.internkim/env/bot-token
ExecStart=/usr/local/bin/blueclaw -runtime /root/.blueclaw/config/runtime.json -policy /root/.blueclaw/config/policy.json
```

헬스체크는 systemd active 상태와 `curl -fsS http://127.0.0.1:8080/admin/api/policy` 둘 다 통과해야 성공으로 봅니다.

## Google Workspace 연동

Google 생성 플로우는 현재 예전 방식으로 유지되어 있습니다.

- 생성은 Apps Script bridge 또는 사용자 권한 경로
- 후속 편집은 `gws-bot`
- 보드 skill은 `/root/.blueclaw/workspace/skills/*` 아래에 배치

즉, Google 연동은 Blueclaw 내부 설정이 아니라 host-side provisioned helper 집합으로 붙습니다.
