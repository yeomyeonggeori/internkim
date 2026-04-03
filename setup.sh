#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$SCRIPT_DIR/bin"

BOARD_IP=""
BOARD_USER="root"
BOARD_PASS="root"
USB_NCM_CANDIDATES=("10.11.60.1")
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 -o LogLevel=ERROR"

LANG_EN=false
[[ "$1" == "--en" ]] && LANG_EN=true

msg() { if $LANG_EN; then echo "$2"; else echo "$1"; fi; }

echo "=== Quick Claw Wi-Fi Setup ==="
echo ""

# 1. USB NCM 연결 확인
msg "[1/4] 보드 연결 확인 중..." "[1/4] Detecting board..."
for ip in "${USB_NCM_CANDIDATES[@]}"; do
    if ping -c 1 -W 2 "$ip" &>/dev/null; then
        BOARD_IP="$ip"
        break
    fi
done

if [ -z "$BOARD_IP" ]; then
    msg "보드를 찾을 수 없습니다." "Board not found."
    msg "USB-C 데이터 케이블로 보드와 컴퓨터를 연결하고 30초 기다린 후 다시 시도하세요." \
        "Connect the board with a USB-C data cable and wait 30 seconds."
    exit 1
fi

msg "보드 발견: $BOARD_IP" "Board found: $BOARD_IP"

# 2. Wi-Fi SSID / 비밀번호 자동 감지
echo ""
msg "[2/4] Wi-Fi 정보 감지 중..." "[2/4] Detecting Wi-Fi..."
WIFI_SSID=$("$BIN_DIR/get-ssid" 2>/dev/null || echo "")

if [ -z "$WIFI_SSID" ] || [[ "$WIFI_SSID" == *"Unknown"* ]]; then
    msg "Wi-Fi에 연결되어 있지 않습니다." "Not connected to Wi-Fi."
    exit 1
fi

echo "SSID: $WIFI_SSID"

msg "키체인 접근 팝업이 뜨면 맥 계정/비밀번호를 입력하세요." \
    "Enter your Mac credentials when the keychain popup appears."
WIFI_PASS=$(security find-generic-password -D "AirPort network password" -wa "$WIFI_SSID" 2>/dev/null || echo "")

if [ -z "$WIFI_PASS" ]; then
    msg "키체인에서 비밀번호를 가져올 수 없습니다." "Failed to retrieve password from keychain."
    exit 1
fi

PASS_MASK=$(printf '*%.0s' $(seq 1 ${#WIFI_PASS}))
msg "비밀번호: $PASS_MASK" "Password: $PASS_MASK"

# 3. 보드에 Wi-Fi 설정 전달
echo ""
msg "[3/4] 보드에 Wi-Fi 설정 중..." "[3/4] Configuring Wi-Fi on board..."

"$BIN_DIR/sshpass" -p "$BOARD_PASS" ssh $SSH_OPTS "${BOARD_USER}@${BOARD_IP}" bash -s <<REMOTE
killall wpa_supplicant 2>/dev/null || true

cat > /etc/wpa_supplicant.conf <<EOF
ctrl_interface=/var/run/wpa_supplicant
ap_scan=1
network={
  ssid="$WIFI_SSID"
  scan_ssid=1
  key_mgmt=WPA-PSK
  psk="$WIFI_PASS"
}
EOF

wpa_supplicant -i wlan0 -c /etc/wpa_supplicant.conf -B
sleep 2
udhcpc -i wlan0 -q -n -t 5 -T 3 2>/dev/null || true
sleep 3

WIFI_IP=\$(ip -4 addr show wlan0 2>/dev/null | grep 'inet ' | awk '{print \$2}' | cut -d/ -f1)
echo "WIFI_IP=\$WIFI_IP"
REMOTE

# 4. 결과 확인
echo ""
msg "[4/4] Wi-Fi 연결 확인 중..." "[4/4] Verifying Wi-Fi connection..."

WIFI_IP=$("$BIN_DIR/sshpass" -p "$BOARD_PASS" ssh $SSH_OPTS "${BOARD_USER}@${BOARD_IP}" \
    "ip -4 addr show wlan0 2>/dev/null | grep 'inet ' | awk '{print \$2}' | cut -d/ -f1")

if [ -n "$WIFI_IP" ]; then
    echo ""
    msg "=== Wi-Fi 연결 성공 ===" "=== Wi-Fi Connected ==="
    echo "Wi-Fi IP: $WIFI_IP"
    echo ""
    msg "이제 Wi-Fi로 접속할 수 있습니다:" "You can now connect via Wi-Fi:"
    echo "  ssh root@$WIFI_IP"
else
    echo ""
    msg "=== Wi-Fi 연결 실패 ===" "=== Wi-Fi Connection Failed ==="
    msg "SSID와 비밀번호를 확인하세요." "Check your SSID and password."
    msg "USB 연결로 직접 디버깅하려면:" "To debug via USB:"
    echo "  ssh root@$BOARD_IP"
    exit 1
fi
