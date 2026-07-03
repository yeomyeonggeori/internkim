# InternKim PoC — Apple Container 멀티테넌트

M1/M2 맥에서 Firecracker 없이 N개의 독립 인턴김 테넌트를 Apple Container로 실행.
Postgres 하나, Mattermost 하나를 공유하고 테넌트당 컨테이너 한 개.

## 아키텍처

```
internkim-poc 네트워크 (192.168.65.0/24)
├── poc-postgres        — postgres:16-alpine (arm64)
├── poc-mattermost      — mattermost-team-edition:10.5 (amd64, Rosetta)
├── poc-tenant-01
├── poc-tenant-02
│   ...
└── poc-tenant-NN
```

각 테넌트 컨테이너는 `internkim-capabilityd` + `blueclaw`를 직접 실행(Firecracker/systemd 없음).
LLM은 OpenRouter, 터미널은 native 모드.

측정 유휴 메모리: 테넌트당 ~8 MiB, 10테넌트 전체 ~1.4 GiB (Postgres + Mattermost 포함).

## 디렉터리 구조

```
poc/
├── setup-apple-container.sh   # 신규 맥 초기 셋업 (한 번만)
├── start-poc.py               # 테넌트 시작/재시작 (매 부팅 또는 수동)
├── poc-autostart.sh           # LaunchAgent 래퍼 (직접 편집 불필요)
├── launchagent.plist.template # LaunchAgent plist 템플릿
├── generate-configs.sh        # 테넌트 runtime.json 생성
├── provision-mattermost.sh    # Mattermost 팀·봇 계정 프로비저닝
├── configgen/                 # runtime.json 생성기 (Go)
├── tenant/                    # 테넌트 이미지 (Dockerfile, entrypoint.sh)
└── infra/                     # 인프라 설정

~/internkim-poc/               # 런타임 데이터 (gitignore)
├── volumes/postgres/          # Postgres 영구 데이터
├── volumes/mattermost/        # Mattermost 영구 데이터
├── config/tenant_NN/          # 테넌트별 runtime.json, policy.json
├── secrets/tenant_NN/         # 테넌트별 봇 토큰, 관리자 비번 등
├── secrets/openrouter-key     # 공용 OpenRouter API 키
└── infra/mattermost.cfg       # Mattermost 환경변수 (DB 접속 정보 등)
```

---

## 신규 맥 셋업

### 1. 사전 준비

```bash
# Apple Container 설치 확인
container system status   # "running" 이어야 함

# 런타임 데이터 디렉터리 생성 및 복사
mkdir -p ~/internkim-poc
# 기존 맥에서 복사:
rsync -av dawn@macstudio.local:~/internkim-poc/secrets/ ~/internkim-poc/secrets/
rsync -av dawn@macstudio.local:~/internkim-poc/infra/   ~/internkim-poc/infra/
rsync -av dawn@macstudio.local:~/internkim-poc/volumes/ ~/internkim-poc/volumes/
```

### 2. 테넌트 이미지 준비

```bash
# 이미지 빌드용 바이너리를 tenant/bin/ 에 넣어야 함:
#   blueclaw, blueclaw-posix-helper, internkim-capabilityd  (linux/arm64)
# .dependency/blueclaw/migrations → tenant/migrations/ 복사

# 기존 맥에서 이미지 내보내기
container image save internkim-poc-tenant:flow -o tenant-flow.tar
# 새 맥에 옮겨서 로드
container image load -i tenant-flow.tar
```

### 3. 셋업 실행

```bash
cd /path/to/internkim

# DB 복원 없이 (secrets/config 이미 있는 경우)
bash poc/setup-apple-container.sh

# 또는 DB 덤프에서 복원
pg_dumpall -U internkim -h <기존_pg_ip> > dump.sql   # 기존 맥에서
bash poc/setup-apple-container.sh --restore-from dump.sql
```

`setup-apple-container.sh`가 하는 일:
1. `internkim-poc` 네트워크 생성
2. postgres/mattermost 이미지 pull
3. 인프라 컨테이너 기동 및 대기
4. 테넌트 configs 생성 (postgres/mattermost IP 자동 주입)
5. LaunchAgent 설치 (부팅 시 자동 시작)
6. 테넌트 컨테이너 기동

---

## 일상적인 관리

### 테넌트 재시작

```bash
python3 poc/start-poc.py
```

컨테이너 IP가 매 실행마다 달라질 수 있어서, `start-poc.py`가 실행 시마다
`container inspect`로 실제 IP를 읽어 `config/tenant_NN/runtime.json`을 자동 패치한다.

### 테넌트 추가

Mac Studio의 `~/internkim-poc`에서 한 번에 실행한다. DB·Mattermost 팀/봇/관리자 계정·
설정 복제·컨테이너 기동·blueclaw 정책 초대·DNS·터널 ingress·DM 왕복 스모크까지 전부
수행하고, 성공 시 `tenants-credentials.md` 테이블을 갱신한다. 각 단계는 idempotent라서
중간 실패 후 재실행해도 안전하다.

```bash
./add-tenant.sh 15
```

계정/비밀번호는 `~/internkim-poc/tenants-credentials.md`(로컬 사본:
`.local/ops/poc-tenants-credentials.md`)와 `secrets/tenant_NN/`에서 확인한다.

아래 구식 절차(docker compose 기준 provision-mattermost.sh 등)는 Apple Container
마이그레이션 이후 동작하지 않으므로 사용하지 않는다.

### Cloudflare 터널 재시작

```bash
python3 poc/restart-tunnel.py
```

컨테이너 IP가 바뀌면 터널 ingress도 자동으로 업데이트한다.
`~/internkim-poc/cf.env`에 `CF_ACCOUNT_ID`, `CLOUDFLARE_API_TOKEN` 필요.

### 인프라만 재시작

```bash
export PATH=/opt/homebrew/bin:$PATH
container start poc-postgres
sleep 5
container start poc-mattermost
sleep 10
python3 poc/start-poc.py
python3 poc/restart-tunnel.py
```

### 컨테이너 상태 확인

```bash
container list
container logs poc-tenant-01
container exec poc-tenant-01 cat /tmp/blueclaw.log
```

---

## 외부 네트워크에서 복구

macstudio는 SSH용 Cloudflare 터널(`ssh-poc.intern.kim`)이 맥OS 호스트 LaunchAgent로 상시 동작.
컨테이너가 전부 죽어도 macOS가 살아있는 한 SSH 접속 가능.

```bash
# 클라이언트에 cloudflared 필요
brew install cloudflare/cloudflare/cloudflared

# 접속
ssh -o ProxyCommand='cloudflared access ssh --hostname %h' dawn@ssh-poc.intern.kim

# ~/.ssh/config 등록 (편의)
Host ssh-poc.intern.kim
    ProxyCommand cloudflared access ssh --hostname %h
```

복구 후:
```bash
export PATH=/opt/homebrew/bin:$PATH
python3 ~/internkim-poc/poc-autostart.sh   # 인프라 + 테넌트 + 터널 전체 재기동
```

새 맥에 SSH 터널 처음 셋업:
```bash
brew install cloudflare/cloudflare/cloudflared
python3 poc/setup-ssh-tunnel.py   # cf.env가 ~/internkim-poc/에 있어야 함
```

## 주의사항

| 항목 | 내용 |
|------|------|
| colima와 동시 실행 불가 | 32GB 맥에서 메모리 부족. colima는 완전히 중단 후 Apple Container 사용 |
| 컨테이너 이름 DNS 없음 | Apple Container 1.0은 네트워크 내 이름 해석 미지원. IP 직접 사용 |
| `container restart` 없음 | 1.0.0 미지원. `container stop` + `container start` 사용 |
| IP 변동 | 재시작할 때마다 IP가 바뀔 수 있음. `start-poc.py`가 자동 패치 |
| mattermost.cfg | DB 접속 정보 포함. `~/internkim-poc/infra/mattermost.cfg`에 위치 |
| arm64 Mattermost 없음 | 10.5 태그가 amd64 전용이라 Rosetta 에뮬레이션으로 실행됨 |

---

## 미구현 항목

- 테넌트별 Cloudflare 터널 (현재는 poc-0.intern.kim → Mattermost 단일 터널만)
- 터미널 툴체인 (bun/uv/python) — `terminal.run` 태스크 필요 시 이미지에 추가
- POSIX per-person 격리
