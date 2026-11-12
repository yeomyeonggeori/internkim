package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

// These tools live in the record, so this carries the call to admind, which
// runs it on the plane as the person who asked. Nothing is decided here.
func (service Service) invokeRecordTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	toolName := strings.TrimSpace(request.ToolName)
	if !theRecordAnswers(toolName) {
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("record tool is not configured: %s", toolName)
	}

	answer, status, errorValue := service.askTheRecord(ctx, toolName, "invoke", request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if status >= http.StatusBadRequest {
		return recordToolFailure(toolName, status, answer), nil
	}

	var answered struct {
		Result json.RawMessage `json:"result"`
	}
	if errorValue := json.Unmarshal(answer, &answered); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponseFrom(
		toolName,
		"ok",
		answered.Result,
		capabilityResponseOrigin{Provider: "internkim", SelectedBackend: "record"},
	)
}

func (service Service) askTheRecord(
	ctx context.Context,
	toolName string,
	verb string,
	request capabilities.ToolInvokeRequest,
) (json.RawMessage, int, error) {
	body, errorValue := json.Marshal(recordToolCallOf(request))
	if errorValue != nil {
		return nil, 0, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		admindRequesterURL("/record/api/tools/"+toolName+"/"+verb), bytes.NewReader(body))
	if errorValue != nil {
		return nil, 0, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := service.askAdmindAsTheRequester(httpRequest, request.Context.RequesterEmail)
	if errorValue != nil {
		return nil, 0, errorValue
	}
	defer httpResponse.Body.Close()
	answer, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return nil, 0, errorValue
	}
	return json.RawMessage(answer), httpResponse.StatusCode, nil
}

func recordToolCallOf(request capabilities.ToolInvokeRequest) capabilityprotocol.RecordToolCall {
	call := capabilityprotocol.RecordToolCall{Input: request.Input}
	if len(bytes.TrimSpace(call.Input)) == 0 {
		call.Input = json.RawMessage("{}")
	}
	if len(request.Transport.WorkspaceFiles) > 0 {
		call.Transport = &capabilityprotocol.ToolInvokeTransport{WorkspaceFiles: request.Transport.WorkspaceFiles}
	}
	return call
}

func recordToolFailure(toolName string, status int, answer json.RawMessage) capabilities.ToolInvokeResponse {
	return capabilityToolFailure(capabilityResponseOrigin{Provider: "internkim", SelectedBackend: "record"}, toolName, status, answer)
}

func capabilityToolFailure(origin capabilityResponseOrigin, toolName string, status int, answer json.RawMessage) capabilities.ToolInvokeResponse {
	var refusal struct {
		Error        string `json:"error"`
		Message      string `json:"message"`
		ErrorCode    string `json:"errorCode"`
		FailureStage string `json:"failureStage"`
		Retryable    bool   `json:"retryable"`
		SafeRetry    bool   `json:"safeRetry"`
	}
	json.Unmarshal(answer, &refusal)
	message := firstNonEmpty(refusal.Error, refusal.Message, string(answer))
	return capabilities.ToolInvokeResponse{
		Provider:        origin.Provider,
		SelectedBackend: origin.SelectedBackend,
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         message,
		IsError:         true,
		Message:         message,
		ErrorCode:       refusal.ErrorCode,
		FailureStage:    refusal.FailureStage,
		Retryable:       refusal.Retryable,
		SafeRetry:       refusal.SafeRetry,
		Result:          recordFailureResult(toolName, status, answer),
	}
}

func recordFailureResult(toolName string, status int, answer json.RawMessage) json.RawMessage {
	if json.Valid(answer) {
		return answer
	}
	log.Printf("capability.record_failure_not_json: tool=%s status=%d bytes=%d", toolName, status, len(answer))
	encoded, _ := json.Marshal(string(answer))
	return encoded
}

func theRecordAnswers(toolName string) bool {
	descriptor, hasDescriptor := capabilityToolDescriptorFor(toolName)
	return hasDescriptor && descriptor.AnsweredBy == capabilityprotocol.AnsweredByRecord
}

func (service Service) previewRecordToolTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	toolName := strings.TrimSpace(request.ToolName)
	answer, status, errorValue := service.askTheRecord(ctx, toolName, "target", request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if status >= http.StatusBadRequest {
		return recordToolFailure(toolName, status, answer), nil
	}

	var previewed struct {
		Target json.RawMessage `json:"target"`
	}
	if errorValue := json.Unmarshal(answer, &previewed); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if namesNoTarget(previewed.Target) {
		return capabilityToolWithoutTargetResponse(toolName), nil
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "record",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          "resolved",
		Result:          previewed.Target,
	}, nil
}

func namesNoTarget(target json.RawMessage) bool {
	trimmed := bytes.TrimSpace(target)
	return len(trimmed) == 0 || string(trimmed) == "null"
}
