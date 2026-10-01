package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
)

type DecideLLMRequest struct {
	Model     string                                 `json:"model,omitempty"`
	State     json.RawMessage                        `json:"state"`
	Questions map[string]llmbackend.DecisionQuestion `json:"questions"`
	SessionID string                                 `json:"sessionID,omitempty"`
}

type DecideLLMUsage struct {
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CostUSD      float64 `json:"costUSD"`
}

type DecideLLMResponse struct {
	Answers             map[string]llmbackend.DecisionAnswer `json:"answers"`
	Usage               DecideLLMUsage                       `json:"usage"`
	ModelName           string                               `json:"modelName"`
	ProviderName        string                               `json:"providerName"`
	UpstreamProvider    string                               `json:"upstreamProvider,omitempty"`
	LatencyMilliseconds int64                                `json:"latencyMs"`
}

func (service Service) decide(ctx context.Context, request DecideLLMRequest) (DecideLLMResponse, error) {
	if service.Configuration.LocalOnly {
		return DecideLLMResponse{}, errors.New("decisions run on the remote decision model; local-only mode has none")
	}
	backend := service.openRouterBackend()
	startedAt := time.Now()
	decisions, errorValue := backend.Decide(ctx, llmbackend.DecisionsRequest{
		Model:     request.Model,
		State:     request.State,
		Questions: request.Questions,
	})
	if errorValue != nil {
		log.Printf("llm.decide_failed: sessionID=%s questions=%d error=%v", request.SessionID, len(request.Questions), errorValue)
		return DecideLLMResponse{}, errorValue
	}
	return decideResponseOf(decisions, backend.Name(), time.Since(startedAt)), nil
}

func decideResponseOf(decisions llmbackend.DecisionsResponse, providerName string, latency time.Duration) DecideLLMResponse {
	return DecideLLMResponse{
		Answers: decisions.Answers,
		Usage: DecideLLMUsage{
			InputTokens:  decisions.Usage.InputTokens,
			OutputTokens: decisions.Usage.OutputTokens,
			CostUSD:      decisions.Usage.CostUSD,
		},
		ModelName:           decisions.Model,
		ProviderName:        providerName,
		UpstreamProvider:    decisions.Provider,
		LatencyMilliseconds: latency.Milliseconds(),
	}
}
