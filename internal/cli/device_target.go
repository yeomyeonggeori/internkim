package cli

import (
	"net/http"
	"os"
	"strings"
)

type deviceTarget struct {
	AdminURL    string
	SSHHostname string
	FleetID     string
	FleetSecret string
}

func deviceTargetFromEnvironment() deviceTarget {
	return deviceTarget{
		AdminURL:    normalizeAdminURL(os.Getenv("INTERNKIM_DEVICE_URL")),
		SSHHostname: strings.TrimSpace(os.Getenv("INTERNKIM_SSH_HOSTNAME")),
		FleetID:     strings.ToLower(strings.TrimSpace(os.Getenv("INTERNKIM_FLEET_ID"))),
		FleetSecret: strings.TrimSpace(os.Getenv("INTERNKIM_FLEET_SECRET")),
	}
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

func attachCloudflareAccess(request *http.Request) {
	clientID := strings.TrimSpace(os.Getenv("INTERNKIM_CF_ACCESS_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("INTERNKIM_CF_ACCESS_CLIENT_SECRET"))
	if clientID == "" || clientSecret == "" {
		return
	}
	request.Header.Set("CF-Access-Client-Id", clientID)
	request.Header.Set("CF-Access-Client-Secret", clientSecret)
}
