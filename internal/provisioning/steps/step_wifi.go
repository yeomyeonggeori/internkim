package setup

import "errors"

var StepWifi = Step{
	Name: "wifi",
	Deps: []string{"board"},
	Title: func(context *Context) string {
		return context.T("Wi-Fi 설정...", "Configuring Wi-Fi...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSSH:
			if context.BoardType == BoardJetsonOrinNano {
				return jetsonWiFiRecoveryIsInstalled(context)
			}
			wlanAddress := trimmedRun(context, `ip -4 addr show wlan0 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1`)
			return wlanAddress != ""
		case BackendSD:
			return stagedFileExists(context, "wpa_supplicant.conf")
		}
		return false
	},
	Run: func(context *Context) error {
		if context.Callbacks.ConfigureWifiSSH == nil {
			return errors.New("wifi SSH callback missing")
		}
		return context.Callbacks.ConfigureWifiSSH(context)
	},
	RunSD: func(context *Context) error {
		if context.Callbacks.StageWifiSD == nil {
			return errors.New("wifi SD callback missing")
		}
		return context.Callbacks.StageWifiSD(context)
	},
}

func jetsonWiFiRecoveryIsInstalled(context *Context) bool {
	return trimmedRun(context, `test -x /usr/local/bin/internkim-wifi-select &&
test -f /etc/systemd/system/internkim-wifi-recovery.service &&
test -f /etc/systemd/system/internkim-wifi-recovery.timer &&
systemctl is-enabled internkim-wifi-recovery.timer >/dev/null 2>&1 &&
find /etc/NetworkManager/system-connections -maxdepth 1 -type f -name 'internkim-wifi-*.nmconnection' | grep -q . &&
echo ready || true`) == "ready"
}
