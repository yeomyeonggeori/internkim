package main

import (
	"encoding/json"
	"io"
	"net/http"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

func llmHandler(settings *dynamicLocalLLM, devMock bool, isStructured bool) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if devMock {
			respondWithDevMock(responseWriter, request, isStructured)
			return
		}
		if !settings.currentSettings().Enabled {
			notImplemented(responseWriter, request)
			return
		}
		if isStructured {
			handleStructuredLLM(responseWriter, request, settings)
			return
		}
		handleTextLLM(responseWriter, request, settings)
	}
}

func respondWithDevMock(responseWriter http.ResponseWriter, request *http.Request, isStructured bool) {
	response := map[string]any{
		"provider":        "companion",
		"model":           "mock-local",
		"selectedBackend": capabilities.LLMBackendCompanionLocal,
		"constraintMode":  llmbackend.ConstraintModeOpenAIJSONSchema,
		"content":         "ok",
	}
	if isStructured {
		document, _ := io.ReadAll(request.Body)
		response["content"] = companionruntime.MockStructuredContent(document)
	}
	writeJSON(responseWriter, response)
}

func handleStructuredLLM(responseWriter http.ResponseWriter, request *http.Request, settings *dynamicLocalLLM) {
	var structuredRequest llmbackend.StructuredRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&structuredRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	providerSet, isEnabled := settings.providerSetFor(structuredRequest.Provider, structuredRequest.Accelerator)
	if !isEnabled {
		notImplemented(responseWriter, request)
		return
	}
	response, errorValue := providerSet.Provider.CompleteStructured(request.Context(), structuredRequest)
	if errorValue != nil {
		respondWithBackendError(responseWriter, errorValue, providerSet.Backends)
		return
	}
	response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	writeJSON(responseWriter, response)
}

func handleTextLLM(responseWriter http.ResponseWriter, request *http.Request, settings *dynamicLocalLLM) {
	var textRequest llmbackend.TextRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&textRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	providerSet, isEnabled := settings.providerSetFor(textRequest.Provider, textRequest.Accelerator)
	if !isEnabled {
		notImplemented(responseWriter, request)
		return
	}
	response, errorValue := providerSet.Provider.CompleteText(request.Context(), textRequest)
	if errorValue != nil {
		respondWithBackendError(responseWriter, errorValue, providerSet.Backends)
		return
	}
	response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	writeJSON(responseWriter, response)
}

func embeddingHandler(settings *dynamicLocalLLM, devMock bool) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if devMock {
			writeJSON(responseWriter, map[string]any{
				"provider":        "companion",
				"model":           llmbackend.DefaultEmbeddingModelName,
				"selectedBackend": capabilities.LLMBackendCompanionLocal,
				"embedding":       []float64{1, 0, 0},
			})
			return
		}
		if !settings.currentSettings().Enabled {
			notImplemented(responseWriter, request)
			return
		}
		handleEmbedding(responseWriter, request, settings)
	}
}

func handleEmbedding(responseWriter http.ResponseWriter, request *http.Request, settings *dynamicLocalLLM) {
	var embeddingRequest llmbackend.EmbeddingRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&embeddingRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	providerSet, isEnabled := settings.embeddingProviderSetFor(embeddingRequest.Provider)
	if !isEnabled {
		notImplemented(responseWriter, request)
		return
	}
	response, errorValue := providerSet.Provider.CreateEmbedding(request.Context(), embeddingRequest)
	if errorValue != nil {
		respondWithEmbeddingBackendError(responseWriter, errorValue, providerSet.Backends)
		return
	}
	response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	writeJSON(responseWriter, response)
}

func respondWithBackendError(responseWriter http.ResponseWriter, errorValue error, backends []llmbackend.Backend) {
	hint := buildBackendHint(backends)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusServiceUnavailable)
	writeJSONDocument(responseWriter, map[string]any{
		"error": errorValue.Error(),
		"hint":  hint,
	})
}

func respondWithEmbeddingBackendError(responseWriter http.ResponseWriter, errorValue error, backends []llmbackend.EmbeddingBackend) {
	hint := buildEmbeddingBackendHint(backends)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusServiceUnavailable)
	writeJSONDocument(responseWriter, map[string]any{
		"error": errorValue.Error(),
		"hint":  hint,
	})
}

func buildBackendHint(backends []llmbackend.Backend) string {
	if len(backends) == 0 {
		return "no local backend configured"
	}
	return "check that one of these backends is reachable: " + localLLMProviderNames(backends)
}

func buildEmbeddingBackendHint(backends []llmbackend.EmbeddingBackend) string {
	if len(backends) == 0 {
		return "no local embedding backend configured"
	}
	return "check that one of these embedding backends is reachable: " + localEmbeddingProviderNames(backends)
}

func notImplemented(responseWriter http.ResponseWriter, request *http.Request) {
	_ = request
	http.Error(responseWriter, "companion capability is not implemented in this daemon slice", http.StatusNotImplemented)
}

func reservedNotImplemented(responseWriter http.ResponseWriter, request *http.Request) {
	_ = request
	http.Error(responseWriter, "reserved endpoint; not yet implemented", http.StatusNotImplemented)
}

func llmStreamHandler(settings *dynamicLocalLLM) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		var textRequest llmbackend.TextRequest
		if errorValue := json.NewDecoder(request.Body).Decode(&textRequest); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		providerSet, isEnabled := settings.providerSetFor(textRequest.Provider, textRequest.Accelerator)
		if !isEnabled {
			notImplemented(responseWriter, request)
			return
		}
		flusher, supportsFlush := responseWriter.(http.Flusher)
		if !supportsFlush {
			http.Error(responseWriter, "streaming not supported by responder", http.StatusInternalServerError)
			return
		}
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		responseWriter.Header().Set("Cache-Control", "no-cache")
		responseWriter.Header().Set("Connection", "keep-alive")
		errorValue := llmbackend.StreamFirstStreamingBackend(request.Context(), providerSet.Backends, textRequest, func(token string) {
			payload, _ := json.Marshal(map[string]string{"token": token})
			_, _ = responseWriter.Write([]byte("data: "))
			_, _ = responseWriter.Write(payload)
			_, _ = responseWriter.Write([]byte("\n\n"))
			flusher.Flush()
		})
		if errorValue != nil {
			payload, _ := json.Marshal(map[string]string{"error": errorValue.Error()})
			_, _ = responseWriter.Write([]byte("event: error\ndata: "))
			_, _ = responseWriter.Write(payload)
			_, _ = responseWriter.Write([]byte("\n\n"))
			flusher.Flush()
			return
		}
		_, _ = responseWriter.Write([]byte("event: done\ndata: {}\n\n"))
		flusher.Flush()
	}
}
