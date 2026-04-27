package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/anthropic-lab/internkim/internal/capabilities"
)

func main() {
	listenAddress := flag.String("listen", "127.0.0.1:7979", "companion listen address")
	localOnly := flag.Bool("local-only", false, "advertise local-only mode")
	devMockLLM := flag.Bool("dev-mock-llm", false, "serve deterministic local LLM responses for development")
	flag.Parse()

	multiplexer := http.NewServeMux()
	multiplexer.HandleFunc("GET /health", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = request
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok\n"))
	})
	multiplexer.HandleFunc("GET /v1/capabilities", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = request
		writeJSON(responseWriter, capabilities.RegistryResponse{
			LocalOnly:    *localOnly,
			Capabilities: defaultCapabilities(*localOnly, *devMockLLM),
		})
	})
	multiplexer.HandleFunc("POST /v1/llm/structured", llmHandler(*devMockLLM, true))
	multiplexer.HandleFunc("POST /v1/llm/text", llmHandler(*devMockLLM, false))
	multiplexer.HandleFunc("POST /v1/tools/invoke", notImplemented)
	multiplexer.HandleFunc("POST /v1/tools/{toolName}/invoke", notImplemented)

	if errorValue := http.ListenAndServe(*listenAddress, multiplexer); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue.Error())
		os.Exit(1)
	}
}

func defaultCapabilities(localOnly bool, devMockLLM bool) []capabilities.Descriptor {
	descriptors := capabilities.CompanionToolDescriptors()
	for index, descriptor := range descriptors {
		if strings.HasPrefix(descriptor.Name, "browser.") {
			descriptors[index].WorksOffline = localOnly
		}
	}
	if devMockLLM {
		descriptors = append(descriptors, capabilities.CompanionLLMDescriptors()...)
	}
	return descriptors
}

func llmHandler(isEnabled bool, isStructured bool) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if !isEnabled {
			notImplemented(responseWriter, request)
			return
		}
		response := map[string]any{
			"provider":        "companion",
			"model":           "mock-local",
			"selectedBackend": capabilities.LLMBackendCompanionLocal,
			"constraintMode":  "prompt_validation",
			"content":         "ok",
		}
		if isStructured {
			response["content"] = mockStructuredContent(request)
		}
		writeJSON(responseWriter, response)
	}
}

func mockStructuredContent(request *http.Request) string {
	var document struct {
		StructuredOutputSchema struct {
			Document struct {
				Required []string `json:"required"`
			} `json:"document"`
		} `json:"structuredOutputSchema"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
		return `{"content":"ok"}`
	}
	values := map[string]string{}
	for _, key := range document.StructuredOutputSchema.Document.Required {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey != "" {
			values[trimmedKey] = "ok"
		}
	}
	if len(values) == 0 {
		values["content"] = "ok"
	}
	response, errorValue := json.Marshal(values)
	if errorValue != nil {
		return `{"content":"ok"}`
	}
	return string(response)
}

func notImplemented(responseWriter http.ResponseWriter, request *http.Request) {
	_ = request
	http.Error(responseWriter, "companion capability is not implemented in this daemon slice", http.StatusNotImplemented)
}

func writeJSON(responseWriter http.ResponseWriter, response any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(response)
}
