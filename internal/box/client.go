package box

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	CompanyID    string `json:"companyID"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"`
}

var ErrRefreshRefused = errors.New("the project refused this refresh token")

type Session struct {
	Configuration Configuration `json:"configuration"`
	Session       HostSession   `json:"session"`
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

func (client Client) Claim(ctx context.Context, identity Identity, connectionKey string) error {
	body, errorValue := json.Marshal(map[string]string{"encryptionKey": identity.EncryptionPublicKey(), "connectionKey": connectionKey})
	if errorValue != nil {
		return errorValue
	}
	response, errorValue := client.post(ctx, identity, "/api/box/claim", body)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return refusalOf(response, "claiming this computer with the connection file")
	}
	return nil
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

func (client Client) Refresh(ctx context.Context, plane companyhost.CentralPlane, refreshToken string) (HostSession, error) {
	body, errorValue := json.Marshal(map[string]string{"refresh_token": refreshToken})
	if errorValue != nil {
		return HostSession{}, errorValue
	}
	address := strings.TrimRight(plane.ProjectURL, "/") + "/auth/v1/token?grant_type=refresh_token"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(body))
	if errorValue != nil {
		return HostSession{}, errorValue
	}
	request.Header.Set("apikey", plane.PublishableKey)
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := client.httpClient().Do(request)
	if errorValue != nil {
		return HostSession{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return HostSession{}, fmt.Errorf("%w: %w", ErrRefreshRefused, refusalOf(response, "renewing this box's session"))
	}
	if response.StatusCode != http.StatusOK {
		return HostSession{}, refusalOf(response, "renewing this box's session")
	}
	var renewed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresAt    int64  `json:"expires_at"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&renewed); errorValue != nil {
		return HostSession{}, fmt.Errorf("renewing this box's session: %w", errorValue)
	}
	return HostSession{AccessToken: renewed.AccessToken, RefreshToken: renewed.RefreshToken, ExpiresAt: renewed.ExpiresAt}, nil
}

func (client Client) SealedModelKey(ctx context.Context, plane companyhost.CentralPlane, accessToken string) (*SealedModelKey, error) {
	address := strings.TrimRight(plane.ProjectURL, "/") + "/rest/v1/rpc/box_sealed_model_key"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, address, strings.NewReader("{}"))
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("apikey", plane.PublishableKey)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := client.httpClient().Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, refusalOf(response, "reading this box's model key")
	}
	var sealedModelKey *SealedModelKey
	if errorValue := json.NewDecoder(response.Body).Decode(&sealedModelKey); errorValue != nil {
		return nil, fmt.Errorf("reading this box's model key: %w", errorValue)
	}
	return sealedModelKey, nil
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
