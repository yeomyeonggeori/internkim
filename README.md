# Intern Kim

ARM 보드(Radxa 5c / RPi 5)에 [ZeroClaw](https://github.com/qwibitai/nanoclaw) AI 에이전트를 탑재하고, Cloudflare Tunnel로 어디서든 접속 가능한 턴키 하드웨어.

전원만 꽂으면 보드가 독립적으로 동작한다. 별도 컴퓨터 불필요.

## 아키텍처

```
사용자 (Google 로그인)
    │
    ▼
Cloudflare Access (SAML SSO)
    │
    ▼  Argo Tunnel
┌─────────────────────────────────────────┐
│  ARM 보드 (Radxa 5c / RPi 5)           │
│                                         │
│  zeroclaw.service (uid: zeroclaw)       │
│  ┌─────────────────────────┐            │
│  │ ZeroClaw gateway :18790 │            │
│  │ (OpenRouter — 전체 모델) │            │
│  └────────────┬────────────┘            │
│               │ stdio MCP               │
│               ▼                         │
│  /usr/local/bin/gws-mcp (root:755)      │
│    └── sudo -u gws gws mcp             │
│          (uid: gws, SA 키 소유)          │
│                                         │
│  /root/.internkim/secrets/              │
│    google-sa.json   (owner: gws  640)   │
│    openrouter-api-key (owner: zeroclaw 640) │
└─────────────────────────────────────────┘
    │
    ▼
Google Drive / Docs / Gmail
OpenRouter API
```

## 보안 설계

크리덴셜은 `/root/.internkim/secrets/`에 집중 관리. 각 크리덴셜은 필요한 uid만 읽을 수 있음:

| 파일 | owner | mode | 접근 가능 |
|------|-------|------|----------|
| `google-sa.json` | gws | 640 | gws 프로세스만 |
| `openrouter-api-key` | zeroclaw | 640 | zeroclaw 프로세스만 |

LLM이 실행되는 Docker 컨테이너는 `nobody` uid로 동작하며 secrets 디렉토리가 마운트되지 않아 경로를 알더라도 접근 불가.

## 구성 요소

| 구성 | 설명 |
|------|------|
| **Go CLI** (`main.go`) | 셋업 도구. Wi-Fi → ZeroClaw/gws 설치 → API 키 → Google SA → 기기 등록 |
| **ZeroClaw** | Rust 단일 바이너리. OpenRouter 전체 모델, Landlock + Bubblewrap 커널 레벨 격리 |
| **gws** | Google Workspace CLI. Drive/Docs/Gmail/Sheets 조작. MCP 서버 모드 지원 |
| **board-bridge** (`cmd/board-bridge/`) | Go HTTP 브릿지. `/board/*` 경로로 ZeroClaw 게이트웨이 프록시 |
| **SvelteKit 웹앱** (`web/`) | Cloudflare Pages. 기기 등록 API, 사용자 관리, OTA |
| **보드 바이너리** (`board-bin/`) | ARM64 정적 링크 바이너리 [gitignored] |
| **맥 유틸** (`bin/`) | get-ssid + sshpass, macOS universal binary |

## 셋업

### 사전 준비

- macOS (Apple Silicon 또는 Intel)
- ARM 보드 (Radxa 5c 또는 RPi 5) + USB-C 데이터 케이블
- Wi-Fi 네트워크
- [OpenRouter API 키](https://openrouter.ai/keys)
- Google Cloud 프로젝트 (Google Workspace 연동 시)

### Go CLI 빌드 & 실행

```bash
go build -o internkim .
./internkim setup
```

9단계 자동 진행:
1. 보드 감지 (USB NCM)
2. Wi-Fi 감지 + 키체인 비밀번호
3. 보드 Wi-Fi 설정
4. Wi-Fi 연결 확인
5. ZeroClaw + gws + cloudflared 설치, 시스템 유저 생성
6. OpenRouter API 키 → `/root/.internkim/secrets/openrouter-api-key`
7. 기기 등록 + Cloudflare 터널 시작
8. Google 서비스 계정 자동 생성 → `/root/.internkim/secrets/google-sa.json`
9. ZeroClaw systemd 서비스 시작 + 스왑

### 개발 시뮬레이터

실제 보드 대신 SSH로 접속 가능한 ARM 환경(별도 구성)에서 개발:

```bash
# 시뮬레이터 SSH 접속 (localhost:2222)
./internkim sim ssh

# 시뮬레이터에 provisioning
./internkim setup --sim
```

### 웹앱 (Cloudflare Pages)

```bash
cd web
npm install
npm run dev
```

환경변수 (`.dev.vars`):
```
CF_API_TOKEN=...
CF_ACCOUNT_ID=...
CF_ZONE_ID=...
CF_DOMAIN=intern.kim
REGISTER_SECRET=...
```

## API 경로 (board-bridge)

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET/WS | `/board/ws` | ZeroClaw WebSocket 프록시 |
| GET | `/board/token` | 인증 토큰 발급 |
| GET/PUT | `/board/model` | 현재 모델 조회/변경 |
| GET | `/board/history/{id}` | 세션 히스토리 |
| GET | `/board/me` | CF Access 사용자 정보 |
| GET | `/board/files/{path}` | workspace 파일 다운로드 |

## 디렉토리 구조

```
quick-claw/
├── main.go                  Go CLI (셋업, 배포, sim)
├── go.mod / go.sum
├── cmd/
│   └── board-bridge/        Go HTTP 브릿지 (/board/* 경로)
├── bin/                     macOS 유틸 (get-ssid, sshpass)
├── board-bin/               ARM64 바이너리 [gitignored]
├── board-scripts/skills/    보드 AI 스킬 (SKILL.md)
├── web/                     SvelteKit + Cloudflare Pages
├── docs/
│   └── architecture.md           아키텍처 상세 문서
└── .env.example
```

## 비용

- 도메인: ~$10/년
- Cloudflare (Tunnel + Access + Pages + KV): 무료 tier
- OpenRouter API: 종량제 (무료 모델 사용 가능)
- Google Cloud IAM: 무료

## 라이선스

TBD
