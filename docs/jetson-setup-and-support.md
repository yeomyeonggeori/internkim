# 젯슨 신규 세팅 및 원격 지원 가이드

## 재부팅이 필요한 경우

**불필요:** WiFi 변경(nmcli가 라이브 적용), OTA 업데이트(admind가 서비스 재기동), 서비스 다운(systemctl이 자동 재기동).

**필요:** OS/커널 업데이트, 완전 시스템 패닉 후 원격 복구 불가 시. 정상 운영 중에는 재부팅이 필요한 상황이 거의 없다.

---

## 1. 신규 젯슨 세팅 (새 SSD)

### 사전 준비

- JetPack OS가 설치된 젯슨 + SSD
- Mac에 `internkim` CLI 빌드 완료 (`make build`)
- `.env` — `INTERNKIM_API_URL`, `INTERNKIM_REGISTER_SECRET` 등
- 고객사 프로파일명 (`--profile`)
- WiFi 사용 환경이면 SSID + 비밀번호 (이더넷 환경이면 불필요)

### 세팅 절차

**1단계: 젯슨에 이더넷 케이블 연결 후 부팅**

이더넷을 꽂으면 WiFi 정보 없이 바로 세팅 가능. 유선 환경이라면 WiFi 설정 전체를 건너뛸 수 있다.

**2단계: 전체 세팅 실행**

이더넷 환경 (WiFi 불필요):
```bash
./internkim setup --profile <고객사프로파일명>
```

WiFi도 함께 등록할 경우:
```bash
./internkim setup \
  --wifi-ssid "고객사WiFi" \
  --wifi-password "비밀번호" \
  --profile <고객사프로파일명>
```

IP를 자동으로 발견하지 못하면 `--host 192.168.x.x` 추가.
실행 전 단계 확인: `--plan` 플래그 추가.

setup이 완료되면:
- Cloudflare 터널 활성화 (외부 HTTPS + SSH 접근 가능)
- WiFi NetworkManager 프로파일 설치 + 2분 주기 자동복구 타이머 기동
- Mattermost, Blueclaw, admind, capabilityd 서비스 기동

**3단계: 정상 동작 확인**

```bash
./internkim ssh --node <node-id>   # Cloudflare 터널 통해 접속
./internkim recover ssh --action status --diagnose
```

---

## 2. WiFi 변경 (권장: Admin UI)

이더넷을 꽂고 admin UI의 **네트워크** 탭에서 WiFi를 추가하거나 비밀번호를 변경한다. 저장 후 이더넷을 뽑으면 자동으로 WiFi로 전환된다. 재부팅 불필요.

---

## 3. WiFi 변경 (대안: 키보드 직접 연결)

admin UI 접근이 불가할 때. 젯슨에 HDMI 모니터 + USB 키보드 연결 후 root로 로그인.

### 3-1. 새 WiFi 추가

```bash
sudo nmcli connection add \
  type wifi \
  con-name "internkim-wifi-new" \
  ssid "새_SSID" \
  wifi-sec.key-mgmt wpa-psk \
  wifi-sec.psk "새_비밀번호"

sudo nmcli connection up "internkim-wifi-new"
```

### 3-2. 기존 WiFi 비밀번호 수정

```bash
nmcli connection show | grep internkim-wifi

sudo nmcli connection modify "internkim-wifi-<이름>" wifi-sec.psk "새_비밀번호"
sudo nmcli connection up "internkim-wifi-<이름>"
```

### 3-3. 인터넷 복구 후 Cloudflare 터널 확인

```bash
ping -c 3 8.8.8.8
sudo systemctl status cloudflared-node-ssh
# 자동 재기동이 안 된 경우:
sudo systemctl restart cloudflared-node-ssh
```

터널이 살아나면 Mac에서 `./internkim ssh` 로 원격 접속 재개.

### 3-4. Mac에서 WiFi 프로파일 정식 등록 (인터넷 복구 후)

```bash
./internkim setup --only wifi --wifi-ssid "새_SSID" --wifi-password "새_비밀번호"
```

---

## 4. 업데이트 방식

### 정상 OTA 경로

```bash
# 특정 컴포넌트만 배포 (가장 흔한 경우)
./internkim deploy --components admind
./internkim deploy --components capabilityd
./internkim deploy --components blueclaw
./internkim deploy --components web

# 전체 릴리즈 채널 발행 (디바이스가 자동 수신)
./internkim release publish --channel stable
```

디바이스의 admind가 `channels/stable.json`을 주기적으로 확인한다. 새 릴리즈 발견 시 청크 단위 다운로드 → SHA256 검증 → 설치 → 서비스 재기동. 네트워크가 불안정해도 청크 retry + resumable upload로 수렴.

```bash
./internkim release status --channel stable
```

### OTA로 해결하기 어려운 경우

| 상황 | 이유 |
|------|------|
| WiFi/네트워크 단절 | 디바이스가 R2에 접근 불가 |
| OS/커널/하드웨어 문제 | OTA 범위 밖 |
| Cloudflare 터널 토큰 만료 | setup --only tunnel 재실행 필요 |

---

## 5. 다른 Mac에서 설치한 젯슨에 내 Mac으로 재연결하기

설치(setup)를 다른 컴퓨터에서 실행했을 경우, 내 Mac에는 해당 디바이스 state가 없다. `./internkim ssh`, `./internkim deploy`, `./internkim recover` 모두 이 state를 참조하므로 복사가 필요하다.

### 전달받아야 할 것

설치한 컴퓨터의 `~/.internkim/` 디렉토리를 압축해서 메일로 받는다:

```bash
# 설치한 컴퓨터에서 실행 — 출력된 파일을 메일로 전달
tar czf internkim-state.tar.gz -C ~ .internkim/
```

### 내 Mac에서 적용

```bash
# 기존 ~/.internkim/ 이 없으면 그냥 압축 해제
tar xzf internkim-state.tar.gz -C ~

# 이미 다른 디바이스 state가 있으면 병합 (덮어쓰기 대신)
tar xzf internkim-state.tar.gz -C ~ --keep-old-files
```

이후 `./internkim ssh --node <node-id>` 그대로 사용 가능.

> **참고:** `.env` 파일(Cloudflare Access 토큰 등)은 팀 공유이므로 따로 받을 필요 없다.

---

## 6. 다중 젯슨 원격 지원 전략

### 5-1. Cloudflare 터널로 일시 SSH

각 젯슨은 고유한 `cloudflared-node-ssh` 터널을 유지한다. Cloudflare Access 정책으로 우리 팀 이메일만 인증 통과 — 고객사는 SSH 불가.

```bash
./internkim ssh --node <node-id>
./internkim ssh --fleet <fleet-id>
```

### 5-2. SSH 없이 HTTPS 복구 API로 내부 점검

```bash
./internkim recover ssh --action status --diagnose
./internkim recover ssh --action journal-tail
./internkim recover ssh --action restart-cloudflared-node-ssh
./internkim recover ssh --action restart-ssh
```

admind의 `/admin/api/recovery/` 엔드포인트를 fleet-secret으로 서명된 HTTPS 요청으로 호출. SSH 없이도 서비스 재기동과 로그 수집 가능.

### 5-3. 채팅 기반 진단

Mattermost + Blueclaw가 동작 중이면 관리자 채널에서 `terminal_run`, `file_read` 등으로 직접 진단.

### 5-4. 지원 계층 요약

| 상황 | 수단 | SSH 필요 |
|------|------|---------|
| 소프트웨어 업데이트 | `./internkim deploy` | 불필요 |
| 서비스 재기동/로그 | `./internkim recover` | 불필요 |
| WiFi 변경 (admin UI) | admin 네트워크 탭 | 불필요 |
| 채팅 기반 진단 | 김인턴 관리자 채널 | 불필요 |
| WiFi 변경 (단절 후) | 현장 키보드 + 섹션 3 절차 | 불필요 |
| 심층 점검/디버깅 | `./internkim ssh --node <id>` | 일시 허용 |
| OS/HW 복구 | 현장 직접 접근 | 현장 |
