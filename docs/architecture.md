# ZeroClaw 아키텍처 — feat/zeroclaw 브랜치

> `feat/zeroclaw` 브랜치의 설계 결정과 보안 아키텍처 스펙 문서.

## 배경 및 전환 이유

| 항목 | 이전 (main) | 변경 (feat/zeroclaw) |
|------|------------|---------------------|
| 하드웨어 | LicheeRV Nano (RISC-V) | Radxa 5c / RPi 5 (ARM64) |
| AI 엔진 | PicoClaw | ZeroClaw (Rust) |
| 빌드 타겟 | `GOARCH=riscv64` | `GOARCH=arm64` |
| 설정 경로 | `/root/.picoclaw/` | `/root/.zeroclaw/` |
| HTTP 브릿지 | `cmd/pico-bridge` `/pico/*` | `cmd/board-bridge` `/board/*` |
| Google Workspace | 없음 | gws MCP 서버 |
| 크리덴셜 관리 | config에 인라인 | `/root/.internkim/secrets/` 분리 |
| 채팅 UI | 웹 UI 전용 | 웹 UI + Mattermost |

## 전체 아키텍처

```
사용자 (Google 로그인)
    │
    ▼
Cloudflare Access (SAML SSO)
    │
    ▼  Argo Tunnel
┌─────────────────────────────────────────────┐
│  ARM 보드 (Radxa 5c / RPi 5)               │
│                                             │
│  board-bridge :8080   ←── Cloudflare 터널  │
│  ┌─────────────────────────────────────┐   │
│  │ /board/chat   → ZeroClaw /webhook   │   │
│  │ /board/token  → webhook secret      │   │
│  │ /board/model  → config.toml 읽기/쓰기│   │
│  │ /             → SPA 정적 파일 서빙   │   │
│  └─────────────────────────────────────┘   │
│                   │                         │
│                   ▼ HTTP POST               │
│  zeroclaw.service :18790 (uid: zeroclaw)    │
│  ┌─────────────────────────────────────┐   │
│  │ ZeroClaw gateway                    │   │
│  │ OpenRouter (전체 모델)               │   │
│  │ native runtime (Landlock)           │   │
│  └──────────────┬──────────────────────┘   │
│                 │ stdio MCP                 │
│                 ▼                           │
│  /usr/local/bin/gws-mcp (root:755)          │
│    └─ sudo -u gws /usr/local/bin/gws mcp   │
│         (uid: gws)                          │
│                                             │
│  /root/.internkim/secrets/                  │
│    google-sa.json     (gws:640)             │
│    openrouter-api-key (zeroclaw:640)        │
│    mattermost-bot-token (zeroclaw:640)      │
└─────────────────────────────────────────────┘
    │                         │
    ▼                         ▼
OpenRouter API         Google Drive/Docs/Gmail

사용자 (Mattermost 앱)
    │
    ▼ REST polling (3s)
Mattermost 서버 (온보드)
    │
    ▼
ZeroClaw (channels.mattermost)
```

## 보안 격리 설계

### 핵심 원칙

LLM이 실행되는 샌드박스가 크리덴셜에 접근하는 모든 경로를 OS 레벨에서 차단.

### 파일 권한

```
/root/.internkim/secrets/          owner: root  mode: 700
  google-sa.json                   owner: gws   mode: 640
  openrouter-api-key               owner: zeroclaw mode: 640
  mattermost-bot-token             owner: zeroclaw mode: 640

/root/.internkim/
  mattermost-url                   owner: root  mode: 600
  mattermost-admin-token           owner: root  mode: 600
```

- **Landlock 샌드박스** (zeroclaw 내 LLM 실행 영역): secrets 디렉토리가 허용 경로에 없음 → 경로를 알아도 접근 불가
- **zeroclaw 프로세스**: `openrouter-api-key`, `mattermost-bot-token`만 읽기 가능
- **gws 프로세스**: `google-sa.json`만 읽기 가능

### gws-mcp wrapper

ZeroClaw config.toml에 SA 키 경로가 노출되지 않도록 wrapper 스크립트를 사용:

```bash
# /usr/local/bin/gws-mcp  (owner: root, mode: 755)
#!/bin/bash
export GOOGLE_WORKSPACE_CLI_CREDENTIALS_FILE=/root/.internkim/secrets/google-sa.json
exec sudo -u gws /usr/local/bin/gws mcp
```

ZeroClaw config.toml:
```toml
[[mcp.servers]]
name = "google-workspace"
command = "/usr/local/bin/gws-mcp"
args = []
# SA 키 경로 없음
```

sudoers:
```
zeroclaw ALL=(gws) NOPASSWD: /usr/local/bin/gws
```

### stdio MCP 동작 방식

ZeroClaw가 `gws-mcp`를 자식 프로세스로 spawn → stdin/stdout으로 MCP 프로토콜 통신. HTTP 포트 불필요, 네트워크 노출 없음.

```
ZeroClaw (zeroclaw uid)
  └─ spawn: /usr/local/bin/gws-mcp
       └─ sudo -u gws: /usr/local/bin/gws mcp
            stdin/stdout ←→ MCP protocol
```

### OpenRouter API 키

zeroclaw.service systemd unit이 시작 시 파일에서 읽어 환경변수로 주입:

```ini
[Service]
User=zeroclaw
EnvironmentFile=/root/.internkim/secrets/openrouter-api-key
ExecStart=/usr/local/bin/zeroclaw gateway
```

## ZeroClaw 설정 (`~/.zeroclaw/config.toml`)

```toml
default_provider = "openai-compatible:https://openrouter.ai/api/v1"
default_model = "google/gemini-3.1-flash-lite-preview"

[runtime]
kind = "native"

[runtime.native]
sandbox = "landlock"  # Landlock + Bubblewrap, Docker 데몬 불필요

[gateway]
port = 18790
host = "127.0.0.1"
require_pairing = false

[channels_config]
cli = false

[channels_config.webhook]
secret = "quickclaw"

# Mattermost 채널 (setup에서 설정 시 추가됨)
[channels.mattermost]
enabled = true
base_url = "https://chat.example.test"
bot_token = "..."
channel_id = "..."
thread_replies = true
mention_only = false
allowed_users = ["*"]

[[mcp.servers]]
name = "google-workspace"
command = "/usr/local/bin/gws-mcp"
args = []
```

## board-bridge API 경로

board-bridge는 ZeroClaw gateway(`localhost:18790`)의 HTTP 프록시입니다.

| 경로 | 메서드 | 설명 |
|------|--------|------|
| `/board/chat` | POST | ZeroClaw `/webhook`으로 메시지 전달 (JSON `{"message":"..."}`) |
| `/board/chat/stream` | POST | SSE 스트리밍 (ZeroClaw 지원 시) |
| `/board/token` | GET | webhook secret 반환 |
| `/board/model` | GET/PUT | 현재 모델 조회/변경 (config.toml 직접 수정 + 재시작) |
| `/board/files/{path}` | GET | workspace 파일 다운로드 |
| `/board/me` | GET | CF Access 사용자 이메일 |
| `/health` | GET | ZeroClaw health check 전달 |

### 메시지 흐름

```
웹 UI
  └─ POST /board/chat {"message": "..."}
       └─ board-bridge
            └─ POST http://localhost:18790/webhook
                 X-Webhook-Secret: quickclaw
                 {"message": "..."}
                 └─ ZeroClaw LLM 처리
                 └─ {"response": "...", "model": "..."}
            └─ JSON 응답 → 웹 UI
```

## Mattermost 설정

ZeroClaw가 Mattermost REST API v4를 3초마다 폴링합니다. bot token + channel_id 필요.

### 준비 사항

1. Mattermost 서버 설치 (온보드 또는 외부)
2. 봇 계정 생성: System Console → Integrations → Bot Accounts
3. 채널 ID 확인: 채널 URL의 마지막 부분

### 푸시 알림 프라이버시

`internkim setup` step 9에서 선택:
- `full` — 메시지 내용 포함 (push.mattermost.com 경유)
- `generic` — "새 메시지가 있습니다" (내용 미포함)
- `id_loaded` — 알림만 전달, 앱이 서버에서 직접 내용 가져옴 (권장, 기본값)

## Google 서비스 계정 자동 생성

`internkim setup` step 8에서 한 번 실행:

1. OAuth2 device flow → 브라우저에서 Google 로그인 (cloud-platform 스코프)
2. Google IAM REST API → `internkim-{device_id}` 서비스 계정 생성
3. SA 키 JSON 발급 → 보드 SCP → `/root/.internkim/secrets/google-sa.json`
4. `chown gws`, `chmod 640` 적용

## 개발 시뮬레이터

```bash
# SSH 접속 (localhost:2222)
./internkim sim ssh

# 시뮬레이터에 provisioning
./internkim setup --sim
./internkim deploy --sim
```

## 변경된 파일

| 파일 | 변경 내용 |
|------|----------|
| `main.go` | GOARCH arm64, zeroclaw 설치, Google SA, Mattermost setup, sim 커맨드 |
| `cmd/board-bridge/main.go` | ZeroClaw gateway HTTP 프록시 (/board/* 경로) |
| `README.md` | ARM64/ZeroClaw/gws/Mattermost 스택 업데이트 |
| `docs/architecture.md` | 이 문서 |

## 소스 수정 없는 외부 컴포넌트

- **ZeroClaw**: `cargo build --target aarch64-unknown-linux-gnu --release`
- **gws (Google Workspace CLI)**: `cargo build --target aarch64-unknown-linux-gnu --release`
- **Mattermost**: Cloudflare Access SAML SSO 설정만, 소스 수정 없음
- **cloudflared**: 기존과 동일
