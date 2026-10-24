package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const relayHandoffsPath = "/browser-handoffs"

type relayHandoffRegistration struct {
	HandoffID      string `json:"handoffID"`
	RequesterEmail string `json:"requesterEmail"`
	DevtoolsURL    string `json:"devtoolsURL"`
	SignInOrigin   string `json:"signInOrigin"`
	ExpiresAt      string `json:"expiresAt"`
}

type relayHandoffStatus struct {
	Status       string `json:"status"`
	SignInOrigin string `json:"signInOrigin"`
}

const (
	relayHandoffWaiting   = "waiting"
	relayHandoffOpen      = "open"
	relayHandoffDone      = "done"
	relayHandoffAbandoned = "abandoned"
)

func (status relayHandoffStatus) hasEnded() bool {
	return status.Status != relayHandoffWaiting && status.Status != relayHandoffOpen
}

func (status relayHandoffStatus) endedAs() string {
	switch status.Status {
	case relayHandoffDone, relayHandoffAbandoned:
		return status.Status
	}
	return "expired"
}

func (service Service) registerHandoffWithTheRelay(ctx context.Context, handoff openHandoff, devtoolsURL string) error {
	document, errorValue := json.Marshal(relayHandoffRegistration{
		HandoffID:      handoff.handoffID,
		RequesterEmail: handoff.requester,
		DevtoolsURL:    devtoolsURL,
		SignInOrigin:   handoff.signInOrigin,
		ExpiresAt:      handoff.expiresAt.UTC().Format(time.RFC3339),
	})
	if errorValue != nil {
		return errorValue
	}
	address := service.relayHandoffsURL()
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return fmt.Errorf("the relay at %s did not take the hand-over: %w", address, errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		answer, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("the relay at %s answered %d to the hand-over: %s", address, response.StatusCode, strings.TrimSpace(string(answer)))
	}
	return nil
}

func (service Service) handoffStatusAtTheRelay(ctx context.Context, handoffID string) (relayHandoffStatus, error) {
	address := service.relayHandoffsURL() + "/" + url.PathEscape(handoffID)
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if errorValue != nil {
		return relayHandoffStatus{}, errorValue
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return relayHandoffStatus{}, fmt.Errorf("the relay at %s could not be asked about the hand-over: %w", address, errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return relayHandoffStatus{Status: "unknown"}, nil
	}
	if response.StatusCode != http.StatusOK {
		return relayHandoffStatus{}, fmt.Errorf("the relay at %s answered %d about the hand-over", address, response.StatusCode)
	}
	var status relayHandoffStatus
	if errorValue := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&status); errorValue != nil {
		return relayHandoffStatus{}, fmt.Errorf("the relay answered about the hand-over in a shape it never uses: %w", errorValue)
	}
	return status, nil
}

func (service Service) relayHandoffsURL() string {
	return strings.TrimRight(service.Configuration.WithDefaults().RelayBaseURL, "/") + relayHandoffsPath
}
