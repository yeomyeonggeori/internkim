# Blueclaw Firecracker Runtime Report

작성 시각: 2026-05-01 01:50 KST

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
- workspace, skills, terminal/bash 실행 표면, artifacts/logs/blobs

제품 전환 조건은 sim smoke와 board smoke가 모두 성공하는 것이다. **현재 sim smoke green (두 번 연속 통과)**, 모델 응답이 fallback이 아니라 실제 LLM-generated reply로 verify 통과.

## 적용한 수정

### 1. Rootfs artifact 빌드 — base cache layer

[`tools/prepare-blueclaw-runtime`](tools/prepare-blueclaw-runtime)을 둘로 분리:

- `ensure_rootfs_base`: mmdebstrap + python venv를 `$cache_directory/rootfs-base-<key>.tar`로 캐시. cache key는 package list + release + `requirements.txt` sha256.
- `prepare_rootfs_directory`: 캐시 tar 추출 후 blueclaw binaries, guest-init, graphiti source만 덮어쓰기.
- 첫 cold build는 동일하지만 이후 iteration이 ~12분 → ~30초로 떨어진다.

### 2. guest-init 재구성 — [`assets/blueclaw-runtime/guest-init`](assets/blueclaw-runtime/guest-init)

- `ip link set lo up` 추가. 빠져 있어 guest 안 모든 `127.0.0.1` 통신이 "network is unreachable"로 silently fail.
- `blueclaw-guest-healthd`를 mount + chown 직후로 끌어올림. supervisor의 120s `WaitForGuestHealth` timeout이 첫 boot에 항상 터지는 것 해결.
- `mkdir`에서 `/workspace/.blueclaw/graphiti/kuzu` 항목 제거. Kuzu는 path를 DB 파일로 기대하지, 디렉터리로 두면 `Database path cannot be a directory` runtime exception. `rmdir kuzu 2>/dev/null || true`로 stale 빈 디렉터리도 자동 정리.
- graphiti가 listen될 때까지 `curl http://127.0.0.1:7791/health`로 대기 후 blueclaw 시작. Python 의존성(graphiti_core/kuzu/numpy/openai) 임포트 + Kuzu init이 30초+ 걸리는 동안 blueclaw가 먼저 떠서 memory.search가 connection refused로 실패하던 race 해결.

### 3. guest-healthd default를 liveness-only로 — [`cmd/blueclaw-guest-healthd/main.go`](.dependency/blueclaw/cmd/blueclaw-guest-healthd/main.go)

`-blueclaw-url`, `-graphiti-url` 기본값을 빈 문자열로. supervisor의 `WaitForGuestHealth`는 vsock 도달성만 확인하면 충분하고, app readiness는 setup verify가 직접 본다. 기존 default는 downstream 안 뜨면 `error`를 응답해 health timeout을 유발했다.

### 4. blueclaw `application.started` race — [`internal/app/application.go`](.dependency/blueclaw/internal/app/application.go)

`application.started` 로그가 `httpServer.ListenAndServe()` 호출 *전*에 찍혀, listener가 바인드되기 전에 host bridge가 dial하면 `connection refused`/empty reply. `net.Listen` → log `application.started` → `httpServer.Serve(listener)` 순서로 변경해 listener가 보장된 뒤에만 ready로 알림.

### 5. graphiti-memoryd가 startup하지 않던 버그 — [`tools/graphiti_memoryd/main.py`](.dependency/blueclaw/tools/graphiti_memoryd/main.py)

`def main():`만 정의돼 있고 호출하는 코드가 없었다. python이 import/정의 끝내고 status 0으로 그냥 종료 → 7791 listen 안 됨 → blueclaw memory 호출 모두 connection refused → agent가 fallback 메시지 반환. `if __name__ == "__main__": main()` 추가.

### 6. Capability vsock guest→host bridge — supervisor inverse proxy

Firecracker는 guest→host 연결을 host측 `<vsockUnixSocketPath>_<port>` UDS로 forward. 기존 supervisor는 base UDS만 만들어 blueclaw가 host capabilityd로 dial하면 "connection reset by peer".

신규/수정:

- 신규 [`internal/firecracker/guest_listener_proxy.go`](.dependency/blueclaw/internal/firecracker/guest_listener_proxy.go) — `HostHTTPProxy`의 inverse. `<vsock>_<port>` UDS pre-bind, accept된 연결을 host의 target unix socket으로 양방향 forward. 단위 테스트 3개.
- [`internal/config/runtime_configuration.go`](.dependency/blueclaw/internal/config/runtime_configuration.go) `FirecrackerConfiguration`에 `GuestListenerProxies` 필드.
- [`cmd/blueclaw-supervisor/main.go`](.dependency/blueclaw/cmd/blueclaw-supervisor/main.go) — `BootGuest` 직후 매핑마다 listener proxy goroutine 구동.
- [`blueclaw_config.go`](internal/runtime/blueclaw/blueclaw_config.go)가 runtime.json에 `[{guestPort: 7000, targetUnixSocketPath: "/run/internkim/capability.sock"}]` emit.

### 7. Mattermost setup의 잘못된 channel/team ID 저장 — [`internal/cli/main.go`](internal/cli/main.go)

`setupMattermost`가 team/channel lookup의 HTTP status를 무시하고 응답 body의 `id`를 그대로 받아 쓰던 버그. 404 응답 body가 `{"id":"app.team.get_by_name.missing.app_error",...}` 형태라 에러 코드가 ID로 저장됐다. status 2xx에서만 ID 사용으로 정정. channel lookup은 신규 team 직후 race를 위해 최대 10초 retry.

### 8. Migration 디렉터리를 host workspace 안으로 — [`blueclaw_contract.go`](internal/runtime/blueclaw/blueclaw_contract.go)

`BlueclawMigrationPath`을 `/root/.blueclaw/migrations` → `/root/.blueclaw/workspace/.blueclaw/migrations`. 기존 경로는 `SyncWorkspaceDirectory` rsync 대상 밖이어서 guest가 마이그레이션 미적용 에러를 냈다.

### 9. verify의 아키텍처 정합성 — [`internal/cli/verify.go`](internal/cli/verify.go)

- host 직접 `127.0.0.1:7791/health` (graphiti) 체크 제거. 새 아키텍처에서 graphiti는 guest 내부 dependency.
- users-sync 검증을 host의 `/root/.blueclaw/config/policy.json` 직접 읽기 대신 blueclaw API `/admin/api/policy`로 변경. invite 결과는 guest workspace에 쓰이는데 sync는 host→guest 단방향이라 host 파일엔 안 반영된다.
- `wait_for_model_reply`/`wait_for_bot_reply`/`wait_for_task_count` timeout을 30~45s에서 120s로. graphiti가 실제로 쓰이면 agent turn 1회가 50초 정도 걸린다.
- 모델 reply 실패 시 `print_recent_bot_replies` 호출해 본문 stderr 덤프 (재현 시 즉시 진단).
- users-sync 검증 mktemp cleanup을 explicit `rm -f` 대신 `trap EXIT`로 (script 실패 시에도 leak 안 남게).

### 10. Makefile

- `smoke-blueclaw-runtime-tart`의 `--only`에 `users-sync` 추가. verify가 unit/state 요구.
- 신규 `smoke-blueclaw-runtime-tart-fast` target. `--only binaries,blueclaw-runtime,services --verify`로 force-all 없이 IsSatisfied 만족된 step은 스킵, 빠른 iteration용.

## 알려진 자잘한 이슈 (smoke green이지만 추후 정리)

- `internkim-capabilityd`의 `--vsock-port` + 커널 `vsock.Listen` 코드는 Firecracker 모델에서 supervisor unix socket bridge 경유라 dead code. 별도 follow-up commit으로 제거 예정.
- graphiti-memoryd가 부팅 시 `posthog.com` 텔레메트리 DNS 해석 실패 (guest에 외부 DNS 없음). 동작에는 영향 없음 (warning만).
- supervisor `main.go`의 `stopProxy()`/`stopListenerProxies()` 호출이 `defer`와 명시 호출 양쪽에 있어 형식상 중복. 동작상 문제 없고 종료 순서를 명시하는 의도라 유지.

## 진단 도구

- `./internkim lab vm-ssh "..."` — Tart VM 명령
- `sudo debugfs -c /var/lib/blueclaw/workspace.ext4 -R "cat /.blueclaw/logs/<file>"` — Firecracker 동작 중에도 workspace ext4 read-only 조회
- `sudo debugfs -c /opt/internkim/blueclaw-runtime/rootfs.ext4 -R "cat /sbin/init"` — deployed rootfs의 guest-init 확인
- `journalctl -u blueclaw -u internkim-capabilityd -n N` — supervisor / capability daemon 로그
- `ss -tlnp` — host listen socket 확인
- guest-init 시작에 `exec >>logs/guest-init.log 2>&1; set -x`를 일시 추가하면 모든 셸 라인 trace (guest 안 디버그용)

## 커밋 (모두 push 완료 또는 push 예정)

이전 push 완료:

1. submodule `Move runtime into Firecracker guest`
2. parent `Move Blueclaw into Firecracker guest runtime`
3. parent `Fix Mattermost team and channel id resolution`
4. submodule `Bridge guest vsock listeners to host unix sockets`
5. parent `Wire capability daemon vsock bridge into Blueclaw runtime config`
6. parent `Print bot reply contents on model-reply timeout in verify`

이번 작업 (push 예정):

7. submodule — graphiti main() 호출 + healthd default + application.started race
8. parent — guest-init kuzu/graphiti wait/healthd flag, prepare-blueclaw-runtime python -u, verify timeout/cleanup, Makefile fast target, REPORT 갱신
