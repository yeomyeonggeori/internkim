package admind

import (
	"net/http"

	"golang.org/x/oauth2"
)

type caldavBearerHTTPClient struct {
	inner  *http.Client
	source oauth2.TokenSource
}

func newCalDAVBearerHTTPClient(inner *http.Client, source oauth2.TokenSource) *caldavBearerHTTPClient {
	if inner == nil {
		inner = http.DefaultClient
	}
	return &caldavBearerHTTPClient{inner: inner, source: source}
}

func (client *caldavBearerHTTPClient) Do(request *http.Request) (*http.Response, error) {
	token, errorValue := client.source.Token()
	if errorValue != nil {
		return nil, errorValue
	}
	token.SetAuthHeader(request)
	return client.inner.Do(request)
}
