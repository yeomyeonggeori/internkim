# 에이전트 비교 조사 Handoff — 인턴킴(김인턴/Blueclaw) vs Codex · OpenCode · Hermes

- 작성일: 2026-07-08
- 목적: OpenAI Codex, OpenCode(Anomaly), Hermes Agent(Nous Research)와 비교해 인턴킴이 개선해야 할 부분을 도출하되, **인턴킴의 장점을 희석하거나 희생하지 않고 오히려 강화하는 방향**으로 정리한다.
- 조사 방법: 인턴킴/Blueclaw 코드베이스 심층 탐색(에이전트 루프, capabilityd, 커넥터, 테스트/배포 인프라) + 3개 비교 대상에 대한 2026년 중반 기준 웹 리서치. 출처는 부록 참조.
- 주의: 비교 대상의 수치(스타 수, 벤치마크, 사용량 순위)는 2026-07-08 기준 스냅샷이며 빠르게 변한다. 방향성 판단 근거로만 사용할 것.

---

## 1. 비교 대상 한 줄 요약

| 시스템 | 정체 | 핵심 포지션 |
|---|---|---|
| **Codex** (OpenAI) | Rust CLI + 클라우드 샌드박스 + 데스크톱 앱 + IDE + 모바일 | 개인/기업 개발자용 코딩 에이전트. 자율성·클라우드 위임·엔터프라이즈 관리가 강점. GPT 계열 전용(모델 락인) |
| **OpenCode** (Anomaly, 구 SST) | MIT 오픈소스 TS 기반 client/server 코딩 에이전트 (~183K 스타) | 프로바이더 자유(75+), LSP 피드백 루프, 강력한 플러그인/훅. OS 샌드박스 없음, 권한 기본값 관대 |
| **Hermes Agent** (Nous Research) | MIT 오픈소스 Python 자가호스팅 개인 비서 에이전트 (~211K 스타) | 자기개선 루프(스킬 자동 생성/큐레이션), 18+ 챗 플랫폼 게이트웨이, 크론 자동화. 개인용 — 엔터프라이즈 RBAC 없음 |

인턴킴과의 근본적 차이: **셋 다 "개발자(또는 개인)의 도구"이고, 인턴킴은 "회사 조직에 들어가는 직원형 어플라이언스"다.** 이 포지션 차이가 아래의 강점·개선점 판단의 기준선이다.

## 2. 인턴킴 현재 위치 (조사 스냅샷)

- **어플라이언스 모델**: Jetson(8GB) 하드웨어에 Firecracker microVM 게스트로 Blueclaw 구동. 데몬은 시크릿을 전혀 들지 않고(`capabilityLLM` 단일 프로바이더), 모든 LLM/플랫폼/시크릿 작업은 Unix socket/vsock 너머 capabilityd가 소유. Cloudflare Tunnel + OAuth 관리 게이트.
- **에이전트 루프**: `AgentKernel` → intake 라우팅 → 확인(confirm) 게이트 → `AgentTurnRunner` 반복 루프. 액션은 네이티브 tool-calling이 아닌 **strict JSON-Schema 구조화 출력**(`blueclaw_agent_turn_action`, oneOf 액션 변형). 노출 도구는 그룹 캡(`maxSchemaCallableToolCount = 15`)으로 제한. effort 프로파일(quick/standard/deep/extended)별 반복/시간/도구 예산과 budget escalation, stall 감지.
- **완료 증거 게이트**: `OutcomeContract` + `completion_gate` — `file.deliver` 등 요구 증거 도구의 성공 관측이 없으면 finish를 거부. "말로 끝났다"가 불가능한 구조.
- **정책/보안**: policy.json의 사람/서클/채널/리소스 ACL을 POSIX 사용자·그룹(`bc_person_*`, `bc_circle_*`)으로 투영. 모든 FS/exec은 setuid `blueclaw-posix-helper`를 통해 요청자 UID로 강등 실행. 워크스페이스 가상 경로 경계.
- **메모리**: Graphiti temporal knowledge graph 사이드카(Kuzu) + Postgres 미러 + ACL/보안등급 필터링 검색.
- **커넥터**: Mattermost(주력, WebSocket+폴링 폴백+ephemeral 플러그인), Slack Socket Mode, Signal JSON-RPC. durable outbox·중복 제거·재시도까지 있는 **배달 보증형** 설계.
- **테스트/배포**: 결정론적 virtual-session 시나리오(26개+) + LLM 카세트 녹화/재생, Apple Container VM 로컬 플릿, `verify-regression`(base에서 실패→현재에서 통과 증명), HMAC 서명 OTA 릴리스.
- **스킬**: `SKILL.md` 프론트매터 번들 21개, 임베딩+BM25 하이브리드 검색 선택, 크기 예산(8/12/15KB) 강제, 아티팩트 품질 게이트(렌더링 후 LLM 심사).
- **Companion**: 사용자 로컬 신뢰 런타임(Tauri). Ed25519 서명 브로커 잡 큐, 브라우저 핸드오프, 파일 마운트, 로컬 LLM.

## 3. 기능 비교 매트릭스

| 축 | 인턴킴/Blueclaw | Codex | OpenCode | Hermes Agent |
|---|---|---|---|---|
| 대상 | 회사 조직(비개발자 포함) | 개발자/엔터프라이즈 | 개발자 | 개인/소규모 팀 |
| 배포 | 온프레미스 어플라이언스(Jetson/PoC 컨테이너) | 로컬 CLI + OpenAI 클라우드 | 자가 호스팅(서버/TUI/웹/데스크톱) | 자가 호스팅(VPS/로컬/Docker/Modal 등 6종) |
| 모델 | capabilityd 라우팅: 로컬 llama.cpp(gemma-4-E2B)+OpenRouter, 3티어 | GPT 전용(락인), 자동 라우팅 | 75+ 프로바이더, 로컬 모델 | 40+ 프로바이더, 로컬 1급 지원 |
| 도구 호출 | strict JSON-Schema 구조화 출력(+native tool-call 시도는 백엔드에서) | 네이티브 | 네이티브 | 네이티브 |
| 도구 노출 | 고정 커널 팔레트, 캡 15 | MCP tool search(지연 로딩) | 전체+권한 필터 | 60+ 내장, MCP 필터 |
| 컨텍스트 관리 | **바이트 트림만**(토큰 기반 컴팩션 없음) | 2단계 자동 컴팩션(세션 메모리→압축 API) | prune-first→요약 폴백 | FTS5 요약 기반 |
| 서브에이전트/병렬 | **없음**(단일 루프) | 1급(TOML 정의, 최대 6스레드) | 1급(task tool, @mention) | v0.18 백그라운드 위임 |
| 장기 목표/재개 | task ledger·스케줄은 있으나 multi-day goal 워크플로 없음 | `/goal` 영속 워크플로, fork/resume | 세션 undo/redo/스냅샷 | `/goal` 완료 계약, 체크포인트 |
| 메모리 | Graphiti 시간 그래프 + ACL 필터(**최상급**) | Memories 프리뷰(로컬 파일) | AGENTS.md 수준 | FTS5+Curator+사용자 모델링 |
| 자기개선 | SOUL.md(수동 유지) | 없음 | 없음 | **스킬 자동 생성/개선/큐레이션 루프** |
| 스킬 표준 | 자체 SKILL.md(크기 게이트, 완료 증거 연동) | Agent Skills + 플러그인 마켓(90+) | Claude 호환 스킬 + 플러그인 | agentskills.io 표준 + Skills Hub |
| MCP | 설정 선언형(정적) | 클라이언트+서버, tool search | 로컬/원격 + code-mode | 카탈로그+필터 |
| 샌드박스 | **Firecracker VM + POSIX 최소권한**(최강) | Seatbelt/Landlock, 네트워크 기본 차단 | 없음(권한 레이어만) | 패턴 매칭 승인 + Docker 옵션 |
| 멀티유저 | **정책 기반 RBAC + OS 강제**(유일) | 워크스페이스 관리(SCIM/RBAC)이나 세션은 1인 | 없음 | 페어링 코드 allowlist |
| 챗 플랫폼 | Mattermost/Slack/Signal(배달 보증) | Slack/Linear(작업 위임형) | GitHub/GitLab | **18+ 플랫폼** |
| 완료 검증 | **OutcomeContract+증거 게이트**(구조 강제) | 리뷰어 에이전트(승인 심사) | LSP 진단 피드백 | 완료 계약(자기평가 편향 지적됨) |
| 회귀 테스트 인프라 | **카세트+virtual session+verify-regression**(최상급) | 내부(비공개) | 일반 CI | 일반 CI |
| 스트리밍 | 없음 | 있음 | 있음 | 있음 |
| 릴리스 롤백 | 501(미구현) | 해당 없음 | 해당 없음 | 체크포인트 |

## 4. 인턴킴의 차별적 강점 — 유지하고 더 강화할 것

아래 6개는 세 비교 대상 **누구도 갖지 못한** 자산이다. 어떤 개선 작업도 이들을 훼손하는 방향이면 중단하고 재설계한다.

### S1. 조직 단위 최소권한 실행 경계 (POSIX 투영)
사람→`bc_person_*`, 서클→`bc_circle_*`로 정책을 OS 계정에 투영하고 setuid 헬퍼로 모든 실행을 요청자 권한으로 강등하는 구조는 세 시스템 모두에 없다. Codex의 샌드박스는 "프로세스 격리"이지 "조직 내 누가 무엇을 볼 수 있나"가 아니고, Hermes의 멀티유저는 allowlist 수준이다.
**강화 방향**: 이 경계를 마케팅 가능한 1급 기능으로 문서화·감사(audit) 리포트화. 새 도구/서브에이전트/MCP가 추가될 때마다 이 경계를 자동 상속하는 테스트 게이트(예: "모든 신규 도구는 WorkspaceActor 경유" lint)를 추가.

### S2. 완료 증거 게이트 (OutcomeContract / CompletionGate)
"성공 관측 없이는 완료 선언 불가"를 런타임이 강제하는 시스템은 인턴킴뿐이다. Hermes는 완료 계약을 넣었지만 자기평가 편향이 커뮤니티에서 지적됐고, Codex는 사후 리뷰어에 의존한다.
**강화 방향**: 증거 종류를 확장(사이트 배포 스크린샷, 플랫폼 전송 영수증, 캘린더 반영 확인 등 도메인별 증거 타입)하고, 증거를 사용자에게 보여주는 UX("이 파일이 실제 결과입니다")로 연결.

### S3. 시크릿 제로 데몬 + capability 프로토콜 경계
에이전트 프로세스가 토큰을 아예 모르는 구조(descriptor의 PrivacyClass/SideEffectClass/RequiresApproval 포함)는 프롬프트 인젝션·유출 시대에 갈수록 가치가 커진다. Codex조차 클라우드 시크릿을 "셋업 단계에만 존재"시키는 수준이다.
**강화 방향**: MCP/신규 도구 확장 시에도 반드시 capability descriptor로 감싸 side-effect 분류·승인 요구를 통과시키는 것을 불변 규칙으로 유지(§6 참조).

### S4. 결정론적 회귀 증명 인프라
LLM 카세트 녹화/재생 + virtual session + `verify-regression`(base에서 실패를 먼저 증명)은 세 시스템의 공개 인프라 어디에도 없는 수준이다. 이것이 낮은 릴리스 빈도를 품질로 보상하는 인턴킴의 무기다.
**강화 방향**: 시나리오 커버리지를 신규 기능의 머지 조건으로 유지·확대(현재 PENDING인 `site_lifecycle_acceptance` 등록 포함). 개선 항목(§5)마다 대응 시나리오를 먼저 추가.

### S5. Graphiti 시간 그래프 메모리 + ACL 필터 검색
비교 대상의 메모리는 파일/FTS 요약 수준. 인턴킴은 시간적 지식 그래프에 보안등급·클래스 필터까지 걸린다. 조직 메모리("누가 언제 무엇을 결정했나")는 회사형 에이전트의 해자다.
**강화 방향**: 유일한 코드 TODO인 휘발성 메모리 업데이트 큐(`memory/update_queue.go:87`)를 durable 큐로 교체해 재시작 유실을 없애고, 메모리 회상을 사용자에게 근거로 제시하는 기능(출처 표시)으로 확장.

### S6. 배달 보증형 커넥터 + LLM-First 응답 정책
durable outbox·중복 제거·승인 버튼 왕복까지 갖춘 커넥터 런타임과 "실패 설명도 LLM이 작성"하는 정책의 결합은 비개발자 사용자를 상대하는 제품에 결정적이다. Hermes가 플랫폼 수는 많지만 배달 보증·조직 정책 게이트가 없다.
**강화 방향**: 플랫폼 추가보다 **기존 플랫폼의 완성도**(Signal의 히스토리/첨부 stub 해소)를 먼저. 새 플랫폼은 커넥터 계약(정규화 이벤트 + outbox) 재사용으로만 추가.

## 5. 개선 필요 영역 (우선순위순)

각 항목에 격차(왜 문제), 참고 사례, 제안, 그리고 **강점 보호 가드레일**(무엇을 희생하면 안 되는가)을 명시한다.

### P0-1. 토큰 인지 컨텍스트 컴팩션
- **격차**: 현재 트림은 바이트 기반(`ToolResultMaxBytes=32768`, progress ledger 바이트 컷)뿐이고 `contextWindowTokens`는 장식이다. 긴 태스크·deep/extended 프로파일에서 컨텍스트 초과 또는 중요 관측 유실이 구조적으로 발생할 수 있다.
- **참고**: Codex는 세션 메모리 치환→압축 API 2단계, OpenCode는 "prune-first(최신 40K 토큰 보존, 20K 이상 확보될 때만) → 마지막 수단 요약, soft-delete" 전략. OpenCode의 prune-first가 인턴킴 구조에 더 맞다.
- **제안**: (1) 관측 레저에 토큰 추정치를 붙이고, (2) 오래된 tool 관측부터 soft-prune(이벤트 레저에는 남기고 프롬프트에서만 제외), (3) 임계 초과 시에만 LLM 요약으로 접기. 요약은 태스크 이벤트(`agent.context_compacted`)로 기록.
- **가드레일**: 이벤트 레저의 완전성(감사·재현성)은 유지 — 컴팩션은 프롬프트 조립층에서만 일어나고 저장층은 건드리지 않는다. 카세트 재현성이 깨지지 않도록 컴팩션 트리거를 결정론적으로(토큰 수 기준) 설계.

### P0-2. 문서·코드 드리프트 즉시 정리 (quick win) — 2026-07-08 처리됨
처리 중 발견한 근본 원인: 최초 조사 시점의 `.dependency/blueclaw` 체크아웃이 superproject가
기록한 커밋(e52c4ec7)보다 뒤처진 상태였다(서브모듈 remote가 SSH URL이라 fetch 실패 →
stale 체크아웃). 아래 1·5번의 "코드" 쪽 근거는 stale 체크아웃에서 관찰된 값이었다.
서브모듈을 gh HTTPS 자격증명으로 fetch해 e52c4ec7로 정렬한 뒤 재검증했다.
1. 도구 캡: 실제 코드는 `maxSchemaCallableToolCount = 15` — README의 "capped at 20"을 15로 수정. **완료**
2. `E2E.md`의 "카세트 플래그는 미배선 스텁" 서술 — 실제로는 `blueclaw-lab/main.go`에 완전 배선됨. 본문·백로그 갱신. **완료**
3. 테스트 모델 핀 불일치: 현행 `INTERNKIM_TEST_MODEL=xiaomi/mimo-v2.5` — `blueclaw-crud-render-gate-handoff.md`에 정정 노트 추가(본문 로그는 보존). **완료**
4. 정책/런타임 예제 파일 형식: 로더는 JSON인데 예제는 `.yaml` 확장자(내용은 JSON) — `config/{policy,runtime,secret.enc}.example.yaml`을 `.json`으로 리네임하고 참조 5곳(cmd 기본값·테스트) 수정, `go build`+`go test ./tests/integration` 통과. **완료**
5. `file.deliver` 혼재 — stale 체크아웃 문제였다. e52c4ec7 기준 Blueclaw 코드도 `file.deliver`를 사용하며 문서·스킬·config와 일치. 서브모듈 정렬로 해소. **완료**
6. 모델명 하드코딩 산재: `DefaultActionModelName`, fallback `z-ai/glm-5.2`, `BlueclawHighModelName`, Graphiti 모델, 임베딩 모델이 서로 다른 파일에 핀 — 단일 카탈로그 파일로 수렴 권장. **후속 과제**
7. LiteRT 레거시 경로(`litert.go`, `cmd/internkim-local-llm-runner`, E4B 모델 URL) 정리 또는 명시적 legacy 마킹. **후속 과제**
재발 방지: 서브모듈 remote를 HTTPS(`https://github.com/Dawn-kim-official/blueclaw.git`)로
바꾸거나 `.gitmodules` URL 전환을 검토할 것 — SSH 키 없는 환경에서 조용히 stale 상태가 된다.

### P1-1. 범위 있는 서브에이전트 (병렬 위임)
- **격차**: 단일 루프라 "리서치하며 문서 만들기", "여러 파일 동시 분석" 같은 fan-out이 직렬화된다. Codex(6스레드 서브에이전트)·OpenCode(task tool)·Hermes(백그라운드 위임) 모두 1급 지원.
- **제안**: 전면 멀티에이전트가 아니라 **read-only 탐색 서브태스크**부터: 부모 태스크가 동일 requester 신원·정책으로 자식 태스크 런을 생성하고, 자식은 조회성 도구만 노출받으며 결과를 부모 관측으로 반환. 기존 task run/step/event 모델을 그대로 재사용(새 인프라 불필요).
- **가드레일**: 자식도 부모와 동일한 POSIX actor·ACL·completion evidence 규칙을 상속. 도구 캡 15와 고정 커널 팔레트 철학 유지 — 자식별로 팔레트를 좁히는 방향이지 넓히는 방향이 아님. 승인 게이트는 부모에서만(자식이 사용자에게 직접 묻지 않음).

### P1-2. 장기 목표(goal) 워크플로와 재개
- **격차**: 스케줄·태스크 레저는 강하지만, "며칠짜리 목표를 걸어두고 진행률을 보고받으며 이어가기"(Codex `/goal`+automations, Hermes `/goal` 완료 계약)에 해당하는 사용자 개념이 없다.
- **제안**: 기존 `ActiveGoal` + `schedule.*` + OutcomeContract를 묶어 "목표 = 증거 계약이 걸린 장기 태스크 체인"으로 노출. 스케줄러가 미완 목표를 재개하고, 진행 보고는 LLM-first 정책대로 생성. 이것은 S2(완료 증거)의 확장이지 신기능이 아니다 — **경쟁사는 자기평가에 의존하지만 인턴킴은 증거 기반 goal을 만들 수 있다**는 차별화 지점.
- **가드레일**: 목표 자동 재개가 승인 게이트를 우회하지 않게(external_send/destructive는 매 회 승인 정책 적용).

### P1-3. 스킬 학습 루프 (Hermes의 강점을 인턴킴 방식으로)
- **격차**: Hermes의 "작업 후 스킬 자동 생성 + 7일 주기 큐레이션"은 사용할수록 좋아지는 체감을 만든다. 인턴킴은 `skill.add`/`skill.search`와 SOUL.md가 있으나 루프가 수동이다.
- **제안**: 완료된 태스크의 이벤트 레저에서 반복 패턴을 감지해 **초안 스킬을 생성하되 사람(관리자) 승인 후 활성화**하는 반자동 루프. 스킬 크기 예산(8/12/15KB)과 SKILL.md 검사 가능성을 그대로 적용. Hermes에서 문제가 된 "자동 생성 스킬이 수동 커스터마이징을 덮어쓰는" 사고는 승인 게이트+버전 이력으로 원천 차단.
- **가드레일**: `disable-model-invocation`·`allowedProfiles` 같은 기존 가시성 통제를 초안 스킬에 기본 적용. 자동 활성화 금지(조직용 제품에서 에이전트가 스스로 능력을 늘리는 것은 보안 사건이다).

### P1-4. MCP 동적 활용 + 스킬 생태계 접점
- **격차**: MCP가 설정 선언형이라 카탈로그가 정적이고, 생태계(Codex 플러그인 마켓, agentskills.io, Skills Hub) 접점이 없다.
- **제안**: (1) MCP 서버의 도구 목록 동적 조회 + 도구 검색(지연 로딩, Codex 방식)으로 캡 15 안에서 필요할 때만 노출. (2) 스킬 프론트매터를 agentskills.io 표준과 상호 변환 가능하게 해 외부 스킬을 "가져와 검사 후 채택"하는 경로 마련.
- **가드레일**: 외부 MCP 도구는 반드시 capability descriptor로 래핑해 PrivacyClass/SideEffectClass/승인 요구를 부여받아야 노출된다(S3 불변). 외부 스킬은 크기 게이트+정책 검토를 통과해야 설치.

### P2-1. 진행 체감 (스트리밍 대체재)
- **격차**: `Stream: false` 전면. 다만 채팅 플랫폼 특성상 토큰 스트리밍의 가치는 CLI보다 낮다.
- **제안**: 토큰 스트리밍 대신 **단계 진행 알림의 질**을 올리는 것이 인턴킴답다: progress.start/stop을 활용한 "지금 무엇을 하는 중" 메시지 갱신(Mattermost 메시지 edit), 장기 태스크 중간 보고. Board UI의 태스크 상세와 연동.
- **가드레일**: 진행 문구도 LLM-first 정책 준수(결정론 문장 금지 예외 규정 유지).

### P2-2. 릴리스 롤백
- **격차**: OTA 롤백이 501. 어플라이언스는 원격 손대기 어려워 롤백 부재가 치명적일 수 있다.
- **제안**: 릴리스 SHA 기반 배포가 이미 멱등이므로, 직전 성공 manifest를 디바이스에 보관하고 "이전 릴리스 재적용"으로 구현(새 경로 불필요). health check 실패 시 자동 롤백은 그다음.

### P2-3. 로컬라이제이션·기타 견고성
- 하드코딩 `Asia/Seoul`(`prompt_assembler.go`) → 정책/디바이스 설정으로.
- Signal 커넥터 히스토리/신원/첨부 stub 해소(§S6).
- Companion 잡 큐의 250ms busy-spin long-poll → 이벤트 기반(단일 호스트에서는 낮은 우선순위).
- placeholder OpenRouter 키 가드 중복 5곳 → 공용 헬퍼로.

### 명시적 비목표 (경쟁사를 따라가지 않을 것)
- **프로바이더 무한 확장(OpenCode식)**: in-daemon 단일 `capabilityLLM`은 약점이 아니라 시크릿 제로 설계의 결과다. 유연성은 capabilityd 라우팅 층에서만 늘린다.
- **개발자용 IDE/코딩 UX 경쟁**: 인턴킴의 사용자는 Mattermost 안의 직원이다. TUI/IDE 확장은 포지션 밖.
- **릴리스 속도 경쟁**(주 수십 회): 회귀 증명 게이트(S4)를 지키는 속도가 상한. 대신 OTA 신뢰성(롤백)으로 보상.
- **네이티브 tool-calling으로 전면 전환**: strict 구조화 출력 프로토콜은 저사양 로컬 모델(gemma-4-E2B)에서의 신뢰성을 위한 선택. 백엔드가 이미 native→json_schema→prompted 폴백을 수행하므로 유지.

## 6. 강점 보호 원칙 (개선 작업 공통 게이트)

1. **모든 신규 실행 경로는 WorkspaceActor/POSIX 경계를 경유한다.** 우회 경로(직접 os.* 호출, 별도 프로세스)가 생기면 그 개선은 반려.
2. **모든 신규 도구·MCP·스킬은 capability descriptor의 side-effect 분류와 승인 정책을 받는다.**
3. **완료 선언은 언제나 관측 증거를 요구한다.** 서브에이전트·goal·자동화가 늘어나도 예외 없음.
4. **프롬프트 조립층 최적화(컴팩션 등)는 저장층(이벤트 레저)의 완전성을 건드리지 않는다.**
5. **사용자 문장은 LLM이 작성한다**(stop/stop-all 류 제어 응답만 예외).
6. **머지 전 결정론 시나리오 우선**: 개선 항목마다 virtual-session 시나리오(또는 fleet 시나리오)를 먼저 추가하고 `verify-regression`으로 증명한다.

## 7. 제안 로드맵

| 단계 | 항목 | 근거 |
|---|---|---|
| 즉시(1주) | P0-2 드리프트 청소, 메모리 durable 큐(S5), 모델 핀 카탈로그화 | 위험 낮고 온보딩·신뢰 개선 |
| 단기(1개월) | P0-1 토큰 컴팩션, P2-2 롤백, Signal stub 해소 | 장기 태스크 안정성 + 어플라이언스 운영 안전망 |
| 중기(분기) | P1-1 read-only 서브에이전트, P1-2 증거 기반 goal, P2-1 진행 체감 | 체감 자율성 격차 해소, S2 강화와 동일 축 |
| 장기 | P1-3 승인형 스킬 학습 루프, P1-4 MCP 동적화/생태계 접점 | 사용할수록 좋아지는 조직 에이전트 — Hermes의 루프를 조직 통제 하에 이식 |

## 8. 한 문장 결론

인턴킴은 "조직에 고용된, 권한이 통제되고, 결과를 증거로 증명하는 직원형 에이전트"라는 세 경쟁자 누구도 점유하지 못한 자리에 있다. 개선의 핵심은 이 자리를 벗어나는 기능 추가가 아니라, **컨텍스트 수명(컴팩션)·병렬 위임·장기 목표·학습 루프**라는 네 가지 체감 격차를 기존의 증거 게이트·POSIX 경계·결정론 테스트 위에 얹는 것이다.

---

## 부록 A. 비교 대상 상세 근거 (요약)

### Codex (OpenAI)
- 5개 표면(CLI/앱/클라우드/IDE/모바일), Rust CLI 오픈소스, GPT-5.5 기본. 서브에이전트 TOML 정의(기본 6스레드), `/goal` 영속 워크플로, 2단계 컴팩션, 세션 fork/resume/archive.
- Seatbelt/Landlock OS 샌드박스, 네트워크 기본 차단+도메인 allowlist, 승인 에스컬레이션을 심사하는 리뷰어 에이전트, 클라우드 시크릿은 셋업 단계에만 존재.
- 플러그인 마켓(90+), Agent Skills(SKILL.md 호환), AGENTS.md 표준 원조, MCP tool search 기본화.
- 약점: 모델 락인, scope creep(과잉 수정), 프론트엔드 약세, 2026-04 토큰 크레딧 전환 후 가격 불투명 논란.
- 출처: developers.openai.com/codex/{models,subagents,skills,plugins,agent-approvals-security,memories,changelog}, github.com/openai/codex

### OpenCode (Anomaly)
- client/server(HTTP+OpenAPI), TUI/웹/데스크톱/IDE, 75+ 프로바이더, Models.dev. prune-first 컴팩션, LSP 진단 피드백 루프(정확성 차별화 요인), 플러그인 훅(tool.execute.before 등), Claude 호환 스킬, 커스텀 도구(TS+Zod).
- 권한: allow/ask/deny 글롭 규칙(`doom_loop` 감지 포함)이나 기본값 관대, **OS 샌드박스 없음**, 서버 무인증 기본. CVE-2026-22812(무인증 RCE) 이력.
- Zen(마진 제로 게이트웨이)·Go($10/월 오픈웨이트) 수익 모델.
- 출처: opencode.ai/docs/{agents,permissions,plugins,skills,custom-tools,server,zen,go}, github.com/anomalyco/opencode

### Hermes Agent (Nous Research)
- Python 자가호스팅, 2026-02 공개 후 ~211K 스타. 자기개선 루프: 태스크 후 스킬 자동 생성, Curator가 7일 주기 채점/통합/정리, Honcho 사용자 모델링, FTS5 회상. `/goal`(증거 기반 완료 계약), `/learn`(워크플로→스킬), 크론 Automation Blueprints, 백그라운드 서브에이전트.
- 18+ 챗 플랫폼 단일 게이트웨이, 페어링 코드 방식 멀티유저(엔터프라이즈 RBAC 아님), 승인 모드+Docker 격리 옵션.
- 약점: 자기평가 편향(성공 과대평가), 자동 스킬이 수동 설정을 덮어쓰는 사고, 출처(attribution) 논란, 오픈 이슈 ~26.7K.
- 출처: github.com/NousResearch/hermes-agent, hermes-agent.nousresearch.com/docs

## 부록 B. 인턴킴 조사에서 나온 코드 참조 (개선 항목 매핑)

| 항목 | 파일 |
|---|---|
| 도구 캡 15 | `.dependency/blueclaw/internal/agent/tool_exposure.go:8` |
| 바이트 트림 컨텍스트 | `.dependency/blueclaw/internal/agent/prompt_assembler.go` (`buildObservationContext`) |
| 휘발성 메모리 큐(유일 TODO) | `.dependency/blueclaw/internal/memory/update_queue.go:87` |
| 단일 프로바이더 팩토리 | `.dependency/blueclaw/internal/llm/provider_factory.go` |
| 하드코딩 타임존 | `.dependency/blueclaw/internal/agent/prompt_assembler.go` (`temporalContextLocation`) |
| 완료 증거 게이트 | `.dependency/blueclaw/internal/agent/completion_gate.go`, `outcome_contract.go` |
| POSIX 투영 | `.dependency/blueclaw/internal/security/posix_identity.go`, `cmd/blueclaw-posix-helper/` |
| 롤백 501 | `internal/admind/release_updates.go` (`rollbackReleaseUpdate`) |
| Signal stub | `internal/capabilityd/signal_jsonrpc.go` (`signalHistoryFromRequest`) |
| 모델 핀 산재 | `internal/llmbackend/openrouter.go`, `internal/runtime/blueclaw/blueclaw_contract.go`, `internal/capabilityd/service.go` |
| 카세트 배선(문서와 달리 완성) | `.dependency/blueclaw/cmd/blueclaw-lab/main.go`, `internal/e2e/language_model_cassette.go` |
