package companyhost

import (
	"net/netip"
	"net/url"
	"strings"
)

func MessengerHost(connection Connection) string {
	appURL, errorValue := url.Parse(connection.AppURL)
	if errorValue != nil {
		return ""
	}
	if address, errorValue := netip.ParseAddr(appURL.Hostname()); errorValue == nil {
		return addressAuthority(address)
	}
	return strings.ToLower(connection.Company.Slug) + "." + appURL.Hostname()
}

// RFC 3986 section 3.2.2 brackets an IPv6 literal in an authority.
func addressAuthority(address netip.Addr) string {
	if address.Is6() {
		return "[" + address.String() + "]"
	}
	return address.String()
}

func MessengerURL(connection Connection) string {
	return socketScheme(connection.AppURL) + "://" + MessengerHost(connection)
}

func messengerMediaBaseURL(connection Connection) string {
	return pageScheme(connection.AppURL) + "://" + MessengerHost(connection) + "/media"
}

func socketScheme(appURL string) string {
	if strings.HasPrefix(appURL, "https://") {
		return "wss"
	}
	return "ws"
}

func pageScheme(appURL string) string {
	if strings.HasPrefix(appURL, "https://") {
		return "https"
	}
	return "http"
}
