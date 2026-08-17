package blueclaw

import "strings"

// RelayPublicHost is the name a company's Buzz relay answers on for clients
// outside the machine it runs on. Most companies have none: the relay listens on
// loopback, and everyone reaches the messenger through the central plane. A
// company that wants its own Buzz app to connect puts the relay behind a tunnel
// on a domain it owns, and that domain is a setting — nothing here derives it,
// because nothing here knows what the company bought.
func RelayPublicHost(relayDomain string) string {
	host := strings.TrimSpace(relayDomain)
	for _, scheme := range []string{"https://", "http://", "wss://", "ws://"} {
		host = strings.TrimPrefix(host, scheme)
	}
	if slashIndex := strings.IndexByte(host, '/'); slashIndex >= 0 {
		host = host[:slashIndex]
	}
	return host
}

func RelayPublicURL(relayDomain string) string {
	host := RelayPublicHost(relayDomain)
	if host == "" {
		return ""
	}
	return "wss://" + host
}
