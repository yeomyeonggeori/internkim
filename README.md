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
│  internkim-capabilityd                 │
│  graphiti-memoryd :7791                │
│  blueclaw.service :8080                │
│  ┌─────────────────────────────────┐   │
│  │ Blueclaw runtime                │   │
│  │ policy / task / ACL / prompt    │   │
│  └──────────────┬──────────────────┘   │
│                 │ stdio MCP             │
│                 ▼                       │
│  /usr/local/bin/gws-bot                 │
│                                         │
│  /root/.internkim/secrets/              │
│    google-sa.json   (gws:640)           │
│    openrouter-api-key (root only)       │
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
             └─▶ InternKim platform sidecar
                    └─▶ Blueclaw connector

사용자 컴퓨터
    └─▶ internkim-companion
             └─▶ user-local browser / local model capability
```

## 보안 설계

크리덴셜은 `/root/.internkim/secrets/`에 집중 관리. 각 크리덴셜은 필요한 uid만 읽을 수 있음:

| 파일 | owner | mode | 접근 가능 |
|------|-------|------|----------|
| `google-sa.json` | gws | 640 | gws 프로세스만 |
| `openrouter-api-key` | root | 600 | internkim-capabilityd만 |

Blueclaw와 Graphiti는 secrets 디렉토리를 직접 읽지 않습니다. LLM, embedding, platform reply 같은 secret-bearing 작업은 `internkim-capabilityd`의 local capability API를 통해서만 실행합니다.

## 구성 요소

| 구성 | 설명 |
|------|------|
| **Go CLI** (`cmd/internkim/main.go`) | 셋업 도구. Wi-Fi → Blueclaw/gws 설치 → API 키 → Google SA → Mattermost → 기기 등록 |
| **Blueclaw** | 런타임 바이너리. `/usr/local/bin/blueclaw`, `/root/.blueclaw/config/*.json`, `/root/.blueclaw/workspace/*` 계약을 사용 |
| **Graphiti memoryd** | Blueclaw memory sidecar. `graphiti-core[kuzu]`로 episode ingestion, temporal graph extraction, hybrid graph search 수행 |
| **internkim-capabilityd** | OpenRouter, LiteRT, Mattermost, Slack, Signal credential을 보유하고 capability API만 노출 |
| **internkim-companion** | 사용자 컴퓨터의 cross-platform trusted runtime. 브라우저 human-in-the-loop와 향후 local-only LLM capability 제공 |
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

Mattermost self-hosted는 기본적으로 한 team의 총 멤버 수에 제한이 있습니다. 기본값은 `TeamSettings.MaxUsersPerTeam = 50`이며, 활성/비활성 사용자를 포함합니다. 반복 검증에서 테스트 사용자를 지우지 않으면 이 제한에 걸려 team/channel join API가 실패할 수 있습니다. 필요하면 운영 환경에서 이 값을 늘릴 수 있지만, 테스트 코드는 생성한 Mattermost 테스트 사용자를 정리해야 합니다.

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

# 설치 후 API/Mattermost connector 검증
./internkim setup --sim --verify

# API/Mattermost 검증 + 브라우저 smoke
./internkim setup --sim --verify-browser

# VM SSH 접속
./internkim lab vm-ssh
```

### 로컬 Graphiti Smoke

보드 없이 macOS 로컬에서 실제 `graphiti-core[kuzu]` sidecar, InternKim capabilityd, OpenRouter LLM/embedding 경로를 함께 검증합니다. `.env` 또는 환경변수에 `OPENROUTER_API_KEY`가 필요합니다.

```bash
make verify-graphiti-local
```

### Companion Runtime

`internkim-companion`은 사용자 컴퓨터에서 실행되는 capability provider입니다. v1은 브라우저 작업 중 사용자 로그인, MFA, 파일 선택, 승인 입력처럼 사람이 필요한 단계를 처리하기 위한 데몬 골격을 제공합니다. 장기적으로는 같은 capability contract로 사용자 컴퓨터의 더 강한 로컬 모델, embedding, 파일, desktop action도 처리합니다.

```bash
make build-companion
./internkim-companion --listen 127.0.0.1:7979 --local-only
./internkim-companion --listen 127.0.0.1:7979 --local-only --dev-mock-llm
```

InternKim `capabilityd`는 companion이 설정되어 있으면 `browser.*`, `user.*`, `file.pick`, companion LLM capability를 provider-neutral하게 라우팅할 수 있습니다. Blueclaw는 provider 구현, 브라우저 바이너리, 로컬 모델 경로, 사용자 브라우저 쿠키를 보지 않습니다.

후속 정리 대상: Blueclaw의 기본 runtime config에는 아직 native terminal profile이 남아 있습니다. 제품 기본 경로는 typed capability/MCP tool이어야 하며, terminal은 dev/admin profile 전용으로 낮춰야 합니다.

### Blueclaw 기록/메모리 초기화

테스트 중 만든 Blueclaw task, conversation, raw event, legacy memory, Graphiti mirror, Kuzu memory files를 지우려면:

```bash
./internkim reset blueclaw-history --plan
./internkim reset blueclaw-history --confirm <deviceID>
./internkim reset blueclaw-history --keep-mattermost-posts --confirm <deviceID>
```

기본 reset은 Blueclaw task, raw event, conversation, legacy memory, Graphiti mirror, Kuzu memory files와 Mattermost 화면에 보이는 post/reaction/thread 기록을 함께 지웁니다. 초대 사용자, policy, platform account link, secrets, Mattermost 사용자, 팀, 채널은 유지합니다. 디버깅 때문에 Mattermost 화면 기록만 남겨야 할 때는 `--keep-mattermost-posts`를 명시합니다. Mattermost/Slack/Signal 검증에서 만든 테스트 메시지와 봇 답변은 검증 직후 삭제해야 합니다.

Slack과 Signal은 외부 플랫폼이므로 이 reset이 원격 서비스의 전체 메시지 기록을 강제로 비우지는 않습니다. InternKim이 만든 테스트 메시지와 봇 답변은 가능한 범위에서 삭제하고, Blueclaw/Graphiti 쪽 기억과 작업 기록은 항상 reset 대상에 포함합니다.

### 웹앱 (Cloudflare Pages)

```bash
cd web
bun install
bun run dev
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
│   ├── internkim-companion/  사용자 컴퓨터 trusted runtime 데몬
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
