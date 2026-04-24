# Intern Kim

ARM 보드(Radxa 5c / RPi 5)에 Blueclaw 런타임을 올리고, Cloudflare Tunnel로 어디서든 접속 가능한 턴키 하드웨어입니다.

전원만 꽂으면 보드가 독립적으로 동작한다. 별도 컴퓨터 불필요.

## 아키텍처

```
사용자 (Google 로그인)
    │
    ▼
Cloudflare Access (Google 로그인)
    │
    ▼  Cloudflare Tunnel
┌─────────────────────────────────────────┐
│  ARM 보드 (Radxa 5c / RPi 5)           │
│                                         │
│  Mattermost :8065 ← Cloudflare 터널    │
│          │                              │
│          ▼                              │
│  blueclaw.service :8080                │
│  ┌─────────────────────────────────┐   │
│  │ Blueclaw runtime                │   │
│  │ policy / memory / task / DB     │   │
│  └──────────────┬──────────────────┘   │
│                 │ stdio MCP             │
│                 ▼                       │
│  /usr/local/bin/gws-bot                 │
│                                         │
│  /root/.internkim/secrets/              │
│    google-sa.json   (gws:640)           │
│    openrouter-api-key (blueclaw:640)    │
│                                         │
│  /root/.blueclaw/                       │
│    config/runtime.json                  │
│    config/policy.json                   │
│    workspace/                           │
└─────────────────────────────────────────┘
    │                         │
    ▼                         ▼
OpenRouter API         Google Drive/Docs/Gmail

사용자 (Slack)
    └─▶ Slack App / Events API
             └─▶ Blueclaw connector
```

## 보안 설계

크리덴셜은 `/root/.internkim/secrets/`에 집중 관리. 각 크리덴셜은 필요한 uid만 읽을 수 있음:

| 파일 | owner | mode | 접근 가능 |
|------|-------|------|----------|
| `google-sa.json` | gws | 640 | gws 프로세스만 |
| `openrouter-api-key` | blueclaw | 640 | blueclaw 프로세스만 |

LLM이 실행되는 런타임 샌드박스는 secrets 디렉토리를 직접 읽지 못하도록 분리되어 있어, 경로를 알더라도 접근할 수 없습니다.

## 구성 요소

| 구성 | 설명 |
|------|------|
| **Go CLI** (`cmd/internkim/main.go`) | 셋업 도구. Wi-Fi → Blueclaw/gws 설치 → API 키 → Google SA → Mattermost → 기기 등록 |
| **Blueclaw** | 런타임 바이너리. `/usr/local/bin/blueclaw`, `/root/.blueclaw/config/*.json`, `/root/.blueclaw/workspace/*` 계약을 사용 |
| **gws** | Google Workspace CLI. Drive/Docs/Gmail/Sheets 조작. MCP 서버 모드 지원 |
| **Mattermost** | 온보드 채팅 서버. 기기 협업 채널과 모바일 알림에 사용 |
| **Slack connector** | Slack workspace에서 들어오는 DM/channel 이벤트를 Blueclaw 작업으로 전달 |
| **SvelteKit 웹앱** (`web/`) | Cloudflare Pages. 기기 등록 API, Access policy 동기화, OTA |
| **보드 바이너리** (`build/board-bin/`) | ARM64 정적 링크 바이너리 [gitignored] |
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
make build
./internkim setup
```

10단계 자동 진행:
1. 보드 감지 (USB NCM)
2. Wi-Fi 감지 + 키체인 비밀번호
3. 보드 Wi-Fi 설정
4. Wi-Fi 연결 확인
5. Blueclaw + gws + cloudflared 설치, 시스템 유저 생성
6. OpenRouter API 키 → `/root/.internkim/secrets/openrouter-api-key`
7. 기기 등록 + Cloudflare 터널 시작
8. Google 서비스 계정 자동 생성 → `/root/.internkim/secrets/google-sa.json`
9. Mattermost 설정 (URL / admin token / bot token / channel ID, 건너뛰기 가능)
10. `blueclaw.service` 시작 + 스왑 + 최신 빌드 배포

### Tart Lab

실제 보드 대신 macOS + Tart ARM Linux VM에서 Blueclaw와 Mattermost를 포함한 소프트웨어 E2E를 테스트합니다:

```bash
make deps-sim

# Tart 이미지 준비
./internkim lab image-build

# VM 부팅
./internkim lab vm-up

# VM provisioning + internkim setup
./internkim lab setup

# 전체 소프트웨어 acceptance
./internkim lab scenario-e2e

# 또는 한 번에
./internkim setup --sim

# VM SSH 접속
./internkim lab vm-ssh
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
CF_DOMAIN=example.test
INTERNKIM_REGISTER_SECRET=...
```

## API 경로

| 메서드 | 경로 | 설명 |
|--------|------|------|
| POST | `/api/register` | 기기 등록, Cloudflare Tunnel, Access app/policy 생성 |
| GET/POST | `/api/users` | 허용 사용자 조회/추가 및 Access policy 동기화 |
| DELETE | `/api/users/{email}` | 허용 사용자 제거 및 Access policy 동기화 |
| GET/POST | `/api/ota/*` | Blueclaw / CLI OTA 확인과 보고 |

## 디렉토리 구조

```
internkim/
├── go.mod / go.sum
├── cmd/
│   ├── internkim/           Go CLI 엔트리포인트
│   └── download/            보드 헬퍼 바이너리
├── internal/
│   ├── cli/                 셋업, 배포, lab, 상태 명령 구현
│   ├── board/               보드 자산 경로와 로더
│   ├── google/browser/      Google 브라우저 자동화
│   ├── lab/                 Tart 기반 실험실 구성과 시나리오
│   ├── provisioning/steps/  단계별 셋업 플로우
│   └── runtime/blueclaw/    Blueclaw 런타임 계약과 설정 생성
├── bin/                     macOS 유틸 (get-ssid, sshpass)
├── build/                   보드 바이너리와 정적 웹 출력 [gitignored]
├── .dependency/blueclaw/    Blueclaw git submodule
├── assets/board/skills/     보드 AI 스킬 (SKILL.md)
├── config/lab.example.json  Tart lab 설정 예시
├── lab/scripts/             VM provisioning / 시나리오 스크립트
├── web/                     SvelteKit + Cloudflare Pages
├── docs/
│   └── architecture.md      아키텍처 상세 문서
└── .env.example
```

## 비용

- 도메인: ~$10/년
- Cloudflare (Tunnel + Access + Pages + KV): 무료 tier
- OpenRouter API: 종량제 (무료 모델 사용 가능)
- Google Cloud IAM: 무료

## 라이선스

TBD
