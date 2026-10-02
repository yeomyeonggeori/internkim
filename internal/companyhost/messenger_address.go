package companyhost

import (
	"net/url"
	"strings"
)

func MessengerHost(connection Connection) string {
	appURL, errorValue := url.Parse(connection.AppURL)
	if errorValue != nil {
		return ""
	}
	return strings.ToLower(connection.Company.Slug) + "." + appURL.Hostname()
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
