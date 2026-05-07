package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type runtimeRemoteModelResponse struct {
	Model       string `json:"model"`
	Restarted   bool   `json:"restarted,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	RuntimePath string `json:"runtimePath,omitempty"`
}

type runtimeRemoteModelUpdateRequest struct {
	Model string `json:"model"`
}

func (service *Service) handleRuntime(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/_internkim/runtime")
	switch {
	case request.Method == http.MethodGet && path == "/remote-model":
		service.readRuntimeRemoteModel(responseWriter, request)
	case request.Method == http.MethodPut && path == "/remote-model":
		service.updateRuntimeRemoteModel(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) readRuntimeRemoteModel(responseWriter http.ResponseWriter, request *http.Request) {
	if companion := service.authorizedCompanion(request); companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	model, errorValue := service.readConfiguredRemoteModel()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, runtimeRemoteModelResponse{
		Model:       model,
		RuntimePath: service.blueclawRuntimeConfigPath(),
	})
}

func (service *Service) updateRuntimeRemoteModel(responseWriter http.ResponseWriter, request *http.Request) {
	if companion := service.authorizedCompanion(request); companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	var payload runtimeRemoteModelUpdateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	model := strings.TrimSpace(payload.Model)
	if model == "" {
		http.Error(responseWriter, "model is required", http.StatusBadRequest)
		return
	}
	if errorValue := service.writeRemoteModel(request.Context(), model); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, runtimeRemoteModelResponse{
		Model:       model,
		Restarted:   true,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		RuntimePath: service.blueclawRuntimeConfigPath(),
	})
}

func (service *Service) readConfiguredRemoteModel() (string, error) {
	document, errorValue := service.readBlueclawRuntimeDocument()
	if errorValue != nil {
		return "", errorValue
	}
	return remoteModelFromRuntimeDocument(document), nil
}

func (service *Service) writeRemoteModel(ctx context.Context, model string) error {
	document, errorValue := service.readBlueclawRuntimeDocument()
	if errorValue != nil {
		return errorValue
	}
	setRemoteModelInRuntimeDocument(document, model)
	if errorValue := service.writeBlueclawRuntimeDocument(document); errorValue != nil {
		return errorValue
	}
	_, errorValue = service.runCommand(ctx, "systemctl", "restart", blueclaw.BlueclawServiceName)
	return errorValue
}

func (service *Service) readBlueclawRuntimeDocument() (map[string]any, error) {
	documentBytes, errorValue := os.ReadFile(service.blueclawRuntimeConfigPath())
	if errorValue != nil {
		return nil, errorValue
	}
	var document map[string]any
	if errorValue := json.Unmarshal(documentBytes, &document); errorValue != nil {
		return nil, errorValue
	}
	return document, nil
}

func (service *Service) writeBlueclawRuntimeDocument(document map[string]any) error {
	documentBytes, errorValue := json.MarshalIndent(document, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	path := service.blueclawRuntimeConfigPath()
	if errorValue := os.WriteFile(path, append(documentBytes, '\n'), 0o640); errorValue != nil {
		return errorValue
	}
	return nil
}

func (service *Service) blueclawRuntimeConfigPath() string {
	if strings.TrimSpace(service.Configuration.BlueclawRuntimeConfigPath) != "" {
		return service.Configuration.BlueclawRuntimeConfigPath
	}
	return blueclaw.BlueclawRuntimeConfigPath
}

func remoteModelFromRuntimeDocument(document map[string]any) string {
	languageModel, _ := document["languageModel"].(map[string]any)
	capabilityModel, _ := languageModel["capability"].(map[string]any)
	model, _ := capabilityModel["model"].(string)
	return strings.TrimSpace(model)
}

func setRemoteModelInRuntimeDocument(document map[string]any, model string) {
	languageModel, _ := document["languageModel"].(map[string]any)
	if languageModel == nil {
		languageModel = map[string]any{}
		document["languageModel"] = languageModel
	}
	capabilityModel, _ := languageModel["capability"].(map[string]any)
	if capabilityModel == nil {
		capabilityModel = map[string]any{}
		languageModel["capability"] = capabilityModel
	}
	capabilityModel["model"] = strings.TrimSpace(model)
}
