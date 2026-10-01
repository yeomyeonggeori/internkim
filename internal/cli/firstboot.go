package cli

import (
	"fmt"
	"strings"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/internal/runtime/locallm"
)

func buildFirstbootScript(isLocalLlamaProvisioned bool) string {
	sections := []string{
		renderFirstbootPreludeSection(),
		renderFirstbootStagingSection(),
		renderFirstbootNetworkSection(),
		renderFirstbootPackagesSection(),
		renderFirstbootToolsSection(),
		renderFirstbootApplicationsSection(),
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
rm -f /root/.blueclaw/workspace/SOUL.md /root/.blueclaw/workspace/IDENTITY.md /root/.blueclaw/workspace/BOT_PROFILE.yaml /root/.blueclaw/workspace/BOT_PROFILE.md
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
chown blueclaw:blueclaw /root/.blueclaw/workspace/AGENTS.md 2>/dev/null || true
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
  sudo -u blueclaw bash -s -- <<'BUNEOF' >/dev/null 2>&1 || true
set -eu
bun_zip_url="https://github.com/oven-sh/bun/releases/download/bun-v1.3.10/bun-linux-aarch64.zip"
bun_zip_sha256="fa5ecb25cafa8e8f5c87a0f833719d46dd0af0a86c7837d806531212d55636d3"
bun_tmp_zip="$(mktemp)"
bun_tmp_dir="$(mktemp -d)"
curl -fsSL -o "$bun_tmp_zip" "$bun_zip_url"
echo "$bun_zip_sha256  $bun_tmp_zip" | sha256sum -c -
unzip -q "$bun_tmp_zip" -d "$bun_tmp_dir"
mkdir -p /home/blueclaw/.bun/bin
install -m 755 "$bun_tmp_dir/bun-linux-aarch64/bun" /home/blueclaw/.bun/bin/bun
rm -rf "$bun_tmp_zip" "$bun_tmp_dir"
BUNEOF
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
    echo "ERROR: Moli device browser missing at `+browserruntime.DeviceBrowserExecutablePath+`" >&2
    exit 1
  fi
`+browserruntime.DeviceBrowserServiceInstallShellScript()+`}

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
if [ -f /opt/internkim/document-conversion/requirements.txt ]; then
  uv venv --clear /opt/internkim/document-venv >/dev/null
  uv pip install --quiet --python /opt/internkim/document-venv/bin/python -r /opt/internkim/document-conversion/requirements.txt >/dev/null
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

func renderFirstbootApplicationsSection() string {
	return strings.TrimSpace(`if phase_done apps; then
  echo "Phase apps already complete"
else
  start_phase apps

  echo "Setting up PostgreSQL..."
  systemctl start postgresql
  sleep 2
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

  mark_phase_done apps
fi`)
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
  systemctl enable --now internkim-users-sync.timer

  echo "Waiting for services..."
  for attemptIndex in $(seq 1 150); do
    allServicesActive=true
    for serviceName in %s %s %s %s postgresql; do
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
		blueclaw.RetireLLMDLeftByEarlierReleasesCommand(),
		blueclaw.BlueclawServicePath,
		blueclaw.BlueclawServiceUnit(),
		blueclaw.CapabilitydServicePath,
		blueclaw.CapabilitydServiceUnit(),
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
		locallm.LlamaCppServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		locallm.LlamaCppServiceName,
		locallm.LlamaCppServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		blueclaw.CapabilitydServiceName,
		blueclaw.CapabilitydServiceName,
		blueclaw.AdmindServiceName,
		blueclaw.AdmindServiceName,
		blueclaw.BlueclawServiceName,
		blueclaw.BlueclawServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		blueclaw.CapabilitydServiceName,
		blueclaw.AdmindServiceName,
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
