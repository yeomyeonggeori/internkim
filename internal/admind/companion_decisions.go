package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
)

// The companion has no inference key, so its computer tasks put every decision
// to the device, which asks the decision model through capabilityd.
type companionDecisionRequest struct {
	State     json.RawMessage                        `json:"state"`
	Questions map[string]llmbackend.DecisionQuestion `json:"questions"`
	SessionID string                                 `json:"sessionID,omitempty"`
}

const largestCompanionDecisionBytes = 1 << 20

func (service *Service) decideForCompanion(responseWriter http.ResponseWriter, request *http.Request) {
	companion := service.authorizedCompanion(request)
	if companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	var payload companionDecisionRequest
	if errorValue := json.NewDecoder(io.LimitReader(request.Body, largestCompanionDecisionBytes)).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := validateCompanionDecisionRequest(payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload.SessionID = "companion:" + companion.CompanionID
	responseDocument, errorValue := service.callCapabilityDecide(request.Context(), payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_, _ = responseWriter.Write(responseDocument)
}

func validateCompanionDecisionRequest(payload companionDecisionRequest) error {
	if len(bytes.TrimSpace(payload.State)) == 0 {
		return errors.New("a decision needs a state")
	}
	if len(payload.Questions) == 0 {
		return errors.New("a decision needs at least one question")
	}
	return nil
}

func (service *Service) callCapabilityDecide(ctx context.Context, payload companionDecisionRequest) ([]byte, error) {
	requestDocument, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, "http://internkim/v1/llm/decide", bytes.NewReader(requestDocument))
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpClient := service.capabilitySocketClient()
	httpResponse, errorValue := httpClient.Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	responseDocument, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return nil, readError
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return nil, fmt.Errorf("the decision model could not be asked: %s", strings.TrimSpace(string(responseDocument)))
	}
	return responseDocument, nil
}
