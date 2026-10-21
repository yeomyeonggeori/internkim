package servicetoken

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
)

func Authorize(request *http.Request, tokenPath string) error {
	trimmedTokenPath := strings.TrimSpace(tokenPath)
	if trimmedTokenPath == "" {
		return nil
	}
	token, errorValue := os.ReadFile(trimmedTokenPath)
	if errorValue != nil {
		return fmt.Errorf("the service token at %s could not be read: %w", trimmedTokenPath, errorValue)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	return nil
}

func Client(base *http.Client, tokenPath string, serviceURLs ...string) *http.Client {
	if strings.TrimSpace(tokenPath) == "" {
		return base
	}
	authorizing := *base
	authorizing.Transport = transport{base: base.Transport, tokenPath: tokenPath, serviceHosts: hostsOf(serviceURLs)}
	return &authorizing
}

type transport struct {
	base         http.RoundTripper
	tokenPath    string
	serviceHosts []string
}

func (transport transport) RoundTrip(request *http.Request) (*http.Response, error) {
	base := transport.base
	if base == nil {
		base = http.DefaultTransport
	}
	if !slices.Contains(transport.serviceHosts, request.URL.Host) {
		return base.RoundTrip(request)
	}
	authorized := request.Clone(request.Context())
	if errorValue := Authorize(authorized, transport.tokenPath); errorValue != nil {
		return nil, errorValue
	}
	return base.RoundTrip(authorized)
}

func hostsOf(serviceURLs []string) []string {
	hosts := []string{}
	for _, serviceURL := range serviceURLs {
		parsed, errorValue := url.Parse(strings.TrimSpace(serviceURL))
		if errorValue == nil && parsed.Host != "" {
			hosts = append(hosts, parsed.Host)
		}
	}
	return hosts
}

func IsCarriedBy(request *http.Request, tokenPath string) bool {
	kept, errorValue := os.ReadFile(strings.TrimSpace(tokenPath))
	expected := strings.TrimSpace(string(kept))
	if errorValue != nil || expected == "" {
		return false
	}
	offered, hasBearer := strings.CutPrefix(request.Header.Get("Authorization"), "Bearer ")
	return hasBearer && subtle.ConstantTimeCompare([]byte(offered), []byte(expected)) == 1
}
