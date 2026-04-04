# Quick Claw

LicheeRV Nano 보드에 [PicoClaw](https://github.com/picoclaw/picoclaw) AI 에이전트를 탑재하고, Cloudflare Tunnel로 어디서든 접속 가능한 턴키 하드웨어.

전원만 꽂으면 보드가 독립적으로 동작한다. 별도 컴퓨터 불필요.

## 아키텍처

```
사용자 브라우저
    │
    ▼
Cloudflare (Access + Pages)
    │
    ▼  Argo Tunnel
┌─────────────────────────────┐
│  LicheeRV Nano              │
│  ┌─────────────────────┐    │
│  │ cloudflared (RISC-V)│────┘  ← S98cloudflared
│  └─────────┬───────────┘
│            ▼ localhost:18790
│  ┌─────────────────────┐
│  │ picoclaw gateway    │       ← S99picoclaw
│  │ (OpenRouter LLM)    │
│  └─────────────────────┘
└─────────────────────────────┘
```

## 구성 요소

| 구성 | 설명 |
|------|------|
| **Go CLI** (`main.go`) | 초기 셋업 도구. Wi-Fi → picoclaw/cloudflared 설치 → API 키 → 기기 등록 → 자동시작 |
| **SvelteKit 웹앱** (`web/`) | Cloudflare Pages. 기기 등록 API, 사용자 관리, 초대, OTA, 채팅 UI |
| **보드 바이너리** (`board-bin/`) | picoclaw (28MB) + cloudflared (34MB), RISC-V 64-bit 정적 링크 |
| **맥 유틸** (`bin/`) | get-ssid + sshpass, macOS universal binary. 셋업 시에만 사용 |

## Cloudflare 스택

- **Tunnel**: 보드 → CF Edge. 포트포워딩/공인 IP 불필요
- **Access**: Google OAuth로 접근 제어
- **Pages**: SvelteKit 웹앱 호스팅
- **KV**: 기기/사용자/초대/OTA 데이터

## 셋업

### 사전 준비

- macOS (Apple Silicon 또는 Intel)
- LicheeRV Nano + USB-C 데이터 케이블
- Wi-Fi 네트워크
- [OpenRouter API 키](https://openrouter.ai/keys)

### Go CLI 빌드 & 실행

```bash
go build -o quick-claw .
./quick-claw setup
```

8단계 자동 진행:
1. 보드 감지 (USB NCM)
2. Wi-Fi 감지 + 키체인 비밀번호
3. 보드 Wi-Fi 설정
4. Wi-Fi 연결 확인
5. picoclaw + cloudflared 보드 설치
6. OpenRouter API 키 + picoclaw 설정
7. 기기 등록 + 터널 시작
8. Gateway 자동시작 + 스왑

### 웹앱 (Cloudflare Pages)

```bash
cd web
npm install
npm run dev          # 로컬 개발
npm run build        # 프로덕션 빌드
```

환경변수 (`.dev.vars`):
```
CF_API_TOKEN=...
CF_ACCOUNT_ID=...
CF_ZONE_ID=...
CF_DOMAIN=dawn.kim
REGISTER_SECRET=...
```

### API 라우트

| 메서드 | 경로 | 설명 |
|--------|------|------|
| POST | `/api/register` | 기기 등록 (터널+DNS+Access 생성) |
| POST | `/api/invite` | 초대 토큰 생성 |
| GET/POST | `/api/users` | 사용자 목록/추가 |
| DELETE | `/api/users/[email]` | 사용자 삭제 |
| GET | `/api/ota/check` | 버전 확인 |
| POST | `/api/ota/report` | 업데이트 결과 보고 |

## 보드 부팅 순서

전원 인가 후 자동 실행:

1. Wi-Fi 자동 연결 (`wpa_supplicant`)
2. `S98cloudflared start` → Cloudflare 터널 연결
3. `S99picoclaw start` → AI Gateway 시작 (port 18790)

## 디렉토리 구조

```
quick-claw/
├── main.go              Go CLI (초기 셋업)
├── go.mod / go.sum
├── bin/                 macOS 유틸 (get-ssid, sshpass)
├── board-bin/           RISC-V 바이너리 (picoclaw, cloudflared) [gitignored]
├── web/                 SvelteKit + Cloudflare Pages
│   ├── src/lib/
│   │   ├── cloudflare.ts   CF API (터널/DNS/Access)
│   │   ├── kv.ts           KV 헬퍼
│   │   └── types.ts        타입 정의
│   └── src/routes/api/     API 라우트
├── licheeRV-nano-setup.md  보드 수동 셋업 가이드 (한국어)
└── .env.example
```

## 비용

- 도메인: ~$10/년
- Cloudflare (Tunnel + Access + Pages + KV): 무료 tier
- OpenRouter API: 종량제 (모델에 따라 다름, 무료 모델 사용 가능)

## 라이선스

TBD
