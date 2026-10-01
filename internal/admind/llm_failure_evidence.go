package admind

import (
	"errors"
	"net/http"
	"os"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
)

func (service *Service) writeLLMFailureEvidence(responseWriter http.ResponseWriter, request *http.Request) {
	document, errorValue := llmbackend.ReadFailureEvidence(service.Configuration.BlueclawWorkspacePath, request.URL.Query().Get("id"))
	if errorValue != nil {
		statusCode := http.StatusBadRequest
		if errors.Is(errorValue, os.ErrNotExist) {
			statusCode = http.StatusNotFound
		}
		http.Error(responseWriter, "llm failure evidence is unavailable", statusCode)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Header().Set("Cache-Control", "no-store")
	responseWriter.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = responseWriter.Write(document)
}
