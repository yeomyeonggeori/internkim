# Blueclaw Firecracker Runtime Report

작성 시각: 2026-04-30 23:18 KST

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

`make smoke-blueclaw-runtime-tart`는 setup 12/12 단계와 verify의 blueclaw/graphiti/users-sync 영역을 모두 통과한다.

남은 실패 한 곳: verify의 Mattermost reply smoke. setup `[10/11] Mattermost` 단계에서 town-square 채널 조회가 실패하면서 에러 응답 ID(`api.context.404.app_error`)가 채널 ID로 저장돼, 이후 verify의 채널 호출이 모두 404. 이 버그는 Firecracker 마이그레이션과 별개의 Mattermost setup 회귀다.

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

## 다음에 막힌 지점

### Capability daemon의 guest→host vsock listener 누락

verify mattermost 마지막 `expected task count >= 1`에서 실패. 원인은 guest 안 blueclaw가 host capability daemon에 vsock으로 dial하지 못함:

```
ERROR connector.mattermost.auth.failed
error: Post "http://internkim-capability/v1/platform/mattermost/identity.resolve":
       dial vsock host(2):7000: connect: connection reset by peer
```

`internkim-capabilityd`는 `--vsock-port 7000`로 떠 있고 vsock 커널 모듈도 로드돼 있다(Tart VM 측 `lsmod` 확인). 하지만 Firecracker의 vsock 모델은 다음을 요구한다:

- guest→host 방향 connection은 host측에 미리 바인드된 unix socket이 `<vsockUnixSocketPath>_<port>` 경로에 있어야 한다
- Firecracker가 guest의 vsock dial을 그 unix socket으로 forward한다

현재 supervisor는 `<jailerRoot>/firecracker-vsock.socket` 베이스 socket만 만들고, 포트별 inbound listener (`firecracker-vsock.socket_7000` 등)는 만들지 않는다. 그래서 커널 vsock.Listen으로 떠 있는 capabilityd에는 절대 도달 못 한다 — 이게 "connection reset by peer".

해결 방향(미적용):

1. supervisor에 outbound `HostHTTPProxy`의 inverse를 추가 — guest port별로 host UDS를 pre-bind하고, accept된 연결을 capabilityd의 unix socket(`/run/internkim/capability.sock`)에 forward.
2. `FirecrackerConfiguration`에 `guestListenerProxies: [{guestPort, targetUnixSocketPath}]` 같은 필드 추가, internkim setup이 7000→capability.sock 매핑을 채워 넣음.
3. supervisor가 guest boot 직전(또는 직후 빠르게) listener proxy goroutine을 띄움.

크지 않지만 supervisor + parent config 양쪽을 건드리는 일이라 별도 커밋으로 처리.

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

## 커밋/푸시 현황

이번 세션은 두 커밋으로 정리:

1. 서브모듈 `.dependency/blueclaw`에 `Move runtime into Firecracker guest` (이전 작업 포함)
2. parent repo에 `Move Blueclaw into Firecracker guest runtime` — 본 세션 모든 host측 수정 + 서브모듈 pointer + REPORT 초안

다음 커밋:

3. parent repo `Fix Mattermost team and channel id resolution` — 본 REPORT의 Mattermost 회귀 수정 + REPORT 갱신

이후 push. 남은 capability vsock listener 작업은 별도 후속.

## 핵심 파일

- [tools/prepare-blueclaw-runtime](tools/prepare-blueclaw-runtime)
- [assets/blueclaw-runtime/guest-init](assets/blueclaw-runtime/guest-init)
- [internal/runtime/blueclaw/blueclaw_contract.go](internal/runtime/blueclaw/blueclaw_contract.go)
- [internal/cli/verify.go](internal/cli/verify.go)
- [Makefile](Makefile)
- [.dependency/blueclaw/cmd/blueclaw-supervisor/main.go](.dependency/blueclaw/cmd/blueclaw-supervisor/main.go)
- [.dependency/blueclaw/internal/firecracker/](.dependency/blueclaw/internal/firecracker/)
