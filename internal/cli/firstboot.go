package cli

import (
	"fmt"
	"strings"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

func buildFirstbootScript(deviceURL, adminEmail string, isLocalLlamaProvisioned bool) string {
	sections := []string{
		renderFirstbootPreludeSection(),
		renderFirstbootStagingSection(),
		renderFirstbootNetworkSection(),
		renderFirstbootPackagesSection(),
		renderFirstbootToolsSection(),
		renderFirstbootApplicationsSection(deviceURL, adminEmail),
		renderFirstbootServicesSection(isLocalLlamaProvisioned),
		renderFirstbootCompletionSection(),
	}

	return strings.Join(sections, "\n\n") + "\n"
}

func renderFirstbootPreludeSection() string {
	return strings.TrimSpace(`#!/bin/bash
set -euo pipefail
PS4='+ line ${LINENO}: '
set -x
exec > /var/log/internkim-firstboot.log 2>&1

cleanup() {
  local returnCode=$?
  mountpoint -q /boot/firmware || mount /boot/firmware 2>/dev/null || true
  mkdir -p /boot/firmware/internkim
  cp -f /var/log/internkim-firstboot.log /boot/firmware/internkim/firstboot.log 2>/dev/null || true
  sync
  if [ $returnCode -eq $RETRYABLE_EXIT ]; then
    echo "Firstboot will retry automatically (exit $returnCode)"
  fi
  if [ $returnCode -ne 0 ] && [ $returnCode -ne $RETRYABLE_EXIT ] && [ -f /sys/class/leds/ACT/trigger ]; then
    echo timer > /sys/class/leds/ACT/trigger 2>/dev/null || true
    echo 1000 > /sys/class/leds/ACT/delay_on 2>/dev/null || true
    echo 1000 > /sys/class/leds/ACT/delay_off 2>/dev/null || true
  fi
}

trap cleanup EXIT
trap 'returnCode=$?; echo "ERROR: line ${LINENO}: ${BASH_COMMAND} (exit ${returnCode})"' ERR

echo "=== Intern Kim first-boot provisioning ==="
echo "Build: $(cat /boot/firmware/internkim/build-id 2>/dev/null || echo unknown)"
date

RETRYABLE_EXIT=75
STATE_DIRECTORY=/var/lib/internkim-firstboot
PHASE_DIRECTORY="$STATE_DIRECTORY/phases"
mkdir -p "$PHASE_DIRECTORY"

phase_done() {
  [ -f "$PHASE_DIRECTORY/$1.done" ]
}

mark_phase_done() {
  mkdir -p "$PHASE_DIRECTORY"
  : > "$PHASE_DIRECTORY/$1.done"
  sync
}

start_phase() {
  local phaseName="$1"
  echo "=== phase: $phaseName ==="
  mountpoint -q /boot/firmware || mount /boot/firmware 2>/dev/null || true
  printf '%s' "$phaseName" > /boot/firmware/internkim/current-phase 2>/dev/null || true
}

retry_later() {
  echo "RETRYABLE: $1"
  exit $RETRYABLE_EXIT
}

save_board_ip() {
  local boardIPAddress
  boardIPAddress="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  echo "SSH started ($boardIPAddress)"
  mountpoint -q /boot/firmware || mount /boot/firmware 2>/dev/null || true
  if [ -n "$boardIPAddress" ] && [ -d /boot/firmware/internkim ]; then
    printf '%s' "$boardIPAddress" > /boot/firmware/internkim/board-ip
    sync
  fi
}

log_network_diagnostics() {
  wpa_cli -i wlan0 status 2>&1 || true
  ip addr show wlan0 2>&1 || true
  journalctl -u wpa_supplicant@wlan0 --no-pager -n 20 2>&1 || true
  journalctl -u systemd-networkd --no-pager -n 20 2>&1 || true
}

wait_for_ipv4() {
  local timeoutSeconds="${1:-120}"
  local attempts=$((timeoutSeconds / 2))
  local attemptIndex
  for attemptIndex in $(seq 1 "$attempts"); do
    if ip -4 addr show wlan0 2>/dev/null | grep -q 'inet '; then
      echo "IPv4 ready (attempt $attemptIndex)"
      return 0
    fi
    sleep 2
  done
  return 1
}

ensure_dns() {
  local dhcpNameservers
  dhcpNameservers="$(networkctl status wlan0 2>/dev/null | awk '/DNS:/{for(i=2;i<=NF;i++) print "nameserver "$i}' || true)"
  systemctl stop systemd-resolved 2>/dev/null || true
  systemctl disable systemd-resolved 2>/dev/null || true
  rm -f /etc/resolv.conf
  if [ -n "$dhcpNameservers" ]; then
    echo "$dhcpNameservers" > /etc/resolv.conf
    echo "nameserver 8.8.8.8" >> /etc/resolv.conf
  else
    cat > /etc/resolv.conf <<DNSEOF
nameserver 8.8.8.8
nameserver 1.1.1.1
DNSEOF
  fi
  chmod 644 /etc/resolv.conf
  echo "DNS fixed: $(tr '\n' ' ' < /etc/resolv.conf)"
}

wait_for_egress() {
  local timeoutSeconds="${1:-90}"
  local attempts=$((timeoutSeconds / 3))
  local attemptIndex
  for attemptIndex in $(seq 1 "$attempts"); do
    if curl -4skf --connect-timeout 5 https://clients3.google.com/generate_204 >/dev/null 2>&1; then
      echo "Egress ready (attempt $attemptIndex)"
      return 0
    fi
    sleep 3
  done
  return 1
}

sync_time_safely() {
  local httpDate
  local currentYear
  local attemptIndex
  httpDate="$(curl -4skI --connect-timeout 5 https://google.com 2>/dev/null | awk 'BEGIN{IGNORECASE=1} /^date:/{sub(/^[Dd]ate: /, ""); print; exit}' || true)"
  if [ -n "$httpDate" ]; then
    date -s "$httpDate" 2>/dev/null || true
    echo "Time set from HTTP: $(date)"
    return 0
  fi
  timedatectl set-ntp true 2>/dev/null || true
  for attemptIndex in $(seq 1 15); do
    currentYear=$(date +%Y)
    if [ "$currentYear" -ge 2026 ]; then
      echo "Time synced via NTP: $(date)"
      return 0
    fi
    sleep 2
  done
  return 1
}`)
}

func renderFirstbootStagingSection() string {
	return strings.TrimSpace(`# ── Skip if filesystem not yet resized (Armbian resizes across 2 boots) ──
ROOT_SIZE=$(df --output=size / 2>/dev/null | tail -1 | tr -d ' ' || echo 0)
if [ "${ROOT_SIZE:-0}" -lt 5000000 ]; then
  echo "Waiting for filesystem resize (${ROOT_SIZE}K). Will run on next boot."
  trap - EXIT
  sync
  exit 0
fi

# ── LED: fast blink while provisioning, heartbeat when done ──
if [ -f /sys/class/leds/ACT/trigger ]; then
  echo timer > /sys/class/leds/ACT/trigger 2>/dev/null || true
  echo 100 > /sys/class/leds/ACT/delay_on 2>/dev/null || true
  echo 100 > /sys/class/leds/ACT/delay_off 2>/dev/null || true
fi

STAGE="/boot/firmware/internkim"
if [ ! -d "$STAGE" ]; then
  echo "ERROR: staging directory $STAGE not found"
  exit 1
fi

echo "Installing staged files from boot partition..."

mkdir -p /root/.ssh
if [ -f "$STAGE/authorized_keys" ]; then
  cp -f "$STAGE/authorized_keys" /root/.ssh/authorized_keys
  chmod 600 /root/.ssh/authorized_keys
fi

echo "internkim" > /etc/hostname
hostname internkim

mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/internkim.conf <<'SSHEOF'
PermitRootLogin yes
PasswordAuthentication no
SSHEOF
systemctl restart sshd 2>/dev/null || true

if [ -f "$STAGE/wpa_supplicant.conf" ]; then
  mkdir -p /etc/wpa_supplicant
  cp -f "$STAGE/wpa_supplicant.conf" /etc/wpa_supplicant/wpa_supplicant.conf
  chmod 600 /etc/wpa_supplicant/wpa_supplicant.conf
fi

for binaryPath in "$STAGE"/bin/*; do
  [ -f "$binaryPath" ] || continue
  cp -f "$binaryPath" /usr/local/bin/
  chmod 755 "/usr/local/bin/$(basename "$binaryPath")"
done

mkdir -p /root/.internkim/secrets /root/.internkim/env /root/.internkim/config
for secretPath in "$STAGE"/secrets/*; do
  [ -f "$secretPath" ] || continue
  cp -f "$secretPath" /root/.internkim/secrets/
  chmod 600 "/root/.internkim/secrets/$(basename "$secretPath")"
done
if [ -f "$STAGE/admin-email" ]; then
  cp -f "$STAGE/admin-email" /root/.internkim/config/admin-email
  chown root:root /root/.internkim/config/admin-email
  chmod 600 /root/.internkim/config/admin-email
fi
if [ -f "$STAGE/device-url" ]; then
  cp -f "$STAGE/device-url" /root/.internkim/env/device-url
  chown root:root /root/.internkim/env/device-url
  chmod 640 /root/.internkim/env/device-url
fi
if [ -f "$STAGE/mattermost-url" ]; then
  cp -f "$STAGE/mattermost-url" /root/.internkim/env/mattermost-url
  chown root:root /root/.internkim/env/mattermost-url
  chmod 640 /root/.internkim/env/mattermost-url
fi
if [ -f "$STAGE/tunnel-origin" ]; then
  cp -f "$STAGE/tunnel-origin" /root/.internkim/env/tunnel-origin
  chown root:root /root/.internkim/env/tunnel-origin
  chmod 640 /root/.internkim/env/tunnel-origin
fi
if [ -f "$STAGE/tunnel-revision" ]; then
  cp -f "$STAGE/tunnel-revision" /root/.internkim/env/tunnel-revision
  chown root:root /root/.internkim/env/tunnel-revision
  chmod 640 /root/.internkim/env/tunnel-revision
fi
if [ -f "$STAGE/tls-certificate-status" ]; then
  cp -f "$STAGE/tls-certificate-status" /root/.internkim/env/tls-certificate-status
  chown root:blueclaw /root/.internkim/env/tls-certificate-status
  chmod 640 /root/.internkim/env/tls-certificate-status
fi
if [ -f "$STAGE/fleet-id" ]; then
  cp -f "$STAGE/fleet-id" /root/.internkim/env/fleet-id
  chown root:blueclaw /root/.internkim/env/fleet-id
  chmod 640 /root/.internkim/env/fleet-id
fi
if [ -f "$STAGE/node-id" ]; then
  cp -f "$STAGE/node-id" /root/.internkim/env/node-id
  chown root:blueclaw /root/.internkim/env/node-id
  chmod 640 /root/.internkim/env/node-id
fi
if [ -f "$STAGE/fleet-role" ]; then
  cp -f "$STAGE/fleet-role" /root/.internkim/env/fleet-role
  chown root:blueclaw /root/.internkim/env/fleet-role
  chmod 640 /root/.internkim/env/fleet-role
fi
if [ -f "$STAGE/fleet-active-count" ]; then
  cp -f "$STAGE/fleet-active-count" /root/.internkim/env/fleet-active-count
  chown root:blueclaw /root/.internkim/env/fleet-active-count
  chmod 640 /root/.internkim/env/fleet-active-count
fi
if [ -f "$STAGE/fleet-pending-count" ]; then
  cp -f "$STAGE/fleet-pending-count" /root/.internkim/env/fleet-pending-count
  chown root:blueclaw /root/.internkim/env/fleet-pending-count
  chmod 640 /root/.internkim/env/fleet-pending-count
fi
if [ -f "$STAGE/fleet-quorum-size" ]; then
  cp -f "$STAGE/fleet-quorum-size" /root/.internkim/env/fleet-quorum-size
  chown root:blueclaw /root/.internkim/env/fleet-quorum-size
  chmod 640 /root/.internkim/env/fleet-quorum-size
fi
if [ -f "$STAGE/api-url" ]; then
  cp -f "$STAGE/api-url" /root/.internkim/env/api-url
  chown root:blueclaw /root/.internkim/env/api-url
  chmod 640 /root/.internkim/env/api-url
fi

mkdir -p /root/.blueclaw/config /root/.blueclaw/workspace
if [ -f "$STAGE/config/runtime.json" ]; then
  cp -f "$STAGE/config/runtime.json" /root/.blueclaw/config/runtime.json
fi
if [ -f "$STAGE/config/policy.json" ]; then
  cp -f "$STAGE/config/policy.json" /root/.blueclaw/config/policy.json
fi
if [ -d "$STAGE/blueclaw-migrations" ]; then
  rm -rf /root/.blueclaw/workspace/.blueclaw/runtime/current/migrations
  mkdir -p /root/.blueclaw/workspace/.blueclaw/runtime/current/migrations
  cp -af "$STAGE/blueclaw-migrations/." /root/.blueclaw/workspace/.blueclaw/runtime/current/migrations/
fi
if [ -d "$STAGE/graphiti_memoryd" ]; then
  rm -rf /opt/internkim/graphiti_memoryd
  mkdir -p /opt/internkim/graphiti_memoryd
  cp -af "$STAGE/graphiti_memoryd/." /opt/internkim/graphiti_memoryd/
fi
if [ -d "$STAGE/admin-ui" ]; then
  rm -rf /opt/internkim/admin-ui
  mkdir -p /opt/internkim/admin-ui
  cp -af "$STAGE/admin-ui/." /opt/internkim/admin-ui/
  chmod -R a+rX /opt/internkim/admin-ui
fi
if [ -f "$STAGE/assets/internkim.png" ]; then
  mkdir -p /opt/internkim/assets
  cp -f "$STAGE/assets/internkim.png" /opt/internkim/assets/internkim.png
  chmod 644 /opt/internkim/assets/internkim.png
fi
if [ -f "$STAGE/SOUL.md" ] && { [ ! -f /root/.blueclaw/workspace/SOUL.md ] || grep -q '^# IDENTITY.md' /root/.blueclaw/workspace/SOUL.md 2>/dev/null; }; then
  cp -f "$STAGE/SOUL.md" /root/.blueclaw/workspace/SOUL.md
fi
if [ -f "$STAGE/IDENTITY.md" ]; then
  cp -f "$STAGE/IDENTITY.md" /root/.blueclaw/workspace/IDENTITY.md
fi
if [ -f "$STAGE/BOT_PROFILE.yaml" ]; then
  cp -f "$STAGE/BOT_PROFILE.yaml" /root/.blueclaw/workspace/BOT_PROFILE.yaml
  rm -f /root/.blueclaw/workspace/BOT_PROFILE.md
fi
if [ -f "$STAGE/AGENTS.md" ]; then
  cp -f "$STAGE/AGENTS.md" /root/.blueclaw/workspace/AGENTS.md
fi
if [ -d "$STAGE/workspace-restore" ]; then
  cp -af "$STAGE/workspace-restore/." /root/.blueclaw/workspace/
  for reservedDirectory in skills bin downloads; do
    if [ -e "/root/.blueclaw/workspace/$reservedDirectory" ] && [ ! -d "/root/.blueclaw/workspace/$reservedDirectory" ]; then
      rm -f "/root/.blueclaw/workspace/$reservedDirectory"
    fi
  done
  echo "Workspace restored from backup"
fi

echo "Staged files installed."

NOLOGIN_BINARY=$(command -v nologin || echo /usr/sbin/nologin)
getent group blueclaw >/dev/null 2>&1 || groupadd --system blueclaw
id blueclaw &>/dev/null || useradd -r -g blueclaw -m -d /home/blueclaw -s "$NOLOGIN_BINARY" blueclaw
getent group internkim-site >/dev/null 2>&1 || groupadd --system internkim-site
id internkim-site &>/dev/null || useradd -r -g internkim-site -d /nonexistent -s "$NOLOGIN_BINARY" internkim-site
install -d -o blueclaw -g blueclaw -m 750 /home/blueclaw /home/blueclaw/.cache /home/blueclaw/.config
chown blueclaw:blueclaw /root/.blueclaw/workspace/AGENTS.md /root/.blueclaw/workspace/IDENTITY.md /root/.blueclaw/workspace/BOT_PROFILE.yaml /root/.blueclaw/workspace/SOUL.md 2>/dev/null || true
chmod 711 /root
mkdir -p /root/.internkim/secrets /root/.internkim/env /root/.internkim/config
chown root:root /root/.internkim/secrets
chmod 700 /root/.internkim/secrets
chown root:root /root/.internkim/config
chmod 700 /root/.internkim/config
chown root:blueclaw /root/.internkim/env
chmod 750 /root/.internkim/env
mkdir -p /root/.internkim/sites /root/.internkim/secrets/sites
chown root:internkim-site /root/.internkim /root/.internkim/sites /root/.internkim/secrets/sites
chmod 755 /root/.internkim
chmod 750 /root/.internkim/sites /root/.internkim/secrets/sites
chown root:root /root/.internkim/secrets/openrouter-api-key 2>/dev/null || true
chmod 600 /root/.internkim/secrets/openrouter-api-key 2>/dev/null || true
if [ ! -s /root/.internkim/secrets/llmd-auth-key ]; then
  umask 077
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > /root/.internkim/secrets/llmd-auth-key
fi
chown root:root /root/.internkim/secrets/llmd-auth-key
chmod 600 /root/.internkim/secrets/llmd-auth-key
if [ -f /root/.internkim/secrets/google-sa.json ]; then
  chown root:root /root/.internkim/secrets/google-sa.json
  chmod 600 /root/.internkim/secrets/google-sa.json
fi
if [ -f /root/.internkim/secrets/gas-webhook-url ]; then
  chown root:root /root/.internkim/secrets/gas-webhook-url
  chmod 600 /root/.internkim/secrets/gas-webhook-url
fi
if [ -f /root/.internkim/secrets/slack-bot-token ]; then
  chown root:root /root/.internkim/secrets/slack-bot-token
  chmod 600 /root/.internkim/secrets/slack-bot-token
fi
if [ -f /root/.internkim/secrets/slack-app-token ]; then
  chown root:root /root/.internkim/secrets/slack-app-token
  chmod 600 /root/.internkim/secrets/slack-app-token
fi
if [ -f /root/.internkim/config/signal-jsonrpc-url ]; then
  chown root:root /root/.internkim/config/signal-jsonrpc-url
  chmod 600 /root/.internkim/config/signal-jsonrpc-url
fi
if [ -f /root/.internkim/config/signal-account ]; then
  chown root:root /root/.internkim/config/signal-account
  chmod 600 /root/.internkim/config/signal-account
fi
install -d -o root -g root -m 700 /root/.internkim/models
if [ -f "$STAGE/models/gemma-4-E4B-it.litertlm" ]; then
  cp -f "$STAGE/models/gemma-4-E4B-it.litertlm" /root/.internkim/models/gemma-4-E4B-it.litertlm
  chown root:root /root/.internkim/models/gemma-4-E4B-it.litertlm
  chmod 600 /root/.internkim/models/gemma-4-E4B-it.litertlm
fi
if [ ! -s /root/.internkim/models/bge-m3-Q8_0.gguf ]; then
  curl -L --fail --retry 3 --output /root/.internkim/models/bge-m3-Q8_0.gguf.tmp https://huggingface.co/gpustack/bge-m3-GGUF/resolve/2d48f1737679ad900d5c26c5aad5410e9c70fdca/bge-m3-Q8_0.gguf
  mv /root/.internkim/models/bge-m3-Q8_0.gguf.tmp /root/.internkim/models/bge-m3-Q8_0.gguf
fi
chown root:root /root/.internkim/models/bge-m3-Q8_0.gguf 2>/dev/null || true
chmod 600 /root/.internkim/models/bge-m3-Q8_0.gguf 2>/dev/null || true
mkdir -p /root/.blueclaw/workspace/bin /root/.blueclaw/workspace/downloads
chown -R blueclaw:blueclaw /root/.blueclaw
chown root:blueclaw /root/.blueclaw/config 2>/dev/null || true
chmod 770 /root/.blueclaw/config 2>/dev/null || true
chown root:blueclaw /root/.blueclaw/config/runtime.json /root/.blueclaw/config/policy.json 2>/dev/null || true
chmod 640 /root/.blueclaw/config/runtime.json /root/.blueclaw/config/policy.json 2>/dev/null || true
chown -R root:blueclaw /root/.blueclaw/workspace/.blueclaw/runtime/current/migrations 2>/dev/null || true
chmod -R u=rwX,g=rX,o= /root/.blueclaw/workspace/.blueclaw/runtime/current/migrations 2>/dev/null || true
chmod 750 /root/.blueclaw/workspace/.blueclaw/runtime/current/migrations 2>/dev/null || true

mkdir -p /etc/sudoers.d
rm -f /usr/local/bin/gws-* /etc/sudoers.d/blueclaw-gws /etc/sudoers.d/blueclaw-mcp`)
}

func renderFirstbootNetworkSection() string {
	return strings.TrimSpace(`# ── Wi-Fi ──
echo "Setting up Wi-Fi..."

for rfkillPath in /sys/class/rfkill/rfkill*; do
  [ -d "$rfkillPath" ] || continue
  rfkillType=$(cat "$rfkillPath/type" 2>/dev/null)
  if [ "$rfkillType" = "wlan" ]; then
    echo 0 > "$rfkillPath/soft" 2>/dev/null || true
    echo "  rfkill: unblocked $rfkillPath"
  fi
done
iw reg set KR 2>/dev/null || true

systemctl stop wpa_supplicant.service 2>/dev/null || true
systemctl mask wpa_supplicant.service 2>/dev/null || true
systemctl stop NetworkManager 2>/dev/null || true
systemctl disable NetworkManager 2>/dev/null || true
systemctl mask NetworkManager 2>/dev/null || true
systemctl stop dhcpcd 2>/dev/null || true
systemctl disable dhcpcd 2>/dev/null || true
systemctl mask dhcpcd 2>/dev/null || true
rm -f /run/wpa_supplicant/wlan0 2>/dev/null || true
rm -f /run/systemd/network/10-netplan-wlan0.network 2>/dev/null || true
systemctl restart wpa_supplicant@wlan0 2>/dev/null || true
sleep 5
networkctl reload 2>/dev/null || true

echo "Waiting for network..."
if ! wait_for_ipv4 120; then
  echo "ERROR: wlan0 did not receive IPv4 within 120s"
  log_network_diagnostics
  retry_later "waiting for DHCP lease on wlan0"
fi

systemctl restart sshd 2>/dev/null || true
save_board_ip
ensure_dns

if ! wait_for_egress 90; then
  echo "ERROR: outbound network not available after 90s"
  log_network_diagnostics
  ensure_dns
  retry_later "waiting for outbound connectivity"
fi

if ! sync_time_safely; then
  echo "ERROR: time sync unavailable"
  ensure_dns
  retry_later "waiting for time sync"
fi`)
}

func renderFirstbootPackagesSection() string {
	return strings.TrimSpace(`if phase_done packages; then
  echo "Phase packages already complete"
else
  start_phase packages

  echo "Setting up swap..."
  if [ ! -f /swapfile ]; then
    dd if=/dev/zero of=/swapfile bs=1M count=2048 2>/dev/null
    chmod 600 /swapfile
    mkswap /swapfile >/dev/null 2>&1
  fi
  swapon /swapfile 2>/dev/null || true
  grep -q '/swapfile' /etc/fstab 2>/dev/null || echo '/swapfile none swap sw 0 0' >> /etc/fstab
  echo "Swap ready: $(free -h | grep Swap | awk '{print $2}')"

  echo "Installing packages..."
  if [ ! -f /var/cache/internkim/debs.tar ] || [ ! -s /var/cache/internkim/debs.tar ]; then
    echo "FATAL: /var/cache/internkim/debs.tar not found. Injection failed." >&2
    exit 1
  fi
  echo "  Using pre-injected packages (offline install)"
  mkdir -p /var/cache/apt/archives
  tar xf /var/cache/internkim/debs.tar -C /var/cache/apt/archives/
  DEBIAN_FRONTEND=noninteractive dpkg -i /var/cache/apt/archives/*.deb 2>&1 || true
  apt-get -f install -y -qq 2>&1 | tail -5 || true
  rm -f /var/cache/internkim/debs.tar

  mark_phase_done packages
fi`)
}

func renderFirstbootToolsSection() string {
	parts := []string{
		strings.TrimSpace(`if phase_done tools; then
  echo "Phase tools already complete"
else
  start_phase tools
  if ! wait_for_egress 60; then
    log_network_diagnostics
    retry_later "waiting for outbound connectivity before tools phase"
  fi`),
		strings.TrimSpace(fmt.Sprintf(deviceBrowserRuntimeDependencyInstallScript()+`
apt-get install -y -qq unzip >/dev/null 2>&1 || true
if ! sudo -u blueclaw test -x /home/blueclaw/.bun/bin/bun; then
  sudo -u blueclaw bash -lc 'curl -fsSL https://bun.sh/install | bash' >/dev/null 2>&1 || true
fi
ln -sf /home/blueclaw/.bun/bin/bun /usr/local/bin/bun
ln -sf /home/blueclaw/.bun/bin/bun /usr/local/bin/node
echo "bun: $(command -v bun >/dev/null && echo ok || echo missing)"

if [ -d "$STAGE/skills" ]; then
  mkdir -p /root/.blueclaw/workspace/skills
  cp -af "$STAGE/skills/." /root/.blueclaw/workspace/skills/
fi
if [ -d "$STAGE/tools" ]; then
  rm -rf /root/.blueclaw/workspace/tools
  mkdir -p /root/.blueclaw/workspace/tools
  cp -af "$STAGE/tools/." /root/.blueclaw/workspace/tools/
fi

install_device_browser_runtime() {
  if [ ! -x "`+browserruntime.DeviceBrowserExecutablePath+`" ]; then
    echo "ERROR: Lightpanda device browser missing at `+browserruntime.DeviceBrowserExecutablePath+`" >&2
    exit 1
  fi
  `+deviceBrowserVersionShellCommand(browserruntime.DeviceBrowserExecutablePath)+`
}

install_device_browser_runtime

agentBrowserSkillDir="/root/.blueclaw/workspace/.agents/skills/agent-browser"
mkdir -p "$agentBrowserSkillDir"
if command -v agent-browser >/dev/null 2>&1; then
  if command -v pkill >/dev/null 2>&1; then
    pkill -TERM -x agent-browser >/tmp/internkim-agent-browser-device-close.log 2>&1 || true
    sleep 1
    pkill -KILL -x agent-browser >>/tmp/internkim-agent-browser-device-close.log 2>&1 || true
  fi
  timeout 5s agent-browser close --all >>/tmp/internkim-agent-browser-device-close.log 2>&1 || true
  rm -f /root/.agent-browser/internkim-device.pid /root/.agent-browser/internkim-device.stream /root/.agent-browser/internkim-device.engine /root/.agent-browser/internkim-device.version
  rm -f /root/.agent-browser/internkim-device-smoke.pid /root/.agent-browser/internkim-device-smoke.stream /root/.agent-browser/internkim-device-smoke.engine /root/.agent-browser/internkim-device-smoke.version
  sleep 1
fi
if [ -f "$STAGE/.agents/skills/agent-browser/SKILL.md" ]; then
  rm -f "$agentBrowserSkillDir/SKILL.md.tmp"
  cp -f "$STAGE/.agents/skills/agent-browser/SKILL.md" "$agentBrowserSkillDir/SKILL.md"
else
  rm -f "$agentBrowserSkillDir/SKILL.md.tmp"
fi

chown -R blueclaw:blueclaw /root/.blueclaw/workspace/.agents 2>/dev/null || true
chown -R root:root "$agentBrowserSkillDir" 2>/dev/null || true
chmod -R a+rX,go-w "$agentBrowserSkillDir" 2>/dev/null || true
chown -R root:root /root/.blueclaw/workspace/skills 2>/dev/null || true
chmod -R a+rX,go-w /root/.blueclaw/workspace/skills 2>/dev/null || true
chown -R root:root /root/.blueclaw/workspace/tools 2>/dev/null || true
chmod -R a+rX,go-w /root/.blueclaw/workspace/tools 2>/dev/null || true

if ! command -v uv >/dev/null 2>&1; then
  curl -LsSf https://astral.sh/uv/0.11.11/install.sh -o /tmp/internkim-uv-install.sh
  UV_UNMANAGED_INSTALL=/usr/local/bin sh /tmp/internkim-uv-install.sh
fi
uv tool install --upgrade litert-lm >/dev/null
ln -sf /root/.local/bin/litert-lm /usr/local/bin/litert-lm
if [ -f /opt/internkim/graphiti_memoryd/requirements.txt ]; then
  uv venv --clear /opt/internkim/graphiti-venv >/dev/null
  uv pip install --python /opt/internkim/graphiti-venv/bin/python -r /opt/internkim/graphiti_memoryd/requirements.txt >/dev/null
  cat > /usr/local/bin/graphiti-memoryd <<'WRAPEOF'
#!/bin/sh
PYTHONPATH=/opt/internkim exec /opt/internkim/graphiti-venv/bin/python -m graphiti_memoryd "$@"
WRAPEOF
  chmod 755 /usr/local/bin/graphiti-memoryd
  chown -R root:blueclaw /opt/internkim
  chmod -R u=rwX,g=rX,o=rX /opt/internkim
fi
if [ ! -s %s ]; then
  curl -L --fail --retry 3 --output %s.tmp %s
  mv %s.tmp %s
fi
chown root:root /root/.internkim/models %s
chmod 700 /root/.internkim/models
chmod 600 %s

mark_phase_done tools
fi`,
			blueclaw.LiteRTModelPath,
			blueclaw.LiteRTModelPath,
			blueclaw.LiteRTModelSourceURL,
			blueclaw.LiteRTModelPath,
			blueclaw.LiteRTModelPath,
			blueclaw.LiteRTModelPath,
			blueclaw.LiteRTModelPath,
		)),
	}

	return strings.Join(parts, "\n\n")
}

func renderFirstbootApplicationsSection(deviceURL, adminEmail string) string {
	section := strings.TrimSpace(`if phase_done apps; then
  echo "Phase apps already complete"
else
  start_phase apps

  echo "Setting up PostgreSQL..."
  systemctl start postgresql
  sleep 2
  if [ -f "$STAGE/mattermost-db.sql" ]; then
    echo "  Restoring Mattermost DB from backup..."
    su - postgres -c "psql -c \"SELECT 1 FROM pg_database WHERE datname='mattermost'\" | grep -q 1 || psql -c \"CREATE DATABASE mattermost\""
    su - postgres -c "psql mattermost" < "$STAGE/mattermost-db.sql" 2>/dev/null || true
    rm -f "$STAGE/mattermost-db.sql"
    echo "  DB restored"
  fi
  MM_DB_PASS=$(cat /root/.internkim/secrets/mm-db-pass 2>/dev/null || echo "")
  if [ -z "$MM_DB_PASS" ]; then
    MM_DB_PASS=$(head -c 12 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 16)
    printf '%s' "$MM_DB_PASS" > /root/.internkim/secrets/mm-db-pass
    chmod 600 /root/.internkim/secrets/mm-db-pass
  fi
  su - postgres -c "psql -c \"SELECT 1 FROM pg_roles WHERE rolname='mmuser'\" | grep -q 1 || psql -c \"CREATE USER mmuser WITH PASSWORD '$MM_DB_PASS'\""
  su - postgres -c "psql -c \"SELECT 1 FROM pg_database WHERE datname='mattermost'\" | grep -q 1 || psql -c \"CREATE DATABASE mattermost OWNER mmuser\""
  su - postgres -c "psql -c \"GRANT ALL PRIVILEGES ON DATABASE mattermost TO mmuser\""
  su - postgres -c "psql -c \"SELECT 1 FROM pg_roles WHERE rolname='blueclaw'\" | grep -q 1 || createuser blueclaw"
  if [ -f "$STAGE/blueclaw-db.sql" ]; then
    echo "  Restoring Blueclaw DB from backup..."
    su - postgres -c "dropdb --if-exists blueclaw && createdb -O blueclaw blueclaw"
    su - postgres -c "psql blueclaw" < "$STAGE/blueclaw-db.sql" 2>/dev/null || true
    rm -f "$STAGE/blueclaw-db.sql"
    echo "  Blueclaw DB restored"
  else
    su - postgres -c "psql -c \"SELECT 1 FROM pg_database WHERE datname='blueclaw'\" | grep -q 1 || createdb -O blueclaw blueclaw"
  fi

  echo "Installing Mattermost..."
  if [ ! -f /var/cache/internkim/mattermost.tar.gz ] || [ ! -s /var/cache/internkim/mattermost.tar.gz ]; then
    echo "FATAL: /var/cache/internkim/mattermost.tar.gz not found. Injection failed." >&2
    exit 1
  fi
  echo "  Using pre-injected archive"
  cd /var/cache/internkim && tar -xzf mattermost.tar.gz
  legacy_data="/opt/mattermost/data"
  persistent_data="/var/lib/mattermost/data"
  if [ -d "$legacy_data" ] && [ ! -L "$legacy_data" ]; then
    mkdir -p "$(dirname "$persistent_data")"
    if [ ! -e "$persistent_data" ]; then
      mv "$legacy_data" "$persistent_data"
    else
      cp -an "$legacy_data"/. "$persistent_data"/ 2>/dev/null || true
    fi
  fi
  rm -rf /opt/mattermost
  mv /var/cache/internkim/mattermost /opt/mattermost
  rm -f /var/cache/internkim/mattermost.tar.gz
  mkdir -p "$persistent_data"
  rm -rf "$legacy_data"
  ln -s "$persistent_data" "$legacy_data"
  id mattermost &>/dev/null || useradd --system --user-group mattermost
  chown -R mattermost:mattermost /opt/mattermost "$(dirname "$persistent_data")"
  chmod -R g+w /opt/mattermost "$(dirname "$persistent_data")"

  SITE_URL="__DEVICE_URL__"
  [ -z "$SITE_URL" ] && SITE_URL="http://localhost:8065"
  cp /opt/mattermost/config/config.defaults.json /opt/mattermost/config/config.json 2>/dev/null || true
  jq --arg ds "postgres://mmuser:${MM_DB_PASS}@localhost/mattermost?sslmode=disable&connect_timeout=10" \
     --arg url "$SITE_URL" \
     --arg resourcePaths "__MANAGED_RESOURCE_PATHS__" \
     '.SqlSettings.DriverName = "postgres" | .SqlSettings.DataSource = $ds | .FileSettings.DriverName = "local" | .FileSettings.Directory = "/var/lib/mattermost/data" | .FileSettings.EnableFileAttachments = true | .ServiceSettings.SiteURL = $url | .ServiceSettings.AllowCorsFrom = $url | .ServiceSettings.CorsAllowCredentials = true | .ServiceSettings.ManagedResourcePaths = $resourcePaths | .ServiceSettings.EnableUserAccessTokens = true | .ServiceSettings.EnableBotAccountCreation = true | .TeamSettings.TeammateNameDisplay = "nickname_full_name" | .EmailSettings.SendPushNotifications = true | .EmailSettings.PushNotificationServer = "https://push-test.mattermost.com" | .EmailSettings.PushNotificationContents = "id_loaded"' \
     /opt/mattermost/config/config.json > /opt/mattermost/config/config.tmp \
     && mv /opt/mattermost/config/config.tmp /opt/mattermost/config/config.json
  chown mattermost:mattermost /opt/mattermost/config/config.json

  cat > /etc/systemd/system/mattermost.service <<'SVCEOF'
[Unit]
Description=Mattermost
After=network.target postgresql.service
BindsTo=postgresql.service

[Service]
Type=notify
ExecStart=/opt/mattermost/bin/mattermost server
TimeoutStartSec=3600
KillMode=mixed
Restart=always
RestartSec=10
WorkingDirectory=/opt/mattermost
User=mattermost
Group=mattermost
LimitNOFILE=49152

[Install]
WantedBy=multi-user.target
SVCEOF
  systemctl daemon-reload
  systemctl enable mattermost
  systemctl start mattermost

  echo "Waiting for Mattermost to be ready..."
  for attemptIndex in $(seq 1 60); do
    if curl -sf http://localhost:8065/api/v4/system/ping 2>/dev/null | grep -q '"status":"OK"'; then
      echo "Mattermost ready"
      break
    fi
    sleep 3
  done

  MM_URL="http://localhost:8065"
  ADMIN_EMAIL="admin@localhost"
  ADMIN_USER="admin"
  ADMIN_PASS=$(cat /root/.internkim/secrets/mm-admin-pass)

  echo "Creating admin account..."
  curl -sf -X POST "$MM_URL/api/v4/users" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" >/dev/null 2>&1 || true

  ADMIN_TOKEN="$(curl -sf -D - -X POST "$MM_URL/api/v4/users/login" \
    -H 'Content-Type: application/json' \
    -d "{\"login_id\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" 2>/dev/null \
    | grep -i '^token:' | awk '{print $2}' | tr -d '\r' || true)"

  if [ -z "$ADMIN_TOKEN" ]; then
    echo "ERROR: Could not get admin token"
    exit 1
  fi

  ADMIN_ID="$(curl -sf "$MM_URL/api/v4/users/username/$ADMIN_USER" \
    -H "Authorization: Bearer $ADMIN_TOKEN" 2>/dev/null | jq -r '.id // empty' || true)"
  if [ -z "$ADMIN_ID" ]; then
    echo "ERROR: Could not resolve admin user id"
    exit 1
  fi
  su - postgres -c "psql mattermost -c \"UPDATE users SET email = 'admin@localhost', roles = 'system_admin system_user', deleteat = 0 WHERE id = '$ADMIN_ID'\"" >/dev/null
  curl -sf -X PUT "$MM_URL/api/v4/users/$ADMIN_ID/patch" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}" >/dev/null
  curl -sf -X PUT "$MM_URL/api/v4/users/$ADMIN_ID/roles" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"roles":"system_admin system_user"}' >/dev/null

  PAT_RESPONSE="$(curl -sf -X POST "$MM_URL/api/v4/users/$ADMIN_ID/tokens" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"description":"internkim-setup"}' 2>/dev/null || true)"
  PAT_TOKEN=$(echo "$PAT_RESPONSE" | jq -r '.token // empty')

  echo "Creating bot account..."
  BOT_RESPONSE="$(curl -sf -X POST "$MM_URL/api/v4/bots" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"username":"internkim","display_name":"김인턴"}' 2>/dev/null || true)"
  BOT_USER_ID=$(echo "$BOT_RESPONSE" | jq -r '.user_id // empty')
  if [ -z "$BOT_USER_ID" ]; then
    BOT_USER_ID="$(curl -sf "$MM_URL/api/v4/users/username/internkim" \
      -H "Authorization: Bearer $ADMIN_TOKEN" 2>/dev/null | jq -r '.id // empty' || true)"
  fi
  if [ -n "$BOT_USER_ID" ]; then
    curl -sf -X PUT "$MM_URL/api/v4/users/$BOT_USER_ID/patch" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H 'Content-Type: application/json' \
      -d '{"first_name":"Intern","last_name":"Kim","nickname":"김인턴","position":""}' >/dev/null 2>&1 || true
    if [ -f /opt/internkim/assets/internkim.png ]; then
      curl -sf -X POST "$MM_URL/api/v4/users/$BOT_USER_ID/image" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -F "image=@/opt/internkim/assets/internkim.png;type=image/png" >/dev/null 2>&1 || true
    fi
  fi

  BOT_TOKEN=""
  if [ -n "$BOT_USER_ID" ]; then
    BOT_PAT="$(curl -sf -X POST "$MM_URL/api/v4/users/$BOT_USER_ID/tokens" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H 'Content-Type: application/json' \
      -d '{"description":"internkim-bot"}' 2>/dev/null || true)"
    BOT_TOKEN=$(echo "$BOT_PAT" | jq -r '.token // empty')
  fi

  if [ -n "$BOT_USER_ID" ] && [ -n "$ADMIN_ID" ]; then
    DM_RESPONSE=$(curl -sf -X POST "$MM_URL/api/v4/channels/direct" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H 'Content-Type: application/json' \
      -d "[\"$ADMIN_ID\",\"$BOT_USER_ID\"]" 2>/dev/null || true)
    DM_ID=$(echo "$DM_RESPONSE" | jq -r '.id // empty' || true)
    if [ -n "$DM_ID" ]; then
      POSTS_RESPONSE="$(curl -sf "$MM_URL/api/v4/channels/$DM_ID/posts?per_page=200" \
        -H "Authorization: Bearer $ADMIN_TOKEN" 2>/dev/null || true)"
      OLDEST_POST_ID=$(echo "$POSTS_RESPONSE" | jq -r '.order[-1] // empty' || true)
      if [ -n "$OLDEST_POST_ID" ]; then
        OLDEST_POST_MESSAGE=$(echo "$POSTS_RESPONSE" | jq -r ".posts[\"$OLDEST_POST_ID\"].message // empty" || true)
        WELCOME_MESSAGE="Please add me to teams and channels you want me to interact in. To do this, use the browser or Mattermost Desktop App."
        if [ "$OLDEST_POST_MESSAGE" = "$WELCOME_MESSAGE" ]; then
          curl -sf -X DELETE "$MM_URL/api/v4/posts/$OLDEST_POST_ID" \
            -H "Authorization: Bearer $ADMIN_TOKEN" >/dev/null 2>&1 || true
        fi
      fi
    fi
  fi

  echo "Setting up team..."
  TEAM_RESPONSE="$(curl -sf "$MM_URL/api/v4/teams/name/internkim" \
    -H "Authorization: Bearer $ADMIN_TOKEN" 2>/dev/null || true)"
  TEAM_ID=$(echo "$TEAM_RESPONSE" | jq -r '.id // empty')
  if [ -z "$TEAM_ID" ]; then
    TEAM_RESPONSE="$(curl -sf -X POST "$MM_URL/api/v4/teams" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H 'Content-Type: application/json' \
      -d '{"name":"internkim","display_name":"Intern Kim","type":"I"}' 2>/dev/null || true)"
    TEAM_ID=$(echo "$TEAM_RESPONSE" | jq -r '.id // empty')
  fi
  echo "Team ID: ${TEAM_ID:-none}"

  if [ -n "$TEAM_ID" ] && [ -n "$BOT_USER_ID" ]; then
    curl -sf -X POST "$MM_URL/api/v4/teams/$TEAM_ID/members" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H 'Content-Type: application/json' \
      -d "{\"team_id\":\"$TEAM_ID\",\"user_id\":\"$BOT_USER_ID\"}" >/dev/null 2>&1 || true
    echo "Bot added to team"
  fi

  if [ -n "$BOT_TOKEN" ] && curl -sf "$MM_URL/api/v4/users/me" \
    -H "Authorization: Bearer $BOT_TOKEN" >/dev/null 2>&1; then
    printf '%s' "$BOT_TOKEN" > /root/.internkim/secrets/mattermost-bot-token
    chown root:root /root/.internkim/secrets/mattermost-bot-token
    chmod 600 /root/.internkim/secrets/mattermost-bot-token
  else
    echo "WARNING: Mattermost bot token was not created or did not validate"
  fi

  mark_phase_done apps
fi`)

	replacements := map[string]string{
		"__DEVICE_URL__":             deviceURL,
		"__ADMIN_EMAIL__":            adminEmail,
		"__MANAGED_RESOURCE_PATHS__": mattermostManagedResourcePathSetting(),
	}

	return fillFirstbootPlaceholders(section, replacements)
}

func renderFirstbootServicesSection(isLocalLlamaProvisioned bool) string {
	section := strings.TrimSpace(fmt.Sprintf(`if phase_done services; then
  echo "Phase services already complete"
else
  start_phase services

  systemctl enable systemd-time-wait-sync.service 2>/dev/null
  systemctl stop zeroclaw 2>/dev/null || true
  systemctl disable zeroclaw 2>/dev/null || true
  rm -f /etc/systemd/system/zeroclaw.service
  rm -rf /etc/systemd/system/zeroclaw.service.d
%s
  cat > %s <<'SVCEOF'
%sSVCEOF
  cat > %s <<'CAPABILITYEOF'
%sCAPABILITYEOF
  cat > %s <<'LLMDEOF'
%sLLMDEOF
  cat > %s <<'GRAPHITIEOF'
%sGRAPHITIEOF
  cat > %s <<'ADMINDEOF'
%sADMINDEOF
  cat > %s <<'LLAMACPP_EOF'
%sLLAMACPP_EOF
  cat > %s <<'LLAMACPP_EMBEDDING_EOF'
%sLLAMACPP_EMBEDDING_EOF
  cat > %s <<'SYNCEOF'
%sSYNCEOF
  chmod 755 %s
  cat > %s <<'SYNCSERVICEEOF'
%sSYNCSERVICEEOF
  cat > %s <<'SYNCTIMEREOF'
%sSYNCTIMEREOF
  cat > %s <<'CLEANEOF'
%sCLEANEOF
  chmod 755 %s
  cat > %s <<'CLEANSERVICEEOF'
%sCLEANSERVICEEOF
  cat > %s <<'CLEANTIMEREOF'
%sCLEANTIMEREOF
  cat > /etc/systemd/system/cloudflared.service <<'CLOUDFLARED_FLEET_EOF'
[Unit]
Description=Cloudflare Fleet Tunnel
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
Type=simple
ExecStart=/bin/sh -c '/usr/local/bin/cloudflared tunnel run --protocol quic --token "$(cat /root/.internkim/secrets/tunnel-token)"'
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
CLOUDFLARED_FLEET_EOF
  cat > /etc/systemd/system/cloudflared-node-ssh.service <<'CLOUDFLARED_NODE_EOF'
[Unit]
Description=Cloudflare Node SSH Tunnel
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
Type=simple
ExecStart=/bin/sh -c '/usr/local/bin/cloudflared tunnel run --protocol quic --token "$(cat /root/.internkim/secrets/node-tunnel-token)"'
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
CLOUDFLARED_NODE_EOF
  if [ ! -f /root/.internkim/secrets/node-tunnel-token ]; then
    cp -f /root/.internkim/secrets/tunnel-token /root/.internkim/secrets/node-tunnel-token
    chmod 600 /root/.internkim/secrets/node-tunnel-token
  fi
  systemctl daemon-reload
  systemctl disable %s 2>/dev/null || true
  systemctl disable %s 2>/dev/null || true
  systemctl enable %s
  systemctl start %s
  systemctl enable %s
  systemctl start %s
  systemctl enable %s
  systemctl start %s
  systemctl enable %s
  systemctl start %s
  systemctl enable %s
  systemctl start %s
  systemctl enable %s
  systemctl start %s
  systemctl enable %s
  systemctl start %s
  systemctl enable --now internkim-users-sync.timer
  systemctl enable --now internkim-blueclaw-tmp-clean.timer
  systemctl enable cloudflared-node-ssh
  systemctl start cloudflared-node-ssh
  cloudflaredServiceNames="cloudflared"
  if [ "$(cat /root/.internkim/env/fleet-role 2>/dev/null || true)" = "pending" ]; then
    systemctl disable --now cloudflared 2>/dev/null || true
    cloudflaredServiceNames="cloudflared-node-ssh"
  else
    systemctl enable cloudflared
    systemctl start cloudflared
    cloudflaredServiceNames="cloudflared cloudflared-node-ssh"
  fi

  echo "Waiting for services..."
  for attemptIndex in $(seq 1 150); do
    allServicesActive=true
    for serviceName in mattermost %s %s %s %s %s %s $cloudflaredServiceNames postgresql; do
      if ! systemctl is-active --quiet "$serviceName" 2>/dev/null; then
        allServicesActive=false
        break
      fi
    done
    if [ "$allServicesActive" = true ]; then
      echo "All services active"
      break
    fi
    sleep 2
  done

  systemctl start internkim-users-sync.service || journalctl -u internkim-users-sync -n 40 --no-pager

  mark_phase_done services
fi`,
		blueclaw.LLMDServiceCredentialInstallCommand(blueclaw.LocalOnlyEnabled()),
		blueclaw.BlueclawServicePath,
		blueclaw.BlueclawServiceUnit(),
		blueclaw.CapabilitydServicePath,
		blueclaw.CapabilitydServiceUnit(),
		blueclaw.LLMDServicePath,
		blueclaw.LLMDServiceUnit(isLocalLlamaProvisioned),
		blueclaw.GraphitiMemorydServicePath,
		blueclaw.GraphitiMemorydServiceUnit(),
		blueclaw.AdmindServicePath,
		blueclaw.AdmindServiceUnit(),
		locallm.LlamaCppServicePath,
		blueclaw.LlamaCppServiceUnit(),
		locallm.LlamaCppEmbeddingServicePath,
		blueclaw.LlamaCppEmbeddingServiceUnit(),
		blueclaw.InternKimUsersSyncScriptPath,
		blueclaw.InternKimUsersSyncScript(),
		blueclaw.InternKimUsersSyncScriptPath,
		blueclaw.InternKimUsersSyncServicePath,
		blueclaw.InternKimUsersSyncServiceUnit(),
		blueclaw.InternKimUsersSyncTimerPath,
		blueclaw.InternKimUsersSyncTimerUnit(),
		blueclaw.InternKimBlueclawTemporaryCleanupScriptPath,
		blueclaw.InternKimBlueclawTemporaryCleanupScript(),
		blueclaw.InternKimBlueclawTemporaryCleanupScriptPath,
		blueclaw.InternKimBlueclawTemporaryCleanupServicePath,
		blueclaw.InternKimBlueclawTemporaryCleanupServiceUnit(),
		blueclaw.InternKimBlueclawTemporaryCleanupTimerPath,
		blueclaw.InternKimBlueclawTemporaryCleanupTimerUnit(),
		locallm.LlamaCppServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		blueclaw.LLMDServiceName,
		blueclaw.LLMDServiceName,
		locallm.LlamaCppServiceName,
		locallm.LlamaCppServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		blueclaw.CapabilitydServiceName,
		blueclaw.CapabilitydServiceName,
		blueclaw.AdmindServiceName,
		blueclaw.AdmindServiceName,
		blueclaw.GraphitiMemorydServiceName,
		blueclaw.GraphitiMemorydServiceName,
		blueclaw.BlueclawServiceName,
		blueclaw.BlueclawServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		blueclaw.LLMDServiceName,
		blueclaw.CapabilitydServiceName,
		blueclaw.AdmindServiceName,
		blueclaw.GraphitiMemorydServiceName,
		blueclaw.BlueclawServiceName,
	))
	return section
}

func renderFirstbootCompletionSection() string {
	return strings.TrimSpace(`# ── Disable first-boot ──
systemctl disable internkim-firstboot
rm -f /usr/local/bin/internkim-firstboot.sh
echo "=== Intern Kim first-boot complete ==="
date

if [ -f /sys/class/leds/ACT/trigger ]; then
  echo heartbeat > /sys/class/leds/ACT/trigger 2>/dev/null || true
fi

echo "first-boot complete"
rm -f /usr/local/bin/internkim-firstboot.sh
cp /var/log/internkim-firstboot.log /boot/firmware/internkim/firstboot.log 2>/dev/null || true`)
}

func fillFirstbootPlaceholders(section string, replacements map[string]string) string {
	filledSection := section
	for placeholder, replacement := range replacements {
		filledSection = strings.ReplaceAll(filledSection, placeholder, replacement)
	}
	return filledSection
}
