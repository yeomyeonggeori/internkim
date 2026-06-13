# PoC 호스트 운영 (맥스튜디오 멀티테넌트)

상시 켜져 있는 맥스튜디오 한 대에서 여러 회사(팀)에 김인턴을 보급하는 구조와 운영 절차입니다.

## 구조

```
맥스튜디오 (macOS, container CLI)
└── apple/container 리눅스 VM 1대 (기본 이름: internkim-pilot-01, --virtualization으로 중첩 KVM)
    ├── 테넌트 acme  → internkim-tenant-{admind,blueclaw,capabilityd,graphiti}-acme.service
    │                  + internkim-mattermost-acme.service  →  https://acme.intern.kim
    ├── 테넌트 bravo → 같은 서비스 세트                      →  https://bravo.intern.kim
    └── ...
```

- 테넌트는 cloud-shared 호스트 런타임 프로파일로 실행됩니다. 코드와 바이너리는 공유하되
  runtime state, workspace, bot token, admin secret, LLM device token은 테넌트별로 분리됩니다.
- 테넌트 상태는 VM 안 `/srv/internkim/tenants/<tenant>/`에 저장됩니다.
- 공개 주소는 Cloudflare named tunnel과 `{tenant}.intern.kim` hostname template로 발급됩니다.
- Blueclaw는 테넌트마다 Firecracker guest로 뜨므로 VM에 `/dev/kvm`(중첩 가상화)이 필요합니다.
  Apple Silicon M3 이상 + 최신 macOS에서 동작하며, `container run --virtualization`이 노출합니다.

## 최초 1회: 호스트 준비

```bash
# 1. container CLI 설치 후 VM 생성 (ubuntu 24.04 + systemd 부트스트랩 + CLI 주입까지 자동)
./internkim host init                # 기본: internkim-pilot-01, 4 CPU, 6g
./internkim host init --vm poc-host --cpus 8 --memory 16g

# 2. VM 안 테넌트 전제조건 설치 (현재 수동)
#    - internkim 바이너리 세트: internkim-admind, internkim-capabilityd, blueclaw-supervisor,
#      graphiti-memoryd 등 /usr/local/bin
#    - Blueclaw 런타임 페이로드: /opt/internkim/blueclaw-runtime/{rootfs.ext4,workspace.ext4,vmlinux.bin}
#      (make prepare-blueclaw-payload 산출물)
#    - Mattermost 서버: /opt/mattermost + PostgreSQL
#    - cloudflared + named tunnel 자격(account ID, tunnel ID, API token)

# 3. 상태 확인
./internkim host status
```

`host init`은 멱등합니다. VM이 이미 떠 있으면 생성을 건너뛰고 CLI 동기화만 다시 합니다.
첫 부팅 시 VM 네트워크가 늦게 올라오면 부트스트랩이 안에서 재시도하므로,
타임아웃이 나도 `container logs <vm>` 확인 후 `host init`을 다시 실행하면 이어집니다.

## 팀 추가

CLI 한 방:

```bash
./internkim host add-team --team acme \
  --display-name "에이크미" \
  --member kim@acme.com:김대표 \
  --member lee@acme.com:이실장:초기비번 \
  -- --hostname-template '{tenant}.intern.kim' \
     --account-id "$CF_ACCOUNT_ID" --tunnel-id "$TUNNEL_ID" \
     --api-token-path /root/.internkim/secrets/cloudflare-api-token
```

- `--member 이메일:이름[:비밀번호]` — 비밀번호를 생략하면 자동 생성되어 결과 JSON에 포함됩니다.
- 멤버는 Mattermost 계정 생성 + internkim 팀 합류 + Blueclaw people 초대까지 한 번에 처리됩니다.
  이미 존재하는 이메일은 팀 합류만 보장하고 건너뜁니다(멱등).
- `--` 뒤 플래그는 VM 안 `internkim tenant provision --runtime host`로 그대로 전달됩니다.
- 내부적으로 create → install-host-runtime → bootstrap-mattermost(멤버 포함) → sync-cloudflare-tunnel을
  체이닝하며, 실패 시 어느 단계에서 멈췄는지와 재개 명령을 출력합니다.

## 콘솔 페이지

```bash
./internkim host console                          # http://127.0.0.1:9090
./internkim host console --auth-token <token>     # 외부 노출 시 Bearer 토큰 요구
```

- 팀 목록(팀 ID / 주소 / 상태)과 `팀 추가` 폼(팀 ID, 표시 이름, 멤버 행 추가식 입력)을 제공합니다.
- 제출하면 백그라운드 잡으로 provision이 실행되고 진행 로그가 표시되며,
  완료 시 발급 주소와 자동 생성된 비밀번호가 복사 버튼과 함께 나타납니다.
- 각 팀 행의 `삭제` 버튼은 팀 ID를 직접 입력해 확인한 뒤 제거 잡을 실행합니다.
- 잡은 한 번에 하나만 실행됩니다(동시 요청은 409).
- 기본 바인딩은 localhost입니다. localhost 밖으로 열 때만 `--auth-token`을 쓰세요.

## 팀 제거

```bash
./internkim host remove-team --team acme \
  -- --account-id "$CF_ACCOUNT_ID" --tunnel-id "$TUNNEL_ID" \
     --api-token-path /root/.internkim/secrets/cloudflare-api-token
```

- 테넌트 systemd 유닛 중지·제거, Cloudflare 터널 ingress 제거(터널 플래그 제공 시),
  `/srv/internkim/tenants/<tenant>/` 삭제를 수행하고 제거/건너뜀 리포트를 JSON으로 출력합니다.
- VM 안에서 직접 실행할 때는 `internkim tenant remove --tenant <id> --confirm <id>`로
  팀 ID를 한 번 더 입력해야 합니다.

## 점검 명령 모음

```bash
./internkim host status                                  # VM + 테넌트 목록/주소/상태
container exec <vm> internkim tenant status --tenant <id>
container exec <vm> systemctl status internkim-tenant-blueclaw-<id>.service
container logs <vm>                                      # VM 부트스트랩 로그
```

## 고객 오픈 전 체크리스트

- 해당 테넌트 admin 계정이 `internkim` 팀 멤버인지
- `/flow`, `/calendar`, `/attendance`, `/_app` 라우트가 그 테넌트의 admind로 가는지
- 그 테넌트의 Blueclaw/capabilityd가 같은 테넌트의 Mattermost bot token과
  `LLM_DEVICE_TOKEN`으로 기동 중인지
- `@internkim` 테스트 포스트에 응답이 오는지 확인 후 원문과 응답 삭제
