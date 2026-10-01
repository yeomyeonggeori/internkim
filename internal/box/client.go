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

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
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

type Announcement struct {
	IsClaimed   bool
	PairingCode *PairingCode
}

func (client Client) Announce(ctx context.Context, identity Identity, wantsPairingCode bool) (Announcement, error) {
	body, errorValue := json.Marshal(map[string]any{
		"encryptionKey":    identity.EncryptionPublicKey(),
		"wantsPairingCode": wantsPairingCode,
	})
	if errorValue != nil {
		return Announcement{}, errorValue
	}
	response, errorValue := client.post(ctx, identity, "/api/box/announce", body)
	if errorValue != nil {
		return Announcement{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Announcement{}, refusalOf(response, "announcing this box")
	}
	var announced struct {
		IsClaimed            bool      `json:"isClaimed"`
		PairingCode          string    `json:"pairingCode"`
		PairingCodeExpiresAt time.Time `json:"pairingCodeExpiresAt"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&announced); errorValue != nil {
		return Announcement{}, fmt.Errorf("announcing this box: %w", errorValue)
	}
	if announced.PairingCode == "" {
		return Announcement{IsClaimed: announced.IsClaimed}, nil
	}
	return Announcement{
		IsClaimed:   announced.IsClaimed,
		PairingCode: &PairingCode{Code: announced.PairingCode, ExpiresAt: announced.PairingCodeExpiresAt},
	}, nil
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
