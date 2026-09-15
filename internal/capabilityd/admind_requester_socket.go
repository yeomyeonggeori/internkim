package capabilityd

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
)

const (
	admindRequesterPlaceholderHost = "http://internkim"
	admindRequesterEmailHeader     = "X-INTERNKIM-REQUESTER-EMAIL"
)

func admindRequesterURL(path string) string {
	return admindRequesterPlaceholderHost + path
}

func (service Service) askAdmindAsTheRequester(httpRequest *http.Request, requesterEmail string) (*http.Response, error) {
	if normalizedEmail := strings.ToLower(strings.TrimSpace(requesterEmail)); normalizedEmail != "" {
		httpRequest.Header.Set(admindRequesterEmailHeader, normalizedEmail)
	}
	httpClient, errorValue := service.admindRequesterClient()
	if errorValue != nil {
		return nil, errorValue
	}
	return httpClient.Do(httpRequest)
}

func (service Service) admindRequesterClient() (*http.Client, error) {
	if service.HTTPClient != nil {
		return service.HTTPClient, nil
	}
	socketPath := strings.TrimSpace(service.Configuration.AdmindSocketPath)
	if socketPath == "" {
		return nil, errors.New("admind honours an asserted requester only on its socket, and no admind socket path is configured")
	}
	return &http.Client{
		Timeout:   service.httpClientTimeout(),
		Transport: &http.Transport{DialContext: dialerOntoAdmindSocket(socketPath)},
	}, nil
}

func dialerOntoAdmindSocket(socketPath string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _ string, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, "unix", socketPath)
	}
}
