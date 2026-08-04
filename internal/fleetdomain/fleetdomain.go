package fleetdomain

import (
	"net/url"
	"strings"
)

var defaultZone = "intern.kim"

func Default() string {
	return strings.ToLower(strings.TrimSpace(defaultZone))
}

func Zone(apiBaseURL string) string {
	if configured := zoneOfAPIBaseURL(apiBaseURL); configured != "" {
		return configured
	}
	return Default()
}

func zoneOfAPIBaseURL(apiBaseURL string) string {
	trimmed := strings.TrimSpace(apiBaseURL)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "//") {
		trimmed = "https://" + trimmed
	}
	parsed, errorValue := url.Parse(trimmed)
	if errorValue != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "api.")
}

func Host(fleetID string, zone string) string {
	trimmedFleetID := strings.ToLower(strings.TrimSpace(fleetID))
	if trimmedFleetID == "" || zone == "" {
		return ""
	}
	return trimmedFleetID + "." + zone
}

func Covers(zone string, host string) bool {
	if zone == "" {
		return false
	}
	lowered := strings.ToLower(strings.TrimSpace(host))
	return lowered == zone || strings.HasSuffix(lowered, "."+zone)
}

func Subdomain(label string, zone string) string {
	if zone == "" {
		return ""
	}
	return "https://" + label + "." + zone
}
