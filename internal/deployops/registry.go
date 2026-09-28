package deployops

import (
	"net/url"
	"os"
	"strings"
)

const (
	DeviceURLVariable   = "INTERNKIM_DEVICE_URL"
	SSHHostnameVariable = "INTERNKIM_SSH_HOSTNAME"
	FleetIDVariable     = "INTERNKIM_FLEET_ID"
	FleetSecretVariable = "INTERNKIM_FLEET_SECRET"
)

func DeviceTargetFromEnvironment() (Target, bool) {
	adminURL := normalizeAdminURL(os.Getenv("INTERNKIM_DEVICE_URL"))
	fleetID := strings.ToLower(strings.TrimSpace(os.Getenv("INTERNKIM_FLEET_ID")))
	return Target{
		ID:          firstNonEmpty(fleetID, hostName(adminURL)),
		Name:        hostName(adminURL),
		AdminURL:    adminURL,
		SSHHostname: strings.TrimSpace(os.Getenv("INTERNKIM_SSH_HOSTNAME")),
		FleetID:     fleetID,
		FleetSecret: strings.TrimSpace(os.Getenv("INTERNKIM_FLEET_SECRET")),
	}, adminURL != ""
}

func LoadRegistry() TargetRegistry {
	target, isNamed := DeviceTargetFromEnvironment()
	if !isNamed {
		return TargetRegistry{}
	}
	return TargetRegistry{Targets: []Target{target}}
}

func normalizeAdminURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "https://" + value
}

func hostName(rawURL string) string {
	parsedURL, errorValue := url.Parse(normalizeAdminURL(rawURL))
	if errorValue != nil || parsedURL.Hostname() == "" {
		return strings.TrimSpace(rawURL)
	}
	return parsedURL.Hostname()
}
