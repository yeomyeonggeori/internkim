# Testing with Simulator

ARM64 Debian 컨테이너를 사용해 실제 보드 없이 provisioning을 테스트합니다.
[apple/container](https://github.com/apple/container)를 사용하므로 systemd, systemctl, landlock이 실제로 동작합니다.

## 요구사항

- macOS 26 이상 (Apple Silicon)
- `apple/container` 설치: https://github.com/apple/container/releases
- `container system start` 로 서비스 실행 중
- `~/.ssh/id_ed25519.pub` (또는 `id_rsa.pub`) — 컨테이너 SSH 인증에 사용

## 시뮬레이터 시작 + Provisioning

```bash
# 빌드
go build -o internkim .

# 시뮬레이터 시작 + setup 자동 실행 (첫 실행은 패키지 설치로 2-3분 소요)
./internkim sim
```

컨테이너가 뜨면 `~/.internkim/shared/` 디렉토리가 컨테이너 `/root/shared`에 마운트되고, setup이 자동으로 이어집니다.

처음부터 다시 테스트하려면:

```bash
./internkim sim reset   # 컨테이너 초기화 후 setup 재실행
```

setup은 실제 보드와 동일한 10단계를 실행합니다:

| 단계 | 내용 | 시뮬레이터 동작 |
|------|------|----------------|
| 1 | 보드 감지 | container inspect로 IP 자동 감지 |
| 2 | Wi-Fi 감지 | 스킵 (컨테이너는 이미 네트워크 연결) |
| 3 | Wi-Fi 설정 | 스킵 |
| 4 | Wi-Fi 확인 | 스킵 |
| 5 | 바이너리 설치 | zeroclaw / gws / cloudflared / rtk / lightpanda 다운로드 및 설치 |
| 6 | OpenRouter API 키 | 입력 프롬프트 |
| 7 | 기기 등록 + 터널 | Cloudflare 터널 설정 |
| 8 | Google Workspace | 서비스 계정 설정 |
| 9 | Mattermost | 설치 및 설정 |
| 10 | 서비스 시작 | zeroclaw / lightpanda systemd 서비스 등록 및 시작 |

## 파일 공유 (shared 디렉토리)

Mac의 `~/.internkim/shared/`는 컨테이너 `/root/shared`에 실시간 마운트됩니다.

```
Mac Finder에서 파일 드래그  →  ~/.internkim/shared/  →  컨테이너 /root/shared/
```

Finder에서 `~/.internkim/shared/`를 사이드바에 즐겨찾기로 등록해두면 편리합니다.

## SSH 직접 접속

```bash
./internkim sim ssh
```

또는 수동으로:

```bash
# 컨테이너 IP 확인
container inspect internkim-sim --format '{{.Network.IPAddress}}'

ssh root@<IP>
```

## 기타 명령

```bash
./internkim sim ssh      # 실행 중인 컨테이너에 SSH 접속
./internkim sim status   # running / stopped
./internkim sim stop     # 컨테이너 정리
container list           # 실행 중인 컨테이너 목록
```

## 자주 쓰는 검증 명령

시뮬레이터에 SSH 접속 후:

```bash
# 서비스 상태
systemctl status zeroclaw
systemctl status lightpanda
systemctl status mattermost
systemctl status cloudflared

# 바이너리 확인
which zeroclaw rtk lightpanda gws cloudflared

# ZeroClaw 설정 확인
cat ~/.zeroclaw/config.toml

# 공유 디렉토리
ls /root/shared/

# lightpanda CDP 포트
ss -tlnp | grep 9222
```

## 주의사항

- 시뮬레이터는 인터넷에 연결되므로 실제 Cloudflare 터널, OpenRouter API 키가 필요합니다
- 테스트 후 `./internkim sim stop`으로 반드시 정리하세요
- 컨테이너를 재시작하면 설치된 패키지가 초기화됩니다 (`--force` 플래그로 재설치 가능)
