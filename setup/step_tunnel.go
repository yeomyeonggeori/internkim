package setup

import "errors"

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
			return cloudflaredActive && sshFileExists(context, "/root/.internkim/env/device-url")
		case BackendSD:
			return stagedFileExists(context, "secrets/tunnel-token") &&
				stagedFileExists(context, "device-url")
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
