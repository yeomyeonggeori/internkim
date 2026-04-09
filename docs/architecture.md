# ZeroClaw 아키텍처 — feat/zeroclaw 브랜치

> 이 문서는 `feat/zeroclaw` 브랜치의 설계 결정과 보안 아키텍처를 정리한 스펙 문서입니다.

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

## 전체 아키텍처

```
사용자 (Google 로그인)
    │
    ▼
Cloudflare Access (SAML SSO)
    │
    ▼  Argo Tunnel
┌─────────────────────────────────────┐
│  ARM 보드                           │
│                                     │
│  zeroclaw.service  (uid: zeroclaw)  │
│  ┌───────────────────────────────┐  │
│  │ ZeroClaw gateway :18790       │  │
│  │ OpenRouter (전체 모델)         │  │
│  │ native runtime (Landlock)     │  │
│  └──────────────┬────────────────┘  │
│                 │ stdio MCP         │
│                 ▼                   │
│  /usr/local/bin/gws-mcp (root:755)  │
│    └─ sudo -u gws /usr/local/bin/gws mcp │
│         (uid: gws)                  │
│                                     │
│  /root/.internkim/secrets/          │
│    google-sa.json   (gws:640)       │  ← gws만 읽기 가능
│    openrouter-api-key (zeroclaw:640)│  ← zeroclaw만 읽기 가능
└─────────────────────────────────────┘
    │                    │
    ▼                    ▼
OpenRouter API    Google Drive/Docs/Gmail
```

## 보안 격리 설계

### 핵심 원칙

LLM이 실행되는 컨테이너가 크리덴셜에 접근하는 모든 경로를 OS 레벨에서 차단.

### 파일 권한

```
/root/.internkim/secrets/          owner: root  mode: 700
  google-sa.json                   owner: gws   mode: 640
  openrouter-api-key               owner: zeroclaw mode: 640
```

- **LLM 컨테이너** (`nobody` uid): secrets 디렉토리 자체가 마운트되지 않음 → 경로를 알아도 접근 불가
- **zeroclaw 프로세스**: `openrouter-api-key`만 읽기 가능, `google-sa.json`은 `Permission denied`
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
# 크리덴셜 경로 없음
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
EnvironmentFile=-/root/.internkim/secrets/openrouter-api-key
ExecStart=/usr/local/bin/zeroclaw gateway
```

## ZeroClaw 설정 (`~/.zeroclaw/config.toml`)

```toml
default_provider = "openai-compatible:https://openrouter.ai/api/v1"
default_model = "openrouter/google/gemini-flash-1.5"

[runtime]
kind = "native"

[runtime.native]
sandbox = "landlock"  # Landlock + Bubblewrap, Docker 데몬 불필요

[[mcp.servers]]
name = "google-workspace"
command = "/usr/local/bin/gws-mcp"
args = []

[channels.board]
enabled = true
token = "quickclaw"
allow_token_query = true
allow_origins = ["*"]
```

## Google 서비스 계정 자동 생성

`internkim setup` step 8에서 한 번 실행:

1. OAuth2 device flow → 브라우저에서 Google 로그인 (cloud-platform 스코프)
2. Google IAM REST API → `internkim-{device_id}` 서비스 계정 생성
3. SA 키 JSON 발급 → 보드 SCP → `/root/.internkim/secrets/google-sa.json`
4. `chown gws`, `chmod 640` 적용

이후 ZeroClaw가 gws MCP를 통해 Google Workspace 작업 수행. 생성된 파일은 서비스 계정 소유 → 사용자 이메일로 공유 필요 시 `gws drive files share` 사용.

## 개발 시뮬레이터

별도 ARM 환경(SSH 접속 가능)을 시뮬레이터로 사용:

```bash
# SSH 접속 (localhost:2222)
./internkim sim ssh

# 시뮬레이터에 provisioning
./internkim setup --sim
```

`--sim` 플래그는 `detectBoard()`에서 `localhost:2222`를 우선 사용.

## board-bridge API 경로

| 경로 | 설명 |
|------|------|
| `GET/WS /board/ws` | ZeroClaw WebSocket 프록시 (:18790) |
| `GET /board/token` | 인증 토큰 (`pico-{pid_token}{channel_token}`) |
| `GET/PUT /board/model` | 현재 모델 조회/변경 |
| `GET /board/history/{session_id}` | 세션 대화 히스토리 |
| `GET /board/me` | CF Access 사용자 이메일 |
| `GET /board/files/{path}` | workspace 파일 다운로드 |

## 변경된 파일

| 파일 | 변경 내용 |
|------|----------|
| `main.go` | GOARCH arm64, zeroclaw 경로, ZeroClaw+gws 배포, Google SA 자동생성, sim 커맨드 |
| `cmd/board-bridge/main.go` | pico-bridge에서 복사 후 `/pico/*` → `/board/*`, 경로 zeroclaw |
| `README.md` | 전체 업데이트 |
| `docs/architecture.md` | 이 문서 |

## 소스 수정 없는 외부 컴포넌트

- **ZeroClaw**: `cargo build --target aarch64-unknown-linux-gnu --release` 크로스컴파일
- **gws (googleworkspace/cli)**: Rust, ARM64 크로스컴파일
- **Mattermost**: Cloudflare Access SAML SSO 설정만, 소스 수정 없음
- **cloudflared**: 기존과 동일
