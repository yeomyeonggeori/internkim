package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/eastriver/internkim/internal/modelladder"
)

// OpenRouter serves decision-only models on its alpha decisions route, not on
// chat completions. github.com/OpenRouterTeam/go-sdk decisions.go
const openRouterDecisionsPath = "/api/alpha/decisions"

const (
	ChoiceQuestionType = "choice"
	NoulQuestionType   = "noul"
)

type DecisionQuestion struct {
	Type         string          `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
}

func ChoiceQuestion(instructions string, criteriaByOption map[string]string) DecisionQuestion {
	return DecisionQuestion{Type: ChoiceQuestionType, Instructions: instructions, Criteria: criteriaDocument(criteriaByOption)}
}

func NoulQuestion(instructions string, criteriaByOutcome map[string]string) DecisionQuestion {
	return DecisionQuestion{Type: NoulQuestionType, Instructions: instructions, Criteria: criteriaDocument(criteriaByOutcome)}
}

func criteriaDocument(criteria map[string]string) json.RawMessage {
	if len(criteria) == 0 {
		return nil
	}
	document, _ := json.Marshal(criteria)
	return document
}

type DecisionsRequest struct {
	Model     string                      `json:"model"`
	State     json.RawMessage             `json:"state"`
	Questions map[string]DecisionQuestion `json:"questions"`
}

type DecisionsUsage struct {
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost"`
}

type DecisionsResponse struct {
	ID       string                    `json:"id"`
	Model    string                    `json:"model"`
	Provider string                    `json:"provider"`
	Answers  map[string]DecisionAnswer `json:"answers"`
	Usage    DecisionsUsage            `json:"usage"`
}

type DecisionAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Noul          *float64           `json:"noul,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
}

type ChoiceAnswer struct {
	Choice        string
	Probabilities map[string]float64
	Confidence    float64
}

type NoulAnswer struct {
	Noul float64
}

func (answer DecisionAnswer) AsChoice() (ChoiceAnswer, bool) {
	if answer.Type != ChoiceQuestionType || strings.TrimSpace(answer.Choice) == "" {
		return ChoiceAnswer{}, false
	}
	return ChoiceAnswer{
		Choice:        answer.Choice,
		Probabilities: answer.Probabilities,
		Confidence:    decisionValue(answer.Confidence),
	}, true
}

func (answer DecisionAnswer) AsNoul() (NoulAnswer, bool) {
	if answer.Type != NoulQuestionType || answer.Noul == nil {
		return NoulAnswer{}, false
	}
	return NoulAnswer{Noul: *answer.Noul}, true
}

func decisionValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func (backend OpenRouterBackend) Decide(ctx context.Context, request DecisionsRequest) (DecisionsResponse, error) {
	apiKey, errorValue := backend.resolveAPIKey()
	if errorValue != nil {
		return DecisionsResponse{}, errorValue
	}
	if errorValue := validateDecisionsRequest(request); errorValue != nil {
		return DecisionsResponse{}, errorValue
	}
	request.Model = decisionModelName(request.Model)
	requestDocument, errorValue := json.Marshal(request)
	if errorValue != nil {
		return DecisionsResponse{}, errorValue
	}
	responseDocument, errorValue := backend.sendDecisions(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return DecisionsResponse{}, errorValue
	}
	return parseDecisionsResponse(responseDocument)
}

func validateDecisionsRequest(request DecisionsRequest) error {
	if len(bytes.TrimSpace(request.State)) == 0 {
		return errors.New("decision request needs a state")
	}
	if len(request.Questions) == 0 {
		return errors.New("decision request needs at least one question")
	}
	return nil
}

func decisionModelName(requestedModel string) string {
	if named := strings.TrimSpace(requestedModel); named != "" {
		return named
	}
	return modelladder.DecisionModel
}

func (backend OpenRouterBackend) sendDecisions(ctx context.Context, apiKey string, requestDocument []byte) ([]byte, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, backend.decisionsURL(), bytes.NewReader(requestDocument))
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	backend.setGatewaySecretHeader(httpRequest)

	httpResponse, errorValue := backend.HTTPClient.Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return nil, errors.New("read openrouter decisions response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, normalizeProviderError("openrouter decisions", httpResponse.StatusCode, responseDocument)
	}
	return responseDocument, nil
}

func parseDecisionsResponse(responseDocument []byte) (DecisionsResponse, error) {
	var response DecisionsResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return DecisionsResponse{}, errors.New("openrouter decisions response could not be read: " + errorValue.Error())
	}
	if len(response.Answers) == 0 {
		return DecisionsResponse{}, errors.New("openrouter decisions response carried no answers")
	}
	return response, nil
}

func (backend OpenRouterBackend) decisionsURL() string {
	origin := originOf(backend.BaseURL)
	if origin == "" {
		origin = originOf(modelladder.Endpoint)
	}
	return origin + openRouterDecisionsPath
}

func originOf(rawURL string) string {
	parsed, errorValue := url.Parse(strings.TrimSpace(rawURL))
	if errorValue != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
