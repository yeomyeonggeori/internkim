# 셀프호스팅 & 노출(exposure) 구현 계획

> 상태: **폐기(2026-08-13)**. `nothing-reaches-in.md`가 대체한다.
> 이 문서가 풀려던 문제 — "직원이 어디서든 브라우저로 NAT 뒤 박스에 접속" — 은
> 중앙 플레인이 대신 답했다. 브라우저는 Pages와 Supabase로 가고 박스로 가지 않으므로,
> exposure 백엔드도 공개 도달점도 필요하지 않다. §3의 `EXPOSURE` 인터페이스와
> §4.3 wireguard-vps는 구현하지 않는다. 기록으로만 남긴다.
>
> 원래 상태: 설계 확정 · 미구현.
> 오늘(2026-07-30) buzz 릴레이 공개호스트 컷오버를 프로덕션 젯슨에 완료하며 도출됨.
> 관련 메모리: `project_buzz_relay_public_host_cutover`.

## 1. 비전

```
git clone …
cp .env.example .env      # 도메인, LLM 키, exposure 백엔드 설정값, 타깃 호스트 SSH …
./internkim selfhost      # 진짜 POSIX 리눅스 호스트를 구성 → 전 서비스 기동 → 끝
```

Blueclaw/internkim을 오픈소스 셀프호스팅 가능하게. "clone → env 채움 → 스크립트 한 방 → 셀프호스팅 완료".

## 2. 확정된 원칙 (이걸 깨는 설계는 반려)

1. **POSIX 유저/그룹/파일권한이 불가침 보안 경계다.** 배포 형태가 이걸 흐리면 안 됨.
   Blueclaw 터미널은 per-person `bc_person_<id>` 유저, circle `bc_circle_<id>` 그룹,
   `/workspace` POSIX 권한으로 실행 격리한다. Firecracker/bwrap은 **선택적 narrowing**(옵션),
   경계 자체는 POSIX. (CLAUDE.md "Blueclaw Terminal Permission Boundary" 참조)
2. **배포단위 = 진짜 POSIX 시스템**(호스트/VM/시스템-컨테이너). **Docker 앱-컨테이너를 기본으로 쓰지 않는다** —
   앱-컨테이너 이디엄(한 프로세스·ephemeral)은 per-person `useradd`+영속 workspace+다중 UID 서브프로세스와
   충돌하고, per-person 경계를 컨테이너 안으로 밀어넣어 모델을 2층으로 흐린다. 컨테이너가 꼭 필요하면
   **시스템-컨테이너(Incus/LXD/systemd-nspawn)** — 전체 POSIX 시스템을 우아하게 담는 것.
3. **Nix는 강제하지 않는다.** 유저-대면 메커니즘은 "POSIX 네이티브 바이너리-오케스트레이터".
   Nix는 (a) 빌드 hermetic용 flake, (b) 선언형 순수주의자용 NixOS 모듈 — **둘 다 선택**.
4. **SaaS 락인 금지 지향.** Cloudflare는 "쉬운 시작" 기본일 뿐, 자립 경로(wireguard-vps)가 형제로 있어야 함.

## 3. 노출(exposure)을 pluggable 컴포넌트로

### 요구사항이 옵션을 좁힌다
"직원이 **어디서든 아무 기기 브라우저로** NAT 뒤 셀프호스트 박스에 접속." 근본 제약:
- NAT/CGNAT 뒤 박스를 브라우저-공개하려면 **공개 도달점이 반드시 하나 필요**.
  선택지: ①SaaS 터널(CF) ②내 VPS 릴레이 ③박스 공인IP+포트포워딩. (셋 다 없이는 불가능.)
- **오버레이 VPN(ZeroTier/Tailscale/Nebula)은 브라우저-공개 "기본 경로"로 부적합** —
  모든 직원 기기에 클라이언트 설치+가입 필요 → zero-install 깨짐. 스태프-전용/네이티브앱 경로엔 적합.

### 백엔드 (`.env`의 `EXPOSURE=`)
| 값 | 성격 | 직원 설치 | SaaS | TLS |
|---|---|---|---|---|
| `cloudflare` | 쉬운 관리형 기본 | 없음 | CF | CF 엣지 |
| `wireguard-vps` | **SaaS-free 아름다운 기본** | 없음 | 없음(내 VPS+도메인) | Caddy+Let's Encrypt |
| `direct` | 공인IP+DDNS(고급) | 없음 | 없음 | Caddy+Let's Encrypt |
| `overlay` (nebula/headscale) | 스태프-전용/네이티브앱 | **있음** | self-host 가능 | 오버레이 내부 |

- **`cloudflare`**: cloudflared 터널 + CF API로 ingress/DNS 관리. 이미 `internal/tenantruntime/cloudflare_tunnel.go`에
  기존규칙 보존 read-modify-write + `upsertCNAME` 구현 존재 — 재사용.
- **`wireguard-vps`** (CF 터널의 self-owned 쌍둥이):
  ```
  직원 브라우저 → https://company.mydomain.com → [내 VPS: Caddy TLS 종단] → WireGuard → [박스: 서비스]
  ```
  박스가 VPS로 WG를 먼저 열어 NAT 관통, VPS Caddy가 Let's Encrypt TLS + 역프록시.
  직원은 브라우저만. 오픈소스(WireGuard/Caddy)+$5 VPS+도메인뿐. (frp/rathole은 대안, WG가 더 깔끔.)

## 4. 구현 로드맵 (순서)

### 4.1 CF ingress를 setup에 코드화 — `exposure=cloudflare` 첫 구현 ★ 오늘 컷오버의 마지막 조각
- 릴레이 공개호스트(`<tenant>-relay.example.test`, 그리고 admind 호스트)를 CF 터널 ingress에 추가 + DNS CNAME.
- `internal/tenantruntime/cloudflare_tunnel.go`의 ingress-보존 sync + `upsertCNAME` 재사용.
- origin: `https://127.0.0.1:443`(stunnel), `originRequest.noTLSVerify=true`. (relay 직결 `http://127.0.0.1:3000`도 가능하나
  내부 stunnel 경로와 동일화하려 stunnel 채택 — NIP-98 스킴은 relay `RELAY_URL`(wss→https)에서 나옴.)
- 토큰: `.env`(현재 `.local/secrets/<cf>-token`). 스코프: tunnel config edit + DNS edit 필요.
- setup 스텝으로: 새 `StepExposure`(또는 tunnel 스텝 확장)가 `EXPOSURE` 값에 따라 백엔드 실행.
- **주의**: CF `PUT /configurations`는 ingress 전체 교체 → 기존 규칙(admind 호스트 등) 반드시 보존.
  (cloudflare_tunnel.go가 이미 이걸 함.)

### 4.2 exposure를 인터페이스로 추출
- `ExposureBackend` 인터페이스: `EnsureReachable(hostnames []Hostname, origins) error` / `Teardown`.
- 구현: `cloudflareExposure`, `wireguardVPSExposure`, `directExposure`, `overlayExposure`.
- `EXPOSURE` env로 선택. setup 스텝이 선택된 백엔드 호출.

### 4.3 `wireguard-vps` 백엔드
- 박스: WireGuard 피어 설정(persistent keepalive, VPS를 엔드포인트로).
- VPS: Caddy(도메인별 Let's Encrypt) + 역프록시 → WG 링크 너머 박스 서비스. cloudflared 불필요.
- VPS 프로비저닝도 `internkim`이 SSH로: WG 설치 + Caddyfile 생성 + systemd.
- `.env`: `VPS_HOST`, `VPS_SSH`, `DOMAIN`, WG 키(자동생성).

### 4.4 단일 진입점 + `.env.example`
- `.env.example`: 전 필수값(도메인, LLM 프로바이더 키, EXPOSURE + 백엔드별 값, 타깃 호스트 SSH, 회사/people)
  + 주석 + 시작 시 검증(빠진 값 loud fail).
- `./internkim selfhost [--host <ip>]`: env 로드 → exposure 프로비전 → step provisioner → 검증. 멱등.

### 4.5 호스티드 fleet 백엔드 디커플
- 현재 setup은 example.test fleet 백엔드(fleet-id/secret/api-url, `0.ssh.<id>.example.test` 터널·DNS 발급)에 의존.
- 셀프호스터는 이게 없음 → fleet 등록을 **옵션화**, `EXPOSURE`가 사용자 CF/VPS로 **직접** 터널·DNS 발급.
- 단일 노드 셀프호스트가 1급 시민이 되게(현재는 fleet 노드 모델 중심).

### 4.6 타깃 제네릭화
- Jetson 전용 스텝(L4T, Firecracker, CUDA llama.cpp)을 프로파일 뒤로.
- `generic-linux-host` board type: Debian/Ubuntu systemd 호스트에 SSH로 구성. (SSH 백엔드 이미 존재.)
- LLM 기본 = 리모트(OpenRouter). 로컬 CUDA LLM + Firecracker = 엣지/젯슨 프로파일 opt-in.

### 4.7 선택적 배포 형태 (core 아님, 나중)
- **Nix flake(빌드)**: Go/bun/arm64 cross/llama.cpp 툴체인 핀 → 애드혹 컨테이너 빌더 대체. 유저 실행엔 불필요.
- **NixOS 모듈**: 전체 POSIX 시스템을 선언형으로. 가장 재현적(단 유저가 NixOS).
- **어플라이언스 이미지**: VM 또는 시스템-컨테이너(Incus/LXD) 이미지. 무설정 포터블, 내부는 네이티브 POSIX.

## 5. 재사용 가능한 기존 자산

- `internal/tenantruntime/cloudflare_tunnel.go` — CF 터널 ingress(기존규칙 보존) + DNS `upsertCNAME` API 관리. (PoC 테넌트용, 재사용)
- `internal/provisioning/steps/` — 멱등 step provisioner (SSH/SD 백엔드), `steps_stub.go` Registry.
- `internal/cli/main.go` — `.env` 로딩.
- PoC(Apple Container / poc-container kind) — 컨테이너화 선례(단 앱-컨테이너 아닌 시스템 성격).
- buzz 릴레이 공개호스트 컷오버 — `StepBuzzPublicHost`(stunnel/cert/hosts/rekey/drop-in)가
  "호스트를 공개 도달가능하게" 구성하는 패턴의 첫 사례.

## 6. 제약/함정

- **POSIX 경계 유지**(§2.1). 컨테이너화하면 시스템-컨테이너로, per-person 유저를 그 안에 네이티브로.
- **CGNAT 현실**: 홈 ISP는 인바운드 불가 다수 → `direct`는 종종 불가, `cloudflare`/`wireguard-vps`가 현실 답.
- **브라우저 TLS 필수** → 어느 백엔드든 유효 인증서(CF 엣지 or Caddy+Let's Encrypt).
- **CF `PUT /configurations` 전체 교체** → ingress 병합 필수(§4.1).
- 배포 위생: 브랜치가 origin/main 포함하는지 확인 후 빌드(회귀 방지). 배포 후 실제 리비전 검증.
