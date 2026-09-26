package box

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/companyhost"
)

type Configuration struct {
	SchemaVersion int                      `json:"schemaVersion"`
	AppURL        string                   `json:"appURL"`
	Company       companyhost.Company      `json:"company"`
	CentralPlane  companyhost.CentralPlane `json:"centralPlane"`
	GatewayURL    string                   `json:"gatewayURL"`
}

type HostSession struct {
	CompanyID   string `json:"companyID"`
	AccessToken string `json:"accessToken"`
	ExpiresAt   int64  `json:"expiresAt"`
}

type Session struct {
	Configuration  Configuration   `json:"configuration"`
	Session        HostSession     `json:"session"`
	SealedModelKey *SealedModelKey `json:"sealedModelKey"`
}

type Client struct {
	AppURL     string
	HTTPClient *http.Client
	Now        func() time.Time
}

func (client Client) Announce(ctx context.Context, identity Identity) (bool, error) {
	body, errorValue := json.Marshal(map[string]string{"encryptionKey": identity.EncryptionPublicKey()})
	if errorValue != nil {
		return false, errorValue
	}
	response, errorValue := client.post(ctx, identity, "/api/box/announce", body)
	if errorValue != nil {
		return false, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, refusalOf(response, "announcing this box")
	}
	var announced struct {
		IsClaimed bool `json:"isClaimed"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&announced); errorValue != nil {
		return false, fmt.Errorf("announcing this box: %w", errorValue)
	}
	return announced.IsClaimed, nil
}

func (client Client) Session(ctx context.Context, identity Identity) (Session, bool, error) {
	response, errorValue := client.post(ctx, identity, "/api/box/session", nil)
	if errorValue != nil {
		return Session{}, false, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return Session{}, false, nil
	}
	if response.StatusCode != http.StatusOK {
		return Session{}, false, refusalOf(response, "asking for this box's session")
	}
	var session Session
	if errorValue := json.NewDecoder(response.Body).Decode(&session); errorValue != nil {
		return Session{}, false, fmt.Errorf("asking for this box's session: %w", errorValue)
	}
	return session, true, nil
}

func (client Client) post(ctx context.Context, identity Identity, path string, body []byte) (*http.Response, error) {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(client.AppURL, "/")+path, bytes.NewReader(body))
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+identity.Assertion(client.now()))
	request.Header.Set("Content-Type", "application/json")
	return client.httpClient().Do(request)
}

func (client Client) now() time.Time {
	if client.Now == nil {
		return time.Now()
	}
	return client.Now()
}

func (client Client) httpClient() *http.Client {
	if client.HTTPClient == nil {
		return &http.Client{Timeout: 15 * time.Second}
	}
	return client.HTTPClient
}

func refusalOf(response *http.Response, doing string) error {
	detail, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	return fmt.Errorf("%s: the central plane answered %d: %s", doing, response.StatusCode, strings.TrimSpace(string(detail)))
}
