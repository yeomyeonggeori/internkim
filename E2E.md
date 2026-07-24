# Blueclaw E2E 게이트

Blueclaw 에이전트의 사용자-가시 행동을 보증하는 e2e 시나리오의 단일 관리
문서. 시나리오를 추가/변경/삭제하면 이 문서를 같은 커밋에서 갱신한다.

## 게이트 계층

| 계층 | 실행 | 정의 위치 | 보증 범위 |
|---|---|---|---|
| 가상 세션 | `./internkim dev simulate --scenario <name>` | `.dependency/blueclaw/internal/e2e/scenarios.go` (등록: `virtual_session.go`) | 스크립트 응답에 대한 에이전트 루프·상태 전이·이벤트 invariant |
| Mattermost 제외 Linux | `./internkim dev fleet run --without-mattermost --scenario <name>` | Blueclaw virtual-session을 Linux VM 내부에서 실행 | Mattermost 서버 없이 Linux toolchain·agent 경로 검증 |
| 로컬 플릿 | `./internkim dev fleet run` | `internal/localfleet/plans.go` | 일회용 Linux+Mattermost+Kim 서버에서 predeploy API·Mattermost·browser 스모크 |
| 플릿 시나리오 | `./internkim dev fleet run --scenario <name>` | `internal/localfleet/service.go` + `.dependency/blueclaw/lab/scripts/scenario-*.sh` | 일회용 실제 커넥터 경유 메시징·재시작 후 정책 보존 |
| 재사용 플릿 | `./internkim dev fleet run --reuse --scenario <name>` | `internal/localfleet` shared VM/state/tunnel | 수동 디버깅용 공유 로컬 플릿 |

가상 세션 시나리오는 `internal/agenttest/scripted_language_model.go`의
응답에 대한 상태 전이, 승인, 취소, 부작용, 증거 연결을 결정적으로 검증한다.
실 LLM의 툴·스킬 판단 품질이나 AI SDK 경로는 보증하지 않는다. AI SDK
acceptance는 LLMD authoritative로 실행되는 `./internkim test expensive` 또는
lab runner의 `--llm-provider llmd --live-llm --strict-assertions` 조합으로
검증한다. `--live-llm`만 사용하면 실 호출을 허용할 뿐 LLMD 경로를 뜻하지 않는다.

## 가상 세션 시나리오 인벤토리

| 시나리오 | 별칭 | 보증하는 행동 |
|---|---|---|
| `slides_local_multiturn_success` | `slides` | simple-slides 스킬 선택, Marp 빌드, 산출물 첨부, DESIGN.md 작성 |
| `memory_guided_followup` | `memory` | 1턴 기억 저장 → 2턴 회상해 답변 반영 |
| `tool_permission_hides_skill` | — | 허용 툴 없는 스킬 억제 + 거짓 거절 없음 |
| `gws_disabled` | — | 비허용 GWS 툴 차단과 거부 설명 |
| `schedule_create_acceptance` | — | scheduled-task 스킬로 interval `schedule.create` |
| `site_prototype_acceptance` | `site` | `terminal.run` 스캐폴드/빌드 → `site.serve`, URL 회신 |
| `ask_choice_reply_acceptance` | — | `ask.choice` 발행과 다음 턴 선택 해석 |
| `ask_confirm_reply_acceptance` | — | `ask.confirm`(external_send) 발행과 승인 후 계속 |
| `attachment_material_read` | — | 컨텍스트 첨부를 `image.read`로 읽기 |
| `attachment_html_preview_recovery` | — | 현재 메시지 HTML 첨부 `file.preview` |
| `attachment_html_previous_preview_recovery` | — | 이전 메시지 첨부 경로 복구 |
| `attachment_current_image_input` | — | 현재 이미지 첨부의 직접 이미지 파트 주입 |
| `plain_question_acceptance` | — | 도구 없이 일반질문 직접 회신 |
| `web_search_acceptance` | — | `web.search` 1회 후 결과 기반 답변 |
| `schedule_lifecycle_acceptance` | — | 반복 예약 생성→`schedule.update` 수정→취소 |
| `one_time_schedule_acceptance` | — | `kind:"once"`+`runAt` 일회성 예약 생성 |
| `dm_send_confirm_acceptance` | — | ask.confirm 승인 후 `platform.message.send` DM 송신 |
| `channel_post_acceptance` | — | 채널 타깃 `platform.message.send` 포스트 작성 |
| `calendar_event_lifecycle_acceptance` | — | 일정 생성→시간 변경→삭제 3턴 |
| `platform_message_edit_acceptance` | — | `platform.message.update`로 기존 포스트 수정 |
| `skill_lifecycle_acceptance` | — | `skill.add` 등록 후 `skill.remove` 삭제 |
| `capability_question_acceptance` | — | 빈 쿼리 `skill.search`로 능력 질문 답변 |
| `task_history_question_acceptance` | — | `task.history`로 선행 작업 질문 답변 |
| `site_edit_redeploy_acceptance` | — | 배포된 사이트 수정→빌드→재배포 |
| `site_lifecycle_acceptance` | — | PENDING: structured `requiredEvidence` 기반 웹사이트 생성→배포→수정→재배포→삭제 승인→삭제 |
| `memory_explicit_tool_acceptance` | — | `memory.remember` 저장과 `memory.search` 회상 명시 단언 |
| `failure_explanation_acceptance` | — | 실패 태스크 사유를 `task.history`로 설명 |

플릿 시나리오: `dm-recipient-resolve`, `mattermost-bot-invited`, `restart-policy-survival`, `web-backed-ui`, `regression-proof`.

## 요구 커버리지 매트릭스

목표: 아래 항목 전부 COVERED. 상태가 바뀌면 이 표를 갱신한다.

| # | 요구 행동 | 상태 | 근거 / 부족분 |
|---|---|---|---|
| 1 | 웹사이트 생성+배포 | COVERED | `site_prototype_acceptance` |
| 2 | 배포된 웹사이트 수정 | COVERED | `site_edit_redeploy_acceptance` |
| 2a | 배포된 웹사이트 삭제 | COVERED | `site_lifecycle_acceptance` (structured `requiredEvidence:["site.unserve"]` 승인 후 삭제) |
| 3 | DM 보내기 (confirm + 상대 수신 확인) | PARTIAL | `dm_send_confirm_acceptance` (confirm 게이트→송신→messageID 관측 단언); `dm-recipient-resolve`가 실 Mattermost 계정 이메일과 Blueclaw 정책 사람 연결을 통해 수신자 해석을 단언. 실제 상대 수신 확인은 실플랫폼 스모크 영역 |
| 4 | 채널 포스트 작성 | COVERED | `channel_post_acceptance` |
| 5 | 포스트 수정 | COVERED | `platform_message_edit_acceptance` (`platform.message.update`) |
| 6 | 반복 예약 생성/수정/삭제 | COVERED | `schedule_lifecycle_acceptance` (생성→`schedule.update` 수정→취소) |
| 7 | 일회성 예약 생성/수정/삭제 | PARTIAL | 생성은 `one_time_schedule_acceptance` (`kind:"once"`+`runAt`); 수정·삭제 흐름은 6과 동일 패턴이라 미중복 |
| 8 | 일정/업무 생성/수정/삭제 | COVERED | `calendar_event_lifecycle_acceptance` (`calendar.event.add/update/delete` 3턴) |
| 9 | 기억 추가 | COVERED | `memory_explicit_tool_acceptance` (`memory.remember` 호출·입력 단언) |
| 10 | 기억해내기 | COVERED | `memory_explicit_tool_acceptance` (`memory.search` 호출·반영 단언) + `memory_guided_followup` |
| 11 | 스킬 생성/삭제 | COVERED | `skill_lifecycle_acceptance` (`skill.add`/`skill.remove`) |
| 12 | 검색 | COVERED | `web_search_acceptance` |
| 13 | 일반질문 | COVERED | `plain_question_acceptance` |
| 14 | introspection: 뭘 할 수 있어? | COVERED | `capability_question_acceptance` (빈 쿼리 `skill.search` 전체 로스터) |
| 15 | introspection: 아까 뭐 했어? | COVERED | `task_history_question_acceptance` (`task.history` 2턴) |
| 16 | introspection: 왜 실패했어? | COVERED | `failure_explanation_acceptance` (실패 태스크 후 `task.history`로 사유 설명) |
| 17 | Mattermost DM 수신자 해석 | COVERED | `dm-recipient-resolve` (실 Mattermost 사용자 생성→정책 초대→인바운드 계정 링크→부분 이름으로 `/admin/api/identity/resolve-recipient` resolved 단언) |
| 18 | Blueclaw 재시작 후 정책 사람 보존 | COVERED | `restart-policy-survival` (재시작 직전/직후 `/admin/api/policy` 사람 수 동일 단언; 서비스 재시작이 있어 기본 `dev fleet run` 게이트 제외) |

## 운영 규칙

- 에이전트 행동(루프·프롬프트·스킬·정책·툴)을 바꾸는 변경은 관련 시나리오를
  먼저 통과시킨 뒤 배포한다 (CLAUDE.md Agent Development Flow).
- 새 사용자-가시 기능에는 시나리오를 같은 변경에서 추가하고 위 매트릭스에
  행을 더한다.
- 시나리오는 사용자-가시 결과(회신 내용·이벤트·산출물)를 단언한다. 내부 구현
  세부는 단위 테스트에 둔다.
- 실제 플랫폼 스모크는 가상 세션과 Mattermost 제외 Linux 게이트 통과 후에만,
  테스트 정리 규칙과 함께 수행한다.

## 백로그

- PARTIAL 잔여분: DM 상대 실수신 확인(3)과 일회성 예약 수정·삭제(7)는 실플랫폼
  스모크/기존 패턴 중복이라 가상 세션 추가 없이 유지.
- 기존 한계: 가상 세션과 `--without-mattermost`는 실제 Mattermost ingress/API/DM
  수신을 증명하지 않는다. 공유 로컬 플릿은 고정 VM·고정 포트·공유 상태를 재사용하므로
  기본 게이트가 아니라 `--reuse` 디버깅 모드로만 사용한다.
- 기본 `dev fleet run` 게이트에 가상 세션 시나리오 묶음 실행 추가 검토.
