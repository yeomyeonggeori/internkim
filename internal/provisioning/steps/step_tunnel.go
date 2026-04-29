package setup

import (
	"errors"
	"strings"
)

const MattermostTunnelOrigin = "http://127.0.0.1:18080"
const CloudflaredTunnelProtocol = "http2"
const TunnelConfigurationRevision = "admin-gateway-v2-http2"

var StepTunnel = Step{
	Name: "tunnel",
	Deps: []string{"binaries", "openrouter"},
	Title: func(context *Context) string {
		return context.T("기기 등록 + 터널 설정 중...", "Registering device + tunnel setup...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSSH:
			cloudflaredActive := trimmedRun(context, "systemctl is-active cloudflared") == "active"
			cloudflaredService := trimmedRun(context, "systemctl cat cloudflared 2>/dev/null")
			deviceID := trimmedRun(context, "cat /root/.internkim/env/device-id 2>/dev/null")
			deviceURL := trimmedRun(context, "cat /root/.internkim/env/device-url 2>/dev/null")
			expectedDeviceID := ""
			expectedDeviceURL := ""
			if context.Callbacks.LoadState != nil {
				expectedDeviceID = context.Callbacks.LoadState("device_id")
				expectedDeviceURL = context.Callbacks.LoadState("device_url")
			}
			tunnelOrigin := trimmedRun(context, "cat /root/.internkim/env/tunnel-origin 2>/dev/null")
			tunnelRevision := trimmedRun(context, "cat /root/.internkim/env/tunnel-revision 2>/dev/null")
			return cloudflaredActive &&
				strings.Contains(cloudflaredService, "--protocol "+CloudflaredTunnelProtocol) &&
				(expectedDeviceID == "" || deviceID == expectedDeviceID) &&
				(expectedDeviceURL == "" || deviceURL == expectedDeviceURL) &&
				tunnelOrigin == MattermostTunnelOrigin &&
				tunnelRevision == TunnelConfigurationRevision &&
				sshFileExists(context, "/root/.internkim/env/api-url") &&
				sshFileExists(context, "/root/.internkim/env/device-id") &&
				sshFileExists(context, "/root/.internkim/secrets/device-secret") &&
				sshFileExists(context, "/root/.internkim/env/device-url") &&
				sshFileExists(context, "/root/.internkim/env/mattermost-url")
		case BackendSD:
			return stagedFileExists(context, "secrets/tunnel-token") &&
				stagedFileExists(context, "device-url") &&
				stagedFileExists(context, "mattermost-url") &&
				stagedFileExists(context, "tunnel-origin") &&
				stagedFileExists(context, "tunnel-revision")
		}
		return false
	},
	Run: func(context *Context) error {
		if context.Callbacks.ProvisionTunnelSSH == nil {
			return errors.New("tunnel SSH callback missing")
		}
		return context.Callbacks.ProvisionTunnelSSH(context)
	},
	RunSD: func(context *Context) error {
		if context.Callbacks.StageTunnelSD == nil {
			return errors.New("tunnel SD callback missing")
		}
		return context.Callbacks.StageTunnelSD(context)
	},
}
