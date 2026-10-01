package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
)

type registerResponse struct {
	FleetID           string `json:"fleet_id"`
	OldFleetID        string `json:"old_fleet_id"`
	NodeID            string `json:"node_id"`
	TunnelToken       string `json:"tunnel_token"`
	NodeTunnelToken   string `json:"node_tunnel_token"`
	URL               string `json:"url"`
	AliasURL          string `json:"alias_url"`
	SSHHostname       string `json:"ssh_hostname"`
	TLSStatus         string `json:"tls_certificate_status"`
	FleetRole         string `json:"fleet_role"`
	FleetActiveCount  int    `json:"fleet_active_count"`
	FleetPendingCount int    `json:"fleet_pending_count"`
	FleetQuorumSize   int    `json:"fleet_quorum_size"`
}

type registrationHTTPError struct {
	statusCode int
	body       string
}

var (
	registerHTTPClient = &http.Client{Timeout: 30 * time.Second}
)

func (registrationError *registrationHTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", registrationError.statusCode, registrationError.body)
}

func (response *registerResponse) publicURL() string {
	return response.URL
}

func (response *registerResponse) registeredFleetID() string {
	return response.FleetID
}

func (response *registerResponse) registeredNodeID() string {
	return response.NodeID
}

func registerFleetNode(configuration config, fleetID, nodeID, nodeKey, fleetSecret, adminEmail string) (*registerResponse, error) {
	return registerFleetRequest(configuration, map[string]string{
		"fleet_id":     fleetID,
		"node_id":      nodeID,
		"node_key":     nodeKey,
		"fleet_secret": fleetSecret,
		"admin_email":  adminEmail,
	})
}

func registerFleetIDMigration(configuration config, oldFleetID, newFleetID, nodeID, nodeKey, fleetSecret, adminEmail string) (*registerResponse, error) {
	return registerFleetRequest(configuration, map[string]string{
		"fleet_id":     oldFleetID,
		"new_fleet_id": newFleetID,
		"node_id":      nodeID,
		"node_key":     nodeKey,
		"fleet_secret": fleetSecret,
		"admin_email":  adminEmail,
	})
}

func registerFleetRequest(configuration config, requestBody map[string]string) (*registerResponse, error) {
	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", configuration.APIBaseURL+"/api/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(configuration.RegisterSecret) != "" {
		req.Header.Set("Authorization", "Bearer "+configuration.RegisterSecret)
	}

	resp, err := registerHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, &registrationHTTPError{statusCode: resp.StatusCode, body: string(respBody)}
	}

	var result registerResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return &result, nil
}

func registerFleetNodeWithCollisionRetry(configuration config, stateDir, fleetID, adminEmail string) (*registerResponse, error) {
	fleetSecret := loadOrCreateFleetSecret(stateDir)
	nodeID := loadNodeID(stateDir)
	nodeKey := loadOrCreateNodeKey(stateDir)

	var registrationResponse *registerResponse
	retryError := retryOperation(retryOptions{
		AttemptCount: 5,
		ShouldRetry: func(errorValue error) bool {
			return isFleetIDCollisionRegistrationError(errorValue)
		},
	}, func(attemptIndex int) error {
		response, registerError := registerFleetNode(configuration, fleetID, nodeID, nodeKey, fleetSecret, adminEmail)
		if registerError == nil {
			saveRegistrationResponse(stateDir, response)
			saveDefaultFleetNode(stateDir, response)
			registrationResponse = response
			return nil
		}
		if !isFleetIDCollisionRegistrationError(registerError) {
			return registerError
		}
		if strings.TrimSpace(argString("--fleet", "")) != "" {
			return errors.New("fleet join rejected; check --fleet and --fleet-secret")
		}

		fleetID, fleetSecret = resetFleetIdentity(stateDir)
		fmt.Printf("  Fleet ID collision detected; retrying with %s\n", fleetID)
		return registerError
	})
	if retryError == nil {
		return registrationResponse, nil
	}
	if isFleetIDCollisionRegistrationError(retryError) {
		return nil, errors.New("fleet_id collision retry limit reached")
	}
	return nil, retryError
}

func isFleetIDCollisionRegistrationError(errorValue error) bool {
	var httpError *registrationHTTPError
	return errors.As(errorValue, &httpError) && httpError.statusCode == http.StatusConflict
}

func saveRegistrationResponse(stateDir string, response *registerResponse) {
	saveState(stateDir, "fleet_id", response.registeredFleetID())
	saveState(stateDir, "node_id", firstNonEmptyString(response.registeredNodeID(), loadNodeID(stateDir)))
	saveState(stateDir, "fleet_role", firstNonEmptyString(response.FleetRole, "active"))
	saveState(stateDir, "fleet_active_count", fmt.Sprint(defaultInt(response.FleetActiveCount, 1)))
	saveState(stateDir, "fleet_pending_count", fmt.Sprint(response.FleetPendingCount))
	saveState(stateDir, "fleet_quorum_size", fmt.Sprint(defaultInt(response.FleetQuorumSize, 1)))
	saveState(stateDir, "tunnel_token", response.TunnelToken)
	if strings.TrimSpace(response.NodeTunnelToken) != "" {
		saveState(stateDir, "node_tunnel_token", response.NodeTunnelToken)
	}
	saveState(stateDir, "device_url", response.publicURL())
	if strings.TrimSpace(response.SSHHostname) != "" {
		saveState(stateDir, "ssh_hostname", response.SSHHostname)
	}
	if strings.TrimSpace(response.TLSStatus) != "" {
		saveState(stateDir, "tls_certificate_status", response.TLSStatus)
	}
}

func savedRemoteSSHHostname(target commandTarget) string {
	if target.useRemoteSSH && strings.TrimSpace(target.host) != "" {
		return strings.TrimSpace(target.host)
	}
	return strings.TrimSpace(target.sshHostname)
}

func updateRemoteDeviceRegistration(connection *sshClient, configuration config, response *registerResponse) {
	connection.run(remoteDeviceRegistrationScript(configuration, response))
}

func remoteDeviceRegistrationScript(configuration config, response *registerResponse) string {
	nodeTunnelToken := firstNonEmptyString(response.NodeTunnelToken, response.TunnelToken)
	return fmt.Sprintf(`mkdir -p /root/.internkim/secrets /root/.internkim/env
printf '%%s' %s > /root/.internkim/env/fleet-id
printf '%%s' %s > /root/.internkim/env/device-url
printf '%%s' %s > /root/.internkim/secrets/tunnel-token
printf '%%s' %s > /root/.internkim/secrets/node-tunnel-token
printf '%%s' %s > /root/.internkim/env/tunnel-origin
printf '%%s' %s > /root/.internkim/env/tunnel-revision
printf '%%s' %s > /root/.internkim/env/api-url
printf '%%s' %s > /root/.internkim/env/tls-certificate-status
chown root:root /root/.internkim/secrets/tunnel-token /root/.internkim/secrets/node-tunnel-token
chmod 600 /root/.internkim/secrets/tunnel-token /root/.internkim/secrets/node-tunnel-token
chown root:blueclaw /root/.internkim/env/fleet-id /root/.internkim/env/device-url /root/.internkim/env/tunnel-origin /root/.internkim/env/tunnel-revision /root/.internkim/env/api-url /root/.internkim/env/tls-certificate-status
chmod 640 /root/.internkim/env/fleet-id /root/.internkim/env/device-url /root/.internkim/env/tunnel-origin /root/.internkim/env/tunnel-revision /root/.internkim/env/api-url /root/.internkim/env/tls-certificate-status
%s
%s
rm -f /etc/init.d/S98cloudflared 2>/dev/null
systemctl daemon-reload
systemctl enable cloudflared cloudflared-node-ssh
systemctl restart cloudflared cloudflared-node-ssh`,
		quoteShellValue(response.registeredFleetID()),
		quoteShellValue(response.publicURL()),
		quoteShellValue(response.TunnelToken),
		quoteShellValue(nodeTunnelToken),
		quoteShellValue(setup.AdminGatewayTunnelOrigin),
		quoteShellValue(setup.TunnelConfigurationRevision),
		quoteShellValue(configuration.APIBaseURL),
		quoteShellValue(response.TLSStatus),
		cloudflaredTunnelUnitFile("cloudflared", "Cloudflare Tunnel", "/root/.internkim/secrets/tunnel-token"),
		cloudflaredTunnelUnitFile("cloudflared-node-ssh", "Cloudflare Node SSH Tunnel", "/root/.internkim/secrets/node-tunnel-token"),
	)
}

func cloudflaredTunnelUnitFile(unitName string, description string, tokenPath string) string {
	return fmt.Sprintf(`cat > /etc/systemd/system/%s.service <<'SVCEOF'
[Unit]
Description=%s
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
Type=simple
ExecStart=/bin/sh -c '/usr/local/bin/cloudflared tunnel run --protocol quic --token "$(cat %s)"'
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SVCEOF`, unitName, description, tokenPath)
}

func saveDefaultFleetNode(stateDir string, response *registerResponse) {
	nodeID := setupNodeIdentityName(response.registeredNodeID())
	if nodeID == "" || strings.TrimSpace(response.FleetRole) != "active" {
		return
	}
	if filepath.Base(filepath.Dir(stateDir)) != "boards" {
		return
	}
	baseStateDir := filepath.Dir(filepath.Dir(stateDir))
	saveState(baseStateDir, "default_node_id", nodeID)
}

func remoteSetupAdminEmail(stateDir string) string {
	return loadState(stateDir, "admin_email")
}
