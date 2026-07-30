package admind

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const relayProxyPrefix = "/relay"

func (service *Service) relayLoopbackHost() string {
	relayURL := strings.TrimSpace(service.Configuration.BuzzRelayURL)
	relayURL = strings.TrimPrefix(relayURL, "wss://")
	relayURL = strings.TrimPrefix(relayURL, "ws://")
	return strings.TrimSuffix(relayURL, "/")
}

func (service *Service) buzzRelayPublicURL() string {
	if configured := strings.TrimSpace(service.Configuration.BuzzRelayPublicURL); configured != "" {
		return configured
	}
	deviceURL := strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceURLPath))
	return blueclaw.DeriveRelayPublicURL(deviceURL)
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

// The Buzz relay listens only on the device loopback and keys each community by
// the connection's Host header. Proxying it under the public gateway — with the
// Host rewritten back to the loopback address — lets external Buzz apps reach
// the same community over wss://<domain>/relay, while the relay's own membership
// and key (NIP-42) auth stay the access boundary.
func (service *Service) handleRelayProxy() http.Handler {
	host := service.relayLoopbackHost()
	target, errorValue := url.Parse("http://" + host)
	if host == "" || errorValue != nil {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			http.Error(responseWriter, "buzz relay unavailable", http.StatusNotImplemented)
		})
	}
	presentedHost := host
	if publicHost := service.buzzRelayPublicHost(); publicHost != "" {
		presentedHost = publicHost
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalDirector(request)
		request.URL.Path = strings.TrimPrefix(request.URL.Path, relayProxyPrefix)
		if request.URL.Path == "" {
			request.URL.Path = "/"
		}
		request.Host = presentedHost
	}
	return proxy
}

func (service *Service) publicRelayURL(request *http.Request) string {
	if publicURL := service.buzzRelayPublicURL(); publicURL != "" {
		return publicURL
	}
	host := strings.TrimSpace(request.Host)
	if host == "" {
		return strings.TrimSpace(service.Configuration.BuzzRelayURL)
	}
	return "wss://" + host + relayProxyPrefix
}
