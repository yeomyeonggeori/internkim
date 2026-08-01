# Site Scaffold v3 — Stateful Prototype Design

2026-07-08 확정. 목표: 저품질 모델도 헤매지 않고 멀티페이지·반응형·인터랙션·auth·CRUD 프로토타입을
만들 수 있게 한다. 인터랙티브한 모든 것은 dist에 미리 컴파일하고 모델은 선언만 한다.
정적 사이트는 오늘과 동일하게 남는다. 근거 검토: codex session 019f3d50-c909-7d90-b244-1696624abffe.

## 고정 결정

- 백엔드는 PocketBase 고정. 정적 사이트는 기동하지 않으며(마커 기반, 기존 동작), stateful 사이트는
  게이트웨이 lazy-start + 유휴 N분 후 stop으로 미사용 시 자원 0을 만든다.
- 동시 발행 캡 10. 초과 발행 시 admind가 발행 목록을 구조화 에러로 반환하고, 에이전트가
  ask_choice로 내릴 사이트를 사용자에게 고르게 한 뒤 unpublish→재발행한다. 기발행 사이트의
  업데이트는 캡과 무관.
- 라우팅은 해시 라우팅. admind SPA fallback으로 히스토리 라우팅도 가능하지만, 스캐폴드의 상대경로
  자산(./site-content.json, ./theme.css)과 `/api/**`,`/_/**` 예약 경로 충돌을 피하는 최저위험 선택.
- 권위 검증은 admind publish/preview(기존 DESIGN.md 검증과 같은 자리). 게스트의
  content_review.py는 빠른 수리를 위한 프리플라이트로 유지(항상 exit 0, 자문).
- pb 마이그레이션은 admind가 publish 시 manifest.collections에서 결정론 코드젠. 버전드 파일로만
  생성하고 적용된 파일은 절대 재작성하지 않는다(PocketBase `_migrations` 추적). 파괴적 스키마
  변경(필드 삭제/개명)은 명시적 정책 전까지 거부.
- 드래프트 프리뷰에 PocketBase 프록시를 확장한다(현재 servePublishedSite에만 있음). 없으면
  stateful 앱을 발행 전에 검증할 수 없다.
- 디자인 자유는 DESIGN.md가 담는다(테마 단일 진실, JSON에 중복 금지): 토큰 확장(style 프리셋,
  palette, typography, rounded/spacing/density, elevation, heroTreatment, motion) + 자유 서술 브리프
  (Style Prompt/Visual System/Signature Move). 원시 CSS 필드는 두지 않는다.
- 폰트는 김인턴 소유 카탈로그(assets/fonts/catalog.json, 라이선스 검증·voice/bestFor 서술).
  admind가 /fonts/로 셀프호스팅 서빙, 게스트엔 presentation vendoring용 투영만 프로비저닝.
  DESIGN.md fontFamily는 카탈로그 family만 유효(결정론 검증). 웹사이트 기본 서체는
  에이투지체(catalog defaultFor: website) — DESIGN.md가 지정하지 않으면 theme.css가 이걸 쓴다.

## Manifest v3 (site-content.json, 하위호환)

version/pages 부재 시 기존 {siteName, tagline, blocks}를 public "/" 단일 페이지로 정규화.

```json
{
  "version": 3,
  "siteName": "...", "tagline": "...",
  "navigation": { "items": [{ "label": "...", "href": "..." }] },
  "auth": { "enabled": false, "methods": ["password", "passkey"],
            "identity": "username | email", "userCollection": "users", "allowSignup": true,
            "loginPath": "/login", "signupPath": "/signup",
            "redirectAfterLogin": "/", "redirectAfterLogout": "/" },
  "collections": [{ "name": "...", "label": "...",
    "permissions": "publicReadAuthenticatedWrite | authenticatedCrud | ownerCrud",
    "fields": [{ "name": "...", "type": "text|number|boolean|date|email|url|select",
                 "required": false, "options": ["select 전용"] }] }],
  "pages": [{ "path": "/ 또는 /tasks/:id", "title": "...",
    "access": "public | authenticated",
    "blocks": [{ "variant": "hero|features|prose|cta|faq|contact|dataList|recordForm|recordDetail",
      "collection": "데이터 블록 필수", "display": "table|cards", "fields": ["..."],
      "recordIDParam": "id", "mode": "create|edit",
      "actions": ["create","view","edit","delete"] }] }],
  "effects": { "motion": "none | subtle" }
}
```

P1에서 이미 예약해야 하는 P2/P3 필드: page path(파라미터 포함)·access, auth 라우트,
collection/field 이름·타입, permissions 프리셋, recordIDParam. AuthGate는 모델이 쓰는 블록이
아니라 page.access가 구동하는 내부 래퍼. 데이터 블록은 empty/loading/error 상태와 delete 확인을
내장한다.

## 단계

- P0 (독립 선행 가능): 발행 캡 10 + 선택 플로우; 프리뷰 pb 프록시; pb lazy-start/idle-stop.
- P1: manifest v3 + 해시 라우터 + 내비 + 테마 토큰 확장 + 폰트 카탈로그 서빙/연결 + admind 검증.
  스캐폴드 CSS 전면 변수화(토큰이 실제 지배). 자산 경로 절대화.
- P2: auth 모듈(로그인/가입/세션/AuthGate) + page.access. 방식은 username/email+비밀번호(PB 내장)와
  passkey(WebAuthn) 둘 다 — passkey는 PB에 내장이 없으므로 우리가 프리베이크한 pb_hooks 번들로
  제공하고 manifest의 auth.methods 선언으로만 켠다(모델이 hooks를 직접 쓰지 않음).
- P3: collections 코드젠 + dataList/recordForm/recordDetail + 권한 프리셋.
- P4: 모션 프리셋, content_review pages[] 확장, eval 러너에 호스트측 발행 URL 스크린샷
  (데스크톱/모바일/페이지별) + 스크립트드 auth 스모크(가입→로그인→레코드 생성, 결정론).

각 단계는 skill-artifact-evals의 site 케이스로 회귀 측정한다(베이스라인 20260707T153809Z:
발행 완주·content_review 100, 단 정적 블록 한계로 예약 '흐름'은 표현 불가 — 이 설계의 존재 이유).

## 리스크 대장 (codex 검증)

- 히스토리 라우팅 시 상대경로 자산 오해석 → 해시 라우팅 + 절대경로화로 회피.
- `/api/**`,`/_/**`는 pb 예약 — 페이지 path 검증에서 거부.
- 적용된 마이그레이션 재작성은 무시됨 → 컬렉션 변경은 항상 새 버전 파일; 파괴적 변경 거부.
- pb_data는 버전 간 영속 → 스키마-데이터 정합은 마이그레이션만이 진실.
- 드래프트 프리뷰 pb 미프록시 → P0에서 확장.
- pb JS SDK 세션은 localStorage 기본 — 프로토타입 고지 문구에 명시.
