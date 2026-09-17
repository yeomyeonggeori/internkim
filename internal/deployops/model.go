package deployops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	runtimeRemoteModelReadAction   = "runtime-remote-model-read"
	runtimeRemoteModelUpdateAction = "runtime-remote-model-update"
)

type runtimeRemoteModelRequest struct {
	signedAdminRequest
	Model string `json:"model,omitempty"`
}

type runtimeRemoteModelResponse struct {
	Model       string `json:"model"`
	Restarted   bool   `json:"restarted,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	RuntimePath string `json:"runtimePath,omitempty"`
}

func (server *Server) ReadLLMModel(contextValue context.Context, target Target) LLMModelStatus {
	response, errorValue := server.performLLMModelRequest(contextValue, target, runtimeRemoteModelReadAction, "")
	if errorValue != nil {
		return LLMModelStatus{State: "failed", Message: Redact(errorValue.Error())}
	}
	return LLMModelStatus{
		State:       "ok",
		Model:       response.Model,
		RuntimePath: response.RuntimePath,
	}
}

func (server *Server) UpdateLLMModel(contextValue context.Context, target Target, model string) (LLMModelStatus, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return LLMModelStatus{}, fmt.Errorf("model is required")
	}
	response, errorValue := server.performLLMModelRequest(contextValue, target, runtimeRemoteModelUpdateAction, model)
	if errorValue != nil {
		return LLMModelStatus{}, errorValue
	}
	return LLMModelStatus{
		State:       "ok",
		Model:       response.Model,
		RuntimePath: response.RuntimePath,
		UpdatedAt:   response.UpdatedAt,
		Restarted:   response.Restarted,
	}, nil
}

func (server *Server) performLLMModelRequest(contextValue context.Context, target Target, action string, model string) (runtimeRemoteModelResponse, error) {
	identity, errorValue := targetIdentity(target)
	if errorValue != nil {
		return runtimeRemoteModelResponse{}, errorValue
	}
	payload := runtimeRemoteModelRequest{
		signedAdminRequest: signedAdminRequestPayload(identity, action),
		Model:              strings.TrimSpace(model),
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return runtimeRemoteModelResponse{}, errorValue
	}
	endpointURL := target.AdminURL + runtimeRemoteModelEndpointPath(action)
	request, errorValue := http.NewRequestWithContext(contextValue, http.MethodPost, endpointURL, bytes.NewReader(document))
	if errorValue != nil {
		return runtimeRemoteModelResponse{}, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	AttachCloudflareAccess(request)
	httpResponse, errorValue := server.client.Do(request)
	if errorValue != nil {
		return runtimeRemoteModelResponse{}, errorValue
	}
	defer httpResponse.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(httpResponse.Body, 256*1024))
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		if httpResponse.StatusCode == http.StatusNotFound {
			return runtimeRemoteModelResponse{}, fmt.Errorf("runtime model endpoint is not available; deploy admind first")
		}
		return runtimeRemoteModelResponse{}, fmt.Errorf("runtime model request failed: HTTP %d %s", httpResponse.StatusCode, strings.TrimSpace(string(body)))
	}
	var response runtimeRemoteModelResponse
	if errorValue := json.NewDecoder(bytes.NewReader(body)).Decode(&response); errorValue != nil {
		return runtimeRemoteModelResponse{}, errorValue
	}
	return response, nil
}

func runtimeRemoteModelEndpointPath(action string) string {
	if action == runtimeRemoteModelUpdateAction {
		return "/admin/api/runtime/remote-model/update"
	}
	return "/admin/api/runtime/remote-model/read"
}
