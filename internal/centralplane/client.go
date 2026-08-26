package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	DefaultProjectURL     = "https://mutvimjbvmoludotyehk.supabase.co"
	DefaultPublishableKey = "sb_publishable_HwxbmtaLFeHiOAZLIUvIZw_V17mbX44"
)

type Settings struct {
	AppURL         string
	AgentAPIKey    string
	ProjectURL     string
	PublishableKey string
	HTTPClient     *http.Client
}

func (settings Settings) Configured() bool {
	return strings.TrimSpace(settings.AppURL) != "" &&
		strings.TrimSpace(settings.AgentAPIKey) != "" &&
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
		httpClient: httpClient,
		sessions:   map[string]memberSession{},
	}
}

type AttendanceRecord struct {
	Platform   string
	ExternalID string
	Kind       string
	Location   string
	OccurredAt time.Time
}

func (client *Client) RecordAttendance(ctx context.Context, record AttendanceRecord) error {
	session, errorValue := client.sessionFor(ctx, record.Platform, record.ExternalID)
	if errorValue != nil {
		return errorValue
	}

	body := map[string]any{
		"member_id":   session.memberID,
		"kind":        record.Kind,
		"occurred_at": record.OccurredAt.UTC().Format(time.RFC3339Nano),
	}
	if record.Kind == "clock_in" && strings.TrimSpace(record.Location) != "" {
		body["location"] = record.Location
	}
	payload, errorValue := json.Marshal(body)
	if errorValue != nil {
		return errorValue
	}

	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/attendance", bytes.NewReader(payload))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("apikey", client.settings.PublishableKey)
	request.Header.Set("Authorization", "Bearer "+session.accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("central plane refused the attendance record: %s", response.Status)
	}
	return nil
}

// A company that will not seat an identity is not a company that is broken. It
// is saying this person is not one of ours, and the caller belongs on the
// device rather than at a bad gateway.
var ErrIdentityNotHeld = errors.New("the central plane holds no session for that identity")

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
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return memberSession{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return memberSession{}, fmt.Errorf("%w: %s identity %s: %s", ErrIdentityNotHeld, platform, externalID, response.Status)
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
