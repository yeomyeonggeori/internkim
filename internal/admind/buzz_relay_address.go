package admind

import (
	"strings"
)

func (service *Service) buzzRelayPublicURL() string {
	return strings.TrimSpace(service.Configuration.BuzzRelayPublicURL)
}

func (service *Service) buzzRelayPublicHost() string {
	publicURL := service.buzzRelayPublicURL()
	publicURL = strings.TrimPrefix(publicURL, "wss://")
	publicURL = strings.TrimPrefix(publicURL, "ws://")
	return strings.TrimSuffix(publicURL, "/")
}

func (service *Service) buzzRelayEffectiveURL() string {
	if publicURL := service.buzzRelayPublicURL(); publicURL != "" {
		return publicURL
	}
	return strings.TrimSpace(service.Configuration.BuzzRelayURL)
}
