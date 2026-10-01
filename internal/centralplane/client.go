package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	DefaultProjectURL     string
	DefaultPublishableKey string
)

type Settings struct {
	AppURL         string
	HostCredential func() string
	ProjectURL     string
	PublishableKey string
	HTTPClient     *http.Client

	// Who a directory write that no person asked for runs as. It is read at
	// write time rather than held as a string, because this client is built
	// once and the first administrator may be claimed after that.
	ClaimedAdministratorEmail func() string
}

func (settings Settings) Configured() bool {
	return strings.TrimSpace(settings.AppURL) != "" &&
		settings.HostCredential != nil &&
		strings.TrimSpace(settings.HostCredential()) != "" &&
		strings.TrimSpace(settings.ProjectURL) != "" &&
		strings.TrimSpace(settings.PublishableKey) != ""
}

type Client struct {
	settings   Settings
	httpClient *http.Client

	mutex    sync.Mutex
	sessions map[string]memberSession
}

type memberSession struct {
	memberID    string
	accessToken string
	expiresAt   time.Time
}

func New(settings Settings) *Client {
	httpClient := settings.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		settings:   settings,
		httpClient: askingForJSON(httpClient),
		sessions:   map[string]memberSession{},
	}
}

// SvelteKit negotiates an error's shape on the Accept header and serves an HTML
// page to a caller that asks for nothing (handle_fatal_error).
func askingForJSON(httpClient *http.Client) *http.Client {
	asking := *httpClient
	asking.Transport = jsonAcceptedByDefault{carrying: transportOf(httpClient)}
	return &asking
}

func transportOf(httpClient *http.Client) http.RoundTripper {
	if httpClient.Transport != nil {
		return httpClient.Transport
	}
	return http.DefaultTransport
}

type jsonAcceptedByDefault struct {
	carrying http.RoundTripper
}

func (transport jsonAcceptedByDefault) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Header.Get("Accept") != "" {
		return transport.carrying.RoundTrip(request)
	}
	asking := request.Clone(request.Context())
	asking.Header.Set("Accept", "application/json")
	return transport.carrying.RoundTrip(asking)
}

func (client *Client) sessionFor(ctx context.Context, platform string, externalID string) (memberSession, error) {
	key := platform + "|" + externalID
	client.mutex.Lock()
	cached, found := client.sessions[key]
	client.mutex.Unlock()
	if found && time.Until(cached.expiresAt) > 5*time.Minute {
		return cached, nil
	}

	payload, errorValue := json.Marshal(map[string]string{"kind": platform, "externalID": externalID})
	if errorValue != nil {
		return memberSession{}, errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(client.settings.AppURL, "/")+"/api/agent/session", bytes.NewReader(payload))
	if errorValue != nil {
		return memberSession{}, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.HostCredential())
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return memberSession{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return memberSession{}, fmt.Errorf("central plane refused %s identity %s: %s", platform, externalID, response.Status)
	}

	var answer struct {
		MemberID    string `json:"memberID"`
		AccessToken string `json:"accessToken"`
		ExpiresAt   int64  `json:"expiresAt"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return memberSession{}, errorValue
	}
	session := memberSession{
		memberID:    answer.MemberID,
		accessToken: answer.AccessToken,
		expiresAt:   time.Unix(answer.ExpiresAt, 0),
	}

	client.mutex.Lock()
	client.sessions[key] = session
	client.mutex.Unlock()
	return session, nil
}
