package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

type directoryPersonRequest struct {
	Platform       string `json:"platform"`
	ExternalUserID string `json:"externalUserID"`
	Email          string `json:"email"`
}

type directoryPersonResponse struct {
	Known bool `json:"known"`
}

// directoryLookupBudget bounds the wait an inbound message inherits from this lookup.
// Somebody is waiting on the answer, so a slow directory gives up rather than holding
// the turn that asked.
const directoryLookupBudget = 5 * time.Second

// The agent asks this when it cannot match an account to anyone it carries. The company
// directory is the host's to read, and admind is where a person is projected onto the
// agent, so this forwards rather than deciding anything itself.
func (service Service) handleDirectoryPerson(responseWriter http.ResponseWriter, request *http.Request) {
	var payload directoryPersonRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	known, errorValue := service.askAdmindAboutPerson(request.Context(), payload)
	service.writeResponse(responseWriter, directoryPersonResponse{Known: known}, errorValue)
}

func (service Service) askAdmindAboutPerson(ctx context.Context, payload directoryPersonRequest) (bool, error) {
	body, errorValue := json.Marshal(map[string]string{
		"email":    strings.TrimSpace(payload.Email),
		"platform": strings.TrimSpace(payload.Platform),
	})
	if errorValue != nil {
		return false, errorValue
	}
	requestContext, cancel := context.WithTimeout(ctx, directoryLookupBudget)
	defer cancel()
	request, errorValue := http.NewRequestWithContext(requestContext, http.MethodPost,
		strings.TrimRight(service.Configuration.AdmindBaseURL, "/")+"/admin/api/directory/person",
		bytes.NewReader(body))
	if errorValue != nil {
		return false, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := http.DefaultClient.Do(request)
	if errorValue != nil {
		return false, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return false, errorFromStatus(response)
	}
	var answer directoryPersonResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return false, errorValue
	}
	return answer.Known, nil
}

func errorFromStatus(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	return &directoryLookupError{status: response.Status, detail: strings.TrimSpace(string(body))}
}

type directoryLookupError struct {
	status string
	detail string
}

func (lookupError *directoryLookupError) Error() string {
	if lookupError.detail == "" {
		return "the company directory answered " + lookupError.status
	}
	return "the company directory answered " + lookupError.status + ": " + lookupError.detail
}
