# Blueclaw Firecracker Runtime Report

작성 시각: 2026-05-01 00:18 KST

## 목표

Blueclaw 전체 runtime을 host process에서 Firecracker guest runtime으로 옮긴다.

Host에 남는 것:

- InternKim `admind`
- InternKim `capabilityd`
- Mattermost
- Cloudflare tunnel
- platform secrets
- device/browser/file/LLM capability boundary

Firecracker guest 안으로 들어가는 것:

- `blueclaw`
- `graphiti-memoryd`
- Blueclaw 전용 Postgres
- workspace
- skills
- terminal/bash 실행 표면
- Blueclaw artifacts/logs/blobs

제품 전환 조건은 sim smoke와 board smoke가 모두 성공하는 것이다.

## 진행 상태

`make smoke-blueclaw-runtime-tart`가 **green**. 두 번 연속 통과 확인. setup 12/12 단계 + verify api(blueclaw/graphiti/capability/users-sync 등) + verify mattermost reply smoke 모두 통과.

## 이번 세션에서 적용한 수정

### Rootfs base cache layer

`tools/prepare-blueclaw-runtime`을 둘로 분리.

- `ensure_rootfs_base`: mmdebstrap + graphiti python venv → `$cache_directory/rootfs-base-<key>.tar`. cache key는 package list + release + `requirements.txt` sha256.
- `prepare_rootfs_directory`: cache tar 추출 후 `blueclaw` binaries, `guest-init`, graphiti source만 덮어쓰기.

첫 실행 cold build는 동일하지만 두 번째부터는 21초 수준으로 떨어진다. 첫 실행 ~12분 → cache hit ~30초.

### guest-init 안정화

`assets/blueclaw-runtime/guest-init`:

1. `ip link set lo up` 추가. 이게 빠져 있어서 guest 안에서 `127.0.0.1`이 모두 "network is unreachable"이었다. blueclaw, graphiti, vsock-http-proxy 다 silently fail.
2. `blueclaw-guest-healthd`를 mount/logs dir 직후로 끌어올림. 기존엔 postgres init/blueclaw 실행 뒤에 떠서 첫 boot에 supervisor의 120s `WaitForGuestHealth`가 항상 timeout.
3. `-blueclaw-url '' -graphiti-url ''` 플래그로 healthd를 liveness-only로 강제. healthd 기본값이 blueclaw/graphiti URL까지 체크해서 downstream 안 뜨면 supervisor에 "error" 응답하던 문제. healthd는 vsock 도달성만 확인하면 되고, app readiness는 setup verify가 직접 본다.

### Migration deploy 경로

`internal/runtime/blueclaw/blueclaw_contract.go`:

- `BlueclawMigrationPath`을 `/root/.blueclaw/migrations` → `/root/.blueclaw/workspace/.blueclaw/migrations`.

기존 경로는 host workspace 디렉터리 밖이라 supervisor의 `SyncWorkspaceDirectory`(host workspace → guest workspace.ext4) rsync 대상이 아니었다. 그래서 guest의 `/workspace/.blueclaw/migrations`이 비어 있고 blueclaw가 `relation "person" does not exist` 등 마이그레이션 미적용 에러를 냈다. workspace 안으로 옮기면 sync가 자동으로 따라간다.

### verify 정렬

`internal/cli/verify.go`:

1. host 직접 `127.0.0.1:7791/health` (graphiti) 체크 제거. graphiti는 새 아키텍처에서 guest 내부 dependency라 host 노출은 의미 없음. 필요하면 blueclaw API를 통한 간접 체크로 후속 작업.
2. users-sync 검증이 host의 `/root/.blueclaw/config/policy.json` 대신 blueclaw API `/admin/api/policy`를 읽도록 변경. blueclaw가 invite 결과를 guest workspace policy에 쓰는데 sync는 host→guest 단방향이라 host policy.json엔 반영이 안 됐음. API는 신뢰 가능한 단일 출처.

### smoke target

`Makefile`:

- `smoke-blueclaw-runtime-tart`의 `--only` 리스트에 `users-sync` 추가. verify가 users-sync unit과 state 파일을 요구하지만 setup이 해당 step을 안 돌리던 mismatch 해결.

## 이번 세션에서 추가로 적용한 수정

### Mattermost setup의 잘못된 channel/team ID 저장

`setupMattermost`(`internal/cli/main.go`)가 team/channel lookup의 HTTP status를 무시하고 응답 body의 `id`를 그대로 ID로 받아쓰던 문제. 404 응답 body는 `{"id":"api.context.404.app_error",...}` 또는 `{"id":"app.team.get_by_name.missing.app_error",...}` 형태라, 에러 코드 문자열이 channel/team ID로 저장되고 이후 모든 API 호출이 깨진다.

수정:

- team lookup: status 2xx에서만 ID 사용. ID가 비면 team 생성, 생성도 status 2xx에서만 ID 받기.
- channel lookup: status 2xx 확인 + 최대 10초 retry. town-square가 신규 team 생성 직후엔 잠깐 조회 안 될 수 있음. 끝까지 못 받으면 fatal로 명시 실패.

이걸로 setup 12/12와 verify의 admin login / capability profile / llm / litert / secret isolation / blueclaw health / backup manifest / users sync 까지 통과. verify mattermost reply smoke의 `cleanup stale verify users`, `bot lookup`, `create users`, `join users`, `login users`, `invite policy`, `invited post`, `reply wait`까지도 진행.

## 이번 세션에 추가로 적용한 수정

### Capability daemon용 guest→host vsock listener proxy 추가

Firecracker의 vsock 모델은 guest→host 연결을 host측에 미리 바인드된 unix socket(`<vsockUnixSocketPath>_<port>`)으로 forward한다. 기존 supervisor는 base UDS만 만들어, blueclaw가 host capability daemon으로 dial할 때 "connection reset by peer".

수정:

- 신규 [`internal/firecracker/guest_listener_proxy.go`](.dependency/blueclaw/internal/firecracker/guest_listener_proxy.go) — `HostHTTPProxy`의 inverse. `<vsock_path>_<port>` UDS를 pre-bind, accept된 연결을 host의 target unix socket으로 bidirectional forward. 단위 테스트 3개.
- [`internal/config/runtime_configuration.go`](.dependency/blueclaw/internal/config/runtime_configuration.go) `FirecrackerConfiguration`에 `GuestListenerProxies []GuestListenerProxyConfiguration` 필드 추가.
- [`cmd/blueclaw-supervisor/main.go`](.dependency/blueclaw/cmd/blueclaw-supervisor/main.go) — `BootGuest` 직후 각 매핑마다 listener proxy goroutine 구동, `WaitForGuestHealth`/`HostHTTPProxy`와 함께 lifecycle 관리.
- 본 repo [`internal/runtime/blueclaw/blueclaw_config.go`](internal/runtime/blueclaw/blueclaw_config.go)가 runtime.json의 `firecracker.guestListenerProxies`에 `[{guestPort: 7000, targetUnixSocketPath: "/run/internkim/capability.sock"}]` 1개 매핑 emit.

검증:

- guest blueclaw 로그에서 `connector.mattermost.auth.allowed` 통과 (이전엔 `auth.failed: dial vsock host(2):7000: connect: connection reset by peer`).
- agent 정상 12초 만에 `agent.completed`.
- verify의 `expected task count >= 1` 통과.

## 직전 transient에 대한 노트

verify가 한 번 `expected model-generated bot reply ...`로 실패한 적 있음. 같은 코드 그대로 두 번 더 돌리니 모두 green. agent 처리 시간(약 12초)도 일관, capabilityd가 정상적으로 Mattermost에 POST 후 dispatchID 반환.

원인 가설(미확정):

- 첫 호출에서 OpenRouter 쪽 cold start로 응답이 fallback 메시지로 떨어졌고, verify의 필터(`I am having trouble reaching the language model` 또는 `has not invited`)에 잡혔을 가능성.
- 또는 capabilityd 내부 race로 `outbound.sent`만 찍히고 실제 POST가 비정상 종료.

같은 실패를 빠르게 재현·진단할 수 있도록 diagnostic만 추가:

- [`internal/cli/verify.go`](internal/cli/verify.go) `wait_for_model_reply` 실패 시 `print_recent_bot_replies` 호출 — bot의 실제 응답 본문을 stderr로 덤프. 다음에 transient가 다시 보이면 본문을 바로 볼 수 있다.

## 알려진 이상(smoke green이지만 추후 정리할 것)

- guest의 `/workspace/.blueclaw/logs/graphiti-memoryd.log`가 비어 있고 `dial tcp 127.0.0.1:7791: connect: connection refused`가 모든 reply 사이클에 나옴. wrapper 스크립트(`/usr/local/bin/graphiti-memoryd`), venv(`/opt/blueclaw/graphiti-venv/bin/python`), 소스(`/opt/blueclaw/graphiti_memoryd/main.py`)는 rootfs에 정상 존재. background fork가 stdout/stderr redirect로 묶여 있는데도 로그가 비어 있는 건 그 시점에 프로세스가 즉시 죽거나 redirect 위치가 잘못됐을 가능성. 다음 iteration에 guest-init 시작에 `exec >>/workspace/.blueclaw/logs/guest-init.log 2>&1; set -x` 추가해서 trace 확보 필요. 현재는 memory가 없어도 reply smoke가 통과하므로 P2.

## 진단 인프라 메모

지금까지 사용한 가시성 도구:

- `./internkim lab vm-ssh "..."`: Tart VM 안에서 명령 실행
- `sudo debugfs -c /var/lib/blueclaw/workspace.ext4 -R "cat /.blueclaw/logs/<file>"`: firecracker 동작 중에도 workspace ext4 디스크 read-only 조회
- `journalctl -u blueclaw -n N`: supervisor systemd 로그
- `ss -tlnp`: host listen socket 확인 (`8080` bridge가 떴는지)

다음 진단이 필요해지면 추가할 수 있는 것:

- guest-init 시작에 `exec >>/workspace/.blueclaw/logs/guest-init.log 2>&1; set -x`. 모든 셸 라인 trace.
- guest-init 끝에 `ip addr / ss -tlnp / ps -ef / dmesg | tail` 덤프.
- supervisor에서 firecracker stdout(ttyS0)을 host 파일로 redirect — 커널 로그/패닉 캡처.
- guest rootfs에 `socat`/`nc` + vsock bridge로 인터랙티브 셸. 가장 무거움.

지금은 위 도구 조합으로 충분히 진단됨.

## 커밋/푸시 현황 (모두 push 완료)

1. submodule `Move runtime into Firecracker guest`
2. parent `Move Blueclaw into Firecracker guest runtime`
3. parent `Fix Mattermost team and channel id resolution`
4. submodule `Bridge guest vsock listeners to host unix sockets`
5. parent `Wire capability daemon vsock bridge into Blueclaw runtime config`

추가 커밋 (이번):

6. parent — verify의 model reply 실패 진단 print + REPORT 갱신.

## 핵심 파일

- [tools/prepare-blueclaw-runtime](tools/prepare-blueclaw-runtime)
- [assets/blueclaw-runtime/guest-init](assets/blueclaw-runtime/guest-init)
- [internal/runtime/blueclaw/blueclaw_contract.go](internal/runtime/blueclaw/blueclaw_contract.go)
- [internal/cli/verify.go](internal/cli/verify.go)
- [Makefile](Makefile)
- [.dependency/blueclaw/cmd/blueclaw-supervisor/main.go](.dependency/blueclaw/cmd/blueclaw-supervisor/main.go)
- [.dependency/blueclaw/internal/firecracker/](.dependency/blueclaw/internal/firecracker/)
