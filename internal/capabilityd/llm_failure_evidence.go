package capabilityd

import (
	"fmt"
	"net/http"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

func (service Service) writeLLMResponse(responseWriter http.ResponseWriter, response any, failure error, request any, capture *llmbackend.FailureCapture) {
	if failure == nil || service.Configuration.BlueclawWorkspacePath == "" {
		service.writeResponse(responseWriter, response, failure)
		return
	}
	identifier, writeError := llmbackend.WriteFailureEvidence(service.Configuration.BlueclawWorkspacePath, request, capture)
	if writeError != nil {
		service.writeResponse(responseWriter, response, fmt.Errorf("llm failure evidence could not be saved: %v; %w", writeError, failure))
		return
	}
	evidenceURL := "/admin/api/diagnostics/llm-failure?id=" + identifier
	service.writeResponse(responseWriter, response, fmt.Errorf("llm failure evidence: %s; %w", evidenceURL, failure))
}
