# Buzz↔Mattermost 양방향 미러 활성화 레시피

> 상태: 메커니즘·레시피 100% 확정, 미실행(2026-07-31 인시던트 세션 말미, 컨텍스트 소진으로 다음 세션 이월).
> 목표(사용자): MM/Buzz/웹 어디서 쓰든 서로 다 싱크(양방향).
> 관련: [[project_buzz_relay_public_host_cutover]], project_buzz_mirror_live_wiring.

## 배경 / 왜 지금 미완인가
- 오늘 릴레이 공개호스트 컷오버 인시던트 + admind DB통합 세션충돌 복구로 컨텍스트 소진.
- 미러 프레임워크는 chatd에 이미 존재(`.dependency/blueclaw/chatd/src/mirror/`: wire.ts, orchestrator.ts, mattermost-puppet.ts, buzz-publisher.ts, echo-suppressor.ts). 현재 배포된 chatd는 **buzz-only**라 미러 미wire.
- SSH(cloudflared interactive) 경로가 젯슨 WiFi에서 배너 타임아웃으로 불안정 → chatd 배포는 **admind HTTP 복구액션 우회** 권장(setup 대신). admind OTA/HTTP는 안정적.

## 메커니즘 (확정)
- `chatd/src/main.ts`: `configuration.admindBaseURL && configuration.buzz`면 `createMirror(...)` 생성.
- `wire.ts`: `mattermost.adminToken`이 있을 때만 MM 게이트웨이 생성 → **admin 토큰 없으면 MM→Buzz 단방향만**. 양방향엔 admin 토큰 필수.
- `mattermost-puppet.ts`: **admin 토큰으로 각 유저의 PAT를 `POST /api/v4/users/{userId}/tokens`로 발급·캐시**해서 그 유저로 포스팅(진짜 per-user 퍼펫팅). 링크된 이메일 없는 author는 드롭. → **MM `ServiceSettings.EnableUserAccessTokens=true` 필요**.
- `chatd/src/configuration.ts`: MM 켜면 `CHATD_MATTERMOST_BOT_TOKEN` 필수(`requireValue`→없으면 throw), `CHATD_BLUECLAW_INGRESS_URL` 필수(없으면 throw). **빈 값 주입 시 chatd 크래시=메신저 다운** → 반드시 값 검증 후 주입.

## 레시피 (새 세션에서 실행)

### 0. 선행 — 디바이스별 미지값 확보(현재 미상, 먼저 조사)
- MM admin 로그인 자격: admin 이메일/유저명 + 비번 파일 경로. (admind config: `MattermostAdminPasswordPath`, `ClaimedAdminEmailPath`/`AdminEmailPath` 확인. MM 유저명은 보통 `admin`.)
- 봇 토큰: `/root/.internkim/secrets/mattermost-bot-token` (=`BlueclawMattermostTokenPath`).
- 값들: MM base=`http://127.0.0.1:8065`, blueclaw ingress=`http://127.0.0.1:8080`.

### 1. admind 복구액션 `enable-buzz-mirror` 추가 (recovery.go), background context로 실행
가드 필수(하나라도 실패하면 드롭인 안 쓰고 중단):
```
set +e
MM=http://127.0.0.1:8065
# admin 세션토큰 (login_id=admin 이메일/유저명, password=파일)
TOKEN=$(curl -s -D- -o /dev/null -X POST $MM/api/v4/users/login \
  -d "{\"login_id\":\"<ADMIN>\",\"password\":\"$(cat <ADMIN_PASS_PATH>)\"}" \
  | awk 'tolower($1)=="token:"{print $2}' | tr -d "\r")
[ -n "$TOKEN" ] || { echo "admin login failed"; exit 1; }
# PAT 발급 허용
curl -s -X PUT $MM/api/v4/config/patch -H "Authorization: Bearer $TOKEN" \
  -d '{"ServiceSettings":{"EnableUserAccessTokens":true}}' >/dev/null
ADMIN_ID=$(curl -s $MM/api/v4/users/me -H "Authorization: Bearer $TOKEN" | jq -r .id)
ADMIN_PAT=$(curl -s -X POST $MM/api/v4/users/$ADMIN_ID/tokens -H "Authorization: Bearer $TOKEN" \
  -d '{"description":"chatd mirror admin"}' | jq -r .token)
[ -n "$ADMIN_PAT" ] && [ "$ADMIN_PAT" != null ] || { echo "PAT create failed"; exit 1; }
BOT=$(cat /root/.internkim/secrets/mattermost-bot-token)
[ -n "$BOT" ] || { echo "bot token missing"; exit 1; }
# chatd 드롭인 (base unit의 buzz 설정 보존, MM만 추가 → 반전 가능)
mkdir -p /etc/systemd/system/chatd.service.d
cat > /etc/systemd/system/chatd.service.d/mirror.conf <<EOF
[Service]
Environment=CHATD_MATTERMOST_BASE_URL=$MM
Environment=CHATD_MATTERMOST_BOT_TOKEN=$BOT
Environment=CHATD_MATTERMOST_ADMIN_TOKEN=$ADMIN_PAT
Environment=CHATD_BLUECLAW_INGRESS_URL=http://127.0.0.1:8080
EOF
systemctl daemon-reload
systemctl restart chatd
sleep 3
# auto-rollback: chatd 안 뜨면 드롭인 제거하고 복구
if [ "$(systemctl is-active chatd)" != active ]; then
  rm -f /etc/systemd/system/chatd.service.d/mirror.conf
  systemctl daemon-reload; systemctl restart chatd
  echo "chatd failed with mirror config; rolled back"; exit 1
fi
echo "mirror enabled; chatd active"
```
- 항구 토큰이므로 PAT(세션토큰 아님) 사용. `jq` 디바이스에 있음(다른 recovery가 사용).

### 2. 배포/트리거
- `deploy --components admind --fleet zd2df6qt6jmc` (OTA/HTTP, SSH 불필요) → `recover ssh --action enable-buzz-mirror`.

### 3. 검증 (인증 불필요, 백엔드)
- Buzz 웹에서 테스트 메시지 → MM API로 해당 채널에 **그 유저 이름으로** 뜨는지 확인.
- MM에서 메시지 → Buzz 웹/relay에 뜨는지.
- **루프 없나**: 한 메시지가 왕복 증식 안 하는지(echo-suppressor). 처음엔 **테스트 채널 1개**로만 확인 후 확대 판단.
- chatd 저널: `[mirror]` 에러 없나.

### 4. 코드화 (검증 후)
- 임시 드롭인 대신 `ChatdServiceUnit`(blueclaw_service.go)에 MM env를 정식 추가 + admin PAT를 시크릿으로 관리 → reprovision 생존.

## 리스크
- **chatd 크래시 = 메신저 다운**: 위 가드+auto-rollback 필수. 빈 토큰 주입 금지.
- **dual-platform 재활성화**: chatd가 MM 메시지도 처리 → 봇이 MM글에 이중반응 가능성. 검증 시 확인.
- **루프**: echo-suppressor 신뢰 전 1채널 스테이징.
- **EnableUserAccessTokens=true**는 MM-wide 설정 변경(보안 표면). 전환 완료 후 MM 폐기 시 무의미해지므로 임시.
