package blueclaw

import "strings"

func DeriveRelayPublicHost(deviceHost string) string {
	host := relayHostOnly(deviceHost)
	if host == "" {
		return ""
	}
	dotIndex := strings.IndexByte(host, '.')
	if dotIndex < 0 {
		return host + "-relay"
	}
	return host[:dotIndex] + "-relay" + host[dotIndex:]
}

func DeriveRelayPublicURL(deviceHost string) string {
	publicHost := DeriveRelayPublicHost(deviceHost)
	if publicHost == "" {
		return ""
	}
	return "wss://" + publicHost
}

func relayHostOnly(value string) string {
	host := strings.TrimSpace(value)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "wss://")
	host = strings.TrimPrefix(host, "ws://")
	if slashIndex := strings.IndexByte(host, '/'); slashIndex >= 0 {
		host = host[:slashIndex]
	}
	return host
}
