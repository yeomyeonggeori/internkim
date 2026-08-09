# SD Card Provisioning Status

> Legacy note: this document records the older ZeroClaw-era provisioning failures and fixes. Current runtime setup uses Blueclaw; keep any `zeroclaw` references here as historical context only.

## 목표

`./internkim setup` 한 번으로 SD 카드를 구워서 RPi5에 꽂으면, 전원만 연결하면 자동으로:
1. Wi-Fi 연결
2. 패키지 설치 (PostgreSQL, Mattermost, chromium, avahi-daemon 등)
3. 서비스 시작 (Mattermost, Blueclaw, Cloudflared)
4. SSH 접속 가능 → `./internkim status`로 상태 확인

## 해결된 것

| 항목 | 상태 |
|------|------|
| Wi-Fi 연결 (wpa_supplicant@wlan0) | ✅ |
| 글로벌 wpa_supplicant 충돌 해결 (stop+mask) | ✅ |
| netplan reconfigure 충돌 (10-netplan-wlan0.network 삭제) | ✅ |
| SSH 조기 시작 (Wi-Fi 직후) | ✅ |
| DHCP IP를 boot 파티션에 저장 (board-ip) | ✅ |
| systemd-resolved 비활성화 (Device or resource busy) | ✅ |
| DHCP DNS 사용 (ISP DNS 우선, networkctl 추출) | ✅ |
| 파티션 리사이즈 대기 (<5GB면 exit 0 → 다음 부팅) | ✅ |
| curl -k (시계 무관 네트워크 체크) | ✅ |
| status 서브넷 SSH 스캔 + hostname 검증 | ✅ |
| LED 상태 표시 (빠른점멸/하트비트/느린점멸) | ✅ |
| 시계 동기화 — HTTP 헤더에서 시간 추출 | ✅ |
| .deb 사전 패키징 (Apple Container → debs.tar) | ✅ |
| Swap 2GB를 dpkg 전에 생성 (OOM 방지) | ✅ |
| ext4 파티션 자동 확장 (resize2fs, MBR 업데이트) | ✅ |
| `--from N` 옵션 (step N부터 재시작, 입력 건너뜀) | ✅ |
| debugfs 에러 감지 (exit 0이지만 출력에 에러 있는 경우) | ✅ |
| firstboot fallback 제거 (사전 주입 필수, 없으면 즉시 실패) | ✅ |
| /tmp tmpfs → /var/cache/internkim/ 경로 변경 | ✅ |
| Sprintf `%s` 이스케이프 (`printf '%s'` → `'%%s'`) | ✅ |
| SSID 변경 자동 감지 → `--reset` 없이 이미지 재굽기 | ✅ |
| Mattermost team curl `|| true` 추가 (set -e 대응) | ✅ |
| e2fsprogs 버전 충돌 해결 (Armbian rootfs chroot 방식) | ✅ |
| bot→team 추가 curl `|| true` (set -e 대응) | ✅ |
| resolv.conf dangling symlink 처리 (rm -f + echo) | ✅ |
| bot team 멤버십 — request body에 team_id 추가 | ✅ |
| Dawn-kim-official zeroclaw (websocket, channel_id 불필요) | ✅ |
| zeroclaw props → config 명령어 변경 | ✅ |
| zeroclaw [memory] auto_save 필드 추가 | ✅ |
| bot Mattermost websocket 연결 + 응답 동작 확인 | ✅ |
| watchdog 서비스 (매 부팅 Wi-Fi/DNS/SSH/LED 보장) | ✅ |
| watchdog firstboot 중 건너뛰기 | ✅ |
| LED heartbeat — 서비스 전부 active 후에만 전환 | ✅ |
| NetworkManager/dhcpcd 비활성화 (Wi-Fi 충돌 방지) | ✅ |
| --reset 시 계정 비밀번호 보존 (boot FAT32 → macOS 백업) | ✅ |
| --reset 시 workspace 보존 (debugfs 파일별 추출) | ✅ |
| --reset 시 DB 보존 (watchdog pg_dump → debugfs 추출) | ✅ |
| --hard-reset 플래그 (완전 초기화) | ✅ |
| XML tool_result 채널 응답 누출 수정 (upstream PR #5796) | ✅ |

## 해결한 이슈 기록

### e2fsprogs 버전 충돌 → postgresql 설치 실패

**증상**: firstboot에서 `su: user postgres does not exist` → 스크립트 종료 (느린 점멸)

**원인**: container(Debian trixie)에서 다운로드한 `e2fsprogs` 1.47.2-3+b10이 Armbian에 설치된 `libext2fs2t64` 1.47.2-3+b7과 충돌. dpkg가 e2fsprogs를 설치 못함 → `apt-get -f install`이 `ssl-cert` → `postgresql-17` → `postgresql` 연쇄 실패 → postgres 유저 미생성.

**해결**: `debian:trixie-slim` 컨테이너 대신 실제 Armbian rootfs에 chroot하여 패키지 다운로드. 이미지에서 ext4 파티션 추출 → 컨테이너 안에서 loop mount → chroot → apt-get install -d. 동일한 dpkg 상태에서 의존성 해결하므로 버전 충돌 원천 차단.

### bot→team 추가 시 set -e 종료

**증상**: "Team ID:" 출력 후 스크립트 종료 (느린 점멸). ZeroClaw 설정까지 도달 못함.

**원인**: bot을 team에 추가하는 `curl -sf`가 실패 → `set -e`로 종료. `|| true` 누락.

**해결**: `curl -sf ... || true` 추가

### resolv.conf dangling symlink

**증상**: 컨테이너 내 chroot 시 `cp: not writing through dangling symlink` → debs.tar 미생성

**원인**: Armbian rootfs의 `/etc/resolv.conf`가 systemd-resolved로의 dangling symlink

**해결**: `cp` 대신 `rm -f` + `echo "nameserver 8.8.8.8"` 로 덮어쓰기

### bot team 멤버십 API 실패 (무응답)

**증상**: firstboot 로그에 "Bot added to team" 출력되지만 실제 team에 미추가

**원인**: `POST /api/v4/teams/{id}/members` request body에 `team_id` 필드 누락

**해결**: `{"team_id":"...","user_id":"..."}` 형태로 수정

### zeroclaw config 파싱 실패

**증상**: zeroclaw 서비스 시작 즉시 실패 — `missing field auto_save`

**원인**: Dawn-kim-official 버전이 `[memory]` 섹션에 `auto_save` 필수

**해결**: config.toml에 `auto_save = false` 추가

### zeroclaw Mattermost websocket 즉시 끊김

**증상**: "connected and authenticated" 직후 "Connection reset without closing handshake" 반복

**원인**: bot_token이 config에 설정 안 됨 (`zeroclaw props` → `zeroclaw config`으로 명령어 변경됨)

**해결**: `zeroclaw props set` → `zeroclaw config set`으로 전부 변경

### XML tool_result 채널 응답 누출

**증상**: bot 응답에 `<tool_result>{"command":"ls"}...</tool_result>` raw 출력 포함

**원인**: `sanitize_channel_response()`가 XML `<tool_result>` 태그를 제거하지 않음 (JSON만 처리)

**해결**: `strip_tool_result_content()` 호출 추가 (upstream PR zeroclaw-labs/zeroclaw#5796)

### Mattermost SiteURL에 이메일이 들어감

**증상**: Mattermost 시작 실패 — `invalid URI for request`

**원인**: firstboot의 `printf '%s'`가 Go `fmt.Sprintf`에 치환됨 → SITE_URL에 adminEmail이 들어감

**해결**: `printf '%s'` → `printf '%%s'`로 이스케이프

### Mattermost team 생성 시 set -e 종료

**증상**: "Setting up team..."에서 스크립트 종료 (느린 점멸)

**원인**: `curl -sf`로 team 조회 시 404 → exit 22 → `set -e`로 종료

**해결**: `curl -sf ... || true` 추가

### /tmp tmpfs 문제

**증상**: ext4에 주입한 파일이 부팅 후 없음

**원인**: Armbian이 `/tmp`를 tmpfs로 마운트 → ext4 파일이 가려짐

**해결**: `/tmp/` → `/var/cache/internkim/`로 변경

### ext4 free blocks 0 문제

**증상**: debugfs write 후 파일 불완전

**원인**: Armbian 원본 ext4 파티션에 여유 공간 0

**해결**: inject 전 ext4를 1GB 확장 (truncate + resize2fs + MBR 업데이트)

### debugfs "already exists" 무시 문제

**증상**: inject 성공처럼 보이지만 파일 없음

**원인**: debugfs가 exit 0 반환하면서 에러 출력

**해결**: 출력에서 "already exists", "No space" 체크 추가

## 현재 방식: 사전 패키징 (Offline-First)

### Armbian rootfs chroot로 사전 패키징
Armbian 이미지에서 ext4 rootfs를 추출 → Apple Container 안에서 loop mount + chroot → `apt-get install -d`로 패키지 다운로드 → ext4에 주입 → firstboot에서 오프라인 설치.

- 실제 Armbian 환경에서 의존성 해결 (버전 충돌 원천 차단)
- 396개 .deb 파일, 275MB (debs.tar)
- 캐시 재사용: 버전 바뀌기 전까지 다운로드 안 함
- `--reset`과도 무관 (캐시는 ~/.internkim/cache/)

## 아키텍처

```
macOS (./internkim setup [--reset] [--from N])
  ├─ [1] SD 카드 감지
  ├─ [2] Wi-Fi SSID/비밀번호 (변경 감지 → 자동 업데이트)
  ├─ [3] Google OAuth
  ├─ [4] OpenRouter API 키
  ├─ [5] 기기 등록 + 터널
  ├─ [6] Google SA 키
  ├─ [7] 이미지 굽기:
  │   ├─ Armbian trixie minimal 이미지 다운로드 + 캐시
  │   ├─ Mattermost tar.gz 다운로드 + 캐시 (430MB)
  │   ├─ Armbian rootfs chroot로 .deb 다운로드 + 캐시 (275MB)
  │   ├─ xz 해제 (항상 fresh)
  │   ├─ ext4 파티션 1GB 확장 (resize2fs + MBR)
  │   ├─ debugfs로 ext4에 주입:
  │   │   ├─ Wi-Fi, SSH, hostname, firstboot, cloudflared
  │   │   ├─ /var/cache/internkim/mattermost.tar.gz
  │   │   └─ /var/cache/internkim/debs.tar
  │   └─ dd로 SD에 쓰기
  ├─ [8] boot 파티션 스테이지 (바이너리, 시크릿, 설정)
  └─ [9] 완료

RPi5 부팅:
  1차: 파티션 확장 → 재부팅
  2차: firstboot →
    Wi-Fi (NM/dhcpcd mask) → SSH → DNS → 시계(HTTP) → Swap 2GB →
    dpkg -i (오프라인) → PostgreSQL (+ DB 복원) → Mattermost →
    Bot 생성 + team 추가 → Blueclaw →
    서비스 전부 active 확인 → LED 하트비트 → 스크립트 삭제
  이후 매 부팅: watchdog →
    Wi-Fi 보장 → DNS → SSH → board-ip 기록 → pg_dump → LED 하트비트

--reset 데이터 보존:
  비밀번호: boot FAT32 → ~/.internkim/backup/
  workspace: ext4 debugfs 파일별 추출
  DB: watchdog pg_dump (ext4) → debugfs 추출
  --hard-reset: 백업 전부 삭제, 완전 초기화
```

## 현재 블로커

### Wi-Fi가 firstboot 후 시간 경과 시 끊김

firstboot 완료 후 모든 서비스 active, SSH 가능. 그러나 일정 시간 후 Wi-Fi 연결 끊김 (ping 불가, SSH 불가, 터널 끊김). 재부팅 아님 — 전원이 켜진 상태에서 발생.

**가설**: NetworkManager 또는 dhcpcd가 wpa_supplicant@wlan0과 충돌. 현재 둘 다 mask하는 코드 추가됨, 테스트 필요.

## 다음 단계

1. Wi-Fi 끊김 원인 확인 및 해결
2. Orange Pi 5 지원
