# Blueclaw E2E 게이트

Blueclaw 에이전트의 사용자-가시 행동을 보증하는 e2e 시나리오의 단일 관리
문서. 시나리오를 추가/변경/삭제하면 이 문서를 같은 커밋에서 갱신한다.

## 게이트 계층

| 계층 | 실행 | 정의 위치 | 보증 범위 |
|---|---|---|---|
| 가상 세션 | `./internkim dev simulate --scenario <name>` | `.dependency/blueclaw/internal/e2e/scenarios.go` (등록: `virtual_session.go`) | 에이전트 루프·툴 선택·스킬 선택·이벤트 (스크립트된 LLM, 결정적) |
| Linux 실행 게이트 | `./internkim dev replay --target container --scenario <name>` | 동일 시나리오를 apple/container Linux VM에서 재생 | 실행 파일·POSIX 권한·terminal.run 경로 |
| 로컬 플릿 | `./internkim dev fleet run --recipe predeploy-gate` | `internal/localfleet/plans.go` | admind API·Mattermost 연결·browser 스모크 (인프라 준비성만; 시나리오 단언 없음) |
| 플릿 시나리오 | `./internkim dev fleet run --scenario <name>` | `internal/localfleet/service.go` + `.dependency/blueclaw/lab/scripts/scenario-*.sh` | 실제 커넥터 경유 메시징 |

가상 세션 시나리오는 `internal/agenttest/scripted_language_model.go`의
스크립트 응답으로 결정성을 확보한다. CLI의 `--cassette`/`--record-cassette`
플래그는 현재 blueclaw-lab 쪽에 배선되어 있지 않은 스텁이다(아래 백로그).

## 가상 세션 시나리오 인벤토리

| 시나리오 | 별칭 | 보증하는 행동 |
|---|---|---|
| `slides_local_multiturn_success` | `slides` | simple-slides 스킬 선택, Marp 빌드, 산출물 첨부, DESIGN.md 작성 |
| `memory_guided_followup` | `memory` | 1턴 기억 저장 → 2턴 회상해 답변 반영 |
| `tool_permission_hides_skill` | — | 허용 툴 없는 스킬 억제 + 거짓 거절 없음 |
| `gws_disabled` | — | 비허용 GWS 툴 차단과 거부 설명 |
| `schedule_create_acceptance` | — | scheduled-task 스킬로 interval `schedule.create` |
| `site_prototype_acceptance` | `site` | `site.app.create` → `terminal.run` → `site.app.publish`, URL 회신 |
| `ask_choice_reply_acceptance` | — | `ask.choice` 발행과 다음 턴 선택 해석 |
| `ask_confirm_reply_acceptance` | — | `ask.confirm`(external_send) 발행과 승인 후 계속 |
| `attachment_material_read` | — | 컨텍스트 첨부를 `image.read`로 읽기 |
| `attachment_html_preview_recovery` | — | 현재 메시지 HTML 첨부 `file.preview` |
| `attachment_html_previous_preview_recovery` | — | 이전 메시지 첨부 경로 복구 |
| `attachment_current_image_input` | — | 현재 이미지 첨부의 직접 이미지 파트 주입 |

플릿 시나리오: `mattermost-bot-invited`, `web-backed-ui`, `regression-proof`.

## 요구 커버리지 매트릭스

목표: 아래 항목 전부 COVERED. 상태가 바뀌면 이 표를 갱신한다.

| # | 요구 행동 | 상태 | 근거 / 부족분 |
|---|---|---|---|
| 1 | 웹사이트 생성+배포 | COVERED | `site_prototype_acceptance` |
| 2 | 배포된 웹사이트 수정 | MISSING | 기존 사이트 `file.edit`→재배포 시나리오 없음 |
| 3 | DM 보내기 (confirm + 상대 수신 확인) | MISSING | confirm 핸드셰이크만 단독 커버; DM 송신+수신 검증 없음 |
| 4 | 채널 포스트 작성 | MISSING | — |
| 5 | 포스트 수정 | MISSING | — |
| 6 | 반복 예약 생성/수정/삭제 | PARTIAL | 생성만 (`schedule_create_acceptance`); 수정·삭제 없음 |
| 7 | 일회성 예약 생성/수정/삭제 | MISSING | interval만 커버, `runAt` 일회성 없음 |
| 8 | 일정/업무 생성/수정/삭제 | MISSING | calendar 툴 시나리오 없음 |
| 9 | 기억 추가 | PARTIAL | `memory_guided_followup` 1턴 (저장 경로 단언은 간접) |
| 10 | 기억해내기 | PARTIAL | `memory_guided_followup` 2턴 (`memory.search` 호출 단언 없음) |
| 11 | 스킬 생성/삭제 | MISSING | skill management 툴 시나리오 없음 |
| 12 | 검색 | MISSING | web search 시나리오 없음 |
| 13 | 일반질문 | MISSING | 툴 없이 직접 회신하는 시나리오 없음 |
| 14 | introspection: 뭘 할 수 있어? | MISSING | 능력 질문 회신 시나리오 없음 |
| 15 | introspection: 아까 뭐 했어? | MISSING | 과거 태스크 조회 시나리오 없음 |
| 16 | introspection: 왜 실패했어? | MISSING | 실패 원인 회신 시나리오 없음 |

## 운영 규칙

- 에이전트 행동(루프·프롬프트·스킬·정책·툴)을 바꾸는 변경은 관련 시나리오를
  먼저 통과시킨 뒤 배포한다 (CLAUDE.md Agent Development Flow).
- 새 사용자-가시 기능에는 시나리오를 같은 변경에서 추가하고 위 매트릭스에
  행을 더한다.
- 시나리오는 사용자-가시 결과(회신 내용·이벤트·산출물)를 단언한다. 내부 구현
  세부는 단위 테스트에 둔다.
- 실제 플랫폼 스모크는 가상 세션·컨테이너 게이트 통과 후에만, 테스트 정리 규칙과
  함께 수행한다.

## 백로그

- 매트릭스 MISSING/PARTIAL 행 채우기 (2→16).
- `--cassette`/`--record-cassette`를 blueclaw-lab `virtual-session`에 실제
  배선하거나 CLI에서 플래그를 제거해 문서·현실 불일치 해소.
- predeploy-gate에 가상 세션 시나리오 묶음 실행 추가 검토 (현재는 인프라
  준비성만 검증).
