# PoC 컨테이너 운영 (legacy 공유 인프라 모델)

현재 Mac-Studio PoC 운영과 배포는 Apple Container 기반 `poc/` 경로와
[docs/poc-host.md](poc-host.md)를 기준으로 한다. 일반 운영자는 `.local/ops/targets.json`에
`kind: poc-container` target을 등록하고 fleet-aware `deploy`를 사용한다.

이 문서는 `internkim tenant container ...`로 공유 Postgres와 공유 Mattermost 위에
테넌트를 여러 개 띄우던 legacy Docker/Colima 실험 모델의 기록이다. 각 테넌트는
Blueclaw를 **direct/native 모드**로 실행하는 컨테이너 하나이며
(Firecracker·supervisor·systemd 없음), 하나의 Postgres와 하나의 Mattermost를 공유한다.

[docs/poc-host.md](poc-host.md)의 nspawn 호스트 모델과 비교하면, 그쪽은 테넌트마다
독립 Mattermost를 가진 강력한 단일 호스트용이고(테넌트당 ~1GB), 이쪽은 공유 인프라로
램을 최소화한 PoC용이다.

## 구조

- **공유 Postgres** — 테넌트당 데이터베이스 하나(`tenant_01`..`tenant_NN`)와
  `mattermost`. blueclaw가 부팅 시 자체 마이그레이션을 적용한다.
- **공유 Mattermost** — 테넌트당 팀 하나와 에이전트 신원 하나. 에이전트의 봇 토큰이
  테넌트 경계다(그 토큰이 볼 수 있는 채널만 connector가 본다).
- **테넌트 컨테이너** — `internkim-capabilityd` + `blueclaw`를 entrypoint가 직접
  실행하고 unix 소켓으로 연결한다. `terminal.mode=native`, graphiti 비활성,
  POSIX 동기화 생략, LLM은 OpenRouter 직접 호출.

검증된 풋프린트: 테넌트당 idle ~8 MiB, 10 테넌트 전체 스택(Postgres·Mattermost 포함)
약 1.4 GiB. 모두 colima 리눅스 VM 한 대 안에서 동작한다.

런타임 설정은 `internal/runtime/blueclaw/blueclaw_config.go`의
`BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{DirectExecution: true, ...})`가
생성한다(디바이스의 firecrackerGuest 경로는 `DirectExecution=false`로 무손상 유지).

## 격리 경계 (회사 간)

테넌트(회사)별로 다음이 분리된다:

- **런타임** — 회사별 컨테이너(파일시스템·프로세스·capability 소켓·봇 토큰).
  컨테이너 경계로 A 회사 에이전트는 B 회사의 컨테이너 파일시스템·프로세스에 닿지 못한다.
- **작업 데이터(DB)** — 각 blueclaw는 자기 `tenant_NN` 데이터베이스에만 접속한다.
  단, 단순히 DB만 나누는 것으로는 부족하다: 모든 테넌트가 공유 Postgres 슈퍼유저
  자격증명을 쓰고 컨테이너가 같은 네트워크에서 `postgres`에 도달하면, 한 테넌트
  에이전트가 `terminal_run`의 `psql`로 다른 테넌트 DB를 읽을 수 있다(실측으로 확인됨).
  그래서 **테넌트별 최소권한 Postgres 롤**을 강제한다: 테넌트마다 자기 비밀번호를 가진
  롤이 자기 DB를 소유하고, `REVOKE CONNECT ON DATABASE ... FROM PUBLIC` + 자기 롤에만
  `GRANT CONNECT`. 각 테넌트 DSN은 슈퍼유저가 아니라 자기 롤을 쓴다. `mattermost` DB도
  PUBLIC CONNECT를 회수한다. 그러면 네트워크·psql이 있어도 Postgres 엔진이
  `permission denied for database`로 교차 접근을 막는다.

채팅(Mattermost)은 **인스턴스 하나에 회사당 팀 하나**다. 팀은 본래 "한 조직 안
부서" 경계이지 상호 불신 테넌트용 하드 경계가 아니므로, 회사 간 가시성을 닫기 위해
다음을 강제한다(infra compose 환경변수로 기본 적용):

- `TeamSettings.RestrictDirectMessage=team` — 타 회사 사용자에게 DM 금지
- `PrivacySettings.ShowFullName=false`, `ShowEmailAddress=false` — 사용자 검색에서
  타 회사 직원 노출 금지
- `TeamSettings.EnableOpenServer=false` — 임의 가입 금지(팀은 초대제, 각 회사 직원은
  자기 팀에만 추가)

이 하드닝으로 A 회사 직원은 정상 사용 범위에서 B 회사의 사용자·채널·메시지를 볼 수
없다. 다만 단일 Mattermost 인스턴스·DB를 공유하는 **소프트 경계**이므로, 계약상
데이터 격리나 상호 불신 고객이 필요한 단계에서는 테넌트별 독립 Mattermost(하드 경계,
테넌트당 ~+440MB)로 승격한다. 데이터·에이전트·봇토큰 분리는 두 모델 모두 동일하다.

## 현재 PoC 기준

- Mac host scripts: `poc/README.md`
- 호스트 멀티테넌트 운영: [docs/poc-host.md](poc-host.md)
- fleet 배포: `.local/ops/targets.json` target `kind: poc-container`
- 배포 명령: `INTERNKIM_POC_SSH_PASSWORD=<pw> ./internkim deploy --components admind,capabilityd,blueclaw,web --fleet <poc-id>`

손으로 build/scp/docker compose/container restart를 조합하지 않고 `deploy`를 사용한다.

## Legacy 최초 1회: 호스트 준비

```bash
# 1. legacy 컨테이너 런타임 (램 가벼운 colima)
brew install colima docker docker-compose
colima start --cpu 6 --memory 20 --disk 60

# 2. 테넌트 이미지 빌드 컨텍스트 스테이징 (<workdir>/tenant/)
#    - linux/arm64 바이너리: internkim-capabilityd, blueclaw, blueclaw-posix-helper
#    - .dependency/blueclaw/migrations 복사
#    (cmd 가 직접 크로스컴파일하지 않는다. 산출물을 미리 둔다.)

# 3. 공유 OpenRouter 키 배치 (BYOK)
#    <workdir>/secrets/openrouter-key
```

## Legacy 인프라와 테넌트 기동

```bash
# 공유 인프라(Postgres + Mattermost) 기동 후 헬스 대기
internkim tenant container infra-up --workdir <DIR> --tenant-count 10

# 인프라 + 이미지 빌드 + 테넌트 01..N 일괄 기동
internkim tenant container up --workdir <DIR> --count 10

# 상태 확인 (컨테이너 상태 + 팀 + DB + 접근 URL)
internkim tenant container status --workdir <DIR>
```

## Legacy 테넌트 추가 / 제거 / 리셋

```bash
# 하나 추가: DB + Mattermost 팀·에이전트·토큰 + 설정 + 컨테이너
internkim tenant container add --workdir <DIR> --tenant tenant11

# 하나 제거 (--purge-data 면 DB까지 드롭)
internkim tenant container remove --workdir <DIR> --tenant tenant11 --purge-data

# 전체 리셋 (--purge 면 인프라·볼륨·생성물까지)
internkim tenant container reset --workdir <DIR> --purge

# 인프라만 내리기
internkim tenant container infra-down --workdir <DIR> [--purge]
```

## 코드 재배포 (바이너리 갱신)

코드를 바꾼 뒤 PoC에 반영할 때는 손으로 빌드·scp·docker를 돌리지 말고
fleet-aware `deploy`를 쓴다. `.local/ops/targets.json`(gitignore)에 PoC를
`kind: poc-container`로 등록해 두면 `--fleet <id>`로 그 호스트만 배포한다.

```jsonc
// .local/ops/targets.json
{ "targets": [
  { "id": "poc", "kind": "poc-container", "sshHost": "<host>", "sshUser": "<user>",
    "workdir": "<DIR>", "imageTag": "internkim-poc-tenant:flow",
    "sshProxyCommand": "cloudflared access ssh --hostname %h" }
]}
```

```bash
# 바뀐 컴포넌트만 (admind/capabilityd/blueclaw/web 공유 어휘)
INTERNKIM_POC_SSH_PASSWORD=<pw> internkim deploy --components admind,web --fleet poc
```

`blueclaw`는 `.dependency/blueclaw`에서 `blueclaw`+`blueclaw-posix-helper`를
linux/arm64로 빌드하고 마이그레이션까지 동기화한다(파이어크래커 페이로드 아님).
deploy가 컴포넌트 빌드 → scp → Apple `container build` →
`start-poc.py` → `restart-tunnel.py`까지 수행한다. 전체 이미지 빌드가
Apple Container builder snapshot 문제로 실패하면 기존 tenant 이미지를 base로
앱 산출물만 overlay build해서 같은 `imageTag`로 적용한다. fleet을 비워 두면
기존 단일 디바이스(`--host`/`--node`) 동작 그대로다.

## 외부 주소 (Cloudflare named 터널)

공유 Mattermost는 진입점 하나이고, 테넌트 10개는 그 호스트의 팀 경로
(`/tenant01`..`/tenant10`)다. 영구 주소는 기존 프로그래밍 명령으로 배선한다.
API 토큰은 파일 경로로만 넘겨 셸/로그에 노출하지 않는다.

```bash
internkim tenant sync-cloudflare-tunnel \
  --account-id <CF_ACCOUNT_ID> \
  --tunnel-id <CF_TUNNEL_ID> \
  --api-token-path .local/secrets/cloudflare-api-token \
  --hostname-template '{tenant}.intern.kim' \
  --tenants tenant01,...,tenant10
```

이후 Mattermost `ServiceSettings.SiteURL`을 공개 호스트로 맞춘다. Mattermost는
팀을 경로로 라우팅하고 SiteURL이 하나이므로, 서브도메인별 완전 분리가 필요하면
앞단에 가벼운 리버스 프록시가 한 겹 더 든다.

## 점검 명령 모음

```bash
container system status
container list
container image list
internkim tenant container status --workdir <DIR>
```

## 아직 안 된 것

- 터미널 toolchain(bun/uv/python)을 테넌트 이미지에 포함(채팅 루프엔 불필요,
  `terminal_run` 기반 작업에만 필요).
- POSIX 개인 격리 복원(이미지에 `blueclaw` 베이스 유저를 굽고 `posixHelperPath`
  복원 시 활성).
- arm64 Mattermost 이미지(현재 `10.5` 태그는 amd64라 에뮬레이션).
