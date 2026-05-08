package setup

import (
	"errors"
	"strings"
)

const MattermostTunnelOrigin = "http://127.0.0.1:18080"
const CloudflaredTunnelProtocol = "http2"
const TunnelConfigurationRevision = "admin-gateway-v4-ssh"

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
			cloudflaredNodeSSHActive := trimmedRun(context, "systemctl is-active cloudflared-node-ssh") == "active"
			cloudflaredService := trimmedRun(context, "systemctl cat cloudflared 2>/dev/null")
			cloudflaredNodeSSHService := trimmedRun(context, "systemctl cat cloudflared-node-ssh 2>/dev/null")
			fleetID := trimmedRun(context, "cat /root/.internkim/env/fleet-id 2>/dev/null")
			nodeID := trimmedRun(context, "cat /root/.internkim/env/node-id 2>/dev/null")
			deviceURL := trimmedRun(context, "cat /root/.internkim/env/device-url 2>/dev/null")
			fleetRole := trimmedRun(context, "cat /root/.internkim/env/fleet-role 2>/dev/null")
			expectedFleetID := ""
			expectedNodeID := ""
			expectedDeviceURL := ""
			expectedFleetRole := ""
			if context.Callbacks.LoadState != nil {
				expectedFleetID = context.Callbacks.LoadState("fleet_id")
				expectedNodeID = context.Callbacks.LoadState("node_id")
				expectedDeviceURL = context.Callbacks.LoadState("device_url")
				expectedFleetRole = context.Callbacks.LoadState("fleet_role")
			}
			tunnelOrigin := trimmedRun(context, "cat /root/.internkim/env/tunnel-origin 2>/dev/null")
			tunnelRevision := trimmedRun(context, "cat /root/.internkim/env/tunnel-revision 2>/dev/null")
			hasFleetIdentity := (expectedNodeID == "" || nodeID == expectedNodeID) &&
				(expectedFleetRole == "" || fleetRole == expectedFleetRole) &&
				sshFileExists(context, "/root/.internkim/env/node-id") &&
				sshFileExists(context, "/root/.internkim/env/fleet-role")
			hasTunnelFiles := (expectedFleetID == "" || fleetID == expectedFleetID) &&
				(expectedDeviceURL == "" || deviceURL == expectedDeviceURL) &&
				tunnelOrigin == MattermostTunnelOrigin &&
				tunnelRevision == TunnelConfigurationRevision &&
				sshFileExists(context, "/root/.internkim/env/api-url") &&
				sshFileExists(context, "/root/.internkim/env/fleet-id") &&
				sshFileExists(context, "/root/.internkim/secrets/fleet-secret") &&
				sshFileExists(context, "/root/.internkim/secrets/node-tunnel-token") &&
				sshFileExists(context, "/root/.internkim/env/device-url") &&
				sshFileExists(context, "/root/.internkim/env/mattermost-url") &&
				hasFleetIdentity
			if fleetRole == "pending" {
				return !cloudflaredActive &&
					cloudflaredNodeSSHActive &&
					strings.Contains(cloudflaredNodeSSHService, "--protocol "+CloudflaredTunnelProtocol) &&
					hasTunnelFiles
			}
			return cloudflaredActive &&
				cloudflaredNodeSSHActive &&
				strings.Contains(cloudflaredService, "--protocol "+CloudflaredTunnelProtocol) &&
				strings.Contains(cloudflaredNodeSSHService, "--protocol "+CloudflaredTunnelProtocol) &&
				hasTunnelFiles
		case BackendSD:
			return stagedFileExists(context, "secrets/tunnel-token") &&
				stagedFileExists(context, "secrets/node-tunnel-token") &&
				stagedFileExists(context, "device-url") &&
				stagedFileExists(context, "mattermost-url") &&
				stagedFileExists(context, "tunnel-origin") &&
				stagedFileExists(context, "tunnel-revision") &&
				stagedFileExists(context, "fleet-id") &&
				stagedFileExists(context, "secrets/fleet-secret") &&
				stagedFileExists(context, "node-id") &&
				stagedFileExists(context, "fleet-role") &&
				stagedFileExists(context, "fleet-active-count") &&
				stagedFileExists(context, "fleet-pending-count") &&
				stagedFileExists(context, "fleet-quorum-size")
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
