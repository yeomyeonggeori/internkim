package setup

import (
	"errors"
	"fmt"
	"strings"
)

var StepPreflight = Step{
	Name: "preflight",
	Title: func(context *Context) string {
		return context.T("사전 점검 중...", "Running preflight checks...")
	},
	IsSatisfied: func(context *Context) bool {
		return context.BoardType == "" || context.BoardType != BoardJetsonOrinNano
	},
	Run: func(context *Context) error {
		if context.BoardType != BoardJetsonOrinNano {
			return nil
		}
		if context.Backend != BackendSSH {
			return errors.New("Jetson Orin Nano setup requires SSH")
		}
		if context.SSH == nil {
			return errors.New("Jetson preflight SSH callback missing")
		}

		output := context.SSH.Run(jetsonPreflightScript())
		return parseJetsonPreflightOutput(output)
	},
}

func jetsonPreflightScript() string {
	return `set -u
echo __INTERNKIM_PREFLIGHT_BEGIN__
fail() {
  echo "fail:$1"
  exit 0
}

architecture="$(uname -m 2>/dev/null || true)"
[ "$architecture" = "aarch64" ] || fail "Jetson setup requires aarch64; detected ${architecture:-unknown}"
command -v systemctl >/dev/null 2>&1 || fail "systemd is required"
command -v sudo >/dev/null 2>&1 || fail "sudo is required"
command -v apt-get >/dev/null 2>&1 || fail "apt is required"
command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v jq >/dev/null 2>&1 || true

if [ -f /etc/os-release ]; then
  . /etc/os-release
else
  fail "/etc/os-release is missing"
fi

if [ "${ID:-}" = "debian" ] && { [ "${VERSION_CODENAME:-}" = "trixie" ] || [ "${VERSION_ID:-}" = "13" ]; }; then
  fail "Debian Trixie is not supported for Jetson local AI; flash JetPack 6.x / Jetson Linux 36.x"
fi

if ! getent hosts nvidia.com >/dev/null 2>&1 && ! getent hosts ubuntu.com >/dev/null 2>&1; then
  fail "outbound DNS/network is unavailable"
fi

availableMegabytes=""
for attemptIndex in $(seq 1 36); do
  availableMegabytes="$(df -Pm / | awk 'NR == 2 {print $4}')"
  case "$availableMegabytes" in
    ''|*[!0-9]*) fail "could not determine available disk space" ;;
  esac
  [ "$availableMegabytes" -ge 12000 ] && break
  if systemctl is-active internkim-jetson-firstboot.service >/dev/null 2>&1; then
    sleep 5
    continue
  fi
  break
done
case "$availableMegabytes" in
  ''|*[!0-9]*) fail "could not determine available disk space" ;;
esac
[ "$availableMegabytes" -ge 12000 ] || fail "at least 12GB free disk is required; detected ${availableMegabytes}MB"

jetsonRelease=""
if [ -f /etc/nv_tegra_release ]; then
  jetsonRelease="$(cat /etc/nv_tegra_release)"
elif command -v dpkg-query >/dev/null 2>&1; then
  jetsonRelease="$(dpkg-query -W -f='${Version}' nvidia-l4t-core 2>/dev/null || true)"
fi

printf '%s\n' "$jetsonRelease" | grep -Eq '(^# R36|^36\.|36\.)' || fail "JetPack 6.x / Jetson Linux 36.x is required"

if ! command -v nvidia-smi >/dev/null 2>&1 &&
   ! command -v tegrastats >/dev/null 2>&1 &&
   ! command -v nvpmodel >/dev/null 2>&1 &&
   [ ! -f /etc/nv_tegra_release ]; then
  fail "NVIDIA Jetson runtime tools were not found"
fi

if [ "$(id -u)" != "0" ]; then
  fail "preflight did not run with sudo privileges"
fi

echo ok`
}

func parseJetsonPreflightOutput(output string) error {
	trimmedOutput := strings.TrimSpace(output)
	if !strings.Contains(trimmedOutput, "__INTERNKIM_PREFLIGHT_BEGIN__") {
		return errors.New("Jetson preflight could not run privileged commands; ensure the SSH user has sudo permission. If sudo requires a password, pass --password")
	}
	for _, line := range strings.Split(trimmedOutput, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "fail:") {
			return fmt.Errorf("Jetson preflight failed: %s", strings.TrimSpace(strings.TrimPrefix(line, "fail:")))
		}
		if line == "ok" {
			fmt.Println("  Jetson preflight: ok")
			return nil
		}
	}
	return fmt.Errorf("Jetson preflight failed: %s", trimmedOutput)
}
