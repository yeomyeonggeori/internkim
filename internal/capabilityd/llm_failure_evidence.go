package capabilityd

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
)

func (service Service) writeLLMResponse(responseWriter http.ResponseWriter, response any, failure error, request any, capture *llmbackend.ExchangeCapture) {
	if failure == nil {
		service.writeJSON(responseWriter, withWireExchange(response, capture))
		return
	}
	if service.Configuration.BlueclawWorkspacePath == "" {
		service.writeResponse(responseWriter, response, failure)
		return
	}
	identifier, writeError := llmbackend.WriteFailureEvidence(service.Configuration.BlueclawWorkspacePath, request, capture)
	if writeError != nil {
		service.writeResponse(responseWriter, response, fmt.Errorf("llm failure evidence could not be saved: %v; %w", writeError, failure))
		return
	}
	service.writeResponse(responseWriter, response, fmt.Errorf("%s%s; %w", llmbackend.FailureEvidenceMarker, identifier, failure))
}

func withWireExchange(response any, capture *llmbackend.ExchangeCapture) any {
	wireExchange, isAnswered := capture.Snapshot().AnsweringExchange()
	if !isAnswered {
		return response
	}
	document, errorValue := json.Marshal(response)
	if errorValue != nil {
		return response
	}
	fields := map[string]json.RawMessage{}
	if json.Unmarshal(document, &fields) != nil {
		return response
	}
	fields["wireExchange"], _ = json.Marshal(wireExchange)
	return fields
}
