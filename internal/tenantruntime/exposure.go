package tenantruntime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type MattermostExposure struct {
	TenantID    string `json:"tenantID"`
	InternalURL string `json:"internalURL"`
	PublicURL   string `json:"publicURL,omitempty"`
	ServiceName string `json:"serviceName"`
	ServicePath string `json:"servicePath"`
}

func (service Service) ExposeMattermost(ctx context.Context, tenantIDs []string) ([]MattermostExposure, error) {
	exposures := []MattermostExposure{}
	for _, tenantID := range tenantIDs {
		exposure, errorValue := service.exposeMattermostTenant(ctx, tenantID)
		if errorValue != nil {
			return nil, errorValue
		}
		exposures = append(exposures, exposure)
	}
	return exposures, nil
}

func (service Service) exposeMattermostTenant(ctx context.Context, tenantID string) (MattermostExposure, error) {
	manifest, errorValue := service.ReadManifest(tenantID)
	if errorValue != nil {
		return MattermostExposure{}, errorValue
	}
	instance := manifest.MattermostInstance
	if instance.Port <= 0 {
		return MattermostExposure{}, errors.New("tenant Mattermost instance port is required")
	}
	serviceName := mattermostTunnelServiceName(manifest.TenantID)
	servicePath := filepath.Join(service.systemdSystemDirectoryPath(), serviceName)
	if errorValue := os.MkdirAll(filepath.Dir(servicePath), 0o755); errorValue != nil {
		return MattermostExposure{}, errorValue
	}
	if errorValue := os.WriteFile(servicePath, []byte(service.mattermostTunnelServiceUnit(manifest)), 0o644); errorValue != nil {
		return MattermostExposure{}, errorValue
	}
	for _, command := range []ExecutableCommand{
		{ExecutableName: "systemctl", Arguments: []string{"daemon-reload"}},
		{ExecutableName: "systemctl", Arguments: []string{"enable", serviceName}},
		{ExecutableName: "systemctl", Arguments: []string{"restart", serviceName}},
	} {
		if _, errorValue := service.commandRunner().Run(ctx, command); errorValue != nil {
			return MattermostExposure{}, errorValue
		}
	}
	publicURL := service.readMattermostTunnelURL(ctx, serviceName)
	return MattermostExposure{
		TenantID:    manifest.TenantID,
		InternalURL: fmt.Sprintf("http://127.0.0.1:%d", instance.Port),
		PublicURL:   publicURL,
		ServiceName: serviceName,
		ServicePath: servicePath,
	}, nil
}

func (service Service) mattermostTunnelServiceUnit(manifest Manifest) string {
	return `[Unit]
Description=InternKim Mattermost public tunnel ` + manifest.TenantID + `
After=network-online.target internkim-mattermost-` + manifest.TenantID + `.service
Wants=network-online.target internkim-mattermost-` + manifest.TenantID + `.service

[Service]
User=root
ExecStart=` + service.cloudflaredPath() + ` tunnel --no-autoupdate --url http://127.0.0.1:` + intString(manifest.MattermostInstance.Port) + `
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`
}

func (service Service) readMattermostTunnelURL(ctx context.Context, serviceName string) string {
	output, errorValue := service.commandRunner().Run(ctx, ExecutableCommand{
		ExecutableName: "journalctl",
		Arguments:      []string{"-u", serviceName, "--no-pager", "-n", "200"},
	})
	if errorValue != nil {
		return ""
	}
	return lastCloudflareTunnelURL(output)
}

var cloudflareTunnelURLPattern = regexp.MustCompile(`https://[-a-zA-Z0-9.]+trycloudflare\.com`)

func lastCloudflareTunnelURL(output string) string {
	matches := cloudflareTunnelURLPattern.FindAllString(output, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimSpace(matches[len(matches)-1])
}

func mattermostTunnelServiceName(tenantID string) string {
	return "internkim-mm-tunnel-" + tenantID + ".service"
}
